package s3balancer_test

import (
	"testing"
	"time"

	// Import the package under test
	"github.com/stackql/any-sdk/pkg/s3balancer"
)

func TestDecideAddressing_Invariants(t *testing.T) {
	// 2026 Contextual Dates
	legacyDate := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	modernDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		bucket    string
		created   time.Time
		forcePath bool
		want      s3balancer.S3Addressing
	}{
		{
			name:      "Modern buckets are strictly VirtualHost (Post-2020 Invariant)",
			bucket:    "new-bucket.2026",
			created:   modernDate,
			forcePath: false,
			want:      s3balancer.VirtualHost,
		},
		{
			name:      "Modern buckets ignore forcePathStyle (Infrastructure Restriction)",
			bucket:    "modern-bucket",
			created:   modernDate,
			forcePath: true, // This is a 'rubbish' configuration for modern S3
			want:      s3balancer.VirtualHost,
		},
		{
			name:      "Legacy dotted buckets default to PathStyle (SSL Safety)",
			bucket:    "old.dotted.bucket",
			created:   legacyDate,
			forcePath: false,
			want:      s3balancer.PathStyle,
		},
		{
			name:      "Legacy clean buckets default to VirtualHost",
			bucket:    "old-clean-bucket",
			created:   legacyDate,
			forcePath: false,
			want:      s3balancer.VirtualHost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s3balancer.DecideAddressing(tt.bucket, tt.created, tt.forcePath)
			if got != tt.want {
				t.Errorf("FAIL [%s]: got %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
