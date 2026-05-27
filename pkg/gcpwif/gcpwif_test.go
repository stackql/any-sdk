package gcpwif

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const gcpStsResponse = `{"access_token":"GOOGLE_FED_ACCESS_123","issued_token_type":"urn:ietf:params:oauth:token-type:access_token","token_type":"Bearer","expires_in":3600,"scope":"https://www.googleapis.com/auth/cloud-platform"}`

// TestTokenSourceExchangesSubjectAtGoogleSTS proves the externalaccount-backed
// TokenSource forwards the configured audience and the supplier-returned
// subject token to Google's STS endpoint and surfaces the resulting access
// token via the standard oauth2 contract.
func TestTokenSourceExchangesSubjectAtGoogleSTS(t *testing.T) {
	var lastForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		lastForm = form
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(gcpStsResponse))
	}))
	defer server.Close()

	ts, err := TokenSource(context.Background(), Config{
		Audience: "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/POOL/providers/PROVIDER",
		TokenURL: server.URL,
		Scopes:   []string{"https://www.googleapis.com/auth/cloud-platform"},
	}, func() (string, error) { return "the-foreign-jwt", nil }, server.Client())
	if err != nil {
		t.Fatalf("TokenSource: %v", err)
	}

	tok, err := ts.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok.AccessToken != "GOOGLE_FED_ACCESS_123" {
		t.Errorf("AccessToken = %q, want GOOGLE_FED_ACCESS_123", tok.AccessToken)
	}
	if got := lastForm.Get("grant_type"); got != "urn:ietf:params:oauth:grant-type:token-exchange" {
		t.Errorf("grant_type = %q, want token-exchange", got)
	}
	if got := lastForm.Get("subject_token"); got != "the-foreign-jwt" {
		t.Errorf("subject_token = %q, want the-foreign-jwt", got)
	}
	if got := lastForm.Get("subject_token_type"); got != "urn:ietf:params:oauth:token-type:jwt" {
		t.Errorf("subject_token_type = %q, want default JWT type", got)
	}
	if !contains(lastForm["audience"], "/providers/PROVIDER") {
		t.Errorf("audience missing provider suffix: %v", lastForm["audience"])
	}
}

// TestTokenSourceServiceAccountImpersonation chains the federated token through
// iamcredentials to mint an impersonated service-account access token, then
// hands that back to the caller — the canonical WIF flow for Google APIs that
// require principal:// permissions.
func TestTokenSourceServiceAccountImpersonation(t *testing.T) {
	const impersonatedToken = "SA_IMPERSONATED_999"
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(gcpStsResponse))
	})
	mux.HandleFunc("/impersonate", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accessToken": impersonatedToken,
			"expireTime":  "2999-01-01T00:00:00Z",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	ts, err := TokenSource(context.Background(), Config{
		Audience:                       "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/POOL/providers/PROVIDER",
		TokenURL:                       server.URL + "/v1/token",
		Scopes:                         []string{"https://www.googleapis.com/auth/cloud-platform"},
		ServiceAccountImpersonationURL: server.URL + "/impersonate",
	}, func() (string, error) { return "subject", nil }, server.Client())
	if err != nil {
		t.Fatalf("TokenSource: %v", err)
	}
	tok, err := ts.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok.AccessToken != impersonatedToken {
		t.Errorf("AccessToken = %q, want %q (impersonated SA token)", tok.AccessToken, impersonatedToken)
	}
}

func TestTokenSourceValidatesInput(t *testing.T) {
	if _, err := TokenSource(context.Background(), Config{}, func() (string, error) { return "x", nil }, nil); err == nil {
		t.Error("expected error when audience missing")
	}
	if _, err := TokenSource(context.Background(), Config{Audience: "//iam.googleapis.com/..."}, nil, nil); err == nil {
		t.Error("expected error when retriever nil")
	}
}

func contains(haystack []string, needleSubstr string) bool {
	for _, s := range haystack {
		if s == needleSubstr || (len(needleSubstr) > 0 && len(s) >= len(needleSubstr) && stringContains(s, needleSubstr)) {
			return true
		}
	}
	return false
}

// stringContains avoids importing strings just for one check in a tiny test.
func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
