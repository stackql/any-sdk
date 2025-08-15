package persistence_test

import (
	"testing"

	"github.com/stackql/any-sdk/pkg/db/sqlcontrol"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/persistence"
	"github.com/stackql/any-sdk/public/sqlengine"
)

func TestPersistenceSetup(t *testing.T) {
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
}

func TestPersistence01(t *testing.T) {
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
}
