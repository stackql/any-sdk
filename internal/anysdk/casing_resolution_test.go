package anysdk

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/stackql/any-sdk/pkg/casing"
)

// stringProperty is a tiny helper for building object-schema fixtures.
func stringProperty() *openapi3.SchemaRef {
	return &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "string"}}
}

func columnsByName(cols []ColumnDescriptor) map[string]ColumnDescriptor {
	rv := make(map[string]ColumnDescriptor, len(cols))
	for _, c := range cols {
		rv[c.GetName()] = c
	}
	return rv
}

// TestSnakeCaseAliasColumnWireName covers issue #108: when snake_case_aliases is
// enabled the column display name is snake-cased but GetWireName retains the wire
// property name so data extraction is keyed correctly.
func TestSnakeCaseAliasColumnWireName(t *testing.T) {
	prov := &standardProvider{StackQLConfig: &standardStackQLConfig{SnakeCaseAliases: true}}
	svc := &standardService{Provider: prov}
	sc := &openapi3.Schema{
		Type: "object",
		Properties: openapi3.Schemas{
			"VpcId":        stringProperty(),
			"cidrBlock":    stringProperty(),
			"echoed_query": stringProperty(),
		},
	}
	s := newStandardSchema(sc, svc, "Echo", "")

	cols := columnsByName(s.Tabulate(false, "").GetColumns())

	cases := []struct {
		displayName  string
		wantWireName string
	}{
		{"vpc_id", "VpcId"},
		{"cidr_block", "cidrBlock"},
		{"echoed_query", "echoed_query"}, // single-word: snake == wire
	}
	for _, c := range cases {
		col, ok := cols[c.displayName]
		if !ok {
			t.Fatalf("expected a column named %q; got columns %v", c.displayName, keysOf(cols))
		}
		if got := col.GetWireName(); got != c.wantWireName {
			t.Errorf("column %q: GetWireName() = %q, want %q", c.displayName, got, c.wantWireName)
		}
	}
}

// TestSnakeCaseAliasesDisabledIsWireKeyed asserts the default (flag absent)
// behaviour is unchanged: display name and wire name are both the wire name.
func TestSnakeCaseAliasesDisabledIsWireKeyed(t *testing.T) {
	prov := &standardProvider{StackQLConfig: &standardStackQLConfig{}}
	svc := &standardService{Provider: prov}
	sc := &openapi3.Schema{
		Type: "object",
		Properties: openapi3.Schemas{
			"VpcId":     stringProperty(),
			"cidrBlock": stringProperty(),
		},
	}
	s := newStandardSchema(sc, svc, "Echo", "")

	cols := columnsByName(s.Tabulate(false, "").GetColumns())
	for _, wire := range []string{"VpcId", "cidrBlock"} {
		col, ok := cols[wire]
		if !ok {
			t.Fatalf("expected a column named %q; got columns %v", wire, keysOf(cols))
		}
		if got := col.GetWireName(); got != wire {
			t.Errorf("column %q: GetWireName() = %q, want %q", wire, got, wire)
		}
	}
}

// TestGetParametersIncludingNativeCasing covers issue #109: snake_case keys
// resolve to their wire (PascalCase) Addressable via direct set membership when
// the method declares request.nativeCasing.
func TestGetParametersIncludingNativeCasing(t *testing.T) {
	m := newPascalCasingTestOperationStore("VpcId", "SubnetId")

	params := m.GetParametersIncludingNativeCasing()

	wantSnakeToWire := map[string]string{
		"vpc_id":    "VpcId",
		"subnet_id": "SubnetId",
	}
	for snakeKey, wireName := range wantSnakeToWire {
		addr, ok := params[snakeKey]
		if !ok {
			t.Fatalf("expected snake alias %q in parameter set; got %v", snakeKey, keysOfAddressable(params))
		}
		if got := addr.GetName(); got != wireName {
			t.Errorf("snake alias %q resolved to wire name %q, want %q", snakeKey, got, wireName)
		}
	}
	// The original wire keys must still be present and unchanged.
	for _, wire := range []string{"VpcId", "SubnetId"} {
		if _, ok := params[wire]; !ok {
			t.Errorf("expected wire key %q to remain in parameter set", wire)
		}
	}
	if len(params) != 4 {
		t.Errorf("expected 4 entries (2 wire + 2 snake), got %d: %v", len(params), keysOfAddressable(params))
	}
}

// TestGetParametersIncludingNativeCasingAbsentIsUnchanged asserts that, absent
// request.nativeCasing, the resolver set is identical to the wire-keyed set.
func TestGetParametersIncludingNativeCasingAbsentIsUnchanged(t *testing.T) {
	m := &standardOpenAPIOperationStore{
		OperationRef: &OperationRef{Value: &openapi3.Operation{
			Parameters: openapi3.Parameters{
				queryParameterRef("VpcId"),
				queryParameterRef("SubnetId"),
			},
		}},
	}

	params := m.GetParametersIncludingNativeCasing()
	if len(params) != 2 {
		t.Fatalf("expected only the 2 wire keys, got %d: %v", len(params), keysOfAddressable(params))
	}
	for _, wire := range []string{"VpcId", "SubnetId"} {
		if _, ok := params[wire]; !ok {
			t.Errorf("expected wire key %q", wire)
		}
	}
}

func newPascalCasingTestOperationStore(queryParamNames ...string) *standardOpenAPIOperationStore {
	refs := make(openapi3.Parameters, 0, len(queryParamNames))
	for _, name := range queryParamNames {
		refs = append(refs, queryParameterRef(name))
	}
	return &standardOpenAPIOperationStore{
		OperationRef: &OperationRef{Value: &openapi3.Operation{Parameters: refs}},
		Request:      &standardExpectedRequest{NativeCasing: casing.Pascal},
	}
}

func queryParameterRef(name string) *openapi3.ParameterRef {
	return &openapi3.ParameterRef{Value: &openapi3.Parameter{Name: name, In: openapi3.ParameterInQuery}}
}

// TestHasRequestBodyContent covers the body-presence guard that lets a request
// block carry metadata (e.g. nativeCasing on a body-less GET) without forcing
// body marshalling. NewHttpParameters seeds an empty non-nil map, so an empty map
// must count as "no content".
func TestHasRequestBodyContent(t *testing.T) {
	cases := []struct {
		name string
		body interface{}
		want bool
	}{
		{"nil interface", nil, false},
		{"nil map", map[string]interface{}(nil), false},
		{"empty map", map[string]interface{}{}, false},
		{"populated map", map[string]interface{}{"name": "x"}, true},
	}
	for _, c := range cases {
		if got := hasRequestBodyContent(c.body); got != c.want {
			t.Errorf("%s: hasRequestBodyContent() = %v, want %v", c.name, got, c.want)
		}
	}
}

func keysOf(m map[string]ColumnDescriptor) []string {
	rv := make([]string, 0, len(m))
	for k := range m {
		rv = append(rv, k)
	}
	return rv
}

func keysOfAddressable(m map[string]Addressable) []string {
	rv := make([]string, 0, len(m))
	for k := range m {
		rv = append(rv, k)
	}
	return rv
}
