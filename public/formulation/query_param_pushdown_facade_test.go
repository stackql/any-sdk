package formulation

import (
	"testing"

	"github.com/stackql/any-sdk/internal/anysdk"
	"gopkg.in/yaml.v3"
)

func TestPushdownIntent_toAnySdk(t *testing.T) {
	in := NewPushdownIntent(
		[]string{"a", "b"},
		[]PushdownPredicate{{Column: "c", Operator: "eq", Value: 1}},
		[]PushdownOrder{{Column: "d", Descending: true}},
		5, true,
		2, true,
		true,
	)
	got := pushdownIntentToAnySdk(in)
	if len(got.Projection) != 2 || got.Projection[1] != "b" {
		t.Fatalf("Projection = %v", got.Projection)
	}
	if len(got.Predicates) != 1 || got.Predicates[0].Column != "c" || got.Predicates[0].Operator != "eq" {
		t.Fatalf("Predicates = %v", got.Predicates)
	}
	if len(got.OrderBy) != 1 || !got.OrderBy[0].Descending || got.OrderBy[0].Column != "d" {
		t.Fatalf("OrderBy = %v", got.OrderBy)
	}
	if got.Limit != 5 || !got.LimitSet || got.Offset != 2 || !got.OffsetSet || !got.Count {
		t.Fatalf("scalar fields not mapped: %+v", got)
	}
}

type facadeFakeSource struct {
	qpp anysdk.QueryParamPushdown
}

func (f facadeFakeSource) GetQueryParamPushdown() (anysdk.QueryParamPushdown, bool) {
	if f.qpp == nil {
		return nil, false
	}
	return f.qpp, true
}

func TestWrappedPushdownResult_Delegates(t *testing.T) {
	qpp := anysdk.GetTestingQueryParamPushdown()
	const yamlStr = `
filter:
  dialect: odata
  supportedOperators:
    - "eq"
  supportedColumns:
    - "status"
top:
  dialect: odata
`
	if err := yaml.Unmarshal([]byte(yamlStr), &qpp); err != nil {
		t.Fatal(err)
	}
	inner := anysdk.ApplyPushdown(facadeFakeSource{qpp: &qpp}, anysdk.PushdownIntent{
		Predicates: []anysdk.PushdownPredicate{{Column: "status", Operator: "eq", Value: "x"}},
		Limit:      7,
		LimitSet:   true,
	})
	w := &wrappedPushdownResult{inner: inner}
	if w.QueryParams()["$filter"] != "status eq 'x'" {
		t.Fatalf("$filter = %v", w.QueryParams())
	}
	if w.QueryParams()["$top"] != "7" {
		t.Fatalf("$top = %v", w.QueryParams())
	}
	if len(w.PushedPredicates()) != 1 || w.PushedPredicates()[0].Column != "status" {
		t.Fatalf("PushedPredicates = %v", w.PushedPredicates())
	}
}
