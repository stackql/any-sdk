package stream_transform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/clbanning/mxj/v2"
)

// SchemaDrivenXMLV1 is a new transform family (distinct from golang_template_mxj_v*)
// that projects mxj-decoded XML rows using the schema referenced by
// response.schema_override, instead of a hand-rolled per-op Go template.
const SchemaDrivenXMLV1 = "schema_driven_xml_v0.1.0"

// AWS wire-protocol identifiers, carried by the spec's info.x-protocol extension.
const (
	XProtocolQuery   = "query"
	XProtocolEC2     = "ec2"
	XProtocolRestXML = "rest-xml"
)

// SchemaTree is the minimal, dependency-free view of a schema node the walker
// needs. The caller (any-sdk's operation store) adapts its own Schema to this so
// that this package never imports internal/anysdk (which would be an import cycle).
type SchemaTree interface {
	// Type returns the OpenAPI type: object | array | string | integer | number | boolean.
	Type() string
	// Items returns the element schema for an array node.
	Items() (SchemaTree, bool)
	// Property returns the named child property schema for an object node.
	Property(name string) (SchemaTree, bool)
	// Properties returns all child property schemas keyed by their wire name.
	Properties() map[string]SchemaTree
}

// schemaDrivenXMLTransformer walks one XML body into stackql's
// {"<listProperty>": [...]} envelope using the row schema.
type schemaDrivenXMLTransformer struct {
	input        string
	overrideTree SchemaTree
	protocol     string
	listProperty string
	outStream    io.ReadWriter
}

func newSchemaDrivenXMLTransformer(
	input string,
	overrideTree SchemaTree,
	protocol string,
	listProperty string,
	outStream io.ReadWriter,
) (StreamTransformer, error) {
	if overrideTree == nil {
		return nil, fmt.Errorf("schema_driven_xml: nil override schema")
	}
	if listProperty == "" {
		return nil, fmt.Errorf("schema_driven_xml: empty list property (objectKey)")
	}
	if outStream == nil {
		outStream = bytes.NewBuffer(nil)
	}
	return &schemaDrivenXMLTransformer{
		input:        input,
		overrideTree: overrideTree,
		protocol:     protocol,
		listProperty: listProperty,
		outStream:    outStream,
	}, nil
}

func (t *schemaDrivenXMLTransformer) GetOutStream() io.Reader {
	if t.outStream == nil {
		return bytes.NewBuffer(nil)
	}
	return t.outStream
}

func (t *schemaDrivenXMLTransformer) Transform() error {
	rowSchema, err := t.rowSchema()
	if err != nil {
		return err
	}
	// Decode WITHOUT mxj casting so leaf values stay strings; the schema's declared
	// type is then authoritative (this is what stops 12-digit IDs becoming float64).
	decoded, err := mxj.NewMapXml([]byte(t.input))
	if err != nil {
		return err
	}
	payload, ok := t.payloadMap(map[string]interface{}(decoded))
	if !ok {
		// Unrecognised envelope: emit an empty result rather than erroring.
		return t.write(make([]interface{}, 0))
	}
	rows := extractRows(payload, rowSchema)
	projected := make([]interface{}, 0, len(rows))
	for _, row := range rows {
		projected = append(projected, projectRow(row, rowSchema))
	}
	return t.write(projected)
}

func (t *schemaDrivenXMLTransformer) write(rows []interface{}) error {
	out := map[string]interface{}{t.listProperty: rows}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, writeErr := t.outStream.Write(b)
	return writeErr
}

// rowSchema resolves the per-row schema: overrideSchema.<listProperty>.items.
func (t *schemaDrivenXMLTransformer) rowSchema() (SchemaTree, error) {
	listSchema, ok := t.overrideTree.Property(t.listProperty)
	if !ok {
		return nil, fmt.Errorf("schema_driven_xml: list property %q not found in override schema", t.listProperty)
	}
	items, ok := listSchema.Items()
	if !ok {
		return nil, fmt.Errorf("schema_driven_xml: list property %q is not an array", t.listProperty)
	}
	return items, nil
}

// payloadMap skips the protocol envelope and returns the map to inspect for rows.
func (t *schemaDrivenXMLTransformer) payloadMap(decoded map[string]interface{}) (map[string]interface{}, bool) {
	top := unwrapSingle(decoded) // <OpName>Response (query/ec2) or service root (rest-xml)
	topMap, ok := top.(map[string]interface{})
	if !ok {
		return nil, false
	}
	if t.protocol == XProtocolQuery {
		if rm, ok := resultWrapper(topMap); ok {
			return rm, true
		}
	}
	return topMap, true
}

// extractRows decides singleton vs list and returns the row maps.
func extractRows(payload map[string]interface{}, rowSchema SchemaTree) []map[string]interface{} {
	rowProps := rowSchema.Properties()
	if mapHasAnyKey(payload, rowProps) {
		// The payload itself carries the row's fields -> singleton response.
		return []map[string]interface{}{payload}
	}
	member, ok := findListMember(payload)
	if !ok {
		return nil
	}
	return normalizeRows(member)
}

func projectRow(row map[string]interface{}, rowSchema SchemaTree) map[string]interface{} {
	out := make(map[string]interface{}, len(rowSchema.Properties()))
	for name, propSchema := range rowSchema.Properties() {
		raw, ok := row[name]
		if !ok {
			out[name] = nil
			continue
		}
		out[name] = convertValue(raw, propSchema.Type())
	}
	return out
}

// convertValue applies the schema-declared type to a (string-typed) mxj leaf value.
func convertValue(raw interface{}, schemaType string) interface{} {
	if raw == nil {
		return nil
	}
	// Self-closing element (<x/>) decodes to "" in mxj -> project as null.
	if s, ok := raw.(string); ok && s == "" && schemaType != "string" {
		return nil
	}
	switch schemaType {
	case "integer":
		if s, ok := raw.(string); ok {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				return n
			}
		}
		return raw
	case "number":
		if s, ok := raw.(string); ok {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f
			}
		}
		return raw
	case "boolean":
		if s, ok := raw.(string); ok {
			if b, err := strconv.ParseBool(s); err == nil {
				return b
			}
		}
		return raw
	case "object", "array":
		b, err := json.Marshal(raw)
		if err != nil {
			return raw
		}
		return string(b)
	case "string":
		if s, ok := raw.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", raw)
	default:
		return raw
	}
}

// --- envelope navigation helpers ---

func unwrapSingle(m map[string]interface{}) interface{} {
	if len(m) == 1 {
		for _, v := range m {
			return v
		}
	}
	return m
}

func resultWrapper(m map[string]interface{}) (map[string]interface{}, bool) {
	for _, k := range sortedKeys(m) {
		if strings.HasSuffix(k, "Result") {
			if rm, ok := m[k].(map[string]interface{}); ok {
				return rm, true
			}
		}
	}
	return nil, false
}

func mapHasAnyKey(m map[string]interface{}, props map[string]SchemaTree) bool {
	for k := range props {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

// findListMember locates the member of payload that bears the row list. It tries,
// in deterministic key order: a direct slice; a map containing botocore's default
// "item" wrapper; a map whose sole child is a slice/map (locationName wrap); then
// an empty self-closing member ("").
func findListMember(payload map[string]interface{}) (interface{}, bool) {
	keys := sortedKeys(payload)
	for _, k := range keys {
		if _, ok := payload[k].([]interface{}); ok {
			return payload[k], true
		}
	}
	for _, k := range keys {
		if mm, ok := payload[k].(map[string]interface{}); ok {
			if _, hasItem := mm["item"]; hasItem {
				return mm, true
			}
		}
	}
	for _, k := range keys {
		if mm, ok := payload[k].(map[string]interface{}); ok && len(mm) == 1 {
			for _, v := range mm {
				switch v.(type) {
				case []interface{}, map[string]interface{}:
					return mm, true
				}
			}
		}
	}
	for _, k := range keys {
		if s, ok := payload[k].(string); ok && s == "" {
			return "", true
		}
	}
	return nil, false
}

func normalizeRows(member interface{}) []map[string]interface{} {
	switch v := member.(type) {
	case nil:
		return nil
	case string:
		return nil // empty self-closing list
	case []interface{}:
		var rows []map[string]interface{}
		for _, e := range v {
			if rm, ok := e.(map[string]interface{}); ok {
				rows = append(rows, rm)
			}
		}
		return rows
	case map[string]interface{}:
		if item, ok := v["item"]; ok {
			return normalizeRows(item)
		}
		if len(v) == 1 {
			for _, child := range v {
				switch child.(type) {
				case []interface{}, map[string]interface{}:
					return normalizeRows(child)
				}
			}
		}
		return []map[string]interface{}{v} // single row
	default:
		return nil
	}
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
