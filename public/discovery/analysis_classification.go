package discovery

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Warning classification bins for static analysis.
// Each warning string is prefixed with its bin tag in square brackets.
const (
	BinResponseShapeUnsafe = "response-shape-unsafe"
	BinObjectKeyUnroutable = "objectKey-unroutable"
	BinEmptyResponseUnsafe = "empty-response-unsafe"
	BinMediaTypeMismatch   = "media-type-mismatch"
	BinMissingSemantics    = "missing-semantics"
)

// classifiedWarning prefixes a warning message with its bin tag.
func classifiedWarning(bin string, format string, args ...interface{}) string {
	return fmt.Sprintf("[%s] %s", bin, fmt.Sprintf(format, args...))
}

// ClassifyWarnings parses tagged warning strings and groups them by bin.
// Untagged warnings go into an "other" bin.
func ClassifyWarnings(warnings []string) map[string][]string {
	bins := make(map[string][]string)
	for _, w := range warnings {
		bin, msg := parseWarningBin(w)
		bins[bin] = append(bins[bin], msg)
	}
	return bins
}

// AnalysisSummary is the JSON-serialisable top-level output of static analysis.
type AnalysisSummary struct {
	TotalOK       int                       `json:"total_ok"`
	TotalWarnings int                       `json:"total_warnings"`
	TotalErrors   int                       `json:"total_errors"`
	Bins          map[string]AnalysisBin    `json:"bins"`
	Errors        []string                  `json:"errors,omitempty"`
}

// AnalysisBin holds the items for a single classification bin.
type AnalysisBin struct {
	Count    int      `json:"count"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// FormatSummaryJSON returns a JSON summary of errors and classified warnings.
func FormatSummaryJSON(errors []error, warnings []string, affirmatives []string) string {
	bins := ClassifyWarnings(warnings)

	summary := AnalysisSummary{
		TotalOK:       len(affirmatives),
		TotalWarnings: len(warnings),
		TotalErrors:   len(errors),
		Bins:          make(map[string]AnalysisBin),
	}

	// Classify errors into bins (if tagged) or the top-level errors list
	for _, e := range errors {
		bin, msg := parseWarningBin(e.Error())
		if bin != "other" {
			b := bins[bin]
			b = append(b, "ERROR: "+msg)
			bins[bin] = b
		} else {
			summary.Errors = append(summary.Errors, e.Error())
		}
	}

	// Sort bin names for stable output
	binNames := make([]string, 0, len(bins))
	for b := range bins {
		binNames = append(binNames, b)
	}
	sort.Strings(binNames)

	for _, b := range binNames {
		items := bins[b]
		ab := AnalysisBin{Count: len(items)}
		for _, item := range items {
			if strings.HasPrefix(item, "ERROR: ") {
				ab.Errors = append(ab.Errors, strings.TrimPrefix(item, "ERROR: "))
			} else {
				ab.Warnings = append(ab.Warnings, item)
			}
		}
		summary.Bins[b] = ab
	}

	out, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal summary: %v"}`, err)
	}
	return string(out)
}

// AnalysisLogEntry is a single JSONL line emitted during verbose analysis.
type AnalysisLogEntry struct {
	Level    string `json:"level"`              // "error", "warning", "info"
	Bin      string `json:"bin,omitempty"`       // classification bin (warnings only)
	Message  string `json:"message"`
}

// FormatLogEntryJSON returns a single JSONL line for an analysis event.
func FormatLogEntryJSON(level string, message string) string {
	entry := AnalysisLogEntry{
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
	out, err := json.Marshal(entry)
	if err != nil {
		return fmt.Sprintf(`{"level":"error","message":"failed to marshal log entry: %v"}`, err)
	}
	return string(out)
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
