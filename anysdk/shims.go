package anysdk

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/stackql/any-sdk/pkg/brickmap"
	"github.com/stackql/stackql-parser/go/vt/sqlparser"
)

var (
	_ ObjectWithoutLineage        = &naiveObjectWithoutLineage{}
	_ ObjectWithLineage           = &standardObjectWithLineage{}
	_ ObjectWithLineageCollection = &standardObjectWithLineageCollection{}
)

type ObjectWithLineageCollection interface {
	Merge() error
	GetFlatObjects() []ObjectWithoutLineage
	PushBack(ObjectWithLineage)
}

type ObjectWithoutLineage interface {
	GetKey() string
	GetValue() interface{}
}

type naiveObjectWithoutLineage struct {
	key string
	val interface{}
}

func (nowl *naiveObjectWithoutLineage) GetKey() string {
	return nowl.key
}

func (nowl *naiveObjectWithoutLineage) GetValue() interface{} {
	return nowl.val
}

func newNaiveObjectWithoutLineage(k string, v interface{}) ObjectWithoutLineage {
	return &naiveObjectWithoutLineage{
		key: k,
		val: v,
	}
}

type ObjectWithLineage interface {
	ObjectWithoutLineage
	GetParentKey() string
}

type standardObjectWithLineageCollection struct {
	inputObjects  []ObjectWithLineage
	outputObjects []ObjectWithoutLineage
}

func newObjectWithLineageCollection() ObjectWithLineageCollection {
	return &standardObjectWithLineageCollection{}
}

func (oc *standardObjectWithLineageCollection) splitPath(prefixPath string, path string) []string {
	return append(strings.Split(prefixPath, "."), strings.Split(path, ".")...)
}

func (oc *standardObjectWithLineageCollection) Merge() error {
	// TODO: for each key, merge all lower level keys
	var err error
	preMergeMap := brickmap.NewBrickMap()
	for _, input := range oc.inputObjects {
		splitPath := oc.splitPath(input.GetParentKey(), input.GetKey())
		err = preMergeMap.Set(splitPath, input.GetValue())
		if err != nil {
			return err
		}
	}
	flatMap, _ := preMergeMap.ToFlatMap()
	for k, v := range flatMap {
		oc.outputObjects = append(oc.outputObjects, newNaiveObjectWithoutLineage(k, v))
	}
	return nil
}

func (oc *standardObjectWithLineageCollection) PushBack(input ObjectWithLineage) {
	oc.inputObjects = append(oc.inputObjects, input)
}

func (oc *standardObjectWithLineageCollection) GetFlatObjects() []ObjectWithoutLineage {
	return oc.outputObjects
}

type standardObjectWithLineage struct {
	parentKey string
	schema    Schema
	path      string
	val       interface{}
}

func (owl *standardObjectWithLineage) GetParentKey() string {
	return owl.parentKey
}

func (owl *standardObjectWithLineage) GetKey() string {
	return owl.path
}

func (owl *standardObjectWithLineage) GetValue() interface{} {
	return owl.val
}

func newObjectWithLineage(val interface{}, schema Schema, parentKey string, path string) ObjectWithLineage {
	return &standardObjectWithLineage{
		parentKey: parentKey,
		schema:    schema,
		path:      path,
		val:       val,
	}
}

func parseRequestBodyParam(k string, v interface{}, s Schema, method OperationStore) (ObjectWithLineage, bool) {
	trimmedKey, revertErr := method.revertRequestBodyAttributeRename(k)
	var parsedVal interface{}
	if revertErr == nil { //nolint:nestif // keep for now
		switch vt := v.(type) {
		case string:
			var isStringRestricted bool
			if s != nil {
				isStringRestrictedRaw, hasStr := s.getExtension(ExtensionKeyStringOnly)
				if hasStr {
					boolBytes, isBoolStr := isStringRestrictedRaw.([]byte)
					if isBoolStr && string(boolBytes) == "true" {
						isStringRestricted = true
					}
				}
			}
			var js map[string]interface{}
			var jArr []interface{}
			//nolint:gocritic // keep for now
			if isStringRestricted {
				parsedVal = vt
			} else if json.Unmarshal([]byte(vt), &js) == nil {
				parsedVal = js
			} else if json.Unmarshal([]byte(vt), &jArr) == nil {
				parsedVal = jArr
			} else {
				parsedVal = vt
			}
		case *sqlparser.FuncExpr:
			if strings.ToLower(vt.Name.GetRawVal()) == "string" && len(vt.Exprs) == 1 {
				pv, err := getStringFromStringFunc(vt)
				if err == nil {
					parsedVal = pv
				} else {
					parsedVal = vt
				}
			} else {
				parsedVal = vt
			}
		default:
			parsedVal = vt
		}
		parentKey, canInferParentKey := method.getRequestBodyAttributeParentKey(method.getRequestBodyTranslateAlgorithmString())
		if !canInferParentKey {
			parentKey = trimmedKey
		}
		return newObjectWithLineage(parsedVal, s, parentKey, trimmedKey), true
	}
	return nil, false
}

//nolint:gocognit // not super complex
func splitHTTPParameters(
	sqlParamMap map[int]map[string]interface{},
	method OperationStore,
) ([]HttpParameters, error) {
	var retVal []HttpParameters
	var rowKeys []int
	requestSchema, _ := method.GetRequestBodySchema()
	responseSchema, _ := method.GetRequestBodySchema()
	for idx := range sqlParamMap {
		rowKeys = append(rowKeys, idx)
	}
	sort.Ints(rowKeys)
	for _, key := range rowKeys {
		requestBodyParams := newObjectWithLineageCollection()
		sqlRow := sqlParamMap[key]
		reqMap := NewHttpParameters(method)
		for k, v := range sqlRow {
			if param, ok := method.GetOperationParameter(k); ok {
				reqMap.StoreParameter(param, v)
			} else {
				if requestSchema != nil {
					// if base is not nil then pre populate the request body with the base
					baseRequestBytes := method.getBaseRequestBodyBytes()
					if len(baseRequestBytes) > 0 {
						var m map[string]interface{}
						mapErr := json.Unmarshal(baseRequestBytes, &m)
						if mapErr != nil {
							return nil, fmt.Errorf("error unmarshalling base request: %v", mapErr)
						}
						for k, v := range m {
							reqMap.SetRequestBodyParam(k, v)
						}
					}
					kCleaned, _ := method.revertRequestBodyAttributeRename(k)
					prop, _ := requestSchema.GetProperty(kCleaned)
					rbp, rbpExists := parseRequestBodyParam(k, v, prop, method)
					if rbpExists {
						requestBodyParams.PushBack(rbp)
						continue
					}
				}
				reqMap.SetServerParam(k, method.GetService(), v)
			}
			if responseSchema != nil && responseSchema.FindByPath(k, nil) != nil {
				reqMap.SetResponseBodyParam(k, v)
			}
		}
		mergeErr := requestBodyParams.Merge()
		if mergeErr != nil {
			return nil, mergeErr
		}
		flattenedRequestBodyParams := requestBodyParams.GetFlatObjects()
		for _, rbp := range flattenedRequestBodyParams {
			rbpVal := rbp.GetValue()
			reqMap.SetRequestBodyParam(rbp.GetKey(), rbpVal)
		}
		retVal = append(retVal, reqMap)
	}
	return retVal, nil
}

func getStringFromStringFunc(fe *sqlparser.FuncExpr) (string, error) {
	if strings.ToLower(fe.Name.GetRawVal()) == "string" && len(fe.Exprs) == 1 {
		//nolint:gocritic // acceptable
		switch et := fe.Exprs[0].(type) {
		case *sqlparser.AliasedExpr:
			switch et2 := et.Expr.(type) {
			case *sqlparser.SQLVal:
				return string(et2.Val), nil
			}
		}
	}
	return "", fmt.Errorf("cannot extract string from func '%s'", fe.Name)
}
