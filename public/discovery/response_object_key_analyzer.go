package discovery

import (
	"fmt"
	"strings"

	"github.com/stackql/any-sdk/internal/anysdk"
	"github.com/stackql/any-sdk/pkg/media"
	"github.com/stackql/any-sdk/pkg/openapitopath"
	"github.com/stackql/any-sdk/pkg/stream_transform"
)

type responseObjectKeyAnalysisResult struct {
	errors       []error
	warnings     []string
	affirmatives []string
	findings     []AnalysisFinding
}

func analyzeResponseAndObjectKey(
	actx AnalysisContext,
	method anysdk.StandardOperationStore,
) responseObjectKeyAnalysisResult {
	result := responseObjectKeyAnalysisResult{}

	expectedResponse, hasExpectedResponse := method.GetResponse()
	objectKey := method.GetSelectItemsKeySimple()
	hasObjectKey := objectKey != ""

	responseSchema, responseMediaType, responseSchemaErr := method.GetResponseBodySchemaAndMediaType()

	if responseSchemaErr != nil {
		if hasObjectKey {
			f := actx.NewError(BinObjectKeyUnroutable, fmt.Sprintf("objectKey '%s' specified but no response schema could be resolved: %v", objectKey, responseSchemaErr))
			result.errors = append(result.errors, f)
			result.findings = append(result.findings, f)
		}
		if method.ShouldBeSelectable() && !hasTransformOverride(expectedResponse, hasExpectedResponse) {
			f := actx.NewWarning(BinEmptyResponseUnsafe, "selectable method has no response schema and no transform override")
			result.warnings = append(result.warnings, f.String())
			result.findings = append(result.findings, f)
		}
		return result
	}

	if responseSchema == nil {
		if hasObjectKey {
			f := actx.NewError(BinObjectKeyUnroutable, fmt.Sprintf("objectKey '%s' specified but response schema is nil", objectKey))
			result.errors = append(result.errors, f)
			result.findings = append(result.findings, f)
		}
		return result
	}

	schemaType := responseSchema.GetType()
	isEmptyObjectSchema := schemaType == "object" && !schemaHasProperties(responseSchema)
	isLeafSchema := schemaType == "string" || schemaType == "integer" || schemaType == "boolean" || schemaType == "number"

	if isEmptyObjectSchema && hasObjectKey {
		f := actx.NewWarning(BinEmptyResponseUnsafe, fmt.Sprintf("objectKey '%s' navigates into an empty object schema (no properties defined)", objectKey))
		result.warnings = append(result.warnings, f.String())
		result.findings = append(result.findings, f)
	}

	if hasObjectKey {
		routabilityResult := analyzeObjectKeyRoutability(actx, method, responseSchema, objectKey, responseMediaType)
		result.errors = append(result.errors, routabilityResult.errors...)
		result.warnings = append(result.warnings, routabilityResult.warnings...)
		result.affirmatives = append(result.affirmatives, routabilityResult.affirmatives...)
		result.findings = append(result.findings, routabilityResult.findings...)
	}

	if hasExpectedResponse {
		transformResult := analyzeResponseTransform(actx, expectedResponse, objectKey, isLeafSchema, isEmptyObjectSchema)
		result.errors = append(result.errors, transformResult.errors...)
		result.warnings = append(result.warnings, transformResult.warnings...)
		result.affirmatives = append(result.affirmatives, transformResult.affirmatives...)
		result.findings = append(result.findings, transformResult.findings...)
	}

	return result
}

func analyzeObjectKeyRoutability(
	actx AnalysisContext,
	method anysdk.StandardOperationStore,
	responseSchema anysdk.Schema,
	objectKey string,
	responseMediaType string,
) responseObjectKeyAnalysisResult {
	result := responseObjectKeyAnalysisResult{}

	pathLang := detectPathLanguage(objectKey)
	expectedMediaType := inferExpectedMediaType(responseMediaType)

	if pathLang != pathLangBare {
		if pathLang == pathLangJSONPath && expectedMediaType == media.MediaTypeXML {
			f := actx.NewWarning(BinMediaTypeMismatch, fmt.Sprintf("objectKey '%s' uses JSONPath syntax but response media type is '%s'", objectKey, responseMediaType))
			result.warnings = append(result.warnings, f.String())
			result.findings = append(result.findings, f)
		}
		if pathLang == pathLangXPath && expectedMediaType == media.MediaTypeJson {
			f := actx.NewWarning(BinMediaTypeMismatch, fmt.Sprintf("objectKey '%s' uses XPath syntax but response media type is '%s'", objectKey, responseMediaType))
			result.warnings = append(result.warnings, f.String())
			result.findings = append(result.findings, f)
		}
	}

	pathSegments := resolvePathSegments(objectKey, pathLang)
	walkResult := walkSchemaPath(actx, responseSchema, pathSegments, pathLang, objectKey)
	result.errors = append(result.errors, walkResult.errors...)
	result.warnings = append(result.warnings, walkResult.warnings...)
	result.findings = append(result.findings, walkResult.findings...)

	_, _, selectErr := method.GetSelectSchemaAndObjectPath()
	if selectErr != nil {
		f := actx.NewError(BinObjectKeyUnroutable, fmt.Sprintf("objectKey '%s' failed schema resolution: %v", objectKey, selectErr))
		result.errors = append(result.errors, f)
		result.findings = append(result.findings, f)
	} else {
		result.affirmatives = append(result.affirmatives, fmt.Sprintf("objectKey '%s' successfully routed through response schema", objectKey))
	}

	return result
}

// Path language constants
const (
	pathLangBare     = "bare"
	pathLangJSONPath = "jsonpath"
	pathLangXPath    = "xpath"
)

func detectPathLanguage(objectKey string) string {
	if strings.HasPrefix(objectKey, "$") {
		return pathLangJSONPath
	}
	if strings.HasPrefix(objectKey, "/") {
		return pathLangXPath
	}
	return pathLangBare
}

func inferExpectedMediaType(responseMediaType string) string {
	lower := strings.ToLower(responseMediaType)
	if strings.Contains(lower, "xml") {
		return media.MediaTypeXML
	}
	if strings.Contains(lower, "json") {
		return media.MediaTypeJson
	}
	return responseMediaType
}

func resolvePathSegments(objectKey string, pathLang string) []string {
	switch pathLang {
	case pathLangJSONPath:
		return openapitopath.NewJSONPathResolver().ToPathSlice(objectKey)
	case pathLangXPath:
		return openapitopath.NewXPathResolver().ToPathSlice(objectKey)
	default:
		return []string{objectKey}
	}
}

func walkSchemaPath(
	actx AnalysisContext,
	schema anysdk.Schema,
	segments []string,
	pathLang string,
	objectKey string,
) responseObjectKeyAnalysisResult {
	result := responseObjectKeyAnalysisResult{}

	current := schema
	for i, segment := range segments {
		if current == nil {
			f := actx.NewWarning(BinObjectKeyUnroutable, fmt.Sprintf("objectKey '%s' path walk terminated at nil schema at segment [%d] '%s'", objectKey, i, segment))
			result.warnings = append(result.warnings, f.String())
			result.findings = append(result.findings, f)
			return result
		}

		if segment == "*" || segment == "[*]" {
			if pathLang == pathLangXPath && i == 0 {
				continue
			}
			items, itemsErr := current.GetItems()
			if itemsErr != nil {
				additionalProps, hasAdditionalProps := current.GetAdditionalProperties()
				if !hasAdditionalProps {
					f := actx.NewWarning(BinObjectKeyUnroutable, fmt.Sprintf("objectKey '%s' uses wildcard '%s' at segment [%d] but schema is not an array/map (type='%s')", objectKey, segment, i, current.GetType()))
					result.warnings = append(result.warnings, f.String())
					result.findings = append(result.findings, f)
					return result
				}
				current = additionalProps
			} else {
				current = items
			}
			continue
		}

		prop, ok := current.GetProperty(segment)
		if !ok {
			if current.GetType() == "array" {
				items, itemsErr := current.GetItems()
				if itemsErr == nil {
					prop, ok = items.GetProperty(segment)
					if ok {
						current = prop
						continue
					}
				}
			}
			f := actx.NewWarning(BinObjectKeyUnroutable, fmt.Sprintf("objectKey '%s' segment [%d] '%s' not found in schema properties (type='%s')", objectKey, i, segment, current.GetType()))
			result.warnings = append(result.warnings, f.String())
			result.findings = append(result.findings, f)
			return result
		}
		current = prop
	}

	return result
}

func analyzeResponseTransform(
	actx AnalysisContext,
	expectedResponse anysdk.ExpectedResponse,
	objectKey string,
	isLeafSchema bool,
	isEmptyObjectSchema bool,
) responseObjectKeyAnalysisResult {
	result := responseObjectKeyAnalysisResult{}

	transform, hasTransform := expectedResponse.GetTransform()
	if !hasTransform {
		return result
	}

	tplAnalyzer := stream_transform.NewTemplateStaticAnalyzer(stream_transform.TemplateAnalysisContext{
		ProviderName: actx.Provider,
		ServiceName:  actx.Service,
		MethodName:   actx.Method,
		ResourceKey:  actx.Resource,
		TemplateType: transform.GetType(),
		TemplateBody: transform.GetBody(),
	})
	tplResult := tplAnalyzer.Analyze()

	result.errors = append(result.errors, tplResult.GetErrors()...)
	result.warnings = append(result.warnings, tplResult.GetWarnings()...)
	result.affirmatives = append(result.affirmatives, tplResult.GetAffirmatives()...)
	for _, tf := range tplResult.GetFindings() {
		result.findings = append(result.findings, AnalysisFinding{
			Level:    tf.Level,
			Bin:      tf.Bin,
			Provider: tf.Provider,
			Service:  tf.Service,
			Resource: tf.Resource,
			Method:   tf.Method,
			Message:  tf.Message,
		})
	}

	if (isEmptyObjectSchema || isLeafSchema) && objectKey != "" {
		f := actx.NewWarning(BinEmptyResponseUnsafe, fmt.Sprintf("transform applied with objectKey '%s' but response schema is %s — transform output may not route correctly", objectKey, describeSchemaShape(isLeafSchema, isEmptyObjectSchema)))
		result.warnings = append(result.warnings, f.String())
		result.findings = append(result.findings, f)
	}

	return result
}

func hasTransformOverride(expectedResponse anysdk.ExpectedResponse, hasExpectedResponse bool) bool {
	if !hasExpectedResponse {
		return false
	}
	_, hasTransform := expectedResponse.GetTransform()
	return hasTransform
}

func schemaHasProperties(schema anysdk.Schema) bool {
	props, err := schema.GetProperties()
	if err != nil {
		return false
	}
	return len(props) > 0
}

func describeSchemaShape(isLeaf bool, isEmpty bool) string {
	if isLeaf {
		return "a leaf/scalar type"
	}
	if isEmpty {
		return "an empty object (no properties)"
	}
	return "unknown shape"
}
