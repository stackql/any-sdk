package anysdk

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

// ParamType classifies a row produced by IntrospectMethod. The classification
// is the only thing a caller needs to understand whether a field is something
// they supply or something the provider returns. Whether an input param goes
// in the path, query, header or body is intentionally hidden — stackql treats
// methods as a single uniform input surface.
type ParamType string

const (
	ParamTypeInputRequired ParamType = "input_required"
	ParamTypeInputOptional ParamType = "input_optional"
	ParamTypeOutput        ParamType = "output"
)

// introspectionMaxDepth is a hard ceiling on schema-walker recursion. It is
// not a UX knob; it is a defence against pathological non-cyclic-but-deep
// schemas that the $ref/identity cycle guard would not catch (because each
// node is a distinct schema). Tune only if you see legitimate provider
// schemas truncating.
const introspectionMaxDepth = 64

// IntrospectedField is one row of a DESCRIBE METHOD result. Consumers read
// fields through accessors; the concrete implementation is package-private
// so the wire shape can evolve without breaking dependents.
//
// GetShape returns a JSON Schema subset (text). It is empty for scalar
// fields; for object/array fields it carries the nested structure the
// caller needs to construct a payload or interpret a response without
// making further round trips. The subset includes type, format, properties,
// items, required, enum, default, description, and the OpenAPI booleans
// readOnly/writeOnly/deprecated. Polymorphism (oneOf/anyOf/allOf) is
// preserved when present in the source document — providers in this
// registry usually fatten it at generation time, but if any survives it is
// rendered.
type IntrospectedField interface {
	GetName() string
	GetType() string
	GetParamType() ParamType
	GetShape() string
	GetDescription() string
}

// MethodIntrospection is the structured form of one DESCRIBE METHOD result.
// Consumers read provenance plus the row slice through accessors. The
// grammar-side caller will flatten GetFields() into a SQL result set.
type MethodIntrospection interface {
	GetProvider() string
	GetService() string
	GetResource() string
	GetMethod() string
	GetFields() []IntrospectedField
}

// introspectedField is the package-private implementation of
// IntrospectedField. Field values are populated by the resolver and never
// mutated after construction; accessors are pure reads.
type introspectedField struct {
	name        string
	dataType    string
	paramType   ParamType
	shape       string
	description string
}

func (f *introspectedField) GetName() string         { return f.name }
func (f *introspectedField) GetType() string         { return f.dataType }
func (f *introspectedField) GetParamType() ParamType { return f.paramType }
func (f *introspectedField) GetShape() string        { return f.shape }
func (f *introspectedField) GetDescription() string  { return f.description }

// methodIntrospection is the package-private implementation of
// MethodIntrospection. The same construction-then-read discipline applies.
type methodIntrospection struct {
	provider string
	service  string
	resource string
	method   string
	fields   []IntrospectedField
}

func (m *methodIntrospection) GetProvider() string          { return m.provider }
func (m *methodIntrospection) GetService() string           { return m.service }
func (m *methodIntrospection) GetResource() string          { return m.resource }
func (m *methodIntrospection) GetMethod() string            { return m.method }
func (m *methodIntrospection) GetFields() []IntrospectedField { return m.fields }

// IntrospectMethod returns input and output field metadata for a single
// method on a resource. It is the any-sdk side of the `DESCRIBE METHOD`
// SQL primitive. The function is intentionally a free function so it does
// not mutate any existing interface: callers obtain a Resource through the
// usual hierarchy lookup and pass it in.
//
// The extended flag controls whether the per-row description is populated;
// the description that lives *inside* the shape JSON is always present (it
// is small, useful, and would cost an extra query to fetch separately).
//
// Empty-response methods (e.g. 204 No Content) produce zero output rows.
// Input rows are always produced when the method has any input parameter.
func IntrospectMethod(rsc Resource, methodName string, extended bool) (MethodIntrospection, error) {
	if rsc == nil {
		return nil, fmt.Errorf("introspect: resource is nil")
	}
	method, err := rsc.FindMethod(methodName)
	if err != nil {
		return nil, fmt.Errorf("introspect: %w", err)
	}
	if method == nil {
		return nil, fmt.Errorf("introspect: method %q not found", methodName)
	}

	out := &methodIntrospection{
		resource: rsc.GetName(),
		method:   methodName,
	}
	if svc, ok := rsc.GetService(); ok && svc != nil {
		out.service = svc.GetName()
	}
	if prov, ok := rsc.GetProvider(); ok && prov != nil {
		out.provider = prov.GetName()
	}

	inputs, err := collectInputs(method, extended)
	if err != nil {
		return nil, err
	}
	out.fields = append(out.fields, inputs...)

	outputs, err := collectOutputs(method, extended)
	if err != nil {
		return nil, err
	}
	out.fields = append(out.fields, outputs...)

	return out, nil
}

// collectInputs returns rows for required and optional input parameters of
// the method, regardless of HTTP location. Body parameters are included via
// the same merge logic the rest of any-sdk uses for SQL projections
// (renamed where translation is configured, raw otherwise). Method-level
// `request.required` annotations are honored: any body field listed there
// is promoted to input_required even if the underlying schema does not
// mark it required.
func collectInputs(m StandardOperationStore, extended bool) ([]IntrospectedField, error) {
	// Method-level required-overrides for body fields (raw, pre-rename).
	bodyRequiredOverride := map[string]struct{}{}
	if op, ok := m.(*standardOpenAPIOperationStore); ok && op.Request != nil {
		for _, r := range op.Request.Required {
			bodyRequiredOverride[r] = struct{}{}
		}
	}

	required := m.GetRequiredParameters()
	optional := m.GetOptionalParameters()

	// Body required-override pass: if the method-level annotation says a
	// body field is required, ensure it lands in `required` even when
	// schema-level required does not list it. Without this, the user-facing
	// behaviour diverges from SHOW METHODS.
	if bodySchema, bodyErr := m.GetRequestBodySchema(); bodyErr == nil && bodySchema != nil && len(bodyRequiredOverride) > 0 {
		for rawKey := range bodyRequiredOverride {
			renamedKey, renameErr := m.RenameRequestBodyAttribute(rawKey)
			if renameErr != nil {
				continue
			}
			if _, alreadyRequired := required[renamedKey]; alreadyRequired {
				continue
			}
			if v, isOptional := optional[renamedKey]; isOptional {
				required[renamedKey] = v
				delete(optional, renamedKey)
			}
		}
	}

	var rows []IntrospectedField

	requiredKeys := make([]string, 0, len(required))
	for k := range required {
		requiredKeys = append(requiredKeys, k)
	}
	sort.Strings(requiredKeys)
	for _, k := range requiredKeys {
		row, err := fieldFromAddressable(k, required[k], ParamTypeInputRequired, extended)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	optionalKeys := make([]string, 0, len(optional))
	for k := range optional {
		// Deduplicate: a key already in required (post-override) wins.
		if _, ok := required[k]; ok {
			continue
		}
		optionalKeys = append(optionalKeys, k)
	}
	sort.Strings(optionalKeys)
	for _, k := range optionalKeys {
		row, err := fieldFromAddressable(k, optional[k], ParamTypeInputOptional, extended)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	return rows, nil
}

// collectOutputs returns rows for top-level fields of the method's response
// payload, anchored at the *selectable* sub-schema (matches what stackql
// SELECTs from). Methods with no response schema (e.g. 204) produce nil.
func collectOutputs(m StandardOperationStore, extended bool) ([]IntrospectedField, error) {
	respSchema, _, err := m.GetSelectSchemaAndObjectPath()
	if err != nil || respSchema == nil {
		// Fall back to the raw response schema in case the select-items key
		// resolution failed but a schema is still present (rare; e.g. for
		// operations whose response is a scalar). If that is also missing,
		// emit nothing — empty response is a legitimate state.
		respSchema, _, err = m.GetResponseBodySchemaAndMediaType()
		if err != nil || respSchema == nil {
			return nil, nil
		}
	}

	ss, ok := respSchema.(*standardSchema)
	if !ok {
		return nil, nil
	}

	// If the selectable schema is an array, anchor on its items.
	if ss.getType() == "array" {
		if items, itemsErr := ss.GetItems(); itemsErr == nil && items != nil {
			if itemSchema, isStd := items.(*standardSchema); isStd {
				ss = itemSchema
			}
		}
	}

	props := ss.getProperties()
	if len(props) == 0 {
		// A scalar or empty response: emit nothing. Callers can recognise
		// "no output rows" as "this method has no enumerable response
		// fields"; it is a more honest signal than a synthetic placeholder.
		return nil, nil
	}

	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var rows []IntrospectedField
	for _, k := range keys {
		child, _ := props[k].(*standardSchema)
		if child == nil {
			continue
		}
		shape := renderShape(child)
		row := &introspectedField{
			name:      k,
			dataType:  typeOf(child),
			paramType: ParamTypeOutput,
			shape:     shape,
		}
		if extended {
			row.description = child.getDescription()
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// fieldFromAddressable builds one input row from an Addressable. The
// Addressable already carries the user-facing name (renamed where stackql
// has configured a body translation algorithm).
func fieldFromAddressable(name string, addr Addressable, pt ParamType, extended bool) (IntrospectedField, error) {
	if addr == nil {
		return nil, fmt.Errorf("introspect: nil addressable for %q", name)
	}
	s, _ := addr.GetSchema()
	row := &introspectedField{
		name:      name,
		dataType:  addr.GetType(),
		paramType: pt,
	}
	if ss, ok := s.(*standardSchema); ok && ss != nil {
		row.shape = renderShape(ss)
		if extended {
			row.description = ss.getDescription()
		}
	}
	return row, nil
}

// typeOf returns the openapi type, accounting for the allOf-merge case
// where the type lives on a contributing variant rather than the parent.
func typeOf(s *standardSchema) string {
	if s == nil {
		return ""
	}
	t := s.getType()
	if t != "" {
		return t
	}
	// Empty type and no allOf fallback: treat as object if it has properties,
	// otherwise leave blank.
	if len(s.Properties) > 0 {
		return "object"
	}
	return ""
}

// renderShape produces a JSON Schema subset for a schema node. Scalar
// fields return "" so the FLAT row stays light; object/array fields return
// a JSON object whose structure mirrors the OpenAPI schema, cycle-guarded
// and depth-ceilinged. The output is text containing JSON for cross-backend
// portability (SQLite has no jsonb; we don't try to special-case Postgres).
//
// The "subset" omits validation keywords (minLength, pattern, multipleOf,
// minItems, etc.) — agents construct payloads, they don't enforce them. It
// keeps everything that affects what a *valid example value* looks like.
func renderShape(s *standardSchema) string {
	if s == nil {
		return ""
	}
	t := typeOf(s)
	if t != "object" && t != "array" && len(s.OneOf) == 0 && len(s.AnyOf) == 0 && len(s.AllOf) == 0 {
		// Scalar — caller already has the type in the row's `type` column.
		// Empty shape keeps non-extended output compact.
		return ""
	}
	visited := newVisitMap()
	node := buildShape(s, visited, 0)
	if node == nil {
		return ""
	}
	b, err := json.Marshal(node)
	if err != nil {
		// Marshalling a map[string]any of primitives should not fail; if it
		// does we degrade gracefully rather than aborting introspection.
		return ""
	}
	return string(b)
}

// buildShape recursively constructs the JSON Schema subset for a schema.
// The visited map tracks the *ancestor path* (not "anywhere ever") so a
// schema reached through two unrelated subtrees is not falsely elided.
// Entries are popped on unwind via defer.
func buildShape(s *standardSchema, visited *visitMap, depth int) map[string]any {
	if s == nil {
		return nil
	}

	// Hard depth ceiling: protects against deeply-nested non-cyclic schemas
	// that escape the cycle guard because every level is a distinct schema.
	if depth >= introspectionMaxDepth {
		return map[string]any{"type": typeOf(s), "x-stackql-truncated": true}
	}

	// Cycle detection: identity + $ref.
	enterErr := visited.enter(s)
	if enterErr != "" {
		return map[string]any{"type": typeOf(s), "x-stackql-cycle-ref": enterErr}
	}
	defer visited.exit(s)

	node := map[string]any{}

	// allOf-flatten: matches the wider any-sdk convention. Most providers in
	// this registry have already collapsed polymorphism at generation time;
	// where allOf survives, fold it once before emitting.
	working := s
	if len(s.AllOf) > 0 && len(s.Properties) == 0 && s.Items == nil {
		if fat, ok := s.getFattnedPolymorphicSchema().(*standardSchema); ok && fat != nil {
			working = fat
		}
	}

	t := typeOf(working)
	if t != "" {
		node["type"] = t
	}
	if working.Format != "" {
		node["format"] = working.Format
	}
	if working.Description != "" {
		node["description"] = working.Description
	}
	if working.Default != nil {
		node["default"] = working.Default
	}
	if len(working.Enum) > 0 {
		node["enum"] = working.Enum
	}
	if working.ReadOnly {
		node["readOnly"] = true
	}
	if working.WriteOnly {
		node["writeOnly"] = true
	}
	if working.Deprecated {
		node["deprecated"] = true
	}
	if len(working.Required) > 0 {
		node["required"] = append([]string(nil), working.Required...)
	}

	// Properties: emit in sorted order for stable output.
	if len(working.Properties) > 0 {
		props := map[string]any{}
		keys := make([]string, 0, len(working.Properties))
		for k := range working.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			ref := working.Properties[k]
			if ref == nil || ref.Value == nil {
				continue
			}
			child := newStandardSchema(ref.Value, working.svc, k, ref.Ref)
			props[k] = buildShape(child, visited, depth+1)
		}
		node["properties"] = props
	}

	// Items.
	if working.Items != nil && working.Items.Value != nil {
		child := newStandardSchema(working.Items.Value, working.svc, "", working.Items.Ref)
		node["items"] = buildShape(child, visited, depth+1)
	}

	// AdditionalProperties: emit when it is a schema (not the boolean form).
	if working.AdditionalProperties != nil && working.AdditionalProperties.Value != nil {
		child := newStandardSchema(working.AdditionalProperties.Value, working.svc, "", working.AdditionalProperties.Ref)
		node["additionalProperties"] = buildShape(child, visited, depth+1)
	}

	// Polymorphism: emitted as-is when not already folded. Most providers in
	// the registry collapse these at generation time, so this rarely fires;
	// when it does, agents get to see the variants.
	if len(s.OneOf) > 0 {
		node["oneOf"] = renderSchemaRefs(s.OneOf, s.svc, visited, depth+1)
	}
	if len(s.AnyOf) > 0 {
		node["anyOf"] = renderSchemaRefs(s.AnyOf, s.svc, visited, depth+1)
	}
	allOfWasFolded := working != s
	if !allOfWasFolded && len(s.AllOf) > 0 {
		// Emit allOf raw only when we did not fold it into properties above.
		// Folding happens when the parent schema has no direct properties of
		// its own; otherwise allOf is informative metadata the agent can use.
		node["allOf"] = renderSchemaRefs(s.AllOf, s.svc, visited, depth+1)
	}

	return node
}

func renderSchemaRefs(refs openapi3.SchemaRefs, svc OpenAPIService, visited *visitMap, depth int) []any {
	out := make([]any, 0, len(refs))
	for _, ref := range refs {
		if ref == nil || ref.Value == nil {
			continue
		}
		child := newStandardSchema(ref.Value, svc, "", ref.Ref)
		out = append(out, buildShape(child, visited, depth))
	}
	return out
}

// visitMap tracks the schemas currently on the recursion stack. Both
// pointer identity (for inline cycles) and $ref string (for named cycles
// where the loader may have produced distinct *openapi3.Schema pointers)
// are checked. Entries pop on unwind, so a schema reached through two
// independent subtrees is not falsely treated as a cycle.
type visitMap struct {
	bySchema map[*openapi3.Schema]string
	byRef    map[string]struct{}
}

func newVisitMap() *visitMap {
	return &visitMap{
		bySchema: map[*openapi3.Schema]string{},
		byRef:    map[string]struct{}{},
	}
}

// enter returns the cycle marker (the $ref string, or a synthetic
// identifier) if the schema is already on the stack, or "" if it is fresh.
func (v *visitMap) enter(s *standardSchema) string {
	if s == nil || s.Schema == nil {
		return ""
	}
	if existing, ok := v.bySchema[s.Schema]; ok {
		if existing != "" {
			return existing
		}
		return fmt.Sprintf("inline:%p", s.Schema)
	}
	if s.path != "" {
		if _, ok := v.byRef[s.path]; ok {
			return s.path
		}
		v.byRef[s.path] = struct{}{}
	}
	v.bySchema[s.Schema] = s.path
	return ""
}

func (v *visitMap) exit(s *standardSchema) {
	if s == nil || s.Schema == nil {
		return
	}
	delete(v.bySchema, s.Schema)
	if s.path != "" {
		delete(v.byRef, s.path)
	}
}
