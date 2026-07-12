package auth_util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stackql/any-sdk/pkg/dto"
)

type fakeHTTPContext struct{}

func (fakeHTTPContext) GetCABundle() string { return "" }

func (fakeHTTPContext) GetHTTPProxyHost() string { return "" }

func (fakeHTTPContext) GetHTTPProxyUser() string { return "" }

func (fakeHTTPContext) GetHTTPProxyPassword() string { return "" }

func (fakeHTTPContext) GetHTTPProxyPort() int { return 0 }

func (fakeHTTPContext) GetHTTPProxyScheme() string { return "http" }

func (fakeHTTPContext) GetAPIRequestTimeout() int { return 30 }

func (fakeHTTPContext) GetTLSAllowInsecure() bool { return false }

func testOciKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
}

func TestOciSigningAuth_RawEnvVarResolution(t *testing.T) {
	t.Setenv("TEST_OCI_TENANCY_OCID", "ocid1.tenancy.oc1..t")
	t.Setenv("TEST_OCI_USER_OCID", "ocid1.user.oc1..u")
	t.Setenv("TEST_OCI_FINGERPRINT", "aa:bb:cc")
	t.Setenv("TEST_OCI_PRIVATE_KEY", testOciKeyPEM(t))

	authCtx := &dto.AuthCtx{
		Type:                 dto.AuthOciSigningv1Str,
		OciTenancyOCIDEnvVar: "TEST_OCI_TENANCY_OCID",
		OciUserOCIDEnvVar:    "TEST_OCI_USER_OCID",
		OciFingerprintEnvVar: "TEST_OCI_FINGERPRINT",
		OciPrivateKeyEnvVar:  "TEST_OCI_PRIVATE_KEY",
	}
	au := NewAuthUtility(nil)
	client, err := au.OciSigningAuth(authCtx, fakeHTTPContext{})
	if err != nil {
		t.Fatalf("OciSigningAuth: %v", err)
	}
	if client == nil || client.Transport == nil {
		t.Fatalf("expected a client with a signing transport")
	}
	if !authCtx.Active || authCtx.Type != dto.AuthOciSigningv1Str {
		t.Fatalf("auth context not activated: active=%v type=%q", authCtx.Active, authCtx.Type)
	}
}

func TestOciSigningAuth_PrivateKeyFilePath(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "oci_api_key.pem")
	if err := os.WriteFile(keyPath, []byte(testOciKeyPEM(t)), 0600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	authCtx := &dto.AuthCtx{
		Type:                  dto.AuthOciSigningv1Str,
		OciTenancyOCID:        "ocid1.tenancy.oc1..t",
		OciUserOCID:           "ocid1.user.oc1..u",
		OciFingerprint:        "aa:bb:cc",
		OciPrivateKeyFilePath: keyPath,
	}
	au := NewAuthUtility(nil)
	if _, err := au.OciSigningAuth(authCtx, fakeHTTPContext{}); err != nil {
		t.Fatalf("OciSigningAuth with key file path: %v", err)
	}
}

func TestOciSigningAuth_IncompleteRawCredentials(t *testing.T) {
	// A partially supplied raw credential set must fail fast rather than fall
	// through to the config file and mask the misconfiguration.
	authCtx := &dto.AuthCtx{
		Type:           dto.AuthOciSigningv1Str,
		OciTenancyOCID: "ocid1.tenancy.oc1..t",
	}
	au := NewAuthUtility(nil)
	_, err := au.OciSigningAuth(authCtx, fakeHTTPContext{})
	if err == nil {
		t.Fatalf("expected an error for incomplete raw credentials")
	}
	if !strings.Contains(err.Error(), "tenancy_ocid, user_ocid, fingerprint and a private key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
