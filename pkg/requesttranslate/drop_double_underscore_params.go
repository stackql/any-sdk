package requesttranslate

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func NewDropDoubleUnderscoreParamsTranslator(byteEncoding string) RequestTranslator {
	return &DropDoubleUnderscoreParamsTranslator{
		byteEncoding: byteEncoding,
	}
}

type DropDoubleUnderscoreParamsTranslator struct {
	byteEncoding string
}

func (gp *DropDoubleUnderscoreParamsTranslator) Translate(req *http.Request) (*http.Request, error) {
	rv := req.Clone(req.Context())
	if req.URL == nil {
		return nil, fmt.Errorf("cannot translate nil URL")
	}
	parameters := url.Values{}

	for k, v := range req.URL.Query() {
		if !strings.HasPrefix(k, "__") {
			for _, inVal := range v {
				parameters.Add(k, inVal)
			}
		}
	}
	rv.URL.RawQuery = parameters.Encode()
	return rv, nil
}
