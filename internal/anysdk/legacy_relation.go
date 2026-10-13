package anysdk

import "github.com/stackql/any-sdk/pkg/casing"

func NewParameterColumnDescriptor(provider Provider, displayName string, wireName string, schema Schema) ColumnDescriptor {
	if displayName == wireName && provider != nil && provider.IsSnakeCaseAliasesEnabled() {
		displayName = casing.ToSnake(wireName)
	}
	return newColumnDescriptorWithWireName("", displayName, wireName, "", "", nil, schema, nil)
}
