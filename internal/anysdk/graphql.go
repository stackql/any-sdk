package anysdk

import (
	"fmt"

	"github.com/go-openapi/jsonpointer"
)

var (
	_ jsonpointer.JSONPointable = standardGraphQL{}
	_ GraphQL                   = &standardGraphQL{}
)

type GraphQLElement map[string]interface{}

func (gqc GraphQLElement) getJSONPath() (string, bool) {
	return gqc.getStringField("jsonPath")
}

func (gqc GraphQLElement) getStringField(key string) (string, bool) {
	v, ok := gqc[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

// GetCursorStrategy returns the configured cursor.strategy, or "" if absent.
// Callers should treat "" as the legacy cursor_after default.
func (gqc GraphQLElement) GetCursorStrategy() (string, bool) {
	return gqc.getStringField("strategy")
}

// GetCursorFormat returns the configured cursor.format template, or "" if absent.
func (gqc GraphQLElement) GetCursorFormat() (string, bool) {
	return gqc.getStringField("format")
}

// GetCursorTerminateOnJSONPath returns the configured cursor.terminateOnJsonPath,
// or "" if absent. Only the page_info strategy consumes this.
func (gqc GraphQLElement) GetCursorTerminateOnJSONPath() (string, bool) {
	return gqc.getStringField("terminateOnJsonPath")
}

// GetCursorPageSize returns the configured cursor.pageSize, or 0 if absent.
// Only the offset strategy consumes this; non-positive values are treated as
// "no short-page termination".
func (gqc GraphQLElement) GetCursorPageSize() (int, bool) {
	v, ok := gqc["pageSize"]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}

type GraphQL interface {
	JSONLookup(token string) (interface{}, error)
	GetCursorJSONPath() (string, bool)
	GetResponseJSONPath() (string, bool)
	GetID() string
	GetQuery() string
	GetURL() string
	GetHTTPVerb() string
	GetCursor() GraphQLElement
	GetResponseSelection() GraphQLElement
	GetCursorStrategy() (string, bool)
	GetCursorFormat() (string, bool)
	GetCursorTerminateOnJSONPath() (string, bool)
	GetCursorPageSize() (int, bool)
}

type standardGraphQL struct {
	ID               string         `json:"id" yaml:"id"`
	Query            string         `json:"query,omitempty" yaml:"query,omitempty"` // Required
	Cursor           GraphQLElement `json:"cursor,omitempty" yaml:"cursor,omitempty"`
	ReponseSelection GraphQLElement `json:"responseSelection,omitempty" yaml:"responseSelection,omitempty"`
	URL              string         `json:"url" yaml:"url"`
	HTTPVerb         string         `json:"httpVerb" yaml:"httpVerb"`
}

func (gq *standardGraphQL) GetCursorJSONPath() (string, bool) {
	if gq.Cursor == nil {
		return "", false
	}
	return gq.Cursor.getJSONPath()
}

func (gq *standardGraphQL) GetResponseJSONPath() (string, bool) {
	if gq.Cursor == nil {
		return "", false
	}
	return gq.ReponseSelection.getJSONPath()
}

func (gq standardGraphQL) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "id":
		return gq.ID, nil
	case "query":
		return gq.Query, nil
	case "cursor":
		return gq.Cursor, nil
	case "url":
		return gq.URL, nil
	case "httpVerb":
		return gq.HTTPVerb, nil
	default:
		return nil, fmt.Errorf("could not resolve token '%s' from GraphQL doc object", token)
	}
}

func (gq *standardGraphQL) GetID() string {
	return gq.ID
}

func (gq *standardGraphQL) GetQuery() string {
	return gq.Query
}

func (gq *standardGraphQL) GetURL() string {
	return gq.URL
}

func (gq *standardGraphQL) GetHTTPVerb() string {
	return gq.HTTPVerb
}

func (gq *standardGraphQL) GetCursor() GraphQLElement {
	return gq.Cursor
}

func (gq *standardGraphQL) GetResponseSelection() GraphQLElement {
	return gq.ReponseSelection
}

func (gq *standardGraphQL) GetCursorStrategy() (string, bool) {
	if gq.Cursor == nil {
		return "", false
	}
	return gq.Cursor.GetCursorStrategy()
}

func (gq *standardGraphQL) GetCursorFormat() (string, bool) {
	if gq.Cursor == nil {
		return "", false
	}
	return gq.Cursor.GetCursorFormat()
}

func (gq *standardGraphQL) GetCursorTerminateOnJSONPath() (string, bool) {
	if gq.Cursor == nil {
		return "", false
	}
	return gq.Cursor.GetCursorTerminateOnJSONPath()
}

func (gq *standardGraphQL) GetCursorPageSize() (int, bool) {
	if gq.Cursor == nil {
		return 0, false
	}
	return gq.Cursor.GetCursorPageSize()
}
