package anysdk

var (
	_ ExpectedRequest = &standardExpectedRequest{}
)

type ExpectedRequest interface {
	GetBodyMediaType() string
	GetSchema() Schema
	GetRequired() []string
	GetDefault() interface{}
	//
	setSchema(Schema)
	setBodyMediaType(string)
}

type standardExpectedRequest struct {
	BodyMediaType string `json:"mediaType,omitempty" yaml:"mediaType,omitempty"`
	Schema        Schema
	Default       interface{} `json:"default,omitempty" yaml:"default,omitempty"`
	Required      []string    `json:"required,omitempty" yaml:"required,omitempty"`
}

func (er *standardExpectedRequest) setBodyMediaType(s string) {
	er.BodyMediaType = s
}

func (er *standardExpectedRequest) setSchema(s Schema) {
	er.Schema = s
}

func (er *standardExpectedRequest) GetBodyMediaType() string {
	return er.BodyMediaType
}

func (er *standardExpectedRequest) GetSchema() Schema {
	return er.Schema
}

func (er *standardExpectedRequest) GetDefault() interface{} {
	return er.Default
}

func (er *standardExpectedRequest) GetRequired() []string {
	return er.Required
}
