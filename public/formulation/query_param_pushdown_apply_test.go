package formulation

import (
	"testing"

	"github.com/stackql/any-sdk/internal/anysdk"
	"gopkg.in/yaml.v3"
)

// fakeConfigSource is a minimal PushdownConfigSource for exercising ApplyPushdown
// without standing up a full OperationStore.
type fakeConfigSource struct {
	qpp QueryParamPushdown
}

func (f fakeConfigSource) GetQueryParamPushdown() (QueryParamPushdown, bool) {
	if f.qpp == nil {
		return nil, false
	}
	return f.qpp, true
}

func buildPushdown(t *testing.T, yamlStr string) QueryParamPushdown {
	t.Helper()
	qpp := anysdk.GetTestingQueryParamPushdown()
	if err := yaml.Unmarshal([]byte(yamlStr), &qpp); err != nil {
		t.Fatalf("failed to unmarshal pushdown config: %v", err)
	}
	return &wrappedQueryParamPushdown{inner: &qpp}
}

func assertParam(t *testing.T, params map[string]string, key, want string) {
	t.Helper()
	got, ok := params[key]
	if !ok {
		t.Fatalf("expected query param %q to be set, params=%v", key, params)
	}
	if got != want {
		t.Fatalf("query param %q = %q, want %q", key, got, want)
	}
}

const odataFullPushdownYaml = `
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
count:
  dialect: odata
`

func TestApplyPushdown_FullOData(t *testing.T) {
	src := fakeConfigSource{qpp: buildPushdown(t, odataFullPushdownYaml)}
	intent := PushdownIntent{
		Projection: []string{"id", "displayName"},
		Predicates: []PushdownPredicate{
			{Column: "displayName", Operator: "startswith", Value: "A"},
			{Column: "status", Operator: "=", Value: "active"},
			{Column: "createdYear", Operator: ">", Value: 2020},
		},
		OrderBy:  []PushdownOrder{{Column: "displayName", Descending: true}},
		Limit:    10,
		LimitSet: true,
		Count:    true,
	}

	res := ApplyPushdown(src, intent)

	assertParam(t, res.QueryParams(), "$select", "id,displayName")
	assertParam(t, res.QueryParams(), "$filter",
		"startswith(displayName,'A') and status eq 'active' and createdYear gt 2020")
	assertParam(t, res.QueryParams(), "$orderby", "displayName desc")
	assertParam(t, res.QueryParams(), "$top", "10")
	assertParam(t, res.QueryParams(), "$count", "true")

	if res.CountResponseKey() != "@odata.count" {
		t.Fatalf("CountResponseKey = %q, want %q", res.CountResponseKey(), "@odata.count")
	}
	if len(res.PushedPredicates()) != 3 {
		t.Fatalf("expected 3 pushed predicates, got %d", len(res.PushedPredicates()))
	}
	if len(res.ResidualPredicates()) != 0 {
		t.Fatalf("expected 0 residual predicates, got %d", len(res.ResidualPredicates()))
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
	src := fakeConfigSource{qpp: buildPushdown(t, partialYaml)}
	intent := PushdownIntent{
		// "secret" is not a supported select column -> whole $select suppressed.
		Projection: []string{"displayName", "secret"},
		Predicates: []PushdownPredicate{
			{Column: "displayName", Operator: "eq", Value: "A"}, // pushable
			{Column: "unknownCol", Operator: "eq", Value: "B"},  // unsupported column
			{Column: "displayName", Operator: "gt", Value: 5},   // unsupported operator
		},
	}

	res := ApplyPushdown(src, intent)

	if _, ok := res.QueryParams()["$select"]; ok {
		t.Fatalf("expected $select to be suppressed when a column is unsupported")
	}
	assertParam(t, res.QueryParams(), "$filter", "displayName eq 'A'")
	if len(res.PushedPredicates()) != 1 {
		t.Fatalf("expected 1 pushed predicate, got %d", len(res.PushedPredicates()))
	}
	if len(res.ResidualPredicates()) != 2 {
		t.Fatalf("expected 2 residual predicates, got %d", len(res.ResidualPredicates()))
	}
}

func TestApplyPushdown_CustomDialect(t *testing.T) {
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
top:
  paramName: "limit"
  maxValue: 100
count:
  paramName: "include_count"
  paramValue: "1"
  responseKey: "meta.total"
`
	src := fakeConfigSource{qpp: buildPushdown(t, customYaml)}
	intent := PushdownIntent{
		Projection: []string{"a", "b"},
		Predicates: []PushdownPredicate{{Column: "status", Operator: "eq", Value: "x"}},
		Limit:      250, // above maxValue -> clamped
		LimitSet:   true,
		Count:      true,
	}

	res := ApplyPushdown(src, intent)

	assertParam(t, res.QueryParams(), "fields", "a|b")
	assertParam(t, res.QueryParams(), "limit", "100")
	assertParam(t, res.QueryParams(), "include_count", "1")

	// Non-OData filter syntax is not renderable here -> residual, no filter param.
	if _, ok := res.QueryParams()["filter"]; ok {
		t.Fatalf("expected custom non-odata filter to be left residual")
	}
	if len(res.ResidualPredicates()) != 1 {
		t.Fatalf("expected 1 residual predicate, got %d", len(res.ResidualPredicates()))
	}
	if res.CountResponseKey() != "meta.total" {
		t.Fatalf("CountResponseKey = %q, want %q", res.CountResponseKey(), "meta.total")
	}
}

func TestApplyPushdown_TopClamp(t *testing.T) {
	src := fakeConfigSource{qpp: buildPushdown(t, odataFullPushdownYaml)}
	res := ApplyPushdown(src, PushdownIntent{Limit: 5000, LimitSet: true})
	assertParam(t, res.QueryParams(), "$top", "1000")
}

func TestApplyPushdown_AbsentConfigNoOp(t *testing.T) {
	intent := PushdownIntent{
		Projection: []string{"a"},
		Predicates: []PushdownPredicate{{Column: "a", Operator: "eq", Value: 1}},
		Limit:      10,
		LimitSet:   true,
		Count:      true,
	}

	res := ApplyPushdown(fakeConfigSource{qpp: nil}, intent)

	if len(res.QueryParams()) != 0 {
		t.Fatalf("expected zero query params with absent config, got %v", res.QueryParams())
	}
	if len(res.PushedPredicates()) != 0 {
		t.Fatalf("expected zero pushed predicates, got %d", len(res.PushedPredicates()))
	}
	if len(res.ResidualPredicates()) != 1 {
		t.Fatalf("expected all predicates residual, got %d", len(res.ResidualPredicates()))
	}
}
