package stream_transform

import (
	"fmt"
	"strings"
)

// EmpiricalTestResult holds the outcome of running a template against test inputs.
type EmpiricalTestResult struct {
	Input  string `json:"input"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
	OK     bool   `json:"ok"`
}

// EmpiricalTestSuite holds all empirical test results for a template.
type EmpiricalTestSuite struct {
	Results []EmpiricalTestResult `json:"results"`
}

func (s *EmpiricalTestSuite) HasFailures() bool {
	for _, r := range s.Results {
		if !r.OK {
			return true
		}
	}
	return false
}

func (s *EmpiricalTestSuite) FailureMessages() []string {
	var msgs []string
	for _, r := range s.Results {
		if !r.OK {
			msgs = append(msgs, fmt.Sprintf("input=%q: %s", r.Input, r.Error))
		}
	}
	return msgs
}

// RunEmpiricalTests executes a response transform template against a suite of
// test inputs including empty string, and returns structured results.
func RunEmpiricalTests(templateBody string, templateType string) EmpiricalTestSuite {
	suite := EmpiricalTestSuite{}

	inputs := testInputsForType(templateType)
	for _, input := range inputs {
		result := runSingleTest(templateBody, templateType, input)
		suite.Results = append(suite.Results, result)
	}
	return suite
}

// RunEmpiricalTestWithInput executes a template against a specific input.
func RunEmpiricalTestWithInput(templateBody string, templateType string, input string) EmpiricalTestResult {
	return runSingleTest(templateBody, templateType, input)
}

func runSingleTest(templateBody string, templateType string, input string) EmpiricalTestResult {
	factory := NewStreamTransformerFactory(templateType, templateBody)
	if !factory.IsTransformable() {
		return EmpiricalTestResult{
			Input: input,
			Error: fmt.Sprintf("template type '%s' is not transformable", templateType),
		}
	}

	tfm, err := factory.GetTransformer(input)
	if err != nil {
		return EmpiricalTestResult{
			Input: input,
			Error: fmt.Sprintf("failed to create transformer: %v", err),
		}
	}

	tfmErr := tfm.Transform()
	if tfmErr != nil {
		return EmpiricalTestResult{
			Input: input,
			Error: fmt.Sprintf("%v", tfmErr),
		}
	}

	var outBuf strings.Builder
	outStream := tfm.GetOutStream()
	if outStream != nil {
		buf := make([]byte, 4096)
		for {
			n, readErr := outStream.Read(buf)
			if n > 0 {
				outBuf.Write(buf[:n])
			}
			if readErr != nil {
				break
			}
		}
	}

	return EmpiricalTestResult{
		Input:  input,
		Output: outBuf.String(),
		OK:     true,
	}
}

// testInputsForType returns test inputs appropriate for the template format.
// Always includes empty string as the critical edge case.
func testInputsForType(tplType string) []string {
	formatClass := classifyTemplateFormat(tplType)
	switch formatClass {
	case templateFormatXML:
		return []string{
			"",
			"<root/>",
			"<root></root>",
		}
	case templateFormatJSON:
		return []string{
			"",
			"{}",
			"null",
		}
	case templateFormatText:
		return []string{
			"",
		}
	default:
		return []string{""}
	}
}
