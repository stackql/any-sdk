package s3balancer

import (
	"strings"
	"time"
)

// S3Addressing represents the forced invariant for the request URL structure.
type S3Addressing int

const (
	VirtualHost S3Addressing = iota
	PathStyle
)

var (
	// Invariant: Buckets created after this date PHYSICALLY CANNOT use PathStyle.
	// S3 returns 403 Forbidden for path-style requests on these buckets.
	PathStyleCutoff = time.Date(2020, time.September, 30, 0, 0, 0, 0, time.UTC)
)

// DecideAddressing strictly determines the required S3 addressing style.
// entropy is zero; logic is dictated by AWS infrastructure invariants.
func DecideAddressing(bucketName string, created time.Time, forcePathStyle bool) S3Addressing {
	// Invariant 1: ForcePathStyle is a user-level override.
	// Logic check: Only allowed if bucket was created BEFORE the cutoff.
	if forcePathStyle && created.Before(PathStyleCutoff) {
		return PathStyle
	}

	// Invariant 2: Modern Buckets (Post-2020)
	// AWS mandates VirtualHost. There is no fallback.
	if created.After(PathStyleCutoff) || created.Equal(PathStyleCutoff) {
		return VirtualHost
	}

	// Invariant 3: DNS Compatibility
	// If a name contains dots, standard SDKs prefer PathStyle for TLS safety
	// on legacy buckets. Since we are here, we know the bucket is Legacy (<2020).
	if strings.Contains(bucketName, ".") {
		return PathStyle
	}

	// Default for legacy DNS-compliant buckets is VirtualHost.
	return VirtualHost
}
