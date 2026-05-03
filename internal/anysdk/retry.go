package anysdk

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-openapi/jsonpointer"
)

const (
	RetryAlgorithmExponential = "exponential"

	defaultRetryAlgorithm      = RetryAlgorithmExponential
	defaultRetryMaxAttempts    = 3
	defaultRetryInitialDelayMs = 500
	defaultRetryMaxDelayMs     = 10000
	defaultRetryMultiplier     = 2.0
	defaultRetryJitterFraction = 0.0
)

var (
	_ RetryPolicy               = &standardRetryPolicy{}
	_ RetryConditions           = &standardRetryConditions{}
	_ jsonpointer.JSONPointable = standardRetryPolicy{}
	_ jsonpointer.JSONPointable = standardRetryConditions{}

	defaultRetryableStatusCodes = []int{
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}

	defaultRetryableMethods = []string{
		http.MethodGet,
		http.MethodHead,
	}
)

// RetryPolicy describes how a remote call should be retried on transient
// failure. Algorithm is a string so we can introduce alternative strategies
// without breaking existing specs; the only currently supported value is
// "exponential".
type RetryPolicy interface {
	GetAlgorithm() string
	GetMaxAttempts() int
	GetInitialDelay() time.Duration
	GetMaxDelay() time.Duration
	GetMultiplier() float64
	GetJitterFraction() float64
	GetRetryableConditions() RetryConditions
	GetRetryableMethods() []string
	IsStatusRetryable(statusCode int) bool
	IsMethodRetryable(method string) bool
	BackoffFor(attempt int) time.Duration
}

// RetryConditions enumerates the per-response signals that mark a result as
// retryable. Currently only HTTP status codes are checked, but the type is
// modelled as an extensible bag so future signals (header probes, body shape,
// error class) can be added without rewriting specs.
type RetryConditions interface {
	GetStatusCodes() []int
	IsStatusRetryable(statusCode int) bool
}

type standardRetryConditions struct {
	StatusCodes []int `json:"status_codes,omitempty" yaml:"status_codes,omitempty"`
}

func (rc standardRetryConditions) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "status_codes":
		return rc.StatusCodes, nil
	default:
		return nil, fmt.Errorf("could not resolve token '%s' from RetryConditions doc object", token)
	}
}

func (rc *standardRetryConditions) GetStatusCodes() []int {
	if len(rc.StatusCodes) == 0 {
		out := make([]int, len(defaultRetryableStatusCodes))
		copy(out, defaultRetryableStatusCodes)
		return out
	}
	return rc.StatusCodes
}

func (rc *standardRetryConditions) IsStatusRetryable(statusCode int) bool {
	for _, c := range rc.GetStatusCodes() {
		if c == statusCode {
			return true
		}
	}
	return false
}

type standardRetryPolicy struct {
	Algorithm           string                   `json:"algorithm,omitempty" yaml:"algorithm,omitempty"`
	MaxAttempts         int                      `json:"max_attempts,omitempty" yaml:"max_attempts,omitempty"`
	InitialDelayMs      int                      `json:"initial_delay_ms,omitempty" yaml:"initial_delay_ms,omitempty"`
	MaxDelayMs          int                      `json:"max_delay_ms,omitempty" yaml:"max_delay_ms,omitempty"`
	Multiplier          float64                  `json:"multiplier,omitempty" yaml:"multiplier,omitempty"`
	JitterFraction      float64                  `json:"jitter_fraction,omitempty" yaml:"jitter_fraction,omitempty"`
	RetryableConditions *standardRetryConditions `json:"retryable_conditions,omitempty" yaml:"retryable_conditions,omitempty"`
	RetryableMethods    []string                 `json:"retryable_methods,omitempty" yaml:"retryable_methods,omitempty"`
}

func (rp standardRetryPolicy) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "algorithm":
		return rp.Algorithm, nil
	case "max_attempts":
		return rp.MaxAttempts, nil
	case "initial_delay_ms":
		return rp.InitialDelayMs, nil
	case "max_delay_ms":
		return rp.MaxDelayMs, nil
	case "multiplier":
		return rp.Multiplier, nil
	case "jitter_fraction":
		return rp.JitterFraction, nil
	case "retryable_conditions":
		return rp.RetryableConditions, nil
	case "retryable_methods":
		return rp.RetryableMethods, nil
	default:
		return nil, fmt.Errorf("could not resolve token '%s' from RetryPolicy doc object", token)
	}
}

func (rp *standardRetryPolicy) GetAlgorithm() string {
	if rp.Algorithm == "" {
		return defaultRetryAlgorithm
	}
	return rp.Algorithm
}

func (rp *standardRetryPolicy) GetMaxAttempts() int {
	if rp.MaxAttempts < 1 {
		return defaultRetryMaxAttempts
	}
	return rp.MaxAttempts
}

func (rp *standardRetryPolicy) GetInitialDelay() time.Duration {
	if rp.InitialDelayMs <= 0 {
		return time.Duration(defaultRetryInitialDelayMs) * time.Millisecond
	}
	return time.Duration(rp.InitialDelayMs) * time.Millisecond
}

func (rp *standardRetryPolicy) GetMaxDelay() time.Duration {
	if rp.MaxDelayMs <= 0 {
		return time.Duration(defaultRetryMaxDelayMs) * time.Millisecond
	}
	return time.Duration(rp.MaxDelayMs) * time.Millisecond
}

func (rp *standardRetryPolicy) GetMultiplier() float64 {
	if rp.Multiplier <= 1 {
		return defaultRetryMultiplier
	}
	return rp.Multiplier
}

func (rp *standardRetryPolicy) GetJitterFraction() float64 {
	if rp.JitterFraction < 0 {
		return 0
	}
	if rp.JitterFraction > 1 {
		return 1
	}
	if rp.JitterFraction == 0 {
		return defaultRetryJitterFraction
	}
	return rp.JitterFraction
}

func (rp *standardRetryPolicy) GetRetryableConditions() RetryConditions {
	if rp.RetryableConditions == nil {
		return &standardRetryConditions{}
	}
	return rp.RetryableConditions
}

func (rp *standardRetryPolicy) GetRetryableMethods() []string {
	if len(rp.RetryableMethods) == 0 {
		out := make([]string, len(defaultRetryableMethods))
		copy(out, defaultRetryableMethods)
		return out
	}
	return rp.RetryableMethods
}

func (rp *standardRetryPolicy) IsStatusRetryable(statusCode int) bool {
	return rp.GetRetryableConditions().IsStatusRetryable(statusCode)
}

func (rp *standardRetryPolicy) IsMethodRetryable(method string) bool {
	for _, m := range rp.GetRetryableMethods() {
		if m == "*" {
			return true
		}
		if m == method {
			return true
		}
	}
	return false
}

// BackoffFor returns the delay to wait before the given attempt number
// (1-indexed). attempt==1 means "wait before the second try", etc.
func (rp *standardRetryPolicy) BackoffFor(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	algo := rp.GetAlgorithm()
	initial := rp.GetInitialDelay()
	maxDelay := rp.GetMaxDelay()
	var d time.Duration
	switch algo {
	case RetryAlgorithmExponential:
		multiplier := rp.GetMultiplier()
		factor := math.Pow(multiplier, float64(attempt-1))
		nanos := float64(initial) * factor
		if math.IsInf(nanos, 0) || nanos > float64(maxDelay) {
			d = maxDelay
		} else {
			d = time.Duration(nanos)
		}
	default:
		d = initial
	}
	if d > maxDelay {
		d = maxDelay
	}
	jf := rp.GetJitterFraction()
	if jf > 0 {
		noise := (rand.Float64()*2 - 1) * jf //nolint:gosec // not security-sensitive
		d = time.Duration(float64(d) * (1 + noise))
		if d < 0 {
			d = 0
		}
		if d > maxDelay {
			d = maxDelay
		}
	}
	return d
}

// DefaultRetryPolicy returns the policy applied when no x-stackQL-config
// retry block is present anywhere in the inheritance chain.
func DefaultRetryPolicy() RetryPolicy {
	return &standardRetryPolicy{}
}
