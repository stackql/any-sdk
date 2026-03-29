package discovery

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stackql/any-sdk/internal/anysdk"
)

// GenerateSampleResponse walks a response schema and produces a sample
// response body as a JSON string. This is used for empirical template testing.
func GenerateSampleResponse(schema anysdk.Schema, mediaType string) string {
	if schema == nil {
		return ""
	}
	sample := generateSampleValue(schema, 0)
	if sample == nil {
		return ""
	}
	out, err := json.MarshalIndent(sample, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\"__error\": \"failed to marshal sample: %v\"}", err)
	}
	return string(out)
}

// GenerateSampleXMLResponse produces a minimal XML sample from the schema.
func GenerateSampleXMLResponse(schema anysdk.Schema, rootElement string) string {
	if schema == nil {
		return ""
	}
	if rootElement == "" {
		rootElement = "Response"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<%s>", rootElement))
	generateSampleXML(&sb, schema, 1)
	sb.WriteString(fmt.Sprintf("</%s>", rootElement))
	return sb.String()
}

const maxSampleDepth = 5

func generateSampleValue(schema anysdk.Schema, depth int) interface{} {
	if depth > maxSampleDepth {
		return nil
	}

	schemaType := schema.GetType()
	switch schemaType {
	case "string":
		return "sample_string"
	case "integer":
		return 0
	case "number":
		return 0.0
	case "boolean":
		return false
	case "array":
		items, err := schema.GetItems()
		if err != nil || items == nil {
			return []interface{}{}
		}
		itemSample := generateSampleValue(items, depth+1)
		if itemSample == nil {
			return []interface{}{}
		}
		return []interface{}{itemSample}
	case "object", "":
		return generateSampleObject(schema, depth)
	default:
		return nil
	}
}

func generateSampleObject(schema anysdk.Schema, depth int) map[string]interface{} {
	result := make(map[string]interface{})
	props, err := schema.GetProperties()
	if err != nil {
		return result
	}
	for key, propSchema := range props {
		val := generateSampleValue(propSchema, depth+1)
		if val != nil {
			result[key] = val
		}
	}
	return result
}

func generateSampleXML(sb *strings.Builder, schema anysdk.Schema, depth int) {
	if depth > maxSampleDepth {
		return
	}
	schemaType := schema.GetType()
	switch schemaType {
	case "string":
		sb.WriteString("sample_string")
	case "integer":
		sb.WriteString("0")
	case "number":
		sb.WriteString("0.0")
	case "boolean":
		sb.WriteString("false")
	case "array":
		items, err := schema.GetItems()
		if err != nil || items == nil {
			return
		}
		sb.WriteString("<item>")
		generateSampleXML(sb, items, depth+1)
		sb.WriteString("</item>")
	case "object", "":
		props, err := schema.GetProperties()
		if err != nil {
			return
		}
		for key, propSchema := range props {
			sb.WriteString(fmt.Sprintf("<%s>", key))
			generateSampleXML(sb, propSchema, depth+1)
			sb.WriteString(fmt.Sprintf("</%s>", key))
		}
	}
}
