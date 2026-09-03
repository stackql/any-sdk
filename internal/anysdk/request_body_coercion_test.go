package anysdk

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/stackql/any-sdk/pkg/media"
)

const coercionTestSchemaJSON = `{
  "type": "object",
  "properties": {
    "backupPeriodInHours": {"type": "number"},
    "backupRetentionPeriodInHours": {"type": "integer"},
    "backupStartTime": {"type": "string"},
    "enabled": {"type": "boolean"},
    "label": {"type": "string"},
    "ports": {"type": "array", "items": {"type": "integer"}},
    "nested": {"type": "object", "properties": {"ratio": {"type": "number"}}},
    "extras": {"type": "object", "additionalProperties": {"type": "integer"}},
    "untyped": {}
  }
}`

func coercionTestSchema(t *testing.T) Schema {
	t.Helper()
	var sc openapi3.Schema
	if err := json.Unmarshal([]byte(coercionTestSchemaJSON), &sc); err != nil {
		t.Fatalf("failed to unmarshal test schema: %v", err)
	}
	return newSchema(&sc, nil, "Body", "")
}

func TestCoerceBodyToSchemaConvertsStringsToDeclaredTypes(t *testing.T) {
	body := map[string]interface{}{
		"backupPeriodInHours":          "24",
		"backupRetentionPeriodInHours": "48",
		"backupStartTime":              "02:00",
		"enabled":                      "true",
		"label":                        float64(12),
		"ports":                        []interface{}{"8123", float64(9000)},
		"nested":                       map[string]interface{}{"ratio": "0.5"},
		"extras":                       map[string]interface{}{"a": "7"},
		"untyped":                      "leave me",
		"unknown":                      "leave me too",
	}
	got, err := coerceBodyToSchema(coercionTestSchema(t), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]interface{}{
		"backupPeriodInHours":          float64(24),
		"backupRetentionPeriodInHours": int64(48),
		"backupStartTime":              "02:00",
		"enabled":                      true,
		"label":                        "12",
		"ports":                        []interface{}{int64(8123), int64(9000)},
		"nested":                       map[string]interface{}{"ratio": float64(0.5)},
		"extras":                       map[string]interface{}{"a": int64(7)},
		"untyped":                      "leave me",
		"unknown":                      "leave me too",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("coerced body mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestCoerceBodyToSchemaLeavesTypedValuesUnchanged(t *testing.T) {
	body := map[string]interface{}{
		"backupPeriodInHours":          float64(24),
		"backupRetentionPeriodInHours": float64(48),
		"enabled":                      true,
		"label":                        "x",
	}
	got, err := coerceBodyToSchema(coercionTestSchema(t), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]interface{}{
		"backupPeriodInHours":          float64(24),
		"backupRetentionPeriodInHours": int64(48),
		"enabled":                      true,
		"label":                        "x",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("coerced body mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestCoerceBodyToSchemaRejectsUnparseableScalars(t *testing.T) {
	cases := []struct {
		name string
		body map[string]interface{}
		key  string
	}{
		{"non-numeric string for number", map[string]interface{}{"backupPeriodInHours": "abc"}, "backupPeriodInHours"},
		{"fractional value for integer", map[string]interface{}{"backupRetentionPeriodInHours": 1.5}, "backupRetentionPeriodInHours"},
		{"non-boolean string for boolean", map[string]interface{}{"enabled": "maybe"}, "enabled"},
		{"nested path reported", map[string]interface{}{"nested": map[string]interface{}{"ratio": "x"}}, "nested.ratio"},
		{"array index reported", map[string]interface{}{"ports": []interface{}{"80", "http"}}, "ports[1]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := coerceBodyToSchema(coercionTestSchema(t), tc.body)
			if err == nil {
				t.Fatalf("expected an error")
			}
			if !strings.Contains(err.Error(), "'"+tc.key+"'") {
				t.Fatalf("error %q does not name key %q", err, tc.key)
			}
		})
	}
}

func TestCoerceBodyToSchemaPassesThroughWithoutSchema(t *testing.T) {
	body := map[string]interface{}{"n": "24"}
	got, err := coerceBodyToSchema(nil, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, body) {
		t.Fatalf("expected body unchanged without a schema, got %#v", got)
	}
	got, err = coerceBodyToSchema(coercionTestSchema(t), nil)
	if err != nil || got != nil {
		t.Fatalf("expected nil body to stay nil, got %#v, err %v", got, err)
	}
}

func TestMarshalBodyCoercesJSONToRequestSchema(t *testing.T) {
	op := &standardOpenAPIOperationStore{}
	er := &standardExpectedRequest{BodyMediaType: media.MediaTypeJson, Schema: coercionTestSchema(t)}
	mb := op.marshalBody(map[string]interface{}{
		"backupPeriodInHours":          "24",
		"backupRetentionPeriodInHours": "48",
		"backupStartTime":              "02:00",
	}, er)
	if err, hasErr := mb.GetError(); hasErr {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	want := `{"backupPeriodInHours":24,"backupRetentionPeriodInHours":48,"backupStartTime":"02:00"}`
	if got := string(mb.GetBytes()); got != want {
		t.Fatalf("marshalled body\n got: %s\nwant: %s", got, want)
	}
	mb = op.marshalBody(map[string]interface{}{"backupPeriodInHours": "abc"}, er)
	if _, hasErr := mb.GetError(); !hasErr {
		t.Fatalf("expected a marshal error for an unparseable number")
	}
}

func TestIsFloatAcceptsOpenAPINumber(t *testing.T) {
	cases := map[string]bool{"number": true, "float64": true, "float": true, "integer": false, "string": false}
	for typ, want := range cases {
		s := newSchema(&openapi3.Schema{Type: typ}, nil, typ, "")
		if got := s.IsFloat(); got != want {
			t.Errorf("IsFloat() for type %q = %v, want %v", typ, got, want)
		}
	}
}
