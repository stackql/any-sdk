package awssign

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// This interface is not fully compliant.
// Ultimately, for full coverage,
// we need to emulate [the SDK auth specifications](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-auth.html).
//
// This is the sort of stuff we need to emulate:
//   - [Resolver doc based resolution](https://github.com/aws/aws-sdk-go-v2/blob/2e08461090ccba679456c05264e2c04bf228138e/service/accessanalyzer/options.go#L150).
//   - SDK doc based code gen settings for auth:
//       - [For the `account` service](https://github.com/aws/aws-sdk-go-v2/blob/3ac24f20bb3b05955fcb1b3fae7883d3a03fe60d/codegen/sdk-codegen/aws-models/account.json#L133).
//

var (
	_                Transport = &standardAwsSignTransport{}
	emptyPayloadHash string    = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

type Transport interface {
	RoundTrip(req *http.Request) (*http.Response, error)
}

type standardAwsSignTransport struct {
	underlyingTransport http.RoundTripper
	signer              *v4.Signer
	credentialsProvider aws.CredentialsProvider
}

func NewAwsSignTransport(
	underlyingTransport http.RoundTripper,
	id, secret, token string,
	options ...func(*v4.SignerOptions),
) (Transport, error) {
	var creds credentials.StaticCredentialsProvider

	if token == "" {
		creds = credentials.NewStaticCredentialsProvider(id, secret, token)
	} else {
		defaultAccessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
		defaultSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
		if defaultAccessKeyID == "" || defaultSecretAccessKey == "" {
			return nil, fmt.Errorf("AWS_SESSION_TOKEN is set, but AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must also be set")
		}
		creds = credentials.NewStaticCredentialsProvider(defaultAccessKeyID, defaultSecretAccessKey, token)
	}

	signer := v4.NewSigner(options...)
	return &standardAwsSignTransport{
		underlyingTransport: underlyingTransport,
		signer:              signer,
		credentialsProvider: creds,
	}, nil
}

// NewAwsSignTransportWithCredentials builds a signing transport from an explicit
// (id, secret, token) triple, using all three verbatim. Unlike NewAwsSignTransport
// it never falls back to AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY when a session
// token is present, which is required for assumed-role credentials whose id and
// secret are themselves temporary and do not live in the environment.
func NewAwsSignTransportWithCredentials(
	underlyingTransport http.RoundTripper,
	id, secret, token string,
	options ...func(*v4.SignerOptions),
) (Transport, error) {
	if id == "" || secret == "" {
		return nil, fmt.Errorf("cannot compose AWS signing credentials: id and secret are required")
	}
	creds := credentials.NewStaticCredentialsProvider(id, secret, token)
	signer := v4.NewSigner(options...)
	return &standardAwsSignTransport{
		underlyingTransport: underlyingTransport,
		signer:              signer,
		credentialsProvider: creds,
	}, nil
}

// NewAwsSignTransportWithProvider builds a signing transport from any
// aws.CredentialsProvider. Pair this with a refreshing provider (e.g. one wrapped
// by aws.NewCredentialsCache around stscreds.WebIdentityRoleProvider) to get
// transparent credential refresh on each request.
func NewAwsSignTransportWithProvider(
	underlyingTransport http.RoundTripper,
	provider aws.CredentialsProvider,
	options ...func(*v4.SignerOptions),
) (Transport, error) {
	if provider == nil {
		return nil, fmt.Errorf("aws sign: credentials provider is required")
	}
	signer := v4.NewSigner(options...)
	return &standardAwsSignTransport{
		underlyingTransport: underlyingTransport,
		signer:              signer,
		credentialsProvider: provider,
	}, nil
}

func (t *standardAwsSignTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	svc := req.Context().Value("service")
	if svc == nil {
		return nil, fmt.Errorf("AWS service is nil")
	}
	rgn := req.Context().Value("region")
	if rgn == nil {
		return nil, fmt.Errorf("AWS region is nil")
	}
	svcStr, ok := svc.(string)
	if !ok {
		return nil, fmt.Errorf("unsupported type for AWS service: '%T'", svc)
	}
	rgnStr, ok := rgn.(string)
	if !ok {
		return nil, fmt.Errorf("unsupported type for AWS region: '%T'", rgn)
	}
	creds, credsErr := t.credentialsProvider.Retrieve(context.TODO())
	if credsErr != nil {
		return nil, credsErr
	}

	var payloadHash string
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		hashBytes := sha256.Sum256(body)
		// Hex encode the hash
		payloadHash = hex.EncodeToString(hashBytes[:])
		rs := io.NopCloser(bytes.NewReader(body))
		req.Body = rs
	} else {
		payloadHash = emptyPayloadHash
	}
	err := t.signer.SignHTTP(
		context.TODO(),
		creds,
		req,
		payloadHash,
		svcStr,
		rgnStr,
		time.Now(),
	)
	if err != nil {
		return nil, err
	}

	if svcStr == "s3" {
		req.Header.Set("x-amz-content-sha256", payloadHash)

		if strings.HasPrefix(req.URL.Host, "s3.") &&
			strings.Contains(req.URL.Host, "amazonaws.com") &&
			len(req.URL.Path) > 1 {

			req.Close = true
			req.Header.Set("Connection", "close")
		}
	}

	return t.underlyingTransport.RoundTrip(req)
}
