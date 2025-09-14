package radix_tree_address_space

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/client"
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
	standardCookiesName         = "cookies"
	standardServerName          = "server"
	standardPostTransformPrefix = "post_transform_"
	// path types
	pathTypeRequestBody    pathType = "request_body"
	pathTypeResponseBody   pathType = "response_body"
	pathTypeURLQuery       pathType = "url_query"
	pathTypeURLPath        pathType = "url_path"
	pathTypeRequestHeader  pathType = "request_header"
	pathTypeResponseHeader pathType = "response_header"
)

type standardAddressSpaceExpansionConfig struct {
	isLegacy           bool
	isAllowNilResponse bool
}

func NewStandardAddressSpaceExpansionConfig(
	isLegacy bool,
	isAllowNilResponse bool,
) anysdk.AddressSpaceExpansionConfig {
	return &standardAddressSpaceExpansionConfig{
		isLegacy:           isLegacy,
		isAllowNilResponse: isAllowNilResponse,
	}
}

func (aec *standardAddressSpaceExpansionConfig) IsLegacy() bool {
	return aec.isLegacy
}

func (aec *standardAddressSpaceExpansionConfig) IsAllowNilResponse() bool {
	return aec.isAllowNilResponse
}

// AddressSpaceGrammar defines the search DSL
type AddressSpaceGrammar interface {
	ExtractSubPath(string, pathType) (string, bool)
}

type standardAddressSpaceGrammar struct {
	requestName          string
	responseName         string
	urlName              string
	headersName          string
	bodyName             string
	queryName            string
	pathName             string
	cookiesName          string
	serverName           string
	postTransformPrefix  string
	requestBodyPrefix    string
	responseBodyPrefix   string
	urlQueryPrefix       string
	requestHeaderPrefix  string
	responseHeaderPrefix string
	urlPathPrefix        string
}

func newStandardAddressSpaceGrammar() AddressSpaceGrammar {
	return &standardAddressSpaceGrammar{
		requestName:          standardRequestName,
		responseName:         standardResponseName,
		urlName:              standardURLName,
		headersName:          standardHeadersName,
		bodyName:             standardBodyName,
		queryName:            standardQueryName,
		pathName:             standardPathName,
		cookiesName:          standardCookiesName,
		postTransformPrefix:  standardPostTransformPrefix,
		serverName:           standardServerName,
		requestBodyPrefix:    fmt.Sprintf("%s.%s.", standardRequestName, standardBodyName),
		responseBodyPrefix:   fmt.Sprintf("%s.%s.", standardResponseName, standardBodyName),
		urlQueryPrefix:       fmt.Sprintf("%s.%s.", standardRequestName, standardQueryName),
		requestHeaderPrefix:  fmt.Sprintf("%s.%s.", standardRequestName, standardHeadersName),
		responseHeaderPrefix: fmt.Sprintf("%s.%s.", standardResponseName, standardHeadersName),
		urlPathPrefix:        fmt.Sprintf("%s.%s.", standardRequestName, standardPathName),
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

func (sg *standardAddressSpaceGrammar) extractQueryParamSubPath(fullPath string) (string, bool) {
	rv := strings.TrimPrefix(fullPath, sg.urlQueryPrefix)
	return rv, rv != fullPath
}

func (sg *standardAddressSpaceGrammar) extractRequestHeaderSubPath(fullPath string) (string, bool) {
	rv := strings.TrimPrefix(fullPath, sg.requestHeaderPrefix)
	return rv, rv != fullPath
}

func (sg *standardAddressSpaceGrammar) extractPathParamSubPath(fullPath string) (string, bool) {
	rv := strings.TrimPrefix(fullPath, sg.urlPathPrefix)
	return rv, rv != fullPath
}

func (sg *standardAddressSpaceGrammar) ExtractSubPath(fullPath string, pathType pathType) (string, bool) {
	switch pathType {
	case pathTypeRequestBody:
		return sg.extractRequestBodySubPath(fullPath)
	case pathTypeResponseBody:
		return sg.extractResponseBodySubPath(fullPath)
	case pathTypeURLQuery:
		return sg.extractQueryParamSubPath(fullPath)
	case pathTypeRequestHeader:
		return sg.extractRequestHeaderSubPath(fullPath)
	case pathTypeURLPath:
		return sg.extractPathParamSubPath(fullPath)
	default:
		return "", false
	}
}

func NewAddressSpaceGrammar() AddressSpaceGrammar {
	return newStandardAddressSpaceGrammar()
}

type AddressSpaceAnalysisPassManager interface {
	ApplyPasses() error
	GetAddressSpace() (anysdk.AddressSpace, bool)
}

func NewAddressSpaceAnalysisPassManager(formulator AddressSpaceFormulator) AddressSpaceAnalysisPassManager {
	return &standardAddressSpaceAnalysisPassManager{
		formulator: formulator,
	}
}

type standardAddressSpaceAnalysisPassManager struct {
	formulator   AddressSpaceFormulator
	addressSpace anysdk.AddressSpace
}

func (pm *standardAddressSpaceAnalysisPassManager) ApplyPasses() error {
	rv := pm.formulator.Formulate()
	if rv != nil {
		return rv
	}
	as := pm.formulator.GetAddressSpace()
	pm.addressSpace = as
	return nil
}

func (pm *standardAddressSpaceAnalysisPassManager) GetAddressSpace() (anysdk.AddressSpace, bool) {
	return pm.addressSpace, pm.addressSpace != nil
}

type AddressSpaceFormulator interface {
	Formulate() error
	GetAddressSpace() anysdk.AddressSpace
}

type standardNamespace struct {
	serverVars            map[string]string
	requestBodyParams     map[string]anysdk.Addressable
	server                *openapi3.Server
	prov                  anysdk.Provider
	svc                   anysdk.Service
	method                anysdk.StandardOperationStore
	simpleSelectKey       string
	simpleSelectSchema    anysdk.Schema
	responseBodySchema    anysdk.Schema
	requestBodySchema     anysdk.Schema
	responseBodyMediaType string
	requestBodyMediaType  string
	serverUrlString       string
	request               *http.Request
	response              *http.Response
	unionSelectSchemas    map[string]anysdk.Schema
	globalSelectSchemas   map[string]anysdk.Schema
	explicitAliasMap      AliasMap
	globalAliasMap        AliasMap
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

func (ns *standardNamespace) ResolveSignature(params map[string]any) (bool, map[string]any) {
	copyParams := make(map[string]any, len(params))
	for k, v := range params {
		copyParams[k] = v
	}
	requiredNonBodyParams := ns.method.GetRequiredNonBodyParameters()
	requiredBodyPrarms := make(map[string]anysdk.Addressable)
	for k, v := range ns.requestBodyParams {
		if v.IsRequired() {
			requiredBodyPrarms[k] = v
		}
	}
	for k, _ := range params {
		_, hasKey := ns.globalAliasMap.Peek(k)
		if !hasKey {
			return false, copyParams
		}
		delete(copyParams, k)
		_, isRequiredNonBodyParam := requiredNonBodyParams[k]
		if isRequiredNonBodyParam {
			delete(requiredNonBodyParams, k)
			continue
		}
		_, isRequiredBodyParam := requiredBodyPrarms[k]
		if isRequiredBodyParam {
			delete(requiredBodyPrarms, k)
			continue
		}
	}
	return len(requiredNonBodyParams) == 0 && len(requiredBodyPrarms) == 0, copyParams
}

func (ns *standardNamespace) copyResponse(resp *http.Response) (*http.Response, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil response")
	}
	var bodyBytes []byte
	var responseBodyCopy io.ReadCloser
	clonedResponse := new(http.Response)
	*clonedResponse = *resp
	if resp.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		responseBodyBytesCopy := make([]byte, len(bodyBytes))
		copy(responseBodyBytesCopy, bodyBytes)
		responseBodyCopy = io.NopCloser(bytes.NewBuffer(responseBodyBytesCopy))
	}
	clonedResponse.Body = responseBodyCopy
	copiedHeaders := make(http.Header)
	for k, v := range resp.Header {
		copiedHeaders[k] = v
	}
	clonedResponse.Header = copiedHeaders
	return clonedResponse, nil
}

func (ns *standardNamespace) copyRequest(req *http.Request) (*http.Request, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	var bodyBytes []byte
	var requestBodyCopy io.ReadCloser
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		requestBodyBytesCopy := make([]byte, len(bodyBytes))
		copy(requestBodyBytesCopy, bodyBytes)
		requestBodyCopy = io.NopCloser(bytes.NewBuffer(requestBodyBytesCopy))
	}
	newReq := req.Clone(req.Context())
	if bodyBytes != nil {
		newReq.Body = requestBodyCopy
	}
	return newReq, nil
}

func (ns *standardNamespace) Invoke(argList ...any) error {
	if len(argList) < 2 {
		return fmt.Errorf("insufficient arguments to invoke")
	}
	container := argList[0]
	switch v := container.(type) {
	case *http.Client:
		req := argList[1]
		httpReq, ok := req.(*http.Request)
		if !ok {
			return fmt.Errorf("expected *http.Request, got %T", req)
		}
		copiedRequest, err := ns.copyRequest(httpReq)
		if err != nil {
			return err
		}
		for k, v := range copiedRequest.Header {
			ns.WriteToAddress(fmt.Sprintf("request.headers.%s", k), v)
		}
		// handle body
		reuestBodyMapVerbose := ns.shadowQuery.ToFlatMap("request.body")
		reuestBodyMap := make(map[string]any)
		requestContentType := ns.method.GetRequestBodyMediaTypeNormalised()
		for k, v := range reuestBodyMapVerbose {
			if requestContentType == media.MediaTypeJson || requestContentType == "" {
				trimmedKey := strings.TrimPrefix(k, "$.")
				reuestBodyMap[trimmedKey] = v
			} else if requestContentType == media.MediaTypeXML {
				trimmedKey := strings.TrimPrefix(k, "/")
				reuestBodyMap[trimmedKey] = v
			} else {
				return fmt.Errorf("unsupported request content type: %s", requestContentType)
			}
		}
		if len(reuestBodyMap) > 0 {
			expectedRequest, hasExpectedRequest := ns.method.GetRequest()
			if !hasExpectedRequest {
				return fmt.Errorf("no expected request found for method %s", ns.method.GetName())
			}
			bodyBytes, marshalErr := ns.method.MarshalBody(reuestBodyMap, expectedRequest)
			if marshalErr != nil {
				return marshalErr
			}
			httpReq.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		ns.request = copiedRequest

		resp, respErr := v.Do(httpReq)
		if respErr != nil {
			return respErr
		}
		copiedResponse, copyResponseErr := ns.copyResponse(resp)
		if copyResponseErr != nil {
			return copyResponseErr
		}
		ns.response = copiedResponse
		if resp == nil {
			return fmt.Errorf("nil response from http client")
		}
	default:
		return fmt.Errorf("expected *http.Client, got %T", v)
	}
	return nil
}

func (ns *standardNamespace) getLegacyColumns(cfg anysdk.AddressSpaceExpansionConfig, requestSchema anysdk.Schema, m anysdk.StandardOperationStore) ([]anysdk.Column, error) {
	schemaAnalyzer := newLegacyTableSchemaAnalyzer(requestSchema, m, cfg.IsAllowNilResponse())
	return schemaAnalyzer.GetColumns()
}

type standardRelation struct {
	columns []anysdk.Column
}

func (sr *standardRelation) GetColumns() []anysdk.Column {
	return sr.columns
}

func (ns *standardNamespace) getLegacyRelation(cfg anysdk.AddressSpaceExpansionConfig, requestSchema anysdk.Schema, m anysdk.StandardOperationStore) (anysdk.Relation, error) {
	cols, err := ns.getLegacyColumns(cfg, requestSchema, m)
	if err != nil {
		return nil, err
	}
	return &standardRelation{
		columns: cols,
	}, nil
}

func (ns *standardNamespace) globalAliasesToRelation() (anysdk.Relation, error) {
	columns := make([]anysdk.Column, 0, len(ns.globalSelectSchemas))
	aliases := make([]string, 0, len(ns.globalSelectSchemas))
	i := 0
	for alias := range ns.globalSelectSchemas {
		aliases[i] = alias
		i++
	}
	sort.Strings(aliases)
	for i, alias := range aliases {
		schema := ns.globalSelectSchemas[alias]
		col := newSimpleColumn(alias, schema)
		columns[i] = col
	}
	return &standardRelation{
		columns: columns,
	}, nil
}

func (ns *standardNamespace) ToRelation(cfg anysdk.AddressSpaceExpansionConfig) (anysdk.Relation, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	if cfg.IsLegacy() {
		return ns.getLegacyRelation(cfg, ns.responseBodySchema, ns.method)
	}
	return ns.globalAliasesToRelation()
}

func (ns *standardNamespace) ToMap(cfg anysdk.AddressSpaceExpansionConfig) (map[string]any, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	rv := make(map[string]any)
	aliasToPrefixMap := ns.globalAliasMap.GetAliasToPrefixMap()
	for alias, path := range aliasToPrefixMap {
		val, ok := ns.shadowQuery.Find(path)
		if !ok {
			// return nil, fmt.Errorf("failed to dereference path '%s' for alias '%s'", path, alias)
		}
		rv[alias] = val
	}
	return rv, nil
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

func (ns *standardNamespace) GetGlobalSelectSchemas() map[string]anysdk.Schema {
	return ns.globalSelectSchemas
}

type standardAddressSpaceFormulator struct {
	grammar                AddressSpaceGrammar
	provider               anysdk.Provider
	service                anysdk.Service
	resource               anysdk.Resource
	method                 anysdk.StandardOperationStore
	requiredNonBodyParams  map[string]anysdk.Addressable
	requiredBodyAttributes map[string]anysdk.Addressable
	aliasedUnionSelectKeys map[string]string
	addressSpace           anysdk.AddressSpace
}

func (asa *standardAddressSpaceFormulator) GetAddressSpace() anysdk.AddressSpace {
	return asa.addressSpace
}

func (asa *standardAddressSpaceFormulator) expandParameterPaths() (map[string]string, error) {
	parameters := asa.method.GetNonBodyParameters()
	rv := make(map[string]string)
	for k, v := range parameters {
		location := v.GetLocation()
		switch location {
		case anysdk.LocationPath:
			rv[k] = fmt.Sprintf("%s.%s.%s", standardRequestName, standardPathName, k)
		case anysdk.LocationQuery:
			rv[k] = fmt.Sprintf("%s.%s.%s", standardRequestName, standardQueryName, k)
		case anysdk.LocationHeader:
			rv[k] = fmt.Sprintf("%s.%s.%s", standardRequestName, standardHeadersName, k)
		case anysdk.LocationCookie:
			rv[k] = fmt.Sprintf("%s.%s.%s", standardRequestName, standardCookiesName, k)
		case anysdk.LocationServer:
			rv[k] = fmt.Sprintf("%s.%s", "server", k)
		// case anysdk.LocationRequestBody:
		// 	rv[k] = fmt.Sprintf("%s.%s.%s", standardRequestName, standardBodyName, k)
		default:
			return nil, fmt.Errorf("unsupported parameter location: %s", location)
		}
	}
	return rv, nil
}

func (asa *standardAddressSpaceFormulator) resolvePathsToSchemas(aliasToPathMap map[string]string) (map[string]anysdk.Schema, error) {
	rv := make(map[string]anysdk.Schema)
	for alias, path := range aliasToPathMap {
		k, isResponseBodyAttribute := asa.grammar.ExtractSubPath(path, pathTypeResponseBody)
		if isResponseBodyAttribute {
			schema, schemaErr := asa.method.GetSchemaAtPath(k)
			if schemaErr != nil {
				return nil, fmt.Errorf("error getting schema at path %s: %v", k, schemaErr)
			}
			if schema == nil {
				return nil, fmt.Errorf("no schema found at path %s", k)
			}
			rv[path] = schema
			continue
		}
		reqKey, isRequestBodyAttribute := asa.grammar.ExtractSubPath(path, pathTypeRequestBody)
		if isRequestBodyAttribute {
			schema, schemaErr := asa.method.GetRequestBodySchema()
			if schemaErr != nil || schema == nil {
				return nil, fmt.Errorf("error getting request body schema for path %s: %v", k, schemaErr)
			}
			subSchema, schemaErr := schema.GetSchemaAtPath(reqKey, asa.method.GetRequestBodyMediaType())
			if schemaErr != nil {
				return nil, fmt.Errorf("error getting schema at path %s: %v", reqKey, schemaErr)
			}
			rv[path] = subSchema
			continue
		}
		queryKey, isQueryAttribute := asa.grammar.ExtractSubPath(path, pathTypeURLQuery)
		if isQueryAttribute {
			schema := anysdk.NewStringSchema(
				nil,
				queryKey,
				queryKey,
			)
			rv[path] = schema
			continue
		}
		requestHeaderKey, isRequestHeaderAttribute := asa.grammar.ExtractSubPath(path, pathTypeRequestHeader)
		if isRequestHeaderAttribute {
			schema := anysdk.NewStringSchema(
				nil,
				requestHeaderKey,
				requestHeaderKey,
			)
			rv[path] = schema
			continue
		}
		urlPathKey, isURLPathAttribute := asa.grammar.ExtractSubPath(path, pathTypeURLPath)
		if isURLPathAttribute {
			schema := anysdk.NewStringSchema(
				nil,
				urlPathKey,
				urlPathKey,
			)
			rv[path] = schema
			continue
		}
		return nil, fmt.Errorf("only response body attributes are supported in union select keys, got '%s' for alias '%s'", path, alias)
	}
	return rv, nil
}

func (asa *standardAddressSpaceFormulator) Formulate() error {
	serverVars := make(map[string]string)
	servers, _ := asa.method.GetServers()
	svcServers, _ := asa.service.GetServers()
	servers = append(servers, svcServers...)
	var selectedServer *openapi3.Server
	protocolType, _ := asa.provider.GetProtocolType()
	if len(servers) == 0 && protocolType != client.LocalTemplated {
		return fmt.Errorf("no servers defined for operation %s", asa.method.GetName())
	}
	if len(servers) > 0 {
		selectedServer = servers[0]
		if selectedServer == nil {
			return fmt.Errorf("no servers defined for operation %s", asa.method.GetName())
		}
		for k, v := range selectedServer.Variables {
			serverVars[k] = v.Default
		}
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
	if schemaErr != nil && !asa.method.IsNullary() {
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
	isResponseBodyConsidered := false
	isParametersConsidered := false
	isServerVarsConsidered := false
	isRequestBodyConsidered := false
	unionSelectSchemas, unionSelectErr := asa.resolvePathsToSchemas(asa.aliasedUnionSelectKeys)
	if unionSelectErr != nil {
		return unionSelectErr
	}
	parameterPaths, paramterDerivationErr := asa.expandParameterPaths()
	if paramterDerivationErr != nil {
		return paramterDerivationErr
	}
	globalSelectSchemas, globalSelectSchemasErr := asa.resolvePathsToSchemas(parameterPaths)
	if globalSelectSchemasErr != nil {
		return globalSelectSchemasErr
	}
	explicitAliasMap := newAliasMap(asa.aliasedUnionSelectKeys)
	globalAliasMap := explicitAliasMap.Copy()
	var parametersDoublySelcted []string
	for k, v := range parameterPaths {
		_, isInExplicitSelect := asa.aliasedUnionSelectKeys[k]
		if isInExplicitSelect {
			parametersDoublySelcted = append(parametersDoublySelcted, k)
		}
		globalAliasMap.Put(k, v)
	}
	if len(parametersDoublySelcted) > 0 {
		return fmt.Errorf("the following parameters were selected both explicitly and implicitly: %v", parametersDoublySelcted)
	}
	// // placeholder
	if !isResponseBodyConsidered && !isRequestBodyConsidered && !isParametersConsidered && !isServerVarsConsidered {
	}
	// for k, v := range unionSelectSchemas {
	// 	globalSelectSchemas[k] = v
	// }
	responseSchema, responseMediaType, _ := asa.method.GetResponseBodySchemaAndMediaType()
	addressSpace := &standardNamespace{
		server:                selectedServer,
		serverVars:            serverVars,
		requestBodyParams:     requestBodyParams,
		simpleSelectKey:       simpleSelectKey,
		simpleSelectSchema:    simpleSelectSchema,
		unionSelectSchemas:    unionSelectSchemas,
		globalSelectSchemas:   globalSelectSchemas,
		responseBodySchema:    responseSchema,
		requestBodySchema:     requestBodySchema,
		responseBodyMediaType: responseMediaType,
		requestBodyMediaType:  requestBodyMediaType,
		explicitAliasMap:      explicitAliasMap,
		globalAliasMap:        globalAliasMap,
		prov:                  asa.provider,
		svc:                   asa.service,
		method:                asa.method,
		shadowQuery:           NewRadixTree(),
	}
	if addressSpace == nil {
		return fmt.Errorf("failed to create address space for operation %s", asa.method.GetName())
	}
	asa.addressSpace = addressSpace
	return nil
}

func NewAddressSpaceFormulator(
	grammar AddressSpaceGrammar,
	provider anysdk.Provider,
	service anysdk.Service,
	resource anysdk.Resource,
	method anysdk.StandardOperationStore,
	aliasedUnionSelectKeys map[string]string,
) AddressSpaceFormulator {
	return &standardAddressSpaceFormulator{
		grammar:                grammar,
		provider:               provider,
		service:                service,
		resource:               resource,
		method:                 method,
		aliasedUnionSelectKeys: aliasedUnionSelectKeys,
		requiredNonBodyParams:  make(map[string]anysdk.Addressable),
		requiredBodyAttributes: make(map[string]anysdk.Addressable),
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
	// find all nodes with keysd containing the prefix and return as a flat map, omitting the prefix
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
	traverse(rt.root, "")
	if prefix != "" {
		prefixedResult := make(map[string]any)
		for k, v := range result {
			if strings.HasPrefix(k, prefix+".") {
				newKey := strings.TrimPrefix(k, prefix+".")
				prefixedResult[newKey] = v
			} else if k == prefix {
				prefixedResult[""] = v
			}
		}
		return prefixedResult
	}
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

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	prefix := strs[0] // Start with the first string as the potential prefix

	for i := 1; i < len(strs); i++ {
		// While the current prefix is not a prefix of the current string, shorten it
		for len(prefix) > 0 && !hasPrefix(strs[i], prefix) {
			prefix = prefix[:len(prefix)-1] // Remove the last character
		}
		if len(prefix) == 0 { // If prefix becomes empty, no common prefix exists
			return ""
		}
	}
	return prefix
}

// Helper function to check if a string starts with a given prefix
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
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
	GetAliasToPrefixMap() map[string]string
}

type standardAliasMap struct {
	aliasToPrefixMap map[string]string
	prefixToAliasMap map[string]string
}

func NewAliasMap(aliasToPrefixMap map[string]string) AliasMap {
	return newAliasMap(aliasToPrefixMap)
}

func newAliasMap(aliasToPrefixMap map[string]string) AliasMap {
	copyMap := make(map[string]string, len(aliasToPrefixMap))
	for k, v := range aliasToPrefixMap {
		copyMap[k] = v
	}
	prefixToAliasMap := make(map[string]string, len(copyMap))
	for k, v := range copyMap {
		prefixToAliasMap[v] = k
	}
	return &standardAliasMap{
		aliasToPrefixMap: aliasToPrefixMap,
		prefixToAliasMap: prefixToAliasMap,
	}
}

func (am *standardAliasMap) GetAliasToPrefixMap() map[string]string {
	return am.aliasToPrefixMap
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

type namespaceAnalyzedInput struct {
	namespace anysdk.AddressSpace
}

func (nai *namespaceAnalyzedInput) GetQueryParams() map[string]any {
	val, ok := nai.namespace.ReadFromAddress("request.query")
	if !ok {
		return make(map[string]any)
	}
	q, ok := val.(map[string]any)
	if !ok {
		return make(map[string]any)
	}
	return q
}

func (nai *namespaceAnalyzedInput) GetHeaderParam(name string) (string, bool) {
	val, ok := nai.namespace.ReadFromAddress(fmt.Sprintf("request.headers.%s", name))
	if !ok {
		return "", false
	}
	switch v := val.(type) {
	case string:
		return v, true
	case []string:
		if len(v) > 0 {
			return v[0], true
		}
		return "", false
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func (nai *namespaceAnalyzedInput) GetPathParam(name string) (string, bool) {
	val, ok := nai.namespace.ReadFromAddress(fmt.Sprintf("request.path.%s", name))
	if !ok {
		return "", false
	}
	switch v := val.(type) {
	case string:
		return v, true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func (nai *namespaceAnalyzedInput) GetServerVars() map[string]any {
	val, ok := nai.namespace.ReadFromAddress("server")
	if !ok {
		return make(map[string]any)
	}
	sv, ok := val.(map[string]any)
	if !ok {
		return make(map[string]any)
	}
	return sv
}

func (nai *namespaceAnalyzedInput) GetRequestBody() any {
	val, ok := nai.namespace.ReadFromAddress("request.body")
	if !ok {
		return nil
	}
	return val
}

func newNamespaceAnalyzedInput(ns anysdk.AddressSpace) AnalyzedInput {
	return &namespaceAnalyzedInput{
		namespace: ns,
	}
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

type AddressSpaceExpander interface {
	Expand() error
}

func NewResourceAddressSpaceExpander(
	provider anysdk.Provider,
	service anysdk.Service,
	resource anysdk.Resource,
) AddressSpaceExpander {
	return &rscNamespaceExpander{
		provider: provider,
		service:  service,
		resource: resource,
	}
}

type rscNamespaceExpander struct {
	provider anysdk.Provider
	service  anysdk.Service
	resource anysdk.Resource
}

func (rex *rscNamespaceExpander) Expand() error {
	shallowRsc := rex.resource
	for _, sm := range shallowRsc.GetMethods() {
		method := &sm // this is poo
		addressSpaceFormulator := NewAddressSpaceFormulator(
			NewAddressSpaceGrammar(),
			rex.provider,
			rex.service,
			rex.resource,
			method,
			method.GetProjections(),
		)
		err := addressSpaceFormulator.Formulate()
		if err != nil {
			return err
		}
		addressSpace := addressSpaceFormulator.GetAddressSpace()
		method.SetAddressSpace(addressSpace)
	}
	return nil
}
