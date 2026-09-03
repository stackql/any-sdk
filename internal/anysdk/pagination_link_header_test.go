package anysdk

import (
	"net/http"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stackql/any-sdk/pkg/internaldto"
	"github.com/stackql/any-sdk/pkg/response"
)

const (
	linkHeaderTestValue = `<https://api.example.com/items?page=2>; rel="next", <https://api.example.com/items?page=5>; rel="last"`
	linkHeaderTestNext  = "https://api.example.com/items?page=2"
)

func TestPaginationLinkHeaderNextConstant(t *testing.T) {
	if PaginationAlgorithmLinkHeaderNext != "link_header_next" {
		t.Fatalf("unexpected identifier %q", PaginationAlgorithmLinkHeaderNext)
	}
}

func TestPaginationAbsentTokensAreUntypedNil(t *testing.T) {
	pag := GetTestingPagination()
	if err := yaml.Unmarshal([]byte("responseToken:\n  key: link\n  location: header\n"), &pag); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if pag.GetRequestToken() != nil {
		t.Fatalf("expected nil request token, got %#v", pag.GetRequestToken())
	}
	if pag.GetResponseToken() == nil {
		t.Fatal("expected a response token")
	}
}

func TestIsLinkHeaderTokenSemantic(t *testing.T) {
	cases := []struct {
		name string
		ts   *standardTokenSemantic
		want bool
	}{
		{"Link header", &standardTokenSemantic{Key: "Link", Location: "header"}, true},
		{"lowercase link header", &standardTokenSemantic{Key: "link", Location: "header"}, true},
		{"algorithm on another header", &standardTokenSemantic{Key: "X-Next", Location: "header", Algorithm: PaginationAlgorithmLinkHeaderNext}, true},
		{"body token", &standardTokenSemantic{Key: "nextPageToken", Location: "body"}, false},
		{"link in body", &standardTokenSemantic{Key: "link", Location: "body"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLinkHeaderTokenSemantic(tc.ts); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	if isLinkHeaderTokenSemantic(nil) {
		t.Fatal("nil token must not be treated as link header")
	}
}

func TestHeaderTransformerAcceptsHeaderSliceAndString(t *testing.T) {
	ts := &standardTokenSemantic{Key: "Link", Location: "header", Algorithm: PaginationAlgorithmLinkHeaderNext}
	tf, err := ts.GetTransformer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := http.Header{}
	h.Add("Link", linkHeaderTestValue)
	for _, in := range []interface{}{h, []string{linkHeaderTestValue}, linkHeaderTestValue} {
		got, tfErr := tf(in)
		if tfErr != nil {
			t.Fatalf("unexpected error for %T: %v", in, tfErr)
		}
		if got != linkHeaderTestNext {
			t.Fatalf("for %T got %q, want %q", in, got, linkHeaderTestNext)
		}
	}
	if got, tfErr := tf(http.Header{}); tfErr != nil || got != "" {
		t.Fatalf("expected empty token without a Link header, got %q, err %v", got, tfErr)
	}
	if _, tfErr := tf(42); tfErr == nil {
		t.Fatal("expected an error for an unsupported input type")
	}
}

func TestTokenSemanticProcessedTokenFromLinkHeader(t *testing.T) {
	ts := &standardTokenSemantic{Key: "Link", Location: "header"}
	h := http.Header{}
	h.Add("Link", linkHeaderTestValue)
	h.Set("Content-Type", "application/json")
	got, err := ts.GetProcessedToken(response.NewResponse(nil, nil, &http.Response{Header: h}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != linkHeaderTestNext {
		t.Fatalf("got %q, want %q", got, linkHeaderTestNext)
	}
}

func linkHeaderTestOpStore(pag *standardPagination) *standardOpenAPIOperationStore {
	return &standardOpenAPIOperationStore{StackQLConfig: &standardStackQLConfig{Pagination: pag}}
}

func TestLinkHeaderPaginationInfersRequestURLToken(t *testing.T) {
	cases := []struct {
		name string
		pag  *standardPagination
	}{
		{"responseToken only", &standardPagination{
			ResponseToken: &standardTokenSemantic{Key: "link", Location: "header"}}},
		{"query requestToken is superseded", &standardPagination{
			RequestToken:  &standardTokenSemantic{Key: "fromName", Location: "query"},
			ResponseToken: &standardTokenSemantic{Key: "Link", Location: "header"}}},
		{"algorithm on the pagination block", &standardPagination{
			Algorithm:     PaginationAlgorithmLinkHeaderNext,
			ResponseToken: &standardTokenSemantic{Key: "X-Next", Location: "header"}}},
		{"algorithm on the response token", &standardPagination{
			RequestToken:  &standardTokenSemantic{Key: "page", Location: "query"},
			ResponseToken: &standardTokenSemantic{Key: "Link", Location: "header", Algorithm: PaginationAlgorithmLinkHeaderNext}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, ok := linkHeaderTestOpStore(tc.pag).GetPaginationRequestTokenSemantic()
			if !ok {
				t.Fatal("expected a request token semantic")
			}
			if ts.GetLocation() != internaldto.RequestStringStr {
				t.Fatalf("got location %q, want %q", ts.GetLocation(), internaldto.RequestStringStr)
			}
		})
	}
}

func TestTokenPaginationRequestTokenIsUnchanged(t *testing.T) {
	op := linkHeaderTestOpStore(&standardPagination{
		RequestToken:  &standardTokenSemantic{Key: "pageToken", Location: "query"},
		ResponseToken: &standardTokenSemantic{Key: "nextPageToken", Location: "body"},
	})
	ts, ok := op.GetPaginationRequestTokenSemantic()
	if !ok || ts.GetKey() != "pageToken" || ts.GetLocation() != "query" {
		t.Fatalf("expected the configured token, got %#v, ok %v", ts, ok)
	}
	op = linkHeaderTestOpStore(&standardPagination{
		ResponseToken: &standardTokenSemantic{Key: "nextPageToken", Location: "body"},
	})
	if _, ok := op.GetPaginationRequestTokenSemantic(); ok {
		t.Fatal("expected no request token for a body token without a request token")
	}
}

func TestProviderLevelPaginationTokensAreResolved(t *testing.T) {
	op := &standardOpenAPIOperationStore{Provider: &standardProvider{StackQLConfig: &standardStackQLConfig{
		Pagination: &standardPagination{
			RequestToken:  &standardTokenSemantic{Key: "pageToken", Location: "query"},
			ResponseToken: &standardTokenSemantic{Key: "nextPageToken", Location: "body"},
		}}}}
	req, ok := op.GetPaginationRequestTokenSemantic()
	if !ok || req.GetKey() != "pageToken" {
		t.Fatalf("expected provider-level request token, got %#v, ok %v", req, ok)
	}
	resp, ok := op.GetPaginationResponseTokenSemantic()
	if !ok || resp.GetKey() != "nextPageToken" {
		t.Fatalf("expected provider-level response token, got %#v, ok %v", resp, ok)
	}
}
