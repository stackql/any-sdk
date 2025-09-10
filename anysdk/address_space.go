package anysdk

type AddressSpace interface {
	GetGlobalSelectSchemas() map[string]Schema
	DereferenceAddress(address string) (any, bool)
	WriteToAddress(address string, val any) error
	ReadFromAddress(address string) (any, bool)
	Analyze() error
	ResolveSignature(map[string]any) bool
	Expand(map[string]any) bool
	Invoke(...any) error
	ToMap() (map[string]any, error)
}
