package closure

import (
	"strings"
)

// unescapeJSONPointer converts JSON Pointer escapes: ~1 → /, ~0 → ~
func unescapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}

// parseOperationRef parses an operation $ref like:
//
//	#/paths/~1?__Action=DescribeVolumes&__Version=2016-11-15/post
//
// Returns the path key (unescaped) and HTTP method.
func parseOperationRef(ref string) (pathKey string, httpMethod string, ok bool) {
	ref = strings.TrimPrefix(ref, "#/paths/")
	if ref == "" {
		return "", "", false
	}
	// The last segment after the final / is the HTTP method
	lastSlash := strings.LastIndex(ref, "/")
	if lastSlash < 0 {
		return "", "", false
	}
	pathEncoded := ref[:lastSlash]
	httpMethod = ref[lastSlash+1:]
	pathKey = unescapeJSONPointer(pathEncoded)
	return pathKey, strings.ToLower(httpMethod), true
}

// parseComponentRef extracts the key from a $ref like #/components/schemas/Foo.
// The component parameter is the component type (e.g., "schemas", "parameters").
func parseComponentRef(ref string, component string) (string, bool) {
	prefix := "#/components/" + component + "/"
	if !strings.HasPrefix(ref, prefix) {
		return "", false
	}
	return ref[len(prefix):], true
}

// collectAllRefs recursively walks a raw YAML node and returns all $ref string values.
func collectAllRefs(node interface{}) []string {
	var refs []string
	walkRefs(node, func(ref string) {
		refs = append(refs, ref)
	})
	return refs
}

func walkRefs(node interface{}, fn func(string)) {
	switch v := node.(type) {
	case map[string]interface{}:
		if ref, ok := v["$ref"]; ok {
			if s, ok := ref.(string); ok {
				fn(s)
			}
		}
		for _, val := range v {
			walkRefs(val, fn)
		}
	case []interface{}:
		for _, item := range v {
			walkRefs(item, fn)
		}
	}
}

// collectSchemaKeys extracts all #/components/schemas/X references from a node,
// returning just the schema key names.
func collectSchemaKeys(node interface{}) map[string]bool {
	keys := make(map[string]bool)
	for _, ref := range collectAllRefs(node) {
		if key, ok := parseComponentRef(ref, "schemas"); ok {
			keys[key] = true
		}
	}
	return keys
}

// collectParameterKeys extracts all #/components/parameters/X references from a node.
func collectParameterKeys(node interface{}) map[string]bool {
	keys := make(map[string]bool)
	for _, ref := range collectAllRefs(node) {
		if key, ok := parseComponentRef(ref, "parameters"); ok {
			keys[key] = true
		}
	}
	return keys
}
