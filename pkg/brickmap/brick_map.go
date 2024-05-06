package brickmap

import "fmt"

var (
	_ BrickMap = &standardBrickMap{}
)

type BrickMap interface {
	Set(keyPath []string, value interface{}) error
	Get(keyPath []string) (interface{}, bool)
	ToFlatMap() (map[string]interface{}, bool)
	Delete(keyPath []string) bool
}

func NewBrickMap() BrickMap {
	return &standardBrickMap{
		m: make(map[string]interface{}),
	}
}

type standardBrickMap struct {
	m map[string]interface{}
}

func (bm *standardBrickMap) ToFlatMap() (map[string]interface{}, bool) {
	return bm.m, len(bm.m) > 0
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
