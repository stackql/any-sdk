package discovery

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/stackql/any-sdk/internal/anysdk"
)

var openAPIParamPattern = regexp.MustCompile(`\{([^}]+)\}`)

// GenerateMockRoute produces a Python Flask route handler string for a given method.
// The returned string is a complete route decorator + function that returns a stub response placeholder.
// resolveHTTPVerb returns the HTTP verb, falling back to parsing it from
// the method name prefix (e.g., "POST_DescribeVolumes" → "POST").
func resolveHTTPVerb(httpVerb string, methodName string) string {
	if httpVerb != "" {
		return strings.ToUpper(httpVerb)
	}
	// Method names like POST_DescribeVolumes, GET_List encode the verb as prefix
	if idx := strings.Index(methodName, "_"); idx > 0 {
		candidate := strings.ToUpper(methodName[:idx])
		switch candidate {
		case "GET", "POST", "PUT", "DELETE", "PATCH":
			return candidate
		}
	}
	return "GET"
}

func GenerateMockRoute(
	providerName string,
	serviceName string,
	resourceName string,
	methodName string,
	httpVerb string,
	operationName string,
	parameterizedPath string,
	responseMediaType string,
	requiredParams map[string]anysdk.Addressable,
) string {
	funcName := sanitizePythonName(fmt.Sprintf("%s_%s_%s_%s", providerName, serviceName, resourceName, methodName))
	httpVerb = resolveHTTPVerb(httpVerb, operationName)
	if responseMediaType == "" {
		responseMediaType = "application/json"
	}

	// Action query style: POST to root with Action discrimination
	if isActionQueryStyle(parameterizedPath) {
		action := deriveAction(operationName, parameterizedPath)
		return fmt.Sprintf(
			"@app.route('/', methods=['POST'])\n"+
				"def %s():\n"+
				"    body = request.get_data(as_text=True)\n"+
				"    if 'Action=%s' in body or request.form.get('Action') == '%s':\n"+
				"        return Response(MOCK_RESPONSE_%s, content_type='%s')\n"+
				"    return Response('Action not matched', status=404)",
			funcName, action, action, strings.ToUpper(funcName), responseMediaType)
	}

	// REST pattern: unique path with parameterized segments
	flaskPath := openAPIParamPattern.ReplaceAllString(parameterizedPath, "<$1>")
	if flaskPath == "" {
		flaskPath = "/"
	}
	return fmt.Sprintf(
		"@app.route('%s', methods=['%s'])\n"+
			"def %s():\n"+
			"    return Response(MOCK_RESPONSE_%s, content_type='%s')",
		flaskPath, httpVerb, funcName, strings.ToUpper(funcName), responseMediaType)
}

// GenerateStackQLQuery produces a StackQL SQL query that exercises the given method.
// It partitions parameters by location: server/path/query params go in WHERE,
// requestBody params go in column/value lists for INSERT or SET for UPDATE.
func GenerateStackQLQuery(
	providerName string,
	serviceName string,
	resourceName string,
	sqlVerb string,
	requiredParams map[string]anysdk.Addressable,
	requestBodyAttrs map[string]anysdk.Addressable,
) string {
	fqResource := fmt.Sprintf("%s.%s.%s", providerName, serviceName, resourceName)
	sqlVerb = strings.ToLower(sqlVerb)

	// WHERE params = required non-body params
	whereParams := make(map[string]anysdk.Addressable)
	for k, p := range requiredParams {
		loc := ""
		if p != nil {
			loc = p.GetLocation()
		}
		if loc != "requestBody" && loc != "body" {
			whereParams[k] = p
		}
	}
	// Body params = explicit request body attributes (preferred) or body-located required params
	bodyParams := make(map[string]anysdk.Addressable)
	if len(requestBodyAttrs) > 0 {
		for k, p := range requestBodyAttrs {
			bodyParams[k] = p
		}
	} else {
		for k, p := range requiredParams {
			if p != nil && (p.GetLocation() == "requestBody" || p.GetLocation() == "body") {
				bodyParams[k] = p
			}
		}
	}

	whereClause := buildWhereClause(whereParams)

	switch sqlVerb {
	case "select":
		if whereClause != "" {
			return fmt.Sprintf("SELECT * FROM %s WHERE %s", fqResource, whereClause)
		}
		return fmt.Sprintf("SELECT * FROM %s", fqResource)

	case "insert":
		// StackQL INSERT: all params (body + where) go as columns/values
		allInsertParams := make(map[string]anysdk.Addressable)
		for k, v := range bodyParams {
			allInsertParams[k] = v
		}
		for k, v := range whereParams {
			allInsertParams[k] = v
		}
		cols, vals := buildInsertColumnsAndValues(allInsertParams)
		return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", fqResource, cols, vals)

	case "delete":
		if whereClause != "" {
			return fmt.Sprintf("DELETE FROM %s WHERE %s", fqResource, whereClause)
		}
		return fmt.Sprintf("DELETE FROM %s", fqResource)

	case "update":
		setClause := buildSetClause(bodyParams)
		q := fmt.Sprintf("UPDATE %s SET %s", fqResource, setClause)
		if whereClause != "" {
			q += fmt.Sprintf(" WHERE %s", whereClause)
		}
		return q

	case "exec":
		if whereClause != "" {
			return fmt.Sprintf("EXEC %s WHERE %s", fqResource, whereClause)
		}
		return fmt.Sprintf("EXEC %s", fqResource)

	default:
		if whereClause != "" {
			return fmt.Sprintf("SELECT * FROM %s WHERE %s", fqResource, whereClause)
		}
		return fmt.Sprintf("SELECT * FROM %s", fqResource)
	}
}

func buildInsertColumnsAndValues(params map[string]anysdk.Addressable) (string, string) {
	if len(params) == 0 {
		return "dummy_col", "'dummy_val'"
	}
	keys := sortedKeys(params)
	cols := make([]string, 0, len(keys))
	vals := make([]string, 0, len(keys))
	for _, k := range keys {
		cols = append(cols, k)
		vals = append(vals, fmt.Sprintf("'%s'", dummyValue(params[k], k)))
	}
	return strings.Join(cols, ", "), strings.Join(vals, ", ")
}

func buildSetClause(params map[string]anysdk.Addressable) string {
	if len(params) == 0 {
		return "dummy_col = 'dummy_val'"
	}
	keys := sortedKeys(params)
	clauses := make([]string, 0, len(keys))
	for _, k := range keys {
		clauses = append(clauses, fmt.Sprintf("%s = '%s'", k, dummyValue(params[k], k)))
	}
	return strings.Join(clauses, ", ")
}

func sortedKeys(params map[string]anysdk.Addressable) []string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func buildWhereClause(params map[string]anysdk.Addressable) string {
	if len(params) == 0 {
		return ""
	}
	// Sort for deterministic output
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	clauses := make([]string, 0, len(keys))
	for _, k := range keys {
		p := params[k]
		clauses = append(clauses, fmt.Sprintf("%s = '%s'", k, dummyValue(p, k)))
	}
	return strings.Join(clauses, " AND ")
}

func dummyValue(p anysdk.Addressable, key string) (rv string) {
	defer func() {
		if r := recover(); r != nil {
			rv = "dummy_" + key
		}
	}()
	if p == nil {
		return "dummy_" + key
	}
	switch strings.ToLower(p.GetType()) {
	case "integer", "number":
		return "0"
	case "boolean":
		return "true"
	default:
		return "dummy_" + p.GetName()
	}
}

// isActionQueryStyle detects the query API pattern where the path has an Action=
// parameter (e.g., "/?Action=DescribeVolumes&Version=..."). These APIs use POST
// to a root path with Action discrimination in the form body at runtime,
// regardless of what the OpenAPI spec says about the HTTP method.
func isActionQueryStyle(path string) bool {
	return strings.Contains(path, "Action=")
}

// deriveAction extracts the AWS Action name from the operation name or parameterized path.
// Operation names like "GET_DescribeVolumes" → "DescribeVolumes".
// Paths like "/?Action=DescribeVolumes&Version=..." → "DescribeVolumes".
func deriveAction(operationName string, parameterizedPath string) string {
	// Try extracting from path query string: ?Action=Xyz or ?__Action=Xyz
	if idx := strings.Index(parameterizedPath, "Action="); idx >= 0 {
		action := parameterizedPath[idx+len("Action="):]
		if ampIdx := strings.Index(action, "&"); ampIdx >= 0 {
			action = action[:ampIdx]
		}
		if action != "" {
			return action
		}
	}
	// Fall back to operation name: strip HTTP verb prefix (e.g., "GET_DescribeVolumes" → "DescribeVolumes")
	if idx := strings.Index(operationName, "_"); idx >= 0 {
		candidate := operationName[idx+1:]
		if candidate != "" {
			return candidate
		}
	}
	return operationName
}

// GenerateExpectedResponse extracts the items array from the post-transform JSON
// using the selectItemsKey, and wraps it as a JSON array — matching `stackql exec -o json` output.
// Returns empty string if selectItemsKey is absent (expected response cannot be reliably predicted).
func GenerateExpectedResponse(postTransform string, selectItemsKey string) string {
	if postTransform == "" || selectItemsKey == "" {
		return ""
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(postTransform), &parsed); err != nil {
		return ""
	}

	// Navigate dot-separated or $. prefixed key path, e.g. "$.items" or "items"
	target := parsed
	keyPath := strings.TrimPrefix(selectItemsKey, "$.")
	keyPath = strings.TrimPrefix(keyPath, "$")
	if keyPath != "" {
		for _, seg := range strings.Split(keyPath, ".") {
			if seg == "" {
				continue
			}
			m, ok := target.(map[string]interface{})
			if !ok {
				return ""
			}
			next, exists := m[seg]
			if !exists {
				return ""
			}
			target = next
		}
	}

	// If target is already an array, marshal it directly
	if arr, ok := target.([]interface{}); ok {
		out, err := json.MarshalIndent(arr, "", "  ")
		if err != nil {
			return ""
		}
		return string(out)
	}
	// Single item — wrap as array
	out, err := json.MarshalIndent([]interface{}{target}, "", "  ")
	if err != nil {
		return ""
	}
	return string(out)
}

// MockResponseVarName returns the Python variable name for the mock response body constant.
func MockResponseVarName(providerName, serviceName, resourceName, methodName string) string {
	return "MOCK_RESPONSE_" + strings.ToUpper(sanitizePythonName(
		fmt.Sprintf("%s_%s_%s_%s", providerName, serviceName, resourceName, methodName)))
}

// InferMediaType determines the content type from the response body content.
// If the body starts with '<', it's XML; otherwise fall back to the provided default.
func InferMediaType(body string, fallback string) string {
	trimmed := strings.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '<' {
		return "application/xml"
	}
	if fallback != "" {
		return fallback
	}
	return "application/json"
}

func sanitizePythonName(s string) string {
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, "/", "_")
	return strings.ToLower(s)
}
