package persistence_test

import (
	"testing"

	"github.com/stackql/any-sdk/pkg/db/sqlcontrol"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/persistence"
	"github.com/stackql/any-sdk/public/sqlengine"
)

func TestPersistence(t *testing.T) {
	// Test case for the persistence layer
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

	// Add more test cases as needed
}
