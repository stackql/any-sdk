package anysdk

type Column interface {
	GetName() string
	GetSchema() Schema
	GetWidth() int
}
