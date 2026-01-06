package latetranslator_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/pkg/latetranslator"
)

func TestTranslate(t *testing.T) {
	translator := latetranslator.NewNaiveLateTranslator()
	legacyDate := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	modernDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)

	tests := []struct {
		name         string
		incomingHost string
		incomingPath string
		wantHost     string
		wantPath     string
		date         string
	}{
		{
			name:         "Canonical Region Dash",
			incomingHost: "my-bucket.s3-us-west-2.amazonaws.com",
			incomingPath: "/photo.jpg",
			wantHost:     "my-bucket.s3-us-west-2.amazonaws.com",
			wantPath:     "/photo.jpg",
			date:         legacyDate,
		},
		{
			name:         "Canonical Region Dot",
			incomingHost: "logs.s3.eu-central-1.amazonaws.com",
			incomingPath: "/",
			wantHost:     "logs.s3.eu-central-1.amazonaws.com",
			wantPath:     "/",
			date:         legacyDate,
		},
		{
			name:         "Dualstack and FIPS",
			incomingHost: "secure-data.s3-dualstack.fips-us-east-1.amazonaws.com",
			incomingPath: "/secret.pdf",
			wantHost:     "secure-data.s3-dualstack.fips-us-east-1.amazonaws.com",
			wantPath:     "/secret.pdf",
			date:         legacyDate,
		},
		{
			name:         "Bucket with Dots (S3 Specific)",
			incomingHost: "my.sub.bucket.s3.us-east-1.amazonaws.com",
			incomingPath: "/index.html",
			wantHost:     "s3.us-east-1.amazonaws.com",
			wantPath:     "/my.sub.bucket/index.html",
			date:         legacyDate,
		},
		{
			name:         "Non-S3 Host (No Mutation)",
			incomingHost: "example.com",
			incomingPath: "/health",
			wantHost:     "example.com",
			wantPath:     "/health",
			date:         modernDate,
		},
		{
			name:         "Already Path Style (No Mutation)",
			incomingHost: "s3.us-west-2.amazonaws.com",
			incomingPath: "/bucket/file",
			wantHost:     "s3.us-west-2.amazonaws.com",
			wantPath:     "/bucket/file",
			date:         modernDate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Host: tt.incomingHost,
				URL: &url.URL{
					Host: tt.incomingHost,
					Path: tt.incomingPath,
				},
			}
			if tt.date != "" {
				ctx := req.Context()
				ctx = context.WithValue(ctx, dto.ContextKeyDate, tt.date)
				req = req.WithContext(ctx)
			}

			translated, err := translator.Translate(req)
			if err != nil {
				t.Fatalf("Translate failed: %v", err)
			}

			if translated.Host != tt.wantHost {
				t.Errorf("Host: got %q, want %q", translated.Host, tt.wantHost)
			}
			if translated.URL.Path != tt.wantPath {
				t.Errorf("Path: got %q, want %q", translated.URL.Path, tt.wantPath)
			}
		})
	}
}
