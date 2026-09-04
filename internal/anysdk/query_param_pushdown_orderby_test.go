package anysdk

import (
	"testing"
)

func orderByTestIntent(orderBy ...PushdownOrder) PushdownIntent {
	return NewPushdownIntent(nil, nil, orderBy, 0, false, 0, false, false)
}

func orderByTestAssertNothingPushed(t *testing.T, res PushdownResult) {
	t.Helper()
	if len(res.QueryParams()) != 0 {
		t.Fatalf("expected no query params, got %v", res.QueryParams())
	}
}

const pushdownOrderByDirectionOnlyYaml = `
orderBy:
  paramName: order
  syntax: direction_only
  supportedColumns:
    - created_at
`

func TestApplyPushdown_OrderByDirectionOnly(t *testing.T) {
	src := applyTestSource{qpp: applyTestBuildPushdown(t, pushdownOrderByDirectionOnlyYaml)}
	rendered := map[string]struct {
		order PushdownOrder
		want  string
	}{
		"desc": {NewPushdownOrder("created_at", true), "desc"},
		"asc":  {NewPushdownOrder("created_at", false), "asc"},
	}
	for name, tc := range rendered {
		t.Run(name, func(t *testing.T) {
			res := ApplyPushdown(src, orderByTestIntent(tc.order))
			applyTestAssertParam(t, res.QueryParams(), "order", tc.want)
			if len(res.QueryParams()) != 1 {
				t.Fatalf("expected only the order param, got %v", res.QueryParams())
			}
		})
	}
	refused := map[string][]PushdownOrder{
		"unsupported column": {NewPushdownOrder("name", true)},
		"multiple terms":     {NewPushdownOrder("created_at", true), NewPushdownOrder("name", false)},
	}
	for name, orderBy := range refused {
		t.Run(name, func(t *testing.T) {
			orderByTestAssertNothingPushed(t, ApplyPushdown(src, orderByTestIntent(orderBy...)))
		})
	}
}

func TestApplyPushdown_OrderByDirectionOnlyRequiresAllowlist(t *testing.T) {
	src := applyTestSource{qpp: applyTestBuildPushdown(t, `
orderBy:
  paramName: order
  syntax: direction_only
`)}
	orderByTestAssertNothingPushed(t, ApplyPushdown(src, orderByTestIntent(NewPushdownOrder("created_at", true))))
}

func TestApplyPushdown_OrderByCustomSyntaxes(t *testing.T) {
	orderBy := []PushdownOrder{NewPushdownOrder("created_at", true), NewPushdownOrder("name", false)}
	cases := map[string]string{
		OrderBySyntaxPrefix:     "-created_at,name",
		OrderBySyntaxSuffix:     "created_at:desc,name:asc",
		OrderBySyntaxColumnOnly: "created_at,name",
	}
	for syntax, want := range cases {
		t.Run(syntax, func(t *testing.T) {
			src := applyTestSource{qpp: applyTestBuildPushdown(t, "orderBy:\n  paramName: sort\n  syntax: "+syntax+"\n")}
			res := ApplyPushdown(src, orderByTestIntent(orderBy...))
			applyTestAssertParam(t, res.QueryParams(), "sort", want)
		})
	}
}

func TestApplyPushdown_OrderByUnknownSyntaxEmitsNothing(t *testing.T) {
	src := applyTestSource{qpp: applyTestBuildPushdown(t, "orderBy:\n  paramName: sort\n  syntax: bespoke\n")}
	orderByTestAssertNothingPushed(t, ApplyPushdown(src, orderByTestIntent(NewPushdownOrder("created_at", true))))
}

func TestApplyPushdown_OrderByODataUnchanged(t *testing.T) {
	src := applyTestSource{qpp: applyTestBuildPushdown(t, "orderBy:\n  dialect: odata\n")}
	res := ApplyPushdown(src, orderByTestIntent(NewPushdownOrder("created_at", true), NewPushdownOrder("name", false)))
	applyTestAssertParam(t, res.QueryParams(), "$orderby", "created_at desc,name asc")
}
