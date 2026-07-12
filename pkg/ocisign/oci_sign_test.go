package ocisign

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	testTenancyOCID = "ocid1.tenancy.oc1..testtenancy"
	testUserOCID    = "ocid1.user.oc1..testuser"
	testFingerprint = "20:3b:97:13:55:1c:5b:0d:d3:37:d8:50:4e:c5:3a:34"
	testRegion      = "us-ashburn-1"
	testDate        = "Thu, 05 Jan 2014 21:31:40 GMT"
)

func testKeyID() string {
	return fmt.Sprintf("%s/%s/%s", testTenancyOCID, testUserOCID, testFingerprint)
}

func generateTestKeyPEM(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return key, string(pemBytes)
}

// captureTransport is the underlying transport stub: it records the signed
// request and drains its body, returning a canned 200.
type captureTransport struct {
	req  *http.Request
	body []byte
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.req = req
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		c.body = b
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
}

var authFieldRegexps = map[string]*regexp.Regexp{
	"version":   regexp.MustCompile(`version="([^"]*)"`),
	"headers":   regexp.MustCompile(`headers="([^"]*)"`),
	"keyId":     regexp.MustCompile(`keyId="([^"]*)"`),
	"algorithm": regexp.MustCompile(`algorithm="([^"]*)"`),
	"signature": regexp.MustCompile(`signature="([^"]*)"`),
}

func parseAuthHeader(t *testing.T, authValue string) map[string]string {
	t.Helper()
	if !strings.HasPrefix(authValue, "Signature ") {
		t.Fatalf("Authorization header must use the Signature scheme, got %q", authValue)
	}
	rv := map[string]string{}
	for field, re := range authFieldRegexps {
		m := re.FindStringSubmatch(authValue)
		if m == nil {
			t.Fatalf("Authorization header missing %q: %q", field, authValue)
		}
		rv[field] = m[1]
	}
	return rv
}

// buildSigningString reconstructs the draft-cavage signing string from the
// signed request, per the OCI Request Signatures specification.
func buildSigningString(req *http.Request, headers string) string {
	parts := []string{}
	for _, h := range strings.Split(headers, " ") {
		var value string
		switch h {
		case "(request-target)":
			value = fmt.Sprintf("%s %s", strings.ToLower(req.Method), req.URL.RequestURI())
		case "host":
			value = req.URL.Host
		case "content-length":
			value = fmt.Sprintf("%d", req.ContentLength)
		default:
			value = req.Header.Get(h)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", h, value))
	}
	return strings.Join(parts, "\n")
}

func verifySignature(t *testing.T, pub *rsa.PublicKey, signingString, signatureB64 string) {
	t.Helper()
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		t.Fatalf("signature is not valid base64: %v", err)
	}
	hashed := sha256.Sum256([]byte(signingString))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hashed[:], sig); err != nil {
		t.Fatalf("signature does not verify over signing string %q: %v", signingString, err)
	}
}

func newTestTransport(t *testing.T, provider ConfigurationProvider) (Transport, *captureTransport) {
	t.Helper()
	capture := &captureTransport{}
	tr, err := NewOciSignTransport(capture, provider)
	if err != nil {
		t.Fatalf("NewOciSignTransport: %v", err)
	}
	return tr, capture
}

func TestOciSignTransport_GetSignsThreeHeaders(t *testing.T) {
	key, keyPEM := generateTestKeyPEM(t)
	provider := NewRawConfigurationProvider(
		testTenancyOCID, testUserOCID, testRegion, testFingerprint, keyPEM, "")
	tr, capture := newTestTransport(t, provider)

	req, _ := http.NewRequest(http.MethodGet,
		"https://identity.us-ashburn-1.oraclecloud.com/20160918/vcns?compartmentId=ocid1.compartment.oc1..x", nil)
	req.Header.Set("date", testDate)

	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}

	auth := parseAuthHeader(t, capture.req.Header.Get("Authorization"))
	if auth["version"] != "1" || auth["algorithm"] != "rsa-sha256" {
		t.Fatalf("unexpected scheme fields: %v", auth)
	}
	if auth["keyId"] != testKeyID() {
		t.Fatalf("keyId = %q, want %q", auth["keyId"], testKeyID())
	}
	if auth["headers"] != "date (request-target) host" {
		t.Fatalf("GET must sign exactly the three generic headers, got %q", auth["headers"])
	}
	wantSigningString := "date: " + testDate + "\n" +
		"(request-target): get /20160918/vcns?compartmentId=ocid1.compartment.oc1..x\n" +
		"host: identity.us-ashburn-1.oraclecloud.com"
	if got := buildSigningString(capture.req, auth["headers"]); got != wantSigningString {
		t.Fatalf("signing string mismatch:\n got: %q\nwant: %q", got, wantSigningString)
	}
	verifySignature(t, &key.PublicKey, wantSigningString, auth["signature"])
}

func TestOciSignTransport_PostSignsBodyHeaders(t *testing.T) {
	key, keyPEM := generateTestKeyPEM(t)
	provider := NewRawConfigurationProvider(
		testTenancyOCID, testUserOCID, testRegion, testFingerprint, keyPEM, "")
	tr, capture := newTestTransport(t, provider)

	body := `{"displayName":"vcn1","cidrBlock":"10.0.0.0/16"}`
	req, _ := http.NewRequest(http.MethodPost,
		"https://identity.us-ashburn-1.oraclecloud.com/20160918/vcns", strings.NewReader(body))
	req.Header.Set("date", testDate)

	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}

	auth := parseAuthHeader(t, capture.req.Header.Get("Authorization"))
	if auth["headers"] != "date (request-target) host content-length content-type x-content-sha256" {
		t.Fatalf("POST must sign the six headers, got %q", auth["headers"])
	}
	bodyHash := sha256.Sum256([]byte(body))
	wantSHA := base64.StdEncoding.EncodeToString(bodyHash[:])
	if got := capture.req.Header.Get("x-content-sha256"); got != wantSHA {
		t.Fatalf("x-content-sha256 = %q, want %q", got, wantSHA)
	}
	if got := capture.req.Header.Get("content-type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if got := capture.req.Header.Get("content-length"); got != fmt.Sprintf("%d", len(body)) {
		t.Fatalf("content-length = %q, want %d", got, len(body))
	}
	// The read-and-restore body handling must leave the payload intact.
	if string(capture.body) != body {
		t.Fatalf("body corrupted by signing: %q", string(capture.body))
	}
	verifySignature(t, &key.PublicKey,
		buildSigningString(capture.req, auth["headers"]), auth["signature"])
}

func TestOciSignTransport_SetsDateWhenAbsent(t *testing.T) {
	_, keyPEM := generateTestKeyPEM(t)
	provider := NewRawConfigurationProvider(
		testTenancyOCID, testUserOCID, testRegion, testFingerprint, keyPEM, "")
	tr, capture := newTestTransport(t, provider)

	req, _ := http.NewRequest(http.MethodGet, "https://identity.us-ashburn-1.oraclecloud.com/20160918/vcns", nil)
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if capture.req.Header.Get("date") == "" {
		t.Fatalf("transport must set a date header when absent")
	}
}

func TestOciSignTransport_PassphraseProtectedKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	//nolint:staticcheck // EncryptPEMBlock is the canonical way to build a legacy passphrase-protected fixture
	block, err := x509.EncryptPEMBlock(
		rand.Reader, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key), []byte("s3cret"), x509.PEMCipherAES256)
	if err != nil {
		t.Fatalf("encrypt test key: %v", err)
	}
	keyPEM := string(pem.EncodeToMemory(block))

	provider := NewRawConfigurationProvider(
		testTenancyOCID, testUserOCID, testRegion, testFingerprint, keyPEM, "s3cret")
	tr, capture := newTestTransport(t, provider)

	req, _ := http.NewRequest(http.MethodGet, "https://identity.us-ashburn-1.oraclecloud.com/20160918/vcns", nil)
	req.Header.Set("date", testDate)
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip with passphrase-protected key: %v", err)
	}
	auth := parseAuthHeader(t, capture.req.Header.Get("Authorization"))
	verifySignature(t, &key.PublicKey,
		buildSigningString(capture.req, auth["headers"]), auth["signature"])
}

func TestNewFileConfigurationProvider(t *testing.T) {
	key, keyPEM := generateTestKeyPEM(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "oci_api_key.pem")
	if err := os.WriteFile(keyPath, []byte(keyPEM), 0600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	configPath := filepath.Join(dir, "config")
	config := fmt.Sprintf(
		"[DEFAULT]\nuser=%s\nfingerprint=%s\ntenancy=%s\nregion=%s\nkey_file=%s\n",
		testUserOCID, testFingerprint, testTenancyOCID, testRegion, keyPath)
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	provider, err := NewFileConfigurationProvider(configPath, "DEFAULT", "")
	if err != nil {
		t.Fatalf("NewFileConfigurationProvider: %v", err)
	}
	keyID, err := provider.KeyID()
	if err != nil {
		t.Fatalf("KeyID: %v", err)
	}
	if keyID != testKeyID() {
		t.Fatalf("KeyID = %q, want %q", keyID, testKeyID())
	}

	tr, capture := newTestTransport(t, provider)
	req, _ := http.NewRequest(http.MethodGet, "https://identity.us-ashburn-1.oraclecloud.com/20160918/vcns", nil)
	req.Header.Set("date", testDate)
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	auth := parseAuthHeader(t, capture.req.Header.Get("Authorization"))
	verifySignature(t, &key.PublicKey,
		buildSigningString(capture.req, auth["headers"]), auth["signature"])
}

func TestOciSignTransport_Negative(t *testing.T) {
	if _, err := NewOciSignTransport(nil, nil); err == nil {
		t.Fatalf("nil configuration provider must be rejected")
	}

	provider := NewRawConfigurationProvider(
		testTenancyOCID, testUserOCID, testRegion, testFingerprint, "not a pem", "")
	tr, _ := newTestTransport(t, provider)
	req, _ := http.NewRequest(http.MethodGet, "https://identity.us-ashburn-1.oraclecloud.com/20160918/vcns", nil)
	req.Header.Set("date", testDate)
	if _, err := tr.RoundTrip(req); err == nil {
		t.Fatalf("malformed PEM must surface a signing error")
	}
}
