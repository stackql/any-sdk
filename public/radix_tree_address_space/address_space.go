package radix_tree_address_space

import (
	"strings"

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

//	func (asg *standardAddressSpaceGrammar) Dereference(address string) (any, bool) {
//		return nil, false
//	}
func NewAddressSpaceGrammar() AddressSpaceGrammar {
	return newStandardAddressSpaceGrammar()
}

type AddressSpaceAnalyzer interface {
	Analyze() error
}

type standardAddressSpaceAnalyzer struct {
	grammar AddressSpaceGrammar
	method  anysdk.StandardOperationStore
}

func (asa *standardAddressSpaceAnalyzer) Analyze() error {
	return nil
}

func NewAddressSpaceAnalyzer(grammar AddressSpaceGrammar, method anysdk.StandardOperationStore) AddressSpaceAnalyzer {
	return &standardAddressSpaceAnalyzer{
		grammar: grammar,
		method:  method,
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
