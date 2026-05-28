package anysdk_test

import (
	"testing"

	. "github.com/stackql/any-sdk/internal/anysdk"
	"gopkg.in/yaml.v3"

	"gotest.tools/assert"
)

// TestPaginationPageNumberYAMLRoundTrip exercises the same YAML shape proposed
// in issue #91 (algorithm at pagination level, current page in responseToken,
// total page count in responseTerminator) and verifies the public Pagination
// interface surfaces each field.
func TestPaginationPageNumberYAMLRoundTrip(t *testing.T) {
	input := `
algorithm: page_number
requestToken:
  key: page
  location: query
responseToken:
  key: result_info.page
  location: body
responseTerminator:
  key: result_info.total_pages
  location: body
`
	pag := GetTestingPagination()
	if err := yaml.Unmarshal([]byte(input), &pag); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	assert.Equal(t, pag.GetAlgorithm(), PaginationAlgorithmPageNumber)

	req := pag.GetRequestToken()
	assert.Assert(t, req != nil, "expected requestToken to be non-nil")
	assert.Equal(t, req.GetKey(), "page")
	assert.Equal(t, req.GetLocation(), "query")

	resp := pag.GetResponseToken()
	assert.Assert(t, resp != nil, "expected responseToken to be non-nil")
	assert.Equal(t, resp.GetKey(), "result_info.page")
	assert.Equal(t, resp.GetLocation(), "body")

	term := pag.GetResponseTerminator()
	assert.Assert(t, term != nil, "expected responseTerminator to be non-nil")
	assert.Equal(t, term.GetKey(), "result_info.total_pages")
	assert.Equal(t, term.GetLocation(), "body")
}

// TestPaginationLegacyShapeNoAlgorithm verifies the existing token-based
// configuration still unmarshals cleanly when no algorithm or
// responseTerminator is supplied. Guards against breakage of the existing
// token / offset / link strategies.
func TestPaginationLegacyShapeNoAlgorithm(t *testing.T) {
	input := `
requestToken:
  key: pageToken
  location: query
responseToken:
  key: nextPageToken
  location: body
`
	pag := GetTestingPagination()
	if err := yaml.Unmarshal([]byte(input), &pag); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	assert.Equal(t, pag.GetAlgorithm(), "")
	req := pag.GetRequestToken()
	assert.Assert(t, req != nil)
	assert.Equal(t, req.GetKey(), "pageToken")
	resp := pag.GetResponseToken()
	assert.Assert(t, resp != nil)
	assert.Equal(t, resp.GetKey(), "nextPageToken")
	assert.Assert(t, pag.GetResponseTerminator() == nil, "expected no responseTerminator")
}

// TestPaginationAlgorithmConstant pins the exported algorithm identifier;
// provider YAML and the invoker switch match on this exact string.
func TestPaginationAlgorithmConstant(t *testing.T) {
	assert.Equal(t, PaginationAlgorithmPageNumber, "page_number")
}
