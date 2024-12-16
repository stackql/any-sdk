package litetemplate_test

import (
	"encoding/json"
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
