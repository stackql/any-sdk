package authsurface

import (
	"net/url"
)

type AuthDTO interface {
	JSONLookup(token string) (interface{}, error)
	GetInlineBasicCredentials() string
	GetType() string
	GetKeyID() string
	GetKeyIDEnvVar() string
	GetKeyFilePath() string
	GetKeyFilePathEnvVar() string
	GetKeyEnvVar() string
	GetScopes() []string
	GetValuePrefix() string
	GetEnvVarUsername() string
	GetEnvVarPassword() string
	GetEnvVarAPIKeyStr() string
	GetEnvVarAPISecretStr() string
	GetSuccessor() (AuthDTO, bool)
	GetLocation() string
	GetSubject() string
	GetName() string
	GetClientID() string
	GetClientIDEnvVar() string
	GetClientSecret() string
	GetClientSecretEnvVar() string
	GetTokenURL() string
	GetGrantType() string
	GetValues() url.Values
	GetAuthStyle() int
	GetAccountID() string
	GetAccountIDEnvVar() string
}
