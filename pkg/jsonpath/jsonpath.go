package jsonpath

import (
	"fmt"
	"strings"

	"github.com/PaesslerAG/gval"
	jp "github.com/PaesslerAG/jsonpath"

	"context"
)

func Get(path string, value interface{}) (interface{}, error) {
	return jp.Get(path, value)
}

func SplitSearchPath(inputMap map[string]interface{}, pathStr string) ([]string, error) {
	return splitSearchPath(inputMap, pathStr)
}

func splitSearchPath(inputMap map[string]interface{}, pathStr string) ([]string, error) {
	pathStr = strings.TrimPrefix(pathStr, "$.")
	vs := gval.VariableSelector(func(path gval.Evaluables) gval.Evaluable {
		return func(c context.Context, v interface{}) (interface{}, error) {
			keys, err := path.EvalStrings(c, v)
			if err != nil {
				return nil, err
			}
			return keys, nil
		}
	})
	lang := gval.NewLanguage(append(
		[]gval.Language{gval.Base()},
		vs,
	)...)
	rawVal, err := lang.Evaluate(
		pathStr,
		inputMap,
	)
	conformedVal, isStringSlice := rawVal.([]string)
	if !isStringSlice {
		return nil, fmt.Errorf("cannot accomodate inferred JSON path of type %T", conformedVal)
	}
	if err != nil {
		return nil, err
	}
	return conformedVal, nil
}

func Set(inputMap map[string]interface{}, pathStr string, rhs interface{}) (map[string]interface{}, error) {
	conformedVal, err := splitSearchPath(inputMap, pathStr)
	if err != nil {
		return nil, err
	}
	lhs := inputMap
	var lhsOk bool
	var k string
	for i := range conformedVal {
		k = conformedVal[i]
		if i == len(conformedVal)-1 {
			break
		}
		lhs, lhsOk = lhs[k].(map[string]interface{})
		if !lhsOk {
			return nil, fmt.Errorf("disallowed map traversal into type %T on key %s at index %d", lhs[k], k, i)
		}
	}
	lhs[k] = rhs
	return inputMap, nil
}
