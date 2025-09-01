package radix_tree_address_space

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/urltranslate"
)

const (
	standardRequestName  = "request"
	standardResponseName = "response"
	standardURLName      = "url"
	standardHeadersName  = "headers"
	standardBodyName     = "body"
	standardQueryName    = "query"
	standardPathName     = "path"
	cookiesName          = "cookies"
)

// AddressSpaceGrammar defines the search DSL
type AddressSpaceGrammar interface {
}

type standardAddressSpaceGrammar struct {
	requestName  string
	responseName string
	urlName      string
	headersName  string
	bodyName     string
	queryName    string
	pathName     string
	cookiesName  string
}

func newStandardAddressSpaceGrammar() AddressSpaceGrammar {
	return &standardAddressSpaceGrammar{
		requestName:  standardRequestName,
		responseName: standardResponseName,
		urlName:      standardURLName,
		headersName:  standardHeadersName,
		bodyName:     standardBodyName,
		queryName:    standardQueryName,
		pathName:     standardPathName,
		cookiesName:  cookiesName,
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
	grammar         AddressSpaceGrammar
	provider        anysdk.Provider
	service         anysdk.Service
	resource        anysdk.Resource
	method          anysdk.StandardOperationStore
	unionSelectKeys []string
	addressSpace    AddressSpace
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
	for _, k := range asa.unionSelectKeys {
		schema, schemaErr := asa.method.GetSchemaAtPath(k)
		if schemaErr != nil {
			return fmt.Errorf("error getting schema at path %s: %v", k, schemaErr)
		}
		if schema == nil {
			return fmt.Errorf("no schema found at path %s", k)
		}
		unionSelectSchemas[k] = schema
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
	unionSelectKeys ...string,
) AddressSpaceAnalyzer {
	return &standardAddressSpaceAnalyzer{
		grammar:         grammar,
		provider:        provider,
		service:         service,
		resource:        resource,
		method:          method,
		unionSelectKeys: unionSelectKeys,
	}
}

type RadixTree interface {
	Insert(path string, address any) error
	Find(path string) (any, bool)
	Delete(path string) error
	ToFlatMap(prefix string) map[string]any
}

type standardRadixTree struct {
	root *standardRadixTrieNode
}

func NewRadixTree() RadixTree {
	return &standardRadixTree{
		root: newStandardRadixTrieNode(nil),
	}
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
