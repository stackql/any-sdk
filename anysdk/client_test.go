package anysdk_test

import (
	"os"
	"path"
	"testing"

	"github.com/stackql/any-sdk/anysdk"
	. "github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/fileutil"

	"gotest.tools/assert"
)

func init() {
	var err error
	OpenapiFileRoot, err = fileutil.GetFilePathFromRepositoryRoot("test/registry/src")
	if err != nil {
		os.Exit(1)
	}
}

func TestLocalTemplateClient(t *testing.T) {
	providerPath := path.Join(OpenapiFileRoot, "local_openssl", "v0.1.0", "provider.yaml")
	servicePath := path.Join(OpenapiFileRoot, "local_openssl", "v0.1.0", "services", "keys.yaml")
	pb, err := os.ReadFile(providerPath)
	if err != nil {
		t.Fatalf("error loading provider doc: %v", err)
	}
	prov, err := anysdk.LoadProviderDocFromBytes(pb)
	if err != nil {
		t.Fatalf("error loading provider doc: %v", err)
	}
	assert.Assert(t, prov != nil)
	svc, err := LoadProviderAndServiceFromPaths(providerPath, servicePath)
	if err != nil {
		t.Fatalf("error loading service: %v", err)
	}
	res, err := svc.GetResource("rsa")
	if err != nil {
		t.Fatalf("error loading resource: %v", err)
	}
	opStore, err := res.FindMethod("create_key_pair")
	if err != nil {
		t.Fatalf("error loading method: %v", err)
	}
	assert.Assert(t, opStore != nil)
}
