package graphql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"

	"github.com/stackql/any-sdk/pkg/jsonpath"
)

var (
	_ template.ExecError = template.ExecError{}
)

func NewStandardGQLReader(
	httpClient *http.Client,
	request *http.Request,
	httpPageLimit int,
	baseQuery string,
	constInput map[string]interface{},
	initialCursor string,
	responseJsonPath string,
	latestCursorJsonPath string,
) (GQLReader, error) {
	tmpl, err := template.New("gqlTmpl").Parse(baseQuery)
	if err != nil {
		return nil, err
	}
	rv := &StandardGQLReader{
		httpClient:           httpClient,
		baseQuery:            baseQuery,
		httpPageLimit:        httpPageLimit,
		constInput:           constInput,
		latestCursorJsonPath: latestCursorJsonPath,
		responseJsonPath:     responseJsonPath,
		queryTemplate:        tmpl,
		request:              request,
		pageCount:            1,
		iterativeInput:       make(map[string]interface{}),
	}
	for k, v := range constInput {
		rv.iterativeInput[k] = v
	}
	rv.iterativeInput["cursor"] = initialCursor
	return rv, nil
}

type StandardGQLReader struct {
	baseQuery            string
	constInput           map[string]interface{}
	iterativeInput       map[string]interface{}
	httpClient           *http.Client
	httpPageLimit        int
	queryTemplate        *template.Template
	responseJsonPath     string
	latestCursorJsonPath string
	request              *http.Request
	pageCount            int
}

func (gq *StandardGQLReader) Read() ([]map[string]interface{}, error) {
	if gq.httpPageLimit > 0 && gq.pageCount >= gq.httpPageLimit {
		return nil, io.EOF
	}
	req := gq.request.Clone(gq.request.Context())
	rb, err := gq.renderQuery()
	if err != nil {
		return nil, err
	}
	req.Body = rb
	req.URL.RawQuery = ""
	req.Header.Set("Content-Type", "application/json")
	r, err := gq.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	gq.pageCount++
	var target map[string]interface{}
	err = json.NewDecoder(r.Body).Decode(&target)
	if err != nil {
		return nil, err
	}
	var returnErr error
	if len(target) == 0 {
		returnErr = io.EOF
	}
	cursorRaw, err := jsonpath.Get(gq.latestCursorJsonPath, target)
	if err != nil {
		returnErr = io.EOF
	} else {
		switch ct := cursorRaw.(type) {
		case []interface{}:
			if len(ct) == 1 {
				switch c := ct[0].(type) {
				case string:
					gq.iterativeInput["cursor"] = fmt.Sprintf(`, after: "%s"`, c)
				default:
					gq.iterativeInput["cursor"] = fmt.Sprintf(`, after: %v`, c)
				}
			} else {
				returnErr = io.EOF
			}
		default:
			returnErr = io.EOF
		}
	}
	processedResponse, err := jsonpath.Get(gq.responseJsonPath, target)
	if err != nil {
		return nil, err
	}
	switch pr := processedResponse.(type) {
	case []interface{}:
		var rv []map[string]interface{}
		for _, v := range pr {
			switch v := v.(type) {
			case map[string]interface{}:
				rv = append(rv, v)
			default:
				return nil, fmt.Errorf("cannot accomodate GraphQL pocessed response item of type = '%T'", v)
			}
		}
		return rv, returnErr
	default:
		return nil, fmt.Errorf("cannot accomodate GraphQL pocessed response of type = '%T'", pr)
	}
}

func (gq *StandardGQLReader) renderQuery() (io.ReadCloser, error) {
	var tplWr bytes.Buffer
	if err := gq.queryTemplate.Execute(&tplWr, gq.iterativeInput); err != nil {
		return nil, err
	}
	s := strings.ReplaceAll(tplWr.String(), "\n", "")
	payload := fmt.Sprintf(`{ "query": "%s" }`, strings.ReplaceAll(s, `"`, `\"`))
	return io.NopCloser(bytes.NewReader([]byte(payload))), nil
}
