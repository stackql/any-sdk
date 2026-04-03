package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteMockFiles writes individual Python mock files for each finding
// that has a mock_route and sample_response. Each file is a standalone
// Flask app that can be run directly.
func WriteMockFiles(findings []AnalysisFinding, outputDir string) error {
	if outputDir == "" {
		return nil
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create mock output dir: %w", err)
	}
	for _, f := range findings {
		if f.MockRoute == "" || f.SampleResponse == nil || f.SampleResponse.PreTransform == "" {
			continue
		}
		filename := mockFileName(f.Provider, f.Service, f.Resource, f.Method)
		path := filepath.Join(outputDir, filename)
		content := buildMockPythonFile(f)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write mock file %s: %w", path, err)
		}
	}
	return nil
}

func mockFileName(provider, service, resource, method string) string {
	base := fmt.Sprintf("mock_%s_%s_%s_%s", provider, service, resource, method)
	base = strings.ReplaceAll(base, ".", "_")
	base = strings.ReplaceAll(base, "-", "_")
	return strings.ToLower(base) + ".py"
}

// WriteExpectationFiles writes individual expected response files for each finding
// that has an expected_response. Each file is a plain text file containing the
// expected JSON output from `stackql exec -o json`.
func WriteExpectationFiles(findings []AnalysisFinding, outputDir string) error {
	if outputDir == "" {
		return nil
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create expectation output dir: %w", err)
	}
	for _, f := range findings {
		if f.ExpectedResponse == "" {
			continue
		}
		base := fmt.Sprintf("expect_%s_%s_%s_%s", f.Provider, f.Service, f.Resource, f.Method)
		base = strings.ReplaceAll(base, ".", "_")
		base = strings.ReplaceAll(base, "-", "_")
		filename := strings.ToLower(base) + ".txt"
		path := filepath.Join(outputDir, filename)
		if err := os.WriteFile(path, []byte(f.ExpectedResponse), 0o644); err != nil {
			return fmt.Errorf("failed to write expectation file %s: %w", path, err)
		}
	}
	return nil
}

func buildMockPythonFile(f AnalysisFinding) string {
	varName := f.SampleResponse.VarName
	if varName == "" {
		varName = MockResponseVarName(f.Provider, f.Service, f.Resource, f.Method)
	}

	// Escape the response body for Python triple-quoted string
	body := strings.ReplaceAll(f.SampleResponse.PreTransform, `\`, `\\`)
	body = strings.ReplaceAll(body, `"""`, `\"\"\"`)

	var sb strings.Builder
	sb.WriteString("from flask import Flask, request, Response\n\n")
	sb.WriteString("app = Flask(__name__)\n\n")
	sb.WriteString(fmt.Sprintf("%s = \"\"\"%s\"\"\"\n\n", varName, body))
	sb.WriteString(f.MockRoute)
	sb.WriteString("\n\n\nif __name__ == '__main__':\n    app.run(port=5000)\n")

	return sb.String()
}
