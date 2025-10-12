package docval_test

import (
	"testing"

	"github.com/stackql/any-sdk/pkg/docval"
)

var (
	validator docval.FileValidator = docval.NewFileValidator("testdata/schema-definitions")
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
	rv, err := validator.ValidateAndParseFile("testdata/docs/local_openssl/v0.1.0/provider.yaml", "provider.schema.json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rv["name"] != "local_openssl" {
		t.Fatalf("unexpected provider name: %v", rv["name"])
	}
}

func TestValidateAndParseGoogleProviderFile(t *testing.T) {
	rv, err := validator.ValidateAndParseFile("testdata/docs/googleapis.com/v0.1.2/provider.yaml", "provider.schema.json")
	if err == nil {
		t.Fatalf("expected an error, got none")
	}
	if rv != nil {
		t.Fatalf("unexpected non nil provider")
	}
}

func TestFragmentedResourcesFile(t *testing.T) {
	rv, err := validator.ValidateAndParseFile("testdata/docs/googleapis.com/v0.1.2/resources/compute-v1.yaml", "fragmented-resources.schema.json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rv["resources"] == nil {
		t.Fatalf("expected resources to be present")
	}
	resources, ok := rv["resources"].(map[string]any)
	if !ok || len(resources) == 0 {
		t.Fatalf("expected non-empty resources map, got %v", rv["resources"])
	}
}

func TestMonolithicCompositeServiceFile(t *testing.T) {
	rv, err := validator.ValidateAndParseFile("testdata/docs/googleapis.com/v0.1.2/services/bigquery-v2.yaml", "service-resources.schema.json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rv["components"] == nil {
		t.Fatalf("expected components to be present")
	}
}

func TestSplitCompositeServiceFile(t *testing.T) {
	rv, err := validator.ValidateAndParseFile("testdata/docs/googleapis.com/v0.1.2/services-split/compute/compute-disks-v1.yaml", "service-resources.schema.json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rv["components"] == nil {
		t.Fatalf("expected components to be present")
	}
}

func TestLocalTemplatedCompositeServiceFile(t *testing.T) {
	rv, err := validator.ValidateAndParseFile("testdata/docs/local_openssl/v0.1.0/services/keys.yaml", "local-templated.service-resources.schema.json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rv["components"] == nil {
		t.Fatalf("expected components to be present")
	}
}
