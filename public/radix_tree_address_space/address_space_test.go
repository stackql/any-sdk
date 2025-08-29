package radix_tree_address_space_test

import (
	"fmt"
	"testing"

	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/discovery"
	"github.com/stackql/any-sdk/public/radix_tree_address_space"
	"github.com/stackql/stackql-provider-registry/signing/Ed25519/app/edcrypto"
)

func getNewTestDataMockRegistry(relativePath string) (anysdk.RegistryAPI, error) {
	return anysdk.NewRegistry(
		anysdk.RegistryConfig{
			RegistryURL:      fmt.Sprintf("file://%s", relativePath),
			LocalDocRoot:     relativePath,
			AllowSrcDownload: false,
			VerifyConfig: &edcrypto.VerifierConfig{
				NopVerify: true,
			},
		},
		nil)
}

func TestNewAddressSpace(t *testing.T) {
	addressSpace := radix_tree_address_space.NewAddressSpaceAnalyzer(
		radix_tree_address_space.NewAddressSpaceGrammar(),
		nil,
	)
	if addressSpace == nil {
		t.Fatalf("expected non-nil address space")
	}
}

func TestDeepDiscoveryGoogleCurrent(t *testing.T) {
	registryLocalPath := "./testdata/registry/basic"
	googleProviderPath := "testdata/registry/basic/src/googleapis.com/v0.1.2/provider.yaml"
	// expectedErrorCount := 282
	analyzerFactory := discovery.NewSimpleSQLiteAnalyzerFactory(registryLocalPath, dto.RuntimeCtx{})
	staticAnalyzer, analyzerErr := analyzerFactory.CreateProviderServiceLevelStaticAnalyzer(
		googleProviderPath,
		"compute",
	)
	if analyzerErr != nil {
		t.Fatalf("Failed to create static analyzer: %v", analyzerErr)
	}
	err := staticAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Static analysis failed: %v", err)
	}
	errorSlice := staticAnalyzer.GetErrors()
	for _, err := range errorSlice {
		t.Logf("Static analysis error: %v", err)
	}
	resources := staticAnalyzer.GetResources()
	t.Logf("Discovered %d resources", len(resources))
	if len(resources) == 0 {
		t.Fatalf("Static analysis failed: expected non-zero resources but got %d", len(resources))
	}
}
