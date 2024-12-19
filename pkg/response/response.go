package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/stackql/any-sdk/pkg/httpelement"
	"github.com/stackql/any-sdk/pkg/jsonpath"
	"github.com/stackql/any-sdk/pkg/media"
	"github.com/stackql/any-sdk/pkg/xmlmap"
)

var (
	_ Response = &basicResponse{}
)

type Response interface {
	GetHttpResponse() *http.Response
	GetBody() interface{}
	GetProcessedBody() interface{}
	ExtractElement(e httpelement.HTTPElement) (interface{}, error)
	Error() string
	HasError() bool
	SetError(string)
	String() string
}

type basicResponse struct {
	_                   struct{}
	rawBody             interface{}
	processedBody       interface{}
	httpResponse        *http.Response
	bodyMediaType       string
	errorStringOverride string
}

func (r *basicResponse) GetHttpResponse() *http.Response {
	return r.httpResponse
}

func (r *basicResponse) GetBody() interface{} {
	return r.rawBody
}

func (r *basicResponse) GetProcessedBody() interface{} {
	return r.processedBody
}

func (r *basicResponse) HasError() bool {
	return r.errorStringOverride != ""
}

func (r *basicResponse) String() string {
	return r.string()
}

func (r *basicResponse) string() string {
	var baseString string
	switch body := r.processedBody.(type) {
	case map[string]interface{}:
		b, err := json.Marshal(body)
		if err == nil {
			baseString = string(b)
		}
	case map[string]string:
		b, err := json.Marshal(body)
		if err == nil {
			baseString = string(b)
		}
	}
	if r.httpResponse != nil {
		if baseString != "" {
			return fmt.Sprintf(`{ "statusCode": %d, "body": %s  }`, r.httpResponse.StatusCode, baseString)
		}
	}
	if baseString != "" {
		return fmt.Sprintf(`{ "body": %s  }`, baseString)
	}
	return ""
}

func (r *basicResponse) SetError(err string) {
	r.errorStringOverride = err
}

func (r *basicResponse) Error() string {
	if r.errorStringOverride != "" {
		return r.errorStringOverride
	}
	baseString := r.string()
	if baseString != "" {
		return fmt.Sprintf(`{ "httpError": %s }`, baseString)
	}
	return `{ "httpError": { "message": "unknown error" } }`
}

func isSlice(v interface{}) bool {
	return reflect.TypeOf(v).Kind() == reflect.Slice
}

func (r *basicResponse) ExtractElement(e httpelement.HTTPElement) (interface{}, error) {
	elementLocation := e.GetLocation()
	rawSearchPath := e.GetName()
	switch elementLocation {
	case httpelement.BodyAttribute:
		// refactor heaps of shit here
		switch body := r.rawBody.(type) {
		case *xmlquery.Node:
			elem, err := xmlmap.GetSubObjFromNode(body, rawSearchPath)
			return elem, err
		default:
			// This is a guard for odd behaviour by the lib:
			//    a, err := jsonpath.Get(<array of maps>, myUnprefixedString)
			//    just returns input and no error.
			if isSlice(body) && (strings.HasPrefix(rawSearchPath, `$`) || strings.HasPrefix(rawSearchPath, `[`)) {
				return nil, fmt.Errorf("invalid json path '%s' for array type", rawSearchPath)
			}
			processedResponse, err := jsonpath.Get(e.GetName(), body)
			return processedResponse, err
		}
	case httpelement.Header:
		return r.httpResponse.Header.Values(rawSearchPath), nil
	default:
		return nil, fmt.Errorf("http element type '%v' not supported", elementLocation)
	}
}

func NewResponse(processedBody, rawBody interface{}, r *http.Response) Response {
	mt, _ := media.GetResponseMediaType(r, "")
	return &basicResponse{
		processedBody: processedBody,
		rawBody:       rawBody,
		httpResponse:  r,
		bodyMediaType: mt,
	}
}
