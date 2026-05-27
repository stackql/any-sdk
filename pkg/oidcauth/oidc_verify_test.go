package oidcauth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testKeyID = "test-key"

// signRS256JWT hand-builds a signed RS256 JWT so the verification tests pull in
// no JWT library beyond the standard library.
func signRS256JWT(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	seg := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal jwt segment: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": testKeyID}
	signingInput := seg(header) + "." + seg(claims)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// jwksJSON renders a JWKS document advertising the given RSA public key.
func jwksJSON(t *testing.T, pub *rsa.PublicKey) string {
	t.Helper()
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	set := map[string]any{
		"keys": []map[string]any{
			{"kty": "RSA", "use": "sig", "alg": "RS256", "kid": testKeyID, "n": n, "e": e},
		},
	}
	b, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}
	return string(b)
}

// startSigningIdP stands up a mock OIDC provider that publishes jwksKey and
// signs id_tokens with signKey (equal for the happy path; different to simulate
// a bad signature). issuerOverride, when non-empty, is advertised as the
// discovery issuer in place of the server's own URL.
func startSigningIdP(t *testing.T, jwksKey, signKey *rsa.PrivateKey, issuerOverride string) *httptest.Server {
	t.Helper()
	var baseURL string
	mux := http.NewServeMux()
	mux.HandleFunc(wellKnownSuffix, func(w http.ResponseWriter, _ *http.Request) {
		issuer := baseURL
		if issuerOverride != "" {
			issuer = issuerOverride
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w,
			`{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"jwks_uri":%q,"id_token_signing_alg_values_supported":["RS256"]}`,
			issuer, baseURL+"/auth", baseURL+"/oauth2/token", baseURL+"/jwks")
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, jwksJSON(t, &jwksKey.PublicKey))
	})
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
		idToken := signRS256JWT(t, signKey, map[string]any{
			"iss": baseURL,
			"aud": "client-abc",
			"sub": "service-account",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(),
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":"ACCESS_123","token_type":"Bearer","expires_in":3600,"id_token":%q}`, idToken)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	baseURL = server.URL
	return server
}

func TestVerifyIDTokenValid(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	server := startSigningIdP(t, key, key, "")

	token, err := FetchToken(context.Background(), Config{
		Issuer:        server.URL,
		ClientID:      "client-abc",
		ClientSecret:  "secret-xyz",
		TokenType:     TokenTypeIDToken,
		VerifyIDToken: true,
	}, server.Client())
	if err != nil {
		t.Fatalf("FetchToken with valid id_token returned error: %v", err)
	}
	if strings.Count(token, ".") != 2 {
		t.Errorf("expected a JWT id_token (three segments), got %q", token)
	}
}

func TestVerifyIDTokenRejectsBadSignature(t *testing.T) {
	jwksKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate jwks key: %v", err)
	}
	signKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	// Publish jwksKey but sign with signKey: the advertised key cannot validate
	// the token, so verification must fail closed.
	server := startSigningIdP(t, jwksKey, signKey, "")

	_, err = FetchToken(context.Background(), Config{
		Issuer:        server.URL,
		ClientID:      "client-abc",
		ClientSecret:  "secret-xyz",
		TokenType:     TokenTypeIDToken,
		VerifyIDToken: true,
	}, server.Client())
	if err == nil {
		t.Fatal("expected id_token verification to fail for mismatched signature, got nil")
	}
}

func TestVerifyIssuerMismatch(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	server := startSigningIdP(t, key, key, "https://evil.example.com")

	base := Config{
		Issuer:       server.URL,
		ClientID:     "client-abc",
		ClientSecret: "secret-xyz",
	}
	client := server.Client()

	// Opted in: the advertised issuer differs, so token resolution must fail.
	optedIn := base
	optedIn.VerifyIssuer = true
	if _, err := FetchToken(context.Background(), optedIn, client); err == nil {
		t.Error("expected error with oidc_verify_issuer enabled and mismatched issuer, got nil")
	}

	// Opted out (default): the mismatch is tolerated and a token is returned.
	if _, err := FetchToken(context.Background(), base, client); err != nil {
		t.Errorf("did not expect error with verification disabled: %v", err)
	}
}
