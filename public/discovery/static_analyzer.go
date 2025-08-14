package discovery

import (
	"github.com/stackql/any-sdk/anysdk"
)

var (
	_ StaticAnalyzer    = &genericStaticAnalyzer{}
	_ PersistenceSystem = &aotPersistenceSystem{}
)

type AnalyzerCfg interface {
	GetAnalysisType() string
}

type standardAnalyzerCfg struct {
	analysisType string
}

func (sac *standardAnalyzerCfg) GetAnalysisType() string {
	return sac.analysisType
}

func NewStaticAnalyzer(analysiscfg AnalyzerCfg) (StaticAnalyzer, error) {
	switch analysiscfg.GetAnalysisType() {
	case "openapi":
		return &genericStaticAnalyzer{}, nil
	default:
		return &genericStaticAnalyzer{}, nil
	}
}

type StaticAnalyzer interface {
	Analyze() error
	GetErrors() []error
	GetWarnings() []string
}

type genericStaticAnalyzer struct {
	errors   []error
	warnings []string
}

func (osa *genericStaticAnalyzer) Analyze() error {
	// Implement OpenAPI specific analysis logic here
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
