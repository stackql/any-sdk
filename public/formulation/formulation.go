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

func NewColumnDescriptor(alias string, name string, qualifier string, decoratedCol string, node sqlparser.SQLNode, schema anysdk.Schema, val *sqlparser.SQLVal) ColumnDescriptor {
	rv := anysdk.NewColumnDescriptor(alias, name, qualifier, decoratedCol, node, schema, val)
	return rv.(ColumnDescriptor)
}

type SQLExternalColumn interface {
	GetName() string
	GetType() string
	GetOid() uint32
	GetWidth() int
	GetPrecision() int
}

func NewMethodAnalysisInput(
	method anysdk.OperationStore,
	service anysdk.Service,
	isNilResponseAllowed bool,
	columns []ColumnDescriptor,
	isAwait bool,
) anysdk.MethodAnalysisInput {
	cols := make([]anysdk.ColumnDescriptor, len(columns))
	for i, c := range columns {
		cols[i] = c.(anysdk.ColumnDescriptor)
	}
	return anysdk.NewMethodAnalysisInput(
		method,
		service,
		isNilResponseAllowed,
		cols,
		isAwait,
	)
}

type SQLExternalTable interface {
	GetCatalogName() string
	GetSchemaName() string
	GetName() string
	GetColumns() []SQLExternalColumn
}
