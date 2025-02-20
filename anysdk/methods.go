package anysdk

import (
	"fmt"
)

type Methods map[string]standardOpenAPIOperationStore

func (ms Methods) FindMethod(key string) (StandardOperationStore, error) {
	if m, ok := ms[key]; ok {
		return &m, nil
	}
	return nil, fmt.Errorf("could not find method for key = '%s'", key)
}

func (ms Methods) OrderMethods() ([]StandardOperationStore, error) {
	var selectBin, insertBin, deleteBin, updateBin, replaceBin, execBin []StandardOperationStore
	for k, pv := range ms {
		v := pv
		switch v.GetSQLVerb() {
		case "select":
			v.setMethodKey(k)
			selectBin = append(selectBin, &v)
		case "insert":
			v.setMethodKey(k)
			insertBin = append(insertBin, &v)
		case "update":
			v.setMethodKey(k)
			updateBin = append(updateBin, &v)
		case "delete":
			v.setMethodKey(k)
			deleteBin = append(deleteBin, &v)
		case "replace":
			v.setMethodKey(k)
			replaceBin = append(replaceBin, &v)
		case "exec":
			v.setMethodKey(k)
			execBin = append(execBin, &v)
		default:
			v.setMethodKey(k)
			v.setSQLVerb("exec")
			execBin = append(execBin, &v)
		}
	}
	sortOpenAPIOperationStoreSlices(selectBin, insertBin, deleteBin, updateBin, replaceBin, execBin)
	rv := combineOpenAPIOperationStoreSlices(selectBin, insertBin, deleteBin, updateBin, replaceBin, execBin)
	return rv, nil
}

func (ms Methods) FindFromSelector(sel OperationSelector) (StandardOperationStore, error) {
	for _, m := range ms {
		if m.GetSQLVerb() == sel.GetSQLVerb() {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("could not locate operation for sql verb  = %s", sel.GetSQLVerb())
}
