package discovery

import (
	"fmt"

	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/persistence"
)

var (
	_ StaticAnalyzer                = &genericStaticAnalyzer{}
	_ persistence.PersistenceSystem = &aotPersistenceSystem{}
)

type AnalyzerCfg interface {
	GetProtocolType() string
	GetDocRoot() string
	GetProviderStr() string
	GetRootURL() string
}

type standardAnalyzerCfg struct {
	protocolType string
	docRoot      string
	providerStr  string
	rootURL      string
}

func NewAnalyzerCfg(
	protocolType string,
	docRoot string,
) AnalyzerCfg {
	return &standardAnalyzerCfg{
		protocolType: protocolType,
		docRoot:      docRoot,
	}
}

func (sac *standardAnalyzerCfg) GetProtocolType() string {
	return sac.protocolType
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

func newGenericStaticAnalyzer(
	analysisCfg AnalyzerCfg,
	persistenceSystem persistence.PersistenceSystem,
	discoveryStore IDiscoveryStore,
	discoveryAdapter IDiscoveryAdapter,
) StaticAnalyzer {
	return &genericStaticAnalyzer{
		cfg:               analysisCfg,
		persistenceSystem: persistenceSystem,
		discoveryStore:    discoveryStore,
		discoveryAdapter:  discoveryAdapter,
	}
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
		return newGenericStaticAnalyzer(analysisCfg, persistenceSystem, discoveryStore, discoveryAdapter), nil
	case "local_templated":
		return newGenericStaticAnalyzer(analysisCfg, persistenceSystem, discoveryStore, discoveryAdapter), nil
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
	if fileErr != nil {
		return fileErr
	}
	providerServices := provider.GetProviderServices()
	for k, providerService := range providerServices {
		if providerService == nil {
			osa.errors = append(osa.errors, fmt.Errorf("service %s is nil", k))
			continue
		}
		// Perform additional checks on the service
		rrr := providerService.GetResourcesRefRef()
		if rrr != "" {
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
			// return disDoc.GetResources(), nil
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
