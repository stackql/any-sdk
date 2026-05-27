package oidcauth

import (
	"fmt"
	"os"
	"strings"
)

// SubjectTokenConfig identifies where the foreign OIDC token used in cloud
// federation (AWS AssumeRoleWithWebIdentity, GCP Workload Identity Federation,
// Azure federated credentials) is read from. Resolution precedence on every
// retrieval: File > FileEnvVar > Inline.
type SubjectTokenConfig struct {
	// File is a literal filesystem path containing the token.
	File string
	// FileEnvVar is the name of an environment variable holding a filesystem
	// path to the token. This is how IRSA (AWS_WEB_IDENTITY_TOKEN_FILE), GitHub
	// Actions (ACTIONS_ID_TOKEN_REQUEST_TOKEN-style), and projected k8s
	// service-account tokens are conventionally surfaced.
	FileEnvVar string
	// Inline is a literal token value. Primarily for tests; production tokens
	// are short-lived and rotated by the platform via file.
	Inline string
}

// SubjectTokenRetriever returns a closure that produces a current token on each
// call. File-backed retrievers re-read the file every invocation so that
// platform-rotated tokens (IRSA, GHA, k8s) are picked up transparently across
// downstream credential refreshes.
func SubjectTokenRetriever(cfg SubjectTokenConfig) (func() (string, error), error) {
	switch {
	case cfg.File != "":
		path := cfg.File
		return func() (string, error) { return readTokenFile(path) }, nil
	case cfg.FileEnvVar != "":
		envVar := cfg.FileEnvVar
		return func() (string, error) {
			path := os.Getenv(envVar)
			if path == "" {
				return "", fmt.Errorf("oidc subject token: env var %q is empty", envVar)
			}
			return readTokenFile(path)
		}, nil
	case cfg.Inline != "":
		token := cfg.Inline
		return func() (string, error) { return token, nil }, nil
	default:
		return nil, fmt.Errorf("oidc subject token: none of oidc_subject_token_file, oidc_subject_token_file_env_var, or oidc_subject_token is set")
	}
}

func readTokenFile(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // user-configured path is the point
	if err != nil {
		return "", fmt.Errorf("oidc subject token: reading %s: %w", path, err)
	}
	// Files typically end in a newline; STS endpoints reject those.
	return strings.TrimSpace(string(b)), nil
}
