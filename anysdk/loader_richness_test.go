package anysdk_test

import (
	"fmt"
	"testing"

	"gotest.tools/assert"

	"github.com/stackql/any-sdk/anysdk"
)

func TestRichSimpleOktaApplicationServiceRead(t *testing.T) {

	vr := "v0.1.0"
	pr, err := anysdk.LoadProviderByName("aws", vr, "./testdata/registry/src")
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	ps, err := pr.GetProviderService("s3")
	if err != nil {
		t.Fatalf("Test failed: could not locate ProviderService for aws.s3")
	}

	b, err := anysdk.GetServiceDocBytes(fmt.Sprintf("aws/%s/services/S3.yaml", vr), "")
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	svc, err := anysdk.LoadServiceDocFromBytes(ps, b)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	assert.Equal(t, svc.GetName(), "s3")
}
