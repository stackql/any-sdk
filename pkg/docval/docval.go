package docval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

// ValidateAndParse is the only exported function.
// It accepts raw document bytes (YAML or JSON), a JSON Schema (bytes),
// and a docType tag (reserved for future behavior).
// It returns the parsed markup as map[string]any when the JSON Schema validates.
func ValidateAndParse(docBytes []byte, schemaBytes []byte, docType string) (map[string]any, error) {
	// 1) Parse the document (YAML first, then JSON)
	obj, err := parseDocument(docBytes)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// 2) Compile and run JSON Schema validation (in-memory)
	if err := validateAgainstSchema(obj, schemaBytes); err != nil {
		return nil, err
	}

	// 3) (Optional) docType-specific normalizations can be added later.
	_ = docType

	return obj, nil
}

// ValidateAndParseFile loads a document and its schema from the local filesystem,
// then delegates to ValidateAndParse().
//
// Example:
//
//	parsed, err := docval.ValidateAndParseFile("provider.yaml", "provider.schema.json", "provider")
func ValidateAndParseFile(docPath, schemaPath, docType string) (map[string]any, error) {
	docBytes, err := os.ReadFile(docPath)
	if err != nil {
		return nil, fmt.Errorf("read document %q: %w", docPath, err)
	}
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read schema %q: %w", schemaPath, err)
	}
	return ValidateAndParse(docBytes, schemaBytes, docType)
}

// parseDocument tries YAML first (common for registry), then JSON.
// It returns a top-level object (map[string]any); non-object roots are rejected.
func parseDocument(b []byte) (map[string]any, error) {
	// Try YAML
	var y map[string]any
	if err := yaml.Unmarshal(b, &y); err == nil && len(y) > 0 {
		return y, nil
	}

	// Fallback to JSON with UseNumber to avoid float precision surprises
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var j map[string]any
	if err := dec.Decode(&j); err == nil && len(j) > 0 {
		return j, nil
	}

	return nil, errors.New("document is neither valid YAML nor JSON object (top-level must be a mapping)")
}

func validateAgainstSchema(obj map[string]any, schemaBytes []byte) error {
	// Compile schema from in-memory bytes
	c := jsonschema.NewCompiler()
	const schemaURL = "inmemory://schema.json"
	if err := c.AddResource(schemaURL, bytesToReadCloser(schemaBytes)); err != nil {
		return fmt.Errorf("schema load error: %w", err)
	}
	schema, err := c.Compile(schemaURL)
	if err != nil {
		// Compilation errors are already aggregated by the library
		return fmt.Errorf("schema compile error: %w", err)
	}

	// Validate the already-parsed object
	if err := schema.Validate(obj); err != nil {
		// Pretty-print nested validation issues
		return fmt.Errorf("schema validation failed:\n%s", formatValidationError(err))
	}
	return nil
}

func bytesToReadCloser(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(b))
}

// formatValidationError flattens jsonschema.ValidationError with nested causes.
func formatValidationError(err error) string {
	var ve *jsonschema.ValidationError
	if errors.As(err, &ve) {
		var sb strings.Builder
		printVE(&sb, ve, "  ")
		return sb.String()
	}
	return "  - " + err.Error()
}

func printVE(sb *strings.Builder, e *jsonschema.ValidationError, prefix string) {
	loc := e.InstanceLocation
	if loc == "" {
		loc = "/"
	}
	sb.WriteString(fmt.Sprintf("%s- at %s: %s\n", prefix, loc, e.Message))
	for _, c := range e.Causes {
		printVE(sb, c, prefix+"  ")
	}
}
