package anysdk_test

import (
	"testing"

	"gotest.tools/assert"

	"github.com/stackql/any-sdk/anysdk"
)

func TestRichSimpleAwsS3BucketABACRequestBodyOverride(t *testing.T) {

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

	schemaDescription := schema.GetDescription()

	assert.Equal(t, schemaDescription, "A convenience for presentation")

	assert.Assert(t, schema != nil, "expected schema to be non-nil")
	props, _ := schema.GetProperties()
	assert.Assert(t, props != nil, "expected schema properties to be non-nil")
	_, hasLineItem := props["line_items"]
	_, hasStatus := props["status"]
	assert.Assert(t, hasLineItem, "expected schema to have 'line_items' property")
	assert.Assert(t, hasStatus, "expected schema to have 'status' property")

	finalSchema := expectedRequest.GetFinalSchema()
	assert.Assert(t, finalSchema != nil, "expected final schema to be non-nil")
	finalProps, _ := finalSchema.GetProperties()
	assert.Assert(t, len(finalProps) != 0, "expected final schema properties to be non-empty")
	_, finalHasStatus := finalProps["Status"]
	assert.Assert(t, finalHasStatus, "expected final schema to have 'Status' property")

	finalDescription := finalSchema.GetDescription()
	assert.Equal(t, finalDescription, "The ABAC status of the general purpose bucket. When ABAC is enabled for the general purpose bucket, you can use tags to manage access to the general purpose buckets as well as for cost tracking purposes. When ABAC is disabled for the general purpose buckets, you can only use tags for cost tracking purposes. For more information, see [Using tags with S3 general purpose buckets](https://docs.aws.amazon.com/AmazonS3/latest/userguide/buckets-tagging.html).")

	// Body media type should inherit from final schema and not override schema
	assert.Equal(t, expectedRequest.GetBodyMediaType(), "application/xml")

	assert.Equal(t, svc.GetName(), "s3")
}
