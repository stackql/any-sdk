package anysdk

import (
	"fmt"

	"github.com/go-openapi/jsonpointer"
)

var (
	_ jsonpointer.JSONPointable = standardStackQLConfig{}
	_ StackQLConfig             = &standardStackQLConfig{}
)

type StackQLConfig interface {
	GetAuth() (AuthDTO, bool)
	GetViewsForSqlDialect(sqlDialect string, viewName string) ([]View, bool)
	GetQueryTranspose() (Transform, bool)
	GetRequestTranslate() (Transform, bool)
	GetRequestBodyTranslate() (Transform, bool)
	GetPagination() (Pagination, bool)
	GetVariations() (Variations, bool)
	GetViews() map[string]View
	GetExternalTables() map[string]SQLExternalTable
	//
	isObjectSchemaImplicitlyUnioned() bool
	setResource(rsc Resource)
}

type standardStackQLConfig struct {
	Resource             Resource                            `json:"-" yaml:"-"`
	QueryTranspose       *standardTransform                  `json:"queryParamTranspose,omitempty" yaml:"queryParamTranspose,omitempty"`
	RequestTranslate     *standardTransform                  `json:"requestTranslate,omitempty" yaml:"requestTranslate,omitempty"`
	RequestBodyTranslate *standardTransform                  `json:"requestBodyTranslate,omitempty" yaml:"requestBodyTranslate,omitempty"`
	Pagination           *standardPagination                 `json:"pagination,omitempty" yaml:"pagination,omitempty"`
	Variations           *standardVariations                 `json:"variations,omitempty" yaml:"variations,omitempty"`
	Views                map[string]*standardViewContainer   `json:"views" yaml:"views"`
	ExternalTables       map[string]standardSQLExternalTable `json:"sqlExternalTables" yaml:"sqlExternalTables"`
	Auth                 *standardAuthDTO                    `json:"auth,omitempty" yaml:"auth,omitempty"`
}

func (qt standardStackQLConfig) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "queryTranspose":
		return qt.QueryTranspose, nil
	case "requestBodyTranslate":
		return qt.RequestBodyTranslate, nil
	case "requestTranslate":
		return qt.RequestTranslate, nil
	case "views":
		return qt.Views, nil
	default:
		return nil, fmt.Errorf("could not resolve token '%s' from QueryTranspose doc object", token)
	}
}

func (cfg *standardStackQLConfig) GetQueryTranspose() (Transform, bool) {
	if cfg.QueryTranspose == nil {
		return nil, false
	}
	return cfg.QueryTranspose, true
}

func (cfg *standardStackQLConfig) setResource(rsc Resource) {
	cfg.Resource = rsc
	if cfg.Views != nil {
		for _, v := range cfg.Views {
			v.setResource(rsc)
		}
	}
}

func (cfg *standardStackQLConfig) GetRequestTranslate() (Transform, bool) {
	if cfg.RequestTranslate == nil {
		return nil, false
	}
	return cfg.RequestTranslate, true
}

func (cfg *standardStackQLConfig) GetRequestBodyTranslate() (Transform, bool) {
	if cfg.RequestBodyTranslate == nil {
		return nil, false
	}
	return cfg.RequestBodyTranslate, true
}

func (cfg *standardStackQLConfig) GetPagination() (Pagination, bool) {
	if cfg.Pagination == nil {
		return nil, false
	}
	return cfg.Pagination, true
}

func (cfg *standardStackQLConfig) GetVariations() (Variations, bool) {
	if cfg.Variations == nil {
		return nil, false
	}
	return cfg.Variations, true
}

func (cfg *standardStackQLConfig) GetViews() map[string]View {
	rv := make(map[string]View, len(cfg.Views))
	if cfg.Views != nil {
		for k, v := range cfg.Views {
			rv[k] = v
		}
	}
	return rv
}

func (cfg *standardStackQLConfig) isObjectSchemaImplicitlyUnioned() bool {
	if cfg.Variations != nil {
		return cfg.Variations.IsObjectSchemaImplicitlyUnioned()
	}
	return false
}

func (cfg *standardStackQLConfig) GetView(viewName string) (View, bool) {
	if cfg.Views != nil {
		v, ok := cfg.Views[viewName]
		return v, ok
	}
	return nil, false
}

func (cfg *standardStackQLConfig) GetAuth() (AuthDTO, bool) {
	return cfg.Auth, cfg.Auth != nil
}

func (cfg *standardStackQLConfig) GetExternalTables() map[string]SQLExternalTable {
	rv := make(map[string]SQLExternalTable, len(cfg.ExternalTables))
	if cfg.ExternalTables != nil {
		for k, v := range cfg.ExternalTables {
			rv[k] = v
		}
		return rv
	}
	return nil
}

func (cfg *standardStackQLConfig) GetViewsForSqlDialect(sqlDialect string, viewName string) ([]View, bool) {
	if cfg.Views != nil {
		v, ok := cfg.Views[viewName]
		if !ok || v == nil {
			return []View{}, false
		}
		return v.GetViewsForSqlDialect(sqlDialect)
	}
	return []View{}, false
}
