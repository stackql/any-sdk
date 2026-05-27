// Package gcpwif implements Google Cloud Workload Identity Federation: it
// exchanges a foreign OIDC subject token at Google's STS endpoint
// (sts.googleapis.com/v1/token) for a Google OAuth2 access token, optionally
// impersonating a service account via iamcredentials. The exchange is carried
// out by the canonical golang.org/x/oauth2/google/externalaccount package, so
// the returned TokenSource caches and auto-refreshes credentials per the
// standard oauth2 contract.
package gcpwif

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google/externalaccount"
)

// defaultSubjectTokenType is the value advertised by IRSA-style OIDC providers
// (GitHub Actions, GitLab, k8s projected service-account tokens). Callers can
// override it via Config.SubjectTokenType for SAML or aws4_request flows.
const defaultSubjectTokenType = "urn:ietf:params:oauth:token-type:jwt"

// Config describes a GCP Workload Identity Federation exchange.
type Config struct {
	// Audience is the full pool-provider resource name, e.g.
	// //iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/POOL/providers/PROVIDER.
	Audience string
	// SubjectTokenType identifies the format of the foreign token. Defaults to
	// urn:ietf:params:oauth:token-type:jwt when empty.
	SubjectTokenType string
	// TokenURL optionally overrides the STS exchange endpoint. Defaults to
	// https://sts.googleapis.com/v1/token (handled by externalaccount).
	TokenURL string
	// Scopes are applied to the resulting Google access token (typically
	// https://www.googleapis.com/auth/cloud-platform).
	Scopes []string
	// ServiceAccountImpersonationURL, when set, exchanges the federated token
	// for an impersonated service-account access token via iamcredentials, e.g.
	// https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SA_EMAIL:generateAccessToken.
	ServiceAccountImpersonationURL string
}

// subjectTokenSupplier adapts a closure to externalaccount.SubjectTokenSupplier.
// It is invoked on every token exchange, so file-backed retrievers pick up
// rotated tokens transparently.
type subjectTokenSupplier func() (string, error)

func (s subjectTokenSupplier) SubjectToken(_ context.Context, _ externalaccount.SupplierOptions) (string, error) {
	return s()
}

// TokenSource builds an auto-refreshing Google OAuth2 token source backed by an
// external OIDC subject token. ctx should be long-lived (e.g. background) — it
// governs refresh fetches, not a single request. httpClient (which may be nil)
// carries TLS/proxy configuration for both the STS exchange and any service
// account impersonation call.
func TokenSource(
	ctx context.Context,
	cfg Config,
	getSubjectToken func() (string, error),
	httpClient *http.Client,
) (oauth2.TokenSource, error) {
	if cfg.Audience == "" {
		return nil, fmt.Errorf("gcp wif: audience is required")
	}
	if getSubjectToken == nil {
		return nil, fmt.Errorf("gcp wif: subject token retriever is required")
	}

	subjectTokenType := cfg.SubjectTokenType
	if subjectTokenType == "" {
		subjectTokenType = defaultSubjectTokenType
	}

	tokenCtx := ctx
	if httpClient != nil {
		tokenCtx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	}

	eaCfg := externalaccount.Config{
		Audience:                       cfg.Audience,
		SubjectTokenType:               subjectTokenType,
		TokenURL:                       cfg.TokenURL,
		Scopes:                         cfg.Scopes,
		ServiceAccountImpersonationURL: cfg.ServiceAccountImpersonationURL,
		SubjectTokenSupplier:           subjectTokenSupplier(getSubjectToken),
	}
	return externalaccount.NewTokenSource(tokenCtx, eaCfg)
}
