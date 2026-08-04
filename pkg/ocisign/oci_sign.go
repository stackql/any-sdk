package ocisign

import (
	"fmt"
	"net/http"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// OCI request signing per the draft-cavage HTTP Signatures scheme, as required
// by every OCI control plane API. The official SDK signer is wrapped in an
// http.RoundTripper decorator, mirroring the aws_signing_v4 architecture in
// pkg/awssign. Unlike SigV4, no service/region participates in the signature:
// the signer needs only the keyId triple (tenancy/user/fingerprint) and the
// RSA private key, so no request-context plumbing is required.
//
// The SDK signer computes and sets x-content-sha256 and content-length itself
// (including the read-and-restore body handling) for POST/PUT/PATCH; this
// transport supplies the remaining preconditions: a `date` header and, for
// body-bearing verbs, a `content-type`.

var _ Transport = &standardOciSignTransport{}

// Transport mirrors the awssign Transport seam: an http.RoundTripper decorator.
type Transport interface {
	RoundTrip(req *http.Request) (*http.Response, error)
}

// ConfigurationProvider aliases the official SDK credentials surface so
// callers need not import the SDK to compose a signing transport.
type ConfigurationProvider = common.ConfigurationProvider

type standardOciSignTransport struct {
	underlyingTransport http.RoundTripper
	signer              common.HTTPRequestSigner
}

// NewOciSignTransport builds a signing transport around the supplied
// configuration provider, using the SDK's default signer (three generic
// headers on all requests; content-length, content-type and x-content-sha256
// additionally on POST/PUT/PATCH).
func NewOciSignTransport(
	underlyingTransport http.RoundTripper,
	provider ConfigurationProvider,
) (Transport, error) {
	if provider == nil {
		return nil, fmt.Errorf("cannot compose OCI signing transport: nil configuration provider")
	}
	if underlyingTransport == nil {
		underlyingTransport = http.DefaultTransport
	}
	return &standardOciSignTransport{
		underlyingTransport: underlyingTransport,
		signer:              common.DefaultRequestSigner(provider),
	}, nil
}

// NewRawConfigurationProvider builds a provider from explicit values,
// twelve-factor style. The region does not participate in signing and may be
// empty. An empty passphrase means an unencrypted key.
func NewRawConfigurationProvider(
	tenancyOCID, userOCID, region, fingerprint, privateKeyPEM, passphrase string,
) ConfigurationProvider {
	var passphrasePtr *string
	if passphrase != "" {
		passphrasePtr = &passphrase
	}
	return common.NewRawConfigurationProvider(
		tenancyOCID, userOCID, region, fingerprint, privateKeyPEM, passphrasePtr)
}

// NewFileConfigurationProvider builds a provider from an OCI config file
// (the ~/.oci/config convention every OCI tool shares; `~` is expanded by the
// SDK), so semantics match the OCI CLI/SDK ecosystem exactly.
func NewFileConfigurationProvider(configFilePath, profile, passphrase string) (ConfigurationProvider, error) {
	return common.ConfigurationProviderFromFileWithProfile(configFilePath, profile, passphrase)
}

func (t *standardOciSignTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// The signer signs the `date` header verbatim; the server enforces clock
	// sync (401 NotAuthenticated on >5 minutes skew).
	if req.Header.Get("date") == "" {
		req.Header.Set("date", time.Now().UTC().Format(http.TimeFormat))
	}
	// `content-type` is a signed body header on POST/PUT/PATCH; default it so
	// the signed value and the transmitted value agree.
	if isBodyVerb(req.Method) && req.Header.Get("content-type") == "" {
		req.Header.Set("content-type", "application/json")
	}
	if err := t.signer.Sign(req); err != nil {
		return nil, err
	}
	return t.underlyingTransport.RoundTrip(req)
}

// isBodyVerb reports whether the SDK's default body-hashing predicate signs
// body headers for the method.
func isBodyVerb(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch
}
