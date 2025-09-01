package discovery

import (
	"fmt"
	"strings"
	"sync"

	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/client"
	"github.com/stackql/any-sdk/pkg/db/sqlcontrol"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/persistence"
	"github.com/stackql/any-sdk/public/sqlengine"
	"github.com/stackql/stackql-provider-registry/signing/Ed25519/app/edcrypto"
)

var (
	_ StaticAnalyzer                  = &genericStaticAnalyzer{}
	_ StaticAnalyzer                  = &serviceLevelStaticAnalyzer{}
	_ ProviderServiceResourceAnalyzer = &standardProviderServiceResourceAnalyzer{}
)

type AnalyzerCfg interface {
	GetProtocolType() string
	GetDocRoot() string
	GetRegistryRootDir() string
	GetProviderStr() string
	GetRootURL() string
	IsProviderServicesMustExpand() bool
	IsVerbose() bool
	SetIsVerbose(bool)
	SetIsProviderServicesMustExpand(bool)
}

type standardAnalyzerCfg struct {
	protocolType               string
	docRoot                    string
	registryRootDir            string
	providerStr                string
	rootURL                    string
	providerServicesMustExpand bool
	isVerbose                  bool
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

func (sac *standardAnalyzerCfg) IsVerbose() bool {
	return sac.isVerbose
}

func (sac *standardAnalyzerCfg) SetIsVerbose(value bool) {
	sac.isVerbose = value
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
	CreateProviderServiceLevelStaticAnalyzer(
		providerURL string,
		serviceName string,
	) (ProviderServiceResourceAnalyzer, error)
	CreateServiceLevelStaticAnalyzer(
		providerURL string,
		serviceName string,
	) (StaticAnalyzer, error)
}

type simpleSQLiteAnalyzerFactory struct {
	registryURL string
	rtCtx       dto.RuntimeCtx
}

func NewSimpleSQLiteAnalyzerFactory(
	registryURL string,
	rtCtx dto.RuntimeCtx,
) StaticAnalyzerFactory {
	return &simpleSQLiteAnalyzerFactory{
		registryURL: registryURL,
		rtCtx:       rtCtx,
	}
}

func (f *simpleSQLiteAnalyzerFactory) CreateStaticAnalyzer(
	providerURL string,
) (StaticAnalyzer, error) {
	rtCtx := f.rtCtx
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
	analysisCfg := NewAnalyzerCfg("openapi", analyzerCfgPath, providerURL)
	analysisCfg.SetIsProviderServicesMustExpand(true)
	analysisCfg.SetIsVerbose(rtCtx.VerboseFlag)
	staticAnalyzer, analyzerErr := NewStaticAnalyzer(
		analysisCfg,
		persistenceSystem,
		registry,
		rtCtx,
	)
	return staticAnalyzer, analyzerErr
}

func (f *simpleSQLiteAnalyzerFactory) CreateProviderServiceLevelStaticAnalyzer(
	providerURL string,
	serviceName string,
) (ProviderServiceResourceAnalyzer, error) {
	rtCtx := f.rtCtx
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
	analysisCfg := NewAnalyzerCfg("openapi", analyzerCfgPath, providerURL)
	analysisCfg.SetIsProviderServicesMustExpand(true)
	analysisCfg.SetIsVerbose(rtCtx.VerboseFlag)
	discoveryStore := getDiscoveryStore(persistenceSystem, registry, rtCtx)
	discoveryAdapter := getDiscoveryAdapter(analysisCfg, persistenceSystem, discoveryStore, registry, rtCtx)
	provider, fileErr := anysdk.LoadProviderDocFromFile(analysisCfg.GetDocRoot())
	anysdk.OpenapiFileRoot = analysisCfg.GetRegistryRootDir()
	if fileErr != nil {
		return nil, fileErr
	}
	providerService, serviceErr := provider.GetProviderService(serviceName)
	if serviceErr != nil {
		return nil, serviceErr
	}
	psra := NewProviderServiceResourceAnalyzer(
		analysisCfg,
		discoveryAdapter,
		discoveryStore,
		provider,
		providerService,
		serviceName,
		registry,
	)
	return psra, nil
}

func (f *simpleSQLiteAnalyzerFactory) CreateServiceLevelStaticAnalyzer(
	providerURL string,
	serviceName string,
) (StaticAnalyzer, error) {
	rtCtx := f.rtCtx
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
	analysisCfg := NewAnalyzerCfg("openapi", analyzerCfgPath, providerURL)
	analysisCfg.SetIsProviderServicesMustExpand(true)
	analysisCfg.SetIsVerbose(rtCtx.VerboseFlag)
	discoveryStore := getDiscoveryStore(persistenceSystem, registry, rtCtx)
	discoveryAdapter := getDiscoveryAdapter(analysisCfg, persistenceSystem, discoveryStore, registry, rtCtx)
	provider, fileErr := anysdk.LoadProviderDocFromFile(analysisCfg.GetDocRoot())
	anysdk.OpenapiFileRoot = analysisCfg.GetRegistryRootDir()
	if fileErr != nil {
		return nil, fileErr
	}
	providerService, serviceErr := provider.GetProviderService(serviceName)
	if serviceErr != nil {
		return nil, serviceErr
	}
	staticAnalyzer := NewServiceLevelStaticAnalyzer(
		analysisCfg,
		discoveryAdapter,
		discoveryStore,
		provider,
		providerService,
		serviceName,
		registry,
	)
	return staticAnalyzer, nil
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
	GetAffirmatives() []string
	GetRegistryAPI() (anysdk.RegistryAPI, bool)
}

type genericStaticAnalyzer struct {
	cfg               AnalyzerCfg
	errors            []error
	warnings          []string
	affirmatives      []string
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
	protocolType, protocolTypeErr := provider.GetProtocolType()
	if protocolTypeErr != nil {
		return protocolTypeErr
	}
	switch protocolType {
	case client.HTTP, client.LocalTemplated:
		// acceptable
		osa.affirmatives = append(osa.affirmatives, fmt.Sprintf("successfully loaded provider %s with protocol type %s", provider.GetName(), provider.GetProtocolTypeString()))
	default:
		// unacceptable
		osa.errors = append(osa.errors, fmt.Errorf("unsupported protocol type for provider %s: %s", provider.GetName(), provider.GetProtocolTypeString()))
	}
	providerServices := provider.GetProviderServices()
	var wg sync.WaitGroup
	serviceAnalyzers := make(map[string]StaticAnalyzer, len(providerServices))
	for k, providerService := range providerServices {
		serviceLevelStaticAnalyzer := NewServiceLevelStaticAnalyzer(
			osa.cfg,
			osa.discoveryAdapter,
			osa.discoveryStore,
			provider,
			providerService,
			k,
			osa.registryAPI,
		)
		serviceAnalyzers[k] = serviceLevelStaticAnalyzer
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			serviceLevelStaticAnalyzer.Analyze()
		}(k)
	}
	wg.Wait()
	for k, serviceLevelStaticAnalyzer := range serviceAnalyzers {
		serviceErrors := serviceLevelStaticAnalyzer.GetErrors()
		if len(serviceErrors) > 0 {
			osa.errors = append(osa.errors, fmt.Errorf("static analysis found errors for service %s, error count %d", k, len(serviceErrors)))
			osa.errors = append(osa.errors, serviceErrors...)
		}
		osa.warnings = append(osa.warnings, serviceLevelStaticAnalyzer.GetWarnings()...)
		osa.affirmatives = append(osa.affirmatives, serviceLevelStaticAnalyzer.GetAffirmatives()...)
	}
	wg.Wait()
	if len(osa.errors) > 0 {
		return fmt.Errorf("static analysis found errors, error count %d", len(osa.errors))
	}
	// Perform analysis on providerDoc
	return nil
}

func (osa *genericStaticAnalyzer) GetRegistryAPI() (anysdk.RegistryAPI, bool) {
	if osa.registryAPI != nil {
		return osa.registryAPI, true
	}
	return nil, false
}

func (osa *genericStaticAnalyzer) GetErrors() []error {
	return osa.errors
}

func (osa *genericStaticAnalyzer) GetWarnings() []string {
	return osa.warnings
}

func (osa *genericStaticAnalyzer) GetAffirmatives() []string {
	return osa.affirmatives
}

func NewProviderServiceResourceAnalyzer(
	cfg AnalyzerCfg,
	discoveryAdapter IDiscoveryAdapter,
	discoveryStore IDiscoveryStore,
	provider anysdk.Provider,
	providerService anysdk.ProviderService,
	resourceKey string,
	registryAPI anysdk.RegistryAPI,
) ProviderServiceResourceAnalyzer {
	return &standardProviderServiceResourceAnalyzer{
		cfg:                      cfg,
		discoveryStore:           discoveryStore,
		discoveryAdapter:         discoveryAdapter,
		provider:                 provider,
		providerService:          providerService,
		serviceKey:               resourceKey,
		errors:                   []error{},
		warnings:                 []string{},
		affirmatives:             []string{},
		resources:                map[string]anysdk.Resource{},
		resourceServiceFragments: map[string]anysdk.Service{},
		registryAPI:              registryAPI,
	}
}

type ProviderServiceResourceAnalyzer interface {
	StaticAnalyzer
	GetResources() map[string]anysdk.Resource
	GetServiceFragments() map[string]anysdk.Service
}

type standardProviderServiceResourceAnalyzer struct {
	cfg                      AnalyzerCfg
	discoveryStore           IDiscoveryStore
	discoveryAdapter         IDiscoveryAdapter
	provider                 anysdk.Provider
	providerService          anysdk.ProviderService
	serviceKey               string
	errors                   []error
	warnings                 []string
	affirmatives             []string
	resources                map[string]anysdk.Resource
	resourceServiceFragments map[string]anysdk.Service
	registryAPI              anysdk.RegistryAPI
}

func (srf *standardProviderServiceResourceAnalyzer) GetRegistryAPI() (anysdk.RegistryAPI, bool) {
	if srf.registryAPI != nil {
		return srf.registryAPI, true
	}
	return nil, false
}

func (srf *standardProviderServiceResourceAnalyzer) GetErrors() []error {
	return srf.errors
}

func (srf *standardProviderServiceResourceAnalyzer) GetWarnings() []string {
	return srf.warnings
}

func (srf *standardProviderServiceResourceAnalyzer) GetAffirmatives() []string {
	return srf.affirmatives
}

func (srf *standardProviderServiceResourceAnalyzer) GetResources() map[string]anysdk.Resource {
	return srf.resources
}

func (srf *standardProviderServiceResourceAnalyzer) GetServiceFragments() map[string]anysdk.Service {
	return srf.resourceServiceFragments
}

func (srf *standardProviderServiceResourceAnalyzer) Analyze() error {
	key := srf.serviceKey
	providerService := srf.providerService
	rrr := providerService.GetResourcesRefRef()
	resources := make(map[string]anysdk.Resource)
	if rrr != "" {
		// Should be sole place for ResourcesRef dereference
		disDoc, docErr := srf.discoveryStore.processResourcesDiscoveryDoc(
			srf.provider,
			providerService,
			fmt.Sprintf("%s.%s", srf.discoveryAdapter.getAlias(), key))
		if docErr != nil {
			srf.errors = append(srf.errors, docErr)
		}
		if disDoc == nil {
			err := fmt.Errorf("discovery document is nil for service %s", key)
			srf.errors = append(srf.errors, err)
			return err
		}
		shallowResources := disDoc.GetResources()
		for resKey := range shallowResources {
			_, foundRsc := resources[resKey]
			if foundRsc {
				continue
			}
			svcFrag, svcFragErr := srf.registryAPI.GetServiceFragment(providerService, resKey)
			if svcFragErr != nil {
				srf.errors = append(srf.errors, fmt.Errorf("failed to get service fragment for svc name = %s: %v", key, svcFragErr))
				continue
			} else if svcFrag == nil {
				srf.errors = append(srf.errors, fmt.Errorf("service fragment is nil for svc name = %s", key))
				continue
			}
			deepResources, deepResourcesErr := svcFrag.GetResources()
			if deepResourcesErr != nil {
				srf.errors = append(srf.errors, fmt.Errorf("failed to get resources for svc name = %s: %v", key, deepResourcesErr))
				continue
			}
			for resourceKey, resVal := range deepResources {
				resVal.SetProvider(srf.provider)
				resVal.SetProviderService(providerService)
				resources[resourceKey] = resVal
				srf.resourceServiceFragments[resourceKey] = svcFrag
			}
		}
	} else {
		// Dereferences ServiceRef, not sole location
		svc, err := providerService.GetService()
		if err != nil {
			if !srf.cfg.IsProviderServicesMustExpand() {
				return nil
			}
			err := fmt.Errorf("failed to get service handle for %s: %v", key, err)
			srf.errors = append(srf.errors, err)
			return err
		}
		resources, err = svc.GetResources()
		if err != nil {
			err := fmt.Errorf("failed to get resources for service %s: %v", key, err)
			srf.errors = append(srf.errors, err)
			return err
		}
	}
	srf.resources = resources
	return nil
}

func NewServiceLevelStaticAnalyzer(
	cfg AnalyzerCfg,
	discoveryAdapter IDiscoveryAdapter,
	discoveryStore IDiscoveryStore,
	provider anysdk.Provider,
	providerService anysdk.ProviderService,
	providerServiceKey string,
	registryAPI anysdk.RegistryAPI,
) StaticAnalyzer {
	return &serviceLevelStaticAnalyzer{
		cfg:                cfg,
		errors:             []error{},
		warnings:           []string{},
		affirmatives:       []string{},
		discoveryAdapter:   discoveryAdapter,
		discoveryStore:     discoveryStore,
		provider:           provider,
		providerService:    providerService,
		providerServiceKey: providerServiceKey,
		registryAPI:        registryAPI,
	}
}

type serviceLevelStaticAnalyzer struct {
	cfg                AnalyzerCfg
	errors             []error
	warnings           []string
	affirmatives       []string
	discoveryAdapter   IDiscoveryAdapter
	discoveryStore     IDiscoveryStore
	provider           anysdk.Provider
	providerService    anysdk.ProviderService
	providerServiceKey string
	registryAPI        anysdk.RegistryAPI
}

func (osa *serviceLevelStaticAnalyzer) GetRegistryAPI() (anysdk.RegistryAPI, bool) {
	if osa.registryAPI != nil {
		return osa.registryAPI, true
	}
	return nil, false
}

func (osa *serviceLevelStaticAnalyzer) Analyze() error {
	anysdk.OpenapiFileRoot = osa.cfg.GetRegistryRootDir()
	protocolType, protocolTypeErr := osa.provider.GetProtocolType()
	if protocolTypeErr != nil {
		return protocolTypeErr
	}
	switch protocolType {
	case client.HTTP, client.LocalTemplated:
		// acceptable
		osa.affirmatives = append(osa.affirmatives, fmt.Sprintf("successfully loaded provider %s with protocol type %s", osa.provider.GetName(), osa.provider.GetProtocolTypeString()))
	default:
		// unacceptable
		osa.errors = append(osa.errors, fmt.Errorf("unsupported protocol type for provider %s: %s", osa.provider.GetName(), osa.provider.GetProtocolTypeString()))
	}
	providerService := osa.providerService
	k := osa.providerServiceKey
	var resources map[string]anysdk.Resource
	if providerService == nil {
		err := fmt.Errorf("service %s is nil", k)
		osa.errors = append(osa.errors, err)
		return err
	}
	providerServiceResourceAnalyzer := NewProviderServiceResourceAnalyzer(
		osa.cfg,
		osa.discoveryAdapter,
		osa.discoveryStore,
		osa.provider,
		providerService,
		k,
		osa.registryAPI,
	)
	psraErr := providerServiceResourceAnalyzer.Analyze()
	osa.affirmatives = append(osa.affirmatives, providerServiceResourceAnalyzer.GetAffirmatives()...)
	osa.warnings = append(osa.warnings, providerServiceResourceAnalyzer.GetWarnings()...)
	osa.errors = append(osa.errors, providerServiceResourceAnalyzer.GetErrors()...)
	if psraErr != nil {
		return psraErr
	}
	resources = providerServiceResourceAnalyzer.GetResources()
	// Perform additional checks on the service
	if len(resources) == 0 {
		if !osa.cfg.IsProviderServicesMustExpand() {
			return nil
		}
		err := fmt.Errorf("no resources found for provider %s", k)
		osa.errors = append(osa.errors, err)
		return err
	}
	for resourceKey, resource := range resources {
		// Loader.mergeResource() dereferences interesting stuff including:
		//   - operation store attributes dereference:
		//        -  OperationRef
		//        -  PathItemRef
		//   - expected response attributes:
		//        -  LocalSchemaRef x 2 for sync and async schema overrides
		//   - OpenAPIOperationStoreRef via resolveSQLVerb()
		methods := resource.GetMethods()
		for methodName, method := range methods {
			// Perform analysis on each method
			if !osa.cfg.IsProviderServicesMustExpand() {
				continue
			}

			switch protocolType {
			case client.HTTP:
				graphQL := method.GetGraphQL()
				isGraphQL := graphQL != nil
				if isGraphQL {
					continue // TODO: GraphQL methods analysis
				}
				// Does this method have selection semantics?
				sqlVerb := strings.ToLower(method.GetSQLVerb())
				isSelectMethod := sqlVerb == "select"
				selectItemsKey := method.GetSelectItemsKey()
				hasSelectionSemantics := selectItemsKey != ""
				if !hasSelectionSemantics && isSelectMethod {
					osa.warnings = append(osa.warnings, fmt.Sprintf("apparent select method %s for resource %s does not have selection semantics", methodName, resourceKey))
				}
				if sqlVerb == "" {
					osa.warnings = append(osa.warnings, fmt.Sprintf("method %s for resource %s has no SQL verb", methodName, resourceKey))
				}
				shouldBeSelectable := method.ShouldBeSelectable()
				if shouldBeSelectable {
					responseSchema, mediaType, responseInferenceErr := method.GetFinalResponseBodySchemaAndMediaType()
					if responseInferenceErr != nil {
						osa.errors = append(osa.errors, fmt.Errorf("failed to infer response schema for method = '%s': %v", methodName, responseInferenceErr))
					}
					if responseSchema == nil {
						osa.errors = append(osa.errors, fmt.Errorf("response schema not found for method = '%s' with media type %s", methodName, mediaType))
						continue
					}
					selectableSchema, objPath, selectionErr := method.GetSelectSchemaAndObjectPath()
					if selectionErr != nil {
						osa.errors = append(osa.errors, fmt.Errorf("failed to infer selectable schema for method = '%s': %v", methodName, selectionErr))
						continue
					}
					if selectableSchema == nil {
						osa.errors = append(osa.errors, fmt.Errorf("selectable schema not found for method = '%s'", methodName))
					}
					osa.affirmatives = append(osa.affirmatives, fmt.Sprintf("successfully inferred response schema for method = '%s' with media type %s  at object path = %s", methodName, mediaType, objPath))
				}
				osa.affirmatives = append(osa.affirmatives, fmt.Sprintf("successfully dereferenced method = '%s' for resource = '%s' with service name = '%s'", methodName, resourceKey, k))
			case client.LocalTemplated:
				// Local templated protocol specific analysis
				inline := method.GetInline()
				if len(inline) != 0 {
					osa.affirmatives = append(osa.affirmatives, fmt.Sprintf("successfully found inline for local templated method = '%s'", methodName))
				} else {
					osa.errors = append(osa.errors, fmt.Errorf("inline not found for local templated method = '%s'", methodName))
				}
			default:
				// placeholder for fine grained protocol type analysis
			}
		}
		osa.affirmatives = append(osa.affirmatives, fmt.Sprintf("successfully dereferenced resource = '%s' with attendant service fragment for svc name = '%s'", resourceKey, k))
	}

	if len(osa.errors) > 0 {
		return fmt.Errorf("static analysis found errors, error count %d", len(osa.errors))
	}
	return nil
}

func (osa *serviceLevelStaticAnalyzer) GetErrors() []error {
	return osa.errors
}

func (osa *serviceLevelStaticAnalyzer) GetWarnings() []string {
	return osa.warnings
}

func (osa *serviceLevelStaticAnalyzer) GetAffirmatives() []string {
	return osa.affirmatives
}
