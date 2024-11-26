package jsonpath_test

import (
	"testing"

	"gotest.tools/assert"

	"github.com/stackql/any-sdk/pkg/jsonpath"

	"encoding/json"

	"reflect"
)

func TestGet(t *testing.T) {
	var val interface{}
	jsonErr := json.Unmarshal([]byte(`{"a": {"b": "c"}}`), &val)
	assert.NilError(t, jsonErr)
	val, err := jsonpath.Get("$.a.b", val)
	assert.NilError(t, err)
	assert.Assert(t, true)
	assert.Equal(t, val, "c")
}

func TestSet(t *testing.T) {
	initVal := make(map[string]interface{})
	jsonErr := json.Unmarshal([]byte(`{"a": {"b": "c"}}`), &initVal)
	assert.NilError(t, jsonErr)
	val, err := jsonpath.Set(initVal, "a.b", float64(22))
	assert.NilError(t, err)
	assert.Assert(t, true)
	rhs := make(map[string]interface{})
	rhsErr := json.Unmarshal([]byte(`{"a": {"b": 22}}`), &rhs)
	assert.NilError(t, rhsErr)
	assert.Assert(t, reflect.DeepEqual(val, rhs))
}

func TestSetDollar(t *testing.T) {
	initVal := make(map[string]interface{})
	jsonErr := json.Unmarshal([]byte(`{"a": {"b": "c"}}`), &initVal)
	assert.NilError(t, jsonErr)
	val, err := jsonpath.Set(initVal, "$.a.b", float64(22))
	assert.NilError(t, err)
	assert.Assert(t, true)
	rhs := make(map[string]interface{})
	rhsErr := json.Unmarshal([]byte(`{"a": {"b": 22}}`), &rhs)
	assert.NilError(t, rhsErr)
	assert.Assert(t, reflect.DeepEqual(val, rhs))
}

func TestSplitPath(t *testing.T) {
	val, err := jsonpath.SplitSearchPath("$.a.b")
	assert.NilError(t, err)
	assert.Assert(t, true)
	rhs := []string{"a", "b"}
	assert.Assert(t, reflect.DeepEqual(val, rhs))
}

func TestSplitPathLong(t *testing.T) {
	val, err := jsonpath.SplitSearchPath("$.a.b.c[\"d.e\"]")
	assert.NilError(t, err)
	assert.Assert(t, true)
	rhs := []string{"a", "b", "c", `d.e`}
	assert.Assert(t, reflect.DeepEqual(val, rhs))
}
