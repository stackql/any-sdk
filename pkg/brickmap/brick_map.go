package brickmap

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
)

const (
	encodingJSON = "application/json"
	encodingXML  = "application/xml"
)

var (
	_ BrickMap = &standardBrickMap{}
)

type BrickMapConfig interface {
	GetStringifiedPaths() map[string]struct{}
	GetEncoding() string
}

type standardBrickMapConfig struct {
	stringifiedPaths map[string]struct{}
	encoding         string
}

func (oc *standardBrickMapConfig) GetStringifiedPaths() map[string]struct{} {
	return oc.stringifiedPaths
}

func (oc *standardBrickMapConfig) GetEncoding() string {
	if oc.encoding != "" {
		return oc.encoding
	}
	return encodingJSON
}

func NewStandardBrickMapConfig(stringifiedPaths map[string]struct{}, encoding string) BrickMapConfig {
	return &standardBrickMapConfig{
		stringifiedPaths: stringifiedPaths,
		encoding:         encoding,
	}
}

type BrickMap interface {
	Set(keyPath []string, value interface{}) error
	Get(keyPath []string) (interface{}, bool)
	ToFlatMap() (map[string]interface{}, bool)
	Delete(keyPath []string) bool
}

func NewBrickMap(cfg BrickMapConfig) BrickMap {
	stringOnlyPaths := cfg.GetStringifiedPaths()
	return &standardBrickMap{
		m:               make(map[string]interface{}),
		stringOnlyPaths: stringOnlyPaths,
		cfg:             cfg,
	}
}

type standardBrickMap struct {
	m               map[string]interface{}
	stringOnlyPaths map[string]struct{}
	cfg             BrickMapConfig
}

func (bm *standardBrickMap) isJSONSynonym(mediaType string) bool {
	return mediaType == encodingJSON
}

func (bm *standardBrickMap) isXMLSynonym(mediaType string) bool {
	return mediaType == encodingXML
}

func (bm *standardBrickMap) ToFlatMap() (map[string]interface{}, bool) {
	output := make(map[string]interface{})
	for k, v := range bm.m {
		_, isStringOnly := bm.stringOnlyPaths[k]
		if isStringOnly {
			switch vt := v.(type) {
			case map[string]interface{}:
				var b []byte
				var err error
				if bm.isJSONSynonym(bm.cfg.GetEncoding()) {
					b, err = json.Marshal(vt)
				} else if bm.isXMLSynonym(bm.cfg.GetEncoding()) {
					b, err = xml.Marshal(vt)
				} else {
					b, err = json.Marshal(vt)
				}
				if err == nil {
					output[k] = string(b)
				} else {
					output[k] = fmt.Sprintf("%v", v)
				}
			default:
				output[k] = fmt.Sprintf("%v", v)
			}
		} else {
			output[k] = v
		}
	}
	return output, len(output) > 0
}

func (bm *standardBrickMap) Set(keyPath []string, value interface{}) error {
	if len(keyPath) == 0 {
		return fmt.Errorf("brick map key path must have at least one element")
	}
	if len(keyPath) == 1 {
		bm.m[keyPath[0]] = value
		return nil
	}
	nodeToMutate := bm.m
	for i, key := range keyPath {
		if key == "" {
			return fmt.Errorf("brick map key path cannot have empty key at index %d", i)
		}
		if i == len(keyPath)-1 {
			nodeToMutate[key] = value
			return nil
		}
		node, nodeExists := nodeToMutate[key]
		if !nodeExists {
			node = make(map[string]interface{})
			nodeToMutate[key] = node
		}
		var nodeIsMap bool
		nodeToMutate, nodeIsMap = node.(map[string]interface{})
		if !nodeIsMap {
			return fmt.Errorf("brick map key path is not a map at index %d", i)
		}
	}
	return nil
}

func (bm *standardBrickMap) Get(keyPath []string) (interface{}, bool) {
	if len(keyPath) == 0 {
		return nil, false
	}
	// if len(keyPath) == 1 {
	// 	v, ok := bm.m[keyPath[0]]
	// 	return v, ok
	// }
	nodeToSearch := bm.m
	for i, key := range keyPath {
		if key == "" {
			return nil, false
		}
		if i == len(keyPath)-1 {
			rv, ok := nodeToSearch[key]
			return rv, ok
		}
		node, nodeExists := nodeToSearch[key]
		if !nodeExists {
			return nil, false
		}
		var nodeIsMap bool
		nodeToSearch, nodeIsMap = node.(map[string]interface{})
		if !nodeIsMap {
			return nil, false
		}
	}
	return nil, false
}

func (bm *standardBrickMap) Delete(keyPath []string) bool {
	if len(keyPath) == 0 {
		return false
	}
	nodeToSearch := bm.m
	for i, key := range keyPath {
		if key == "" {
			return false
		}
		if i == len(keyPath)-1 {
			_, ok := nodeToSearch[key]
			if ok {
				delete(nodeToSearch, key)
				return true
			}
			return false
		}
		node, nodeExists := nodeToSearch[key]
		if !nodeExists {
			return false
		}
		var nodeIsMap bool
		nodeToSearch, nodeIsMap = node.(map[string]interface{})
		if !nodeIsMap {
			return false
		}
	}
	return false
}
