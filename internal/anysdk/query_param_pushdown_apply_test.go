package anysdk

import (
	"net/http"
	"net/url"
	"testing"

	"gopkg.in/yaml.v3"
)

type applyTestSource struct {
	qpp QueryParamPushdown
}

func (f applyTestSource) GetQueryParamPushdown() (QueryParamPushdown, bool) {
	if f.qpp == nil {
		return nil, false
	}
	return f.qpp, true
}

func applyTestBuildPushdown(t *testing.T, yamlStr string) QueryParamPushdown {
	t.Helper()
	qpp := GetTestingQueryParamPushdown()
	if err := yaml.Unmarshal([]byte(yamlStr), &qpp); err != nil {
		t.Fatalf("failed to unmarshal pushdown config: %v", err)
	}
	return &qpp
}

func applyTestAssertParam(t *testing.T, params map[string]string, key, want string) {
	t.Helper()
	got, ok := params[key]
	if !ok {
		t.Fatalf("expected query param %q to be set, params=%v", key, params)
	}
	if got != want {
		t.Fatalf("query param %q = %q, want %q", key, got, want)
	}
}

const pushdownApplyODataYaml = `
select:
  dialect: odata
  supportedColumns:
    - "id"
    - "displayName"
filter:
  dialect: odata
  supportedOperators:
    - "eq"
    - "ne"
    - "gt"
    - "startswith"
    - "and"
  supportedColumns:
    - "displayName"
    - "status"
    - "createdYear"
orderBy:
  dialect: odata
  supportedColumns:
    - "displayName"
    - "createdYear"
top:
  dialect: odata
  maxValue: 1000
skip:
  dialect: odata
  maxValue: 2000
count:
  dialect: odata
`

func TestApplyPushdown_FullOData(t *testing.T) {
	src := applyTestSource{qpp: applyTestBuildPushdown(t, pushdownApplyODataYaml)}
	intent := PushdownIntent{
		Projection: []string{"id", "displayName"},
		Predicates: []PushdownPredicate{
			{Column: "displayName", Operator: "startswith", Value: "A"},
			{Column: "status", Operator: "=", Value: "active"},
			{Column: "createdYear", Operator: ">", Value: 2020},
		},
		OrderBy:   []PushdownOrder{{Column: "displayName", Descending: true}},
		Limit:     10,
		LimitSet:  true,
		Offset:    20,
		OffsetSet: true,
		Count:     true,
	}

	res := ApplyPushdown(src, intent)

	applyTestAssertParam(t, res.QueryParams(), "$select", "id,displayName")
	applyTestAssertParam(t, res.QueryParams(), "$filter",
		"startswith(displayName,'A') and status eq 'active' and createdYear gt 2020")
	applyTestAssertParam(t, res.QueryParams(), "$orderby", "displayName desc")
	applyTestAssertParam(t, res.QueryParams(), "$top", "10")
	applyTestAssertParam(t, res.QueryParams(), "$skip", "20")
	applyTestAssertParam(t, res.QueryParams(), "$count", "true")

	if res.CountResponseKey() != "@odata.count" {
		t.Fatalf("CountResponseKey = %q, want @odata.count", res.CountResponseKey())
	}
	if len(res.PushedPredicates()) != 3 || len(res.ResidualPredicates()) != 0 {
		t.Fatalf("pushed=%d residual=%d, want 3/0", len(res.PushedPredicates()), len(res.ResidualPredicates()))
	}
}

func TestApplyPushdown_PartialResidual(t *testing.T) {
	const partialYaml = `
select:
  dialect: odata
  supportedColumns:
    - "displayName"
filter:
  dialect: odata
  supportedOperators:
    - "eq"
    - "startswith"
  supportedColumns:
    - "displayName"
`
	src := applyTestSource{qpp: applyTestBuildPushdown(t, partialYaml)}
	intent := PushdownIntent{
		Projection: []string{"displayName", "secret"}, // secret unsupported -> $select suppressed
		Predicates: []PushdownPredicate{
			{Column: "displayName", Operator: "eq", Value: "A"}, // pushable
			{Column: "unknownCol", Operator: "eq", Value: "B"},  // unsupported column
			{Column: "displayName", Operator: "gt", Value: 5},   // unsupported operator
		},
	}

	res := ApplyPushdown(src, intent)

	if _, ok := res.QueryParams()["$select"]; ok {
		t.Fatalf("expected $select suppressed when a column is unsupported")
	}
	applyTestAssertParam(t, res.QueryParams(), "$filter", "displayName eq 'A'")
	if len(res.PushedPredicates()) != 1 || len(res.ResidualPredicates()) != 2 {
		t.Fatalf("pushed=%d residual=%d, want 1/2", len(res.PushedPredicates()), len(res.ResidualPredicates()))
	}
}

func TestApplyPushdown_CustomDialectAndSkipClamp(t *testing.T) {
	const customYaml = `
select:
  paramName: "fields"
  delimiter: "|"
  supportedColumns:
    - "*"
filter:
  paramName: "filter"
  syntax: "key_value"
  supportedOperators:
    - "eq"
  supportedColumns:
    - "status"
skip:
  paramName: "offset"
  maxValue: 100
count:
  paramName: "include_count"
  paramValue: "1"
  responseKey: "meta.total"
`
	src := applyTestSource{qpp: applyTestBuildPushdown(t, customYaml)}
	res := ApplyPushdown(src, PushdownIntent{
		Projection: []string{"a", "b"},
		Predicates: []PushdownPredicate{{Column: "status", Operator: "eq", Value: "x"}},
		Offset:     250, // above maxValue -> clamped
		OffsetSet:  true,
		Count:      true,
	})

	applyTestAssertParam(t, res.QueryParams(), "fields", "a|b")
	applyTestAssertParam(t, res.QueryParams(), "offset", "100")
	applyTestAssertParam(t, res.QueryParams(), "include_count", "1")
	if _, ok := res.QueryParams()["filter"]; ok {
		t.Fatalf("non-odata custom filter must be residual")
	}
	if len(res.ResidualPredicates()) != 1 {
		t.Fatalf("residual=%d, want 1", len(res.ResidualPredicates()))
	}
	if res.CountResponseKey() != "meta.total" {
		t.Fatalf("CountResponseKey = %q, want meta.total", res.CountResponseKey())
	}
}

func TestApplyPushdown_TopClampAndAbsentConfig(t *testing.T) {
	src := applyTestSource{qpp: applyTestBuildPushdown(t, pushdownApplyODataYaml)}
	res := ApplyPushdown(src, PushdownIntent{Limit: 5000, LimitSet: true})
	applyTestAssertParam(t, res.QueryParams(), "$top", "1000")

	// Absent config: no params, all predicates residual.
	noConfig := ApplyPushdown(applyTestSource{qpp: nil}, PushdownIntent{
		Predicates: []PushdownPredicate{{Column: "a", Operator: "eq", Value: 1}},
		Limit:      10, LimitSet: true,
	})
	if len(noConfig.QueryParams()) != 0 {
		t.Fatalf("expected zero params with absent config, got %v", noConfig.QueryParams())
	}
	if len(noConfig.ResidualPredicates()) != 1 {
		t.Fatalf("expected all predicates residual, got %d", len(noConfig.ResidualPredicates()))
	}
}

// --- query-setting seam (the apply-to-request half) ---

type applyTestQueryParam struct {
	req *http.Request
}

func (f *applyTestQueryParam) GetQuery() url.Values { return f.req.URL.Query() }

func (f *applyTestQueryParam) SetRawQuery(q string) { f.req.URL.RawQuery = q }

func TestSetPushdownQueryParams_SetsAndPreserves(t *testing.T) {
	req, err := http.NewRequest("GET", "https://example.com/api?existing=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	setPushdownQueryParams([]queryParamSettable{&applyTestQueryParam{req: req}}, map[string]string{
		"$top":    "10",
		"$filter": "startswith(name,'A')",
	})
	q := req.URL.Query()
	if q.Get("$top") != "10" || q.Get("$filter") != "startswith(name,'A')" || q.Get("existing") != "1" {
		t.Fatalf("unexpected query: %v", q)
	}
}

func TestSetPushdownQueryParams_EmptyIsByteForByteNoOp(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://example.com/api?a=1&b=2", nil)
	before := req.URL.RawQuery
	setPushdownQueryParams([]queryParamSettable{&applyTestQueryParam{req: req}}, map[string]string{})
	if req.URL.RawQuery != before {
		t.Fatalf("RawQuery changed on empty params: %q -> %q", before, req.URL.RawQuery)
	}
}

func TestApplyPushdown_IntentToRequestQuery(t *testing.T) {
	src := applyTestSource{qpp: applyTestBuildPushdown(t, pushdownApplyODataYaml)}
	res := ApplyPushdown(src, PushdownIntent{
		Predicates: []PushdownPredicate{{Column: "status", Operator: "eq", Value: "active"}},
		Limit:      10,
		LimitSet:   true,
	})
	req, _ := http.NewRequest("GET", "https://example.com/api", nil)
	setPushdownQueryParams([]queryParamSettable{&applyTestQueryParam{req: req}}, res.QueryParams())
	q := req.URL.Query()
	if q.Get("$filter") != "status eq 'active'" {
		t.Errorf("$filter = %q, want status eq 'active'", q.Get("$filter"))
	}
	if q.Get("$top") != "10" {
		t.Errorf("$top = %q, want 10", q.Get("$top"))
	}
}
