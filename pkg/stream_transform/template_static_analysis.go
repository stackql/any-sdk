package stream_transform

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"golang.org/x/mod/semver"
)

// Warning bin tags — must stay in sync with discovery.Bin* constants.
const (
	BinResponseShapeUnsafe = "response-shape-unsafe"
	BinEmptyResponseUnsafe = "empty-response-unsafe"
)

var bareDotArgPattern = regexp.MustCompile(`\{\{-?\s+\w+\s+\.\s+[^.}]|\{\{-?\s+\w+\s+\.\s*-?\}\}`)

var chainedIndexPattern = regexp.MustCompile(`index\s+\S+\s+"[^"]+"\s+"[^"]+"`)

// TemplateFinding is a structured finding from template static analysis.
type TemplateFinding struct {
	Level         string `json:"level"`
	Bin           string `json:"bin,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Service       string `json:"service,omitempty"`
	Resource      string `json:"resource,omitempty"`
	Method        string `json:"method,omitempty"`
	Message       string `json:"message"`
	FixedTemplate string `json:"fixed_template,omitempty"`
}

func (f TemplateFinding) Error() string {
	return fmt.Sprintf("[%s] %s", f.Bin, f.Message)
}

func (f TemplateFinding) String() string {
	return fmt.Sprintf("[%s] %s", f.Bin, f.Message)
}

// TemplateStaticAnalyzer performs static analysis on Go templates.
type TemplateStaticAnalyzer interface {
	Analyze() TemplateAnalysisResult
}

// TemplateAnalysisResult holds the outcomes of template static analysis.
type TemplateAnalysisResult interface {
	GetErrors() []error
	GetWarnings() []string
	GetAffirmatives() []string
	GetFindings() []TemplateFinding
}

// TemplateAnalysisContext provides the caller's context.
type TemplateAnalysisContext struct {
	ProviderName string
	ServiceName  string
	MethodName   string
	ResourceKey  string
	TemplateType string
	TemplateBody string
}

func NewTemplateStaticAnalyzer(ctx TemplateAnalysisContext) TemplateStaticAnalyzer {
	return &standardTemplateStaticAnalyzer{ctx: ctx}
}

// --- implementation ---

type standardTemplateStaticAnalyzer struct {
	ctx TemplateAnalysisContext
}

type standardTemplateAnalysisResult struct {
	errors       []error
	warnings     []string
	affirmatives []string
	findings     []TemplateFinding
}

func (r *standardTemplateAnalysisResult) GetErrors() []error       { return r.errors }
func (r *standardTemplateAnalysisResult) GetWarnings() []string    { return r.warnings }
func (r *standardTemplateAnalysisResult) GetAffirmatives() []string { return r.affirmatives }
func (r *standardTemplateAnalysisResult) GetFindings() []TemplateFinding { return r.findings }

func (a *standardTemplateStaticAnalyzer) newFinding(level, bin, message string) TemplateFinding {
	return TemplateFinding{
		Level:    level,
		Bin:      bin,
		Provider: a.ctx.ProviderName,
		Service:  a.ctx.ServiceName,
		Resource: a.ctx.ResourceKey,
		Method:   a.ctx.MethodName,
		Message:  message,
	}
}

func (a *standardTemplateStaticAnalyzer) Analyze() TemplateAnalysisResult {
	result := &standardTemplateAnalysisResult{}

	tplType := a.ctx.TemplateType
	tplBody := a.ctx.TemplateBody

	factory := NewStreamTransformerFactory(tplType, tplBody)
	if !factory.IsTransformable() {
		msg := fmt.Sprintf("response transform type '%s' is not a recognised transformable type", tplType)
		result.errors = append(result.errors, fmt.Errorf("%s", msg))
		result.findings = append(result.findings, a.newFinding("error", "", msg))
		return result
	}

	funcMap := analysisFuncMap(tplType)
	_, parseErr := template.New("__static_analysis__").Funcs(funcMap).Parse(tplBody)
	if parseErr != nil {
		msg := fmt.Sprintf("response transform template failed to parse: %v", parseErr)
		result.errors = append(result.errors, fmt.Errorf("%s", msg))
		result.findings = append(result.findings, a.newFinding("error", "", msg))
		return result
	}

	result.affirmatives = append(result.affirmatives, fmt.Sprintf(
		"response transform template parses successfully (type='%s')", tplType))

	a.analyzeEmptyResilience(result)
	a.analyzeFormatSpecific(result)

	// If any errors or warnings were found, attempt to produce a fixed template
	if len(result.errors) > 0 || len(result.warnings) > 0 {
		fixed := FixTemplate(tplBody, tplType)
		if fixed != "" {
			for i := range result.findings {
				result.findings[i].FixedTemplate = fixed
			}
		}
	}

	return result
}

func (a *standardTemplateStaticAnalyzer) analyzeEmptyResilience(result *standardTemplateAnalysisResult) {
	tplBody := a.ctx.TemplateBody

	hasDirectDotAccess := containsAny(tplBody,
		"{{ .", "{{.", "{{ range .", "{{range .",
		"{{ range $", "{{range $",
	)
	hasBareDotArg := bareDotArgPattern.MatchString(tplBody)
	hasNilGuard := containsAny(tplBody,
		"{{ if .", "{{if .",
		"{{ with .", "{{with .",
		"{{ if not .", "{{if not .",
		"{{ if . }}", "{{if . }}",
		"{{ if .}}", "{{if .}}",
		"{{ with . }}", "{{with . }}",
		"{{- with index .", "{{ with index .",
		"{{- if or (not ", "{{ if or (not ",
	)

	if (hasDirectDotAccess || hasBareDotArg) && !hasNilGuard {
		msg := "response transform template accesses input directly without nil/empty guards — may fail on empty response bodies"
		f := a.newFinding("warning", BinEmptyResponseUnsafe, msg)
		result.warnings = append(result.warnings, f.String())
		result.findings = append(result.findings, f)
	}
}

func (a *standardTemplateStaticAnalyzer) analyzeFormatSpecific(result *standardTemplateAnalysisResult) {
	tplBody := a.ctx.TemplateBody
	tplType := a.ctx.TemplateType

	hasNilGuard := containsAny(tplBody,
		"{{ if .", "{{if .",
		"{{ with .", "{{with .",
	)

	formatClass := classifyTemplateFormat(tplType)

	switch formatClass {
	case templateFormatXML:
		usesXPath := containsAny(tplBody, "getXPath", "getXPathAllOuter")
		if usesXPath && !hasNilGuard {
			msg := "XML response transform uses XPath functions without nil guards — empty XML responses will cause errors"
			f := a.newFinding("warning", BinEmptyResponseUnsafe, msg)
			result.warnings = append(result.warnings, f.String())
			result.findings = append(result.findings, f)
		}
		chainedMatches := chainedIndexPattern.FindAllString(tplBody, -1)
		for _, match := range chainedMatches {
			msg := fmt.Sprintf("MXJ response transform has chained index call '%s' — intermediate XML nodes may be slices (multiple elements) instead of maps, causing 'cannot index slice/array with type string' panics", match)
			f := a.newFinding("error", BinResponseShapeUnsafe, msg)
			result.errors = append(result.errors, f)
			result.findings = append(result.findings, f)
		}
	case templateFormatJSON:
		usesJSONMap := strings.Contains(tplBody, "jsonMapFromString")
		if usesJSONMap && !hasNilGuard {
			msg := "JSON response transform uses jsonMapFromString without nil guards — empty JSON responses will cause errors"
			f := a.newFinding("warning", BinEmptyResponseUnsafe, msg)
			result.warnings = append(result.warnings, f.String())
			result.findings = append(result.findings, f)
		}
	case templateFormatText:
		// Text templates are more forgiving; no specific checks yet.
	}
}

const (
	templateFormatXML     = "xml"
	templateFormatJSON    = "json"
	templateFormatText    = "text"
	templateFormatUnknown = "unknown"
)

func classifyTemplateFormat(tplType string) string {
	lower := strings.ToLower(tplType)
	if strings.Contains(lower, "mxj") || strings.Contains(lower, "xml") {
		return templateFormatXML
	}
	if strings.Contains(lower, "json") {
		return templateFormatJSON
	}
	if strings.Contains(lower, "text") {
		return templateFormatText
	}
	return templateFormatUnknown
}

func analysisFuncMap(tplType string) template.FuncMap {
	fm := template.FuncMap{
		"separator":           func(s string) func() string { return func() string { return "" } },
		"jsonMapFromString":   func(s string) (map[string]interface{}, error) { return nil, nil },
		"getXPath":            func(xml string, path string) (string, error) { return "", nil },
		"getXPathAllOuter":    func(xml string, path string) ([]string, error) { return nil, nil },
		"getRegexpFirstMatch": func(input string, pattern string) (string, error) { return "", nil },
		"getRegexpAllMatches": func(input string, pattern string) ([]string, error) { return nil, nil },
		"safeIndex":           func(m map[string]interface{}, key string) interface{} { return nil },
		"toBool":              func(v interface{}) bool { return false },
		"toInt":               func(v interface{}) int { return 0 },
	}
	tmplSemVerMatches := tmplSemVerRegexp.FindStringSubmatch(tplType)
	if len(tmplSemVerMatches) == 3 {
		tmplSemVer := tmplSemVerMatches[2]
		if semver.Compare(tmplSemVer, "v0.2.0") >= 0 {
			fm["toJson"] = func(v interface{}) (string, error) { return "", nil }
			fm["kindOf"] = func(x interface{}) string { return "" }
			fm["plus1"] = func(x int) int { return 0 }
		}
	} else {
		fm["toJson"] = func(v interface{}) (string, error) { return "", nil }
		fm["kindOf"] = func(x interface{}) string { return "" }
		fm["plus1"] = func(x int) int { return 0 }
	}
	return fm
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
