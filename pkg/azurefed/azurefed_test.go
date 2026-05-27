package azurefed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

// TestTokenSourceExchangesSubjectAsClientAssertion proves the exchange shape:
// the foreign OIDC token is sent as client_assertion (JWT-bearer) — not as a
// client secret — alongside the federated app's client_id and the target
// resource's scope.
func TestTokenSourceExchangesSubjectAsClientAssertion(t *testing.T) {
	var lastForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		lastForm, _ = url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"ENTRA_456","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	ts, err := TokenSource(context.Background(), Config{
		TenantID: "00000000-0000-0000-0000-000000000000",
		ClientID: "app-id-abc",
		Scopes:   []string{"https://management.azure.com/.default"},
		Endpoint: server.URL,
	}, func() (string, error) { return "the-foreign-jwt", nil }, server.Client())
	if err != nil {
		t.Fatalf("TokenSource: %v", err)
	}
	tok, err := ts.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok.AccessToken != "ENTRA_456" {
		t.Errorf("AccessToken = %q, want ENTRA_456", tok.AccessToken)
	}
	if got := lastForm.Get("grant_type"); got != "client_credentials" {
		t.Errorf("grant_type = %q, want client_credentials", got)
	}
	if got := lastForm.Get("client_id"); got != "app-id-abc" {
		t.Errorf("client_id = %q, want app-id-abc", got)
	}
	if got := lastForm.Get("client_assertion_type"); got != clientAssertionType {
		t.Errorf("client_assertion_type = %q, want %q", got, clientAssertionType)
	}
	if got := lastForm.Get("client_assertion"); got != "the-foreign-jwt" {
		t.Errorf("client_assertion = %q, want the-foreign-jwt", got)
	}
	if got := lastForm.Get("client_secret"); got != "" {
		t.Errorf("client_secret unexpectedly sent: %q", got)
	}
	if got := lastForm.Get("scope"); got != "https://management.azure.com/.default" {
		t.Errorf("scope = %q, want management/.default", got)
	}
}

// TestTokenSourceRefreshesOnExpiry proves the reuse wrapper re-invokes the
// token endpoint once the cached token is past its renewal window, picking up
// a fresh subject token in the process.
func TestTokenSourceRefreshesOnExpiry(t *testing.T) {
	var fetches int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&fetches, 1)
		w.Header().Set("Content-Type", "application/json")
		// expires_in=1 sits well inside the oauth2 reuse delta, so each call refetches.
		_, _ = fmt.Fprintf(w, `{"access_token":"ENTRA_%d","token_type":"Bearer","expires_in":1}`, n)
	}))
	defer server.Close()

	ts, err := TokenSource(context.Background(), Config{
		TenantID: "tenant",
		ClientID: "app",
		Scopes:   []string{"https://graph.microsoft.com/.default"},
		Endpoint: server.URL,
	}, func() (string, error) { return "subject", nil }, server.Client())
	if err != nil {
		t.Fatalf("TokenSource: %v", err)
	}

	first, err := ts.Token()
	if err != nil {
		t.Fatalf("first Token: %v", err)
	}
	second, err := ts.Token()
	if err != nil {
		t.Fatalf("second Token: %v", err)
	}
	if first.AccessToken == second.AccessToken {
		t.Errorf("expected refresh between calls; got same token %q twice", first.AccessToken)
	}
	if n := atomic.LoadInt32(&fetches); n != 2 {
		t.Errorf("token endpoint hits = %d, want 2", n)
	}
}

func TestTokenSourceSurfacesEntraErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"AADSTS70021: No matching federated identity credential found."}`))
	}))
	defer server.Close()

	ts, err := TokenSource(context.Background(), Config{
		TenantID: "tenant",
		ClientID: "app",
		Scopes:   []string{"x/.default"},
		Endpoint: server.URL,
	}, func() (string, error) { return "subject", nil }, server.Client())
	if err != nil {
		t.Fatalf("TokenSource: %v", err)
	}
	if _, err := ts.Token(); err == nil {
		t.Fatal("expected error from invalid_client response, got nil")
	}
}

func TestTokenSourceValidatesInput(t *testing.T) {
	get := func() (string, error) { return "x", nil }
	cases := []struct {
		name string
		cfg  Config
	}{
		{"missing tenant", Config{ClientID: "id"}},
		{"missing client", Config{TenantID: "tenant"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := TokenSource(context.Background(), tc.cfg, get, nil); err == nil {
				t.Errorf("expected error for %s", tc.name)
			}
		})
	}
	if _, err := TokenSource(context.Background(), Config{TenantID: "t", ClientID: "c"}, nil, nil); err == nil {
		t.Error("expected error when retriever nil")
	}
}
