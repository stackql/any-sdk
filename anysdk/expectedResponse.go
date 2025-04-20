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
	getOverrideSchema() (*openapi3.SchemaRef, bool)
	setOverrideSchemaValue(*openapi3.Schema)
	GetTransform() (Transform, bool)
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
	OverrideSchema        *openapi3.SchemaRef `json:"schema_override,omitempty" yaml:"schema_override,omitempty"`
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

func (er *standardExpectedResponse) setOverrideSchemaValue(s *openapi3.Schema) {
	if er.OverrideSchema == nil {
		er.OverrideSchema = &openapi3.SchemaRef{}
	}
	er.OverrideSchema.Value = s
}

func (er *standardExpectedResponse) GetOpenAPIDocKey() string {
	return er.OpenAPIDocKey
}

func (er *standardExpectedResponse) GetObjectKey() string {
	return er.ObjectKey
}

func (er *standardExpectedResponse) GetSchema() Schema {
	if er.OverrideSchema != nil && er.OverrideSchema.Value != nil {
		return newSchema(er.OverrideSchema.Value, nil, "", "")
	}
	return er.Schema
}

func (er *standardExpectedResponse) getOverrideSchema() (*openapi3.SchemaRef, bool) {
	isNilSchema := er.OverrideSchema == nil
	if isNilSchema {
		return nil, false
	}
	overrideSchema := er.OverrideSchema
	return overrideSchema, true
}

func (er *standardExpectedResponse) GetTransform() (Transform, bool) {
	return er.Transform, er.Transform != nil
}
