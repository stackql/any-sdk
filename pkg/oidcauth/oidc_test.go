package oidcauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

// newMockIdP stands up a mock OIDC provider exposing a discovery document and a
// client_credentials token endpoint. It records the most recent token-request
// form so tests can assert on the request shape.
func newMockIdP(t *testing.T) (server *httptest.Server, lastForm *url.Values) {
	t.Helper()
	var baseURL string
	form := &url.Values{}

	mux := http.NewServeMux()
	mux.HandleFunc(wellKnownSuffix, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ProviderMetadata{
			Issuer:        baseURL,
			TokenEndpoint: baseURL + "/oauth2/token",
		})
	})
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		*form = r.Form
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"access_token":"ACCESS_123","token_type":"Bearer","expires_in":3600,"id_token":"ID_456"}`)
	})

	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	baseURL = server.URL
	return server, form
}

func TestFetchTokenViaDiscovery(t *testing.T) {
	server, form := newMockIdP(t)

	token, err := FetchToken(context.Background(), Config{
		Issuer:       server.URL,
		ClientID:     "client-abc",
		ClientSecret: "secret-xyz",
		Scopes:       []string{"openid", "api.read"},
		Audience:     "https://api.example.com",
	})
	if err != nil {
		t.Fatalf("FetchToken returned error: %v", err)
	}
	if token != "ACCESS_123" {
		t.Errorf("token = %q, want ACCESS_123 (default access_token)", token)
	}
	if got := form.Get("audience"); got != "https://api.example.com" {
		t.Errorf("audience param = %q, want https://api.example.com", got)
	}
	if got := form.Get("scope"); got != "openid api.read" {
		t.Errorf("scope param = %q, want \"openid api.read\"", got)
	}
	if got := form.Get("grant_type"); got != "client_credentials" {
		t.Errorf("grant_type = %q, want client_credentials", got)
	}
}

func TestFetchTokenExplicitEndpointSkipsDiscovery(t *testing.T) {
	server, _ := newMockIdP(t)

	// Point discovery at a non-existent path to prove it is never consulted when
	// an explicit token URL is supplied.
	token, err := FetchToken(context.Background(), Config{
		Issuer:       "http://127.0.0.1:1/should-not-be-used",
		TokenURL:     server.URL + "/oauth2/token",
		ClientID:     "client-abc",
		ClientSecret: "secret-xyz",
	})
	if err != nil {
		t.Fatalf("FetchToken returned error: %v", err)
	}
	if token != "ACCESS_123" {
		t.Errorf("token = %q, want ACCESS_123", token)
	}
}

func TestFetchTokenIDTokenSelection(t *testing.T) {
	server, _ := newMockIdP(t)

	token, err := FetchToken(context.Background(), Config{
		DiscoveryURL: server.URL + wellKnownSuffix,
		ClientID:     "client-abc",
		ClientSecret: "secret-xyz",
		TokenType:    TokenTypeIDToken,
	})
	if err != nil {
		t.Fatalf("FetchToken returned error: %v", err)
	}
	if token != "ID_456" {
		t.Errorf("token = %q, want ID_456 (id_token)", token)
	}
}

func TestFetchTokenValidation(t *testing.T) {
	testCases := []struct {
		name string
		cfg  Config
	}{
		{
			name: "missing client credentials",
			cfg:  Config{Issuer: "https://idp.example.com"},
		},
		{
			name: "no endpoint or issuer",
			cfg:  Config{ClientID: "id", ClientSecret: "secret"},
		},
		{
			name: "unsupported token type",
			cfg:  Config{TokenURL: "https://idp.example.com/token", ClientID: "id", ClientSecret: "secret", TokenType: "bogus"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := FetchToken(context.Background(), tc.cfg); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

type capturingRoundTripper struct {
	req *http.Request
}

func (c *capturingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.req = req
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
}

// TestTransportRefreshesToken proves the Transport re-fetches a token once the
// previous one is no longer valid, rather than pinning a single token for the
// life of the client. The mock issues tokens with a 1s lifetime; the oauth2
// reuse source treats those as already expired (its renewal delta exceeds the
// lifetime), so every request triggers a fresh fetch with an incrementing value.
func TestTransportRefreshesToken(t *testing.T) {
	var fetches int32
	var baseURL string
	mux := http.NewServeMux()
	mux.HandleFunc(wellKnownSuffix, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ProviderMetadata{Issuer: baseURL, TokenEndpoint: baseURL + "/oauth2/token"})
	})
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&fetches, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":"ACCESS_%d","token_type":"Bearer","expires_in":1}`, n)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	baseURL = server.URL

	ts, err := TokenSource(context.Background(), Config{
		Issuer:       server.URL,
		ClientID:     "client-abc",
		ClientSecret: "secret-xyz",
		HTTPClient:   server.Client(),
	})
	if err != nil {
		t.Fatalf("TokenSource returned error: %v", err)
	}
	capturer := &capturingRoundTripper{}
	tr := &Transport{Base: capturer, TokenSource: ts}

	doRequest := func() string {
		req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/x", nil)
		if _, rtErr := tr.RoundTrip(req); rtErr != nil {
			t.Fatalf("RoundTrip returned error: %v", rtErr)
		}
		return capturer.req.Header.Get("Authorization")
	}

	if got := doRequest(); got != "Bearer ACCESS_1" {
		t.Errorf("first request Authorization = %q, want Bearer ACCESS_1", got)
	}
	if got := doRequest(); got != "Bearer ACCESS_2" {
		t.Errorf("second request Authorization = %q, want Bearer ACCESS_2 (expected refresh)", got)
	}
	if n := atomic.LoadInt32(&fetches); n != 2 {
		t.Errorf("token fetches = %d, want 2 (one per request after expiry)", n)
	}
}

// TestTransportPlacement covers the configurable attach targets: custom header,
// and query parameter.
func TestTransportPlacement(t *testing.T) {
	server, _ := newMockIdP(t)
	ts, err := TokenSource(context.Background(), Config{
		Issuer: server.URL, ClientID: "id", ClientSecret: "secret", HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("TokenSource returned error: %v", err)
	}

	t.Run("custom header", func(t *testing.T) {
		capturer := &capturingRoundTripper{}
		tr := &Transport{Base: capturer, TokenSource: ts, Name: "X-Api-Token"}
		req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/x", nil)
		if _, rtErr := tr.RoundTrip(req); rtErr != nil {
			t.Fatalf("RoundTrip returned error: %v", rtErr)
		}
		if got := capturer.req.Header.Get("X-Api-Token"); got != "ACCESS_123" {
			t.Errorf("X-Api-Token = %q, want ACCESS_123", got)
		}
		if got := capturer.req.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization unexpectedly set: %q", got)
		}
	})

	t.Run("query param", func(t *testing.T) {
		capturer := &capturingRoundTripper{}
		tr := &Transport{Base: capturer, TokenSource: ts, Location: LocationQuery, Name: "access_token"}
		req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/x", nil)
		if _, rtErr := tr.RoundTrip(req); rtErr != nil {
			t.Fatalf("RoundTrip returned error: %v", rtErr)
		}
		if got := capturer.req.URL.Query().Get("access_token"); got != "ACCESS_123" {
			t.Errorf("access_token query = %q, want ACCESS_123", got)
		}
	})
}

func TestDiscoverRejectsMissingTokenEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"issuer":"https://idp.example.com"}`)
	}))
	defer server.Close()

	if _, err := Discover(context.Background(), server.URL, server.Client()); err == nil {
		t.Error("expected error when token_endpoint is absent, got nil")
	}
}
