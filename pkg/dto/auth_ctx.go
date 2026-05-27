package dto

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type AuthContexts map[string]*AuthCtx

func (as AuthContexts) Clone() AuthContexts {
	rv := make(AuthContexts)
	for k, v := range as {
		rv[k] = v.Clone()
	}
	return rv
}

type AuthCtx struct {
	Scopes                      []string       `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	SQLCfg                      *SQLBackendCfg `json:"sqlDataSource" yaml:"sqlDataSource"`
	Type                        string         `json:"type" yaml:"type"`
	ValuePrefix                 string         `json:"valuePrefix" yaml:"valuePrefix"`
	ID                          string         `json:"-" yaml:"-"`
	KeyID                       string         `json:"keyID" yaml:"keyID"`
	KeyIDEnvVar                 string         `json:"keyIDenvvar" yaml:"keyIDenvvar"`
	KeyFilePath                 string         `json:"credentialsfilepath" yaml:"credentialsfilepath"`
	KeyFilePathEnvVar           string         `json:"credentialsfilepathenvvar" yaml:"credentialsfilepathenvvar"`
	KeyEnvVar                   string         `json:"credentialsenvvar" yaml:"credentialsenvvar"`
	APIKeyStr                   string         `json:"api_key" yaml:"api_key"`
	APISecretStr                string         `json:"api_secret" yaml:"api_secret"`
	Username                    string         `json:"username" yaml:"username"`
	Password                    string         `json:"password" yaml:"password"`
	EnvVarAPIKeyStr             string         `json:"api_key_var" yaml:"api_key_var"`
	EnvVarAPISecretStr          string         `json:"api_secret_var" yaml:"api_secret_var"`
	EnvVarUsername              string         `json:"username_var" yaml:"username_var"`
	EnvVarPassword              string         `json:"password_var" yaml:"password_var"`
	EncodedBasicCredentials     string         `json:"-" yaml:"-"`
	Successor                   *AuthCtx       `json:"successor" yaml:"successor"`
	Subject                     string         `json:"sub" yaml:"sub"`
	Active                      bool           `json:"-" yaml:"-"`
	Location                    string         `json:"location" yaml:"location"`
	Name                        string         `json:"name" yaml:"name"`
	TokenURL                    string         `json:"token_url" yaml:"token_url"`
	GrantType                   string         `json:"grant_type" yaml:"grant_type"`
	ClientID                    string         `json:"client_id" yaml:"client_id"`
	ClientSecret                string         `json:"client_secret" yaml:"client_secret"`
	ClientIDEnvVar              string         `json:"client_id_env_var" yaml:"client_id_env_var"`
	ClientSecretEnvVar          string         `json:"client_secret_env_var" yaml:"client_secret_env_var"`
	Values                      url.Values     `json:"values" yaml:"values"`
	AuthStyle                   int            `json:"auth_style" yaml:"auth_style"`
	AccountID                   string         `json:"account_id" yaml:"account_id"`
	AccoountIDEnvVar            string         `json:"account_id_env_var" yaml:"account_id_var"`
	AwsRoleArn                  string         `json:"aws_role_arn" yaml:"aws_role_arn"`
	AwsRoleArnEnvVar            string         `json:"aws_role_arn_env_var" yaml:"aws_role_arn_env_var"`
	AwsRoleSessionName          string         `json:"aws_role_session_name" yaml:"aws_role_session_name"`
	AwsRoleExternalID           string         `json:"aws_role_external_id" yaml:"aws_role_external_id"`
	AwsRoleExternalIDEnvVar     string         `json:"aws_role_external_id_env_var" yaml:"aws_role_external_id_env_var"`
	AwsStsRegion                string         `json:"aws_sts_region" yaml:"aws_sts_region"`
	AwsStsEndpoint              string         `json:"aws_sts_endpoint" yaml:"aws_sts_endpoint"`
	AwsRoleDurationSeconds      int32          `json:"aws_role_duration_seconds" yaml:"aws_role_duration_seconds"`
	OIDCIssuer                  string         `json:"oidc_issuer" yaml:"oidc_issuer"`
	OIDCDiscoveryURL            string         `json:"oidc_discovery_url" yaml:"oidc_discovery_url"`
	OIDCTokenType               string         `json:"oidc_token_type" yaml:"oidc_token_type"`
	OIDCAudience                string         `json:"oidc_audience" yaml:"oidc_audience"`
	OIDCSkipIssuerVerification  bool           `json:"oidc_skip_issuer_verification" yaml:"oidc_skip_issuer_verification"`
	OIDCSkipIDTokenVerification bool           `json:"oidc_skip_id_token_verification" yaml:"oidc_skip_id_token_verification"`

	// Shared "subject token" — the foreign OIDC JWT presented to a target cloud's
	// STS / token endpoint for federation. Used by aws_web_identity,
	// gcp_workload_identity, and azure_federated. Resolution order on each
	// refresh: file (literal path) > file (path from env var) > inline. Files are
	// re-read on every retrieval so platform-rotated tokens (IRSA, GHA, k8s
	// projected service-account tokens) refresh transparently.
	OIDCSubjectTokenFile       string `json:"oidc_subject_token_file" yaml:"oidc_subject_token_file"`
	OIDCSubjectTokenFileEnvVar string `json:"oidc_subject_token_file_env_var" yaml:"oidc_subject_token_file_env_var"`
	OIDCSubjectToken           string `json:"oidc_subject_token" yaml:"oidc_subject_token"`

	// GCP Workload Identity Federation.
	GCPWorkloadIdentityAudience         string `json:"gcp_workload_identity_audience" yaml:"gcp_workload_identity_audience"`
	GCPWorkloadIdentitySubjectTokenType string `json:"gcp_workload_identity_subject_token_type" yaml:"gcp_workload_identity_subject_token_type"`
	GCPWorkloadIdentityTokenURL         string `json:"gcp_workload_identity_token_url" yaml:"gcp_workload_identity_token_url"`
	GCPServiceAccountImpersonationURL   string `json:"gcp_service_account_impersonation_url" yaml:"gcp_service_account_impersonation_url"`

	// Azure federated identity credential (workload identity). AzureTenantID
	// drives the Entra token endpoint; ClientID identifies the federated app.
	// The subject token is sent as client_assertion (JWT-bearer) in place of a
	// client secret.
	AzureTenantID       string `json:"azure_tenant_id" yaml:"azure_tenant_id"`
	AzureTenantIDEnvVar string `json:"azure_tenant_id_env_var" yaml:"azure_tenant_id_env_var"`
}

func (ac *AuthCtx) GetSQLCfg() (SQLBackendCfg, bool) {
	var retVal SQLBackendCfg
	if ac.SQLCfg != nil {
		return *ac.SQLCfg, true
	}
	return retVal, false
}

func (ac *AuthCtx) Clone() *AuthCtx {
	var scopesCopy []string
	scopesCopy = append(scopesCopy, ac.Scopes...)
	rv := &AuthCtx{
		Scopes:                              scopesCopy,
		Type:                                ac.Type,
		ValuePrefix:                         ac.ValuePrefix,
		ID:                                  ac.ID,
		KeyID:                               ac.KeyID,
		KeyIDEnvVar:                         ac.KeyIDEnvVar,
		KeyFilePath:                         ac.KeyFilePath,
		KeyFilePathEnvVar:                   ac.KeyFilePathEnvVar,
		KeyEnvVar:                           ac.KeyEnvVar,
		Active:                              ac.Active,
		Username:                            ac.Username,
		Password:                            ac.Password,
		APIKeyStr:                           ac.APIKeyStr,
		APISecretStr:                        ac.APISecretStr,
		EnvVarAPIKeyStr:                     ac.EnvVarAPIKeyStr,
		EnvVarAPISecretStr:                  ac.EnvVarAPISecretStr,
		EnvVarUsername:                      ac.EnvVarUsername,
		EnvVarPassword:                      ac.EnvVarPassword,
		Successor:                           ac.Successor,
		EncodedBasicCredentials:             ac.EncodedBasicCredentials,
		Location:                            ac.Location,
		Name:                                ac.Name,
		Subject:                             ac.Subject,
		TokenURL:                            ac.TokenURL,
		GrantType:                           ac.GrantType,
		ClientID:                            ac.ClientID,
		ClientSecret:                        ac.ClientSecret,
		ClientIDEnvVar:                      ac.ClientIDEnvVar,
		ClientSecretEnvVar:                  ac.ClientSecretEnvVar,
		Values:                              ac.Values,
		AuthStyle:                           ac.AuthStyle,
		AccountID:                           ac.AccountID,
		AccoountIDEnvVar:                    ac.AccoountIDEnvVar,
		AwsRoleArn:                          ac.AwsRoleArn,
		AwsRoleArnEnvVar:                    ac.AwsRoleArnEnvVar,
		AwsRoleSessionName:                  ac.AwsRoleSessionName,
		AwsRoleExternalID:                   ac.AwsRoleExternalID,
		AwsRoleExternalIDEnvVar:             ac.AwsRoleExternalIDEnvVar,
		AwsStsRegion:                        ac.AwsStsRegion,
		AwsStsEndpoint:                      ac.AwsStsEndpoint,
		AwsRoleDurationSeconds:              ac.AwsRoleDurationSeconds,
		OIDCIssuer:                          ac.OIDCIssuer,
		OIDCDiscoveryURL:                    ac.OIDCDiscoveryURL,
		OIDCTokenType:                       ac.OIDCTokenType,
		OIDCAudience:                        ac.OIDCAudience,
		OIDCSkipIssuerVerification:          ac.OIDCSkipIssuerVerification,
		OIDCSkipIDTokenVerification:         ac.OIDCSkipIDTokenVerification,
		OIDCSubjectTokenFile:                ac.OIDCSubjectTokenFile,
		OIDCSubjectTokenFileEnvVar:          ac.OIDCSubjectTokenFileEnvVar,
		OIDCSubjectToken:                    ac.OIDCSubjectToken,
		GCPWorkloadIdentityAudience:         ac.GCPWorkloadIdentityAudience,
		GCPWorkloadIdentitySubjectTokenType: ac.GCPWorkloadIdentitySubjectTokenType,
		GCPWorkloadIdentityTokenURL:         ac.GCPWorkloadIdentityTokenURL,
		GCPServiceAccountImpersonationURL:   ac.GCPServiceAccountImpersonationURL,
		AzureTenantID:                       ac.AzureTenantID,
		AzureTenantIDEnvVar:                 ac.AzureTenantIDEnvVar,
	}
	return rv
}

func (ac *AuthCtx) GetValues() url.Values {
	if ac.Values == nil {
		return url.Values{}
	}
	return ac.Values
}

func (ac *AuthCtx) GetSuccessor() (*AuthCtx, bool) {
	if ac.Successor != nil {
		return ac.Successor, true
	}
	return nil, false
}

func (ac *AuthCtx) GetInlineBasicCredentials() string {
	if ac.Username != "" && ac.Password != "" {
		plaintext := fmt.Sprintf("%s:%s", ac.Username, ac.Password)
		encoded := base64.StdEncoding.EncodeToString([]byte(plaintext))
		return encoded
	}
	if ac.APIKeyStr != "" && ac.APISecretStr != "" {
		plaintext := fmt.Sprintf("%s:%s", ac.APIKeyStr, ac.APISecretStr)
		encoded := base64.StdEncoding.EncodeToString([]byte(plaintext))
		return encoded
	}
	return ""
}

func (ac *AuthCtx) getEnvVarBasicCredentials() string {
	if ac.EnvVarUsername != "" && ac.EnvVarPassword != "" {
		userName := os.Getenv(ac.EnvVarUsername)
		passWord := os.Getenv(ac.EnvVarPassword)
		plaintext := fmt.Sprintf("%s:%s", userName, passWord)
		encoded := base64.StdEncoding.EncodeToString([]byte(plaintext))
		return encoded
	}
	if ac.EnvVarAPIKeyStr != "" && ac.EnvVarAPISecretStr != "" {
		userName := os.Getenv(ac.EnvVarAPIKeyStr)
		passWord := os.Getenv(ac.EnvVarAPISecretStr)
		plaintext := fmt.Sprintf("%s:%s", userName, passWord)
		encoded := base64.StdEncoding.EncodeToString([]byte(plaintext))
		return encoded
	}
	return ""
}

func (ac *AuthCtx) HasKey() bool {
	if ac.KeyFilePath != "" || ac.KeyEnvVar != "" {
		return true
	}
	return false
}

func (ac *AuthCtx) GetKeyIDString() (string, error) {
	if ac.KeyIDEnvVar != "" {
		rv := os.Getenv(ac.KeyIDEnvVar)
		if rv == "" {
			return "", fmt.Errorf("keyIDenvvar references empty string")
		}
		return rv, nil
	}
	return ac.KeyID, nil
}

func (ac *AuthCtx) GetAwsSessionTokenString() (string, error) {
	token := os.Getenv("AWS_SESSION_TOKEN")
	return token, nil // Session token is optional, so an empty token isn't considered an error.
}

// GetAwsRoleArn resolves the ARN of the role to assume, preferring the
// environment variable indirection when supplied. The role ARN is mandatory
// for the aws_assume_role auth type.
func (ac *AuthCtx) GetAwsRoleArn() (string, error) {
	if ac.AwsRoleArnEnvVar != "" {
		rv := os.Getenv(ac.AwsRoleArnEnvVar)
		if rv == "" {
			return "", fmt.Errorf("aws_role_arn_env_var references empty string")
		}
		return rv, nil
	}
	if ac.AwsRoleArn == "" {
		return "", fmt.Errorf("aws_role_arn is empty")
	}
	return ac.AwsRoleArn, nil
}

// GetAwsRoleSessionName returns the configured STS session name, falling back to
// a deterministic default when none is supplied. AWS requires a session name on
// every AssumeRole call.
func (ac *AuthCtx) GetAwsRoleSessionName() string {
	if ac.AwsRoleSessionName != "" {
		return ac.AwsRoleSessionName
	}
	return "stackql-assume-role-session"
}

// GetAwsRoleExternalID resolves the optional STS external ID, preferring the
// environment variable indirection when supplied. An empty result is valid.
func (ac *AuthCtx) GetAwsRoleExternalID() string {
	if ac.AwsRoleExternalIDEnvVar != "" {
		return os.Getenv(ac.AwsRoleExternalIDEnvVar)
	}
	return ac.AwsRoleExternalID
}

// GetAwsStsRegion returns the region used to reach the STS endpoint when
// assuming a role, defaulting to us-east-1. This is independent of the region
// used to sign the eventual service request.
func (ac *AuthCtx) GetAwsStsRegion() string {
	if ac.AwsStsRegion != "" {
		return ac.AwsStsRegion
	}
	return "us-east-1"
}

func (ac *AuthCtx) InferAuthType(authTypeRequested string) string {
	ft := strings.ToLower(authTypeRequested)
	switch ft {
	case AuthAPIKeyStr:
		return AuthAPIKeyStr
	case AuthServiceAccountStr:
		return AuthServiceAccountStr
	case AuthInteractiveStr:
		return AuthInteractiveStr
	}
	if ac.KeyFilePath != "" || ac.KeyEnvVar != "" || ac.KeyFilePathEnvVar != "" {
		return AuthServiceAccountStr
	}
	return AuthInteractiveStr
}

func (ac *AuthCtx) GetCredentialsBytes() ([]byte, error) {
	if ac.KeyEnvVar != "" {
		rv := os.Getenv(ac.KeyEnvVar)
		if rv == "" {
			return nil, fmt.Errorf("credentialsenvvar references empty string")
		}
		return []byte(rv), nil
	}
	if ac.KeyFilePathEnvVar != "" {
		credentialFile := os.Getenv(ac.KeyFilePathEnvVar)
		return os.ReadFile(credentialFile)
	}
	credentialFile := ac.KeyFilePath
	if credentialFile != "" {
		return os.ReadFile(credentialFile)
	}
	if ac.getEnvVarBasicCredentials() != "" {
		return []byte(ac.getEnvVarBasicCredentials()), nil
	}
	if ac.GetInlineBasicCredentials() != "" {
		return []byte(ac.GetInlineBasicCredentials()), nil
	}
	if ac.EncodedBasicCredentials != "" {
		return []byte(ac.EncodedBasicCredentials), nil
	}
	return nil, fmt.Errorf("no credentials found")
}

func (ac *AuthCtx) GetClientID() (string, error) {
	if ac.ClientIDEnvVar != "" {
		rv := os.Getenv(ac.ClientIDEnvVar)
		if rv == "" {
			return "", fmt.Errorf("client_id_env_var references empty string")
		}
		return rv, nil
	}
	if ac.ClientID == "" {
		return "", fmt.Errorf("client_id is empty")
	}
	return ac.ClientID, nil
}

func (ac *AuthCtx) GetClientSecret() (string, error) {
	if ac.ClientSecretEnvVar != "" {
		rv := os.Getenv(ac.ClientSecretEnvVar)
		if rv == "" {
			return "", fmt.Errorf("client_secret_env_var references empty string")
		}
		return rv, nil
	}
	if ac.ClientSecret == "" {
		return "", fmt.Errorf("client_secret is empty")
	}
	return ac.ClientSecret, nil
}

func (ac *AuthCtx) GetGrantType() string {
	return ac.GrantType
}

func (ac *AuthCtx) GetTokenURL() string {
	return ac.TokenURL
}

func (ac *AuthCtx) GetAuthStyle() int {
	return ac.AuthStyle
}

func (ac *AuthCtx) GetCredentialsSourceDescriptorString() string {
	if ac.KeyEnvVar != "" {
		return fmt.Sprintf("credentialsenvvar:%s", ac.KeyEnvVar)
	}
	return fmt.Sprintf("credentialsfilepath:%s", ac.KeyFilePath)
}

func GetAuthCtx(scopes []string, keyFilePath string, keyFileType string) *AuthCtx {
	var authType string
	if keyFilePath == "" {
		authType = AuthInteractiveStr
	} else {
		authType = inferKeyFileType(keyFileType)
	}
	return &AuthCtx{
		Scopes:      scopes,
		Type:        authType,
		KeyFilePath: keyFilePath,
		Active:      false,
	}
}
