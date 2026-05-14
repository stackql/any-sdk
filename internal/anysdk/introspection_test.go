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

// TestIntrospectMethod_GoogleStorageBuckets_GetHasResponseFields exercises
// the basic happy path on a read method: the GET on a bucket should
// produce a fat set of output rows (every property on the Bucket schema)
// and the required input parameter `bucket`.
func TestIntrospectMethod_GoogleStorageBuckets_GetHasResponseFields(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)

	// Provenance fields should be filled in.
	assert.Equal(t, mi.Resource, "buckets")
	assert.Equal(t, mi.Method, "get")
	assert.Assert(t, mi.Service != "")

	// `bucket` is the required path param on GET /b/{bucket}.
	var seenBucket bool
	for _, f := range mi.Fields {
		if f.Name == "bucket" && f.ParamType == anysdk.ParamTypeInputRequired {
			seenBucket = true
			break
		}
	}
	assert.Assert(t, seenBucket, "expected required input 'bucket'")

	// Output fields: at minimum `id`, `name`, `kind`, `encryption`, `cors`
	// — these are top-level properties on the Bucket schema.
	expectedOutputs := []string{"id", "name", "kind", "encryption", "cors", "iamConfiguration"}
	for _, want := range expectedOutputs {
		var found bool
		for _, f := range mi.Fields {
			if f.Name == want && f.ParamType == anysdk.ParamTypeOutput {
				found = true
				break
			}
		}
		assert.Assert(t, found, "expected output field %q in response", want)
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
	var encryption anysdk.IntrospectedField
	var seenEncryption bool
	for _, f := range mi.Fields {
		if f.Name == "encryption" && f.ParamType == anysdk.ParamTypeOutput {
			encryption = f
			seenEncryption = true
			break
		}
	}
	assert.Assert(t, seenEncryption, "missing encryption field")
	assert.Equal(t, encryption.Type, "object")
	assert.Assert(t, encryption.Shape != "", "object field must carry a shape")
	m := shapeAsMap(t, encryption.Shape)
	props, ok := m["properties"].(map[string]any)
	assert.Assert(t, ok, "encryption shape missing properties")
	_, hasKMS := props["defaultKmsKeyName"]
	assert.Assert(t, hasKMS, "encryption.properties.defaultKmsKeyName missing in shape")

	// A scalar field must NOT carry shape.
	var nameField anysdk.IntrospectedField
	var seenName bool
	for _, f := range mi.Fields {
		if f.Name == "name" && f.ParamType == anysdk.ParamTypeOutput {
			nameField = f
			seenName = true
			break
		}
	}
	assert.Assert(t, seenName, "missing name field")
	assert.Equal(t, nameField.Shape, "", "scalar field must not carry shape")
}

// TestIntrospectMethod_GoogleStorageBuckets_ShapeRendersDeepNesting walks
// into a three-level nested response field (`iamConfiguration`) and
// verifies the JSON Schema subset preserves the depth.
func TestIntrospectMethod_GoogleStorageBuckets_ShapeRendersDeepNesting(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)

	var iam anysdk.IntrospectedField
	for _, f := range mi.Fields {
		if f.Name == "iamConfiguration" && f.ParamType == anysdk.ParamTypeOutput {
			iam = f
			break
		}
	}
	assert.Assert(t, iam.Shape != "", "iamConfiguration must have shape")

	m := shapeAsMap(t, iam.Shape)
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

	var cors anysdk.IntrospectedField
	for _, f := range mi.Fields {
		if f.Name == "cors" && f.ParamType == anysdk.ParamTypeOutput {
			cors = f
			break
		}
	}
	assert.Equal(t, cors.Type, "array")
	assert.Assert(t, cors.Shape != "", "array field must have shape")
	m := shapeAsMap(t, cors.Shape)
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
	for _, f := range mi.Fields {
		switch f.ParamType {
		case anysdk.ParamTypeInputRequired:
			requiredInputs[f.Name] = true
		case anysdk.ParamTypeInputOptional:
			optionalInputs[f.Name] = true
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

	// Find an output field that has a description in the source schema.
	var plainID, extID anysdk.IntrospectedField
	for _, f := range miPlain.Fields {
		if f.Name == "id" && f.ParamType == anysdk.ParamTypeOutput {
			plainID = f
		}
	}
	for _, f := range miExt.Fields {
		if f.Name == "id" && f.ParamType == anysdk.ParamTypeOutput {
			extID = f
		}
	}
	assert.Equal(t, plainID.Description, "", "non-extended must not include description")
	assert.Assert(t, extID.Description != "", "extended must include description for id")
}

// TestIntrospectMethod_GoogleStorageBuckets_ShapeAlwaysContainsDescription
// the description inside the shape JSON must always be present regardless
// of the `extended` flag — that's the agent-context-saving design choice.
func TestIntrospectMethod_GoogleStorageBuckets_ShapeAlwaysContainsDescription(t *testing.T) {
	svc := loadGoogleStorage(t)
	rsc := resourceFor(t, svc, "buckets")

	mi, err := anysdk.IntrospectMethod(rsc, "get", false)
	assert.NilError(t, err)

	var encryption anysdk.IntrospectedField
	for _, f := range mi.Fields {
		if f.Name == "encryption" && f.ParamType == anysdk.ParamTypeOutput {
			encryption = f
			break
		}
	}
	m := shapeAsMap(t, encryption.Shape)
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

	var labels anysdk.IntrospectedField
	for _, f := range mi.Fields {
		if f.Name == "labels" && f.ParamType == anysdk.ParamTypeOutput {
			labels = f
			break
		}
	}
	assert.Equal(t, labels.Type, "object")
	m := shapeAsMap(t, labels.Shape)
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

	assert.Equal(t, len(mi1.Fields), len(mi2.Fields))
	for i := range mi1.Fields {
		assert.Equal(t, mi1.Fields[i].Name, mi2.Fields[i].Name)
		assert.Equal(t, mi1.Fields[i].ParamType, mi2.Fields[i].ParamType)
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
	for _, f := range mi.Fields {
		switch f.ParamType {
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
	for _, f := range mi.Fields {
		if f.ParamType == anysdk.ParamTypeInputRequired {
			required[f.Name] = true
		}
	}
	for _, f := range mi.Fields {
		if f.ParamType == anysdk.ParamTypeInputOptional {
			assert.Assert(t, !required[f.Name],
				"field %q appears as both required and optional", f.Name)
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
		for _, f := range mi.Fields {
			if f.Shape == "" {
				continue
			}
			var decoded interface{}
			err := json.Unmarshal([]byte(f.Shape), &decoded)
			assert.NilError(t, err, "method=%s field=%s shape=%s",
				methodName, f.Name, f.Shape)
		}
	}
}
