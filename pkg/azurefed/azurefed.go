// Package azurefed implements Microsoft Entra ID (Azure AD) federated identity
// credentials: it presents a foreign OIDC token as a client_assertion
// (JWT-bearer) to the tenant's OAuth2 token endpoint in place of a client
// secret, and exposes the resulting Entra access token via an auto-refreshing
// oauth2.TokenSource.
package azurefed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	defaultEntraBase    = "https://login.microsoftonline.com"
	clientAssertionType = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
)

// Config describes an Azure federated identity exchange.
type Config struct {
	// TenantID is the Entra tenant GUID or verified domain. Required.
	TenantID string
	// ClientID is the Entra app registration (object) ID. Required.
	ClientID string
	// Scopes target the resource being called (e.g.
	// https://management.azure.com/.default,
	// https://graph.microsoft.com/.default). Required.
	Scopes []string
	// Endpoint optionally overrides the full token endpoint URL, primarily for
	// tests / sovereign clouds / private endpoints. Defaults to
	// https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token.
	Endpoint string
}

// TokenSource returns an auto-refreshing oauth2.TokenSource that calls Entra's
// token endpoint on each refresh with the current foreign subject token as
// client_assertion. ctx should be long-lived; httpClient (may be nil) carries
// TLS/proxy configuration.
func TokenSource(
	ctx context.Context,
	cfg Config,
	getSubjectToken func() (string, error),
	httpClient *http.Client,
) (oauth2.TokenSource, error) {
	if cfg.TenantID == "" {
		return nil, fmt.Errorf("azure federated: tenant ID is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("azure federated: client ID is required")
	}
	if getSubjectToken == nil {
		return nil, fmt.Errorf("azure federated: subject token retriever is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("%s/%s/oauth2/v2.0/token", defaultEntraBase, cfg.TenantID)
	}
	src := &federatedTokenSource{
		ctx:             ctx,
		endpoint:        endpoint,
		clientID:        cfg.ClientID,
		scope:           strings.Join(cfg.Scopes, " "),
		getSubjectToken: getSubjectToken,
		httpClient:      httpClient,
	}
	return oauth2.ReuseTokenSource(nil, src), nil
}

// federatedTokenSource performs the client_assertion exchange. The reuse
// wrapper above caches its result until expiry; on expiry, Token is re-invoked
// and the subject-token retriever is consulted again so platform-rotated
// subject tokens (e.g. projected k8s tokens, GHA tokens) are picked up.
type federatedTokenSource struct {
	ctx             context.Context //nolint:containedctx // long-lived for refreshes
	endpoint        string
	clientID        string
	scope           string
	getSubjectToken func() (string, error)
	httpClient      *http.Client
}

type entraTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

func (s *federatedTokenSource) Token() (*oauth2.Token, error) {
	subjectToken, err := s.getSubjectToken()
	if err != nil {
		return nil, fmt.Errorf("azure federated: subject token: %w", err)
	}
	form := url.Values{
		"grant_type":            {"client_credentials"},
		"client_id":             {s.clientID},
		"client_assertion_type": {clientAssertionType},
		"client_assertion":      {subjectToken},
	}
	if s.scope != "" {
		form.Set("scope", s.scope)
	}

	req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, s.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("azure federated: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure federated: token request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close on read path

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("azure federated: read response: %w", err)
	}
	var parsed entraTokenResponse
	if jsonErr := json.Unmarshal(body, &parsed); jsonErr != nil {
		return nil, fmt.Errorf("azure federated: decode response (status %d): %w", resp.StatusCode, jsonErr)
	}
	if resp.StatusCode != http.StatusOK || parsed.AccessToken == "" {
		if parsed.Error != "" {
			return nil, fmt.Errorf("azure federated: %s: %s", parsed.Error, parsed.ErrorDesc)
		}
		return nil, fmt.Errorf("azure federated: token endpoint returned status %d", resp.StatusCode)
	}
	tok := &oauth2.Token{
		AccessToken: parsed.AccessToken,
		TokenType:   parsed.TokenType,
	}
	if parsed.ExpiresIn > 0 {
		tok.Expiry = time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second)
	}
	return tok, nil
}
