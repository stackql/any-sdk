package anysdk

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-openapi/jsonpointer"
)

var (
	sqlDialectRegex    *regexp.Regexp            = regexp.MustCompile(`sqlDialect(?:\s)*==(?:\s)*"(?P<sqlDialect>[^<>"\s]*)"`)
	requiredParamRegex *regexp.Regexp            = regexp.MustCompile(`requiredParams(?:\s)*==(?:\s)*\[(?P<requiredParams>[\S0-9]*)\]`)
	_                  ViewContainer             = &standardViewContainer{}
	_                  jsonpointer.JSONPointable = standardViewContainer{}
)

type ViewContainer interface {
	GetViewsForSqlDialect(sqlBackend string) ([]View, bool)
	setResource(rsc Resource)
}

type View interface {
	GetDDL() string
	GetPredicate() string
	GetNameNaive() string
	GetRequiredParamNames() []string
}

func GetTestingView() standardViewContainer {
	return standardViewContainer{}
}

type standardViewContainer struct {
	Resource  Resource               `json:"-" yaml:"-"`
	Predicate string                 `json:"predicate" yaml:"predicate"`
	DDL       string                 `json:"ddl" yaml:"ddl"`
	Fallback  *standardViewContainer `json:"fallback" yaml:"fallback"` // Future proofing for predicate failover
}

func (v *standardViewContainer) getSqlDialectName() string {
	inputString := v.Predicate
	for i, name := range sqlDialectRegex.SubexpNames() {
		if name == "sqlDialect" {
			submatches := sqlDialectRegex.FindStringSubmatch(inputString)
			if len(submatches) > i {
				return submatches[i]
			}
		}
	}
	return ""
}

func (v *standardViewContainer) setResource(rsc Resource) {
	v.Resource = rsc
	if v.Fallback != nil {
		v.Fallback.setResource(rsc)
	}
}

func (v *standardViewContainer) GetRequiredParamNames() []string {
	return v.getRequiredParamNames()
}

func (v *standardViewContainer) getRequiredParamNames() []string {
	inputString := v.Predicate
	for i, name := range requiredParamRegex.SubexpNames() {
		if name == "requiredParams" {
			submatches := requiredParamRegex.FindStringSubmatch(inputString)
			if len(submatches) > i {
				crudeArr := strings.Split(submatches[i], ",")
				var rv []string
				for _, v := range crudeArr {
					rv = append(rv, strings.ReplaceAll(strings.TrimSpace(v), `"`, ``))
				}
				return rv
			}
		}
	}
	return []string{}
}

func (v *standardViewContainer) GetDDL() string {
	return v.DDL
}

func (v *standardViewContainer) GetPredicate() string {
	return v.Predicate
}

func (v *standardViewContainer) GetNameNaive() string {
	if v.Resource != nil {
		return v.Resource.GetID()
	}
	return ""
}

func (v *standardViewContainer) GetViewsForSqlDialect(sqlBackend string) ([]View, bool) {
	sqlBackendAccepted := v.getSqlDialectName()
	var rv []View
	var containsView bool
	if sqlBackendAccepted == "" {
		rv = append(rv, v)
		containsView = true
	}
	if sqlBackendAccepted == sqlBackend {
		rv = append(rv, v)
		containsView = true
	}
	if v.Fallback != nil {
		rhs, rhsContainsViews := v.Fallback.GetViewsForSqlDialect(sqlBackend)
		if rhsContainsViews {
			rv = append(rv, rhs...)
			containsView = true
		}
	}
	return rv, containsView
}

func (qt standardViewContainer) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "ddl":
		return qt.DDL, nil
	case "predicate":
		return qt.Predicate, nil
	case "fallback":
		return qt.Fallback, nil
	default:
		return nil, fmt.Errorf("could not resolve token '%s' from View doc object", token)
	}
}
