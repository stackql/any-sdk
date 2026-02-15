package formulation

import (
	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/stackql-parser/go/sqltypes"
	"github.com/stackql/stackql-parser/go/vt/sqlparser"
)

type ArmouryGenerator interface {
	GetHTTPArmoury() (anysdk.HTTPArmoury, error)
}

type Addressable interface {
	ConditionIsValid(lhs string, rhs interface{}) bool
	GetLocation() string
	GetName() string
	GetAlias() string
	GetSchema() (anysdk.Schema, bool)
	GetType() string
	IsRequired() bool
}

type ITable interface {
	GetName() string
	KeyExists(string) bool
	GetKey(string) (interface{}, error)
	GetKeyAsSqlVal(string) (sqltypes.Value, error)
	GetRequiredParameters() map[string]Addressable
	FilterBy(func(interface{}) (ITable, error)) (ITable, error)
}

type ColumnDescriptor interface {
	GetAlias() string
	GetDecoratedCol() string
	GetIdentifier() string
	GetName() string
	GetNode() sqlparser.SQLNode
	GetQualifier() string
	GetRepresentativeSchema() anysdk.Schema
	GetSchema() anysdk.Schema
	GetVal() *sqlparser.SQLVal
	setName(string)
}

type SQLExternalColumn interface {
	GetName() string
	GetType() string
	GetOid() uint32
	GetWidth() int
	GetPrecision() int
}

type SQLExternalTable interface {
	GetCatalogName() string
	GetSchemaName() string
	GetName() string
	GetColumns() []SQLExternalColumn
}
