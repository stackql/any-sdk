package anysdk

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-openapi/jsonpointer"

	"github.com/stackql/any-sdk/pkg/internaldto"
)

var (
	linksNextRegex *regexp.Regexp            = regexp.MustCompile(`.*<(?P<nextURL>[^>]*)>;\ rel="next".*`)
	_              Pagination                = &standardPagination{}
	_              jsonpointer.JSONPointable = standardPagination{}
)

const (
	// PaginationAlgorithmPageNumber identifies the page-number + total-pages
	// pagination strategy: termination is by comparing the current page number
	// in the response against a page-count field (`responseTerminator`).
	PaginationAlgorithmPageNumber = "page_number"
	// PaginationAlgorithmODataNextLink identifies the OData v4 follow-the-link
	// strategy: the `@odata.nextLink` value in the response body is used verbatim
	// as the next request URL, and traversal terminates when it is absent/empty.
	// Like page_number this is a registered identifier; the caller drives the loop
	// using the public Pagination / TokenSemantic accessors (responseToken keyed at
	// `@odata.nextLink`).
	PaginationAlgorithmODataNextLink = "odata_next_link"
	// PaginationAlgorithmLinkHeaderNext identifies RFC 5988 Link-header
	// pagination: the `rel="next"` URL replaces the request URL for the next
	// page. Inferred for a header-located `Link` response token; the identifier
	// makes it explicit or names a different header.
	PaginationAlgorithmLinkHeaderNext = "link_header_next"
)

type Pagination interface {
	JSONLookup(token string) (interface{}, error)
	GetAlgorithm() string
	GetRequestToken() TokenSemantic
	GetResponseToken() TokenSemantic
	GetResponseTerminator() TokenSemantic
}

type standardPagination struct {
	Algorithm          string                 `json:"algorithm,omitempty" yaml:"algorithm,omitempty"`
	RequestToken       *standardTokenSemantic `json:"requestToken,omitempty" yaml:"requestToken,omitempty"`
	ResponseToken      *standardTokenSemantic `json:"responseToken,omitempty" yaml:"responseToken,omitempty"`
	ResponseTerminator *standardTokenSemantic `json:"responseTerminator,omitempty" yaml:"responseTerminator,omitempty"`
}

func (qt *standardPagination) GetAlgorithm() string {
	return qt.Algorithm
}

func (qt *standardPagination) GetRequestToken() TokenSemantic {
	if qt.RequestToken == nil {
		return nil
	}
	return qt.RequestToken
}

func (qt *standardPagination) GetResponseToken() TokenSemantic {
	if qt.ResponseToken == nil {
		return nil
	}
	return qt.ResponseToken
}

func (qt *standardPagination) GetResponseTerminator() TokenSemantic {
	if qt.ResponseTerminator == nil {
		return nil
	}
	return qt.ResponseTerminator
}

func (qt standardPagination) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "algorithm":
		return qt.Algorithm, nil
	case "requestToken":
		return qt.RequestToken, nil
	case "responseToken":
		return qt.ResponseToken, nil
	case "responseTerminator":
		return qt.ResponseTerminator, nil
	default:
		return nil, fmt.Errorf("could not resolve token '%s' from QueryTranspose doc object", token)
	}
}

// GetTestingPagination returns a zero-value Pagination for testing.
// Mirrors the GetTestingQueryParamPushdown helper convention.
func GetTestingPagination() standardPagination {
	return standardPagination{}
}

type TokenTransformer func(interface{}) (interface{}, error)

type TransformerLocator interface {
	GetTransformer(tokenSemantic TokenSemantic) (TokenTransformer, error)
}

type StandardTransformerLocator struct{}

func NewStandardTransformerLocator() TransformerLocator {
	return &StandardTransformerLocator{}
}

func (stl *StandardTransformerLocator) GetTransformer(tokenSemantic TokenSemantic) (TokenTransformer, error) {
	switch strings.ToLower(tokenSemantic.GetLocation()) {
	case "header":
		return getHeaderTransformer(tokenSemantic)
	default:
		return getNopTransformer()
	}
}

func getNopTransformer() (TokenTransformer, error) {
	return func(input interface{}) (interface{}, error) {
		return input, nil
	}, nil
}

func getHeaderTransformer(tokenSemantic TokenSemantic) (TokenTransformer, error) {
	if isLinkHeaderTokenSemantic(tokenSemantic) {
		return defaultLinkHeaderTransformer, nil
	}
	key := tokenSemantic.GetKey()
	return func(input interface{}) (interface{}, error) {
		return extractNextLink(input, key)
	}, nil
}

// isLinkHeaderTokenSemantic reports whether a token names the RFC 5988 Link
// header, by algorithm or by location and key.
func isLinkHeaderTokenSemantic(ts TokenSemantic) bool {
	if ts == nil {
		return false
	}
	if ts.GetAlgorithm() == PaginationAlgorithmLinkHeaderNext {
		return true
	}
	return strings.EqualFold(ts.GetLocation(), internaldto.HeaderStr) && strings.EqualFold(ts.GetKey(), "link")
}

// newRequestURLTokenSemantic describes a request token that replaces the whole
// request URL, as link-header pagination requires.
func newRequestURLTokenSemantic() TokenSemantic {
	return &standardTokenSemantic{Location: internaldto.RequestStringStr}
}

func DefaultLinkHeaderTransformer(input interface{}) (interface{}, error) {
	return defaultLinkHeaderTransformer(input)
}

func defaultLinkHeaderTransformer(input interface{}) (interface{}, error) {
	return extractNextLink(input, "Link")
}

// extractNextLink returns the rel="next" URL from a header supplied as an
// http.Header, its value slice, or a single value; "" when absent.
func extractNextLink(input interface{}, key string) (interface{}, error) {
	var vals []string
	switch h := input.(type) {
	case http.Header:
		vals = h.Values(key)
	case []string:
		vals = h
	case string:
		vals = []string{h}
	default:
		return nil, fmt.Errorf("cannot ingest purported http header of type = '%T'", input)
	}
	resArr := linksNextRegex.FindStringSubmatch(strings.Join(vals, ","))
	if len(resArr) == 2 {
		return resArr[1], nil
	}
	return "", nil
}
