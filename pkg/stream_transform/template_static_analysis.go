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

func tagWarning(bin, msg string) string {
	return fmt.Sprintf("[%s] %s", bin, msg)
}

// bareDotArgPattern matches bare dot (`.`) passed as an argument to a template
// function, e.g. `{{ toJson . }}`, `{{ index . "key" }}`. The dot must be
// surrounded by whitespace or closing braces, distinguishing it from field
// access like `.Field`.
var bareDotArgPattern = regexp.MustCompile(`\{\{-?\s+\w+\s+\.\s+[^.}]|\{\{-?\s+\w+\s+\.\s*-?\}\}`)

// chainedIndexPattern matches `index <expr> "key1" "key2" ...` with 2+ string
// keys. In MXJ templates this is dangerous: any intermediate node can be a
// slice (multiple XML elements) instead of a map, and `index` panics when it
// tries to use a string key on a slice.
var chainedIndexPattern = regexp.MustCompile(`index\s+\S+\s+"[^"]+"\s+"[^"]+"`)


// TemplateStaticAnalyzer performs static analysis on Go templates
// used for stream transformation. It validates syntax, checks for
// empty-input resilience, and reports potential issues without
// executing the template.
type TemplateStaticAnalyzer interface {
	// Analyze performs the full static analysis suite and returns the result.
	Analyze() TemplateAnalysisResult
}

// TemplateAnalysisResult holds the outcomes of template static analysis.
type TemplateAnalysisResult interface {
	GetErrors() []error
	GetWarnings() []string
	GetAffirmatives() []string
}

// TemplateAnalysisContext provides the caller's context so that
// analysis messages can be attributed to a specific method/resource.
type TemplateAnalysisContext struct {
	MethodName   string
	ResourceKey  string
	TemplateType string
	TemplateBody string
}

// NewTemplateStaticAnalyzer creates a new analyzer for the given template context.
func NewTemplateStaticAnalyzer(ctx TemplateAnalysisContext) TemplateStaticAnalyzer {
	return &standardTemplateStaticAnalyzer{
		ctx: ctx,
	}
}

// --- implementation ---

type standardTemplateStaticAnalyzer struct {
	ctx TemplateAnalysisContext
}

type standardTemplateAnalysisResult struct {
	errors       []error
	warnings     []string
	affirmatives []string
}

func (r *standardTemplateAnalysisResult) GetErrors() []error {
	return r.errors
}

func (r *standardTemplateAnalysisResult) GetWarnings() []string {
	return r.warnings
}

func (r *standardTemplateAnalysisResult) GetAffirmatives() []string {
	return r.affirmatives
}

func (a *standardTemplateStaticAnalyzer) Analyze() TemplateAnalysisResult {
	result := &standardTemplateAnalysisResult{
		errors:       []error{},
		warnings:     []string{},
		affirmatives: []string{},
	}

	tplType := a.ctx.TemplateType
	tplBody := a.ctx.TemplateBody

	// Check: is this a recognised transformable type?
	factory := NewStreamTransformerFactory(tplType, tplBody)
	if !factory.IsTransformable() {
		result.errors = append(result.errors, fmt.Errorf(
			"method '%s' on resource '%s': response transform type '%s' is not a recognised transformable type",
			a.ctx.MethodName, a.ctx.ResourceKey, tplType))
		return result
	}

	// Check: does the template parse?
	funcMap := analysisFuncMap(tplType)
	_, parseErr := template.New("__static_analysis__").Funcs(funcMap).Parse(tplBody)
	if parseErr != nil {
		result.errors = append(result.errors, fmt.Errorf(
			"method '%s' on resource '%s': response transform template failed to parse: %v",
			a.ctx.MethodName, a.ctx.ResourceKey, parseErr))
		return result
	}

	result.affirmatives = append(result.affirmatives, fmt.Sprintf(
		"method '%s' on resource '%s': response transform template parses successfully (type='%s')",
		a.ctx.MethodName, a.ctx.ResourceKey, tplType))

	// Check: empty-input resilience
	a.analyzeEmptyResilience(result)

	// Check: format-specific concerns
	a.analyzeFormatSpecific(result)

	return result
}

// analyzeEmptyResilience detects patterns that will fail on nil/empty input.
func (a *standardTemplateStaticAnalyzer) analyzeEmptyResilience(result *standardTemplateAnalysisResult) {
	tplBody := a.ctx.TemplateBody

	// Direct field access: {{ .Field }}, {{.Field}}, {{ range .Items }}
	hasDirectDotAccess := containsAny(tplBody,
		"{{ .", "{{.", "{{ range .", "{{range .",
		"{{ range $", "{{range $",
	)

	// Bare dot passed as argument to a function: {{ toJson . }}, {{ index . "key" }}
	// These will panic or produce garbage on nil input.
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
		result.warnings = append(result.warnings, tagWarning(BinEmptyResponseUnsafe,
			fmt.Sprintf("method '%s' on resource '%s': response transform template accesses input directly without nil/empty guards — may fail on empty response bodies",
				a.ctx.MethodName, a.ctx.ResourceKey)))
	}
}

// analyzeFormatSpecific checks for issues particular to XML/JSON/text templates.
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
			result.warnings = append(result.warnings, tagWarning(BinEmptyResponseUnsafe,
				fmt.Sprintf("method '%s' on resource '%s': XML response transform uses XPath functions without nil guards — empty XML responses will cause errors",
					a.ctx.MethodName, a.ctx.ResourceKey)))
		}
		// MXJ slice/map ambiguity: `index $var "k1" "k2"` will panic if any
		// intermediate value is a slice (multiple XML elements) rather than a map.
		// A type check (printf "%T") on the RESULT doesn't help — the chained
		// index itself panics before the check runs.
		chainedMatches := chainedIndexPattern.FindAllString(tplBody, -1)
		for _, match := range chainedMatches {
			result.errors = append(result.errors, fmt.Errorf("[%s] method '%s' on resource '%s': MXJ response transform has chained index call '%s' — intermediate XML nodes may be slices (multiple elements) instead of maps, causing 'cannot index slice/array with type string' panics",
				BinResponseShapeUnsafe, a.ctx.MethodName, a.ctx.ResourceKey, match))
		}
	case templateFormatJSON:
		usesJSONMap := strings.Contains(tplBody, "jsonMapFromString")
		if usesJSONMap && !hasNilGuard {
			result.warnings = append(result.warnings, tagWarning(BinEmptyResponseUnsafe,
				fmt.Sprintf("method '%s' on resource '%s': JSON response transform uses jsonMapFromString without nil guards — empty JSON responses will cause errors",
					a.ctx.MethodName, a.ctx.ResourceKey)))
		}
	case templateFormatText:
		// Text templates are more forgiving; no specific checks yet.
	}
}

// Template format classification
const (
	templateFormatXML     = "xml"
	templateFormatJSON    = "json"
	templateFormatText    = "text"
	templateFormatUnknown = "unknown"
)

// classifyTemplateFormat determines the format class from the template type string.
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

// analysisFuncMap returns a FuncMap with stub implementations of all template
// functions, sufficient for parsing but not execution. The available functions
// are version-dependent, mirroring newTemplateStreamTransformer.
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
	// Version-gated functions: add them if the template version is >= v0.2.0
	tmplSemVerMatches := tmplSemVerRegexp.FindStringSubmatch(tplType)
	if len(tmplSemVerMatches) == 3 {
		tmplSemVer := tmplSemVerMatches[2]
		if semver.Compare(tmplSemVer, "v0.2.0") >= 0 {
			fm["toJson"] = func(v interface{}) (string, error) { return "", nil }
			fm["kindOf"] = func(x interface{}) string { return "" }
			fm["plus1"] = func(x int) int { return 0 }
		}
	} else {
		// If we can't parse the version, include all functions to avoid false parse errors
		fm["toJson"] = func(v interface{}) (string, error) { return "", nil }
		fm["kindOf"] = func(x interface{}) string { return "" }
		fm["plus1"] = func(x int) int { return 0 }
	}
	return fm
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
