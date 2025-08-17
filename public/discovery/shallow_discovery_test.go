package discovery_test

import (
	"fmt"
	"testing"

	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/db/sqlcontrol"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/discovery"
	"github.com/stackql/any-sdk/public/persistence"
	"github.com/stackql/any-sdk/public/sqlengine"
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

func TestDiscovery01(t *testing.T) {
	registryLocalPath := "./testdata/registry/basic"
	controlAttributes := sqlcontrol.GetControlAttributes("standard")
	sqlCfg, err := dto.GetSQLBackendCfg("{}")
	if err != nil {
		t.Fatalf("Failed to get SQL backend config: %v", err)
	}
	sqlEngine, engineErr := sqlengine.NewSQLEngine(
		sqlCfg,
		controlAttributes,
	)
	if engineErr != nil {
		t.Fatalf("Failed to create SQL engine: %v", engineErr)
	}
	persistenceSystem, err := persistence.NewSQLPersistenceSystem("naive", sqlEngine)
	if err != nil {
		t.Fatalf("Failed to create persistence system: %v", err)
	}
	if persistenceSystem == nil {
		t.Fatal("Failed to create persistence system: got nil")
	}
	setUpScript, scriptErr := sqlengine.GetSQLEngineSetupDDL("sqlite")
	if scriptErr != nil {
		t.Fatalf("Failed to get SQL engine setup DDL: %v", scriptErr)
	}
	scriptRunErr := sqlEngine.ExecInTxn([]string{setUpScript})
	if scriptRunErr != nil {
		t.Fatalf("Failed to run SQL engine setup DDL: %v", scriptRunErr)
	}
	putErr := persistenceSystem.CacheStorePut("key", []byte("value"), "", 3600)
	if putErr != nil {
		t.Fatalf("Failed to put cache: %v", putErr)
	}
	cachedVal, getErr := persistenceSystem.CacheStoreGet("key")
	if getErr != nil {
		t.Fatalf("Failed to get cache: %v", getErr)
	}
	if string(cachedVal) != "value" {
		t.Fatalf("Unexpected cached value: %v", string(cachedVal))
	}
	registry, registryErr := getNewTestDataMockRegistry(registryLocalPath)
	if registryErr != nil {
		t.Fatalf("Failed to create mock registry: %v", registryErr)
	}
	awsProvider, providersErr := registry.LoadProviderByName("aws", "v0.1.0")
	if providersErr != nil {
		t.Fatalf("Failed to list all available providers: %v", providersErr)
	}
	if awsProvider == nil {
		t.Fatal("Expected 'aws' provider to be available")
	}
}

func TestDiscovery02(t *testing.T) {
	registryLocalPath := "./testdata/registry/basic"
	controlAttributes := sqlcontrol.GetControlAttributes("standard")
	sqlCfg, err := dto.GetSQLBackendCfg("{}")
	if err != nil {
		t.Fatalf("Failed to get SQL backend config: %v", err)
	}
	sqlEngine, engineErr := sqlengine.NewSQLEngine(
		sqlCfg,
		controlAttributes,
	)
	if engineErr != nil {
		t.Fatalf("Failed to create SQL engine: %v", engineErr)
	}
	persistenceSystem, err := persistence.NewSQLPersistenceSystem("naive", sqlEngine)
	if err != nil {
		t.Fatalf("Failed to create persistence system: %v", err)
	}
	if persistenceSystem == nil {
		t.Fatal("Failed to create persistence system: got nil")
	}
	setUpScript, scriptErr := sqlengine.GetSQLEngineSetupDDL("sqlite")
	if scriptErr != nil {
		t.Fatalf("Failed to get SQL engine setup DDL: %v", scriptErr)
	}
	scriptRunErr := sqlEngine.ExecInTxn([]string{setUpScript})
	if scriptRunErr != nil {
		t.Fatalf("Failed to run SQL engine setup DDL: %v", scriptRunErr)
	}
	putErr := persistenceSystem.CacheStorePut("key", []byte("value"), "", 3600)
	if putErr != nil {
		t.Fatalf("Failed to put cache: %v", putErr)
	}
	cachedVal, getErr := persistenceSystem.CacheStoreGet("key")
	if getErr != nil {
		t.Fatalf("Failed to get cache: %v", getErr)
	}
	if string(cachedVal) != "value" {
		t.Fatalf("Unexpected cached value: %v", string(cachedVal))
	}
	registry, registryErr := getNewTestDataMockRegistry(registryLocalPath)
	if registryErr != nil {
		t.Fatalf("Failed to create mock registry: %v", registryErr)
	}
	awsProvider, providersErr := registry.LoadProviderByName("aws", "v0.1.0")
	if providersErr != nil {
		t.Fatalf("Failed to list all available providers: %v", providersErr)
	}
	if awsProvider == nil {
		t.Fatal("Expected 'aws' provider to be available")
	}
	awsProviderPath := "testdata/registry/basic/src/aws/v0.1.0/provider.yaml"
	analysisCfg := discovery.NewAnalyzerCfg("openapi", awsProviderPath)
	rtCtx := dto.RuntimeCtx{}
	staticAnalyzer, analyzerErr := discovery.NewStaticAnalyzer(
		analysisCfg,
		persistenceSystem,
		registry,
		rtCtx,
	)
	if analyzerErr != nil {
		t.Fatalf("Failed to create static analyzer: %v", analyzerErr)
	}
	err = staticAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Static analysis failed: %v", err)
	}
}
