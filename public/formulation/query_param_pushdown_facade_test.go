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
	if len(got.GetProjection()) != 2 || got.GetProjection()[1] != "b" {
		t.Fatalf("Projection = %v", got.GetProjection())
	}
	preds := got.GetPredicates()
	if len(preds) != 1 || preds[0].GetColumn() != "c" || preds[0].GetOperator() != "eq" {
		t.Fatalf("Predicates = %v", preds)
	}
	orders := got.GetOrderBy()
	if len(orders) != 1 || !orders[0].IsDescending() || orders[0].GetColumn() != "d" {
		t.Fatalf("OrderBy = %v", orders)
	}
	limit, limitSet := got.GetLimit()
	offset, offsetSet := got.GetOffset()
	if limit != 5 || !limitSet || offset != 2 || !offsetSet || !got.IsCount() {
		t.Fatalf("scalar fields not mapped: limit=%d/%v offset=%d/%v count=%v", limit, limitSet, offset, offsetSet, got.IsCount())
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
	inner := anysdk.ApplyPushdown(facadeFakeSource{qpp: &qpp}, anysdk.NewPushdownIntent(
		nil,
		[]anysdk.PushdownPredicate{anysdk.NewPushdownPredicate("status", "eq", "x")},
		nil,
		7, true,
		0, false,
		false,
	))
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
