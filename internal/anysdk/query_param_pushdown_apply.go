package anysdk

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

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
// to push down to the upstream API. It carries no foreign (OData/custom) syntax;
// ApplyPushdown performs the dialect translation.
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

// PushdownResult is the outcome of translating a PushdownIntent against an
// OperationStore's pushdown config.
type PushdownResult interface {
	QueryParams() map[string]string
	PushedPredicates() []PushdownPredicate
	ResidualPredicates() []PushdownPredicate
	CountResponseKey() string
}

// pushdownConfigSource is the minimal surface ApplyPushdown needs to resolve the
// pushdown config (with the Method -> Resource -> Service -> ProviderService ->
// Provider inheritance already implemented internally). OperationStore satisfies it.
type pushdownConfigSource interface {
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
func ApplyPushdown(src pushdownConfigSource, intent PushdownIntent) PushdownResult {
	res := &standardPushdownResult{queryParams: map[string]string{}}

	var qpp QueryParamPushdown
	if src != nil {
		qpp, _ = src.GetQueryParamPushdown()
	}
	if qpp == nil {
		res.residualPredicates = append(res.residualPredicates, intent.Predicates...)
		return res
	}

	applyPushdownSelect(qpp, intent, res)
	applyPushdownFilter(qpp, intent, res)
	applyPushdownOrderBy(qpp, intent, res)
	applyPushdownTop(qpp, intent, res)
	applyPushdownSkip(qpp, intent, res)
	applyPushdownCount(qpp, intent, res)

	return res
}

func applyPushdownSelect(qpp QueryParamPushdown, intent PushdownIntent, res *standardPushdownResult) {
	if len(intent.Projection) == 0 {
		return
	}
	sel, ok := qpp.GetSelect()
	if !ok {
		return
	}
	// All-or-nothing: pushing a partial projection would silently drop columns the
	// caller still needs, so emit $select only when every column is supported.
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

func applyPushdownFilter(qpp QueryParamPushdown, intent PushdownIntent, res *standardPushdownResult) {
	if len(intent.Predicates) == 0 {
		return
	}
	fil, ok := qpp.GetFilter()
	if !ok {
		res.residualPredicates = append(res.residualPredicates, intent.Predicates...)
		return
	}

	odataSyntax := strings.EqualFold(fil.GetSyntax(), ODataDialect)
	paramName := fil.GetParamName()

	var pushable []PushdownPredicate
	for _, p := range intent.Predicates {
		odataOp := normalizePushdownFilterOperator(p.Operator)
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

func applyPushdownOrderBy(qpp QueryParamPushdown, intent PushdownIntent, res *standardPushdownResult) {
	if len(intent.OrderBy) == 0 {
		return
	}
	ob, ok := qpp.GetOrderBy()
	if !ok {
		return
	}
	// Only the OData syntax has a well-defined "col asc|desc" rendering here.
	if !strings.EqualFold(ob.GetSyntax(), ODataDialect) {
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

func applyPushdownTop(qpp QueryParamPushdown, intent PushdownIntent, res *standardPushdownResult) {
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

func applyPushdownSkip(qpp QueryParamPushdown, intent PushdownIntent, res *standardPushdownResult) {
	if !intent.OffsetSet {
		return
	}
	sp, ok := qpp.GetSkip()
	if !ok {
		return
	}
	paramName := sp.GetParamName()
	if paramName == "" {
		return
	}
	v := intent.Offset
	if v < 0 {
		return
	}
	if maxV := sp.GetMaxValue(); maxV > 0 && v > maxV {
		v = maxV
	}
	res.queryParams[paramName] = strconv.Itoa(v)
}

func applyPushdownCount(qpp QueryParamPushdown, intent PushdownIntent, res *standardPushdownResult) {
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
	op := normalizePushdownFilterOperator(p.Operator)
	switch op {
	case "startswith", "endswith", "contains":
		return fmt.Sprintf("%s(%s,%s)", op, p.Column, formatODataValue(p.Value))
	default: // eq, ne, gt, ge, lt, le
		return fmt.Sprintf("%s %s %s", p.Column, op, formatODataValue(p.Value))
	}
}

// normalizePushdownFilterOperator maps a neutral operator (SQL symbol or OData
// name) to its canonical OData logical name, or "" if it is not translatable.
func normalizePushdownFilterOperator(op string) string {
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

// queryParamSettable is the minimal surface needed to merge query params onto a
// built request. HTTPArmouryParameters satisfies it.
type queryParamSettable interface {
	GetQuery() url.Values
	SetRawQuery(string)
}

// setPushdownQueryParams merges qp onto each param's query string. It is a no-op
// when qp is empty, leaving the request byte-for-byte unchanged.
func setPushdownQueryParams(params []queryParamSettable, qp map[string]string) {
	if len(qp) == 0 {
		return
	}
	for _, p := range params {
		q := p.GetQuery()
		for k, v := range qp {
			q.Set(k, v)
		}
		p.SetRawQuery(q.Encode())
	}
}

// applyPushdownToArmoury translates the intent against the source's pushdown config
// and sets the resulting query params on every request in the armoury. Absent
// config (or no translatable options) yields zero params and leaves it untouched.
func applyPushdownToArmoury(armoury HTTPArmoury, src pushdownConfigSource, intent PushdownIntent) {
	qp := ApplyPushdown(src, intent).QueryParams()
	if len(qp) == 0 {
		return
	}
	inner := armoury.GetRequestParams()
	settable := make([]queryParamSettable, 0, len(inner))
	for _, p := range inner {
		settable = append(settable, p)
	}
	setPushdownQueryParams(settable, qp)
}
