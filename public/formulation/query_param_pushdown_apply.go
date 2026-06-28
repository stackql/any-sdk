package formulation

import (
	"fmt"
	"strconv"
	"strings"
)

// dialectODataSyntax is the dialect/syntax token used by OData pushdown configs.
// It mirrors anysdk.ODataDialect / anysdk.DefaultFilterSyntax without re-exporting
// the internal constant.
const dialectODataSyntax = "odata"

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
// stackql would like to push down to the upstream API. It carries no foreign
// (OData/custom) syntax; ApplyPushdown performs the dialect translation.
type PushdownIntent struct {
	// Projection holds the SELECT columns.
	Projection []string
	// Predicates holds the WHERE predicates.
	Predicates []PushdownPredicate
	// OrderBy holds the ORDER BY terms.
	OrderBy []PushdownOrder
	// Limit is the LIMIT value, honoured only when LimitSet is true.
	Limit int
	// LimitSet reports whether Limit is meaningful.
	LimitSet bool
	// Count requests a SELECT COUNT(*) pushdown.
	Count bool
}

// PushdownResult is the outcome of translating a PushdownIntent against an
// OperationStore's pushdown config. It is returned as an interface so no mutable
// data-carrier struct leaks across the public boundary.
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

// PushdownConfigSource is the minimal surface ApplyPushdown needs to resolve the
// pushdown config (with the Method -> Resource -> Service -> ProviderService ->
// Provider inheritance already implemented internally). OperationStore satisfies
// it, so callers normally pass an OperationStore directly.
type PushdownConfigSource interface {
	GetQueryParamPushdown() (QueryParamPushdown, bool)
}

type standardPushdownResult struct {
	queryParams        map[string]string
	pushedPredicates   []PushdownPredicate
	residualPredicates []PushdownPredicate
	countResponseKey   string
}

func (r *standardPushdownResult) QueryParams() map[string]string { return r.queryParams }

func (r *standardPushdownResult) PushedPredicates() []PushdownPredicate { return r.pushedPredicates }

func (r *standardPushdownResult) ResidualPredicates() []PushdownPredicate {
	return r.residualPredicates
}

func (r *standardPushdownResult) CountResponseKey() string { return r.countResponseKey }

// ApplyPushdown translates a neutral PushdownIntent into the request query params
// supported by the supplied config source. Anything not reported supported by the
// config (unknown column, unsupported operator, missing sub-config, or a dialect
// this helper cannot render) is left for the caller: predicates land in
// ResidualPredicates, and projection/order-by/limit/count are simply not emitted.
// With no pushdown config at all the result is empty and every predicate is
// residual, preserving current behaviour.
func ApplyPushdown(src PushdownConfigSource, intent PushdownIntent) PushdownResult {
	res := &standardPushdownResult{queryParams: map[string]string{}}

	var qpp QueryParamPushdown
	if src != nil {
		qpp, _ = src.GetQueryParamPushdown()
	}
	if qpp == nil {
		// Absent config: no params, all predicates residual.
		res.residualPredicates = append(res.residualPredicates, intent.Predicates...)
		return res
	}

	applySelect(qpp, intent, res)
	applyFilter(qpp, intent, res)
	applyOrderBy(qpp, intent, res)
	applyTop(qpp, intent, res)
	applyCount(qpp, intent, res)

	return res
}

func applySelect(qpp QueryParamPushdown, intent PushdownIntent, res *standardPushdownResult) {
	if len(intent.Projection) == 0 {
		return
	}
	sel, ok := qpp.GetSelect()
	if !ok {
		return
	}
	// All-or-nothing: pushing a partial projection would silently drop columns
	// the caller still needs, so emit $select only when every column is supported.
	for _, col := range intent.Projection {
		if !sel.IsColumnSupported(col) {
			return
		}
	}
	paramName := sel.GetParamName()
	if paramName == "" {
		return
	}
	res.queryParams[paramName] = strings.Join(intent.Projection, sel.GetDelimiter())
}

func applyFilter(qpp QueryParamPushdown, intent PushdownIntent, res *standardPushdownResult) {
	if len(intent.Predicates) == 0 {
		return
	}
	fil, ok := qpp.GetFilter()
	if !ok {
		// No filter config: everything residual.
		res.residualPredicates = append(res.residualPredicates, intent.Predicates...)
		return
	}

	odataSyntax := strings.EqualFold(fil.GetSyntax(), dialectODataSyntax)
	paramName := fil.GetParamName()

	var pushable []PushdownPredicate
	for _, p := range intent.Predicates {
		odataOp := normalizeFilterOperator(p.Operator)
		// Only OData-syntax filters can be rendered here; a column/operator must be
		// supported, and the operator must be one we know how to emit.
		if odataSyntax && paramName != "" && odataOp != "" &&
			fil.IsColumnSupported(p.Column) && fil.IsOperatorSupported(odataOp) {
			pushable = append(pushable, p)
		} else {
			res.residualPredicates = append(res.residualPredicates, p)
		}
	}

	if len(pushable) == 0 {
		return
	}
	// Combining predicates needs the OData "and" operator. If the config does not
	// allow it, only the first predicate is pushed and the rest stay residual.
	if len(pushable) > 1 && !fil.IsOperatorSupported("and") {
		res.residualPredicates = append(res.residualPredicates, pushable[1:]...)
		pushable = pushable[:1]
	}

	parts := make([]string, 0, len(pushable))
	for _, p := range pushable {
		parts = append(parts, buildODataFilterTerm(p))
	}
	res.queryParams[paramName] = strings.Join(parts, " and ")
	res.pushedPredicates = append(res.pushedPredicates, pushable...)
}

func applyOrderBy(qpp QueryParamPushdown, intent PushdownIntent, res *standardPushdownResult) {
	if len(intent.OrderBy) == 0 {
		return
	}
	ob, ok := qpp.GetOrderBy()
	if !ok {
		return
	}
	// Only the OData syntax has a well-defined "col asc|desc" rendering here.
	if !strings.EqualFold(ob.GetSyntax(), dialectODataSyntax) {
		return
	}
	paramName := ob.GetParamName()
	if paramName == "" {
		return
	}
	// All-or-nothing: a partial ordering would mis-order results.
	for _, o := range intent.OrderBy {
		if !ob.IsColumnSupported(o.Column) {
			return
		}
	}
	parts := make([]string, 0, len(intent.OrderBy))
	for _, o := range intent.OrderBy {
		dir := "asc"
		if o.Descending {
			dir = "desc"
		}
		parts = append(parts, o.Column+" "+dir)
	}
	res.queryParams[paramName] = strings.Join(parts, ",")
}

func applyTop(qpp QueryParamPushdown, intent PushdownIntent, res *standardPushdownResult) {
	if !intent.LimitSet {
		return
	}
	tp, ok := qpp.GetTop()
	if !ok {
		return
	}
	paramName := tp.GetParamName()
	if paramName == "" {
		return
	}
	v := intent.Limit
	if v < 0 {
		return
	}
	if maxV := tp.GetMaxValue(); maxV > 0 && v > maxV {
		v = maxV
	}
	res.queryParams[paramName] = strconv.Itoa(v)
}

func applyCount(qpp QueryParamPushdown, intent PushdownIntent, res *standardPushdownResult) {
	if !intent.Count {
		return
	}
	cp, ok := qpp.GetCount()
	if !ok {
		return
	}
	paramName := cp.GetParamName()
	if paramName == "" {
		return
	}
	res.queryParams[paramName] = cp.GetParamValue()
	res.countResponseKey = cp.GetResponseKey()
}

// buildODataFilterTerm renders one supported predicate as an OData $filter term.
func buildODataFilterTerm(p PushdownPredicate) string {
	op := normalizeFilterOperator(p.Operator)
	switch op {
	case "startswith", "endswith", "contains":
		return fmt.Sprintf("%s(%s,%s)", op, p.Column, formatODataValue(p.Value))
	default: // eq, ne, gt, ge, lt, le
		return fmt.Sprintf("%s %s %s", p.Column, op, formatODataValue(p.Value))
	}
}

// normalizeFilterOperator maps a neutral operator (SQL symbol or OData name) to
// its canonical OData logical name, or "" if it is not translatable.
func normalizeFilterOperator(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "eq", "=", "==":
		return "eq"
	case "ne", "!=", "<>":
		return "ne"
	case "gt", ">":
		return "gt"
	case "ge", ">=":
		return "ge"
	case "lt", "<":
		return "lt"
	case "le", "<=":
		return "le"
	case "startswith":
		return "startswith"
	case "endswith":
		return "endswith"
	case "contains":
		return "contains"
	default:
		return ""
	}
}

// formatODataValue renders a comparison value using OData literal conventions:
// strings are single-quoted (embedded quotes doubled), bools/numbers are bare.
func formatODataValue(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return "'" + strings.ReplaceAll(t, "'", "''") + "'"
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", t)
	}
}
