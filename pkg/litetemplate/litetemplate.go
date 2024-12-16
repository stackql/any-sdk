package litetemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
)

const (
	EnvPrefix string = "__env__"
)

func RenderTemplateFromSerializable(templateString string, data interface{}) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	var m map[string]interface{}
	err = json.Unmarshal(b, &m)
	if err != nil {
		return "", err
	}
	for _, keyVal := range os.Environ() {
		kv := strings.SplitN(keyVal, "=", 2)
		if len(kv) != 2 {
			continue
		}
		m[fmt.Sprintf("%s%s", EnvPrefix, kv[0])] = kv[1]
	}
	return renderTemplate(templateString, m)
}

func renderTemplate(templateString string, data interface{}) (string, error) {
	liteTmpl, newErr := newLiteTemplate()
	if newErr != nil {
		return "", newErr
	}
	return liteTmpl.render(templateString, data)
}

type liteTemplate struct {
}

func (lt *liteTemplate) generate(templateString string) (*template.Template, error) {
	tmpl, err := template.New("example").
		// Delims("{", "}").
		Parse(templateString)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

func (lt *liteTemplate) render(templateString string, data interface{}) (string, error) {
	var buf bytes.Buffer
	tmpl, err := lt.generate(templateString)
	if err != nil {
		return "", err
	}
	exErr := tmpl.Execute(&buf, data)
	if err != nil {
		return "", exErr
	}
	return buf.String(), nil
}

func newLiteTemplate() (*liteTemplate, error) {
	return &liteTemplate{}, nil
}
