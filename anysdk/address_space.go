package anysdk

type AddressSpaceExpansionConfig interface {
	IsAsync() bool
	IsLegacy() bool
	IsAllowNilResponse() bool
}

type AddressSpace interface {
	GetGlobalSelectSchemas() map[string]Schema
	DereferenceAddress(address string) (any, bool)
	WriteToAddress(address string, val any) error
	ReadFromAddress(address string) (any, bool)
	ResolveSignature(map[string]any) (bool, map[string]any)
	Invoke(...any) error
	ToMap(AddressSpaceExpansionConfig) (map[string]any, error)
	ToRelation(AddressSpaceExpansionConfig) (Relation, error)
}
