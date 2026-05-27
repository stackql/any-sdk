package oidcauth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSubjectTokenRetrieverFileReReadsOnEachCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("first\n"), 0o600); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	get, err := SubjectTokenRetriever(SubjectTokenConfig{File: path})
	if err != nil {
		t.Fatalf("SubjectTokenRetriever: %v", err)
	}

	got, err := get()
	if err != nil || got != "first" {
		t.Fatalf("first read = (%q, %v), want (\"first\", nil)", got, err)
	}

	if err := os.WriteFile(path, []byte("second\n"), 0o600); err != nil {
		t.Fatalf("rotate token: %v", err)
	}
	got, err = get()
	if err != nil || got != "second" {
		t.Fatalf("post-rotation read = (%q, %v), want (\"second\", nil) — file must be re-read each call", got, err)
	}
}

func TestSubjectTokenRetrieverFileEnvVar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("env-token"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Setenv("OIDC_TEST_TOKEN_FILE", path)

	get, err := SubjectTokenRetriever(SubjectTokenConfig{FileEnvVar: "OIDC_TEST_TOKEN_FILE"})
	if err != nil {
		t.Fatalf("SubjectTokenRetriever: %v", err)
	}
	got, err := get()
	if err != nil || got != "env-token" {
		t.Errorf("env-var path read = (%q, %v), want (\"env-token\", nil)", got, err)
	}
}

func TestSubjectTokenRetrieverInline(t *testing.T) {
	get, err := SubjectTokenRetriever(SubjectTokenConfig{Inline: "literal"})
	if err != nil {
		t.Fatalf("SubjectTokenRetriever: %v", err)
	}
	got, err := get()
	if err != nil || got != "literal" {
		t.Errorf("inline read = (%q, %v), want (\"literal\", nil)", got, err)
	}
}

func TestSubjectTokenRetrieverRequiresASource(t *testing.T) {
	if _, err := SubjectTokenRetriever(SubjectTokenConfig{}); err == nil {
		t.Error("expected error when no source configured, got nil")
	}
}
