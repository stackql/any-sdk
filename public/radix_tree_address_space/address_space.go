package radix_tree_address_space

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stackql/any-sdk/anysdk"
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
}

type standardNamespace struct {
	serverVars         map[string]string
	requestBodyParams  map[string]anysdk.Addressable
	server             *openapi3.Server
	simpleSelectKey    string
	simpleSelectSchema anysdk.Schema
	unionSelectSchemas map[string]anysdk.Schema
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
	reequestBodySchema, requestBodySchemaErr := asa.method.GetRequestBodySchema()
	requestBodyParams := map[string]anysdk.Addressable{}
	var err error
	if reequestBodySchema != nil && requestBodySchemaErr == nil {
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
	}
	if simpleSelectSchema == nil {
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
	addressSpace := &standardNamespace{
		server:             firstServer,
		serverVars:         serverVars,
		requestBodyParams:  requestBodyParams,
		simpleSelectKey:    simpleSelectKey,
		simpleSelectSchema: simpleSelectSchema,
		unionSelectSchemas: unionSelectSchemas,
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

type PrefixTree interface {
	InsertBodyAttribute(path string, value interface{}) error
}

type RadixTree interface {
	Insert(path string, address any) error
	Find(path string) (any, bool)
	Delete(path string) error
}

type standardRadixTree struct {
	root *standardRadixTrieNode
}

func NewRadixTree() RadixTree {
	return &standardRadixTree{
		root: newStandardRadixTrieNode(nil),
	}
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
