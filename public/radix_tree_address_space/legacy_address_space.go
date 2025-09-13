package radix_tree_address_space

import (
	"strings"

	"github.com/stackql/any-sdk/anysdk"
)

type legacyTableSchemaAnalyzer interface {
	GetColumns() ([]anysdk.Column, error)
	GetColumnDescriptors(anysdk.Tabulation) ([]anysdk.ColumnDescriptor, error)
}

type simpleLegacyTableSchemaAnalyzer struct {
	s                    anysdk.Schema
	m                    anysdk.OperationStore
	isNilResponseAllowed bool
}

func newLegacyTableSchemaAnalyzer(
	s anysdk.Schema,
	m anysdk.OperationStore,
	isNilResponseAllowed bool,
) legacyTableSchemaAnalyzer {
	return &simpleLegacyTableSchemaAnalyzer{
		s:                    s,
		m:                    m,
		isNilResponseAllowed: isNilResponseAllowed,
	}
}

func TrimSelectItemsKey(selectItemsKey string) string {
	splitSet := strings.Split(selectItemsKey, "/")
	if len(splitSet) == 0 {
		return ""
	}
	return splitSet[len(splitSet)-1]
}

// TODO: operate on namespace
func (ta *simpleLegacyTableSchemaAnalyzer) GetColumns() ([]anysdk.Column, error) {
	var rv []anysdk.Column
	// This will be a function of method not schema
	// addressSpace, hasAddressSpace := ta.m.GetAddressSpace()
	// if !hasAddressSpace || addressSpace == nil {
	// 	return nil, fmt.Errorf("no address space found for method %s", ta.m.GetName())
	// }
	tableColumns := ta.s.Tabulate(false, "").GetColumns()
	existingColumns := make(map[string]struct{})
	for _, col := range tableColumns {
		existingColumns[col.GetName()] = struct{}{}
		rv = append(rv, newSimpleColumn(col.GetName(), col.GetSchema()))
	}
	unionedRequiredParams, err := ta.m.GetUnionRequiredParameters()
	if err != nil && !ta.isNilResponseAllowed {
		return nil, err
	}
	for k, col := range unionedRequiredParams {
		if _, ok := existingColumns[k]; ok {
			continue
		}
		schema, _ := col.GetSchema()
		existingColumns[col.GetName()] = struct{}{}
		rv = append(rv, newSimpleColumn(k, schema))
	}
	servers, serversDoExist := ta.m.GetServers()
	if serversDoExist {
		for _, srv := range servers {
			for k := range srv.Variables {
				if _, ok := existingColumns[k]; ok {
					continue
				}
				existingColumns[k] = struct{}{}
				rv = append(rv, newSimpleStringColumn(k, ta.m))
			}
		}
	}
	return rv, nil
}

func (ta *simpleLegacyTableSchemaAnalyzer) generateServerVarColumnDescriptor(
	k string, m anysdk.OperationStore) anysdk.ColumnDescriptor {
	schema := anysdk.NewStringSchema(
		m.GetService(),
		"",
		"",
	)
	colDesc := anysdk.NewColumnDescriptor(
		"",
		k,
		"",
		"",
		nil,
		schema,
		nil,
	)
	return colDesc
}

func (ta *simpleLegacyTableSchemaAnalyzer) GetColumnDescriptors(
	tabulation anysdk.Tabulation,
) ([]anysdk.ColumnDescriptor, error) {
	existingColumns := make(map[string]struct{})
	var rv []anysdk.ColumnDescriptor
	for _, col := range tabulation.GetColumns() {
		colName := col.GetName()
		existingColumns[colName] = struct{}{}
		rv = append(rv, col)
	}
	unionedRequiredParams, err := ta.m.GetUnionRequiredParameters()
	if err != nil {
		return nil, err
	}
	for k, col := range unionedRequiredParams {
		if _, ok := existingColumns[k]; ok {
			continue
		}
		existingColumns[k] = struct{}{}
		schema, _ := col.GetSchema()
		colDesc := anysdk.NewColumnDescriptor(
			"",
			k,
			"",
			"",
			nil,
			schema,
			nil,
		)
		rv = append(rv, colDesc)
	}
	servers, serversDoExist := ta.m.GetServers()
	if serversDoExist {
		for _, srv := range servers {
			for k := range srv.Variables {
				if _, ok := existingColumns[k]; ok {
					continue
				}
				existingColumns[k] = struct{}{}
				colDesc := ta.generateServerVarColumnDescriptor(k, ta.m)
				rv = append(rv, colDesc)
			}
		}
	}
	return rv, nil
}

type simpleColumn struct {
	name   string
	schema anysdk.Schema
}

func newSimpleColumn(name string, schema anysdk.Schema) anysdk.Column {
	return &simpleColumn{
		name:   name,
		schema: schema,
	}
}

func newSimpleStringColumn(name string, m anysdk.OperationStore) anysdk.Column {
	return newSimpleColumn(name, anysdk.NewStringSchema(
		m.GetService(),
		"",
		"",
	),
	)
}

func (sc simpleColumn) GetName() string {
	return sc.name
}

func (sc simpleColumn) GetWidth() int {
	return -1
}

func (sc simpleColumn) GetSchema() anysdk.Schema {
	return sc.schema
}
