package discovery

import (
	"fmt"

	"github.com/stackql/any-sdk/anysdk"
)

var (
	_ StaticAnalyzer    = &genericStaticAnalyzer{}
	_ PersistenceSystem = &aotPersistenceSystem{}
)

type AnalyzerCfg interface {
	GetProtocolType() string
	GetDocRoot() string
}

type standardAnalyzerCfg struct {
	protocolType string
	docRoot      string
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

func NewStaticAnalyzer(analysiscfg AnalyzerCfg) (StaticAnalyzer, error) {
	switch analysiscfg.GetProtocolType() {
	case "openapi":
		return &genericStaticAnalyzer{cfg: analysiscfg}, nil
	case "local_templated":
		return &genericStaticAnalyzer{cfg: analysiscfg}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol type: %s", analysiscfg.GetProtocolType())
	}
}

type StaticAnalyzer interface {
	Analyze() error
	GetErrors() []error
	GetWarnings() []string
}

type genericStaticAnalyzer struct {
	cfg      AnalyzerCfg
	errors   []error
	warnings []string
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
	for k, v := range provider.GetProviderServices() {
		if v == nil {
			osa.errors = append(osa.errors, fmt.Errorf("service %s is nil", k))
			continue
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
