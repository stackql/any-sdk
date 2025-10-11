package docval_test

import (
	"testing"

	"github.com/stackql/any-sdk/pkg/docval"
)

func TestValidateAndParse_ValidJSON(t *testing.T) {
	jsonDoc := []byte(`{"name": "Alice", "age": 30}`)
	jsonSchema := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		},
		"required": ["name", "age"]
	}`)

	result, err := docval.ValidateAndParse(jsonDoc, jsonSchema, "test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result["name"] != "Alice" || result["age"] != 30 { // JSON numbers are float64
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestLocalValidateAndParseValidProviderFile(t *testing.T) {
	rv, err := docval.ValidateAndParseFile("testdata/docs/local_openssl/v0.1.0/provider.yaml", "testdata/schema-definitions/provider.schema.json", "provider")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rv["name"] != "local_openssl" {
		t.Fatalf("unexpected provider name: %v", rv["name"])
	}
}

func TestValidateAndParseGoogleProviderFile(t *testing.T) {
	rv, err := docval.ValidateAndParseFile("testdata/docs/googleapis.com/v0.1.2/provider.yaml", "testdata/schema-definitions/provider.schema.json", "provider")
	if err == nil {
		t.Fatalf("expected an error, got none")
	}
	if rv != nil {
		t.Fatalf("unexpected non nil provider")
	}
}
