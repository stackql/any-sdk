package anysdk_test

import (
	"testing"

	. "github.com/stackql/any-sdk/anysdk"
	"gopkg.in/yaml.v3"

	"gotest.tools/assert"
)

var (
	simpleYamlLoadTestInput string = `
predicate: sqlDialect == "sqlite3"
ddl: select * from someprovider.someservice.someresource
fallback:
    ddl: select * from someprovider.someservice.someresource where x = true
`
	noFallbackYamlLoadTestInput string = `
predicate: sqlDialect == "sqlite3"
ddl: select * from someprovider.someservice.someresource
`
)

func TestSimpleViewApi(t *testing.T) {

	v := GetTestingView()
	err := yaml.Unmarshal([]byte(simpleYamlLoadTestInput), &v)
	if err != nil {
		t.Fatalf("TestSimpleViewApi failed at unmarshal step, err = '%s'", err.Error())
	}

	ddlForSQLite3, ok := v.GetViewsForSqlDialect("sqlite3")
	if !ok {
		t.Fatalf("TestSimpleViewApi failed at get DDL for sqlite3 step")
	}
	assert.Assert(t, ddlForSQLite3[0].GetDDL() == "select * from someprovider.someservice.someresource")

	ddlForPostgres, ok := v.GetViewsForSqlDialect("postgres")
	if !ok {
		t.Fatalf("TestSimpleViewApi failed at get DDL for postgres step")
	}
	assert.Assert(t, ddlForPostgres[0].GetDDL() == "select * from someprovider.someservice.someresource where x = true")

	t.Logf("TestSimpleViewApi passed")
}

func TestNoFallbackViewApi(t *testing.T) {

	v := GetTestingView()
	err := yaml.Unmarshal([]byte(noFallbackYamlLoadTestInput), &v)
	if err != nil {
		t.Fatalf("TestNoFallbackViewApi failed at unmarshal step, err = '%s'", err.Error())
	}

	ddlForSQLite3, ok := v.GetViewsForSqlDialect("sqlite3")
	if !ok {
		t.Fatalf("TestNoFallbackViewApi failed at get DDL for sqlite3 step")
	}
	assert.Assert(t, ddlForSQLite3[0].GetDDL() == "select * from someprovider.someservice.someresource")

	_, ok = v.GetViewsForSqlDialect("postgres")
	if ok {
		t.Fatalf("TestNoFallbackViewApi failed at get DDL for postgres step; should **NOT** receive any DDL")
	}
	// assert.Assert(t, ddlForPostgres[0].GetDDL() == "")

	t.Logf("TestNoFallbackViewApi passed")
}
