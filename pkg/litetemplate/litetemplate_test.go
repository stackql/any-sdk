package litetemplate_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stackql/any-sdk/pkg/litetemplate"
	"gotest.tools/assert"
)

func TestRenderTemplate(t *testing.T) {
	t.Parallel()
	templateString := `{{.Name}}`
	data := struct {
		Name string
	}{
		Name: "example",
	}
	expected := "example"
	actual, err := litetemplate.RenderTemplateFromSerializable(templateString, data)

	assert.NilError(t, err)
	assert.Equal(t, expected, actual)
}

func TestRenderEnvTemplate(t *testing.T) {
	os.Unsetenv("UNIT_TEST_MY_SILLY_NAME")
	os.Setenv("UNIT_TEST_MY_SILLY_NAME", "some_example")
	t.Parallel()
	templateString := `{{ .__env__UNIT_TEST_MY_SILLY_NAME }}`
	data := map[string]interface{}{}
	expected := "some_example"
	actual, err := litetemplate.RenderTemplateFromSerializable(templateString, data)

	assert.NilError(t, err)
	assert.Equal(t, expected, actual)
}

func TestRenderEnvTemplateExotic(t *testing.T) {
	os.Unsetenv("_UNIT_TEST_MY_SILLY_NAME")
	os.Setenv("_UNIT_TEST_MY_SILLY_NAME", "Welcome to the sunlight")
	t.Parallel()
	templateString := `{{ .message }}: {{ .__env___UNIT_TEST_MY_SILLY_NAME }}`
	data := map[string]interface{}{
		"message": "hello, here is my env var dereferenced",
	}
	expected := "hello, here is my env var dereferenced: Welcome to the sunlight"
	actual, err := litetemplate.RenderTemplateFromSerializable(templateString, data)

	assert.NilError(t, err)
	assert.Equal(t, expected, actual)
}

func TestRenderEnvURLTemplateExotic(t *testing.T) {
	os.Setenv("_UNIT_TEST_MY_HOST_NAME", "my.domain.com")
	os.Setenv("_UNIT_TEST_MY_LHS_SUB_PATH", "system")
	os.Setenv("_UNIT_TEST_MY_RHS_SUB_PATH", "identity/rules/share")
	t.Parallel()
	templateString := `{{ .protocol }}://{{ .__env___UNIT_TEST_MY_HOST_NAME }}/{{ .__env___UNIT_TEST_MY_LHS_SUB_PATH }}/{{ .__env___UNIT_TEST_MY_RHS_SUB_PATH }}{{ .suffix }}`
	data := map[string]interface{}{
		"protocol": "https",
		"suffix":   "?a=1&b=2",
	}
	expected := "https://my.domain.com/system/identity/rules/share?a=1&b=2"
	actual, err := litetemplate.RenderTemplateFromSerializable(templateString, data)

	assert.NilError(t, err)
	assert.Equal(t, expected, actual)

	os.Unsetenv("_UNIT_TEST_MY_HOST_NAME")
	os.Unsetenv("_UNIT_TEST_MY_LHS_SUB_PATH")
	os.Unsetenv("_UNIT_TEST_MY_RHS_SUB_PATH")

}

func TestRenderTemplateFromSerializable(t *testing.T) {
	t.Parallel()
	type testingStructure struct {
		SomeOtherName string `json:"my_var" yaml:"my_var"`
	}
	templateString := `{{.my_var}}`
	var data testingStructure
	json.Unmarshal([]byte(`{"my_var":"example"}`), &data)
	expected := "example"
	actual, err := litetemplate.RenderTemplateFromSerializable(templateString, data)

	assert.NilError(t, err)
	assert.Equal(t, expected, actual)
}

func TestRenderURLTemplateFromSerializable(t *testing.T) {
	t.Parallel()
	type testingStructure struct {
		SomeOtherName string `json:"my_var" yaml:"my_var"`
	}
	templateString := `https://example.com/{{.my_var}}/token`
	var data testingStructure
	json.Unmarshal([]byte(`{"my_var":"example"}`), &data)
	expected := "https://example.com/example/token"
	actual, err := litetemplate.RenderTemplateFromSerializable(templateString, data)

	assert.NilError(t, err)
	assert.Equal(t, expected, actual)
}

func TestRenderNilURLTemplateFromSerializable(t *testing.T) {
	t.Parallel()
	type testingStructure struct {
		SomeOtherName string `json:"my_var" yaml:"my_var"`
	}
	templateString := `https://example.com/`
	var data testingStructure
	json.Unmarshal([]byte(`{"my_var":"example"}`), &data)
	expected := "https://example.com/"
	actual, err := litetemplate.RenderTemplateFromSerializable(templateString, data)

	assert.NilError(t, err)
	assert.Equal(t, expected, actual)
}

// func TestRenderErroneousTemplateFromSerializable(t *testing.T) {
// 	t.Parallel()
// 	type testingStructure struct {
// 		SomeOtherName string `json:"my_var" yaml:"my_var"`
// 	}
// 	templateString := `{{.non_existent_var}}`
// 	var data testingStructure
// 	json.Unmarshal([]byte(`{"my_var":"example"}`), &data)
// 	expected := "example"
// 	actual, err := litetemplate.RenderTemplateFromSerializable(templateString, data)

// 	assert.ErrorContains(t, err, "expected string")
// 	assert.Equal(t, expected, actual)
// }
