package latetranslator

import (
	"net/http"
	"regexp"
	"time"

	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/pkg/s3balancer"
)

var (
	s3VHostPattern string         = `^(.+)\.(s3[.-](?:dualstack\.)?(?:fips-)?(?:[a-z0-9-]+))\.amazonaws\.com$`
	s3VHostRegexp  *regexp.Regexp = regexp.MustCompile(s3VHostPattern)
)

type naiveLateTranslator struct{}

func NewNaiveLateTranslator() *naiveLateTranslator {
	return &naiveLateTranslator{}
}

func (nlt *naiveLateTranslator) requestDate(req *http.Request) time.Time {
	if req == nil {
		return time.Now()
	}
	ctx := req.Context()
	if ctx == nil {
		return time.Now()
	}
	raw := ctx.Value(dto.ContextKeyDate)
	dateStr, ok := raw.(string)
	if !ok || dateStr == "" {
		return time.Now()
	}
	if parsed, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return parsed
	}
	return time.Now()
}

func (nlt *naiveLateTranslator) isS3BucketRequest(req *http.Request) bool {
	hostname := req.URL.Hostname()
	return s3VHostRegexp.MatchString(hostname)
}

func (nlt *naiveLateTranslator) mutateS3BucketRequest(req *http.Request) (*http.Request, error) {
	hostname := req.Host // Use req.Host for client-side reliability
	matches := s3VHostRegexp.FindStringSubmatch(hostname)

	if len(matches) > 2 {
		bucket := matches[1]
		s3Segment := matches[2] // e.g., "s3.us-west-2" or "s3-fips.us-east-1"

		routeStyle := s3balancer.DecideAddressing(
			bucket,
			nlt.requestDate(req),
			false,
		)

		if routeStyle == s3balancer.PathStyle {
			// 1. Change Host to regional base (e.g., s3.us-west-2.amazonaws.com)
			req.URL.Host = s3Segment + ".amazonaws.com"
			req.Host = req.URL.Host

			// 2. Prepend bucket to path
			// Ensure we don't double-slash if Path is already "/"
			if req.URL.Path == "" || req.URL.Path == "/" {
				req.URL.Path = "/" + bucket
			} else {
				req.URL.Path = "/" + bucket + req.URL.Path
			}
		}
	}
	return req, nil
}

func (nlt *naiveLateTranslator) Translate(req *http.Request) (*http.Request, error) {
	isS3BucketRequest := nlt.isS3BucketRequest(req)
	if isS3BucketRequest {
		return nlt.mutateS3BucketRequest(req)
	}
	return req, nil
}
