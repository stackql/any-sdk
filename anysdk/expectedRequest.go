package anysdk

var (
	_ ExpectedRequest = &standardExpectedRequest{}
)

type ExpectedRequest interface {
	GetBodyMediaType() string
	GetSchema() Schema
	GetFinalSchema() Schema
	GetRequired() []string
	GetDefault() string
	GetBase() string
	GetXMLDeclaration() string
	GetXMLTransform() string
	//
	setSchema(Schema)
	setBodyMediaType(string)
	GetTransform() (Transform, bool)
	getOverrideSchema() (*LocalSchemaRef, bool)
	setOverrideSchemaValue(Schema)
}

type standardExpectedRequest struct {
	BodyMediaType     string `json:"mediaType,omitempty" yaml:"mediaType,omitempty"`
	Schema            Schema
	Default           string             `json:"default,omitempty" yaml:"default,omitempty"`
	Base              string             `json:"base,omitempty" yaml:"base,omitempty"`
	ProjectionMap     map[string]string  `json:"projection_map,omitempty" yaml:"projection_map,omitempty"`
	Required          []string           `json:"required,omitempty" yaml:"required,omitempty"`
	XMLDeclaration    string             `json:"xmlDeclaration,omitempty" yaml:"xmlDeclaration,omitempty"`
	XMLTransform      string             `json:"xmlTransform,omitempty" yaml:"xmlTransform,omitempty"`
	XMLRootAnnotation string             `json:"xmlRootAnnotation,omitempty" yaml:"xmlRootAnnotation,omitempty"`
	OverrideSchema    *LocalSchemaRef    `json:"schema_override,omitempty" yaml:"schema_override,omitempty"`
	Transform         *standardTransform `json:"transform,omitempty" yaml:"transform,omitempty"`
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

func (er *standardExpectedRequest) GetTransform() (Transform, bool) {
	return er.Transform, er.Transform != nil
}

func (er *standardExpectedRequest) getOverrideSchema() (*LocalSchemaRef, bool) {
	if er.OverrideSchema == nil {
		return nil, false
	}
	return er.OverrideSchema, true
}

func (er *standardExpectedRequest) setOverrideSchemaValue(s Schema) {
	if er.OverrideSchema == nil {
		er.OverrideSchema = &LocalSchemaRef{}
	}
	er.OverrideSchema.Value = s.(*standardSchema)
}

func (er *standardExpectedRequest) GetXMLDeclaration() string {
	return er.XMLDeclaration
}

func (er *standardExpectedRequest) GetXMLTransform() string {
	return er.XMLTransform
}

func (er *standardExpectedRequest) GetSchema() Schema {
	if er.OverrideSchema != nil && er.OverrideSchema.Value != nil {
		return er.OverrideSchema.Value
	}
	return er.Schema
}

func (er *standardExpectedRequest) GetFinalSchema() Schema {
	return er.Schema
}

func (er *standardExpectedRequest) GetDefault() string {
	return er.Default
}

func (er *standardExpectedRequest) GetBase() string {
	return er.Base
}

func (er *standardExpectedRequest) GetRequired() []string {
	return er.Required
}
