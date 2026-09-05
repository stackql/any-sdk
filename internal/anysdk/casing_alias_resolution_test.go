package anysdk

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/stackql/any-sdk/pkg/casing"
)

// Issue #131 (surface parity) and #119 (hyphenated wire names): every alias
// path accepts the wire spelling first and the snake alias second, and nothing
// changes for a method that declares no native casing.

func aliasTestParam(name, in string, required bool) *openapi3.ParameterRef {
	return &openapi3.ParameterRef{Value: &openapi3.Parameter{Name: name, In: in, Required: required}}
}

func aliasTestOpStore(nativeCasing string, snakeFlag bool, params ...*openapi3.ParameterRef) *standardOpenAPIOperationStore {
	op := &standardOpenAPIOperationStore{
		Provider:     &standardProvider{StackQLConfig: &standardStackQLConfig{SnakeCaseAliases: snakeFlag}},
		OperationRef: &OperationRef{Value: &openapi3.Operation{Parameters: openapi3.Parameters(params)}},
	}
	if nativeCasing != "" {
		op.Request = &standardExpectedRequest{NativeCasing: nativeCasing}
	}
	return op
}

func aliasTestBodySchema(propNames ...string) Schema {
	props := openapi3.Schemas{}
	for _, p := range propNames {
		props[p] = stringProperty()
	}
	return newStandardSchema(&openapi3.Schema{Type: "object", Properties: props, Required: []string{propNames[0]}}, nil, "Body", "")
}

func TestParameterAliasesResolveAcronymsAndHyphens(t *testing.T) {
	op := aliasTestOpStore(casing.Camel, false,
		aliasTestParam("IPProtocol", openapi3.ParameterInQuery, false),
		aliasTestParam("BinaryId", openapi3.ParameterInPath, true),
		aliasTestParam("openai-organization", openapi3.ParameterInHeader, false),
	)
	for alias, wire := range map[string]string{
		"ip_protocol":         "IPProtocol",
		"binary_id":           "BinaryId",
		"openai_organization": "openai-organization",
		"IPProtocol":          "IPProtocol",
	} {
		p, ok := op.GetParameter(alias)
		if !ok || p.GetName() != wire {
			t.Errorf("GetParameter(%q): ok=%v name=%v, want %q", alias, ok, p, wire)
		}
		p, ok = op.GetOperationParameter(alias)
		if !ok || p.GetName() != wire {
			t.Errorf("GetOperationParameter(%q): ok=%v name=%v, want %q", alias, ok, p, wire)
		}
	}
	if _, ok := op.parameterMatch(map[string]interface{}{"binary_id": "b"}); !ok {
		t.Error("snake alias of an acronym-headed required parameter must satisfy routing")
	}
	if _, ok := op.GetParameter("binary_idz"); ok {
		t.Error("an unresolvable key must not resolve")
	}
	aliases := op.GetParametersIncludingNativeCasing()
	for _, want := range []string{"openai_organization", "openai-organization", "ip_protocol", "IPProtocol"} {
		if _, ok := aliases[want]; !ok {
			t.Errorf("alias set is missing %q", want)
		}
	}
	plain := aliasTestOpStore("", false, aliasTestParam("BinaryId", openapi3.ParameterInPath, true))
	if _, ok := plain.GetParameter("binary_id"); ok {
		t.Error("snake alias must not resolve absent nativeCasing")
	}
}

func TestRequestBodyAttributeSnakeAliasRevertsToWireKey(t *testing.T) {
	op := aliasTestOpStore(casing.Camel, false)
	op.Request = &standardExpectedRequest{NativeCasing: casing.Camel, BodyMediaType: "application/json", Schema: aliasTestBodySchema("storageClass", "name")}
	for _, in := range []string{"data__storage_class", "data__storageClass"} {
		got, err := op.revertRequestBodyAttributeRename(in)
		if err != nil || got != "storageClass" {
			t.Errorf("revert(%q) = %q, %v; want storageClass", in, got, err)
		}
	}
	if got, _ := op.revertRequestBodyAttributeRename("data__unknown_key"); got != "unknown_key" {
		t.Errorf("unknown key must pass through, got %q", got)
	}
	for _, key := range []string{"data__storage_class", "data__storageClass"} {
		params, err := splitHTTPParameters(map[int]map[string]interface{}{0: {key: "NEARLINE"}}, op)
		if err != nil || len(params) != 1 {
			t.Fatalf("splitHTTPParameters(%q): %v", key, err)
		}
		if got := params[0].GetRequestBody()["storageClass"]; got != "NEARLINE" {
			t.Errorf("body for %q = %v, want storageClass=NEARLINE", key, params[0].GetRequestBody())
		}
	}
	plain := aliasTestOpStore("", false)
	plain.Request = &standardExpectedRequest{BodyMediaType: "application/json", Schema: aliasTestBodySchema("storageClass")}
	if got, _ := plain.revertRequestBodyAttributeRename("data__storage_class"); got != "storage_class" {
		t.Errorf("absent nativeCasing the key must be untouched, got %q", got)
	}
}

func TestPresentationIsSnakeOnlyUnderBothGates(t *testing.T) {
	body := aliasTestBodySchema("storageClass")
	cases := []struct {
		name           string
		nativeCasing   string
		snakeFlag      bool
		wantParams     string
		wantBodyRename string
	}{
		{"both gates", casing.Camel, true, "max_results, data__storage_class", "data__storage_class"},
		{"flag only", "", true, "maxResults, data__storageClass", "data__storageClass"},
		{"casing only", casing.Camel, false, "maxResults, data__storageClass", "data__storageClass"},
		{"neither", "", false, "maxResults, data__storageClass", "data__storageClass"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := aliasTestOpStore(tc.nativeCasing, tc.snakeFlag, aliasTestParam("maxResults", openapi3.ParameterInQuery, true))
			op.Request = &standardExpectedRequest{NativeCasing: tc.nativeCasing, BodyMediaType: "application/json", Schema: body}
			if got := op.ToPresentationMap(false)[RequiredParams]; got != tc.wantParams {
				t.Errorf("RequiredParams = %q, want %q", got, tc.wantParams)
			}
			if got, _ := op.RenameRequestBodyAttribute("storageClass"); got != tc.wantBodyRename {
				t.Errorf("RenameRequestBodyAttribute = %q, want %q", got, tc.wantBodyRename)
			}
			// Resolution keeps the wire spelling regardless of presentation.
			if _, ok := op.GetParameters()["data__storageClass"]; !ok {
				t.Errorf("wire body key must stay resolvable: %v", wireKeysOf(op.GetParameters()))
			}
		})
	}
}

func TestBodylessMethodHasNoRequestSchemaError(t *testing.T) {
	op := aliasTestOpStore(casing.Camel, false)
	s, err := op.GetRequestBodySchema()
	if err != nil || s != nil {
		t.Fatalf("metadata-only request block: schema=%v err=%v, want nil, nil", s, err)
	}
	paths, err := op.getRequestBodyStringifiedPaths()
	if err == nil || len(paths) != 0 {
		t.Errorf("stringified paths = %v, %v; want empty with an explicit error", paths, err)
	}
	if _, err := op.getRequestBodySchemaAttributeMatcher(""); err == nil {
		t.Error("an attribute matcher needs a schema and must still error")
	}
	if s, err := aliasTestOpStore("", false).GetRequestBodySchema(); err != nil || s != nil {
		t.Errorf("no request block: schema=%v err=%v, want nil, nil", s, err)
	}
}

func TestSchemaPropertyLookupAcceptsSnakeAliasUnderFlag(t *testing.T) {
	s := snakeAliasedObjectSchema(true, "machineType", "IPProtocol")
	for _, key := range []string{"machineType", "machine_type", "IPProtocol", "ip_protocol"} {
		if _, ok := s.GetProperty(key); !ok {
			t.Errorf("GetProperty(%q) must resolve under snake_case_aliases", key)
		}
		if s.FindByPath(key, nil) == nil {
			t.Errorf("FindByPath(%q) must resolve under snake_case_aliases", key)
		}
	}
	if _, ok := s.GetProperty("machine_typez"); ok {
		t.Error("an unknown column must not resolve")
	}
	off := snakeAliasedObjectSchema(false, "machineType")
	if _, ok := off.GetProperty("machine_type"); ok {
		t.Error("snake alias must not resolve when the flag is off")
	}
	if off.FindByPath("machine_type", nil) != nil {
		t.Error("FindByPath snake alias must not resolve when the flag is off")
	}
}
