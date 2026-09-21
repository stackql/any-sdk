package auth_util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stackql/any-sdk/pkg/dto"
)

const testOauthAccessToken = "test-access-token"

type shortTimeoutHTTPContext struct {
	fakeHTTPContext
}

func (shortTimeoutHTTPContext) GetAPIRequestTimeout() int { return 1 }

// newTestOauthServer serves a token endpoint plus a fast and a slow API
// endpoint; the slow endpoint outlives the client timeout.
func newTestOauthServer(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":%q,"token_type":"Bearer","expires_in":3600}`, testOauthAccessToken)
	})
	mux.HandleFunc("/fast", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, r.Header.Get("Authorization"))
	})
	mux.HandleFunc("/slow", func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		close(release)
		server.Close()
	})
	return server
}

func captureStdLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	original := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(original) })
	return &buf
}

// assertOauthClientCancelBehaviour pins stackql/any-sdk#143. net/http calls
// CancelRequest on any unrecognised RoundTripper that has one when the client
// timeout fires, and the oauth2 implementation only logs a deprecation warning.
// The structural check is the authoritative one: oauth2 emits the warning at
// most once per process, so the absence of the log line alone proves little.
func assertOauthClientCancelBehaviour(t *testing.T, httpClient *http.Client, server *httptest.Server) {
	t.Helper()
	if _, isCanceler := httpClient.Transport.(interface{ CancelRequest(*http.Request) }); isCanceler {
		t.Fatalf("oauth client transport %T exposes deprecated CancelRequest", httpClient.Transport)
	}
	logged := captureStdLog(t)

	response, err := httpClient.Get(server.URL + "/fast")
	if err != nil {
		t.Fatalf("authorised request: %v", err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatalf("read authorised response: %v", err)
	}
	if string(body) != "Bearer "+testOauthAccessToken {
		t.Fatalf("expected bearer token to be applied, got %q", string(body))
	}

	start := time.Now()
	_, err = httpClient.Get(server.URL + "/slow") //nolint:bodyclose // request is expected to fail
	elapsed := time.Since(start)
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected client timeout error, got %v", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("client timeout not honoured, request took %s", elapsed)
	}
	if strings.Contains(logged.String(), "CancelRequest") {
		t.Fatalf("unexpected deprecation log: %q", logged.String())
	}
}

func TestGenericOauthClientCredentials_NoDeprecatedCancelRequest(t *testing.T) {
	server := newTestOauthServer(t)
	authCtx := &dto.AuthCtx{
		Type:         dto.OAuth2Str,
		GrantType:    dto.ClientCredentialsStr,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		TokenURL:     server.URL + "/token",
	}
	au := NewAuthUtility(nil)
	httpClient, err := au.GenericOauthClientCredentials(authCtx, nil, shortTimeoutHTTPContext{})
	if err != nil {
		t.Fatalf("GenericOauthClientCredentials: %v", err)
	}
	if httpClient.Timeout != time.Second {
		t.Fatalf("expected client timeout to be preserved, got %s", httpClient.Timeout)
	}
	assertOauthClientCancelBehaviour(t, httpClient, server)
}

func TestGoogleOauthServiceAccount_NoDeprecatedCancelRequest(t *testing.T) {
	server := newTestOauthServer(t)
	credentials, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"client_email": "test@example.iam.gserviceaccount.com",
		"private_key":  testOciKeyPEM(t),
		"token_uri":    server.URL + "/token",
	})
	if err != nil {
		t.Fatalf("marshal service account credentials: %v", err)
	}
	t.Setenv("TEST_GOOGLE_SERVICE_ACCOUNT", string(credentials))
	authCtx := &dto.AuthCtx{
		Type:      dto.AuthServiceAccountStr,
		KeyEnvVar: "TEST_GOOGLE_SERVICE_ACCOUNT",
	}
	au := NewAuthUtility(nil)
	httpClient, err := au.GoogleOauthServiceAccount("google", authCtx, nil, shortTimeoutHTTPContext{})
	if err != nil {
		t.Fatalf("GoogleOauthServiceAccount: %v", err)
	}
	if httpClient.Timeout != time.Second {
		t.Fatalf("expected client timeout to be preserved, got %s", httpClient.Timeout)
	}
	assertOauthClientCancelBehaviour(t, httpClient, server)
}
