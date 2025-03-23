package anysdk

import "github.com/getkin/kin-openapi/openapi3"

var (
	_ ExpectedResponse = &standardExpectedResponse{}
)

type ExpectedResponse interface {
	GetBodyMediaType() string
	GetOpenAPIDocKey() string
	GetObjectKey() string
	GetSchema() Schema
	getOverrideSchemaRef() (*openapi3.SchemaRef, bool)
	//
	setSchema(Schema)
	setBodyMediaType(string)
}

type standardExpectedResponse struct {
	OverrideBodyMediaType string `json:"overrideMediaType,omitempty" yaml:"overrideMediaType,omitempty"`
	BodyMediaType         string `json:"mediaType,omitempty" yaml:"mediaType,omitempty"`
	OpenAPIDocKey         string `json:"openAPIDocKey,omitempty" yaml:"openAPIDocKey,omitempty"`
	ObjectKey             string `json:"objectKey,omitempty" yaml:"objectKey,omitempty"`
	Schema                Schema
	OverrideSchemaRef     *openapi3.SchemaRef `json:"overrideSchema,omitempty" yaml:"overrideSchema,omitempty"`
	Transform             *standardTransform  `json:"transform,omitempty" yaml:"transform,omitempty"`
}

func (er *standardExpectedResponse) setBodyMediaType(s string) {
	er.BodyMediaType = s
}

func (er *standardExpectedResponse) setSchema(s Schema) {
	er.Schema = s
}

func (er *standardExpectedResponse) GetBodyMediaType() string {
	return er.BodyMediaType
}

func (er *standardExpectedResponse) GetOpenAPIDocKey() string {
	return er.OpenAPIDocKey
}

func (er *standardExpectedResponse) GetObjectKey() string {
	return er.ObjectKey
}

func (er *standardExpectedResponse) GetSchema() Schema {
	return er.Schema
}

func (er *standardExpectedResponse) getOverrideSchemaRef() (*openapi3.SchemaRef, bool) {
	return er.OverrideSchemaRef, er.OverrideSchemaRef != nil
}

func (er *standardExpectedResponse) GetTransform() (Transform, bool) {
	return er.Transform, er.Transform != nil
}
