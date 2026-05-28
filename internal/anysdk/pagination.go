package anysdk

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-openapi/jsonpointer"
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
	return qt.RequestToken
}

func (qt *standardPagination) GetResponseToken() TokenSemantic {
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
	if tokenSemantic.GetAlgorithm() == "" && strings.ToLower(tokenSemantic.GetKey()) == "link" && strings.ToLower(tokenSemantic.GetLocation()) == "header" {
		return defaultLinkHeaderTransformer, nil
	}

	return func(input interface{}) (interface{}, error) {
		h, ok := input.(http.Header)
		if !ok {
			return nil, fmt.Errorf("cannot ingest purported http header of type = '%T'", h)
		}
		s := h.Values(tokenSemantic.GetKey())
		resArr := linksNextRegex.FindStringSubmatch(strings.Join(s, ","))
		if len(resArr) == 2 {
			return resArr[1], nil
		}
		return "", nil
	}, nil
}

func DefaultLinkHeaderTransformer(input interface{}) (interface{}, error) {
	return defaultLinkHeaderTransformer(input)
}

func defaultLinkHeaderTransformer(input interface{}) (interface{}, error) {
	h, ok := input.(http.Header)
	if !ok {
		return nil, fmt.Errorf("cannot ingest purported http header of type = '%T'", h)
	}
	s := h.Values("Link")
	resArr := linksNextRegex.FindStringSubmatch(strings.Join(s, ","))
	if len(resArr) == 2 {
		return resArr[1], nil
	}
	return "", nil
}
