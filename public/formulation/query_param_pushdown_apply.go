package formulation

import "github.com/stackql/any-sdk/internal/anysdk"

// PushdownPredicate is a single neutral, dialect-agnostic WHERE predicate.
// Operator accepts either SQL-style symbols ("=", "!=", ">", ">=", "<", "<=")
// or OData logical names ("eq", "ne", "gt", "ge", "lt", "le", "startswith",
// "endswith", "contains"). Value is the raw comparison value.
type PushdownPredicate struct {
	Column   string
	Operator string
	Value    interface{}
}

// PushdownOrder is a single neutral ORDER BY term.
type PushdownOrder struct {
	Column     string
	Descending bool
}

// PushdownIntent is a neutral, dialect-agnostic description of the query options
// to push down to the upstream API. The dialect translation lives in the internal
// anysdk layer; this is the facade data-carrier (cf. RegistryConfig).
type PushdownIntent struct {
	Projection []string
	Predicates []PushdownPredicate
	OrderBy    []PushdownOrder
	Limit      int
	LimitSet   bool
	Offset     int
	OffsetSet  bool
	Count      bool
}

func (i PushdownIntent) toAnySdk() anysdk.PushdownIntent {
	return anysdk.PushdownIntent{
		Projection: i.Projection,
		Predicates: toAnySdkPushdownPredicates(i.Predicates),
		OrderBy:    toAnySdkPushdownOrders(i.OrderBy),
		Limit:      i.Limit,
		LimitSet:   i.LimitSet,
		Offset:     i.Offset,
		OffsetSet:  i.OffsetSet,
		Count:      i.Count,
	}
}

func toAnySdkPushdownPredicates(in []PushdownPredicate) []anysdk.PushdownPredicate {
	if in == nil {
		return nil
	}
	out := make([]anysdk.PushdownPredicate, len(in))
	for j, p := range in {
		out[j] = anysdk.PushdownPredicate{Column: p.Column, Operator: p.Operator, Value: p.Value}
	}
	return out
}

func toAnySdkPushdownOrders(in []PushdownOrder) []anysdk.PushdownOrder {
	if in == nil {
		return nil
	}
	out := make([]anysdk.PushdownOrder, len(in))
	for j, o := range in {
		out[j] = anysdk.PushdownOrder{Column: o.Column, Descending: o.Descending}
	}
	return out
}

func fromAnySdkPushdownPredicates(in []anysdk.PushdownPredicate) []PushdownPredicate {
	if in == nil {
		return nil
	}
	out := make([]PushdownPredicate, len(in))
	for j, p := range in {
		out[j] = PushdownPredicate{Column: p.Column, Operator: p.Operator, Value: p.Value}
	}
	return out
}

// PushdownResult is the outcome of translating a PushdownIntent against an
// OperationStore's pushdown config.
type PushdownResult interface {
	// QueryParams are the request query params to set, e.g.
	// {"$filter": "startswith(displayName,'A')", "$top": "10"}.
	QueryParams() map[string]string
	// PushedPredicates were fully translated to QueryParams; the caller may skip
	// client-side filtering for these.
	PushedPredicates() []PushdownPredicate
	// ResidualPredicates were NOT pushed; the caller must still filter these
	// client-side (the partial-pushdown contract).
	ResidualPredicates() []PushdownPredicate
	// CountResponseKey is the response key carrying the count (e.g. "@odata.count")
	// when a COUNT(*) was pushed; empty otherwise.
	CountResponseKey() string
}

type wrappedPushdownResult struct {
	inner anysdk.PushdownResult
}

func (w *wrappedPushdownResult) QueryParams() map[string]string { return w.inner.QueryParams() }

func (w *wrappedPushdownResult) PushedPredicates() []PushdownPredicate {
	return fromAnySdkPushdownPredicates(w.inner.PushedPredicates())
}

func (w *wrappedPushdownResult) ResidualPredicates() []PushdownPredicate {
	return fromAnySdkPushdownPredicates(w.inner.ResidualPredicates())
}

func (w *wrappedPushdownResult) CountResponseKey() string { return w.inner.CountResponseKey() }

// ApplyPushdown translates a neutral PushdownIntent into the request query params
// supported by the OperationStore's queryParamPushdown config. It is a thin facade
// over the internal anysdk translation. With no config the result is empty and
// every predicate is residual. Prefer HTTPPreparator.WithPushdownIntent to apply
// the params directly to a prepared request.
func ApplyPushdown(op OperationStore, intent PushdownIntent) PushdownResult {
	return &wrappedPushdownResult{inner: anysdk.ApplyPushdown(op.unwrap(), intent.toAnySdk())}
}
