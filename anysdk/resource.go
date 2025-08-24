package anysdk

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-openapi/jsonpointer"
	"github.com/stackql/stackql-parser/go/sqltypes"
)

var (
	_ Resource                  = &standardResource{}
	_ jsonpointer.JSONPointable = standardResource{}
	_ jsonpointer.JSONSetable   = standardResource{}
)

type Resource interface {
	ITable
	getQueryTransposeAlgorithm() string
	GetID() string
	GetTitle() string
	GetDescription() string
	GetSelectorAlgorithm() string
	GetMethods() Methods
	GetServiceDocPath() *ServiceRef
	GetRequestTranslateAlgorithm() string
	GetPaginationRequestTokenSemantic() (TokenSemantic, bool)
	GetPaginationResponseTokenSemantic() (TokenSemantic, bool)
	FindMethod(key string) (StandardOperationStore, error)
	GetFirstMethodFromSQLVerb(sqlVerb string) (StandardOperationStore, string, bool)
	GetFirstMethodMatchFromSQLVerb(sqlVerb string, parameters map[string]interface{}) (StandardOperationStore, map[string]interface{}, bool)
	GetService() (OpenAPIService, bool)
	GetViewsForSqlDialect(sqlDialect string) ([]View, bool)
	GetMethodsMatched() Methods
	ToMap(extended bool) map[string]interface{}
	// unexported mutators
	getSQLVerbs() map[string][]OpenAPIOperationStoreRef
	setProvider(p Provider)
	setService(s OpenAPIService)
	setProviderService(ps ProviderService)
	getUnionRequiredParameters(method StandardOperationStore) (map[string]Addressable, error)
	setMethod(string, *standardOpenAPIOperationStore)
	mutateSQLVerb(k string, idx int, v OpenAPIOperationStoreRef)
	propogateToConfig() error
}

type standardResource struct {
	ID                string                                `json:"id" yaml:"id"`       // Required
	Name              string                                `json:"name" yaml:"name"`   // Required
	Title             string                                `json:"title" yaml:"title"` // Required
	Description       string                                `json:"description,omitempty" yaml:"desription,omitempty"`
	SelectorAlgorithm string                                `json:"selectorAlgorithm,omitempty" yaml:"selectorAlgorithm,omitempty"`
	Methods           Methods                               `json:"methods" yaml:"methods"`
	ServiceDocPath    *ServiceRef                           `json:"serviceDoc,omitempty" yaml:"serviceDoc,omitempty"`
	SQLVerbs          map[string][]OpenAPIOperationStoreRef `json:"sqlVerbs" yaml:"sqlVerbs"`
	BaseUrl           string                                `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"` // hack
	StackQLConfig     *standardStackQLConfig                `json:"config,omitempty" yaml:"config,omitempty"`
	OpenAPIService    OpenAPIService                        `json:"-" yaml:"-"` // upwards traversal
	ProviderService   ProviderService                       `json:"-" yaml:"-"` // upwards traversal
	Provider          Provider                              `json:"-" yaml:"-"` // upwards traversal
}

func NewEmptyResource() Resource {
	return &standardResource{
		Methods:  make(Methods),
		SQLVerbs: make(map[string][]OpenAPIOperationStoreRef),
	}
}

func (r *standardResource) propogateToConfig() error {
	if r.StackQLConfig == nil {
		return nil
	}
	r.StackQLConfig.setResource(r)
	return nil
}

func (r *standardResource) GetService() (OpenAPIService, bool) {
	if r.OpenAPIService == nil {
		return nil, false
	}
	return r.OpenAPIService, true
}

func (r *standardResource) getSQLVerbs() map[string][]OpenAPIOperationStoreRef {
	return r.SQLVerbs
}

func (r *standardResource) setService(s OpenAPIService) {
	r.OpenAPIService = s
}

func (r *standardResource) mutateSQLVerb(k string, idx int, v OpenAPIOperationStoreRef) {
	r.SQLVerbs[k][idx] = v
	if v.Value != nil {
		v.Value.setSQLVerb(k)
	}
}

func (r *standardResource) setMethod(k string, v *standardOpenAPIOperationStore) {
	if v == nil {
		return
	}
	r.Methods[k] = *v
}

func (r *standardResource) setProvider(p Provider) {
	r.Provider = p
}

func (r *standardResource) setProviderService(ps ProviderService) {
	r.ProviderService = ps
}

func (r *standardResource) GetID() string {
	return r.ID
}

func (r *standardResource) GetTitle() string {
	return r.Title
}

func (r *standardResource) GetDescription() string {
	return r.Description
}

func (r *standardResource) GetSelectorAlgorithm() string {
	return r.SelectorAlgorithm
}

func (r *standardResource) GetMethods() Methods {
	return r.Methods
}

func (r *standardResource) GetServiceDocPath() *ServiceRef {
	return r.ServiceDocPath
}

func (r *standardResource) getQueryTransposeAlgorithm() string {
	if r.StackQLConfig == nil || r.StackQLConfig.QueryTranspose == nil {
		return ""
	}
	return r.StackQLConfig.QueryTranspose.Algorithm
}

func (r *standardResource) GetRequestTranslateAlgorithm() string {
	if r.StackQLConfig == nil || r.StackQLConfig.RequestTranslate == nil {
		return ""
	}
	return r.StackQLConfig.RequestTranslate.Algorithm
}

func (r *standardResource) GetPaginationRequestTokenSemantic() (TokenSemantic, bool) {
	if r.StackQLConfig != nil {
		pag, pagExists := r.StackQLConfig.GetPagination()
		if pagExists && pag.GetRequestToken() != nil {
			return pag.GetRequestToken(), true
		}
	}
	return nil, false
}

func (r *standardResource) GetViewsForSqlDialect(sqlDialect string) ([]View, bool) {
	if r.StackQLConfig != nil {
		return r.StackQLConfig.GetViewsForSqlDialect(sqlDialect, ViewKeyResourceLevelSelect)
	}
	return []View{}, false
}

func (r *standardResource) GetPaginationResponseTokenSemantic() (TokenSemantic, bool) {
	if r.StackQLConfig != nil {
		pag, pagExists := r.StackQLConfig.GetPagination()
		if pagExists && pag.GetResponseToken() != nil {
			return pag.GetResponseToken(), true
		}
	}
	return nil, false
}

func (rsc standardResource) JSONLookup(token string) (interface{}, error) {
	ss := strings.Split(token, "/")
	tokenRoot := ""
	if len(ss) > 1 {
		tokenRoot = ss[len(ss)-2]
	}
	switch tokenRoot {
	case "methods":
		if rsc.Methods == nil {
			return nil, fmt.Errorf("Provider.JSONLookup() failure due to prov.ProviderServices == nil")
		}
		m, ok := rsc.Methods[ss[len(ss)-1]]
		if !ok {
			return nil, fmt.Errorf("cannot resolve json pointer path '%s'", token)
		}
		return &m, nil
	default:
		val, _, err := jsonpointer.GetForToken(rsc.OpenAPIService.getT(), token)
		return val, err
	}
}

func (rsc standardResource) JSONSet(token string, value interface{}) error {
	ss := strings.Split(token, "/")
	tokenRoot := ""
	if len(ss) > 1 {
		tokenRoot = ss[len(ss)-2]
	}
	switch tokenRoot {
	case "methods":
		if rsc.Methods == nil {
			return fmt.Errorf("Provider.JSONLookup() failure due to prov.ProviderServices == nil")
		}
		newMethod, isMethod := value.(standardOpenAPIOperationStore)
		if !isMethod {
			return fmt.Errorf("cannot resolve json pointer path '%s'", token)
		}
		rsc.Methods[ss[len(ss)-1]] = newMethod
		return nil
	default:
		return fmt.Errorf("cannot set json pointer path '%s'", token)
	}
}

func (rs *standardResource) GetDefaultMethodKeysForSQLVerb(sqlVerb string) []string {
	return rs.getDefaultMethodKeysForSQLVerb(sqlVerb)
}

func (rs *standardResource) GetMethodsMatched() Methods {
	return rs.getMethodsMatched()
}

func (rs *standardResource) matchSQLVerbs() {
	for k, v := range rs.SQLVerbs {
		for _, or := range v {
			orp := &or
			mutated, err := resolveSQLVerbFromResource(rs, orp, k)
			if err == nil && mutated != nil {
				mk := or.extractMethodItem()
				_, ok := rs.Methods[mk]
				if mk != "" && ok {
					rs.Methods[mk] = *mutated
				}
			}
		}
	}
}

func (rs *standardResource) getMethodsMatched() Methods {
	rs.matchSQLVerbs()
	rv := rs.Methods
	for k, v := range rv {
		m := v
		sqlVerb := m.GetSQLVerb()
		if sqlVerb == "" {
			sqlVerb = rs.getDefaultSQLVerbForMethodKey(k)
		}
		m.setSQLVerb(sqlVerb)
		rv[k] = m
	}
	return rv
}

func (rs *standardResource) GetFirstMethodMatchFromSQLVerb(sqlVerb string, parameters map[string]interface{}) (StandardOperationStore, map[string]interface{}, bool) {
	return rs.getFirstMethodMatchFromSQLVerb(sqlVerb, parameters)
}

func (rs *standardResource) getFirstMethodMatchFromSQLVerb(sqlVerb string, parameters map[string]interface{}) (StandardOperationStore, map[string]interface{}, bool) {
	ms, err := rs.getMethodsForSQLVerb(sqlVerb)
	if err != nil {
		return nil, parameters, false
	}
	return ms.getFirstMatch(parameters)
}

func (rs *standardResource) GetFirstMethodFromSQLVerb(sqlVerb string) (StandardOperationStore, string, bool) {
	return rs.getFirstMethodFromSQLVerb(sqlVerb)
}

func (rs *standardResource) getUnionRequiredParameters(method StandardOperationStore) (map[string]Addressable, error) {
	targetSchema, _, err := method.GetSelectSchemaAndObjectPath()
	if err != nil {
		return nil, fmt.Errorf("getUnionRequiredParameters(): cannot infer fat required parameters: %s", err.Error())
	}
	if targetSchema == nil {
		return nil, fmt.Errorf("getUnionRequiredParameters(): target schem is nil")
	}
	targetPath := targetSchema.GetPath()
	rv := method.getRequiredParameters()
	for _, m := range rs.Methods {
		s, _, err := m.GetSelectSchemaAndObjectPath()
		if err != nil || s == nil {
			continue
		}
		methodSchemaPath := s.GetPath()
		if err == nil && s != nil && methodSchemaPath != "" && methodSchemaPath == targetPath {
			reqParams := m.getRequiredParameters()
			for k, v := range reqParams {
				existingParam, ok := rv[k]
				if ok && v.GetType() != existingParam.GetType() {
					return nil, fmt.Errorf("getUnionRequiredParameters(): required params '%s' of conflicting types on resource = '%s'", k, rs.GetName())
				}
				rv[k] = v
			}
		}
	}
	return rv, nil
}

func (rs *standardResource) getFirstMethodFromSQLVerb(sqlVerb string) (StandardOperationStore, string, bool) {
	ms, err := rs.getMethodsForSQLVerb(sqlVerb)
	if err != nil {
		return nil, "", false
	}
	return ms.getFirst()
}

func (rs *standardResource) getDefaultMethodKeysForSQLVerb(sqlVerb string) []string {
	switch strings.ToLower(sqlVerb) {
	case "insert":
		return []string{"insert", "create"}
	case "delete":
		return []string{"delete"}
	case "select":
		return []string{"select", "list", "aggregatedList", "get"}
	default:
		return []string{}
	}
}

func (rs *standardResource) getDefaultSQLVerbForMethodKey(methodName string) string {
	switch strings.ToLower(methodName) {
	case "insert", "create":
		return "insert"
	case "delete":
		return "delete"
	case "select", "list", "aggregatedList", "get":
		return "select"
	default:
		return ""
	}
}

func (rs *standardResource) getMethodsForSQLVerb(sqlVerb string) (MethodSet, error) {
	var retVal MethodSet
	v, ok := rs.SQLVerbs[sqlVerb]
	if ok {
		for _, opt := range v {
			if opt.Value != nil {
				retVal = append(retVal, opt.Value)
			}
		}
		if len(retVal) > 0 {
			return retVal, nil
		}
	} else {
		defaultMethodKeys := rs.getDefaultMethodKeysForSQLVerb(sqlVerb)
		for _, k := range defaultMethodKeys {
			m, ok := rs.Methods[k]
			if ok {
				retVal = append(retVal, &m)
			}
		}
		if len(retVal) > 0 {
			return retVal, nil
		}
	}
	return nil, fmt.Errorf("could not resolve SQL verb '%s'", sqlVerb)
}

func (rs *standardResource) GetSelectableObject() string {
	if m, ok := rs.Methods["list"]; ok {
		sc, _, err := m.getResponseBodySchemaAndMediaType()
		if err == nil {
			return sc.GetName()
		}
	}
	return ""
}

func (rs *standardResource) FindOpenAPIOperationStore(sel OperationSelector) (StandardOperationStore, error) {
	switch rs.SelectorAlgorithm {
	case "", "standard":
		return rs.findOpenAPIOperationStoreStandard(sel)
	}
	return nil, fmt.Errorf("cannot search for operation with selector algorithm = '%s'", rs.SelectorAlgorithm)
}

func (rs *standardResource) findOpenAPIOperationStoreStandard(sel OperationSelector) (StandardOperationStore, error) {
	rv, err := rs.Methods.FindFromSelector(sel)
	if err == nil {
		return rv, nil
	}
	return nil, fmt.Errorf("could not locate operation for resource = %s and sql verb  = %s", rs.Name, sel.GetSQLVerb())
}

func (r *standardResource) ConditionIsValid(lhs string, rhs interface{}) bool {
	elem := r.ToMap(true)[lhs]
	return reflect.TypeOf(elem) == reflect.TypeOf(rhs)
}

func (r *standardResource) FilterBy(predicate func(interface{}) (ITable, error)) (ITable, error) {
	return predicate(r)
}

func (r *standardResource) FindMethod(key string) (StandardOperationStore, error) {
	if r.Methods == nil {
		return nil, fmt.Errorf("cannot find method with key = '%s' from nil methods", key)
	}
	return r.Methods.FindMethod(key)
}

func (rs *standardResource) ToMap(extended bool) map[string]interface{} {
	retVal := make(map[string]interface{})
	retVal["id"] = rs.ID
	retVal["name"] = rs.Name
	retVal["title"] = rs.Title
	retVal["description"] = rs.Description
	return retVal
}

func (rs *standardResource) GetKeyAsSqlVal(lhs string) (sqltypes.Value, error) {
	val, ok := rs.ToMap(true)[lhs]
	rv, err := InterfaceToSQLType(val)
	if !ok {
		return rv, fmt.Errorf("key '%s' no preset in metadata_service", lhs)
	}
	return rv, err
}

func (rs *standardResource) GetKey(lhs string) (interface{}, error) {
	val, ok := rs.ToMap(true)[lhs]
	if !ok {
		return nil, fmt.Errorf("key '%s' no preset in metadata_service", lhs)
	}
	return val, nil
}

func (rs *standardResource) KeyExists(lhs string) bool {
	_, ok := rs.ToMap(true)[lhs]
	return ok
}

func (rs *standardResource) GetRequiredParameters() map[string]Addressable {
	return nil
}

func (rs *standardResource) GetName() string {
	return rs.Name
}

func ResourceConditionIsValid(lhs string, rhs interface{}) bool {
	rs := &standardResource{}
	return rs.ConditionIsValid(lhs, rhs)
}

func ResourceKeyExists(key string) bool {
	rs := &standardResource{}
	return rs.KeyExists(key)
}
