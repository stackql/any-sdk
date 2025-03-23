package anysdk

import (
	"fmt"

	"github.com/go-openapi/jsonpointer"
)

var (
	_ Transform                 = &standardTransform{}
	_ jsonpointer.JSONPointable = (Transform)(standardTransform{})
)

type Transform interface {
	JSONLookup(token string) (interface{}, error)
	GetAlgorithm() string
	GetType() string
	GetBody() string
}

type standardTransform struct {
	Algorithm string `json:"algorithm,omitempty" yaml:"algorithm,omitempty"`
	Type      string `json:"type,omitempty" yaml:"type,omitempty"`
	Body      string `json:"body,omitempty" yaml:"body,omitempty"`
}

func (ts standardTransform) GetAlgorithm() string {
	return ts.Algorithm
}

func (ts standardTransform) GetType() string {
	return ts.Type
}

func (ts standardTransform) GetBody() string {
	return ts.Body
}

func (qt standardTransform) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "algorithm":
		return qt.Algorithm, nil
	default:
		return nil, fmt.Errorf("could not resolve token '%s' from Transform doc object", token)
	}
}
