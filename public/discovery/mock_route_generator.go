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
	requiredParams map[string]anysdk.Addressable,
) string {
	funcName := sanitizePythonName(fmt.Sprintf("%s_%s_%s_%s", providerName, serviceName, resourceName, methodName))
	httpVerb = resolveHTTPVerb(httpVerb, operationName)

	// AWS pattern: POST to root with Action discrimination
	if isAWSStyle(providerName, parameterizedPath) {
		action := deriveAction(operationName, parameterizedPath)
		return fmt.Sprintf(
			"@app.route('/', methods=['POST'])\n"+
				"def %s():\n"+
				"    if request.form.get('Action') == '%s':\n"+
				"        return Response(MOCK_RESPONSE_%s, content_type='application/xml')\n"+
				"    return Response('Action not matched', status=404)",
			funcName, action, strings.ToUpper(funcName))
	}

	// REST pattern: unique path with parameterized segments
	flaskPath := openAPIParamPattern.ReplaceAllString(parameterizedPath, "<$1>")
	if flaskPath == "" {
		flaskPath = "/"
	}
	return fmt.Sprintf(
		"@app.route('%s', methods=['%s'])\n"+
			"def %s():\n"+
			"    return Response(MOCK_RESPONSE_%s, content_type='application/json')",
		flaskPath, httpVerb, funcName, strings.ToUpper(funcName))
}

// GenerateStackQLQuery produces a StackQL SQL query that exercises the given method.
func GenerateStackQLQuery(
	providerName string,
	serviceName string,
	resourceName string,
	sqlVerb string,
	requiredParams map[string]anysdk.Addressable,
) string {
	fqResource := fmt.Sprintf("%s.%s.%s", providerName, serviceName, resourceName)
	sqlVerb = strings.ToLower(sqlVerb)

	var prefix string
	switch sqlVerb {
	case "select":
		prefix = "SELECT * FROM"
	case "insert":
		prefix = "INSERT INTO"
	case "delete":
		prefix = "DELETE FROM"
	case "exec":
		prefix = "EXEC"
	default:
		prefix = "SELECT * FROM"
	}

	whereClause := buildWhereClause(requiredParams)
	if whereClause != "" {
		return fmt.Sprintf("%s %s WHERE %s", prefix, fqResource, whereClause)
	}
	return fmt.Sprintf("%s %s", prefix, fqResource)
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

// isAWSStyle detects the AWS query API pattern. AWS EC2-style services use POST
// to root with Action discrimination, regardless of what the OpenAPI spec says
// about the HTTP method (specs often say GET but runtime uses POST).
func isAWSStyle(providerName string, path string) bool {
	if !strings.HasPrefix(providerName, "aws") {
		return false
	}
	// Root path or query-string-only path (e.g., "/?Action=..." or "/")
	cleanPath := strings.SplitN(path, "?", 2)[0]
	return cleanPath == "/" || cleanPath == ""
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
// If selectItemsKey is empty or navigation fails, wraps the entire response as a single-element array.
func GenerateExpectedResponse(postTransform string, selectItemsKey string) string {
	if postTransform == "" {
		return "[]"
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(postTransform), &parsed); err != nil {
		return "[]"
	}

	target := parsed
	if selectItemsKey != "" {
		// Navigate dot-separated or $. prefixed key path, e.g. "$.items" or "items"
		keyPath := strings.TrimPrefix(selectItemsKey, "$.")
		keyPath = strings.TrimPrefix(keyPath, "$")
		if keyPath != "" {
			segments := strings.Split(keyPath, ".")
			for _, seg := range segments {
				if seg == "" {
					continue
				}
				m, ok := target.(map[string]interface{})
				if !ok {
					break
				}
				next, exists := m[seg]
				if !exists {
					break
				}
				target = next
			}
		}
	}

	// If target is already an array, marshal it directly
	if arr, ok := target.([]interface{}); ok {
		out, err := json.MarshalIndent(arr, "", "  ")
		if err != nil {
			return "[]"
		}
		return string(out)
	}
	// Otherwise wrap as single-element array
	out, err := json.MarshalIndent([]interface{}{target}, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(out)
}

// MockResponseVarName returns the Python variable name for the mock response body constant.
func MockResponseVarName(providerName, serviceName, resourceName, methodName string) string {
	return "MOCK_RESPONSE_" + strings.ToUpper(sanitizePythonName(
		fmt.Sprintf("%s_%s_%s_%s", providerName, serviceName, resourceName, methodName)))
}

func sanitizePythonName(s string) string {
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, "/", "_")
	return strings.ToLower(s)
}
