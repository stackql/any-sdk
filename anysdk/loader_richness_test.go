package anysdk_test

import (
	"testing"

	"gotest.tools/assert"

	"github.com/stackql/any-sdk/anysdk"
)

func TestRichSimpleOktaApplicationServiceRead(t *testing.T) {

	vr := "v0.1.0"
	svc, err := anysdk.LoadProviderAndServiceFromPaths(
		"./testdata/registry/src/aws/"+vr+"/provider.yaml",
		"./testdata/registry/src/aws/"+vr+"/services/s3.yaml",
	)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	rsc, rscErr := svc.GetResource("bucket_abac")
	if rscErr != nil {
		t.Fatalf("Test failed: could not locate resource bucket_abac, error: %v", rscErr)
	}
	method, methodErr := rsc.FindMethod("put_bucket_abac")
	if methodErr != nil {
		t.Fatalf("Test failed: could not locate method put_bucket_abac, error: %v", methodErr)
	}
	expectedRequest, hasRequest := method.GetRequest()
	if !hasRequest || expectedRequest == nil {
		t.Fatalf("Test failed: expected request is nil")
	}

	transform, hasTransform := expectedRequest.GetTransform()
	if !hasTransform || transform == nil {
		t.Fatalf("Test failed: expected transform is nil")
	}
	schema := expectedRequest.GetSchema()

	assert.Assert(t, schema != nil, "expected schema to be non-nil")
	props, _ := schema.GetProperties()
	assert.Assert(t, props != nil, "expected schema properties to be non-nil")
	_, hasLineItem := props["line_items"]
	_, hasStatus := props["status"]
	assert.Assert(t, hasLineItem, "expected schema to have 'line_items' property")
	assert.Assert(t, hasStatus, "expected schema to have 'status' property")
	assert.Equal(t, expectedRequest.GetBodyMediaType(), "application/xml")

	assert.Equal(t, svc.GetName(), "s3")
}
