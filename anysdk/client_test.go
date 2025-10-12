package anysdk_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/fileutil"

	"gotest.tools/assert"

	"github.com/stackql/any-sdk/pkg/local_template_executor"
)

var (
	testRoot string
)

func init() {
	var err error
	OpenapiFileRoot, err = fileutil.GetFilePathFromRepositoryRoot("test/registry/src")
	if err != nil {
		os.Exit(1)
	}
	testRoot, err = fileutil.GetFilePathFromRepositoryRoot("test")
	if err != nil {
		os.Exit(1)
	}
}

func TestLocalTemplateClient(t *testing.T) {
	providerPath := filepath.Join(OpenapiFileRoot, "local_openssl", "v0.1.0", "provider.yaml")
	servicePath := filepath.Join(OpenapiFileRoot, "local_openssl", "v0.1.0", "services", "keys.yaml")
	pb, err := os.ReadFile(providerPath)
	if err != nil {
		t.Fatalf("error loading provider doc: %v", err)
	}
	prov, err := LoadProviderDocFromBytes(pb)
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
	args := opStore.GetInline()
	if len(args) == 0 {
		t.Fatalf("no args found")
	}
	executor := local_template_executor.NewLocalTemplateExecutor(args[0], args[1:], nil)
	resp, err := executor.Execute(map[string]any{
		"parameters": map[string]any{
			"config_file":   filepath.Join(testRoot, "openssl/openssl.cnf"),
			"key_out_file":  filepath.Join(testRoot, "tmp/key.pem"),
			"cert_out_file": filepath.Join(testRoot, "tmp/cert.pem"),
			"days":          90,
		},
	})
	if err != nil {
		t.Fatalf("error executing command: %v", err)
	}
	stdOut, ok := resp.GetStdOut()
	if !ok {
		t.Fatalf("no stdout")
	}
	t.Logf("stdout: %s", stdOut.String())
}
