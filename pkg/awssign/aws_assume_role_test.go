package awssign

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const stsAssumeRoleResponse = `<?xml version="1.0" encoding="UTF-8"?>
<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <Credentials>
      <AccessKeyId>ASIA_TEMP_KEY</AccessKeyId>
      <SecretAccessKey>temp_secret</SecretAccessKey>
      <SessionToken>temp_session_token</SessionToken>
      <Expiration>2999-01-01T00:00:00Z</Expiration>
    </Credentials>
    <AssumedRoleUser>
      <Arn>arn:aws:sts::123456789012:assumed-role/test-role/stackql-assume-role-session</Arn>
      <AssumedRoleId>AROATESTID:stackql-assume-role-session</AssumedRoleId>
    </AssumedRoleUser>
  </AssumeRoleResult>
  <ResponseMetadata>
    <RequestId>test-request-id</RequestId>
  </ResponseMetadata>
</AssumeRoleResponse>`

// TestAssumeRoleParsesTemporaryCredentials exercises the full STS AssumeRole
// exchange against a mock endpoint, asserting both the request shape and that
// the temporary credentials are parsed out of the response.
func TestAssumeRoleParsesTemporaryCredentials(t *testing.T) {
	var capturedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(stsAssumeRoleResponse))
	}))
	defer server.Close()

	creds, err := AssumeRole(context.Background(), AssumeRoleConfig{
		BaseAccessKeyID:     "AKIDBASE",
		BaseSecretAccessKey: "basesecret",
		RoleARN:             "arn:aws:iam::123456789012:role/test-role",
		RoleSessionName:     "stackql-assume-role-session",
		ExternalID:          "ext-123",
		Region:              "us-east-1",
		Endpoint:            server.URL,
	})
	if err != nil {
		t.Fatalf("AssumeRole returned error: %v", err)
	}

	if creds.AccessKeyID != "ASIA_TEMP_KEY" {
		t.Errorf("AccessKeyID = %q, want ASIA_TEMP_KEY", creds.AccessKeyID)
	}
	if creds.SecretAccessKey != "temp_secret" {
		t.Errorf("SecretAccessKey = %q, want temp_secret", creds.SecretAccessKey)
	}
	if creds.SessionToken != "temp_session_token" {
		t.Errorf("SessionToken = %q, want temp_session_token", creds.SessionToken)
	}

	if !strings.Contains(capturedBody, "Action=AssumeRole") {
		t.Errorf("request body missing Action=AssumeRole: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "ExternalId=ext-123") {
		t.Errorf("request body missing ExternalId: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "RoleSessionName=stackql-assume-role-session") {
		t.Errorf("request body missing RoleSessionName: %s", capturedBody)
	}
}

func TestAssumeRoleValidatesInput(t *testing.T) {
	testCases := []struct {
		name string
		cfg  AssumeRoleConfig
	}{
		{
			name: "missing role ARN",
			cfg: AssumeRoleConfig{
				BaseAccessKeyID:     "AKIDBASE",
				BaseSecretAccessKey: "basesecret",
			},
		},
		{
			name: "missing base credentials",
			cfg: AssumeRoleConfig{
				RoleARN: "arn:aws:iam::123456789012:role/test-role",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AssumeRole(context.Background(), tc.cfg)
			if err == nil {
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
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

// TestNewAwsSignTransportWithCredentialsSignsWithSessionToken verifies that the
// credentials-aware constructor honours an explicit (id, secret, token) triple:
// the assumed-role access key id appears in the SigV4 Authorization header and
// the session token is forwarded verbatim, rather than being overridden from the
// environment as the default constructor would do.
func TestNewAwsSignTransportWithCredentialsSignsWithSessionToken(t *testing.T) {
	capturer := &capturingRoundTripper{}
	tr, err := NewAwsSignTransportWithCredentials(capturer, "ASIATEMP", "tempsecret", "tempsessiontoken")
	if err != nil {
		t.Fatalf("NewAwsSignTransportWithCredentials returned error: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "https://s3.ap-southeast-2.amazonaws.com/bucket", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	//nolint:revive,staticcheck // string context keys mirror awssign.RoundTrip lookups
	ctx := context.WithValue(req.Context(), "service", "s3")
	//nolint:revive,staticcheck // string context keys mirror awssign.RoundTrip lookups
	ctx = context.WithValue(ctx, "region", "ap-southeast-2")
	req = req.WithContext(ctx)

	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}

	auth := capturer.req.Header.Get("Authorization")
	if !strings.Contains(auth, "AWS4-HMAC-SHA256") {
		t.Errorf("Authorization header missing SigV4 algorithm: %q", auth)
	}
	if !strings.Contains(auth, "ASIATEMP") {
		t.Errorf("Authorization header missing assumed-role access key id: %q", auth)
	}
	if got := capturer.req.Header.Get("X-Amz-Security-Token"); got != "tempsessiontoken" {
		t.Errorf("X-Amz-Security-Token = %q, want tempsessiontoken", got)
	}
}

func TestNewAwsSignTransportWithCredentialsRequiresIDAndSecret(t *testing.T) {
	if _, err := NewAwsSignTransportWithCredentials(http.DefaultTransport, "", "secret", ""); err == nil {
		t.Error("expected error for empty id, got nil")
	}
	if _, err := NewAwsSignTransportWithCredentials(http.DefaultTransport, "id", "", ""); err == nil {
		t.Error("expected error for empty secret, got nil")
	}
}
