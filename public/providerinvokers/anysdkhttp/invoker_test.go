package anysdkhttp

import (
	"net/http"
	"testing"

	sdk_internal_dto "github.com/stackql/any-sdk/pkg/internaldto"
	"github.com/stackql/any-sdk/pkg/response"

	"gotest.tools/assert"
)

func makeBodyResponse(body map[string]interface{}) response.Response {
	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	return response.NewResponse(body, body, httpResp)
}

// TestExtractPageNumberNextToken_NotLast covers the common case: current page
// is below the terminator, so the next page (current+1) must be requested.
func TestExtractPageNumberNextToken_NotLast(t *testing.T) {
	body := map[string]interface{}{
		"page":        float64(1),
		"total_pages": float64(3),
	}
	res := makeBodyResponse(body)
	curr := sdk_internal_dto.NewHTTPElement(sdk_internal_dto.BodyAttribute, "$.page")
	total := sdk_internal_dto.NewHTTPElement(sdk_internal_dto.BodyAttribute, "$.total_pages")

	next, finished := extractPageNumberNextToken(res, curr, total)
	assert.Equal(t, finished, false)
	assert.Equal(t, next, "2")
}

// TestExtractPageNumberNextToken_LastPage covers Cloudflare's actual failure
// mode from issue #91: when page == total_pages the loop must terminate.
// Without the page_number algorithm, stackql instead re-requested page=1
// indefinitely because the response token never went empty.
func TestExtractPageNumberNextToken_LastPage(t *testing.T) {
	body := map[string]interface{}{
		"page":        float64(3),
		"total_pages": float64(3),
	}
	res := makeBodyResponse(body)
	curr := sdk_internal_dto.NewHTTPElement(sdk_internal_dto.BodyAttribute, "$.page")
	total := sdk_internal_dto.NewHTTPElement(sdk_internal_dto.BodyAttribute, "$.total_pages")

	next, finished := extractPageNumberNextToken(res, curr, total)
	assert.Equal(t, finished, true)
	assert.Equal(t, next, "")
}

// TestExtractPageNumberNextToken_SinglePage covers the single-row Cloudflare
// case in the issue reproducer (page==1, total_pages==1).
func TestExtractPageNumberNextToken_SinglePage(t *testing.T) {
	body := map[string]interface{}{
		"page":        float64(1),
		"total_pages": float64(1),
	}
	res := makeBodyResponse(body)
	curr := sdk_internal_dto.NewHTTPElement(sdk_internal_dto.BodyAttribute, "$.page")
	total := sdk_internal_dto.NewHTTPElement(sdk_internal_dto.BodyAttribute, "$.total_pages")

	next, finished := extractPageNumberNextToken(res, curr, total)
	assert.Equal(t, finished, true)
	assert.Equal(t, next, "")
}

// TestExtractPageNumberNextToken_MissingTerminator: if the terminator element
// is nil (misconfigured yaml) we must terminate rather than loop forever.
// Strictly safer than today's behaviour.
func TestExtractPageNumberNextToken_MissingTerminator(t *testing.T) {
	body := map[string]interface{}{
		"page":        float64(1),
		"total_pages": float64(3),
	}
	res := makeBodyResponse(body)
	curr := sdk_internal_dto.NewHTTPElement(sdk_internal_dto.BodyAttribute, "$.page")

	next, finished := extractPageNumberNextToken(res, curr, nil)
	assert.Equal(t, finished, true)
	assert.Equal(t, next, "")
}

// TestExtractPageNumberNextToken_Unparseable: if either field is missing or
// non-numeric, we terminate rather than spin.
func TestExtractPageNumberNextToken_Unparseable(t *testing.T) {
	body := map[string]interface{}{
		"page":        "notanumber",
		"total_pages": float64(3),
	}
	res := makeBodyResponse(body)
	curr := sdk_internal_dto.NewHTTPElement(sdk_internal_dto.BodyAttribute, "$.page")
	total := sdk_internal_dto.NewHTTPElement(sdk_internal_dto.BodyAttribute, "$.total_pages")

	next, finished := extractPageNumberNextToken(res, curr, total)
	assert.Equal(t, finished, true)
	assert.Equal(t, next, "")
}
