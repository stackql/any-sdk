package sqlengine

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/stackql/any-sdk/pkg/constants"
	"github.com/stackql/any-sdk/pkg/db/sqlcontrol"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/pkg/internaldto"
)

func getFilePathFromRepositoryRoot(relativePath string) (string, error) {
	_, filename, _, _ := runtime.Caller(0)
	curDir := filepath.Dir(filename)
	rv, err := filepath.Abs(filepath.Join(curDir, "../../..", relativePath))
	return strings.ReplaceAll(rv, `\`, `\\`), err
}

type SQLEngine interface {
	GetDB() (*sql.DB, error)
	GetTx() (*sql.Tx, error)
	Exec(string, ...interface{}) (sql.Result, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	// ExecFileLocal(string) error
	// ExecFile(string) error
	ExecInTxn(queries []string) error
	GetCurrentGenerationID() (int, error)
	GetNextGenerationID() (int, error)
	GetCurrentSessionID(int) (int, error)
	GetNextSessionID(int) (int, error)
	GetCurrentDiscoveryGenerationID(discoveryID string) (int, error)
	GetNextDiscoveryGenerationID(discoveryID string) (int, error)
	CacheStoreGet(string) ([]byte, error)
	CacheStoreGetAll() ([]internaldto.KeyVal, error)
	CacheStorePut(string, []byte, string, int) error
	IsMemory() bool
}

func NewSQLEngine(cfg dto.SQLBackendCfg, controlAttributes sqlcontrol.ControlAttributes) (SQLEngine, error) {
	switch cfg.DBEngine {
	case constants.DBEngineSQLite3Embedded:
		return newSQLiteEmbeddedEngine(cfg, controlAttributes)
	case constants.DBEnginePostgresTCP:
		return newPostgresTCPEngine(cfg, controlAttributes)
	case constants.SQLDialectSnowflake:
		return newSnowflakeTCPEngine(cfg, controlAttributes)
	default:
		return nil, fmt.Errorf(`SQL backend DB Engine of type '%s' is not permitted`, cfg.DBEngine)
	}
}
