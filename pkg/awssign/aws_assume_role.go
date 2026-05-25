package awssign

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// AwsTemporaryCredentials holds the short-lived credentials returned by an STS
// AssumeRole call.
type AwsTemporaryCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

// AssumeRoleConfig describes an STS AssumeRole exchange: the base (long-lived)
// credentials that authenticate the AssumeRole call, the target role, and a set
// of optional refinements.
type AssumeRoleConfig struct {
	BaseAccessKeyID     string
	BaseSecretAccessKey string
	BaseSessionToken    string
	RoleARN             string
	RoleSessionName     string
	ExternalID          string
	Region              string
	DurationSeconds     int32
	// Endpoint optionally overrides the STS endpoint. Primarily useful for
	// testing against a mock, or for non-default partitions / private endpoints.
	Endpoint string
	// HTTPClient optionally supplies the HTTP client used for the STS call.
	HTTPClient aws.HTTPClient
}

// AssumeRole exchanges base credentials for temporary credentials scoped to the
// supplied role, via the AWS STS AssumeRole API. The returned credentials are
// suitable for NewAwsSignTransportWithCredentials.
func AssumeRole(ctx context.Context, cfg AssumeRoleConfig) (AwsTemporaryCredentials, error) {
	var rv AwsTemporaryCredentials
	if cfg.RoleARN == "" {
		return rv, fmt.Errorf("aws assume role: role ARN is required")
	}
	if cfg.BaseAccessKeyID == "" || cfg.BaseSecretAccessKey == "" {
		return rv, fmt.Errorf("aws assume role: base credentials are required")
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	awsCfg := aws.Config{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.BaseAccessKeyID, cfg.BaseSecretAccessKey, cfg.BaseSessionToken,
		),
	}
	if cfg.HTTPClient != nil {
		awsCfg.HTTPClient = cfg.HTTPClient
	}
	client := sts.NewFromConfig(awsCfg, func(o *sts.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(cfg.RoleARN),
		RoleSessionName: aws.String(cfg.RoleSessionName),
	}
	if cfg.ExternalID != "" {
		input.ExternalId = aws.String(cfg.ExternalID)
	}
	if cfg.DurationSeconds > 0 {
		input.DurationSeconds = aws.Int32(cfg.DurationSeconds)
	}
	out, err := client.AssumeRole(ctx, input)
	if err != nil {
		return rv, fmt.Errorf("aws assume role: %w", err)
	}
	if out.Credentials == nil ||
		out.Credentials.AccessKeyId == nil ||
		out.Credentials.SecretAccessKey == nil {
		return rv, fmt.Errorf("aws assume role: STS returned incomplete credentials")
	}
	rv.AccessKeyID = aws.ToString(out.Credentials.AccessKeyId)
	rv.SecretAccessKey = aws.ToString(out.Credentials.SecretAccessKey)
	rv.SessionToken = aws.ToString(out.Credentials.SessionToken)
	return rv, nil
}
