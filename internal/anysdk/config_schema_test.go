package anysdk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stackql/any-sdk/pkg/docval"
)

// configSchemaPath is the canonical (published) config schema artefact.
var configSchemaPath = filepath.Join("..", "..", "cicd", "schema-definitions", "stackql-config.schema.json")

// schemaSurface is a minimal view of the bits of a JSON Schema object we assert on.
type schemaSurface struct {
	AdditionalProperties *bool                      `json:"additionalProperties"`
	Properties           map[string]json.RawMessage `json:"properties"`
}

func readConfigSchema(t *testing.T) schemaSurface {
	t.Helper()
	b, err := os.ReadFile(configSchemaPath)
	if err != nil {
		t.Fatalf("read config schema %q: %v", configSchemaPath, err)
	}
	var s schemaSurface
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse config schema: %v", err)
	}
	return s
}

// configStructYamlTags returns the yaml tag names of standardStackQLConfig that
// are part of the document surface (excluding `-`). This is the source of truth
// the schema must mirror.
func configStructYamlTags() map[string]struct{} {
	rv := map[string]struct{}{}
	tp := reflect.TypeOf(standardStackQLConfig{})
	for i := 0; i < tp.NumField(); i++ {
		name := strings.Split(tp.Field(i).Tag.Get("yaml"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		rv[name] = struct{}{}
	}
	return rv
}

func sortedKeys(m map[string]struct{}) []string {
	rv := make([]string, 0, len(m))
	for k := range m {
		rv = append(rv, k)
	}
	sort.Strings(rv)
	return rv
}

// TestStackQLConfigSchemaMatchesStruct is the struct-schema agreement guard
// (issue #110): every config key the loader understands must be modelled in the
// published schema and vice versa, so the artefact cannot silently drift from the
// code. It also asserts the config object rejects unknown keys.
func TestStackQLConfigSchemaMatchesStruct(t *testing.T) {
	schema := readConfigSchema(t)

	if schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		t.Errorf("config schema must set additionalProperties:false so mistyped keys fail validation")
	}

	want := configStructYamlTags()
	got := map[string]struct{}{}
	for k := range schema.Properties {
		got[k] = struct{}{}
	}

	for k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("config key %q exists on standardStackQLConfig but is missing from the schema", k)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("schema models config key %q that no longer exists on standardStackQLConfig", k)
		}
	}
	if t.Failed() {
		t.Logf("struct yaml tags: %v", sortedKeys(want))
		t.Logf("schema properties: %v", sortedKeys(got))
	}
}

// TestStackQLConfigSchemaAcceptsKnownKeys asserts representative real-world config
// shapes validate against the published schema.
func TestStackQLConfigSchemaAcceptsKnownKeys(t *testing.T) {
	schemaBytes, err := os.ReadFile(configSchemaPath)
	if err != nil {
		t.Fatalf("read config schema: %v", err)
	}
	doc := []byte(`{
		"auth": {"type": "aws_signing_v4", "credentialsenvvar": "AWS_SECRET_ACCESS_KEY"},
		"sqlExternalTables": {"information_schema.applicable_roles": {"name": "applicable_roles"}},
		"requestBodyTranslate": {"algorithm": "naive_request_body_json"},
		"minStackQLVersion": "v0.5.0",
		"snake_case_aliases": true
	}`)
	if _, err := docval.ValidateAndParse(doc, schemaBytes, "config"); err != nil {
		t.Fatalf("expected known config keys to validate, got: %v", err)
	}
}

// TestStackQLConfigSchemaRejectsUnknownKey is the "CI fails on mistyped keys"
// guarantee: a typo'd config key must be rejected rather than silently ignored.
func TestStackQLConfigSchemaRejectsUnknownKey(t *testing.T) {
	schemaBytes, err := os.ReadFile(configSchemaPath)
	if err != nil {
		t.Fatalf("read config schema: %v", err)
	}
	// `snake_case_alias` (singular) is a common typo of `snake_case_aliases`.
	doc := []byte(`{"snake_case_alias": true}`)
	if _, err := docval.ValidateAndParse(doc, schemaBytes, "config"); err == nil {
		t.Fatalf("expected mistyped config key to fail validation, got nil error")
	}
}

// TestProviderSchemaResolvesConfigRef validates a real provider document against
// the published provider schema, exercising the provider -> config $ref and
// confirming a sample document validates end to end (issue #110 acceptance).
func TestProviderSchemaResolvesConfigRef(t *testing.T) {
	schemaRoot := filepath.Join("..", "..", "cicd", "schema-definitions")
	validator := docval.NewFileValidator(schemaRoot)
	docPath := filepath.Join("testdata", "registry", "src", "local_openssl", "v0.1.0", "provider.yaml")
	rv, err := validator.ValidateAndParseFile(docPath, "provider.schema.json")
	if err != nil {
		t.Fatalf("expected provider doc to validate, got: %v", err)
	}
	if rv["name"] != "local_openssl" {
		t.Fatalf("unexpected provider name: %v", rv["name"])
	}
}
