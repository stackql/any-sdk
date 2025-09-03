package radix_tree_address_space

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/media"
	"github.com/stackql/any-sdk/pkg/queryrouter"
	"github.com/stackql/any-sdk/pkg/urltranslate"
	"github.com/stackql/any-sdk/pkg/util"
	"github.com/stackql/any-sdk/pkg/xmlmap"
)

type pathType string

const (
	standardRequestName         = "request"
	standardResponseName        = "response"
	standardURLName             = "url"
	standardHeadersName         = "headers"
	standardBodyName            = "body"
	standardQueryName           = "query"
	standardPathName            = "path"
	cookiesName                 = "cookies"
	standardPostTransformPrefix = "post_transform_"
	// path types
	pathTypeRequestBody  pathType = "request_body"
	pathTypeResponseBody pathType = "response_body"
)

// AddressSpaceGrammar defines the search DSL
type AddressSpaceGrammar interface {
	ExtractSubPath(string, pathType) (string, bool)
}

type standardAddressSpaceGrammar struct {
	requestName         string
	responseName        string
	urlName             string
	headersName         string
	bodyName            string
	queryName           string
	pathName            string
	cookiesName         string
	postTransformPrefix string
	requestBodyPrefix   string
	responseBodyPrefix  string
}

func newStandardAddressSpaceGrammar() AddressSpaceGrammar {
	return &standardAddressSpaceGrammar{
		requestName:         standardRequestName,
		responseName:        standardResponseName,
		urlName:             standardURLName,
		headersName:         standardHeadersName,
		bodyName:            standardBodyName,
		queryName:           standardQueryName,
		pathName:            standardPathName,
		cookiesName:         cookiesName,
		postTransformPrefix: standardPostTransformPrefix,
		requestBodyPrefix:   fmt.Sprintf("%s.%s.", standardRequestName, standardBodyName),
		responseBodyPrefix:  fmt.Sprintf("%s.%s.", standardResponseName, standardBodyName),
	}
}

func (sg *standardAddressSpaceGrammar) extractRequestBodySubPath(fullPath string) (string, bool) {
	if strings.HasPrefix(fullPath, sg.requestBodyPrefix) {
		return strings.TrimPrefix(fullPath, sg.requestBodyPrefix), true
	}
	return "", false
}

func (sg *standardAddressSpaceGrammar) extractResponseBodySubPath(fullPath string) (string, bool) {
	rv := strings.TrimPrefix(fullPath, sg.responseBodyPrefix)
	return rv, rv != fullPath
}

func (sg *standardAddressSpaceGrammar) ExtractSubPath(fullPath string, pathType pathType) (string, bool) {
	switch pathType {
	case pathTypeRequestBody:
		return sg.extractRequestBodySubPath(fullPath)
	case pathTypeResponseBody:
		return sg.extractResponseBodySubPath(fullPath)
	default:
		return "", false
	}
}

// type standardAddressSpaceAnalyzerFactory struct {
// 	provider anysdk.Provider
// 	service  anysdk.Service
// 	resource anysdk.Resource
// 	method   anysdk.StandardOperationStore
// }

// func (asf *standardAddressSpaceAnalyzerFactory) CreateAddressSpaceAnalyzer() AddressSpaceAnalyzer {
// 	return NewAddressSpaceAnalyzer(
// 		NewAddressSpaceGrammar(),
// 		asf.method,
// 	)
// }

//	func (asg *standardAddressSpaceGrammar) Dereference(address string) (any, bool) {
//		return nil, false
//	}
func NewAddressSpaceGrammar() AddressSpaceGrammar {
	return newStandardAddressSpaceGrammar()
}

type AddressSpaceAnalyzer interface {
	Analyze() error
	GetAddressSpace() AddressSpace
}

type AddressSpace interface {
	GetServer() *openapi3.Server
	GetServerVars() map[string]string
	GetRequestBodyParams() map[string]anysdk.Addressable
	GetSimpleSelectKey() string
	GetSimpleSelectSchema() anysdk.Schema
	GetUnionSelectSchemas() map[string]anysdk.Schema
	DereferenceAddress(address string) (any, bool)
	WriteToAddress(address string, val any) error
	ReadFromAddress(address string) (any, bool)
	Analyze() error
	GetRequest() (*http.Request, bool)
}

type standardNamespace struct {
	serverVars            map[string]string
	requestBodyParams     map[string]anysdk.Addressable
	server                *openapi3.Server
	method                anysdk.StandardOperationStore
	simpleSelectKey       string
	simpleSelectSchema    anysdk.Schema
	responseBodySchema    anysdk.Schema
	requestBodySchema     anysdk.Schema
	responseBodyMediaType string
	requestBodyMediaType  string
	pathString            string
	serverUrlString       string
	request               *http.Request
	response              *http.Response
	unionSelectSchemas    map[string]anysdk.Schema
	shadowQuery           RadixTree
}

func selectServer(servers openapi3.Servers, inputParams map[string]interface{}) (string, error) {
	paramsConformed := make(map[string]string)
	for k, v := range inputParams {
		switch v := v.(type) {
		case string:
			paramsConformed[k] = v
		}
	}
	srvs, err := anysdk.ObtainServerURLsFromServers(servers, paramsConformed)
	if err != nil {
		return "", err
	}
	return urltranslate.SanitiseServerURL(srvs[0])
}

func (ns *standardNamespace) Analyze() error {
	var err error
	// idea here is to poulate the request object from the shadow query
	// e.g. if shadow query has "query.project" = "my-project"
	req := &http.Request{}
	serverVarsSuplied := ns.shadowQuery.ToFlatMap("server")
	ns.serverUrlString, err = selectServer([]*openapi3.Server{ns.server}, serverVarsSuplied)
	if err != nil {
		return err
	}
	matureURL, err := url.Parse(ns.serverUrlString)
	if err != nil {
		return err
	}

	req.URL = matureURL
	ns.request = req
	return nil
}

func (ns *standardNamespace) GetRequest() (*http.Request, bool) {
	if ns.request != nil {
		return ns.request, true
	}
	return nil, false
}

func (ns *standardNamespace) WriteToAddress(address string, val any) error {
	err := ns.shadowQuery.Insert(address, val)
	return err
}

func (ns *standardNamespace) ReadFromAddress(address string) (any, bool) {
	val, ok := ns.shadowQuery.Find(address)
	return val, ok
}

func (ns *standardNamespace) DereferenceAddress(address string) (any, bool) {
	parts := strings.Split(address, ".")
	if len(parts) == 0 {
		return nil, false
	}
	if parts[0] == "" {
		if len(parts) < 2 {
			return nil, false
		}
		if ns.method == nil {
			return nil, false
		}
		return ns.method.GetParameter(parts[1])
	}
	if parts[0] == standardRequestName {
		if len(parts) < 2 {
			return nil, false
		}
		switch parts[1] {
		case standardBodyName:
			return ns.requestBodySchema, true
		case standardHeadersName:
			return ns.request.Header, true
		default:
			return nil, false
		}
	}
	if parts[0] == standardResponseName {
		if len(parts) < 2 {
			return nil, false
		}
		switch parts[1] {
		case standardHeadersName:
			return ns.server.Variables, true
		case standardBodyName:
			return ns.responseBodySchema, true
		default:
			return nil, false
		}
	}
	return nil, false
}

func (ns *standardNamespace) GetServer() *openapi3.Server {
	return ns.server
}

func (ns *standardNamespace) GetServerVars() map[string]string {
	return ns.serverVars
}

func (ns *standardNamespace) GetRequestBodyParams() map[string]anysdk.Addressable {
	return ns.requestBodyParams
}

func (ns *standardNamespace) GetSimpleSelectKey() string {
	return ns.simpleSelectKey
}

func (ns *standardNamespace) GetSimpleSelectSchema() anysdk.Schema {
	return ns.simpleSelectSchema
}

func (ns *standardNamespace) GetUnionSelectSchemas() map[string]anysdk.Schema {
	return ns.unionSelectSchemas
}

type standardAddressSpaceAnalyzer struct {
	grammar                AddressSpaceGrammar
	provider               anysdk.Provider
	service                anysdk.Service
	resource               anysdk.Resource
	method                 anysdk.StandardOperationStore
	aliasedUnionSelectKeys map[string]string
	addressSpace           AddressSpace
}

func (asa *standardAddressSpaceAnalyzer) GetAddressSpace() AddressSpace {
	return asa.addressSpace
}

func (asa *standardAddressSpaceAnalyzer) Analyze() error {
	serverVars := make(map[string]string)
	servers, _ := asa.method.GetServers()
	svcServers, _ := asa.service.GetServers()
	servers = append(servers, svcServers...)
	if len(servers) == 0 {
		return fmt.Errorf("no servers defined for operation %s", asa.method.GetName())
	}
	firstServer := servers[0]
	if firstServer == nil {
		return fmt.Errorf("no servers defined for operation %s", asa.method.GetName())
	}
	for k, v := range firstServer.Variables {
		serverVars[k] = v.Default
	}
	requestBodySchema, requestBodySchemaErr := asa.method.GetRequestBodySchema()
	requestBodyMediaType := asa.method.GetRequestBodyMediaType()
	requestBodyParams := map[string]anysdk.Addressable{}
	var err error
	if requestBodySchema != nil && requestBodySchemaErr == nil {
		requestBodyParams, err = asa.method.GetRequestBodyAttributesNoRename()
		if err != nil {
			return err
		}
	}
	simpleSelectKey := asa.method.GetSelectItemsKeySimple()
	simpleSelectSchema, schemaErr := asa.method.GetSchemaAtPath(simpleSelectKey)
	if schemaErr != nil {
		inferredSelectKey := asa.method.LookupSelectItemsKey()
		simpleSelectSchema, schemaErr = asa.method.GetSchemaAtPath(inferredSelectKey)
		if schemaErr != nil {
			return fmt.Errorf("error getting schema at path %s: %v", inferredSelectKey, schemaErr)
		}
		simpleSelectKey = inferredSelectKey
	}
	if simpleSelectSchema == nil && !asa.method.IsNullary() {
		return fmt.Errorf("no schema found at path %s", simpleSelectKey)
	}
	unionSelectSchemas := make(map[string]anysdk.Schema)
	for alias, path := range asa.aliasedUnionSelectKeys {
		k, isResponseBodyAttribute := asa.grammar.ExtractSubPath(path, pathTypeResponseBody)
		if isResponseBodyAttribute {
			schema, schemaErr := asa.method.GetSchemaAtPath(k)
			if schemaErr != nil {
				return fmt.Errorf("error getting schema at path %s: %v", k, schemaErr)
			}
			if schema == nil {
				return fmt.Errorf("no schema found at path %s", k)
			}
			unionSelectSchemas[path] = schema
			continue
		}
		reqKey, isRequestBodyAttribute := asa.grammar.ExtractSubPath(path, pathTypeRequestBody)
		if isRequestBodyAttribute {
			schema, schemaErr := asa.method.GetRequestBodySchema()
			if schemaErr != nil || schema == nil {
				return fmt.Errorf("error getting request body schema for path %s: %v", k, schemaErr)
			}
			subSchema, schemaErr := schema.GetSchemaAtPath(reqKey, asa.method.GetRequestBodyMediaType())
			if schemaErr != nil {
				return fmt.Errorf("error getting schema at path %s: %v", reqKey, schemaErr)
			}
			unionSelectSchemas[path] = subSchema
			continue
		}

		return fmt.Errorf("only response body attributes are supported in union select keys, got '%s' for alias '%s'", path, alias)
	}
	responseSchema, responseMediaType, _ := asa.method.GetResponseBodySchemaAndMediaType()
	addressSpace := &standardNamespace{
		server:                firstServer,
		serverVars:            serverVars,
		requestBodyParams:     requestBodyParams,
		simpleSelectKey:       simpleSelectKey,
		simpleSelectSchema:    simpleSelectSchema,
		unionSelectSchemas:    unionSelectSchemas,
		responseBodySchema:    responseSchema,
		requestBodySchema:     requestBodySchema,
		responseBodyMediaType: responseMediaType,
		requestBodyMediaType:  requestBodyMediaType,
		method:                asa.method,
		shadowQuery:           NewRadixTree(),
	}
	if addressSpace == nil {
		return fmt.Errorf("failed to create address space for operation %s", asa.method.GetName())
	}
	asa.addressSpace = addressSpace
	return nil
}

func NewAddressSpaceAnalyzer(
	grammar AddressSpaceGrammar,
	provider anysdk.Provider,
	service anysdk.Service,
	resource anysdk.Resource,
	method anysdk.StandardOperationStore,
	aliasedUnionSelectKeys map[string]string,
) AddressSpaceAnalyzer {
	return &standardAddressSpaceAnalyzer{
		grammar:                grammar,
		provider:               provider,
		service:                service,
		resource:               resource,
		method:                 method,
		aliasedUnionSelectKeys: aliasedUnionSelectKeys,
	}
}

type RadixTree interface {
	Insert(path string, address any) error
	Find(path string) (any, bool)
	Delete(path string) error
	ToFlatMap(prefix string) map[string]any
	Copy() RadixTree
}

type standardRadixTree struct {
	root *standardRadixTrieNode
}

func NewRadixTree() RadixTree {
	return &standardRadixTree{
		root: newStandardRadixTrieNode(nil),
	}
}

func (rt *standardRadixTree) Copy() RadixTree {
	newTree := NewRadixTree()
	var traverse func(node *standardRadixTrieNode, currentPath string)
	traverse = func(node *standardRadixTrieNode, currentPath string) {
		if node.address != nil {
			newTree.Insert(currentPath, node.address)
		}
		for k, child := range node.children {
			newPath := k
			if currentPath != "" {
				newPath = currentPath + "." + k
			}
			traverse(child, newPath)
		}
	}
	traverse(rt.root, "")
	return newTree
}

func (rt *standardRadixTree) ToFlatMap(prefix string) map[string]any {
	result := make(map[string]any)
	var traverse func(node *standardRadixTrieNode, currentPath string)
	traverse = func(node *standardRadixTrieNode, currentPath string) {
		if node.address != nil {
			result[currentPath] = node.address
		}
		for k, child := range node.children {
			newPath := k
			if currentPath != "" {
				newPath = currentPath + "." + k
			}
			traverse(child, newPath)
		}
	}
	traverse(rt.root, prefix)
	return result
}

func (rt *standardRadixTree) Insert(path string, address any) error {
	currentNode := rt.root
	if path == "" {
		currentNode.address = address
		return nil
	}
	for {
		for k := range currentNode.children {
			if strings.HasPrefix(path, k) {
				currentNode = currentNode.children[k]
				path = strings.TrimPrefix(path, k)
				if path == "" {
					currentNode.address = address
					return nil
				}
				continue
			}
			if strings.HasPrefix(k, path) {
				// need to split node
				existingChild := currentNode.children[k]
				newChild := newStandardRadixTrieNode(address)
				currentNode.children[path] = newChild
				delete(currentNode.children, k)
				newChild.children[strings.TrimPrefix(k, path)] = existingChild
				return nil
			}
		}
		break
	}
	newChild := newStandardRadixTrieNode(address)
	currentNode.children[path] = newChild
	return nil
}

func (rt *standardRadixTree) Find(path string) (any, bool) {
	currentNode := rt.root
	if path == "" {
		return currentNode.address, currentNode.address != nil
	}
	for {
		foundPrefix := false
		for k := range currentNode.children {
			if strings.HasPrefix(path, k) {
				currentNode = currentNode.children[k]
				path = strings.TrimPrefix(path, k)
				foundPrefix = true
				if path == "" {
					return currentNode.address, currentNode.address != nil
				}
				break
			}
		}
		if !foundPrefix {
			return nil, false
		}
	}
}

func (rt *standardRadixTree) Delete(path string) error {
	currentNode := rt.root
	if path == "" {
		currentNode.address = nil
		return nil
	}
	var parentNode *standardRadixTrieNode
	var parentKey string
	for {
		foundPrefix := false
		for k := range currentNode.children {
			if strings.HasPrefix(path, k) {
				parentNode = currentNode
				parentKey = k
				currentNode = currentNode.children[k]
				path = strings.TrimPrefix(path, k)
				foundPrefix = true
				if path == "" {
					currentNode.address = nil
					if len(currentNode.children) == 0 && parentNode != nil {
						delete(parentNode.children, parentKey)
					}
					return nil
				}
				break
			}
		}
		if !foundPrefix {
			return nil
		}
	}
}

type standardRadixTrieNode struct {
	children map[string]*standardRadixTrieNode
	address  any
}

func newStandardRadixTrieNode(rhs any) *standardRadixTrieNode {
	return &standardRadixTrieNode{
		children: make(map[string]*standardRadixTrieNode),
		address:  rhs,
	}
}

type AliasMap interface {
	Put(string, string)
	Peek(string) (string, bool)
	Pop(string) (string, bool)
	Copy() AliasMap
}

type standardAliasMap struct {
	aliasToPrefixMap map[string]string
	prefixToAliasMap map[string]string
}

func NewAliasMap(aliasToPrefixMap map[string]string) AliasMap {
	return newAliasMap(aliasToPrefixMap)
}

func newAliasMap(aliasToPrefixMap map[string]string) AliasMap {
	prefixToAliasMap := make(map[string]string, len(aliasToPrefixMap))
	for k, v := range aliasToPrefixMap {
		prefixToAliasMap[v] = k
	}
	return &standardAliasMap{
		aliasToPrefixMap: aliasToPrefixMap,
		prefixToAliasMap: prefixToAliasMap,
	}
}

func (am *standardAliasMap) Copy() AliasMap {
	copyMap := make(map[string]string, len(am.aliasToPrefixMap))
	for k, v := range am.aliasToPrefixMap {
		copyMap[k] = v
	}
	newMap := newAliasMap(copyMap)
	return newMap
}

func (am *standardAliasMap) Put(key string, val string) {
	am.aliasToPrefixMap[key] = val
	am.prefixToAliasMap[val] = key
}

func (am *standardAliasMap) Peek(key string) (string, bool) {
	val, ok := am.aliasToPrefixMap[key]
	return val, ok
}

func (am *standardAliasMap) Pop(key string) (string, bool) {
	val, ok := am.aliasToPrefixMap[key]
	if ok {
		delete(am.aliasToPrefixMap, key)
		delete(am.prefixToAliasMap, val)
	}
	return val, ok
}

func (am *standardAliasMap) PeekByPrefix(prefix string) (string, bool) {
	val, ok := am.prefixToAliasMap[prefix]
	return val, ok
}

type AnalyzedInput interface {
	GetQueryParams() map[string]any
	GetHeaderParam(string) (string, bool)
	GetPathParam(string) (string, bool)
	GetServerVars() map[string]any
	GetRequestBody() any
}

type PartiallyAssignedInput interface {
	GetQueryParams() map[string]any
	GetHeaderParam(string) (string, bool)
	GetPathParam(string) (string, bool)
	GetServerVars() map[string]any
	GetRequestBody() any
	GetUnassignedParams() map[string]any
	GetUnassignedParam(string) (any, bool)
}

func isOpenapi3ParamRequired(param *openapi3.Parameter) bool {
	return param.Required && !param.AllowEmptyValue
}

func marshalBody(op anysdk.StandardOperationStore, body interface{}, expectedRequest anysdk.ExpectedRequest) ([]byte, error) {
	mediaType := expectedRequest.GetBodyMediaType()
	if expectedRequest.GetSchema() != nil {
		mediaType = expectedRequest.GetSchema().ExtractMediaTypeSynonym(mediaType)
	}
	switch mediaType {
	case media.MediaTypeJson:
		return json.Marshal(body)
	case media.MediaTypeXML, media.MediaTypeTextXML:
		return xmlmap.MarshalXMLUserInput(
			body,
			expectedRequest.GetSchema().GetXMLALiasOrName(),
			op.GetXMLTransform(),
			op.GetXMLDeclaration(),
			op.GetXMLRootAnnotation(),
		)
	}
	return nil, fmt.Errorf("media type = '%s' not supported", expectedRequest.GetBodyMediaType())
}

func replaceSimpleStringVars(template string, vars map[string]string) string {
	args := make([]string, len(vars)*2)
	i := 0
	for k, v := range vars {
		if strings.Contains(template, "{"+k+"}") {
			args[i] = "{" + k + "}"
			args[i+1] = v
			i += 2
		}
	}
	return strings.NewReplacer(args...).Replace(template)
}

func (asa *standardAddressSpaceAnalyzer) parameterizeFromPartiallyAssignedInput(prov anysdk.Provider, parentDoc anysdk.Service, op anysdk.OperationStore, inputParams PartiallyAssignedInput) (*openapi3filter.RequestValidationInput, error) {

	// aliasMap := newAliasMap()

	params := op.GetOperationRef().Value.Parameters
	pathParams := make(map[string]string)
	q := make(url.Values)
	prefilledHeader := make(http.Header)

	explicitQueryParams := inputParams.GetQueryParams()
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

			pVal, present := explicitQueryParams[p.Value.Name]
			if present {
				switch val := pVal.(type) {
				case []interface{}:
					for _, v := range val {
						q.Add(name, fmt.Sprintf("%v", v))
					}
				default:
					q.Set(name, fmt.Sprintf("%v", val))
				}
				delete(explicitQueryParams, name)
			}
		}
	}
	for k, v := range explicitQueryParams {
		q.Set(k, fmt.Sprintf("%v", v))
		delete(explicitQueryParams, k)
	}
	openapiSvc := op.GetService()
	router, err := queryrouter.NewRouter(openapiSvc.GetT())
	if err != nil {
		return nil, err
	}
	servers, _ := op.GetServers()
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

	requestBody := inputParams.GetRequestBody()

	expectedRequest, hasExpectedRequest := op.GetRequest()

	predOne := !util.IsNil(requestBody)
	predTwo := hasExpectedRequest && !util.IsNil(expectedRequest)
	if predOne && predTwo {
		stdOp, isStdOp := op.(anysdk.StandardOperationStore)
		if !isStdOp {
			return nil, fmt.Errorf("expected standard operation store")
		}
		b, err := marshalBody(stdOp, requestBody, expectedRequest)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
		contentTypeHeaderRequired = true
	}
	// TODO: clean up
	sv = strings.TrimSuffix(sv, "/")
	path := replaceSimpleStringVars(fmt.Sprintf("%s%s", sv, op.GetOperationRef().ExtractPathItem()), pathParams)
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
	httpReq, err := http.NewRequest(strings.ToUpper(op.GetOperationRef().ExtractMethodItem()), u.String(), bodyReader)
	if err != nil {
		return nil, err
	}
	if contentTypeHeaderRequired {
		if prefilledHeader.Get("Content-Type") != "" {
			prefilledHeader.Set("Content-Type", expectedRequest.GetBodyMediaType())
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

func parameterizeFromAnalyzedInput(prov anysdk.Provider, parentDoc anysdk.Service, op anysdk.OperationStore, inputParams AnalyzedInput) (*openapi3filter.RequestValidationInput, error) {

	params := op.GetOperationRef().Value.Parameters
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
	openapiSvc := op.GetService()
	router, err := queryrouter.NewRouter(openapiSvc.GetT())
	if err != nil {
		return nil, err
	}
	servers, _ := op.GetServers()
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

	requestBody := inputParams.GetRequestBody()

	expectedRequest, hasExpectedRequest := op.GetRequest()

	predOne := !util.IsNil(requestBody)
	predTwo := hasExpectedRequest && !util.IsNil(expectedRequest)
	if predOne && predTwo {
		stdOp, isStdOp := op.(anysdk.StandardOperationStore)
		if !isStdOp {
			return nil, fmt.Errorf("expected standard operation store")
		}
		b, err := marshalBody(stdOp, requestBody, expectedRequest)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
		contentTypeHeaderRequired = true
	}
	// TODO: clean up
	sv = strings.TrimSuffix(sv, "/")
	path := replaceSimpleStringVars(fmt.Sprintf("%s%s", sv, op.GetOperationRef().ExtractPathItem()), pathParams)
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
	httpReq, err := http.NewRequest(strings.ToUpper(op.GetOperationRef().ExtractMethodItem()), u.String(), bodyReader)
	if err != nil {
		return nil, err
	}
	if contentTypeHeaderRequired {
		if prefilledHeader.Get("Content-Type") != "" {
			prefilledHeader.Set("Content-Type", expectedRequest.GetBodyMediaType())
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
