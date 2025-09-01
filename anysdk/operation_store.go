package anysdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/stackql/any-sdk/pkg/fuzzymatch"
	"github.com/stackql/any-sdk/pkg/media"
	"github.com/stackql/any-sdk/pkg/parametertranslate"
	"github.com/stackql/any-sdk/pkg/queryrouter"
	"github.com/stackql/any-sdk/pkg/response"
	"github.com/stackql/any-sdk/pkg/stream_transform"
	"github.com/stackql/any-sdk/pkg/urltranslate"
	"github.com/stackql/any-sdk/pkg/util"
	"github.com/stackql/any-sdk/pkg/xmlmap"

	"github.com/stackql/stackql-parser/go/sqltypes"
)

const (
	defaultSelectItemsKey = "items"
	defaultXMLDeclaration = `<?xml version="1.0" encoding="UTF-8"?>`
	xmlTransformUnescape  = "unescape"
	xmlTransformDefault   = xmlTransformUnescape
)

var (
	_ StandardOperationStore = &standardOpenAPIOperationStore{}
)

func sortOpenAPIOperationStoreSlices(opSlices ...[]StandardOperationStore) {
	for _, opSlice := range opSlices {
		sort.SliceStable(opSlice, func(i, j int) bool {
			return opSlice[i].GetMethodKey() < opSlice[j].GetMethodKey()
		})
	}
}

func combineOpenAPIOperationStoreSlices(opSlices ...[]StandardOperationStore) []StandardOperationStore {
	var rv []StandardOperationStore
	for _, sl := range opSlices {
		rv = append(rv, sl...)
	}
	return rv
}

type OperationStore interface {
	ITable
	GetMethodKey() string
	GetSQLVerb() string
	GetGraphQL() GraphQL
	GetInverse() (OperationInverse, bool)
	GetStackQLConfig() StackQLConfig
	GetParameters() map[string]Addressable
	GetPathItem() *openapi3.PathItem
	GetAPIMethod() string
	GetInline() []string
	GetOperationRef() *OperationRef
	GetPathRef() *PathItemRef
	GetRequest() (ExpectedRequest, bool)
	GetResponse() (ExpectedResponse, bool)
	GetServers() (openapi3.Servers, bool)
	GetParameterizedPath() string
	GetProviderService() ProviderService
	GetProvider() Provider
	GetService() OpenAPIService
	GetResource() Resource
	parameterMatch(params map[string]interface{}) (map[string]interface{}, bool)
	GetOperationParameter(key string) (Addressable, bool)
	GetSelectSchemaAndObjectPath() (Schema, string, error)
	ProcessResponse(*http.Response) (ProcessedOperationResponse, error) // to be removed
	parameterize(prov Provider, parentDoc Service, inputParams HttpParameters, requestBody interface{}) (*openapi3filter.RequestValidationInput, error)
	GetSelectItemsKey() string
	GetResponseBodySchemaAndMediaType() (Schema, string, error)
	GetFinalResponseBodySchemaAndMediaType() (Schema, string, error)
	GetRequiredParameters() map[string]Addressable
	GetOptionalParameters() map[string]Addressable
	GetParameter(paramKey string) (Addressable, bool)
	GetUnionRequiredParameters() (map[string]Addressable, error)
	GetPaginationResponseTokenSemantic() (TokenSemantic, bool)
	MarshalBody(body interface{}, expectedRequest ExpectedRequest) ([]byte, error)
	GetRequestBodySchema() (Schema, error)
	GetNonBodyParameters() map[string]Addressable
	IsAwaitable() bool
	DeprecatedProcessResponse(response *http.Response) (map[string]interface{}, error)
	GetRequestTranslateAlgorithm() string
	IsRequiredRequestBodyProperty(key string) bool
	GetPaginationRequestTokenSemantic() (TokenSemantic, bool)
	IsNullary() bool
	ToPresentationMap(extended bool) map[string]interface{}
	GetColumnOrder(extended bool) []string
	RenameRequestBodyAttribute(string) (string, error)
	RevertRequestBodyAttributeRename(string) (string, error)
	IsRequestBodyAttributeRenamed(string) bool
	GetRequiredNonBodyParameters() map[string]Addressable
	ShouldBeSelectable() bool
	getServiceNameForProvider() string
}

type StandardOperationStore interface {
	OperationStore
	// Assist analysis
	GetRequestBodyAttributesNoRename() (map[string]Addressable, error)
	GetSchemaAtPath(key string) (Schema, error)
	GetSelectItemsKeySimple() string
	LookupSelectItemsKey() string
	//
	getQueryTransposeAlgorithm() string
	getRequiredNonBodyParameters() map[string]Addressable
	getDefaultRequestBodyBytes() []byte
	getBaseRequestBodyBytes() []byte
	getName() string
	getServerVariable(key string) (*openapi3.ServerVariable, bool)
	setMethodKey(string)
	setSQLVerb(string)
	getRequiredParameters() map[string]Addressable
	getResponseBodySchemaAndMediaType() (Schema, string, error)
	setGraphQL(GraphQL)
	setRequest(*standardExpectedRequest)
	setResponse(*standardExpectedResponse)
	setServers(*openapi3.Servers)
	setProvider(Provider)
	setProviderService(ProviderService)
	setResource(Resource)
	setService(OpenAPIService)
	setOperationRef(*OperationRef)
	setPathItem(*openapi3.PathItem)
	renameRequestBodyAttribute(string) (string, error)
	revertRequestBodyAttributeRename(string) (string, error)
	getRequestBodyAttributeParentKey(string) (string, bool)
	getRequestBodyTranslateAlgorithmString() string
	getRequestBodyStringifiedPaths() (map[string]struct{}, error)
	GetRequestBodyMediaType() string
	getRequestBodyMediaType() string
	getRequestBodyMediaTypeNormalised() string
	getXMLDeclaration() string
	// getRequestBodyAttributeLineage(string) (string, error)
}

type standardOpenAPIOperationStore struct {
	MethodKey     string                 `json:"-" yaml:"-"`
	SQLVerb       string                 `json:"-" yaml:"-"`
	GraphQL       GraphQL                `json:"-" yaml:"-"`
	StackQLConfig *standardStackQLConfig `json:"config,omitempty" yaml:"config,omitempty"`
	// Optional parameters.
	Parameters   map[string]map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	PathItem     *openapi3.PathItem                `json:"-" yaml:"-"`                 // Required
	APIMethod    string                            `json:"apiMethod" yaml:"apiMethod"` // Required
	OperationRef *OperationRef                     `json:"operation" yaml:"operation"` // Required
	InlineOp     []string                          `json:"inline" yaml:"inline"`       // Deprecated
	PathRef      *PathItemRef                      `json:"path" yaml:"path"`           // Deprecated
	Request      *standardExpectedRequest          `json:"request" yaml:"request"`
	Response     *standardExpectedResponse         `json:"response" yaml:"response"`
	Servers      *openapi3.Servers                 `json:"servers" yaml:"servers"`
	Inverse      *operationInverse                 `json:"inverse" yaml:"inverse"`
	ServiceName  string                            `json:"serviceName,omitempty" yaml:"serviceName,omitempty"`
	// private
	parameterizedPath string          `json:"-" yaml:"-"`
	ProviderService   ProviderService `json:"-" yaml:"-"` // upwards traversal
	Provider          Provider        `json:"-" yaml:"-"` // upwards traversal
	OpenAPIService    OpenAPIService  `json:"-" yaml:"-"` // upwards traversal
	Resource          Resource        `json:"-" yaml:"-"` // upwards traversal
}

func (op *standardOpenAPIOperationStore) GetInline() []string {
	if op.InlineOp != nil {
		return op.InlineOp
	}
	return []string{}
}

func (op *standardOpenAPIOperationStore) getXMLDeclaration() string {
	rv := ""
	if op.Request != nil {
		rv = op.Request.XMLDeclaration
	}
	if rv == "" {
		rv = defaultXMLDeclaration
	}
	return rv
}

func (op *standardOpenAPIOperationStore) getServiceNameForProvider() string {
	if op.ServiceName != "" {
		return op.ServiceName
	}
	if op.OpenAPIService != nil {
		return op.OpenAPIService.GetName()
	}
	return ""
}

func (op *standardOpenAPIOperationStore) getXMLRootAnnotation() string {
	rv := ""
	if op.Request != nil {
		rv = op.Request.XMLRootAnnotation
	}
	return rv
}

func (op *standardOpenAPIOperationStore) getXMLTransform() string {
	rv := ""
	if op.Request != nil {
		rv = op.Request.XMLTransform
	}
	if rv == "" {
		rv = xmlTransformDefault
	}
	return rv
}

func (op *standardOpenAPIOperationStore) getRequestBodyStringifiedPaths() (map[string]struct{}, error) {
	rv := make(map[string]struct{})
	requestBodySchema, schemaErr := op.getRequestBodySchema()
	if schemaErr != nil {
		return rv, schemaErr
	}
	for k, v := range requestBodySchema.getProperties() {
		if v == nil {
			continue
		}
		if v.isStringOnly() {
			rv[k] = struct{}{}
		}
	}
	return rv, nil
}

func NewEmptyOperationStore() StandardOperationStore {
	return &standardOpenAPIOperationStore{
		Parameters: make(map[string]map[string]interface{}),
	}
}

func (op *standardOpenAPIOperationStore) GetRequestBodyMediaType() string {
	return op.getRequestBodyMediaType()
}

func (op *standardOpenAPIOperationStore) getRequestBodyMediaType() string {
	if op.Request != nil {
		return op.Request.BodyMediaType
	}
	return ""
}

func (op *standardOpenAPIOperationStore) getRequestBodyMediaTypeNormalised() string {
	return media.NormaliseMediaType(op.getRequestBodyMediaType())
}

func (op *standardOpenAPIOperationStore) setPathItem(pi *openapi3.PathItem) {
	op.PathItem = pi
}

func (op *standardOpenAPIOperationStore) setService(svc OpenAPIService) {
	op.OpenAPIService = svc
}

func (op *standardOpenAPIOperationStore) setOperationRef(opr *OperationRef) {
	op.OperationRef = opr
}

func (op *standardOpenAPIOperationStore) setProvider(pr Provider) {
	op.Provider = pr
}

func (op *standardOpenAPIOperationStore) setProviderService(ps ProviderService) {
	op.ProviderService = ps
}

func (op *standardOpenAPIOperationStore) setResource(rs Resource) {
	op.Resource = rs
}

func (op *standardOpenAPIOperationStore) setServers(servers *openapi3.Servers) {
	op.Servers = servers
}

func (op *standardOpenAPIOperationStore) setGraphQL(gql GraphQL) {
	op.GraphQL = gql
}

func (op *standardOpenAPIOperationStore) setRequest(req *standardExpectedRequest) {
	op.Request = req
}

func (op *standardOpenAPIOperationStore) getDefaultRequestBodyBytes() []byte {
	var rv []byte
	if op.Request != nil && op.Request.Default != "" {
		rv = []byte(op.Request.Default)
	}
	return rv
}

func (op *standardOpenAPIOperationStore) getBaseRequestBodyBytes() []byte {
	var rv []byte
	if op.Request != nil && op.Request.Base != "" {
		rv = []byte(op.Request.Base)
	}
	return rv
}

func (op *standardOpenAPIOperationStore) setResponse(resp *standardExpectedResponse) {
	op.Response = resp
}

func (op *standardOpenAPIOperationStore) setMethodKey(methodKey string) {
	op.MethodKey = methodKey
}

func (op *standardOpenAPIOperationStore) setSQLVerb(sqlVerb string) {
	op.SQLVerb = sqlVerb
}

func (op *standardOpenAPIOperationStore) GetMethodKey() string {
	return op.MethodKey
}

func (op *standardOpenAPIOperationStore) GetSQLVerb() string {
	return op.SQLVerb
}

func (op *standardOpenAPIOperationStore) GetGraphQL() GraphQL {
	return op.GraphQL
}

func (op *standardOpenAPIOperationStore) GetInverse() (OperationInverse, bool) {
	return op.Inverse, op.Inverse != nil
}

func (op *standardOpenAPIOperationStore) GetStackQLConfig() StackQLConfig {
	rv, isPresent := op.getStackQLConfig()
	if !isPresent {
		return nil
	}
	return rv
}

func (op *standardOpenAPIOperationStore) getStackQLConfig() (StackQLConfig, bool) {
	rv := op.StackQLConfig
	return rv, rv != nil
}

func (op *standardOpenAPIOperationStore) GetAPIMethod() string {
	return op.APIMethod
}

func (op *standardOpenAPIOperationStore) GetOperationRef() *OperationRef {
	return op.OperationRef
}

func (op *standardOpenAPIOperationStore) GetPathRef() *PathItemRef {
	return op.PathRef
}

func (op *standardOpenAPIOperationStore) GetPathItem() *openapi3.PathItem {
	return op.PathItem
}

func (op *standardOpenAPIOperationStore) GetRequest() (ExpectedRequest, bool) {
	if op.Request == nil {
		return nil, false
	}
	return op.Request, true
}

func (op *standardOpenAPIOperationStore) GetResponse() (ExpectedResponse, bool) {
	if op.Response == nil {
		return nil, false
	}
	return op.Response, true
}

func (op *standardOpenAPIOperationStore) GetServers() (openapi3.Servers, bool) {
	return op.getServers()
}

func (op *standardOpenAPIOperationStore) getServers() (openapi3.Servers, bool) {
	servers := getServersFromHeirarchy(op)
	if len(servers) > 0 {
		return servers, true
	}
	if op.Servers != nil {
		return *(op.Servers), true
	}
	if op.OpenAPIService != nil {
		return op.OpenAPIService.GetServers()
	}
	return nil, false
}

func (op *standardOpenAPIOperationStore) GetProviderService() ProviderService {
	return op.ProviderService
}

func (op *standardOpenAPIOperationStore) GetProvider() Provider {
	return op.Provider
}

func (op *standardOpenAPIOperationStore) GetService() OpenAPIService {
	return op.OpenAPIService
}

func (op *standardOpenAPIOperationStore) GetResource() Resource {
	return op.Resource
}

func (op *standardOpenAPIOperationStore) GetViewsForSqlDialect(sqlDialect string) ([]View, bool) {
	if op.StackQLConfig != nil {
		return op.StackQLConfig.GetViewsForSqlDialect(sqlDialect, "")
	}
	return []View{}, false
}

func (op *standardOpenAPIOperationStore) getQueryTransposeAlgorithm() string {
	if op.StackQLConfig != nil {
		transpose, transposeExists := op.StackQLConfig.GetQueryTranspose()
		if transposeExists && transpose.GetAlgorithm() != "" {
			return transpose.GetAlgorithm()
		}
	}
	if op.Resource != nil && op.Resource.getQueryTransposeAlgorithm() != "" {
		return op.Resource.getQueryTransposeAlgorithm()
	}
	if op.OpenAPIService != nil && op.OpenAPIService.getQueryTransposeAlgorithm() != "" {
		return op.OpenAPIService.getQueryTransposeAlgorithm()
	}
	if op.ProviderService != nil && op.ProviderService.getQueryTransposeAlgorithm() != "" {
		return op.ProviderService.getQueryTransposeAlgorithm()
	}
	if op.Provider != nil && op.Provider.getQueryTransposeAlgorithm() != "" {
		return op.Provider.getQueryTransposeAlgorithm()
	}
	return ""
}

func (op *standardOpenAPIOperationStore) GetRequestTranslateAlgorithm() string {
	if op.StackQLConfig != nil {
		translate, translateExists := op.StackQLConfig.GetRequestTranslate()
		if translateExists && translate.GetAlgorithm() != "" {
			return translate.GetAlgorithm()
		}
	}
	if op.Resource != nil && op.Resource.GetRequestTranslateAlgorithm() != "" {
		return op.Resource.GetRequestTranslateAlgorithm()
	}
	if op.OpenAPIService != nil && op.OpenAPIService.getRequestTranslateAlgorithm() != "" {
		return op.OpenAPIService.getRequestTranslateAlgorithm()
	}
	if op.ProviderService != nil && op.ProviderService.GetRequestTranslateAlgorithm() != "" {
		return op.ProviderService.GetRequestTranslateAlgorithm()
	}
	if op.Provider != nil && op.Provider.GetRequestTranslateAlgorithm() != "" {
		return op.Provider.GetRequestTranslateAlgorithm()
	}
	return ""
}

func (op *standardOpenAPIOperationStore) GetPaginationRequestTokenSemantic() (TokenSemantic, bool) {
	if op.StackQLConfig != nil {
		pag, pagExists := op.StackQLConfig.GetPagination()
		if pagExists && pag.GetRequestToken() != nil {
			return pag.GetRequestToken(), true
		}
	}
	if op.Resource != nil {
		if ts, ok := op.Resource.GetPaginationRequestTokenSemantic(); ok {
			return ts, true
		}
	}
	if op.OpenAPIService != nil {
		if ts, ok := op.OpenAPIService.getPaginationRequestTokenSemantic(); ok {
			return ts, true
		}
	}
	if op.ProviderService != nil {
		if ts, ok := op.ProviderService.GetPaginationRequestTokenSemantic(); ok {
			return ts, true
		}
	}
	if op.Provider != nil {
		if ts, ok := op.ProviderService.GetPaginationRequestTokenSemantic(); ok {
			return ts, true
		}
	}
	return nil, false
}

func (op *standardOpenAPIOperationStore) GetPaginationResponseTokenSemantic() (TokenSemantic, bool) {
	if op.StackQLConfig != nil {
		pag, pagExists := op.StackQLConfig.GetPagination()
		if pagExists && pag.GetResponseToken() != nil {
			return pag.GetResponseToken(), true
		}
	}
	if op.Resource != nil {
		if ts, ok := op.Resource.GetPaginationResponseTokenSemantic(); ok {
			return ts, true
		}
	}
	if op.OpenAPIService != nil {
		if ts, ok := op.OpenAPIService.getPaginationResponseTokenSemantic(); ok {
			return ts, true
		}
	}
	if op.ProviderService != nil {
		if ts, ok := op.ProviderService.getPaginationResponseTokenSemantic(); ok {
			return ts, true
		}
	}
	if op.Provider != nil {
		if ts, ok := op.ProviderService.getPaginationResponseTokenSemantic(); ok {
			return ts, true
		}
	}
	return nil, false
}

func (op *standardOpenAPIOperationStore) parameterMatch(params map[string]interface{}) (map[string]interface{}, bool) {
	copiedParams := make(map[string]interface{})
	for k, v := range params {
		copiedParams[k] = v
	}
	requiredParameters := NewParameterSuffixMap()
	optionalParameters := NewParameterSuffixMap()
	for k, v := range op.getRequiredParameters() {
		key := fmt.Sprintf("%s.%s", op.getName(), k)
		_, keyExists := requiredParameters.Get(key)
		if keyExists {
			return copiedParams, false
		}
		requiredParameters.Put(key, v)
	}
	for k, vOpt := range op.getOptionalParameters() {
		key := fmt.Sprintf("%s.%s", op.getName(), k)
		_, keyExists := optionalParameters.Get(key)
		if keyExists {
			return copiedParams, false
		}
		optionalParameters.Put(key, vOpt)
	}
	for k := range copiedParams {
		if requiredParameters.Delete(k) {
			delete(copiedParams, k)
			continue
		}
		if optionalParameters.Delete(k) {
			delete(copiedParams, k)
			continue
		}
		// log.Debugf("parameter '%s' unmatched for method '%s'\n", k, op.getName())
	}
	if requiredParameters.Size() == 0 {
		return copiedParams, true
	}
	// log.Debugf("unmatched **required** paramter count = %d for method '%s'\n", requiredParameters.Size(), op.getName())
	return copiedParams, false
}

func (op *standardOpenAPIOperationStore) GetParameterizedPath() string {
	return op.parameterizedPath
}

func (op *standardOpenAPIOperationStore) GetOptimalResponseMediaType() string {
	return op.getOptimalResponseMediaType()
}

func (op *standardOpenAPIOperationStore) getOptimalResponseMediaType() string {
	if op.Response != nil && op.Response.BodyMediaType != "" {
		return op.Response.BodyMediaType
	}
	return media.MediaTypeJson
}

func (op *standardOpenAPIOperationStore) IsNullary() bool {
	rbs, _, _ := op.GetResponseBodySchemaAndMediaType()
	return rbs == nil
}

func (m *standardOpenAPIOperationStore) KeyExists(lhs string) bool {
	if lhs == MethodName {
		return true
	}
	if m.OperationRef == nil {
		return false
	}
	if m.OperationRef.Value == nil {
		return false
	}
	params := m.OperationRef.Value.Parameters
	if params == nil {
		return false
	}
	for _, p := range params {
		if p.Value == nil {
			continue
		}
		if lhs == p.Value.Name {
			return true
		}
	}
	availableServers, availableServersDoExist := m.getServers()
	if availableServersDoExist {
		for _, s := range availableServers {
			for k, _ := range s.Variables {
				if lhs == k {
					return true
				}
			}
		}
	}
	return false
}

func (m *standardOpenAPIOperationStore) GetSelectItemsKey() string {
	return m.getSelectItemsKeySimple()
}

func (m *standardOpenAPIOperationStore) GetUnionRequiredParameters() (map[string]Addressable, error) {
	return m.getUnionRequiredParameters()
}

func (m *standardOpenAPIOperationStore) getUnionRequiredParameters() (map[string]Addressable, error) {
	return m.Resource.getUnionRequiredParameters(m)
}

func (m *standardOpenAPIOperationStore) GetSelectItemsKeySimple() string {
	return m.getSelectItemsKeySimple()
}

func (m *standardOpenAPIOperationStore) getSelectItemsKeySimple() string {
	if m.Response != nil {
		return m.Response.ObjectKey
	}
	return ""
}

func (m *standardOpenAPIOperationStore) GetKey(lhs string) (interface{}, error) {
	val, ok := m.ToPresentationMap(true)[lhs]
	if !ok {
		return nil, fmt.Errorf("key '%s' no preset in metadata_method", lhs)
	}
	return val, nil
}

func (m *standardOpenAPIOperationStore) GetColumnOrder(extended bool) []string {
	retVal := []string{
		MethodName,
		RequiredParams,
		SQLVerb,
	}
	if extended {
		retVal = append(retVal, MethodDescription)
	}
	return retVal
}

func (m *standardOpenAPIOperationStore) IsAwaitable() bool {
	rs, _, err := m.GetResponseBodySchemaAndMediaType()
	if err != nil {
		return false
	}
	return strings.HasSuffix(rs.getKey(), "Operation")
}

func (m *standardOpenAPIOperationStore) FilterBy(predicate func(interface{}) (ITable, error)) (ITable, error) {
	return predicate(m)
}

func (m *standardOpenAPIOperationStore) GetKeyAsSqlVal(lhs string) (sqltypes.Value, error) {
	val, ok := m.ToPresentationMap(true)[lhs]
	rv, err := InterfaceToSQLType(val)
	if !ok {
		return rv, fmt.Errorf("key '%s' no preset in metadata_service", lhs)
	}
	return rv, err
}

// This method needs to incorporate request body parameters
func (m *standardOpenAPIOperationStore) GetRequiredParameters() map[string]Addressable {
	return m.getRequiredParameters()
}

func (m *standardOpenAPIOperationStore) getRequestBodyAttributes() (map[string]Addressable, error) {
	s, err := m.getRequestBodySchema()
	if err != nil {
		return nil, err
	}
	rv := make(map[string]Addressable)
	if s != nil {
		propz := s.getProperties()
		for k, v := range propz {
			isRequired := slices.Contains(s.GetRequired(), k)
			renamedKey, keyRenameErr := m.renameRequestBodyAttribute(k)
			if keyRenameErr != nil {
				return nil, keyRenameErr
			}
			if isRequired {
				rv[renamedKey] = NewRequiredAddressableRequestBodyProperty(renamedKey, v)
			} else {
				rv[renamedKey] = NewOptionalAddressableRequestBodyProperty(renamedKey, v)
			}
		}
	}
	return rv, nil
}

func (m *standardOpenAPIOperationStore) getRequestBodyAttributesNoRename() (map[string]Addressable, error) {
	s, err := m.getRequestBodySchema()
	if err != nil {
		return nil, err
	}
	rv := make(map[string]Addressable)
	if s != nil {
		propz := s.getProperties()
		for k, v := range propz {
			isRequired := slices.Contains(s.GetRequired(), k)
			if isRequired {
				rv[k] = NewRequiredAddressableRequestBodyProperty(k, v)
			} else {
				rv[k] = NewOptionalAddressableRequestBodyProperty(k, v)
			}
		}
	}
	return rv, nil
}

func (m *standardOpenAPIOperationStore) getRequiredRequestBodyAttributes() (map[string]Addressable, error) {
	return m.getIndicatedRequestBodyAttributes(true)
}

func (m *standardOpenAPIOperationStore) getOptionalRequestBodyAttributes() (map[string]Addressable, error) {
	return m.getIndicatedRequestBodyAttributes(false)
}

func (m *standardOpenAPIOperationStore) getIndicatedRequestBodyAttributes(required bool) (map[string]Addressable, error) {
	rv := make(map[string]Addressable)
	allAttr, err := m.getRequestBodyAttributes()
	if err != nil {
		return nil, err
	}
	for k, v := range allAttr {
		if v.IsRequired() == required {
			rv[k] = v
		}
	}
	return rv, nil
}

func (m *standardOpenAPIOperationStore) RenameRequestBodyAttribute(k string) (string, error) {
	return m.renameRequestBodyAttribute(k)
}

func (m *standardOpenAPIOperationStore) renameRequestBodyAttribute(k string) (string, error) {
	paramTranslator, translatorInferErr := m.inferTranslator(m.getRequestBodyTranslateAlgorithmString())
	if translatorInferErr != nil {
		return "", translatorInferErr
	}
	output, outputErr := paramTranslator.Translate(k)
	return output, outputErr
}

func (m *standardOpenAPIOperationStore) RevertRequestBodyAttributeRename(k string) (string, error) {
	return m.revertRequestBodyAttributeRename(k)
}

func (m *standardOpenAPIOperationStore) revertRequestBodyAttributeRename(k string) (string, error) {
	paramTranslator, translatorInferErr := m.inferTranslator(m.getRequestBodyTranslateAlgorithmString())
	if translatorInferErr != nil {
		return "", translatorInferErr
	}
	output, outputErr := paramTranslator.ReverseTranslate(k)
	return output, outputErr
}

func (m *standardOpenAPIOperationStore) getRequestBodyAttributeParentKey(algorithm string) (string, bool) {
	algorithmPrefix := extractAlgorithmPrefix(algorithm)
	algorithmSuffix := extractAlgorithmSuffix(algorithm, algorithmPrefix)
	if algorithmPrefix == translateAlgorithmNaiveNaming {
		return algorithmSuffix, true
	}
	return "", false
}

// func (op *standardOpenAPIOperationStore) getRequestBodyAttributeLineage(rawKey string) (string, error) {
// 	return "", nil
// }

func (m *standardOpenAPIOperationStore) getDefaultRequestBodyMatcher() fuzzymatch.FuzzyMatcher[string] {
	return requestBodyBaseKeyFuzzyMatcher
}

func (m *standardOpenAPIOperationStore) getRequestBodySchemaAttributeMatcher(path string) (fuzzymatch.FuzzyMatcher[string], error) {
	schemaOfInterest, err := m.getRequestBodySchema()
	if err != nil {
		return nil, err
	}
	if path != "" {
		schemaOfInterest = schemaOfInterest.FindByPath(path, map[string]bool{})
		if schemaOfInterest == nil {
			return nil, fmt.Errorf("could not find schema at path '%s'", path)
		}
	}
	return getschemaAttributeMatcher(schemaOfInterest)
}

func getschemaAttributeMatcher(schemaOfInterest Schema) (fuzzymatch.FuzzyMatcher[string], error) {
	var matchers []fuzzymatch.StringFuzzyPair
	for k := range schemaOfInterest.getProperties() {
		if k == "" {
			return nil, fmt.Errorf("empty key in schema")
		}
		keyRegexpStr := fmt.Sprintf("^%s$", regexp.QuoteMeta(k))
		keyRegexp, regexpErr := regexp.Compile(keyRegexpStr)
		if regexpErr != nil {
			return nil, regexpErr
		}
		matchers = append(matchers, fuzzymatch.NewFuzzyPair(keyRegexp, k))
	}
	return fuzzymatch.NewRegexpStringMetcher(matchers), nil
}

func extractAlgorithmSuffix(algorithm string, prefix string) string {
	trimmed := strings.TrimPrefix(algorithm, fmt.Sprintf("%s_", prefix))
	if trimmed == algorithm {
		return ""
	}
	return trimmed
}

func extractAlgorithmPrefix(algorithm string) string {
	if strings.HasPrefix(algorithm, translateAlgorithmNaiveNaming) {
		return translateAlgorithmNaiveNaming
	}
	if strings.HasPrefix(algorithm, translateAlgorithmDefault) {
		return translateAlgorithmDefault
	}
	return algorithm
}

func (m *standardOpenAPIOperationStore) inferTranslator(algorithm string) (parametertranslate.ParameterTranslator, error) {
	algorithmPrefix := extractAlgorithmPrefix(algorithm)
	algorithmSuffix := extractAlgorithmSuffix(algorithm, algorithmPrefix)
	switch algorithmPrefix {
	case "", translateAlgorithmDefault:
		requestBodyMatcher := m.getDefaultRequestBodyMatcher()
		algorithmName := fmt.Sprintf("%s%s", parametertranslate.GetPrefixPrefix(), requestBodyBaseKey)
		return parametertranslate.NewParameterTranslator(
			algorithmName,
			requestBodyMatcher,
		), nil
	case translateAlgorithmNaiveNaming:
		requestBodyMatcher, err := m.getRequestBodySchemaAttributeMatcher(algorithmSuffix)
		if err != nil {
			return nil, err
		}
		return parametertranslate.NewNaiveBodyTranslator(
			algorithmSuffix,
			requestBodyMatcher,
		), nil
	default:
		return nil, fmt.Errorf("unsupported request body parameter translation algorithm '%s'", algorithm)
	}
}

func (m *standardOpenAPIOperationStore) getRequestBodyTranslateAlgorithmString() string {
	retVal := ""
	cfg, cfgExists := m.getStackQLConfig()
	if cfgExists {
		requestBodyTranslate, requestBodyTranslateExists := cfg.GetRequestBodyTranslate()
		if requestBodyTranslateExists {
			algorithmStr := requestBodyTranslate.GetAlgorithm()
			if algorithmStr != "" {
				retVal = algorithmStr
			}
		}
	}
	return retVal
}

func (m *standardOpenAPIOperationStore) IsRequestBodyAttributeRenamed(k string) bool {
	paramTranslator, translatorInferErr := m.inferTranslator(m.getRequestBodyTranslateAlgorithmString())
	if translatorInferErr != nil {
		return false
	}
	_, outputErr := paramTranslator.ReverseTranslate(k)
	return outputErr == nil
}

func (m *standardOpenAPIOperationStore) GetRequiredNonBodyParameters() map[string]Addressable {
	return m.getRequiredNonBodyParameters()
}

func (m *standardOpenAPIOperationStore) getRequiredNonBodyParameters() map[string]Addressable {
	retVal := make(map[string]Addressable)
	for k, v := range m.Parameters {
		b, err := json.Marshal(v)
		if err != nil {
			continue
		}
		var param openapi3.Parameter
		err = json.Unmarshal(b, &param)
		if err != nil {
			continue
		}
		paramObj := NewParameter(&param, m.OpenAPIService)
		if paramObj.IsRequired() {
			retVal[k] = paramObj
		}
	}
	if m.PathItem != nil {
		for _, p := range m.PathItem.Parameters {
			param := p.Value
			if param != nil && isOpenapi3ParamRequired(param) {
				retVal[param.Name] = NewParameter(p.Value, m.OpenAPIService)
			}
		}
	}
	if m.OperationRef == nil || m.OperationRef.Value == nil || m.OperationRef.Value.Parameters == nil {
		return retVal
	}
	for _, p := range m.OperationRef.Value.Parameters {
		param := p.Value
		if param != nil && isOpenapi3ParamRequired(param) {
			retVal[param.Name] = NewParameter(p.Value, m.OpenAPIService)
		}
	}
	return retVal
}

func (m *standardOpenAPIOperationStore) getRequiredParameters() map[string]Addressable {
	retVal := m.getRequiredNonBodyParameters()
	ss, err := m.getRequiredRequestBodyAttributes()
	if err != nil {
		return retVal
	}
	for k, v := range ss {
		retVal[k] = v
	}
	availableServers, availableServersDoExist := m.getServers()
	if availableServersDoExist {
		sv := availableServers[0]
		serverVarMap := getServerVariablesMap(sv, m.OpenAPIService)
		for k, v := range serverVarMap {
			retVal[k] = v
		}
	}
	return retVal
}

// This method needs to incorporate request body parameters
func (m *standardOpenAPIOperationStore) GetOptionalParameters() map[string]Addressable {
	return m.getOptionalParameters()
}

func (m *standardOpenAPIOperationStore) getOptionalParameters() map[string]Addressable {
	retVal := make(map[string]Addressable)
	for k, v := range m.Parameters {
		b, err := json.Marshal(v)
		if err != nil {
			continue
		}
		var param openapi3.Parameter
		err = json.Unmarshal(b, &param)
		if err != nil {
			continue
		}
		paramObj := NewParameter(&param, m.OpenAPIService)
		if !paramObj.IsRequired() {
			retVal[k] = paramObj
		}
	}
	if m.OperationRef == nil || m.OperationRef.Value == nil || m.OperationRef.Value.Parameters == nil {
		return retVal
	}
	for _, p := range m.OperationRef.Value.Parameters {
		param := p.Value
		// TODO: handle the `?param` where value is not only not required but should NEVER be sent
		if param != nil && !param.Required {
			retVal[param.Name] = NewParameter(p.Value, m.OpenAPIService)
		}
	}
	ss, err := m.getOptionalRequestBodyAttributes()
	if err != nil {
		return retVal
	}
	for k, v := range ss {
		retVal[k] = v
	}
	return retVal
}

func (ops *standardOpenAPIOperationStore) getMethod() (*openapi3.Operation, error) {
	if ops.OperationRef != nil && ops.OperationRef.Value != nil {
		return ops.OperationRef.Value, nil
	}
	return nil, fmt.Errorf("no method attached to operation store")
}

func (m *standardOpenAPIOperationStore) getNonBodyParameters() map[string]Addressable {
	retVal := make(map[string]Addressable)
	if m.PathItem != nil {
		for _, p := range m.PathItem.Parameters {
			param := p.Value
			if param != nil {
				retVal[param.Name] = NewParameter(p.Value, m.OpenAPIService)
			}
		}
	}
	if m.OperationRef == nil || m.OperationRef.Value.Parameters == nil {
		return retVal
	}
	for _, p := range m.OperationRef.Value.Parameters {
		param := p.Value
		if param != nil {
			retVal[param.Name] = NewParameter(p.Value, m.OpenAPIService)
		}
	}
	return retVal
}

func (m *standardOpenAPIOperationStore) GetParameters() map[string]Addressable {
	retVal := m.getNonBodyParameters()
	ss, err := m.getRequestBodyAttributes()
	if err != nil {
		return retVal
	}
	for k, v := range ss {
		retVal[k] = v
	}
	return retVal
}

func (m *standardOpenAPIOperationStore) GetNonBodyParameters() map[string]Addressable {
	return m.getNonBodyParameters()
}

func (m *standardOpenAPIOperationStore) GetParameter(paramKey string) (Addressable, bool) {
	params := m.GetParameters()
	rv, ok := params[paramKey]
	return rv, ok
}

func (m *standardOpenAPIOperationStore) GetName() string {
	return m.getName()
}

func (m *standardOpenAPIOperationStore) getName() string {
	if m.OperationRef != nil && m.OperationRef.Value != nil && m.OperationRef.Value.OperationID != "" {
		return m.OperationRef.Value.OperationID
	}
	return m.MethodKey
}

func (m *standardOpenAPIOperationStore) GetRequestBodyAttributesNoRename() (map[string]Addressable, error) {
	return m.getRequestBodyAttributesNoRename()
}

func (m *standardOpenAPIOperationStore) ToPresentationMap(extended bool) map[string]interface{} {
	requiredParams := m.getRequiredNonBodyParameters()
	var requiredParamNames []string
	for s := range requiredParams {
		requiredParamNames = append(requiredParamNames, s)
	}
	var requiredBodyParamNames []string
	rs, _ := m.getRequestBodyAttributesNoRename()
	for k, v := range rs {
		isRequiredFromMethodAnnotation := false
		if m.Request != nil && len(m.Request.Required) > 0 {
			isRequiredFromMethodAnnotation = slices.Contains(m.Request.Required, k)
		}
		if v.IsRequired() || isRequiredFromMethodAnnotation {
			renamedKey, renamedKeyErr := m.renameRequestBodyAttribute(k)
			if renamedKeyErr != nil {
				requiredBodyParamNames = append(requiredBodyParamNames, k)
				continue
			}
			requiredBodyParamNames = append(requiredBodyParamNames, renamedKey)
		}
	}

	var requiredServerParamNames []string
	availableServers, availableServersDoExist := m.getServers()
	if availableServersDoExist {
		sv := availableServers[0]
		serverVarMap := getServerVariablesMap(sv, m.OpenAPIService)
		for k := range serverVarMap {
			requiredServerParamNames = append(requiredServerParamNames, k)
		}
	}

	sort.Strings(requiredParamNames)
	sort.Strings(requiredBodyParamNames)
	sort.Strings(requiredServerParamNames)
	requiredParamNames = append(requiredParamNames, requiredBodyParamNames...)
	requiredParamNames = append(requiredParamNames, requiredServerParamNames...)

	sqlVerb := m.SQLVerb
	if sqlVerb == "" {
		sqlVerb = "EXEC"
	}

	retVal := map[string]interface{}{
		MethodName:     m.MethodKey,
		RequiredParams: strings.Join(requiredParamNames, ", "),
		SQLVerb:        strings.ToUpper(sqlVerb),
	}
	if extended {
		retVal[MethodDescription] = m.OperationRef.Value.Description
	}
	return retVal
}

func (op *standardOpenAPIOperationStore) GetOperationParameters() Params {
	return NewParameters(op.OperationRef.Value.Parameters, op.OpenAPIService)
}

func (op *standardOpenAPIOperationStore) GetOperationParameter(key string) (Addressable, bool) {
	paramLocal, isParamLocal := op.Parameters[key]
	if isParamLocal {
		b, err := json.Marshal(paramLocal)
		if err != nil {
			return nil, false
		}
		var param openapi3.Parameter
		err = json.Unmarshal(b, &param)
		if err != nil {
			return nil, false
		}
		if param.Name == "" && param.In == "inline" {
			param.Name = key
		}
		paramObj := NewParameter(&param, op.OpenAPIService)
		return paramObj, true
	}
	if op.OperationRef == nil || op.OperationRef.Value == nil || op.OperationRef.Value.Parameters == nil {
		return nil, false
	}
	params := NewParameters(op.OperationRef.Value.Parameters, op.GetService())
	if op.OperationRef.Value.Parameters == nil {
		return nil, false
	}
	return params.GetParameter(key)
}

func (op *standardOpenAPIOperationStore) getServerVariable(key string) (*openapi3.ServerVariable, bool) {
	srvs, _ := op.getServers()
	for _, srv := range srvs {
		v, ok := srv.Variables[key]
		if ok {
			return v, true
		}
	}
	return nil, false
}

func getServersFromHeirarchy(op *standardOpenAPIOperationStore) openapi3.Servers {
	if op.OperationRef != nil && op.OperationRef.Value != nil && op.OperationRef.Value.Servers != nil && len(*op.OperationRef.Value.Servers) > 0 {
		return *op.OperationRef.Value.Servers
	}
	if op.PathItem != nil && len(op.PathItem.Servers) > 0 {
		return op.PathItem.Servers
	}
	return nil
}

func selectServer(servers openapi3.Servers, inputParams map[string]interface{}) (string, error) {
	paramsConformed := make(map[string]string)
	for k, v := range inputParams {
		switch v := v.(type) {
		case string:
			paramsConformed[k] = v
		}
	}
	srvs, err := obtainServerURLsFromServers(servers, paramsConformed)
	if err != nil {
		return "", err
	}
	return urltranslate.SanitiseServerURL(srvs[0])
}

func (op *standardOpenAPIOperationStore) acceptPathParam(mutableParamMap map[string]interface{}) {}

func (op *standardOpenAPIOperationStore) MarshalBody(body interface{}, expectedRequest ExpectedRequest) ([]byte, error) {
	return op.marshalBody(body, expectedRequest)
}

func (op *standardOpenAPIOperationStore) marshalBody(body interface{}, expectedRequest ExpectedRequest) ([]byte, error) {
	mediaType := expectedRequest.GetBodyMediaType()
	if expectedRequest.GetSchema() != nil {
		mediaType = expectedRequest.GetSchema().extractMediaTypeSynonym(mediaType)
	}
	switch mediaType {
	case media.MediaTypeJson:
		return json.Marshal(body)
	case media.MediaTypeXML, media.MediaTypeTextXML:
		return xmlmap.MarshalXMLUserInput(
			body,
			expectedRequest.GetSchema().getXMLALiasOrName(),
			op.getXMLTransform(),
			op.getXMLDeclaration(),
			op.getXMLRootAnnotation(),
		)
	}
	return nil, fmt.Errorf("media type = '%s' not supported", expectedRequest.GetBodyMediaType())
}

type AnalyzedInput interface {
	GetQueryParams() map[string]any
	GetHeaderParam(string) (string, bool)
	GetPathParam(string) (string, bool)
	GetServerVars() map[string]any
	GetREquestBody() any
}

func (op *standardOpenAPIOperationStore) parameterizeFromAnalyzedInput(prov Provider, parentDoc Service, inputParams AnalyzedInput) (*openapi3filter.RequestValidationInput, error) {

	params := op.OperationRef.Value.Parameters
	pathParams := make(map[string]string)
	q := make(url.Values)
	prefilledHeader := make(http.Header)

	queryParamsRemaining := inputParams.GetQueryParams()
	for _, p := range params {
		if p.Value == nil {
			continue
		}
		name := p.Value.Name

		if p.Value.In == openapi3.ParameterInHeader {
			val, present := inputParams.GetHeaderParam(p.Value.Name)
			if present {
				prefilledHeader.Set(name, fmt.Sprintf("%v", val))
			} else if p.Value != nil && p.Value.Schema != nil && p.Value.Schema.Value != nil && p.Value.Schema.Value.Default != nil {
				prefilledHeader.Set(name, fmt.Sprintf("%v", p.Value.Schema.Value.Default))
			} else if isOpenapi3ParamRequired(p.Value) {
				return nil, fmt.Errorf("standardOpenAPIOperationStore.parameterize() failure; missing required header '%s'", name)
			}
		}
		if p.Value.In == openapi3.ParameterInPath {
			val, present := inputParams.GetPathParam(p.Value.Name)
			if present {
				pathParams[name] = fmt.Sprintf("%v", val)
			}
			if !present && isOpenapi3ParamRequired(p.Value) {
				return nil, fmt.Errorf("standardOpenAPIOperationStore.parameterize() failure; missing required path parameter '%s'", name)
			}
		} else if p.Value.In == openapi3.ParameterInQuery {

			pVal, present := queryParamsRemaining[p.Value.Name]
			if present {
				switch val := pVal.(type) {
				case []interface{}:
					for _, v := range val {
						q.Add(name, fmt.Sprintf("%v", v))
					}
				default:
					q.Set(name, fmt.Sprintf("%v", val))
				}
				delete(queryParamsRemaining, name)
			}
		}
	}
	for k, v := range queryParamsRemaining {
		q.Set(k, fmt.Sprintf("%v", v))
		delete(queryParamsRemaining, k)
	}
	openapiSvc, openapiSvcOk := op.OpenAPIService.(OpenAPIService)
	if !openapiSvcOk {
		return nil, fmt.Errorf("could not cast OpenAPIService to standardOpenAPIServiceStore")
	}
	router, err := queryrouter.NewRouter(openapiSvc.getT())
	if err != nil {
		return nil, err
	}
	servers, _ := op.getServers()
	serverParams := inputParams.GetServerVars()
	if err != nil {
		return nil, err
	}
	sv, err := selectServer(servers, serverParams)
	if err != nil {
		return nil, err
	}
	contentTypeHeaderRequired := false
	var bodyReader io.Reader

	requestBody := inputParams.GetREquestBody()

	predOne := !util.IsNil(requestBody)
	predTwo := !util.IsNil(op.Request)
	if predOne && predTwo {
		b, err := op.marshalBody(requestBody, op.Request)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
		contentTypeHeaderRequired = true
	}
	// TODO: clean up
	sv = strings.TrimSuffix(sv, "/")
	path := replaceSimpleStringVars(fmt.Sprintf("%s%s", sv, op.OperationRef.extractPathItem()), pathParams)
	u, err := url.Parse(fmt.Sprintf("%s?%s", path, q.Encode()))
	if strings.Contains(path, "?") {
		if len(q) > 0 {
			u, err = url.Parse(fmt.Sprintf("%s&%s", path, q.Encode()))
		} else {
			u, err = url.Parse(path)
		}
	}
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(strings.ToUpper(op.OperationRef.extractMethodItem()), u.String(), bodyReader)
	if err != nil {
		return nil, err
	}
	if contentTypeHeaderRequired {
		if prefilledHeader.Get("Content-Type") != "" {
			prefilledHeader.Set("Content-Type", op.Request.BodyMediaType)
		}
	}
	httpReq.Header = prefilledHeader
	route, checkedPathParams, err := router.FindRoute(httpReq)
	if err != nil {
		return nil, err
	}
	options := &openapi3filter.Options{
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	}
	// Validate request
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Options:    options,
		PathParams: checkedPathParams,
		Request:    httpReq,
		Route:      route,
	}
	return requestValidationInput, nil
}

func (op *standardOpenAPIOperationStore) parameterize(prov Provider, parentDoc Service, inputParams HttpParameters, requestBody interface{}) (*openapi3filter.RequestValidationInput, error) {

	params := op.OperationRef.Value.Parameters
	copyParams := make(map[string]interface{})
	flatParameters, err := inputParams.ToFlatMap()
	if err != nil {
		return nil, err
	}
	for k, v := range flatParameters {
		copyParams[k] = v
	}
	pathParams := make(map[string]string)
	q := make(url.Values)
	prefilledHeader := make(http.Header)
	for _, p := range params {
		if p.Value == nil {
			continue
		}
		name := p.Value.Name

		if p.Value.In == openapi3.ParameterInHeader {
			val, present := inputParams.GetParameter(p.Value.Name, openapi3.ParameterInHeader)
			if present {
				prefilledHeader.Set(name, fmt.Sprintf("%v", val.GetVal()))
				delete(copyParams, name)
			} else if p.Value != nil && p.Value.Schema != nil && p.Value.Schema.Value != nil && p.Value.Schema.Value.Default != nil {
				prefilledHeader.Set(name, fmt.Sprintf("%v", p.Value.Schema.Value.Default))
			} else if isOpenapi3ParamRequired(p.Value) {
				return nil, fmt.Errorf("standardOpenAPIOperationStore.parameterize() failure; missing required header '%s'", name)
			}
		}
		if p.Value.In == openapi3.ParameterInPath {
			val, present := inputParams.GetParameter(p.Value.Name, openapi3.ParameterInPath)
			if present {
				pathParams[name] = fmt.Sprintf("%v", val.GetVal())
				delete(copyParams, name)
			}
			if !present && isOpenapi3ParamRequired(p.Value) {
				return nil, fmt.Errorf("standardOpenAPIOperationStore.parameterize() failure; missing required path parameter '%s'", name)
			}
		} else if p.Value.In == openapi3.ParameterInQuery {
			queryParamsRemaining, err := inputParams.GetRemainingQueryParamsFlatMap(copyParams)
			if err != nil {
				return nil, err
			}
			pVal, present := queryParamsRemaining[p.Value.Name]
			if present {
				switch val := pVal.(type) {
				case []interface{}:
					for _, v := range val {
						q.Add(name, fmt.Sprintf("%v", v))
					}
				default:
					q.Set(name, fmt.Sprintf("%v", val))
				}
				delete(copyParams, name)
			}
		}
	}
	queryParamsRemaining, err := inputParams.GetRemainingQueryParamsFlatMap(copyParams)
	if err != nil {
		return nil, err
	}
	for k, v := range queryParamsRemaining {
		q.Set(k, fmt.Sprintf("%v", v))
		delete(copyParams, k)
	}
	openapiSvc, openapiSvcOk := op.OpenAPIService.(OpenAPIService)
	if !openapiSvcOk {
		return nil, fmt.Errorf("could not cast OpenAPIService to standardOpenAPIServiceStore")
	}
	router, err := queryrouter.NewRouter(openapiSvc.getT())
	if err != nil {
		return nil, err
	}
	servers, _ := op.getServers()
	serverParams, err := inputParams.GetServerParameterFlatMap()
	if err != nil {
		return nil, err
	}
	sv, err := selectServer(servers, serverParams)
	if err != nil {
		return nil, err
	}
	contentTypeHeaderRequired := false
	var bodyReader io.Reader
	predOne := !util.IsNil(requestBody)
	predTwo := !util.IsNil(op.Request)
	if predOne && predTwo {
		b, err := op.marshalBody(requestBody, op.Request)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
		contentTypeHeaderRequired = true
	}
	// TODO: clean up
	sv = strings.TrimSuffix(sv, "/")
	path := replaceSimpleStringVars(fmt.Sprintf("%s%s", sv, op.OperationRef.extractPathItem()), pathParams)
	u, err := url.Parse(fmt.Sprintf("%s?%s", path, q.Encode()))
	if strings.Contains(path, "?") {
		if len(q) > 0 {
			u, err = url.Parse(fmt.Sprintf("%s&%s", path, q.Encode()))
		} else {
			u, err = url.Parse(path)
		}
	}
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(strings.ToUpper(op.OperationRef.extractMethodItem()), u.String(), bodyReader)
	if err != nil {
		return nil, err
	}
	if contentTypeHeaderRequired {
		if prefilledHeader.Get("Content-Type") != "" {
			prefilledHeader.Set("Content-Type", op.Request.BodyMediaType)
		}
	}
	httpReq.Header = prefilledHeader
	route, checkedPathParams, err := router.FindRoute(httpReq)
	if err != nil {
		return nil, err
	}
	options := &openapi3filter.Options{
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	}
	// Validate request
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Options:    options,
		PathParams: checkedPathParams,
		Request:    httpReq,
		Route:      route,
	}
	return requestValidationInput, nil
}

func (op *standardOpenAPIOperationStore) GetRequestBodySchema() (Schema, error) {
	return op.getRequestBodySchema()
}

func (op *standardOpenAPIOperationStore) getRequestBodySchema() (Schema, error) {
	if op.Request != nil {
		return op.Request.Schema, nil
	}
	return nil, fmt.Errorf("no request body for operation =  %s", op.GetName())
}

func (op *standardOpenAPIOperationStore) GetRequestBodyRequiredProperties() ([]string, error) {
	if op.Request != nil {
		return op.Request.Required, nil
	}
	return nil, fmt.Errorf("no request body required elements for operation =  %s", op.GetName())
}

func (op *standardOpenAPIOperationStore) IsRequiredRequestBodyProperty(key string) bool {
	if op.Request == nil || op.Request.Required == nil {
		return false
	}
	for _, k := range op.Request.Required {
		if k == key {
			return true
		}
	}
	return false
}

func (op *standardOpenAPIOperationStore) GetResponseBodySchemaAndMediaType() (Schema, string, error) {
	return op.getResponseBodySchemaAndMediaType()
}

func (op *standardOpenAPIOperationStore) GetFinalResponseBodySchemaAndMediaType() (Schema, string, error) {
	return op.getFinalResponseBodySchemaAndMediaType()
}

func (op *standardOpenAPIOperationStore) getFinalResponseBodySchemaAndMediaType() (Schema, string, error) {
	if op.Response != nil && op.Response.AsyncOverrideSchema != nil && op.Response.AsyncOverrideSchema.Value != nil {
		return op.Response.AsyncOverrideSchema.Value, op.Response.AsyncOverrideBodyMediaType, nil
	}
	return op.getResponseBodySchemaAndMediaType()
}

func (op *standardOpenAPIOperationStore) getResponseBodySchemaAndMediaType() (Schema, string, error) {
	if op.Response != nil && op.Response.OverrideSchema != nil && op.Response.OverrideSchema.Value != nil {
		return op.Response.OverrideSchema.Value, op.Response.OverrideBodyMediaType, nil
	}
	if op.Response != nil && op.Response.Schema != nil {
		mediaType := op.Response.BodyMediaType
		if op.Response.OverrideBodyMediaType != "" {
			mediaType = op.Response.OverrideBodyMediaType
		}
		return op.Response.Schema, mediaType, nil
	}
	return nil, "", fmt.Errorf("no response body for operation =  %s", op.GetName())
}

func (op *standardOpenAPIOperationStore) ShouldBeSelectable() bool {
	itemsKey := op.lookupSelectItemsKey()
	if op.Response != nil && op.Response.OverrideSchema != nil {
		return true
	}
	if op.Response != nil && op.Response.Schema != nil {
		return true
	}
	return itemsKey != ""
}

func (op *standardOpenAPIOperationStore) GetSelectSchemaAndObjectPath() (Schema, string, error) {
	k := op.lookupSelectItemsKey()
	if op.Response != nil && op.Response.OverrideSchema != nil && op.Response.OverrideSchema.Value != nil {
		return op.Response.OverrideSchema.Value.getSelectItemsSchema(k, op.Response.OverrideBodyMediaType)
	}
	if op.Response != nil && op.Response.Schema != nil {
		return op.Response.Schema.getSelectItemsSchema(k, op.getOptimalResponseMediaType())
	}
	return nil, "", fmt.Errorf("no response body for operation =  %s", op.GetName())
}

func (op *standardOpenAPIOperationStore) GetSchemaAtPath(path string) (Schema, error) {
	k := path
	if op.Response != nil && op.Response.OverrideSchema != nil && op.Response.OverrideSchema.Value != nil {
		rv, _, err := op.Response.OverrideSchema.Value.getSelectItemsSchema(k, op.Response.OverrideBodyMediaType)
		return rv, err
	}
	if op.Response != nil && op.Response.Schema != nil {
		rv, _, err := op.Response.Schema.getSelectItemsSchema(k, op.getOptimalResponseMediaType())
		return rv, err
	}
	return nil, fmt.Errorf("no response body for operation =  %s", op.GetName())
}

type ProcessedOperationResponse interface {
	GetResponse() (response.Response, bool)
	GetReversal() (HTTPPreparator, bool)
	GetReversalError() (error, bool)
	setReversalError(error)
}

func newStandardOperationResponse(response response.Response, reversal HTTPPreparator) ProcessedOperationResponse {
	return &standardOperationResponse{
		response: response,
		reversal: reversal,
	}
}

type standardOperationResponse struct {
	response      response.Response
	reversal      HTTPPreparator
	reversalError error
}

func (sor *standardOperationResponse) GetReversalError() (error, bool) {
	return sor.reversalError, sor.reversalError != nil
}

func (sor *standardOperationResponse) setReversalError(err error) {
	sor.reversalError = err
}

func (sor *standardOperationResponse) GetResponse() (response.Response, bool) {
	return sor.response, sor.response != nil
}

func (sor *standardOperationResponse) GetReversal() (HTTPPreparator, bool) {
	return sor.reversal, sor.reversal != nil
}

func (op *standardOpenAPIOperationStore) isOverridable(httpResponse *http.Response) bool {
	expectedResponse, isExpectedResponse := op.GetResponse()
	if isExpectedResponse {
		responseTransform, responseTransformExists := expectedResponse.GetTransform()
		overrideMediaType := expectedResponse.GetOverrrideBodyMediaType()
		if !responseTransformExists {
			return false
		}
		streamTransformerFactory := stream_transform.NewStreamTransformerFactory(
			responseTransform.GetType(),
			responseTransform.GetBody(),
		)
		return overrideMediaType != "" && streamTransformerFactory.IsTransformable()
	}
	return false
}

func (op *standardOpenAPIOperationStore) getOverridenResponse(httpResponse *http.Response, responseSchema Schema) (response.Response, error) {
	defer httpResponse.Body.Close()
	bodyBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}
	expectedResponse, isExpectedResponse := op.GetResponse()
	if isExpectedResponse {
		responseTransform, responseTransformExists := expectedResponse.GetTransform()
		overrideMediaType := expectedResponse.GetOverrrideBodyMediaType()
		if responseTransformExists {
			input := string(bodyBytes)
			streamTransformerFactory := stream_transform.NewStreamTransformerFactory(
				responseTransform.GetType(),
				responseTransform.GetBody(),
			)
			if !streamTransformerFactory.IsTransformable() {
				return nil, fmt.Errorf("unsupported template type: %s", responseTransform.GetType())
			}
			tfm, err := streamTransformerFactory.GetTransformer(input)
			if err != nil {
				return nil, fmt.Errorf("failed to transform: %v", err)
			}
			tfmErr := tfm.Transform()
			if tfmErr != nil {
				return nil, fmt.Errorf("failed to transform: %v", tfmErr)
			}
			outStream := tfm.GetOutStream()
			if overrideMediaType == "" {
				overrideMediaType = media.MediaTypeJson
			}
			processedResponse, rawResponse, err := responseSchema.unmarshalReaderResponseAtPath(outStream, op.lookupSelectItemsKey(), overrideMediaType, overrideMediaType)
			if err != nil {
				return nil, err
			}
			return response.NewResponse(processedResponse, rawResponse, httpResponse), nil
		}
	}
	return nil, fmt.Errorf("unprocessable response body for operation =  %s", op.GetName())
}

func (op *standardOpenAPIOperationStore) ProcessResponse(httpResponse *http.Response) (ProcessedOperationResponse, error) {
	responseSchema, mediaType, err := op.GetResponseBodySchemaAndMediaType()
	if err != nil {
		return nil, err
	}
	overrideMediaType := ""
	if op.Response != nil {
		overrideMediaType = op.Response.OverrideBodyMediaType
	}
	var rv response.Response
	if op.isOverridable(httpResponse) {
		rv, err = op.getOverridenResponse(httpResponse, responseSchema)
	} else {
		rv, err = responseSchema.processHttpResponse(httpResponse, op.lookupSelectItemsKey(), mediaType, overrideMediaType)
	}
	var reversal HTTPPreparator
	inverse, inverseExists := op.GetInverse()
	if inverseExists {
		inverseOpStore, inverseOpStoreExists := inverse.GetOperationStore()
		if inverseOpStoreExists {
			paramMap, err := inverse.GetParamMap(rv)
			if err != nil {
				retVal := newStandardOperationResponse(rv, nil)
				retVal.setReversalError(err)
				return retVal, nil
			}
			reversal = newHTTPPreparator(
				inverseOpStore.GetProvider(),
				inverseOpStore.GetService(),
				inverseOpStore,
				map[int]map[string]interface{}{
					0: paramMap,
				},
				nil,
				nil,
				nil,
			)
		}
	}
	return newStandardOperationResponse(rv, reversal), err
}

func (ops *standardOpenAPIOperationStore) LookupSelectItemsKey() string {
	return ops.lookupSelectItemsKey()
}

func (ops *standardOpenAPIOperationStore) lookupSelectItemsKey() string {
	s := ops.getSelectItemsKeySimple()
	if s != "" {
		return s
	}
	responseSchema, _, err := ops.GetResponseBodySchemaAndMediaType()
	if responseSchema == nil || err != nil {
		return ""
	}
	mediaType := responseSchema.GetType()
	if ops.Response != nil && ops.Response.OverrideBodyMediaType != "" {
		mediaType = ops.Response.OverrideBodyMediaType
	}
	switch mediaType {
	case "string", "integer":
		return AnonymousColumnName
	}
	if _, ok := responseSchema.getRawProperty(defaultSelectItemsKey); ok {
		return defaultSelectItemsKey
	}
	return ""
}

func (op *standardOpenAPIOperationStore) DeprecatedProcessResponse(response *http.Response) (map[string]interface{}, error) {
	responseSchema, _, err := op.GetResponseBodySchemaAndMediaType()
	if err != nil {
		return nil, err
	}
	return responseSchema.DeprecatedProcessHttpResponse(response, op.lookupSelectItemsKey())
}
