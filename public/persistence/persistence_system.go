package persistence

import (
	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/constants"
	"github.com/stackql/any-sdk/pkg/name_mangle"
	"github.com/stackql/any-sdk/public/discovery"
	"github.com/stackql/any-sdk/public/sqlengine"
)

var (
	_ discovery.PersistenceSystem = &NaiveSQLPersistenceSystem{}
)

type NaiveSQLPersistenceSystem struct {
	sqlEngine       sqlengine.SQLEngine
	viewNameMangler name_mangle.NameMangler
}

func NewSQLPersistenceSystem(systemType string, sqlEngine sqlengine.SQLEngine) (discovery.PersistenceSystem, error) {
	switch systemType {
	case "naive":
		return newNaiveSQLPersistenceSystem(sqlEngine), nil
	default:
		return newNaiveSQLPersistenceSystem(sqlEngine), nil
	}
}

func newNaiveSQLPersistenceSystem(sqlEngine sqlengine.SQLEngine) *NaiveSQLPersistenceSystem {
	return &NaiveSQLPersistenceSystem{
		sqlEngine:       sqlEngine,
		viewNameMangler: name_mangle.NewViewNameMangler(),
	}
}

func (s *NaiveSQLPersistenceSystem) GetSystemName() string {
	return constants.SQLDialectSQLite3
}

func (s *NaiveSQLPersistenceSystem) HandleExternalTables(
	providerName string, externalTables map[string]anysdk.SQLExternalTable) error {
	for _, tbl := range externalTables {
		// TODO: add some validation
		var err error
		if tbl == nil {
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *NaiveSQLPersistenceSystem) HandleViewCollection(viewCollection []anysdk.View) error {
	for i, view := range viewCollection {
		viewNameNaive := view.GetNameNaive()
		viewName := s.viewNameMangler.MangleName(viewNameNaive, i)
		// TODO: add meaningful checking
		if viewName == "" {
			continue
		}
	}
	return nil
}

func (s *NaiveSQLPersistenceSystem) CacheStoreGet(key string) ([]byte, error) {
	return s.sqlEngine.CacheStoreGet(key)
}

func (s *NaiveSQLPersistenceSystem) CacheStorePut(key string, value []byte, expiration string, ttl int) error {
	return s.sqlEngine.CacheStorePut(key, value, expiration, ttl)
}
