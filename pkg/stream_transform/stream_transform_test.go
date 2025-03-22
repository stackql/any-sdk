package stream_transform_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	. "github.com/stackql/any-sdk/pkg/stream_transform"
)

var (
	_           io.Reader = &bytes.Buffer{}
	_           io.Writer = &bytes.Buffer{}
	jsonExample           = `{ 
	  "animals": [
		{"name": "Platypus", "order": "Monotremata", "votes": 1, "bank_balance": 100.0},
		{"name": "Quokka", "order": "Diprotodontia", "votes": 3, "bank_balance": 200.0},
		{"name": "Quoll", "order": "Dasyuromorphia", "votes": 2, "bank_balance": 300.0, "premierships": [1993, 2000]}
	  ],
	  "meta": {
	    "institution": "University of Tasmania",
		"total_votes": 6,
		"total_bank_balance": 600.0
	  }
	}`
	xmlExample = `<?xml version="1.0" encoding="UTF-8"?>
	<root>
	  <animals>
	 		<animal>
				<name>Platypus</name>
				<order>Monotremata</order>
				<votes>1</votes>
				<bank_balance>100.0</bank_balance>
			</animal>
			<animal>
				<name>Quokka</name>
				<order>Diprotodontia</order>
				<votes>3</votes>
				<bank_balance>200.0</bank_balance>
			</animal>
			<animal>
				<name>Quoll</name>
				<order>Dasyuromorphia</order>
				<votes>2</votes>
				<bank_balance>300.0</bank_balance>
				<premierships>
					<premiership>1993</premiership>
					<premiership>2000</premiership>
				</premierships>
			</animal>
		</animals>
		<meta>
			<institution>University of Tasmania</institution>
			<total_votes>6</total_votes>
			<total_bank_balance>600.0</total_bank_balance>
		</meta>
	</root>  	
	  `
	xmlSchema = &openapi3.Schema{
		Type: "object",
		Properties: openapi3.Schemas{
			"meta": &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: "object",
					Properties: openapi3.Schemas{
						"institution": &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: "string",
							},
						},
						"total_votes": &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: "integer",
							},
						},
						"total_bank_balance": &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: "number",
							},
						},
					},
				},
			},
			"animals": &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type: "list",
					Items: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: "object",
							Properties: openapi3.Schemas{
								"name": &openapi3.SchemaRef{
									Value: &openapi3.Schema{
										Type: "string",
									},
								},
								"order": &openapi3.SchemaRef{
									Value: &openapi3.Schema{
										Type: "string",
									},
								},
								"votes": &openapi3.SchemaRef{
									Value: &openapi3.Schema{
										Type: "integer",
									},
								},
								"bank_balance": &openapi3.SchemaRef{
									Value: &openapi3.Schema{
										Type: "number",
									},
								},
								"premierships": &openapi3.SchemaRef{
									Value: &openapi3.Schema{
										Type: "list",
										Items: &openapi3.SchemaRef{
											Value: &openapi3.Schema{
												Type: "integer",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	yamlExample = `---
animals:
	- name: Platypus
	  order: Monotremata
	  votes: 1
	  bank_balance: 100.0				
	- name: Quokka
	  order: Diprotodontia
	  votes: 3
	  bank_balance: 200.0
	- name: Quoll
	  order: Dasyuromorphia
	  votes: 2
	  bank_balance: 300.0
	  premierships:
		- 1993
		- 2000
meta:	
	institution: University of Tasmania
	total_votes: 6
	total_bank_balance: 600.0
`
	jsonTmpl = `
	{{- $s := separator ", " -}}
	[
	{{- range $idx, $animal := $.animals -}}
	{{- call $s}}{"name": "{{ $animal.name }}", "democratic_votes": {{ $animal.votes }}} 
	{{- end -}}
	]`
	xmlTmpl            = `[{ "name": "{{- getXPath . "/root/animals/animal/name" }}"}]`
	expectedJsonOutput = `[{"name": "Platypus", "democratic_votes": 1}, {"name": "Quokka", "democratic_votes": 3}, {"name": "Quoll", "democratic_votes": 2}]`
)

func TestSimpleStreamTransform(t *testing.T) {
	input := fmt.Sprintf(`"Hello, %s!"`, "World")
	t.Log("TestSimpleStream")
	tmpl := `{{.}}`
	inStream := NewJSONReader(bytes.NewBufferString(input))
	outStream := bytes.NewBuffer(nil)
	tfm, err := NewTemplateStreamTransformer(tmpl, inStream, outStream)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}
	if err := tfm.Transform(); err != nil {
		t.Fatalf("failed to transform: %v", err)
	}
	outputStr := outStream.String()
	if outputStr != "Hello, World!" {
		t.Fatalf("unexpected output: %s", outStream.String())
	}
}

func TestMeaningfulStreamTransform(t *testing.T) {
	input := jsonExample
	t.Log("TestSimpleStream")
	tmpl := jsonTmpl
	inStream := NewJSONReader(bytes.NewBufferString(input))
	outStream := bytes.NewBuffer(nil)
	tfm, err := NewTemplateStreamTransformer(tmpl, inStream, outStream)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}
	if err := tfm.Transform(); err != nil {
		t.Fatalf("failed to transform: %v", err)
	}
	outputStr := outStream.String()
	if outputStr != expectedJsonOutput {
		t.Fatalf("unexpected output: '%s' != '%s'", outputStr, expectedJsonOutput)
	}
}

func TestSimpleXMLStreamTransform(t *testing.T) {
	input := xmlExample
	t.Log("v")
	tmpl := xmlTmpl
	inStream := NewTextReader(bytes.NewBufferString(input))
	outStream := bytes.NewBuffer(nil)
	tfm, err := NewTemplateStreamTransformer(tmpl, inStream, outStream)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}
	if err := tfm.Transform(); err != nil {
		t.Fatalf("failed to transform: %v", err)
	}
	outputStr := outStream.String()
	expected := `[{ "name": "Platypus"}]`
	if outputStr != expected {
		t.Fatalf("unexpected output: '%s' != '%s'", outputStr, expected)
	}
}
