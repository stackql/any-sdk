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

	// HTTPClient is used for both discovery and the token request, carrying TLS
	// and proxy configuration. Defaults to http.DefaultClient when nil.
	HTTPClient *http.Client
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
func resolveTokenEndpoint(ctx context.Context, cfg Config) (string, error) {
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
	md, err := Discover(ctx, discoveryURL, cfg.HTTPClient)
	if err != nil {
		return "", err
	}
	return md.TokenEndpoint, nil
}

// TokenSource builds an auto-refreshing OAuth2 token source for the OIDC
// client_credentials exchange. Endpoint discovery (when required) happens once,
// here; the returned source caches the token and transparently re-fetches it
// against the resolved endpoint whenever it expires. The supplied context
// governs the lifetime of those refreshes, so callers should pass a long-lived
// (e.g. background) context rather than a request-scoped one.
func TokenSource(ctx context.Context, cfg Config) (oauth2.TokenSource, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("oidc: client_id and client_secret are required")
	}
	tokenEndpoint, err := resolveTokenEndpoint(ctx, cfg)
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
	if cfg.HTTPClient != nil {
		tokenCtx = context.WithValue(ctx, oauth2.HTTPClient, cfg.HTTPClient)
	}
	// clientcredentials.Config.TokenSource returns a reusing source that caches
	// the token until expiry and re-fetches afterwards.
	return ccConfig.TokenSource(tokenCtx), nil
}

// FetchToken performs a one-shot client_credentials exchange and returns the
// selected token. Prefer TokenSource + Transport for long-lived clients, which
// refreshes automatically.
func FetchToken(ctx context.Context, cfg Config) (string, error) {
	tokenSource, err := TokenSource(ctx, cfg)
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
