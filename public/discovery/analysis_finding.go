package discovery

import (
	"fmt"
)

// AnalysisFinding is a structured finding from static analysis.
// It carries the full provider/service/resource/method hierarchy.
type AnalysisFinding struct {
	Level    string `json:"level"`              // "error", "warning", "info"
	Bin      string `json:"bin,omitempty"`
	Provider string `json:"provider,omitempty"`
	Service  string `json:"service,omitempty"`
	Resource string `json:"resource,omitempty"`
	Method        string `json:"method,omitempty"`
	Message       string `json:"message"`
	FixedTemplate string `json:"fixed_template,omitempty"`
}

func (f AnalysisFinding) Error() string {
	return f.String()
}

func (f AnalysisFinding) String() string {
	return fmt.Sprintf("[%s] %s/%s/%s/%s: %s", f.Bin, f.Provider, f.Service, f.Resource, f.Method, f.Message)
}

// FindingsAware is an optional interface for analyzers that produce structured findings.
type FindingsAware interface {
	GetFindings() []AnalysisFinding
}

// AnalysisContext carries the hierarchy context for producing findings.
type AnalysisContext struct {
	Provider string
	Service  string
	Resource string
	Method   string
}

func (ctx AnalysisContext) NewError(bin string, message string) AnalysisFinding {
	return AnalysisFinding{
		Level:    "error",
		Bin:      bin,
		Provider: ctx.Provider,
		Service:  ctx.Service,
		Resource: ctx.Resource,
		Method:   ctx.Method,
		Message:  message,
	}
}

func (ctx AnalysisContext) NewWarning(bin string, message string) AnalysisFinding {
	return AnalysisFinding{
		Level:    "warning",
		Bin:      bin,
		Provider: ctx.Provider,
		Service:  ctx.Service,
		Resource: ctx.Resource,
		Method:   ctx.Method,
		Message:  message,
	}
}

func (ctx AnalysisContext) NewInfo(message string) AnalysisFinding {
	return AnalysisFinding{
		Level:    "info",
		Provider: ctx.Provider,
		Service:  ctx.Service,
		Resource: ctx.Resource,
		Method:   ctx.Method,
		Message:  message,
	}
}
