package requesttranslate

import (
	"net/http"
	"strings"
)

const (
	GetQueryToPostFormUTF8     string = "get_query_to_post_form_utf_8"
	DropDoubleUnderscoreParams string = "drop_double_underscore_params"
)

type RequestTranslator interface {
	Translate(*http.Request) (*http.Request, error)
}

func NewRequestTranslator(algorithm string) (RequestTranslator, error) {
	switch strings.ToLower(algorithm) {
	case GetQueryToPostFormUTF8:
		return NewGetQueryToPostFormEncodedTranslator("utf-8"), nil
	case DropDoubleUnderscoreParams:
		return NewDropDoubleUnderscoreParamsTranslator("utf-8"), nil
	default:
		return NewNilTranslator(), nil
	}
}
