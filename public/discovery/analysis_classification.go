package discovery

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Warning classification bins for static analysis.
const (
	BinResponseShapeUnsafe = "response-shape-unsafe"
	BinObjectKeyUnroutable = "objectKey-unroutable"
	BinEmptyResponseUnsafe = "empty-response-unsafe"
	BinMediaTypeMismatch   = "media-type-mismatch"
	BinMissingSemantics    = "missing-semantics"
)

// classifiedWarning prefixes a warning message with its bin tag (legacy string format).
func classifiedWarning(bin string, format string, args ...interface{}) string {
	return fmt.Sprintf("[%s] %s", bin, fmt.Sprintf(format, args...))
}

// ScoreMetrics provides aggregate pass rates and health scores.
type ScoreMetrics struct {
	TotalMethods       int     `json:"total_methods"`
	MethodsWithMocks   int     `json:"methods_with_mocks"`
	MethodsWithTransforms int  `json:"methods_with_transforms"`
	MethodsClean       int     `json:"methods_clean"`
	ErrorRate          float64 `json:"error_rate"`
	WarningRate        float64 `json:"warning_rate"`
	CleanRate          float64 `json:"clean_rate"`
	MockCoverage       float64 `json:"mock_coverage"`
}

// AnalysisSummary is the JSON-serialisable top-level output of static analysis.
type AnalysisSummary struct {
	TotalOK       int                    `json:"total_ok"`
	TotalWarnings int                    `json:"total_warnings"`
	TotalErrors   int                    `json:"total_errors"`
	Scores        *ScoreMetrics          `json:"scores,omitempty"`
	Bins          map[string]AnalysisBin `json:"bins"`
	Services      map[string]ServiceSummary `json:"services"`
	Errors        []string               `json:"errors,omitempty"`
}

// AnalysisBin holds the items for a single classification bin.
type AnalysisBin struct {
	Count    int      `json:"count"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ServiceSummary aggregates error and warning counts per service.
type ServiceSummary struct {
	ErrorCount   int `json:"error_count"`
	WarningCount int `json:"warning_count"`
}

// FormatSummaryJSON returns a JSON summary from structured findings.
func FormatSummaryJSON(legacyErrors []error, legacyWarnings []string, affirmatives []string, findings []AnalysisFinding) string {
	summary := AnalysisSummary{
		TotalOK:  len(affirmatives),
		Bins:     make(map[string]AnalysisBin),
		Services: make(map[string]ServiceSummary),
	}

	// Classify findings into bins and services
	for _, f := range findings {
		bin := f.Bin
		if bin == "" {
			bin = "other"
		}
		ab := summary.Bins[bin]
		ab.Count++
		resourceRef := f.Resource
		if f.Service != "" {
			resourceRef = f.Service + "." + f.Resource
		}
		if f.Level == "error" {
			ab.Errors = append(ab.Errors, resourceRef)
			summary.TotalErrors++
		} else {
			ab.Warnings = append(ab.Warnings, resourceRef)
			summary.TotalWarnings++
		}
		summary.Bins[bin] = ab

		if f.Service != "" {
			ss := summary.Services[f.Service]
			if f.Level == "error" {
				ss.ErrorCount++
			} else {
				ss.WarningCount++
			}
			summary.Services[f.Service] = ss
		}
	}

	// Compute score metrics from findings
	methodSet := make(map[string]bool)         // all methods seen
	methodHasMock := make(map[string]bool)     // methods with sample_response
	methodHasTransform := make(map[string]bool) // methods with prior_template (has transform)
	methodHasIssue := make(map[string]bool)    // methods with errors or warnings
	for _, f := range findings {
		mk := f.Provider + "." + f.Service + "." + f.Resource + "." + f.Method
		methodSet[mk] = true
		if f.SampleResponse != nil {
			methodHasMock[mk] = true
		}
		if f.PriorTemplate != "" {
			methodHasTransform[mk] = true
		}
		methodHasIssue[mk] = true
	}
	// Methods from affirmatives that had no findings are clean
	totalMethods := len(methodSet)
	if totalMethods == 0 {
		totalMethods = summary.TotalOK // fallback to affirmative count
	}
	methodsClean := 0
	for mk := range methodHasMock {
		if !methodHasIssue[mk] {
			methodsClean++
		}
	}
	// For methods that only appear in affirmatives (no findings), count as clean
	cleanFromAffirmatives := summary.TotalOK
	if len(methodSet) > 0 {
		cleanFromAffirmatives = 0
	}
	totalForRate := totalMethods
	if totalForRate == 0 {
		totalForRate = 1
	}
	scores := &ScoreMetrics{
		TotalMethods:        totalMethods,
		MethodsWithMocks:    len(methodHasMock),
		MethodsWithTransforms: len(methodHasTransform),
		MethodsClean:        methodsClean + cleanFromAffirmatives,
		ErrorRate:           float64(summary.TotalErrors) / float64(totalForRate),
		WarningRate:         float64(summary.TotalWarnings) / float64(totalForRate),
		MockCoverage:        float64(len(methodHasMock)) / float64(totalForRate),
	}
	scores.CleanRate = float64(scores.MethodsClean) / float64(totalForRate)
	summary.Scores = scores

	// Include legacy errors that aren't in findings (infrastructure errors)
	for _, e := range legacyErrors {
		bin, msg := parseWarningBin(e.Error())
		if bin == "other" {
			// Only include if not already represented in findings
			summary.Errors = append(summary.Errors, msg)
		}
	}
	if summary.TotalErrors == 0 {
		summary.TotalErrors = len(legacyErrors)
	}
	if summary.TotalWarnings == 0 {
		summary.TotalWarnings = len(legacyWarnings)
	}

	out, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal summary: %v"}`, err)
	}
	return string(out)
}

// FormatLogEntryJSON returns a single JSONL line for an analysis event.
func FormatLogEntryJSON(level string, message string) string {
	entry := struct {
		Level   string `json:"level"`
		Bin     string `json:"bin,omitempty"`
		Message string `json:"message"`
	}{
		Level:   level,
		Message: message,
	}
	if level == "warning" || level == "error" {
		bin, msg := parseWarningBin(message)
		if bin != "other" {
			entry.Bin = bin
			entry.Message = msg
		}
	}
	out, _ := json.Marshal(entry)
	return string(out)
}

// FormatFindingJSON returns a single JSONL line for a structured finding.
func FormatFindingJSON(f AnalysisFinding) string {
	out, _ := json.Marshal(f)
	return string(out)
}

// ClassifyWarnings parses tagged warning strings and groups them by bin (legacy).
func ClassifyWarnings(warnings []string) map[string][]string {
	bins := make(map[string][]string)
	for _, w := range warnings {
		bin, msg := parseWarningBin(w)
		bins[bin] = append(bins[bin], msg)
	}
	return bins
}

// FormatWarningSummary returns a human-readable summary (legacy, kept for reference).
func FormatWarningSummary(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	bins := ClassifyWarnings(warnings)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== Warning Summary (%d total) ===\n", len(warnings)))
	binNames := make([]string, 0, len(bins))
	for b := range bins {
		binNames = append(binNames, b)
	}
	sort.Strings(binNames)
	for _, b := range binNames {
		items := bins[b]
		sb.WriteString(fmt.Sprintf("\n[%s] (%d)\n", b, len(items)))
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("  - %s\n", item))
		}
	}
	return sb.String()
}

func parseWarningBin(warning string) (string, string) {
	if !strings.HasPrefix(warning, "[") {
		return "other", warning
	}
	end := strings.Index(warning, "] ")
	if end == -1 {
		return "other", warning
	}
	return warning[1:end], warning[end+2:]
}
