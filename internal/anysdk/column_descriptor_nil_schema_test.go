package anysdk

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// A column descriptor for a literal carries no schema. GetSchema must return an
// untyped nil so that a caller's `!= nil` check is meaningful; the public facade
// relies on this to avoid wrapping nil into a non-nil interface.
func TestColumnDescriptorNilSchemaIsUntypedNil(t *testing.T) {
	cd := NewColumnDescriptor("", "literal", "", "literal", nil, nil, nil)
	if cd.GetSchema() != nil {
		t.Fatalf("expected nil Schema, got %#v", cd.GetSchema())
	}
}

func TestColumnDescriptorSchemaIsReturnedWhenPresent(t *testing.T) {
	schema := newSchema(openapi3.NewStringSchema(), nil, "some_schema", "")
	cd := NewColumnDescriptor("", "col", "", "col", nil, schema, nil)
	got := cd.GetSchema()
	if got == nil {
		t.Fatal("expected the schema to be returned, got nil")
	}
	if got.GetName() != "some_schema" {
		t.Fatalf("expected the same schema back, got name %q", got.GetName())
	}
}
