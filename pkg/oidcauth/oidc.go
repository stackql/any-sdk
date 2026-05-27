// Package oidcauth implements a generic, provider-agnostic OpenID Connect
// machine-to-machine (client_credentials) auth exchange: it resolves the token
// endpoint (explicitly, via a discovery document, or via issuer discovery),
// requests a token, and returns the caller-selected token (access_token or
// id_token) for attachment to outbound requests.
package oidcauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	// TokenTypeAccessToken selects the OAuth2 access_token as the credential.
	TokenTypeAccessToken string = "access_token"
	// TokenTypeIDToken selects the OIDC id_token as the credential.
	TokenTypeIDToken string = "id_token"

	// LocationHeader attaches the token to a request header (the default).
	LocationHeader string = "header"
	// LocationQuery attaches the token to a URL query parameter.
	LocationQuery string = "query"

	wellKnownSuffix     string = "/.well-known/openid-configuration"
	defaultHeaderName   string = "Authorization"
	defaultBearerPrefix string = "Bearer "
	defaultQueryName    string = "access_token"
)

// ProviderMetadata captures the subset of the OIDC discovery document that this
// package consumes. The full document carries far more, but only the token
// endpoint is required for the client_credentials grant.
type ProviderMetadata struct {
	Issuer        string `json:"issuer"`
	TokenEndpoint string `json:"token_endpoint"`
}

// Config fully describes an OIDC token exchange. Every field is caller-supplied;
// FetchToken applies defaults only where a value is omitted.
type Config struct {
	// Endpoint resolution. Precedence: TokenURL > DiscoveryURL > Issuer.
	Issuer       string
	DiscoveryURL string
	TokenURL     string

	// Client credentials and client-authentication style.
	ClientID     string
	ClientSecret string
	AuthStyle    int

	// Token request shaping.
	Scopes         []string
	Audience       string
	EndpointParams url.Values

	// TokenType selects which token is returned: access_token (default) or id_token.
	TokenType string

	// VerifyIssuer, when true, asserts that the issuer advertised in the
	// discovery document matches the configured Issuer. Off by default.
	VerifyIssuer bool
	// VerifyIDToken, when true, cryptographically verifies the id_token in each
	// (refreshed) token response against the provider's JWKS — checking
	// signature, expiry, and issuer — before the token is used. Requires Issuer.
	// Off by default.
	VerifyIDToken bool
}

// Discover fetches and parses an OIDC discovery document, returning its metadata.
func Discover(ctx context.Context, discoveryURL string, httpClient *http.Client) (ProviderMetadata, error) {
	var md ProviderMetadata
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return md, fmt.Errorf("oidc discovery: %w", err)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return md, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close on read path
	if resp.StatusCode != http.StatusOK {
		return md, fmt.Errorf("oidc discovery: unexpected status %d from %s", resp.StatusCode, discoveryURL)
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&md); decodeErr != nil {
		return md, fmt.Errorf("oidc discovery: decode %s: %w", discoveryURL, decodeErr)
	}
	if md.TokenEndpoint == "" {
		return md, fmt.Errorf("oidc discovery: token_endpoint absent from %s", discoveryURL)
	}
	return md, nil
}

// resolveTokenEndpoint determines the token endpoint, performing discovery only
// when an explicit token URL is not supplied.
func resolveTokenEndpoint(ctx context.Context, cfg Config, httpClient *http.Client) (string, error) {
	if cfg.TokenURL != "" {
		return cfg.TokenURL, nil
	}
	discoveryURL := cfg.DiscoveryURL
	if discoveryURL == "" && cfg.Issuer != "" {
		discoveryURL = strings.TrimSuffix(cfg.Issuer, "/") + wellKnownSuffix
	}
	if discoveryURL == "" {
		return "", fmt.Errorf("oidc: one of token_url, oidc_discovery_url, or oidc_issuer is required")
	}
	md, err := Discover(ctx, discoveryURL, httpClient)
	if err != nil {
		return "", err
	}
	// Opt-in: per the discovery spec the advertised issuer must match the one
	// used to construct the request. Only enforceable when an issuer is configured.
	if cfg.VerifyIssuer && cfg.Issuer != "" {
		want := strings.TrimSuffix(cfg.Issuer, "/")
		got := strings.TrimSuffix(md.Issuer, "/")
		if got != want {
			return "", fmt.Errorf("oidc: discovery issuer %q does not match configured issuer %q", md.Issuer, cfg.Issuer)
		}
	}
	return md.TokenEndpoint, nil
}

// TokenSource builds an auto-refreshing OAuth2 token source for the OIDC
// client_credentials exchange. Endpoint discovery (when required) happens once,
// here; the returned source caches the token and transparently re-fetches it
// against the resolved endpoint whenever it expires. The supplied context
// governs the lifetime of those refreshes, so callers should pass a long-lived
// (e.g. background) context rather than a request-scoped one. httpClient (which
// may be nil) carries TLS/proxy configuration for discovery, the token request,
// and JWKS retrieval.
func TokenSource(ctx context.Context, cfg Config, httpClient *http.Client) (oauth2.TokenSource, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("oidc: client_id and client_secret are required")
	}
	tokenEndpoint, err := resolveTokenEndpoint(ctx, cfg, httpClient)
	if err != nil {
		return nil, err
	}

	endpointParams := cfg.EndpointParams
	// Surface a first-class audience as an endpoint param without clobbering an
	// audience the caller already supplied via extra values.
	if cfg.Audience != "" {
		if endpointParams == nil {
			endpointParams = url.Values{}
		}
		if endpointParams.Get("audience") == "" {
			endpointParams.Set("audience", cfg.Audience)
		}
	}

	ccConfig := &clientcredentials.Config{
		ClientID:       cfg.ClientID,
		ClientSecret:   cfg.ClientSecret,
		TokenURL:       tokenEndpoint,
		Scopes:         cfg.Scopes,
		EndpointParams: endpointParams,
	}
	if cfg.AuthStyle > 0 {
		ccConfig.AuthStyle = oauth2.AuthStyle(cfg.AuthStyle)
	}

	tokenCtx := ctx
	if httpClient != nil {
		tokenCtx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	}

	// Each call performs one client_credentials fetch; the reuse wrapper added
	// below caches the result until expiry and re-fetches afterwards.
	var source oauth2.TokenSource = &configTokenSource{config: ccConfig, ctx: tokenCtx}

	// Opt-in: verify the id_token on every fetch/refresh before the token is
	// handed out. Layered beneath the reuse wrapper so verification runs only
	// when a token is actually (re)minted, not on every request.
	if cfg.VerifyIDToken {
		verifier, verifyErr := newIDTokenVerifier(ctx, cfg, httpClient)
		if verifyErr != nil {
			return nil, verifyErr
		}
		// go-oidc's remote key set fetches JWKS using the context passed to
		// Verify, so that context must carry the configured HTTP client.
		verifyCtx := ctx
		if httpClient != nil {
			verifyCtx = oidc.ClientContext(ctx, httpClient)
		}
		source = &verifyingTokenSource{
			source:   source,
			verifier: verifier,
			audience: cfg.Audience,
			ctx:      verifyCtx,
		}
	}

	return oauth2.ReuseTokenSource(nil, source), nil
}

// configTokenSource adapts a clientcredentials.Config to oauth2.TokenSource,
// performing a single (non-caching) token fetch per call.
type configTokenSource struct {
	config *clientcredentials.Config
	ctx    context.Context //nolint:containedctx // intentionally long-lived for refreshes
}

func (s *configTokenSource) Token() (*oauth2.Token, error) {
	return s.config.Token(s.ctx)
}

// newIDTokenVerifier discovers the provider (via go-oidc, which itself enforces
// issuer matching) and returns a verifier for its id_tokens. The client-id /
// audience check is delegated to verifyingTokenSource, since for the
// client_credentials grant the id_token audience is frequently the target API
// rather than the client id.
func newIDTokenVerifier(ctx context.Context, cfg Config, httpClient *http.Client) (*oidc.IDTokenVerifier, error) {
	if cfg.Issuer == "" {
		return nil, fmt.Errorf("oidc: oidc_verify_id_token requires oidc_issuer")
	}
	providerCtx := ctx
	if httpClient != nil {
		providerCtx = oidc.ClientContext(ctx, httpClient)
	}
	provider, err := oidc.NewProvider(providerCtx, strings.TrimSuffix(cfg.Issuer, "/"))
	if err != nil {
		return nil, fmt.Errorf("oidc: provider discovery for id_token verification failed: %w", err)
	}
	return provider.Verifier(&oidc.Config{SkipClientIDCheck: true}), nil
}

// verifyingTokenSource verifies the id_token carried by each token its delegate
// produces, failing closed if the id_token is missing, malformed, or does not
// satisfy the configured audience.
type verifyingTokenSource struct {
	source   oauth2.TokenSource
	verifier *oidc.IDTokenVerifier
	audience string
	ctx      context.Context //nolint:containedctx // intentionally long-lived for JWKS fetches
}

func (v *verifyingTokenSource) Token() (*oauth2.Token, error) {
	token, err := v.source.Token()
	if err != nil {
		return nil, err
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, fmt.Errorf("oidc: oidc_verify_id_token is enabled but the token response carries no id_token")
	}
	idToken, verifyErr := v.verifier.Verify(v.ctx, rawIDToken)
	if verifyErr != nil {
		return nil, fmt.Errorf("oidc: id_token verification failed: %w", verifyErr)
	}
	if v.audience != "" {
		satisfied := false
		for _, aud := range idToken.Audience {
			if aud == v.audience {
				satisfied = true
				break
			}
		}
		if !satisfied {
			return nil, fmt.Errorf("oidc: id_token audience %v does not include configured audience %q", idToken.Audience, v.audience)
		}
	}
	return token, nil
}

// FetchToken performs a one-shot client_credentials exchange and returns the
// selected token. Prefer TokenSource + Transport for long-lived clients, which
// refreshes automatically. httpClient may be nil.
func FetchToken(ctx context.Context, cfg Config, httpClient *http.Client) (string, error) {
	tokenSource, err := TokenSource(ctx, cfg, httpClient)
	if err != nil {
		return "", err
	}
	token, err := tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("oidc: token request failed: %w", err)
	}
	return selectToken(token, cfg.TokenType)
}

// selectToken extracts the configured credential — access_token by default, or
// id_token — from an OAuth2 token response.
func selectToken(token *oauth2.Token, tokenType string) (string, error) {
	if tokenType == "" {
		tokenType = TokenTypeAccessToken
	}
	switch tokenType {
	case TokenTypeAccessToken:
		if token.AccessToken == "" {
			return "", fmt.Errorf("oidc: access_token absent from token response")
		}
		return token.AccessToken, nil
	case TokenTypeIDToken:
		idToken, ok := token.Extra("id_token").(string)
		if !ok || idToken == "" {
			return "", fmt.Errorf("oidc: id_token requested but absent from token response")
		}
		return idToken, nil
	default:
		return "", fmt.Errorf("oidc: unsupported oidc_token_type %q", tokenType)
	}
}

// Transport attaches an auto-refreshing OIDC token to every outbound request.
// By default it sets "Authorization: Bearer <token>"; the credential (access
// vs id token), the value prefix, the header/param name, and header-vs-query
// placement are all configurable.
type Transport struct {
	Base        http.RoundTripper
	TokenSource oauth2.TokenSource
	TokenType   string
	Location    string
	Name        string
	ValuePrefix string
}

// RoundTrip retrieves a current token from the source (refreshing it if expired)
// and attaches it to a copy of the request before delegating to the base.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.TokenSource == nil {
		return nil, fmt.Errorf("oidc: transport has no token source")
	}
	token, err := t.TokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("oidc: token retrieval failed: %w", err)
	}
	credential, err := selectToken(token, t.TokenType)
	if err != nil {
		return nil, err
	}

	// Per the RoundTripper contract, do not mutate the caller's request.
	outbound := req.Clone(req.Context())
	location := t.Location
	if location == "" {
		location = LocationHeader
	}
	switch location {
	case LocationQuery:
		name := t.Name
		if name == "" {
			name = defaultQueryName
		}
		query := outbound.URL.Query()
		query.Set(name, credential)
		outbound.URL.RawQuery = query.Encode()
	default:
		name := t.Name
		if name == "" || name == defaultHeaderName {
			prefix := t.ValuePrefix
			if prefix == "" {
				prefix = defaultBearerPrefix
			}
			outbound.Header.Set(defaultHeaderName, prefix+credential)
		} else {
			outbound.Header.Set(name, t.ValuePrefix+credential)
		}
	}

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(outbound)
}
