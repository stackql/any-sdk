package anysdk

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stackql/any-sdk/pkg/client"
)

// We test the HTTP retry path end-to-end against an in-process httptest.Server.
// This deliberately avoids depending on the broader stackql/flask mock corpus
// (which lives in the stackql repo) so the test stays self-contained here.
//
// Tests drive doWithRetry directly to dodge the need to stub the very wide
// OperationStore interface; the resolveRetryPolicy path is covered separately.

// scriptedHandler returns the next status code from `script` for each request.
// When the script is exhausted it keeps returning the final entry. It also
// records how many requests it received and the bodies it observed.
type scriptedHandler struct {
	script        []int
	calls         int64
	receivedBodies []string
}

func (h *scriptedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	idx := atomic.AddInt64(&h.calls, 1) - 1
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	h.receivedBodies = append(h.receivedBodies, string(body))
	status := h.script[len(h.script)-1]
	if int(idx) < len(h.script) {
		status = h.script[idx]
	}
	w.WriteHeader(status)
	fmt.Fprintf(w, "attempt %d -> %d", idx+1, status)
}

func newScriptedServer(t *testing.T, script ...int) (*httptest.Server, *scriptedHandler) {
	t.Helper()
	h := &scriptedHandler{script: script}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, h
}

func newTestHttpClient() *anySdkHttpClient {
	return &anySdkHttpClient{client: http.DefaultClient}
}

// fastPolicy returns a policy whose backoff is sub-millisecond so tests stay
// fast. It still exercises the real exponential math.
func fastPolicy(maxAttempts int, methods []string, statusCodes []int) RetryPolicy {
	return &standardRetryPolicy{
		Algorithm:        RetryAlgorithmExponential,
		MaxAttempts:      maxAttempts,
		InitialDelayMs:   1,
		MaxDelayMs:       5,
		Multiplier:       2,
		RetryableMethods: methods,
		RetryableConditions: &standardRetryConditions{
			StatusCodes: statusCodes,
		},
	}
}

func mustReq(t *testing.T, method, url string, body string) *http.Request {
	t.Helper()
	var br io.Reader
	if body != "" {
		br = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, br)
	if err != nil {
		t.Fatalf("could not build request: %v", err)
	}
	return req
}

// --- max_attempts == 1 means the policy is opted out of retrying ----------

func TestRetry_ZeroRepeats_OnlyOneAttempt(t *testing.T) {
	srv, h := newScriptedServer(t, http.StatusServiceUnavailable, http.StatusOK)
	hc := newTestHttpClient()
	policy := fastPolicy(1, []string{"*"}, []int{http.StatusServiceUnavailable})

	resp, err := hc.doWithRetry(mustReq(t, http.MethodGet, srv.URL, ""), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (no retry), got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt64(&h.calls); got != 1 {
		t.Fatalf("expected exactly 1 server call, got %d", got)
	}
}

// --- default policy: 3 attempts, retry on 503, GET only -------------------

func TestRetry_Default_RecoversWithinBudget(t *testing.T) {
	srv, h := newScriptedServer(t,
		http.StatusServiceUnavailable,
		http.StatusServiceUnavailable,
		http.StatusOK,
	)
	hc := newTestHttpClient()
	// Use the bare default policy — exercises the real fallback path.
	// Override the delay to keep the test fast without changing semantics.
	policy := &standardRetryPolicy{InitialDelayMs: 1, MaxDelayMs: 2}

	resp, err := hc.doWithRetry(mustReq(t, http.MethodGet, srv.URL, ""), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after retry, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt64(&h.calls); got != 3 {
		t.Fatalf("expected 3 server calls (2 retried + 1 success), got %d", got)
	}
}

func TestRetry_Default_ExhaustsAttemptsAndReturnsLastResponse(t *testing.T) {
	srv, h := newScriptedServer(t, http.StatusServiceUnavailable)
	hc := newTestHttpClient()
	policy := &standardRetryPolicy{InitialDelayMs: 1, MaxDelayMs: 2}

	resp, err := hc.doWithRetry(mustReq(t, http.MethodGet, srv.URL, ""), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 after exhaustion, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt64(&h.calls); got != int64(defaultRetryMaxAttempts) {
		t.Fatalf("expected %d server calls, got %d", defaultRetryMaxAttempts, got)
	}
}

func TestRetry_Default_DoesNotRetryNonRetryableStatus(t *testing.T) {
	srv, h := newScriptedServer(t, http.StatusBadRequest)
	hc := newTestHttpClient()
	policy := &standardRetryPolicy{InitialDelayMs: 1, MaxDelayMs: 2}

	resp, err := hc.doWithRetry(mustReq(t, http.MethodGet, srv.URL, ""), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt64(&h.calls); got != 1 {
		t.Fatalf("expected 1 server call (400 is not retryable by default), got %d", got)
	}
}

func TestRetry_Default_DoesNotRetryNonRetryableMethod(t *testing.T) {
	srv, h := newScriptedServer(t, http.StatusServiceUnavailable)
	hc := newTestHttpClient()
	policy := &standardRetryPolicy{InitialDelayMs: 1, MaxDelayMs: 2}

	resp, err := hc.doWithRetry(mustReq(t, http.MethodPost, srv.URL, "{}"), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt64(&h.calls); got != 1 {
		t.Fatalf("expected 1 server call (POST is not retryable by default), got %d", got)
	}
}

// --- caller-configured policy ---------------------------------------------

func TestRetry_Configured_FiveAttempts_SucceedsOnFifth(t *testing.T) {
	srv, h := newScriptedServer(t,
		http.StatusServiceUnavailable,
		http.StatusServiceUnavailable,
		http.StatusServiceUnavailable,
		http.StatusServiceUnavailable,
		http.StatusOK,
	)
	hc := newTestHttpClient()
	policy := fastPolicy(5, []string{http.MethodGet}, []int{http.StatusServiceUnavailable})

	resp, err := hc.doWithRetry(mustReq(t, http.MethodGet, srv.URL, ""), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on fifth attempt, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt64(&h.calls); got != 5 {
		t.Fatalf("expected 5 server calls, got %d", got)
	}
}

func TestRetry_Configured_WildcardMethodAndCustomStatus(t *testing.T) {
	srv, h := newScriptedServer(t, http.StatusConflict, http.StatusOK)
	hc := newTestHttpClient()
	policy := fastPolicy(3, []string{"*"}, []int{http.StatusConflict})

	resp, err := hc.doWithRetry(mustReq(t, http.MethodPost, srv.URL, `{"k":"v"}`), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after retry on 409, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt64(&h.calls); got != 2 {
		t.Fatalf("expected 2 server calls, got %d", got)
	}
	// Body should have been replayed exactly on retry.
	for i, b := range h.receivedBodies {
		if b != `{"k":"v"}` {
			t.Fatalf("attempt %d received body %q; expected body to be replayed verbatim", i+1, b)
		}
	}
}

// --- network-level transient failure --------------------------------------

func TestRetry_RetriesOnTransportError(t *testing.T) {
	// Stand a server up, snag its URL, then close it so the next dial fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	url := srv.URL
	srv.Close()

	hc := newTestHttpClient()
	policy := fastPolicy(3, []string{http.MethodGet}, nil)

	_, err := hc.doWithRetry(mustReq(t, http.MethodGet, url, ""), policy)
	if err == nil {
		t.Fatalf("expected dial error after exhausting retries")
	}
}

// --- context cancellation interrupts backoff ------------------------------

func TestRetry_ContextCancellationStopsBackoff(t *testing.T) {
	srv, h := newScriptedServer(t, http.StatusServiceUnavailable)
	hc := newTestHttpClient()
	policy := &standardRetryPolicy{
		MaxAttempts:    3,
		InitialDelayMs: 200,
		MaxDelayMs:     200,
		Multiplier:     1.0001,
		RetryableMethods: []string{http.MethodGet},
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := mustReq(t, http.MethodGet, srv.URL, "").WithContext(ctx)
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, _ = hc.doWithRetry(req, policy)
	// First attempt always runs; backoff before attempt 2 should be cut
	// short by ctx cancellation, so we should never see attempt 3.
	if got := atomic.LoadInt64(&h.calls); got > 2 {
		t.Fatalf("expected at most 2 server calls before cancellation, got %d", got)
	}
}

// --- resolveRetryPolicy fallback ------------------------------------------

type retrylessDesignation struct{}

func (retrylessDesignation) GetDesignation() (interface{}, bool) { return "not-an-op-store", true }

func TestResolveRetryPolicy_FallsBackToDefaultsWhenNoOperationStore(t *testing.T) {
	policy := resolveRetryPolicy(nil)
	if policy == nil {
		t.Fatalf("expected non-nil policy from nil designation")
	}
	if policy.GetMaxAttempts() != defaultRetryMaxAttempts {
		t.Fatalf("expected default max attempts %d, got %d", defaultRetryMaxAttempts, policy.GetMaxAttempts())
	}

	var d client.AnySdkDesignation = retrylessDesignation{}
	policy = resolveRetryPolicy(d)
	if policy == nil {
		t.Fatalf("expected non-nil policy when designation has no op store")
	}
	if policy.GetAlgorithm() != RetryAlgorithmExponential {
		t.Fatalf("expected default exponential algorithm, got %q", policy.GetAlgorithm())
	}
}
