package awssign

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// AwsWebIdentityConfig describes an STS AssumeRoleWithWebIdentity exchange:
// the target role, optional refinements, and the STS endpoint/transport.
type AwsWebIdentityConfig struct {
	RoleARN         string
	RoleSessionName string
	DurationSeconds int32
	Region          string
	// Endpoint optionally overrides the STS endpoint (testing, private endpoints,
	// non-default partitions).
	Endpoint string
	// HTTPClient optionally supplies the HTTP client used by STS.
	HTTPClient aws.HTTPClient
}

// subjectTokenRetriever adapts a "give me a fresh subject token" closure to the
// stscreds.IdentityTokenRetriever interface. It is invoked on every credential
// refresh, so file-backed retrievers transparently pick up rotated tokens.
type subjectTokenRetriever func() (string, error)

func (f subjectTokenRetriever) GetIdentityToken() ([]byte, error) {
	s, err := f()
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}

// NewWebIdentityRoleProvider builds an auto-refreshing AWS credentials provider
// that exchanges a foreign OIDC token at STS for temporary role credentials.
// The returned provider is wrapped in aws.NewCredentialsCache, so callers get
// caching with expiry-aware refresh out of the box.
func NewWebIdentityRoleProvider(
	cfg AwsWebIdentityConfig,
	getSubjectToken func() (string, error),
) (aws.CredentialsProvider, error) {
	if cfg.RoleARN == "" {
		return nil, fmt.Errorf("aws web identity: role ARN is required")
	}
	if getSubjectToken == nil {
		return nil, fmt.Errorf("aws web identity: subject token retriever is required")
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	awsCfg := aws.Config{Region: region}
	if cfg.HTTPClient != nil {
		awsCfg.HTTPClient = cfg.HTTPClient
	}
	stsClient := sts.NewFromConfig(awsCfg, func(o *sts.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	sessionName := cfg.RoleSessionName
	if sessionName == "" {
		sessionName = "stackql-web-identity-session"
	}
	provider := stscreds.NewWebIdentityRoleProvider(
		stsClient,
		cfg.RoleARN,
		subjectTokenRetriever(getSubjectToken),
		func(o *stscreds.WebIdentityRoleOptions) {
			o.RoleSessionName = sessionName
			if cfg.DurationSeconds > 0 {
				o.Duration = time.Duration(cfg.DurationSeconds) * time.Second
			}
		},
	)
	return aws.NewCredentialsCache(provider), nil
}
