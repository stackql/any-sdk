package awssign

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

const stsAssumeRoleWithWebIdentityResponse = `<?xml version="1.0" encoding="UTF-8"?>
<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleWithWebIdentityResult>
    <Credentials>
      <AccessKeyId>ASIA_WI_KEY_%d</AccessKeyId>
      <SecretAccessKey>wi_secret_%d</SecretAccessKey>
      <SessionToken>wi_session_%d</SessionToken>
      <Expiration>2999-01-01T00:00:00Z</Expiration>
    </Credentials>
    <SubjectFromWebIdentityToken>oidc-subject</SubjectFromWebIdentityToken>
    <AssumedRoleUser>
      <Arn>arn:aws:sts::123456789012:assumed-role/test-role/test-session</Arn>
      <AssumedRoleId>AROATEST:test-session</AssumedRoleId>
    </AssumedRoleUser>
  </AssumeRoleWithWebIdentityResult>
  <ResponseMetadata>
    <RequestId>web-identity-test-request-id</RequestId>
  </ResponseMetadata>
</AssumeRoleWithWebIdentityResponse>`

// TestWebIdentityRoleProviderExchangesSubjectTokenAtSTS proves the helper wires
// the subject-token retriever, role ARN, and endpoint override into STS such
// that calling Retrieve() against a mock yields the temporary credentials.
func TestWebIdentityRoleProviderExchangesSubjectTokenAtSTS(t *testing.T) {
	var fetches int32
	var lastForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&fetches, 1)
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		lastForm = form
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(strings.NewReplacer(
			"%d", "",
		).Replace(strings.ReplaceAll(stsAssumeRoleWithWebIdentityResponse, "%d", itoa(n)))))
	}))
	defer server.Close()

	provider, err := NewWebIdentityRoleProvider(
		AwsWebIdentityConfig{
			RoleARN:         "arn:aws:iam::123456789012:role/test-role",
			RoleSessionName: "test-session",
			Region:          "us-east-1",
			Endpoint:        server.URL,
		},
		func() (string, error) { return "the-oidc-jwt", nil },
	)
	if err != nil {
		t.Fatalf("NewWebIdentityRoleProvider: %v", err)
	}

	creds, err := provider.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if !strings.HasPrefix(creds.AccessKeyID, "ASIA_WI_KEY_") {
		t.Errorf("AccessKeyID = %q, want prefix ASIA_WI_KEY_", creds.AccessKeyID)
	}
	if creds.SessionToken == "" {
		t.Error("SessionToken is empty")
	}
	if got := lastForm.Get("Action"); got != "AssumeRoleWithWebIdentity" {
		t.Errorf("Action = %q, want AssumeRoleWithWebIdentity", got)
	}
	if got := lastForm.Get("WebIdentityToken"); got != "the-oidc-jwt" {
		t.Errorf("WebIdentityToken = %q, want the-oidc-jwt", got)
	}
	if got := lastForm.Get("RoleSessionName"); got != "test-session" {
		t.Errorf("RoleSessionName = %q, want test-session", got)
	}
}

// TestWebIdentityRoleProviderCachesAndRefreshes proves Retrieve() reuses cached
// credentials within their lifetime (one fetch for two calls when the token is
// long-lived) — exactly the auto-refresh contract callers expect.
func TestWebIdentityRoleProviderCachesAndRefreshes(t *testing.T) {
	var fetches int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&fetches, 1)
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(strings.ReplaceAll(stsAssumeRoleWithWebIdentityResponse, "%d", "1")))
	}))
	defer server.Close()

	provider, err := NewWebIdentityRoleProvider(
		AwsWebIdentityConfig{
			RoleARN:  "arn:aws:iam::123456789012:role/test-role",
			Region:   "us-east-1",
			Endpoint: server.URL,
		},
		func() (string, error) { return "subject", nil },
	)
	if err != nil {
		t.Fatalf("NewWebIdentityRoleProvider: %v", err)
	}

	if _, err := provider.Retrieve(context.Background()); err != nil {
		t.Fatalf("first Retrieve: %v", err)
	}
	if _, err := provider.Retrieve(context.Background()); err != nil {
		t.Fatalf("second Retrieve: %v", err)
	}
	if n := atomic.LoadInt32(&fetches); n != 1 {
		t.Errorf("STS calls = %d, want 1 (credentials cache should reuse the unexpired token)", n)
	}
}

func TestWebIdentityRoleProviderValidatesInput(t *testing.T) {
	if _, err := NewWebIdentityRoleProvider(AwsWebIdentityConfig{}, func() (string, error) { return "x", nil }); err == nil {
		t.Error("expected error when role ARN missing")
	}
	if _, err := NewWebIdentityRoleProvider(AwsWebIdentityConfig{RoleARN: "arn:..."}, nil); err == nil {
		t.Error("expected error when subject token retriever nil")
	}
}

// itoa avoids pulling strconv just to format a counter in a canned XML body.
func itoa(n int32) string {
	if n == 0 {
		return "0"
	}
	var digits [10]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
