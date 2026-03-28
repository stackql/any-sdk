package discovery

import (
	"fmt"
	"strings"

	"github.com/stackql/any-sdk/internal/anysdk"
	"github.com/stackql/any-sdk/pkg/media"
	"github.com/stackql/any-sdk/pkg/openapitopath"
	"github.com/stackql/any-sdk/pkg/stream_transform"
)

// responseObjectKeyAnalysisResult holds the findings from analyzing
// a method's response schema and objectKey routability together.
// Pain points 1 (empty response handling) and 2 (objectKey routability)
// are interwoven: an objectKey may be syntactically valid but unresolvable
// when the response body is empty or the schema is nil/leaf.
type responseObjectKeyAnalysisResult struct {
	errors       []error
	warnings     []string
	affirmatives []string
}

// analyzeResponseAndObjectKey performs combined analysis of response emptiness
// and objectKey routability for a single method.
func analyzeResponseAndObjectKey(
	method anysdk.StandardOperationStore,
	methodName string,
	resourceKey string,
) responseObjectKeyAnalysisResult {
	result := responseObjectKeyAnalysisResult{
		errors:       []error{},
		warnings:     []string{},
		affirmatives: []string{},
	}

	expectedResponse, hasExpectedResponse := method.GetResponse()
	objectKey := method.GetSelectItemsKeySimple()
	hasObjectKey := objectKey != ""

	// Phase 1: Probe the response schema existence and emptiness
	responseSchema, responseMediaType, responseSchemaErr := method.GetResponseBodySchemaAndMediaType()

	if responseSchemaErr != nil {
		if hasObjectKey {
			result.errors = append(result.errors, fmt.Errorf(
				"method '%s' on resource '%s': objectKey '%s' specified but no response schema could be resolved: %v",
				methodName, resourceKey, objectKey, responseSchemaErr))
		}
		if method.ShouldBeSelectable() && !hasTransformOverride(expectedResponse, hasExpectedResponse) {
			result.warnings = append(result.warnings, classifiedWarning(BinEmptyResponseUnsafe,
				"method '%s' on resource '%s': selectable method has no response schema and no transform override",
				methodName, resourceKey))
		}
		return result
	}

	if responseSchema == nil {
		if hasObjectKey {
			result.errors = append(result.errors, fmt.Errorf(
				"method '%s' on resource '%s': objectKey '%s' specified but response schema is nil",
				methodName, resourceKey, objectKey))
		}
		return result
	}

	// Phase 2: Check if the response schema is empty (no properties, not an array)
	schemaType := responseSchema.GetType()
	isEmptyObjectSchema := schemaType == "object" && !schemaHasProperties(responseSchema)
	isLeafSchema := schemaType == "string" || schemaType == "integer" || schemaType == "boolean" || schemaType == "number"

	if isEmptyObjectSchema && hasObjectKey {
		result.warnings = append(result.warnings, classifiedWarning(BinEmptyResponseUnsafe,
			"method '%s' on resource '%s': objectKey '%s' navigates into an empty object schema (no properties defined)",
			methodName, resourceKey, objectKey))
	}

	// Phase 3: objectKey routability — validate the path resolves in the schema
	if hasObjectKey {
		routabilityResult := analyzeObjectKeyRoutability(
			method, responseSchema, objectKey, responseMediaType,
			methodName, resourceKey,
		)
		result.errors = append(result.errors, routabilityResult.errors...)
		result.warnings = append(result.warnings, routabilityResult.warnings...)
		result.affirmatives = append(result.affirmatives, routabilityResult.affirmatives...)
	}

	// Phase 4: If there's a response transform, analyze it for empty-input resilience
	if hasExpectedResponse {
		transformResult := analyzeResponseTransform(
			expectedResponse, objectKey,
			methodName, resourceKey, isLeafSchema, isEmptyObjectSchema,
		)
		result.errors = append(result.errors, transformResult.errors...)
		result.warnings = append(result.warnings, transformResult.warnings...)
		result.affirmatives = append(result.affirmatives, transformResult.affirmatives...)
	}

	return result
}

// analyzeObjectKeyRoutability validates that the objectKey path can be walked
// through the response schema. It also checks path language vs media type consistency.
func analyzeObjectKeyRoutability(
	method anysdk.StandardOperationStore,
	responseSchema anysdk.Schema,
	objectKey string,
	responseMediaType string,
	methodName string,
	resourceKey string,
) responseObjectKeyAnalysisResult {
	result := responseObjectKeyAnalysisResult{
		errors:       []error{},
		warnings:     []string{},
		affirmatives: []string{},
	}

	// Detect the path language used in objectKey
	pathLang := detectPathLanguage(objectKey)
	expectedMediaType := inferExpectedMediaType(responseMediaType)

	// Check path language vs media type consistency
	if pathLang != pathLangBare {
		if pathLang == pathLangJSONPath && expectedMediaType == media.MediaTypeXML {
			result.warnings = append(result.warnings, classifiedWarning(BinMediaTypeMismatch,
				"method '%s' on resource '%s': objectKey '%s' uses JSONPath syntax but response media type is '%s'",
				methodName, resourceKey, objectKey, responseMediaType))
		}
		if pathLang == pathLangXPath && expectedMediaType == media.MediaTypeJson {
			result.warnings = append(result.warnings, classifiedWarning(BinMediaTypeMismatch,
				"method '%s' on resource '%s': objectKey '%s' uses XPath syntax but response media type is '%s'",
				methodName, resourceKey, objectKey, responseMediaType))
		}
	}

	// Validate path segments resolve in the schema tree
	pathSegments := resolvePathSegments(objectKey, pathLang)
	walkResult := walkSchemaPath(responseSchema, pathSegments, pathLang, objectKey, methodName, resourceKey)
	result.errors = append(result.errors, walkResult.errors...)
	result.warnings = append(result.warnings, walkResult.warnings...)

	// Also try the existing resolution mechanism as a cross-check
	_, _, selectErr := method.GetSelectSchemaAndObjectPath()
	if selectErr != nil {
		result.errors = append(result.errors, fmt.Errorf(
			"method '%s' on resource '%s': objectKey '%s' failed schema resolution: %v",
			methodName, resourceKey, objectKey, selectErr))
	} else {
		result.affirmatives = append(result.affirmatives, fmt.Sprintf(
			"method '%s' on resource '%s': objectKey '%s' successfully routed through response schema",
			methodName, resourceKey, objectKey))
	}

	return result
}

// Path language constants
const (
	pathLangBare     = "bare"     // simple property name, e.g. "rows"
	pathLangJSONPath = "jsonpath" // e.g. "$.items[*]"
	pathLangXPath    = "xpath"    // e.g. "/*/Buckets/*"
)

// detectPathLanguage determines which path language an objectKey uses.
func detectPathLanguage(objectKey string) string {
	if strings.HasPrefix(objectKey, "$") {
		return pathLangJSONPath
	}
	if strings.HasPrefix(objectKey, "/") {
		return pathLangXPath
	}
	return pathLangBare
}

// inferExpectedMediaType normalises the response media type to a canonical form
// for comparison with path language.
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

// resolvePathSegments splits the objectKey into schema-walkable segments
// according to its path language.
func resolvePathSegments(objectKey string, pathLang string) []string {
	switch pathLang {
	case pathLangJSONPath:
		resolver := openapitopath.NewJSONPathResolver()
		return resolver.ToPathSlice(objectKey)
	case pathLangXPath:
		resolver := openapitopath.NewXPathResolver()
		return resolver.ToPathSlice(objectKey)
	default:
		// Bare property name — single segment
		return []string{objectKey}
	}
}

// walkSchemaPath attempts to walk the schema tree along the given path segments,
// reporting warnings for each segment that cannot be resolved.
func walkSchemaPath(
	schema anysdk.Schema,
	segments []string,
	pathLang string,
	objectKey string,
	methodName string,
	resourceKey string,
) responseObjectKeyAnalysisResult {
	result := responseObjectKeyAnalysisResult{
		errors:   []error{},
		warnings: []string{},
	}

	current := schema
	for i, segment := range segments {
		if current == nil {
			result.warnings = append(result.warnings, classifiedWarning(BinObjectKeyUnroutable,
				"method '%s' on resource '%s': objectKey '%s' path walk terminated at nil schema at segment [%d] '%s'",
				methodName, resourceKey, objectKey, i, segment))
			return result
		}

		if segment == "*" || segment == "[*]" {
			// In XPath, a leading wildcard means "any element name" (the XML root
			// wrapper element). The response schema already represents the content
			// inside that wrapper, so we skip the segment without descending into
			// array items.
			if pathLang == pathLangXPath && i == 0 {
				continue
			}
			// Array dereference — check the schema is actually an array or has items
			items, itemsErr := current.GetItems()
			if itemsErr != nil {
				additionalProps, hasAdditionalProps := current.GetAdditionalProperties()
				if !hasAdditionalProps {
					result.warnings = append(result.warnings, classifiedWarning(BinObjectKeyUnroutable,
						"method '%s' on resource '%s': objectKey '%s' uses wildcard '%s' at segment [%d] but schema is not an array/map (type='%s')",
						methodName, resourceKey, objectKey, segment, i, current.GetType()))
					return result
				}
				current = additionalProps
			} else {
				current = items
			}
			continue
		}

		// Named property lookup
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
			result.warnings = append(result.warnings, classifiedWarning(BinObjectKeyUnroutable,
				"method '%s' on resource '%s': objectKey '%s' segment [%d] '%s' not found in schema properties (type='%s')",
				methodName, resourceKey, objectKey, i, segment, current.GetType()))
			return result
		}
		current = prop
	}

	return result
}

// analyzeResponseTransform delegates template analysis to the stream_transform
// package's TemplateStaticAnalyzer, then adds response-schema-aware context.
func analyzeResponseTransform(
	expectedResponse anysdk.ExpectedResponse,
	objectKey string,
	methodName string,
	resourceKey string,
	isLeafSchema bool,
	isEmptyObjectSchema bool,
) responseObjectKeyAnalysisResult {
	result := responseObjectKeyAnalysisResult{
		errors:       []error{},
		warnings:     []string{},
		affirmatives: []string{},
	}

	transform, hasTransform := expectedResponse.GetTransform()
	if !hasTransform {
		return result
	}

	// Delegate to the modularised template analyzer
	tplAnalyzer := stream_transform.NewTemplateStaticAnalyzer(stream_transform.TemplateAnalysisContext{
		MethodName:   methodName,
		ResourceKey:  resourceKey,
		TemplateType: transform.GetType(),
		TemplateBody: transform.GetBody(),
	})
	tplResult := tplAnalyzer.Analyze()

	result.errors = append(result.errors, tplResult.GetErrors()...)
	result.warnings = append(result.warnings, tplResult.GetWarnings()...)
	result.affirmatives = append(result.affirmatives, tplResult.GetAffirmatives()...)

	// Additional schema-aware check: transform + objectKey on empty/leaf schemas
	if (isEmptyObjectSchema || isLeafSchema) && objectKey != "" {
		result.warnings = append(result.warnings, classifiedWarning(BinEmptyResponseUnsafe,
			"method '%s' on resource '%s': transform applied with objectKey '%s' but response schema is %s — transform output may not route correctly",
			methodName, resourceKey, objectKey, describeSchemaShape(isLeafSchema, isEmptyObjectSchema)))
	}

	return result
}

// hasTransformOverride checks if a response has a transform that could
// substitute for a missing schema.
func hasTransformOverride(expectedResponse anysdk.ExpectedResponse, hasExpectedResponse bool) bool {
	if !hasExpectedResponse {
		return false
	}
	_, hasTransform := expectedResponse.GetTransform()
	return hasTransform
}

// schemaHasProperties checks whether a schema has any defined properties.
func schemaHasProperties(schema anysdk.Schema) bool {
	props, err := schema.GetProperties()
	if err != nil {
		return false
	}
	return len(props) > 0
}

// describeSchemaShape returns a human-readable description of the schema shape.
func describeSchemaShape(isLeaf bool, isEmpty bool) string {
	if isLeaf {
		return "a leaf/scalar type"
	}
	if isEmpty {
		return "an empty object (no properties)"
	}
	return "unknown shape"
}
