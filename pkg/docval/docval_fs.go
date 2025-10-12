package docval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type FileValidator interface {
	ValidateAndParseFile(docPath string, schemaPath string) (map[string]any, error)
}

type fileValidator struct {
	rootSchemaDir string
}

func NewFileValidator(rootSchemaDir string) FileValidator {
	return &fileValidator{
		rootSchemaDir: rootSchemaDir,
	}
}

// ValidateAndParseFile loads a document and its schema from the local filesystem,
// rewrites external $ref values to absolute file:// URLs, and delegates to ValidateAndParse().
//
// Example:
//
//	parsed, err := docval.ValidateAndParseFile("fragment.yaml", "fragmented-resources.schema.json", "fragment")
func (v *fileValidator) ValidateAndParseFile(docPath string, schemaPath string) (map[string]any, error) {
	docType := "" // reserved for future use
	docBytes, err := os.ReadFile(docPath)
	if err != nil {
		return nil, fmt.Errorf("read document %q: %w", docPath, err)
	}

	rewritten, err := v.rewriteSchemaRefsToFileURLs(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("prepare schema %q: %w", schemaPath, err)
	}

	return ValidateAndParse(docBytes, rewritten, docType)
}

// --- helpers ---

// rewriteSchemaRefsToFileURLs loads a JSON schema, finds all objects with a "$ref" string,
// and for refs that are *external* (not starting with "#", not http/https, not file://),
// rewrites them into absolute file:// URLs using schemaPath as the base.
func (v *fileValidator) rewriteSchemaRefsToFileURLs(schemaPath string) ([]byte, error) {
	absSchema, err := filepath.Abs(path.Join(v.rootSchemaDir, schemaPath))
	if err != nil {
		return nil, err
	}

	rootPath, err := filepath.Abs(v.rootSchemaDir)
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if err := readJSONSchema(absSchema, &m); err != nil {
		return nil, err
	}

	// walk and rewrite
	rewriteRefs(m, rootPath)

	// marshal back to JSON

	// re-encode pretty
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func readJSONSchema(path string, out *map[string]any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("schema must be JSON (%s): %w", path, err)
	}
	return nil
}

func rewriteRefs(v any, baseDir string) {
	switch t := v.(type) {
	case map[string]any:
		if raw, ok := t["$ref"]; ok {
			if s, ok := raw.(string); ok && s != "" {
				// keep internal and already-URL refs as-is
				if !(strings.HasPrefix(s, "#") ||
					strings.HasPrefix(s, "http://") ||
					strings.HasPrefix(s, "https://") ||
					strings.HasPrefix(s, "file://")) {

					// split path + fragment
					path, frag := splitRef(s)
					if path != "" {
						abs := filepath.Join(baseDir, filepath.FromSlash(path))
						abs = filepath.Clean(abs)
						// Construct file:// URL with platform-independent separators
						url := "file://" + filepath.ToSlash(abs)
						if frag != "" {
							url += "#" + frag
						}
						t["$ref"] = url
					}
				}
			}
		}
		// walk children
		for k, child := range t {
			rewriteRefs(child, baseDir)
			t[k] = child
		}
	case []any:
		for i, child := range t {
			rewriteRefs(child, baseDir)
			t[i] = child
		}
	}
}

func splitRef(ref string) (path string, frag string) {
	if i := strings.IndexByte(ref, '#'); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return ref, ""
}
