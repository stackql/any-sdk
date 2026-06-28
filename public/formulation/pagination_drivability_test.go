package formulation

import (
	"fmt"
	"testing"

	"github.com/stackql/any-sdk/internal/anysdk"
	"gopkg.in/yaml.v3"
)

// These tests confirm the public surface exposes everything stackql needs to
// drive the pagination loop itself: the page_number algorithm (request increment
// key + start value + total_pages terminator), the OData @odata.nextLink
// response token, and the $skip offset request token.

func wrapToken(ts anysdk.TokenSemantic) TokenSemantic {
	if ts == nil {
		return nil
	}
	return &wrappedTokenSemantic{inner: ts}
}

func TestPagination_PageNumberDrivable(t *testing.T) {
	const pageNumberYaml = `
algorithm: page_number
requestToken:
  key: page
  location: query
  algorithm: page_number
  args:
    initialValue: 1
responseToken:
  key: result_info.page
  location: body
responseTerminator:
  key: result_info.total_pages
  location: body
`
	pg := anysdk.GetTestingPagination()
	if err := yaml.Unmarshal([]byte(pageNumberYaml), &pg); err != nil {
		t.Fatalf("failed to unmarshal pagination config: %v", err)
	}

	if pg.GetAlgorithm() != "page_number" {
		t.Fatalf("algorithm = %q, want page_number", pg.GetAlgorithm())
	}

	// Request increment: which query param holds the page number + start value.
	req := wrapToken(pg.GetRequestToken())
	if req.GetKey() != "page" {
		t.Fatalf("request key = %q, want page", req.GetKey())
	}
	if req.GetLocation() != "query" {
		t.Fatalf("request location = %q, want query", req.GetLocation())
	}
	if req.GetAlgorithm() != "page_number" {
		t.Fatalf("request algorithm = %q, want page_number", req.GetAlgorithm())
	}
	if got := fmt.Sprintf("%v", req.GetArgs()["initialValue"]); got != "1" {
		t.Fatalf("request args[initialValue] = %v, want 1", got)
	}

	// Terminator: stop when result_info.page >= result_info.total_pages.
	term := wrapToken(pg.GetResponseTerminator())
	if term.GetKey() != "result_info.total_pages" {
		t.Fatalf("terminator key = %q, want result_info.total_pages", term.GetKey())
	}
}

func TestPagination_ODataNextLinkDrivable(t *testing.T) {
	const nextLinkYaml = `
algorithm: odata_next_link
responseToken:
  key: "@odata.nextLink"
  location: body
`
	pg := anysdk.GetTestingPagination()
	if err := yaml.Unmarshal([]byte(nextLinkYaml), &pg); err != nil {
		t.Fatalf("failed to unmarshal pagination config: %v", err)
	}

	if pg.GetAlgorithm() != anysdk.PaginationAlgorithmODataNextLink {
		t.Fatalf("algorithm = %q, want %q", pg.GetAlgorithm(), anysdk.PaginationAlgorithmODataNextLink)
	}

	resp := wrapToken(pg.GetResponseToken())
	if resp.GetKey() != "@odata.nextLink" {
		t.Fatalf("response token key = %q, want @odata.nextLink", resp.GetKey())
	}
	if resp.GetLocation() != "body" {
		t.Fatalf("response token location = %q, want body", resp.GetLocation())
	}
}

func TestPagination_SkipOffsetDrivable(t *testing.T) {
	const skipYaml = `
algorithm: offset
requestToken:
  key: $skip
  location: query
  algorithm: offset
  args:
    initialValue: 0
`
	pg := anysdk.GetTestingPagination()
	if err := yaml.Unmarshal([]byte(skipYaml), &pg); err != nil {
		t.Fatalf("failed to unmarshal pagination config: %v", err)
	}

	req := wrapToken(pg.GetRequestToken())
	if req.GetKey() != "$skip" {
		t.Fatalf("request key = %q, want $skip", req.GetKey())
	}
	if req.GetAlgorithm() != "offset" {
		t.Fatalf("request algorithm = %q, want offset", req.GetAlgorithm())
	}
	if got := fmt.Sprintf("%v", req.GetArgs()["initialValue"]); got != "0" {
		t.Fatalf("request args[initialValue] = %v, want 0", got)
	}
}
