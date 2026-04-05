package closure

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ClosureConfig configures the closure builder.
type ClosureConfig struct {
	ResourceName string
	RewriteURL   string // optional: rewrite all server URLs to this base
}

// BuildClosure produces the minimal service document YAML containing only
// the paths, schemas, parameters, and resource definitions needed for
// the specified resource.
func BuildClosure(serviceDocBytes []byte, cfg ClosureConfig) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(serviceDocBytes, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse service document: %w", err)
	}

	// If no resource specified, rewrite the entire service doc (all resources)
	if cfg.ResourceName == "" {
		if cfg.RewriteURL != "" {
			if servers, ok := doc["servers"]; ok {
				if serverSlice, ok := servers.([]interface{}); ok {
					doc["servers"] = RewriteServers(serverSlice, cfg.RewriteURL)
				}
			}
		}
		return yaml.Marshal(doc)
	}

	// Locate target resource
	resource, err := getResource(doc, cfg.ResourceName)
	if err != nil {
		return nil, err
	}

	// Phase 1: Collect operation $refs from methods → path keys
	pathKeys, methodHTTPMethods := collectPathKeysFromResource(resource)

	// Phase 2: Collect schema refs from methods (request/response schema_overrides)
	schemaKeys := collectSchemaKeysFromResource(resource)

	// Phase 3: Collect schema + parameter refs from the referenced path operations
	paramKeys := make(map[string]bool)
	paths := getMap(doc, "paths")
	for pathKey, httpMethod := range methodHTTPMethods {
		pathItem := getMap(paths, pathKey)
		if pathItem == nil {
			continue
		}
		operation := getMap(pathItem, httpMethod)
		if operation == nil {
			continue
		}
		// Schemas from responses
		for k := range collectSchemaKeys(operation) {
			schemaKeys[k] = true
		}
		// Parameters from path item and operation
		for k := range collectParameterKeys(pathItem) {
			paramKeys[k] = true
		}
		for k := range collectParameterKeys(operation) {
			paramKeys[k] = true
		}
	}

	// Phase 4: Transitively resolve schema refs
	schemas := getMap(getMap(doc, "components"), "schemas")
	if schemas != nil {
		resolveTransitiveSchemas(schemas, schemaKeys)
	}

	// Phase 5: Build closure document
	out := buildClosureDoc(doc, cfg, resource, pathKeys, schemaKeys, paramKeys)

	return yaml.Marshal(out)
}

func getResource(doc map[string]interface{}, name string) (map[string]interface{}, error) {
	components := getMap(doc, "components")
	if components == nil {
		return nil, fmt.Errorf("service document has no components")
	}
	resources := getMap(components, "x-stackQL-resources")
	if resources == nil {
		return nil, fmt.Errorf("service document has no x-stackQL-resources")
	}
	resource := getMap(resources, name)
	if resource == nil {
		return nil, fmt.Errorf("resource '%s' not found in x-stackQL-resources", name)
	}
	return resource, nil
}

func collectPathKeysFromResource(resource map[string]interface{}) (map[string]bool, map[string]string) {
	pathKeys := make(map[string]bool)
	methodHTTPMethods := make(map[string]string) // pathKey → httpMethod

	methods := getMap(resource, "methods")
	if methods == nil {
		return pathKeys, methodHTTPMethods
	}
	for _, methodDef := range methods {
		m, ok := methodDef.(map[string]interface{})
		if !ok {
			continue
		}
		op := getMap(m, "operation")
		if op == nil {
			continue
		}
		ref, ok := op["$ref"].(string)
		if !ok {
			continue
		}
		pathKey, httpMethod, ok := parseOperationRef(ref)
		if ok {
			pathKeys[pathKey] = true
			methodHTTPMethods[pathKey] = httpMethod
		}
	}
	return pathKeys, methodHTTPMethods
}

func collectSchemaKeysFromResource(resource map[string]interface{}) map[string]bool {
	keys := make(map[string]bool)
	methods := getMap(resource, "methods")
	if methods == nil {
		return keys
	}
	for _, methodDef := range methods {
		m, ok := methodDef.(map[string]interface{})
		if !ok {
			continue
		}
		// response.schema_override.$ref
		if resp := getMap(m, "response"); resp != nil {
			if so := getMap(resp, "schema_override"); so != nil {
				if ref, ok := so["$ref"].(string); ok {
					if key, ok := parseComponentRef(ref, "schemas"); ok {
						keys[key] = true
					}
				}
			}
		}
		// request.schema_override.$ref
		if req := getMap(m, "request"); req != nil {
			if so := getMap(req, "schema_override"); so != nil {
				if ref, ok := so["$ref"].(string); ok {
					if key, ok := parseComponentRef(ref, "schemas"); ok {
						keys[key] = true
					}
				}
			}
		}
	}
	return keys
}

// resolveTransitiveSchemas expands the schemaKeys set by walking each collected
// schema for nested $ref values, iterating until stable.
func resolveTransitiveSchemas(schemas map[string]interface{}, schemaKeys map[string]bool) {
	for {
		added := false
		for key := range schemaKeys {
			schema, ok := schemas[key]
			if !ok {
				continue
			}
			for nested := range collectSchemaKeys(schema) {
				if !schemaKeys[nested] {
					schemaKeys[nested] = true
					added = true
				}
			}
		}
		if !added {
			break
		}
	}
}

func buildClosureDoc(
	doc map[string]interface{},
	cfg ClosureConfig,
	resource map[string]interface{},
	pathKeys map[string]bool,
	schemaKeys map[string]bool,
	paramKeys map[string]bool,
) map[string]interface{} {
	out := make(map[string]interface{})

	// Copy top-level fields verbatim
	for _, key := range []string{"openapi", "info", "security", "externalDocs", "x-hasEquivalentPaths"} {
		if v, ok := doc[key]; ok {
			out[key] = v
		}
	}
	// Copy any remaining top-level x- keys
	for k, v := range doc {
		if strings.HasPrefix(k, "x-") {
			out[k] = v
		}
	}

	// Servers — with optional rewrite
	if servers, ok := doc["servers"]; ok {
		if serverSlice, ok := servers.([]interface{}); ok && cfg.RewriteURL != "" {
			out["servers"] = RewriteServers(serverSlice, cfg.RewriteURL)
		} else {
			out["servers"] = servers
		}
	}

	// Paths — only referenced paths
	srcPaths := getMap(doc, "paths")
	if srcPaths != nil {
		filteredPaths := make(map[string]interface{})
		for key := range pathKeys {
			if v, ok := srcPaths[key]; ok {
				filteredPaths[key] = v
			}
		}
		out["paths"] = filteredPaths
	}

	// Components
	srcComponents := getMap(doc, "components")
	if srcComponents == nil {
		return out
	}
	components := make(map[string]interface{})

	// x-stackQL-resources — only the target resource
	components["x-stackQL-resources"] = map[string]interface{}{
		cfg.ResourceName: resource,
	}

	// schemas — only referenced
	if srcSchemas := getMap(srcComponents, "schemas"); srcSchemas != nil {
		filtered := make(map[string]interface{})
		for key := range schemaKeys {
			if v, ok := srcSchemas[key]; ok {
				filtered[key] = v
			}
		}
		if len(filtered) > 0 {
			components["schemas"] = filtered
		}
	}

	// parameters — only referenced
	if srcParams := getMap(srcComponents, "parameters"); srcParams != nil {
		filtered := make(map[string]interface{})
		for key := range paramKeys {
			if v, ok := srcParams[key]; ok {
				filtered[key] = v
			}
		}
		if len(filtered) > 0 {
			components["parameters"] = filtered
		}
	}

	// securitySchemes — copy if present
	if ss, ok := srcComponents["securitySchemes"]; ok {
		components["securitySchemes"] = ss
	}

	out["components"] = components
	return out
}

// ListResources returns all resource names from the x-stackQL-resources section.
func ListResources(serviceDocBytes []byte) ([]string, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(serviceDocBytes, &doc); err != nil {
		return nil, err
	}
	components := getMap(doc, "components")
	if components == nil {
		return nil, nil
	}
	resources := getMap(components, "x-stackQL-resources")
	if resources == nil {
		return nil, nil
	}
	names := make([]string, 0, len(resources))
	for k := range resources {
		names = append(names, k)
	}
	return names, nil
}

// getMap safely navigates to a nested map key.
func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	result, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return result
}
