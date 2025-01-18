package auth_util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/stackql/any-sdk/pkg/awssign"
	"github.com/stackql/any-sdk/pkg/azureauth"
	"github.com/stackql/any-sdk/pkg/constants"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/pkg/google_sdk"
	"github.com/stackql/any-sdk/pkg/litetemplate"
	"github.com/stackql/any-sdk/pkg/netutils"

	"net/http"
	"regexp"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
)

const (
	ServiceAccountPathErrStr string = "[ERROR] credentialsfilepath not supplied or key file does not exist."
)

var (
	storageObjectsRegex *regexp.Regexp = regexp.MustCompile(`^storage\.objects\..*$`) //nolint:unused,revive,nolintlint,lll // prefer declarative
)

type serviceAccount struct {
	Email      string `json:"client_email"`
	PrivateKey string `json:"private_key"`
}

type tokenCfg struct {
	token           []byte
	authType        string
	authValuePrefix string
	tokenLocation   string
	key             string
}

func newTokenConfig(
	token []byte,
	authType,
	authValuePrefix,
	tokenLocation,
	key string,
) *tokenCfg {
	return &tokenCfg{
		token:           token,
		authType:        authType,
		authValuePrefix: authValuePrefix,
		tokenLocation:   tokenLocation,
		key:             key,
	}
}

type AssistedTransport interface {
	addTokenCfg(tokenConfig *tokenCfg) error
	RoundTrip(req *http.Request) (*http.Response, error)
}

type AuthUtility interface {
	ActivateAuth(authCtx *dto.AuthCtx, principal string, authType string)
	DeactivateAuth(authCtx *dto.AuthCtx)
	AuthRevoke(authCtx *dto.AuthCtx) error
	ParseServiceAccountFile(ac *dto.AuthCtx) (serviceAccount, error)
	GetGoogleJWTConfig(
		provider string,
		credentialsBytes []byte,
		scopes []string,
		subject string,
	) (*jwt.Config, error)
	GetGenericClientCredentialsConfig(authCtx *dto.AuthCtx, scopes []string) (*clientcredentials.Config, error)
	GoogleOauthServiceAccount(
		provider string,
		authCtx *dto.AuthCtx,
		scopes []string,
		httpContext netutils.HTTPContext,
	) (*http.Client, error)
	GenericOauthClientCredentials(
		authCtx *dto.AuthCtx,
		scopes []string,
		httpContext netutils.HTTPContext,
	) (*http.Client, error)
	ApiTokenAuth(authCtx *dto.AuthCtx, httpContext netutils.HTTPContext, enforceBearer bool) (*http.Client, error)
	AwsSigningAuth(authCtx *dto.AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error)
	BasicAuth(authCtx *dto.AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error)
	CustomAuth(authCtx *dto.AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error)
	AzureDefaultAuth(authCtx *dto.AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error)
	GCloudOAuth(runtimeCtx dto.RuntimeCtx, authCtx *dto.AuthCtx, enforceRevokeFirst bool) (*http.Client, error)
	GetCurrentGCloudOauthUser() ([]byte, error)
}

type authUtil struct {
	// Placeholder for future implementation
}

func NewAuthUtility() AuthUtility {
	return &authUtil{}
}

type transport struct {
	tokenConfigs        []*tokenCfg
	underlyingTransport http.RoundTripper
}

func NewTransport(
	token []byte,
	authType,
	authValuePrefix,
	tokenLocation,
	key string,
	underlyingTransport http.RoundTripper,
) (AssistedTransport, error) {
	return newTransport(token, authType, authValuePrefix, tokenLocation, key, underlyingTransport)
}

func newTransport(
	token []byte,
	authType,
	authValuePrefix,
	tokenLocation,
	key string,
	underlyingTransport http.RoundTripper,
) (AssistedTransport, error) {
	switch authType {
	case AuthTypeBasic, AuthTypeBearer, AuthTypeAPIKey:
		if len(token) < 1 {
			return nil, fmt.Errorf("no credentials provided for auth type = '%s'", authType)
		}
		if tokenLocation != LocationHeader {
			return nil, fmt.Errorf(
				"improper location provided for auth type = '%s', provided = '%s', expected = '%s'",
				authType, tokenLocation, LocationHeader)
		}
	default:
		switch tokenLocation {
		case LocationHeader:
		case LocationQuery:
			if key == "" {
				return nil, fmt.Errorf("key required for query param based auth")
			}
		default:
			return nil, fmt.Errorf("token location not supported: '%s'", tokenLocation)
		}
	}
	tokenConfigObj := newTokenConfig(token, authType, authValuePrefix, tokenLocation, key)
	return &transport{
		tokenConfigs:        []*tokenCfg{tokenConfigObj},
		underlyingTransport: underlyingTransport,
	}, nil
}

//nolint:unparam // future proofing
func (t *transport) addTokenCfg(tokenConfig *tokenCfg) error {
	t.tokenConfigs = append(t.tokenConfigs, tokenConfig)
	return nil
}

const (
	LocationHeader string = "header"
	LocationQuery  string = "query"
	AuthTypeBasic  string = "BASIC"
	AuthTypeCustom string = "custom"
	AuthTypeBearer string = "Bearer"
	AuthTypeAPIKey string = "api_key"
)

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	for _, tc := range t.tokenConfigs {
		tokenConfig := tc
		switch tokenConfig.tokenLocation {
		case LocationHeader:
			switch tokenConfig.authType {
			case AuthTypeBasic, AuthTypeBearer, AuthTypeAPIKey:
				authValuePrefix := tokenConfig.authValuePrefix
				if tokenConfig.authValuePrefix == "" {
					authValuePrefix = fmt.Sprintf("%s ", tokenConfig.authType)
				}
				req.Header.Set(
					"Authorization",
					fmt.Sprintf("%s%s", authValuePrefix, string(tokenConfig.token)),
				)
			default:
				req.Header.Set(
					tokenConfig.key,
					string(tokenConfig.token),
				)
			}
		case LocationQuery:
			qv := req.URL.Query()
			qv.Set(
				tokenConfig.key, string(tokenConfig.token),
			)
			req.URL.RawQuery = qv.Encode()
		}
	}
	return t.underlyingTransport.RoundTrip(req)
}

func (au *authUtil) ActivateAuth(authCtx *dto.AuthCtx, principal string, authType string) {
	authCtx.Active = true
	authCtx.Type = authType
	if principal != "" {
		authCtx.ID = principal
	}
}

func (au *authUtil) GetCurrentGCloudOauthUser() ([]byte, error) {
	return google_sdk.GetAccessToken()
}

func (au *authUtil) DeactivateAuth(authCtx *dto.AuthCtx) {
	authCtx.Active = false
}

func (au *authUtil) ParseServiceAccountFile(ac *dto.AuthCtx) (serviceAccount, error) {
	b, err := ac.GetCredentialsBytes()
	var c serviceAccount
	if err != nil {
		return c, fmt.Errorf(ServiceAccountPathErrStr) //nolint:stylecheck //TODO: review
	}
	return c, json.Unmarshal(b, &c)
}

func (au *authUtil) GetGoogleJWTConfig(
	provider string,
	credentialsBytes []byte,
	scopes []string,
	subject string,
) (*jwt.Config, error) {
	switch provider {
	case "google", "googleads", "googleanalytics",
		"googledevelopers", "googlemybusiness", "googleworkspace",
		"youtube", "googleadmin":
		if scopes == nil {
			scopes = []string{
				"https://www.googleapis.com/auth/cloud-platform",
			}
		}
		rv, err := google.JWTConfigFromJSON(credentialsBytes, scopes...)
		if err != nil {
			return nil, err
		}
		if subject != "" {
			rv.Subject = subject
		}
		return rv, nil
	default:
		return nil, fmt.Errorf("service account auth for provider = '%s' currently not supported", provider)
	}
}

func (au *authUtil) AuthRevoke(authCtx *dto.AuthCtx) error {
	switch strings.ToLower(authCtx.Type) {
	case dto.AuthServiceAccountStr:
		return errors.New(constants.ServiceAccountRevokeErrStr)
	case dto.AuthInteractiveStr:
		err := google_sdk.RevokeGoogleAuth()
		if err == nil {
			au.DeactivateAuth(authCtx)
		}
		return err
	}
	return fmt.Errorf(`Auth revoke for Google Failed; improper auth method: "%s" specified`, authCtx.Type)
}

func (au *authUtil) GCloudOAuth(runtimeCtx dto.RuntimeCtx, authCtx *dto.AuthCtx, enforceRevokeFirst bool) (*http.Client, error) {
	var err error
	var tokenBytes []byte
	tokenBytes, err = google_sdk.GetAccessToken()
	if enforceRevokeFirst && authCtx.Type == dto.AuthInteractiveStr && err == nil {
		return nil, fmt.Errorf(constants.OAuthInteractiveAuthErrStr) //nolint:stylecheck // happy with message
	}
	if err != nil {
		err = google_sdk.OAuthToGoogle()
		if err == nil {
			tokenBytes, err = google_sdk.GetAccessToken()
		}
	}
	if err != nil {
		return nil, err
	}
	au.ActivateAuth(authCtx, "", dto.AuthInteractiveStr)
	client := netutils.GetHTTPClient(runtimeCtx, nil)
	tr, err := NewTransport(tokenBytes, AuthTypeBearer, authCtx.ValuePrefix, LocationHeader, "", client.Transport)
	if err != nil {
		return nil, err
	}
	client.Transport = tr
	return client, nil
}

func (au *authUtil) GetGenericClientCredentialsConfig(authCtx *dto.AuthCtx, scopes []string) (*clientcredentials.Config, error) {
	clientID, clientIDErr := authCtx.GetClientID()
	if clientIDErr != nil {
		return nil, clientIDErr
	}
	clientSecret, secretErr := authCtx.GetClientSecret()
	if secretErr != nil {
		return nil, secretErr
	}
	templatedTokenURL, templateErr := litetemplate.RenderTemplateFromSerializable(authCtx.GetTokenURL(), authCtx)
	if templateErr != nil {
		return nil, fmt.Errorf("incorrect token url templating %w", templateErr)
	}
	rv := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
		TokenURL:     templatedTokenURL,
	}
	if len(authCtx.GetValues()) > 0 {
		rv.EndpointParams = authCtx.GetValues()
	}
	if authCtx.GetAuthStyle() > 0 {
		rv.AuthStyle = oauth2.AuthStyle(authCtx.GetAuthStyle())
	}
	return rv, nil
}

func (au *authUtil) GoogleOauthServiceAccount(
	provider string,
	authCtx *dto.AuthCtx,
	scopes []string,
	httpContext netutils.HTTPContext,
) (*http.Client, error) {
	b, err := authCtx.GetCredentialsBytes()
	if err != nil {
		return nil, fmt.Errorf("service account credentials error: %w", err)
	}
	config, errToken := au.GetGoogleJWTConfig(provider, b, scopes, authCtx.Subject)
	if errToken != nil {
		return nil, errToken
	}
	au.ActivateAuth(authCtx, "", dto.AuthServiceAccountStr)
	httpClient := netutils.GetHTTPClient(httpContext, http.DefaultClient)
	return config.Client(context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)), nil
}

func (au *authUtil) GenericOauthClientCredentials(
	authCtx *dto.AuthCtx,
	scopes []string,
	httpContext netutils.HTTPContext,
) (*http.Client, error) {
	config, errToken := au.GetGenericClientCredentialsConfig(authCtx, scopes)
	if errToken != nil {
		return nil, errToken
	}
	au.ActivateAuth(authCtx, "", dto.ClientCredentialsStr)
	httpClient := netutils.GetHTTPClient(httpContext, http.DefaultClient)
	return config.Client(context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)), nil
}

func (au *authUtil) ApiTokenAuth(authCtx *dto.AuthCtx, httpContext netutils.HTTPContext, enforceBearer bool) (*http.Client, error) {
	b, err := authCtx.GetCredentialsBytes()
	if err != nil {
		return nil, fmt.Errorf("credentials error: %w", err)
	}
	au.ActivateAuth(authCtx, "", "api_key")
	httpClient := netutils.GetHTTPClient(httpContext, http.DefaultClient)
	valPrefix := authCtx.ValuePrefix
	if enforceBearer {
		valPrefix = "Bearer "
	}
	tr, err := newTransport(b, AuthTypeAPIKey, valPrefix, LocationHeader, "", httpClient.Transport)
	if err != nil {
		return nil, err
	}
	httpClient.Transport = tr
	return httpClient, nil
}

func (au *authUtil) AwsSigningAuth(authCtx *dto.AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error) {
	// Retrieve the AWS access key and secret key.
	credentialsBytes, err := authCtx.GetCredentialsBytes()
	if err != nil {
		return nil, fmt.Errorf("credentials error: %w", err)
	}
	keyStr := string(credentialsBytes)

	// Retrieve the AWS access key ID.
	keyID, err := authCtx.GetKeyIDString()
	if err != nil {
		return nil, err
	}

	// Validate that both keyID and keyStr are not empty.
	if keyStr == "" || keyID == "" {
		return nil, fmt.Errorf("cannot compose AWS signing credentials")
	}

	// Retrieve the optional session token. Note: No error handling for missing session token.
	sessionToken, _ := authCtx.GetAwsSessionTokenString()

	// Mark the authentication context as active for AWS signing.
	au.ActivateAuth(authCtx, "", dto.AuthAWSSigningv4Str)

	// Get the HTTP client from the runtime context.
	httpClient := netutils.GetHTTPClient(httpContext, http.DefaultClient)

	// Initialize the AWS signing transport with credentials and optional session token.
	tr, err := awssign.NewAwsSignTransport(httpClient.Transport, keyID, keyStr, sessionToken)
	if err != nil {
		return nil, err
	}

	// Set the custom AWS signing transport as the client's transport.
	httpClient.Transport = tr

	return httpClient, nil
}

func (au *authUtil) BasicAuth(authCtx *dto.AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error) {
	b, err := authCtx.GetCredentialsBytes()
	if err != nil {
		return nil, fmt.Errorf("credentials error: %w", err)
	}
	au.ActivateAuth(authCtx, "", "basic")
	httpClient := netutils.GetHTTPClient(httpContext, http.DefaultClient)
	tr, err := newTransport(b, AuthTypeBasic, authCtx.ValuePrefix, LocationHeader, "", httpClient.Transport)
	if err != nil {
		return nil, err
	}
	httpClient.Transport = tr
	return httpClient, nil
}

func (au *authUtil) CustomAuth(authCtx *dto.AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error) {
	b, err := authCtx.GetCredentialsBytes()
	if err != nil {
		return nil, fmt.Errorf("credentials error: %w", err)
	}
	au.ActivateAuth(authCtx, "", "custom")
	httpClient := netutils.GetHTTPClient(httpContext, http.DefaultClient)
	tr, err := newTransport(b, AuthTypeCustom, authCtx.ValuePrefix, authCtx.Location, authCtx.Name, httpClient.Transport)
	if err != nil {
		return nil, err
	}
	successor, successorExists := authCtx.GetSuccessor()
	for {
		if successorExists {
			successorCredentialsBytes, sbErr := successor.GetCredentialsBytes()
			if sbErr != nil {
				return nil, fmt.Errorf("successor credentials error: %w", sbErr)
			}
			successorTokenConfig := newTokenConfig(
				successorCredentialsBytes,
				AuthTypeCustom,
				successor.ValuePrefix,
				successor.Location,
				successor.Name,
			)
			addTknErr := tr.addTokenCfg(successorTokenConfig)
			if addTknErr != nil {
				return nil, addTknErr
			}
			successor, successorExists = successor.GetSuccessor()
		} else {
			break
		}
	}
	httpClient.Transport = tr
	return httpClient, nil
}

func (au *authUtil) AzureDefaultAuth(authCtx *dto.AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error) {
	azureTokenSource, err := azureauth.NewDefaultCredentialAzureTokenSource()
	if err != nil {
		return nil, fmt.Errorf("azure default credentials error: %w", err)
	}
	token, err := azureTokenSource.GetToken(context.Background())
	if err != nil {
		return nil, fmt.Errorf("azure default credentials token error: %w", err)
	}
	tokenString := token.Token
	au.ActivateAuth(authCtx, "", "azure_default")
	httpClient := netutils.GetHTTPClient(httpContext, http.DefaultClient)
	tr, err := newTransport([]byte(tokenString), AuthTypeBearer, "Bearer ", LocationHeader, "", httpClient.Transport)
	if err != nil {
		return nil, err
	}
	httpClient.Transport = tr
	return httpClient, nil
}
