package formulation

import (
	"net/http"
	"net/url"
	"testing"
)

// fakeQueryParam satisfies queryParamSettable, backed by a real *http.Request so we
// can assert the resulting query string.
type fakeQueryParam struct {
	req *http.Request
}

func (f *fakeQueryParam) GetQuery() url.Values { return f.req.URL.Query() }

func (f *fakeQueryParam) SetRawQuery(q string) { f.req.URL.RawQuery = q }

func TestApplyQueryParams_SetsAndPreserves(t *testing.T) {
	req, err := http.NewRequest("GET", "https://example.com/api?existing=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	applyQueryParams([]queryParamSettable{&fakeQueryParam{req: req}}, map[string]string{
		"$top":    "10",
		"$filter": "startswith(name,'A')",
	})
	q := req.URL.Query()
	if q.Get("$top") != "10" {
		t.Errorf("$top = %q, want 10", q.Get("$top"))
	}
	if q.Get("$filter") != "startswith(name,'A')" {
		t.Errorf("$filter = %q, want startswith(name,'A')", q.Get("$filter"))
	}
	if q.Get("existing") != "1" {
		t.Errorf("existing param not preserved: %q", q.Get("existing"))
	}
}

func TestApplyQueryParams_EmptyIsByteForByteNoOp(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://example.com/api?a=1&b=2", nil)
	before := req.URL.RawQuery
	applyQueryParams([]queryParamSettable{&fakeQueryParam{req: req}}, map[string]string{})
	if req.URL.RawQuery != before {
		t.Fatalf("RawQuery changed on empty params: %q -> %q", before, req.URL.RawQuery)
	}
}

func TestApplyPushdown_IntentToRequestQuery(t *testing.T) {
	src := fakeConfigSource{qpp: buildPushdown(t, odataFullPushdownYaml)}
	res := ApplyPushdown(src, PushdownIntent{
		Predicates: []PushdownPredicate{{Column: "status", Operator: "eq", Value: "active"}},
		Limit:      10,
		LimitSet:   true,
	})
	req, _ := http.NewRequest("GET", "https://example.com/api", nil)
	applyQueryParams([]queryParamSettable{&fakeQueryParam{req: req}}, res.QueryParams())
	q := req.URL.Query()
	if q.Get("$filter") != "status eq 'active'" {
		t.Errorf("$filter = %q, want status eq 'active'", q.Get("$filter"))
	}
	if q.Get("$top") != "10" {
		t.Errorf("$top = %q, want 10", q.Get("$top"))
	}
}

func TestWithPushdownIntent_CloneSemantics(t *testing.T) {
	base := &wrappedHTTPPreparator{}
	withIntent := base.WithPushdownIntent(PushdownIntent{Limit: 5, LimitSet: true})
	if base.pushdownIntent != nil {
		t.Fatalf("base preparator was mutated by WithPushdownIntent")
	}
	wp, ok := withIntent.(*wrappedHTTPPreparator)
	if !ok {
		t.Fatalf("unexpected type %T", withIntent)
	}
	if wp.pushdownIntent == nil || !wp.pushdownIntent.LimitSet || wp.pushdownIntent.Limit != 5 {
		t.Fatalf("intent not set on clone: %+v", wp.pushdownIntent)
	}
}
