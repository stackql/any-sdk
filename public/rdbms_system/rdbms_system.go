package rdbms_system

import (
	"github.com/stackql/any-sdk/anysdk"
)

type RDBMSSystem interface {
	GetSystemName() string
	RegisterExternalTable(connectionName string, tableDetails anysdk.SQLExternalTable) error
	HandleViewCollection(viewCollection []anysdk.View) error
	CacheStoreGet(key string) ([]byte, error)
	CacheStorePut(key string, value []byte, expiration string, ttl int) error
}
