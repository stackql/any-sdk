package jsonpath

import (
	"strings"

	"github.com/PaesslerAG/gval"
	jp "github.com/PaesslerAG/jsonpath"

	"context"
)

func Get(path string, value interface{}) (interface{}, error) {
	return jp.Get(path, value)
}

func Set(pathStr string, value interface{}, rhs interface{}) (interface{}, error) {
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
	rv, err := lang.Evaluate(
		pathStr,
		value,
	)
	// rv, err := gval.Evaluate(pathStr,
	// 	"!",
	// 	gval.VariableSelector(func(path gval.Evaluables) gval.Evaluable {
	// 		return func(c context.Context, v interface{}) (interface{}, error) {
	// 			keys, err := path.EvalStrings(c, v)
	// 			if err != nil {
	// 				return nil, err
	// 			}
	// 			return keys, nil
	// 		}
	// 	}),
	// )
	if err != nil {
		return nil, err
	}
	return rv, nil
}
