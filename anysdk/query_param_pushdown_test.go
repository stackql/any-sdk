package anysdk_test

import (
	"testing"

	. "github.com/stackql/any-sdk/anysdk"
	"gopkg.in/yaml.v3"

	"gotest.tools/assert"
)

var (
	odataFullYamlInput string = `
select:
  dialect: odata
  supportedColumns:
    - "id"
    - "displayName"
    - "mail"
filter:
  dialect: odata
  supportedOperators:
    - "eq"
    - "ne"
    - "gt"
    - "lt"
    - "contains"
    - "startswith"
  supportedColumns:
    - "displayName"
    - "status"
    - "createdDate"
orderBy:
  dialect: odata
  supportedColumns:
    - "name"
    - "createdDate"
top:
  dialect: odata
  maxValue: 1000
count:
  dialect: odata
`

	customDialectYamlInput string = `
select:
  paramName: "fields"
  delimiter: ","
  supportedColumns:
    - "*"
filter:
  paramName: "filter"
  syntax: "key_value"
  supportedOperators:
    - "eq"
  supportedColumns:
    - "status"
    - "region"
orderBy:
  paramName: "sort"
  syntax: "prefix"
  supportedColumns:
    - "createdAt"
    - "name"
top:
  paramName: "limit"
  maxValue: 100
count:
  paramName: "include_count"
  paramValue: "1"
  responseKey: "meta.total"
`

	minimalOdataYamlInput string = `
filter:
  dialect: odata
  supportedOperators:
    - "eq"
`
)

func TestODataFullConfig(t *testing.T) {
	qpp := GetTestingQueryParamPushdown()
	err := yaml.Unmarshal([]byte(odataFullYamlInput), &qpp)
	if err != nil {
		t.Fatalf("TestODataFullConfig failed at unmarshal step, err = '%s'", err.Error())
	}

	// Test select pushdown
	selectPD, ok := qpp.GetSelect()
	if !ok {
		t.Fatalf("TestODataFullConfig failed: expected select pushdown to exist")
	}
	assert.Equal(t, selectPD.GetDialect(), "odata")
	assert.Equal(t, selectPD.GetParamName(), "$select") // OData default
	assert.Equal(t, selectPD.GetDelimiter(), ",")       // OData default
	assert.Assert(t, selectPD.IsColumnSupported("displayName"))
	assert.Assert(t, !selectPD.IsColumnSupported("unknownColumn"))

	// Test filter pushdown
	filterPD, ok := qpp.GetFilter()
	if !ok {
		t.Fatalf("TestODataFullConfig failed: expected filter pushdown to exist")
	}
	assert.Equal(t, filterPD.GetDialect(), "odata")
	assert.Equal(t, filterPD.GetParamName(), "$filter") // OData default
	assert.Equal(t, filterPD.GetSyntax(), "odata")      // OData default
	assert.Assert(t, filterPD.IsOperatorSupported("eq"))
	assert.Assert(t, filterPD.IsOperatorSupported("contains"))
	assert.Assert(t, !filterPD.IsOperatorSupported("like"))
	assert.Assert(t, filterPD.IsColumnSupported("displayName"))
	assert.Assert(t, !filterPD.IsColumnSupported("unknownColumn"))

	// Test orderBy pushdown
	orderByPD, ok := qpp.GetOrderBy()
	if !ok {
		t.Fatalf("TestODataFullConfig failed: expected orderBy pushdown to exist")
	}
	assert.Equal(t, orderByPD.GetDialect(), "odata")
	assert.Equal(t, orderByPD.GetParamName(), "$orderby") // OData default
	assert.Equal(t, orderByPD.GetSyntax(), "odata")       // OData default
	assert.Assert(t, orderByPD.IsColumnSupported("name"))
	assert.Assert(t, !orderByPD.IsColumnSupported("unknownColumn"))

	// Test top pushdown
	topPD, ok := qpp.GetTop()
	if !ok {
		t.Fatalf("TestODataFullConfig failed: expected top pushdown to exist")
	}
	assert.Equal(t, topPD.GetDialect(), "odata")
	assert.Equal(t, topPD.GetParamName(), "$top") // OData default
	assert.Equal(t, topPD.GetMaxValue(), 1000)

	// Test count pushdown
	countPD, ok := qpp.GetCount()
	if !ok {
		t.Fatalf("TestODataFullConfig failed: expected count pushdown to exist")
	}
	assert.Equal(t, countPD.GetDialect(), "odata")
	assert.Equal(t, countPD.GetParamName(), "$count")           // OData default
	assert.Equal(t, countPD.GetParamValue(), "true")            // OData default
	assert.Equal(t, countPD.GetResponseKey(), "@odata.count")   // OData default

	t.Logf("TestODataFullConfig passed")
}

func TestCustomDialectConfig(t *testing.T) {
	qpp := GetTestingQueryParamPushdown()
	err := yaml.Unmarshal([]byte(customDialectYamlInput), &qpp)
	if err != nil {
		t.Fatalf("TestCustomDialectConfig failed at unmarshal step, err = '%s'", err.Error())
	}

	// Test select pushdown with custom params
	selectPD, ok := qpp.GetSelect()
	if !ok {
		t.Fatalf("TestCustomDialectConfig failed: expected select pushdown to exist")
	}
	assert.Equal(t, selectPD.GetDialect(), "custom")
	assert.Equal(t, selectPD.GetParamName(), "fields")
	assert.Equal(t, selectPD.GetDelimiter(), ",")
	assert.Assert(t, selectPD.IsColumnSupported("anyColumn")) // "*" means all supported

	// Test filter pushdown with custom params
	filterPD, ok := qpp.GetFilter()
	if !ok {
		t.Fatalf("TestCustomDialectConfig failed: expected filter pushdown to exist")
	}
	assert.Equal(t, filterPD.GetDialect(), "custom")
	assert.Equal(t, filterPD.GetParamName(), "filter")
	assert.Equal(t, filterPD.GetSyntax(), "key_value")
	assert.Assert(t, filterPD.IsOperatorSupported("eq"))
	assert.Assert(t, !filterPD.IsOperatorSupported("ne")) // Only eq is supported

	// Test orderBy pushdown with custom params
	orderByPD, ok := qpp.GetOrderBy()
	if !ok {
		t.Fatalf("TestCustomDialectConfig failed: expected orderBy pushdown to exist")
	}
	assert.Equal(t, orderByPD.GetDialect(), "custom")
	assert.Equal(t, orderByPD.GetParamName(), "sort")
	assert.Equal(t, orderByPD.GetSyntax(), "prefix")

	// Test top pushdown with custom params
	topPD, ok := qpp.GetTop()
	if !ok {
		t.Fatalf("TestCustomDialectConfig failed: expected top pushdown to exist")
	}
	assert.Equal(t, topPD.GetDialect(), "custom")
	assert.Equal(t, topPD.GetParamName(), "limit")
	assert.Equal(t, topPD.GetMaxValue(), 100)

	// Test count pushdown with custom params
	countPD, ok := qpp.GetCount()
	if !ok {
		t.Fatalf("TestCustomDialectConfig failed: expected count pushdown to exist")
	}
	assert.Equal(t, countPD.GetDialect(), "custom")
	assert.Equal(t, countPD.GetParamName(), "include_count")
	assert.Equal(t, countPD.GetParamValue(), "1")
	assert.Equal(t, countPD.GetResponseKey(), "meta.total")

	t.Logf("TestCustomDialectConfig passed")
}

func TestMinimalODataConfig(t *testing.T) {
	qpp := GetTestingQueryParamPushdown()
	err := yaml.Unmarshal([]byte(minimalOdataYamlInput), &qpp)
	if err != nil {
		t.Fatalf("TestMinimalODataConfig failed at unmarshal step, err = '%s'", err.Error())
	}

	// Select should not exist
	_, ok := qpp.GetSelect()
	assert.Assert(t, !ok, "expected select pushdown to NOT exist")

	// Filter should exist with OData defaults
	filterPD, ok := qpp.GetFilter()
	if !ok {
		t.Fatalf("TestMinimalODataConfig failed: expected filter pushdown to exist")
	}
	assert.Equal(t, filterPD.GetDialect(), "odata")
	assert.Equal(t, filterPD.GetParamName(), "$filter")
	assert.Assert(t, filterPD.IsOperatorSupported("eq"))
	// All columns supported when not specified
	assert.Assert(t, filterPD.IsColumnSupported("anyColumn"))

	// OrderBy should not exist
	_, ok = qpp.GetOrderBy()
	assert.Assert(t, !ok, "expected orderBy pushdown to NOT exist")

	// Top should not exist
	_, ok = qpp.GetTop()
	assert.Assert(t, !ok, "expected top pushdown to NOT exist")

	// Count should not exist
	_, ok = qpp.GetCount()
	assert.Assert(t, !ok, "expected count pushdown to NOT exist")

	t.Logf("TestMinimalODataConfig passed")
}

func TestEmptySupportedColumns(t *testing.T) {
	// When supportedColumns is empty/nil, all columns should be supported
	qpp := GetTestingQueryParamPushdown()
	yamlInput := `
filter:
  dialect: odata
  supportedOperators:
    - "eq"
`
	err := yaml.Unmarshal([]byte(yamlInput), &qpp)
	if err != nil {
		t.Fatalf("TestEmptySupportedColumns failed at unmarshal step, err = '%s'", err.Error())
	}

	filterPD, ok := qpp.GetFilter()
	if !ok {
		t.Fatalf("TestEmptySupportedColumns failed: expected filter pushdown to exist")
	}

	// All columns should be supported when list is empty
	assert.Assert(t, filterPD.IsColumnSupported("anyColumn"))
	assert.Assert(t, filterPD.IsColumnSupported("anotherColumn"))

	t.Logf("TestEmptySupportedColumns passed")
}

func TestWildcardSupportedColumns(t *testing.T) {
	// When supportedColumns contains "*", all columns should be supported
	qpp := GetTestingQueryParamPushdown()
	yamlInput := `
filter:
  dialect: odata
  supportedOperators:
    - "eq"
  supportedColumns:
    - "*"
`
	err := yaml.Unmarshal([]byte(yamlInput), &qpp)
	if err != nil {
		t.Fatalf("TestWildcardSupportedColumns failed at unmarshal step, err = '%s'", err.Error())
	}

	filterPD, ok := qpp.GetFilter()
	if !ok {
		t.Fatalf("TestWildcardSupportedColumns failed: expected filter pushdown to exist")
	}

	// All columns should be supported with "*"
	assert.Assert(t, filterPD.IsColumnSupported("anyColumn"))
	assert.Assert(t, filterPD.IsColumnSupported("anotherColumn"))

	t.Logf("TestWildcardSupportedColumns passed")
}
