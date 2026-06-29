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
// to push down to the upstream API. Construct one with NewPushdownIntent. The
// dialect translation lives in the internal anysdk layer.
type PushdownIntent interface {
	GetProjection() []string
	GetPredicates() []PushdownPredicate
	GetOrderBy() []PushdownOrder
	GetLimit() (int, bool)  // value, and whether a LIMIT was set
	GetOffset() (int, bool) // value, and whether an OFFSET was set
	IsCount() bool
}

type pushdownIntent struct {
	projection []string
	predicates []PushdownPredicate
	orderBy    []PushdownOrder
	limit      int
	limitSet   bool
	offset     int
	offsetSet  bool
	count      bool
}

func (i *pushdownIntent) GetProjection() []string { return i.projection }

func (i *pushdownIntent) GetPredicates() []PushdownPredicate { return i.predicates }

func (i *pushdownIntent) GetOrderBy() []PushdownOrder { return i.orderBy }

func (i *pushdownIntent) GetLimit() (int, bool) { return i.limit, i.limitSet }

func (i *pushdownIntent) GetOffset() (int, bool) { return i.offset, i.offsetSet }

func (i *pushdownIntent) IsCount() bool { return i.count }

// NewPushdownIntent builds a PushdownIntent. limitSet / offsetSet report whether
// the corresponding value is meaningful (mirroring SQL LIMIT/OFFSET being optional).
func NewPushdownIntent(
	projection []string,
	predicates []PushdownPredicate,
	orderBy []PushdownOrder,
	limit int, limitSet bool,
	offset int, offsetSet bool,
	count bool,
) PushdownIntent {
	return &pushdownIntent{
		projection: projection,
		predicates: predicates,
		orderBy:    orderBy,
		limit:      limit,
		limitSet:   limitSet,
		offset:     offset,
		offsetSet:  offsetSet,
		count:      count,
	}
}

func pushdownIntentToAnySdk(i PushdownIntent) anysdk.PushdownIntent {
	if i == nil {
		return nil
	}
	limit, limitSet := i.GetLimit()
	offset, offsetSet := i.GetOffset()
	return anysdk.NewPushdownIntent(
		i.GetProjection(),
		toAnySdkPushdownPredicates(i.GetPredicates()),
		toAnySdkPushdownOrders(i.GetOrderBy()),
		limit, limitSet,
		offset, offsetSet,
		i.IsCount(),
	)
}

func toAnySdkPushdownPredicates(in []PushdownPredicate) []anysdk.PushdownPredicate {
	if in == nil {
		return nil
	}
	out := make([]anysdk.PushdownPredicate, len(in))
	for j, p := range in {
		out[j] = anysdk.NewPushdownPredicate(p.Column, p.Operator, p.Value)
	}
	return out
}

func toAnySdkPushdownOrders(in []PushdownOrder) []anysdk.PushdownOrder {
	if in == nil {
		return nil
	}
	out := make([]anysdk.PushdownOrder, len(in))
	for j, o := range in {
		out[j] = anysdk.NewPushdownOrder(o.Column, o.Descending)
	}
	return out
}

func fromAnySdkPushdownPredicates(in []anysdk.PushdownPredicate) []PushdownPredicate {
	if in == nil {
		return nil
	}
	out := make([]PushdownPredicate, len(in))
	for j, p := range in {
		out[j] = PushdownPredicate{Column: p.GetColumn(), Operator: p.GetOperator(), Value: p.GetValue()}
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
	return &wrappedPushdownResult{inner: anysdk.ApplyPushdown(op.unwrap(), pushdownIntentToAnySdk(intent))}
}
