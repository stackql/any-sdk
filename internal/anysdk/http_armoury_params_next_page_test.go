package anysdk

import (
	"net/http"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stackql/any-sdk/pkg/internaldto"
)

const (
	nextPageTestURL   = "https://api.example.com/v1/organization/costs?group_by=project_id&start_time=1781481600"
	nextPageTestToken = "page_AAAAAGpY30vJnSVZAAAAAGo4ewA="
)

func nextPageTestParams(t *testing.T, rawURL string) *standardHTTPArmouryParameters {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	return &standardHTTPArmouryParameters{request: req}
}

func nextPageTestOpStore(encoding string) *standardOpenAPIOperationStore {
	return &standardOpenAPIOperationStore{StackQLConfig: &standardStackQLConfig{Pagination: &standardPagination{
		RequestToken:  &standardTokenSemantic{Key: "page", Location: "query", Encoding: encoding},
		ResponseToken: &standardTokenSemantic{Key: "$.next_page", Location: "body"},
	}}}
}

func TestSetNextPageEscapesQueryTokenByDefault(t *testing.T) {
	cases := map[string]OperationStore{
		"no encoding key":      nextPageTestOpStore(""),
		"explicit url":         nextPageTestOpStore(TokenEncodingURL),
		"no pagination config": &standardOpenAPIOperationStore{},
	}
	want := "group_by=project_id&page=page_AAAAAGpY30vJnSVZAAAAAGo4ewA%3D&start_time=1781481600"
	for name, ops := range cases {
		t.Run(name, func(t *testing.T) {
			req, err := nextPageTestParams(t, nextPageTestURL).SetNextPage(
				ops, nextPageTestToken, internaldto.NewHTTPElement(internaldto.QueryParam, "page"))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.URL.RawQuery != want {
				t.Fatalf("got %q, want %q", req.URL.RawQuery, want)
			}
		})
	}
}

func TestSetNextPageWritesQueryTokenVerbatimWhenEncodingNone(t *testing.T) {
	ops := nextPageTestOpStore(TokenEncodingNone)
	elem := internaldto.NewHTTPElement(internaldto.QueryParam, "page")
	cases := []struct {
		name   string
		rawURL string
		token  string
		want   string
	}{
		{"base64 padding", nextPageTestURL, nextPageTestToken,
			"group_by=project_id&start_time=1781481600&page=page_AAAAAGpY30vJnSVZAAAAAGo4ewA="},
		{"plus slash equals", nextPageTestURL, "page_ab+cd/ef==xy",
			"group_by=project_id&start_time=1781481600&page=page_ab+cd/ef==xy"},
		{"previous token replaced, other params stay escaped", "https://api.example.com/items?page=old%3D&name=a+b", "new=",
			"name=a+b&page=new="},
		{"no other params", "https://api.example.com/items", "tok=", "page=tok="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := nextPageTestParams(t, tc.rawURL).SetNextPage(ops, tc.token, elem)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.URL.RawQuery != tc.want {
				t.Fatalf("got %q, want %q", req.URL.RawQuery, tc.want)
			}
		})
	}
}

func TestTokenSemanticEncodingDefaultsToURL(t *testing.T) {
	if got := (&standardTokenSemantic{}).GetEncoding(); got != TokenEncodingURL {
		t.Fatalf("default encoding = %q, want %q", got, TokenEncodingURL)
	}
	pag := GetTestingPagination()
	input := "requestToken:\n  key: page\n  location: query\n  encoding: none\nresponseToken:\n  key: $.next_page\n  location: body\n"
	if err := yaml.Unmarshal([]byte(input), &pag); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got := pag.GetRequestToken().GetEncoding(); got != TokenEncodingNone {
		t.Fatalf("request token encoding = %q, want %q", got, TokenEncodingNone)
	}
	if got := pag.GetResponseToken().GetEncoding(); got != TokenEncodingURL {
		t.Fatalf("response token encoding = %q, want %q", got, TokenEncodingURL)
	}
	v, err := pag.GetRequestToken().JSONLookup("encoding")
	if err != nil || v != TokenEncodingNone {
		t.Fatalf("JSONLookup(encoding) = %v, %v", v, err)
	}
}
