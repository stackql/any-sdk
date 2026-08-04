package anysdk

import (
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"gotest.tools/assert"
)

const serverVarEnvTestDoc string = `{
	"url": "https://{endpoint}.snowflakecomputing.com",
	"variables": {
		"endpoint": {
			"default": "orgname-accountname",
			"x-stackQL-envVar": "ANY_SDK_TEST_SNOWFLAKE_ENDPOINT"
		}
	}
}`

func getServerVarEnvTestServer(t *testing.T) *openapi3.Server {
	var sv openapi3.Server
	err := json.Unmarshal([]byte(serverVarEnvTestDoc), &sv)
	assert.NilError(t, err)
	return &sv
}

func TestServerVariableEnvVarResolution(t *testing.T) {
	sv := getServerVarEnvTestServer(t)
	t.Setenv("ANY_SDK_TEST_SNOWFLAKE_ENDPOINT", "myorg-myaccount")

	urls, err := obtainServerURLsFromServers([]*openapi3.Server{sv}, nil)
	assert.NilError(t, err)
	assert.Equal(t, urls[0], "https://myorg-myaccount.snowflakecomputing.com")
}

func TestServerVariableExplicitValueBeatsEnvVar(t *testing.T) {
	sv := getServerVarEnvTestServer(t)
	t.Setenv("ANY_SDK_TEST_SNOWFLAKE_ENDPOINT", "myorg-myaccount")

	urls, err := obtainServerURLsFromServers([]*openapi3.Server{sv}, map[string]string{"endpoint": "explicit-value"})
	assert.NilError(t, err)
	assert.Equal(t, urls[0], "https://explicit-value.snowflakecomputing.com")
}

func TestServerVariableEnvVarAbsentFallsBackToDefault(t *testing.T) {
	sv := getServerVarEnvTestServer(t)
	t.Setenv("ANY_SDK_TEST_SNOWFLAKE_ENDPOINT", "")

	urls, err := obtainServerURLsFromServers([]*openapi3.Server{sv}, nil)
	assert.NilError(t, err)
	assert.Equal(t, urls[0], "https://orgname-accountname.snowflakecomputing.com")
}

func TestServerVariableRequirednessTracksEnvVar(t *testing.T) {
	sv := getServerVarEnvTestServer(t)

	t.Setenv("ANY_SDK_TEST_SNOWFLAKE_ENDPOINT", "")
	varMap := getServerVariablesMap(sv, nil)
	assert.Assert(t, varMap["endpoint"].IsRequired())

	t.Setenv("ANY_SDK_TEST_SNOWFLAKE_ENDPOINT", "myorg-myaccount")
	varMap = getServerVariablesMap(sv, nil)
	assert.Assert(t, !varMap["endpoint"].IsRequired())
}
