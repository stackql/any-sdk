package anysdk_test

import (
	"encoding/json"
	"strings"
	"testing"

	"gotest.tools/assert"

	"github.com/stackql/any-sdk/internal/anysdk"
)

// loadGoogleStorage is the standard fixture for these tests. It loads the
// google storage service from the in-repo testdata. The storage service is
// useful because its `Bucket` schema has multi-level nesting (`encryption`,
// `iamConfiguration.uniformBucketLevelAccess`, `lifecycle.rule[].action`),
// arrays-of-objects (`cors`, `lifecycle.rule[]`), `additionalProperties`
// (`labels`), and method-level `request.required` annotations on insert
// (`required: [name]`).
func loadGoogleStorage(t *testing.T) anysdk.Service {
	t.Helper()
	vr := "v0.1.2"
	svc, err := anysdk.LoadProviderAndServiceFromPaths(
		"./testdata/registry/src/googleapis.com/"+vr+"/provider.yaml",
		"./testdata/registry/src/googleapis.com/"+vr+"/services/storage-v1.yaml",
	)
	assert.NilError(t, err)
	assert.Assert(t, svc != nil)
	return svc
}

// resourceFor pulls a resource by name and asserts.
func resourceFor(t *testing.T, svc anysdk.Service, name string) anysdk.Resource {
	t.Helper()
	rsc, err := svc.GetResource(name)
	assert.NilError(t, err)
	assert.Assert(t, rsc != nil)
	return rsc
}

// shapeAsMap decodes a JSON Schema subset blob into a map so tests can
// poke at nested keys without manually parsing.
func shapeAsMap(t *testing.T, shape string) map[string]any {
	t.Helper()
	if shape == "" {
		return nil
	}
	var m map[string]any
	err := json.Unmarshal([]byte(shape), &m)
	assert.NilError(t, err)
	return m
}

// findField returns the first field with the given name and param type,
// or nil. Tests use this in place of raw slice iteration so assertion
// failures point at the right line.
func findField(mi anysdk.MethodIntrospection, name string, pt anysdk.ParamType) anysdk.IntrospectedField {
	for _, f := range mi.GetFields() {
		if f.GetName() == name && f.GetParamType() == pt {
			return f
		}
	}
	return nil
}

// TestIntrospectMethod_GoogleStorageBuckets_GetHasResponseFields exercises
// the basic happy path on a read method: the GET on a bucket should
// produce a fat set of output rows (every property on the Bucket schema)
// and the required input parameter `bucket`.
func TestIntrospectMethod_GoogleStorageBuckets_GetHasResponseFields(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)
	assert.Assert(t, mi != nil)

	// Provenance fields should be filled in.
	assert.Equal(t, mi.GetResource(), "buckets")
	assert.Equal(t, mi.GetMethod(), "get")
	assert.Assert(t, mi.GetService() != "")

	// `bucket` is the required path param on GET /b/{bucket}.
	assert.Assert(t,
		findField(mi, "bucket", anysdk.ParamTypeInputRequired) != nil,
		"expected required input 'bucket'")

	// Output fields: at minimum `id`, `name`, `kind`, `encryption`, `cors`
	// — these are top-level properties on the Bucket schema.
	for _, want := range []string{"id", "name", "kind", "encryption", "cors", "iamConfiguration"} {
		assert.Assert(t,
			findField(mi, want, anysdk.ParamTypeOutput) != nil,
			"expected output field %q in response", want)
	}
}

// TestIntrospectMethod_GoogleStorageBuckets_ShapeForObjectField confirms
// that an object response field carries a non-empty JSON Schema subset
// in `shape`, with nested properties present beyond depth 1. Scalars
// should carry empty shape.
func TestIntrospectMethod_GoogleStorageBuckets_ShapeForObjectField(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)

	// `encryption` is `{properties: {defaultKmsKeyName: {type: string}}}` —
	// a two-level object. Shape must include the nested property.
	encryption := findField(mi, "encryption", anysdk.ParamTypeOutput)
	assert.Assert(t, encryption != nil, "missing encryption field")
	assert.Equal(t, encryption.GetType(), "object")
	assert.Assert(t, encryption.GetShape() != "", "object field must carry a shape")
	m := shapeAsMap(t, encryption.GetShape())
	props, ok := m["properties"].(map[string]any)
	assert.Assert(t, ok, "encryption shape missing properties")
	_, hasKMS := props["defaultKmsKeyName"]
	assert.Assert(t, hasKMS, "encryption.properties.defaultKmsKeyName missing in shape")

	// A scalar field must NOT carry shape.
	nameField := findField(mi, "name", anysdk.ParamTypeOutput)
	assert.Assert(t, nameField != nil, "missing name field")
	assert.Equal(t, nameField.GetShape(), "", "scalar field must not carry shape")
}

// TestIntrospectMethod_GoogleStorageBuckets_ShapeRendersDeepNesting walks
// into a three-level nested response field (`iamConfiguration`) and
// verifies the JSON Schema subset preserves the depth.
func TestIntrospectMethod_GoogleStorageBuckets_ShapeRendersDeepNesting(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)

	iam := findField(mi, "iamConfiguration", anysdk.ParamTypeOutput)
	assert.Assert(t, iam != nil)
	assert.Assert(t, iam.GetShape() != "", "iamConfiguration must have shape")

	m := shapeAsMap(t, iam.GetShape())
	level1 := m["properties"].(map[string]any)
	bplo, ok := level1["bucketPolicyOnly"].(map[string]any)
	assert.Assert(t, ok, "missing iamConfiguration.bucketPolicyOnly")
	level2 := bplo["properties"].(map[string]any)
	enabled, ok := level2["enabled"].(map[string]any)
	assert.Assert(t, ok, "missing iamConfiguration.bucketPolicyOnly.enabled")
	assert.Equal(t, enabled["type"], "boolean")
}

// TestIntrospectMethod_GoogleStorageBuckets_ArrayItemsHaveShape verifies
// that an array-of-objects field carries the items schema in `shape`.
func TestIntrospectMethod_GoogleStorageBuckets_ArrayItemsHaveShape(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)

	cors := findField(mi, "cors", anysdk.ParamTypeOutput)
	assert.Assert(t, cors != nil)
	assert.Equal(t, cors.GetType(), "array")
	assert.Assert(t, cors.GetShape() != "", "array field must have shape")
	m := shapeAsMap(t, cors.GetShape())
	items, ok := m["items"].(map[string]any)
	assert.Assert(t, ok, "cors shape must include items")
	assert.Equal(t, items["type"], "object")
	// The cors item is an inline object with method, origin, etc.
	itemProps, ok := items["properties"].(map[string]any)
	assert.Assert(t, ok, "cors items missing properties")
	_, hasMethod := itemProps["method"]
	assert.Assert(t, hasMethod, "cors items.properties.method missing")
}

// TestIntrospectMethod_GoogleStorageBuckets_InsertHasBodyRequired covers
// the body-field merge into inputs. The insert method has method-level
// `request.required: [name]` annotation, so `name` must be present as
// input_required.
func TestIntrospectMethod_GoogleStorageBuckets_InsertHasBodyRequired(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "insert", false)
	assert.NilError(t, err)

	requiredInputs := map[string]bool{}
	optionalInputs := map[string]bool{}
	for _, f := range mi.GetFields() {
		switch f.GetParamType() {
		case anysdk.ParamTypeInputRequired:
			requiredInputs[f.GetName()] = true
		case anysdk.ParamTypeInputOptional:
			optionalInputs[f.GetName()] = true
		}
	}

	// `project` is the required query param on POST /b.
	assert.Assert(t, requiredInputs["project"], "expected required input 'project'")

	// `name` (body field, promoted by request.required annotation).
	// The body-translation algorithm may or may not rename it; check both.
	hasName := requiredInputs["name"] || requiredInputs["data__name"]
	assert.Assert(t, hasName, "expected required body field 'name' (or renamed)")

	// Optional inputs should include other Bucket body fields like
	// `location` (or its renamed form).
	hasLocation := optionalInputs["location"] || optionalInputs["data__location"]
	hasACL := optionalInputs["acl"] || optionalInputs["data__acl"]
	assert.Assert(t, hasLocation || hasACL, "expected at least one optional body field")
}

// TestIntrospectMethod_GoogleStorageBuckets_ExtendedAddsDescription
// confirms that the `extended` flag is what gates the per-row description
// — non-extended leaves it empty, extended fills it in.
func TestIntrospectMethod_GoogleStorageBuckets_ExtendedAddsDescription(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	miPlain, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)

	miExt, err := anysdk.IntrospectMethod(rsc, "get", true)
	assert.NilError(t, err)

	plainID := findField(miPlain, "id", anysdk.ParamTypeOutput)
	extID := findField(miExt, "id", anysdk.ParamTypeOutput)
	assert.Assert(t, plainID != nil)
	assert.Assert(t, extID != nil)
	assert.Equal(t, plainID.GetDescription(), "", "non-extended must not include description")
	assert.Assert(t, extID.GetDescription() != "", "extended must include description for id")
}

// TestIntrospectMethod_GoogleStorageBuckets_ShapeAlwaysContainsDescription
// the description inside the shape JSON must always be present regardless
// of the `extended` flag — that's the agent-context-saving design choice.
func TestIntrospectMethod_GoogleStorageBuckets_ShapeAlwaysContainsDescription(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)

	encryption := findField(mi, "encryption", anysdk.ParamTypeOutput)
	assert.Assert(t, encryption != nil)
	m := shapeAsMap(t, encryption.GetShape())
	// `encryption` description in the source: "Encryption configuration for a bucket."
	desc, _ := m["description"].(string)
	assert.Assert(t, strings.Contains(strings.ToLower(desc), "encryption"),
		"shape JSON must always carry description regardless of extended flag, got %q", desc)
}

// TestIntrospectMethod_GoogleStorageBuckets_AdditionalProperties tests that
// `labels` (an additionalProperties-only object) emits an
// `additionalProperties` key in its shape rather than `properties`.
func TestIntrospectMethod_GoogleStorageBuckets_AdditionalProperties(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)

	labels := findField(mi, "labels", anysdk.ParamTypeOutput)
	assert.Assert(t, labels != nil)
	assert.Equal(t, labels.GetType(), "object")
	m := shapeAsMap(t, labels.GetShape())
	ap, ok := m["additionalProperties"].(map[string]any)
	assert.Assert(t, ok, "labels shape must include additionalProperties")
	assert.Equal(t, ap["type"], "string")
}

// TestIntrospectMethod_GoogleStorageBuckets_FieldOrderingIsStable verifies
// determinism: two introspections on the same method produce identical
// field ordering. Without this, agents see flaky output and golden tests
// in downstream repos break.
func TestIntrospectMethod_GoogleStorageBuckets_FieldOrderingIsStable(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi1, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)
	mi2, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)

	f1 := mi1.GetFields()
	f2 := mi2.GetFields()
	assert.Equal(t, len(f1), len(f2))
	for i := range f1 {
		assert.Equal(t, f1[i].GetName(), f2[i].GetName())
		assert.Equal(t, f1[i].GetParamType(), f2[i].GetParamType())
	}
}

// TestIntrospectMethod_UnknownMethodReturnsError confirms the error path
// for a method that doesn't exist on a known resource.
func TestIntrospectMethod_UnknownMethodReturnsError(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	_, err := anysdk.IntrospectMethod(rsc, "nonexistent_method", false)
	assert.Assert(t, err != nil, "expected error for unknown method")
	assert.Assert(t, strings.Contains(err.Error(), "introspect"),
		"error message should mention introspect, got: %v", err)
}

// TestIntrospectMethod_NilResource confirms the nil-resource guard.
func TestIntrospectMethod_NilResource(t *testing.T) {
	_, err := anysdk.IntrospectMethod(nil, "anything", false)
	assert.Assert(t, err != nil, "expected error for nil resource")
}

// TestIntrospectMethod_GoogleStorageBuckets_DeleteHasInputNoOutput tests
// the empty-response case. DELETE /b/{bucket} returns no body — the
// resolver should produce input rows but zero output rows.
func TestIntrospectMethod_GoogleStorageBuckets_DeleteHasInputNoOutput(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "delete", false)
	assert.NilError(t, err)

	var inputCount, outputCount int
	for _, f := range mi.GetFields() {
		switch f.GetParamType() {
		case anysdk.ParamTypeInputRequired, anysdk.ParamTypeInputOptional:
			inputCount++
		case anysdk.ParamTypeOutput:
			outputCount++
		}
	}
	assert.Assert(t, inputCount >= 1, "delete must have at least the bucket input")
	assert.Equal(t, outputCount, 0, "delete returns no body; output rows must be zero")
}

// TestIntrospectMethod_GoogleStorageBuckets_RequiredAndOptionalAreDisjoint
// confirms a field never appears as both required and optional in the
// same result.
func TestIntrospectMethod_GoogleStorageBuckets_RequiredAndOptionalAreDisjoint(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "insert", false)
	assert.NilError(t, err)

	required := map[string]bool{}
	for _, f := range mi.GetFields() {
		if f.GetParamType() == anysdk.ParamTypeInputRequired {
			required[f.GetName()] = true
		}
	}
	for _, f := range mi.GetFields() {
		if f.GetParamType() == anysdk.ParamTypeInputOptional {
			assert.Assert(t, !required[f.GetName()],
				"field %q appears as both required and optional", f.GetName())
		}
	}
}

// TestIntrospectMethod_GoogleStorageBuckets_ShapeIsValidJSON sanity-checks
// every non-empty shape blob in a full result. This catches regressions
// where a code path forgets to marshal or emits a truncated string.
func TestIntrospectMethod_GoogleStorageBuckets_ShapeIsValidJSON(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	for _, methodName := range []string{"get", "list", "insert", "update", "delete"} {
		mi, err := anysdk.IntrospectMethod(rsc, methodName, false)
		assert.NilError(t, err, "method %s", methodName)
		for _, f := range mi.GetFields() {
			shape := f.GetShape()
			if shape == "" {
				continue
			}
			var decoded interface{}
			err := json.Unmarshal([]byte(shape), &decoded)
			assert.NilError(t, err, "method=%s field=%s shape=%s",
				methodName, f.GetName(), shape)
		}
	}
}
