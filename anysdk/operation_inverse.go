package anysdk

import (
	"fmt"

	"github.com/go-openapi/jsonpointer"
	"github.com/stackql/any-sdk/pkg/response"
)

var (
	_ OperationInverse          = &operationInverse{}
	_ jsonpointer.JSONPointable = (OperationInverse)(&operationInverse{})
	_ OperationTokens           = operationTokens{}
	_ jsonpointer.JSONPointable = operationTokens{}
)

type OperationTokens interface {
	JSONLookup(token string) (interface{}, error)
	GetTokenSemantic(key string) (TokenSemantic, bool)
}

type operationTokens map[string]*standardTokenSemantic

func (oits operationTokens) JSONLookup(token string) (interface{}, error) {
	if tokenSemantic, ok := oits[token]; ok {
		return tokenSemantic, nil
	}
	return nil, fmt.Errorf("could not resolve token '%s' from OperationInverseTokens doc object", token)
}

func (oits operationTokens) GetTokenSemantic(key string) (TokenSemantic, bool) {
	tokenSemantic, ok := oits[key]
	return tokenSemantic, ok
}

type OperationInverse interface {
	JSONLookup(token string) (interface{}, error)
	GetOperationStore() (StandardOperationStore, bool)
	GetTokens() (OperationTokens, bool)
	GetParamMap(response.Response) (map[string]interface{}, error)
}

type operationInverse struct {
	OpRef         *OpenAPIOperationStoreRef `json:"sqlVerb" yaml:"sqlVerb"`
	ReverseTokens operationTokens           `json:"tokens,omitempty" yaml:"tokens,omitempty"`
}

func (oi *operationInverse) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "sqlVerb":
		return oi.OpRef, nil
	case "tokens":
		return oi.ReverseTokens, nil
	default:
		return nil, fmt.Errorf("could not resolve token '%s' from OperationInverse doc object", token)
	}
}

func (oi *operationInverse) GetOperationStore() (StandardOperationStore, bool) {
	return oi.getOpenAPIOperationStore()
}

func (oi *operationInverse) getOpenAPIOperationStore() (StandardOperationStore, bool) {
	if oi.OpRef != nil && (oi.OpRef.Ref == "" || oi.OpRef.Value == nil) {
		return nil, false
	}
	return oi.OpRef.Value, true
}

func (oi *operationInverse) GetParamMap(res response.Response) (map[string]interface{}, error) {
	return oi.getParamMap(res)
}

func (oi *operationInverse) getParamMap(res response.Response) (map[string]interface{}, error) {
	rv := make(map[string]interface{})
	for k, v := range oi.ReverseTokens {
		tokenKey := k
		val, err := v.GetProcessedToken(res)
		if err != nil {
			return nil, err
		}
		rv[tokenKey] = val
	}
	return rv, nil
}

func (oi *operationInverse) GetTokens() (OperationTokens, bool) {
	return oi.ReverseTokens, oi.ReverseTokens != nil
}
