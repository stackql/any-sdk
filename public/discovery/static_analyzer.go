package discovery

import (
	"fmt"
	"strings"

	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/db/sqlcontrol"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/persistence"
	"github.com/stackql/any-sdk/public/sqlengine"
	"github.com/stackql/stackql-provider-registry/signing/Ed25519/app/edcrypto"
)

var (
	_ StaticAnalyzer                = &genericStaticAnalyzer{}
	_ persistence.PersistenceSystem = &aotPersistenceSystem{}
)

type AnalyzerCfg interface {
	GetProtocolType() string
	GetDocRoot() string
	GetRegistryRootDir() string
	GetProviderStr() string
	GetRootURL() string
	IsProviderServicesMustExpand() bool
	SetIsProviderServicesMustExpand(bool)
}

type standardAnalyzerCfg struct {
	protocolType               string
	docRoot                    string
	registryRootDir            string
	providerStr                string
	rootURL                    string
	providerServicesMustExpand bool
}

func NewAnalyzerCfg(
	protocolType string,
	registryRootDir string,
	docRoot string,
) AnalyzerCfg {
	return &standardAnalyzerCfg{
		protocolType:               protocolType,
		registryRootDir:            registryRootDir,
		docRoot:                    docRoot,
		providerServicesMustExpand: true, // default thorough analysis
	}
}

func (sac *standardAnalyzerCfg) GetProtocolType() string {
	return sac.protocolType
}

func (sac *standardAnalyzerCfg) GetRegistryRootDir() string {
	return sac.registryRootDir
}

func (sac *standardAnalyzerCfg) GetDocRoot() string {
	return sac.docRoot
}

func (sac *standardAnalyzerCfg) GetProviderStr() string {
	return sac.providerStr
}

func (sac *standardAnalyzerCfg) GetRootURL() string {
	return sac.rootURL
}

func (sac *standardAnalyzerCfg) IsProviderServicesMustExpand() bool {
	return sac.providerServicesMustExpand
}

func (sac *standardAnalyzerCfg) SetIsProviderServicesMustExpand(value bool) {
	sac.providerServicesMustExpand = value
}

func newGenericStaticAnalyzer(
	analysisCfg AnalyzerCfg,
	persistenceSystem persistence.PersistenceSystem,
	discoveryStore IDiscoveryStore,
	discoveryAdapter IDiscoveryAdapter,
	registryAPI anysdk.RegistryAPI,
) StaticAnalyzer {
	return &genericStaticAnalyzer{
		cfg:               analysisCfg,
		persistenceSystem: persistenceSystem,
		discoveryStore:    discoveryStore,
		discoveryAdapter:  discoveryAdapter,
		registryAPI:       registryAPI,
	}
}

func getNewMockRegistry(relativePath string) (anysdk.RegistryAPI, error) {
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

type StaticAnalyzerFactory interface {
	CreateStaticAnalyzer(
		providerURL string,
	) (StaticAnalyzer, error)
}

type simpleSQLiteAnalyzerFactory struct {
	registryURL string
}

func NewSimpleSQLiteAnalyzerFactory(
	registryURL string,
) StaticAnalyzerFactory {
	return &simpleSQLiteAnalyzerFactory{
		registryURL: registryURL,
	}
}

func (f *simpleSQLiteAnalyzerFactory) CreateStaticAnalyzer(
	providerURL string,
) (StaticAnalyzer, error) {
	registryLocalPath := f.registryURL
	analyzerCfgPath := strings.TrimPrefix(registryLocalPath, "./") + "/src"
	controlAttributes := sqlcontrol.GetControlAttributes("standard")
	sqlCfg, err := dto.GetSQLBackendCfg("{}")
	if err != nil {
		return nil, err
	}
	sqlEngine, engineErr := sqlengine.NewSQLEngine(
		sqlCfg,
		controlAttributes,
	)
	if engineErr != nil {
		return nil, engineErr
	}
	persistenceSystem, err := persistence.NewSQLPersistenceSystem("naive", sqlEngine)
	if err != nil {
		return nil, err
	}
	if persistenceSystem == nil {
		return nil, fmt.Errorf("failed to create persistence system: got nil")
	}
	setUpScript, scriptErr := sqlengine.GetSQLEngineSetupDDL("sqlite")
	if scriptErr != nil {
		return nil, scriptErr
	}
	scriptRunErr := sqlEngine.ExecInTxn([]string{setUpScript})
	if scriptRunErr != nil {
		return nil, scriptRunErr
	}
	putErr := persistenceSystem.CacheStorePut("key", []byte("value"), "", 3600)
	if putErr != nil {
		return nil, putErr
	}
	cachedVal, getErr := persistenceSystem.CacheStoreGet("key")
	if getErr != nil {
		return nil, getErr
	}
	if string(cachedVal) != "value" {
		return nil, fmt.Errorf("unexpected cached value: %v", string(cachedVal))
	}
	registry, registryErr := getNewMockRegistry(registryLocalPath)
	if registryErr != nil {
		return nil, registryErr
	}
	googleProvider, providersErr := registry.LoadProviderByName("google", "v0.1.2")
	if providersErr != nil {
		return nil, providersErr
	}
	if googleProvider == nil {
		return nil, fmt.Errorf("expected 'google' provider to be available")
	}
	analysisCfg := NewAnalyzerCfg("openapi", analyzerCfgPath, providerURL)
	analysisCfg.SetIsProviderServicesMustExpand(true)
	rtCtx := dto.RuntimeCtx{}
	staticAnalyzer, analyzerErr := NewStaticAnalyzer(
		analysisCfg,
		persistenceSystem,
		registry,
		rtCtx,
	)
	return staticAnalyzer, analyzerErr
}

func NewStaticAnalyzer(
	analysisCfg AnalyzerCfg,
	persistenceSystem persistence.PersistenceSystem,
	registryAPI anysdk.RegistryAPI,
	rtCtx dto.RuntimeCtx,
) (StaticAnalyzer, error) {
	discoveryStore := getDiscoveryStore(persistenceSystem, registryAPI, rtCtx)
	discoveryAdapter := getDiscoveryAdapter(analysisCfg, persistenceSystem, discoveryStore, registryAPI, rtCtx)
	switch analysisCfg.GetProtocolType() {
	case "openapi":
		return newGenericStaticAnalyzer(analysisCfg, persistenceSystem, discoveryStore, discoveryAdapter, registryAPI), nil
	case "local_templated":
		return newGenericStaticAnalyzer(analysisCfg, persistenceSystem, discoveryStore, discoveryAdapter, registryAPI), nil
	default:
		return nil, fmt.Errorf("unsupported protocol type: %s", analysisCfg.GetProtocolType())
	}
}

func getDiscoveryStore(persistor persistence.PersistenceSystem, registryAPI anysdk.RegistryAPI, rtCtx dto.RuntimeCtx) IDiscoveryStore {
	return NewTTLDiscoveryStore(
		persistor,
		registryAPI,
		rtCtx,
	)
}

func getDiscoveryAdapter(cfg AnalyzerCfg, persistor persistence.PersistenceSystem, discoveryStore IDiscoveryStore, registryAPI anysdk.RegistryAPI, rtCtx dto.RuntimeCtx) IDiscoveryAdapter {
	da := NewBasicDiscoveryAdapter(
		cfg.GetProviderStr(),
		cfg.GetRootURL(),
		discoveryStore,
		&rtCtx,
		registryAPI,
		persistor,
	)
	return da
}

type StaticAnalyzer interface {
	Analyze() error
	GetErrors() []error
	GetWarnings() []string
}

type genericStaticAnalyzer struct {
	cfg               AnalyzerCfg
	errors            []error
	warnings          []string
	persistenceSystem persistence.PersistenceSystem
	discoveryAdapter  IDiscoveryAdapter
	discoveryStore    IDiscoveryStore
	registryAPI       anysdk.RegistryAPI
}

// For each operation store in each resource:
// For each provider:
//   - Each service reference should dereference to a non nil object and wothout error.
//   - If resources.ref is present then all resources routable through this should behave
//   - ELSE if services.ref then all services routable through this should behave
//   - GetSelectSchemaAndObjectPath() should return a non nil schema and nil error
func (osa *genericStaticAnalyzer) Analyze() error {
	// Implement OpenAPI specific analysis logic here
	provider, fileErr := anysdk.LoadProviderDocFromFile(osa.cfg.GetDocRoot())
	anysdk.OpenapiFileRoot = osa.cfg.GetRegistryRootDir()
	if fileErr != nil {
		return fileErr
	}
	providerServices := provider.GetProviderServices()
	for k, providerService := range providerServices {
		var resources map[string]anysdk.Resource
		if providerService == nil {
			osa.errors = append(osa.errors, fmt.Errorf("service %s is nil", k))
			continue
		}
		// Perform additional checks on the service
		rrr := providerService.GetResourcesRefRef()
		if rrr != "" {
			// Should be sole place for ResourcesRef dereference
			disDoc, docErr := osa.discoveryStore.processResourcesDiscoveryDoc(
				provider,
				providerService,
				fmt.Sprintf("%s.%s", osa.discoveryAdapter.getAlias(), k))
			if docErr != nil {
				osa.errors = append(osa.errors, docErr)
			}
			if disDoc == nil {
				osa.errors = append(osa.errors, fmt.Errorf("discovery document is nil for service %s", k))
				continue
			}
			resources = disDoc.GetResources()
		} else {
			// Dereferences ServiceRef, not sole location
			svc, err := providerService.GetService()
			if err != nil {
				if !osa.cfg.IsProviderServicesMustExpand() {
					continue
				}
				osa.errors = append(osa.errors, fmt.Errorf("failed to get service handle for %s: %v", k, err))
				continue
			}
			resources, err = svc.GetResources()
			if err != nil {
				osa.errors = append(osa.errors, fmt.Errorf("failed to get resources for service %s: %v", k, err))
				continue
			}
		}
		if len(resources) == 0 {
			osa.errors = append(osa.errors, fmt.Errorf("no resources found for provider %s", k))
			continue
		}
		for resourceKey := range resources {
			// Loader.mergeResource() dereferences interesting stuff including:
			//   - operation store attributes dereference:
			//        -  OperationRef
			//        -  PathItemRef
			//   - expected response attributes:
			//        -  LocalSchemaRef x 2 for sync and async schema overrides
			//   - OpenAPIOperationStoreRef via resolveSQLVerb()
			svc, svcErr := osa.registryAPI.GetServiceFragment(providerService, resourceKey)
			if svcErr != nil {
				osa.errors = append(osa.errors, fmt.Errorf("failed to get service fragment for svc name = %s: %v", svc.GetName(), svcErr))
				continue
			}
		}
	}
	if len(osa.errors) > 0 {
		return fmt.Errorf("static analysis found errors, error count %d", len(osa.errors))
	}
	// Perform analysis on providerDoc
	return nil
}

func (osa *genericStaticAnalyzer) GetErrors() []error {
	return osa.errors
}

func (osa *genericStaticAnalyzer) GetWarnings() []string {
	return osa.warnings
}

type aotPersistenceSystem struct {
	systemName string
}

func (aps *aotPersistenceSystem) GetSystemName() string {
	return aps.systemName
}

func (aps *aotPersistenceSystem) HandleExternalTables(providerName string, externalTables map[string]anysdk.SQLExternalTable) error {
	// Implement logic to handle external tables
	return nil
}

func (aps *aotPersistenceSystem) HandleViewCollection(viewCollection []anysdk.View) error {
	// Implement logic to handle view collection
	return nil
}

func (aps *aotPersistenceSystem) CacheStoreGet(key string) ([]byte, error) {
	// Implement logic to get data from cache store
	return nil, nil
}

func (aps *aotPersistenceSystem) CacheStorePut(key string, value []byte, expiration string, ttl int) error {
	// Implement logic to put data into cache store
	return nil
}
