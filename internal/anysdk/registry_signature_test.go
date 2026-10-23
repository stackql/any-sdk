package anysdk_test

import (
	"bytes"
	"crypto/mldsa"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stackql/any-sdk/internal/anysdk"
	"github.com/stackql/stackql-provider-registry/signing/pqsig"
)

func TestRegistryMLDSASignatures(t *testing.T) {
	key, err := mldsa.GenerateKey(mldsa.MLDSA65())
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{"test": hex.EncodeToString(key.PublicKey().Bytes())}
	source := []byte("name: example\n")
	signed, err := pqsig.Sign(source, "test", key)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"file", "cache", "remote"} {
		for _, tc := range []struct {
			name      string
			doc       []byte
			nop       bool
			wantError bool
		}{
			{"valid", signed, false, false},
			{"tampered", bytes.Replace(signed, []byte("name: example"), []byte("name: changed"), 1), false, true},
			{"unknown", bytes.Replace(signed, []byte("kid: test"), []byte("kid: unknown"), 1), false, true},
			{"unsigned", source, false, true},
			{"opt-out", source, true, false},
			{"opt-out-tampered", []byte("x-stackql-signature: invalid\n"), true, false},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				const docPath = "example/v1/provider.yaml"
				root := t.TempDir()
				localPath := filepath.Join(root, "src", filepath.FromSlash(docPath))
				if err := os.MkdirAll(filepath.Dir(localPath), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(localPath, tc.doc, 0600); err != nil {
					t.Fatal(err)
				}
				// A sidecar must neither rescue an invalid embedded signature nor be fetched.
				if err := os.WriteFile(localPath+".sig", []byte("legacy"), 0600); err != nil {
					t.Fatal(err)
				}
				requests := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					if r.URL.Path != "/src/"+docPath {
						t.Errorf("unexpected request: %s", r.URL.Path)
					}
					_, _ = w.Write(tc.doc)
				}))
				defer server.Close()
				cfg := anysdk.RegistryConfig{
					RegistryURL: server.URL, AllowSrcDownload: true,
					VerifyConfig: &pqsig.VerifierConfig{NopVerify: tc.nop, PublicKeys: keys},
				}
				switch mode {
				case "file":
					p := filepath.ToSlash(root)
					if !strings.HasPrefix(p, "/") {
						p = "/" + p
					}
					cfg.RegistryURL = (&url.URL{Scheme: "file", Path: p}).String()
				case "cache":
					cfg.LocalDocRoot = root
				}
				reg, err := anysdk.NewRegistry(cfg, nil)
				if err != nil {
					t.Fatal(err)
				}
				got, err := reg.GetDocBytes(docPath)
				if (err != nil) != tc.wantError {
					t.Fatalf("GetDocBytes: %s, %v", got, err)
				}
				if err != nil && !strings.Contains(err.Error(), docPath) {
					t.Fatalf("error omitted document path: %v", err)
				}
				if !tc.wantError && !tc.nop && !bytes.Equal(got, source) {
					t.Fatalf("expected exact authenticated payload, got %s", got)
				}
				if tc.nop && !bytes.Equal(got, tc.doc) {
					t.Fatal("opt-out changed the document bytes")
				}
				wantRequests := 0
				if mode == "remote" {
					wantRequests = 1
				}
				if requests != wantRequests {
					t.Fatalf("made %d HTTP requests; want %d", requests, wantRequests)
				}
			})
		}
	}
}
