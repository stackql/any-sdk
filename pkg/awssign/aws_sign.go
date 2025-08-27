package awssign

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/stackql/any-sdk/pkg/logging"
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
	_ Transport = &standardAwsSignTransport{}
)

type Transport interface {
	RoundTrip(req *http.Request) (*http.Response, error)
}

type standardAwsSignTransport struct {
	underlyingTransport http.RoundTripper
	signer              *v4.Signer
}

func NewAwsSignTransport(
	underlyingTransport http.RoundTripper,
	id, secret, token string,
	options ...func(*v4.Signer),
) (Transport, error) {
	var creds *credentials.Credentials

	if token == "" {
		creds = credentials.NewStaticCredentials(id, secret, token)
	} else {
		defaultAccessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
		defaultSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
		if defaultAccessKeyID == "" || defaultSecretAccessKey == "" {
			return nil, fmt.Errorf("AWS_SESSION_TOKEN is set, but AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must also be set")
		}
		creds = credentials.NewEnvCredentials()
	}

	signer := v4.NewSigner(creds, options...)
	return &standardAwsSignTransport{
		underlyingTransport: underlyingTransport,
		signer:              signer,
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
	var rs io.ReadSeeker
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		rs = bytes.NewReader(body)
		req.Body = nil
	}
	header, err := t.signer.Sign(
		req,
		rs,
		svcStr,
		rgnStr,
		time.Now(),
	)
	logging.GetLogger().Infof("header = %v\n", header)
	if err != nil {
		return nil, err
	}

	return t.underlyingTransport.RoundTrip(req)
}
