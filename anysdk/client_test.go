package anysdk_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/sirupsen/logrus"
	. "github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/pkg/fileutil"

	"gotest.tools/assert"

	"github.com/stackql/any-sdk/pkg/local_template_executor"
)

var (
	testRoot string
)

func init() {
	var err error
	OpenapiFileRoot, err = fileutil.GetFilePathFromRepositoryRoot("test/registry/src")
	if err != nil {
		os.Exit(1)
	}
	testRoot, err = fileutil.GetFilePathFromRepositoryRoot("test")
	if err != nil {
		os.Exit(1)
	}
}

func TestLocalTemplateClient(t *testing.T) {
	providerPath := path.Join(OpenapiFileRoot, "local_openssl", "v0.1.0", "provider.yaml")
	servicePath := path.Join(OpenapiFileRoot, "local_openssl", "v0.1.0", "services", "keys.yaml")
	pb, err := os.ReadFile(providerPath)
	if err != nil {
		t.Fatalf("error loading provider doc: %v", err)
	}
	prov, err := LoadProviderDocFromBytes(pb)
	if err != nil {
		t.Fatalf("error loading provider doc: %v", err)
	}
	assert.Assert(t, prov != nil)
	svc, err := LoadProviderAndServiceFromPaths(providerPath, servicePath)
	if err != nil {
		t.Fatalf("error loading service: %v", err)
	}
	res, err := svc.GetResource("rsa")
	if err != nil {
		t.Fatalf("error loading resource: %v", err)
	}
	opStore, err := res.FindMethod("create_key_pair")
	if err != nil {
		t.Fatalf("error loading method: %v", err)
	}
	assert.Assert(t, opStore != nil)
	args := opStore.GetInline()
	if len(args) == 0 {
		t.Fatalf("no args found")
	}
	executor := local_template_executor.NewLocalTemplateExecutor(args[0], args[1:], nil)
	resp, err := executor.Execute(map[string]any{
		"parameters": map[string]any{
			"config_file":   path.Join(testRoot, "openssl/openssl.cnf"),
			"key_out_file":  path.Join(testRoot, "tmp/key.pem"),
			"cert_out_file": path.Join(testRoot, "tmp/cert.pem"),
			"days":          90,
		},
	})
	if err != nil {
		t.Fatalf("error executing command: %v", err)
	}
	stdOut, ok := resp.GetStdOut()
	if !ok {
		t.Fatalf("no stdout")
	}
	t.Logf("stdout: %s", stdOut.String())
}

func TestAwsS3BucketABACRequestBodyOverride(t *testing.T) {

	vr := "v0.1.0"
	pb, err := os.ReadFile("./testdata/registry/src/aws/" + vr + "/provider.yaml")
	if err != nil {
		t.Fatalf("Test failed: could not read provider doc, error: %v", err)
	}
	prov, provErr := LoadProviderDocFromBytes(pb)
	if provErr != nil {
		t.Fatalf("Test failed: could not load provider doc, error: %v", provErr)
	}
	svc, err := LoadProviderAndServiceFromPaths(
		"./testdata/registry/src/aws/"+vr+"/provider.yaml",
		"./testdata/registry/src/aws/"+vr+"/services/s3.yaml",
	)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	rsc, rscErr := svc.GetResource("bucket_abac")
	if rscErr != nil {
		t.Fatalf("Test failed: could not locate resource bucket_abac, error: %v", rscErr)
	}
	method, methodErr := rsc.FindMethod("put_bucket_abac")
	if methodErr != nil {
		t.Fatalf("Test failed: could not locate method put_bucket_abac, error: %v", methodErr)
	}
	expectedRequest, hasRequest := method.GetRequest()
	if !hasRequest || expectedRequest == nil {
		t.Fatalf("Test failed: expected request is nil")
	}

	transform, hasTransform := expectedRequest.GetTransform()
	if !hasTransform || transform == nil {
		t.Fatalf("Test failed: expected transform is nil")
	}
	schema := expectedRequest.GetSchema()

	schemaDescription := schema.GetDescription()

	assert.Equal(t, schemaDescription, "A convenience for presentation")

	assert.Assert(t, schema != nil, "expected schema to be non-nil")
	props, _ := schema.GetProperties()
	assert.Assert(t, props != nil, "expected schema properties to be non-nil")
	_, hasLineItem := props["line_items"]
	_, hasStatus := props["status"]
	assert.Assert(t, hasLineItem, "expected schema to have 'line_items' property")
	assert.Assert(t, hasStatus, "expected schema to have 'status' property")

	finalSchema := expectedRequest.GetFinalSchema()
	assert.Assert(t, finalSchema != nil, "expected final schema to be non-nil")
	finalProps, _ := finalSchema.GetProperties()
	assert.Assert(t, len(finalProps) != 0, "expected final schema properties to be non-empty")
	_, finalHasStatus := finalProps["Status"]
	assert.Assert(t, finalHasStatus, "expected final schema to have 'Status' property")

	finalDescription := finalSchema.GetDescription()
	assert.Equal(t, finalDescription, "The ABAC status of the general purpose bucket. When ABAC is enabled for the general purpose bucket, you can use tags to manage access to the general purpose buckets as well as for cost tracking purposes. When ABAC is disabled for the general purpose buckets, you can only use tags for cost tracking purposes. For more information, see [Using tags with S3 general purpose buckets](https://docs.aws.amazon.com/AmazonS3/latest/userguide/buckets-tagging.html).")

	// Body media type should inherit from final schema and not override schema
	assert.Equal(t, expectedRequest.GetBodyMediaType(), "application/xml")

	assert.Equal(t, svc.GetName(), "s3")

	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(tlsServer.Close)

	baseTransport := tlsServer.Client().Transport.(*http.Transport)
	dummyClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   baseTransport.TLSClientConfig,
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("tcp", tlsServer.Listener.Addr().String())
			},
		},
	}

	authCtx := dto.GetAuthCtx([]string{}, "./testdata/dummy_credentials/dummy-sa-key.json", "null_auth")

	configurator := NewAnySdkClientConfigurator(
		dto.RuntimeCtx{
			AllowInsecure: true,
		},
		"aws",
		dummyClient,
	)
	// client, clientErr := configurator.Auth(
	// 	authCtx,
	// 	dto.AuthNullStr,
	// 	false,
	// )
	// if clientErr != nil {
	// 	t.Fatalf("Test failed: could not create client, error: %v", clientErr)
	// }
	// if client == nil {
	// 	t.Fatalf("Test failed: client is nil")
	// }
	httpPreparator := NewHTTPPreparator(
		prov,
		svc,
		method,
		map[int]map[string]interface{}{
			0: {
				"Bucket": "my-test-bucket",
				"Status": "Enabled",
			},
		},
		nil,
		nil,
		logrus.StandardLogger(),
	)
	armoury, armouryErr := httpPreparator.BuildHTTPRequestCtx(NewHTTPPreparatorConfig(false))
	if armouryErr != nil {
		t.Fatalf("Test failed: could not build HTTP preparator armoury, error: %v", armouryErr)
	}
	reqParams := armoury.GetRequestParams()
	if len(reqParams) < 1 {
		t.Fatalf("Test failed: no request parameters found")
	}

	for _, v := range reqParams {

		argList := v.GetArgList()

		// response, apiErr := CallFromSignature(
		// 		cc, payload.rtCtx, authCtx, authCtx.Type, false, os.Stderr, prov, anysdk.NewAnySdkOpStoreDesignation(opStore), argList)

		response, apiErr := CallFromSignature(
			configurator,
			dto.RuntimeCtx{
				AllowInsecure: true,
			},
			authCtx,
			authCtx.Type,
			false,
			nil,
			prov,
			NewAnySdkOpStoreDesignation(method),
			argList, // TODO: abstract
		)
		if apiErr != nil {
			t.Fatalf("Test failed: API call error: %v", apiErr)
		}
		if response.IsErroneous() {
			t.Fatalf("Test failed: API call returned erroneous response")
		}
		t.Logf("Test passed: received response: %+v", response)
	}

}

func TestAwsS3BucketAclsGet(t *testing.T) {

	vr := "v0.1.0"
	pb, err := os.ReadFile("./testdata/registry/src/aws/" + vr + "/provider.yaml")
	if err != nil {
		t.Fatalf("Test failed: could not read provider doc, error: %v", err)
	}
	prov, provErr := LoadProviderDocFromBytes(pb)
	if provErr != nil {
		t.Fatalf("Test failed: could not load provider doc, error: %v", provErr)
	}
	svc, err := LoadProviderAndServiceFromPaths(
		"./testdata/registry/src/aws/"+vr+"/provider.yaml",
		"./testdata/registry/src/aws/"+vr+"/services/s3.yaml",
	)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	rsc, rscErr := svc.GetResource("bucket_acls")
	if rscErr != nil {
		t.Fatalf("Test failed: could not locate resource bucket_acls, error: %v", rscErr)
	}
	method, methodErr := rsc.FindMethod("get_bucket_acl")
	if methodErr != nil {
		t.Fatalf("Test failed: could not locate method get_bucket_acl, error: %v", methodErr)
	}
	// expectedRequest, hasRequest := method.GetRequest()
	// if !hasRequest || expectedRequest == nil {
	// 	t.Fatalf("Test failed: expected request is nil")
	// }

	// transform, hasTransform := expectedRequest.GetTransform()
	// if !hasTransform || transform == nil {
	// 	t.Fatalf("Test failed: expected transform is nil")
	// }
	// schema := expectedRequest.GetSchema()

	// schemaDescription := schema.GetDescription()

	// assert.Equal(t, schemaDescription, "A convenience for presentation")

	// assert.Assert(t, schema != nil, "expected schema to be non-nil")
	// props, _ := schema.GetProperties()
	// assert.Assert(t, props != nil, "expected schema properties to be non-nil")
	// _, hasLineItem := props["line_items"]
	// _, hasStatus := props["status"]
	// assert.Assert(t, hasLineItem, "expected schema to have 'line_items' property")
	// assert.Assert(t, hasStatus, "expected schema to have 'status' property")

	// finalSchema := expectedRequest.GetFinalSchema()
	// assert.Assert(t, finalSchema != nil, "expected final schema to be non-nil")
	// finalProps, _ := finalSchema.GetProperties()
	// assert.Assert(t, len(finalProps) != 0, "expected final schema properties to be non-empty")
	// _, finalHasStatus := finalProps["Status"]
	// assert.Assert(t, finalHasStatus, "expected final schema to have 'Status' property")

	// finalDescription := finalSchema.GetDescription()
	// assert.Equal(t, finalDescription, "The ABAC status of the general purpose bucket. When ABAC is enabled for the general purpose bucket, you can use tags to manage access to the general purpose buckets as well as for cost tracking purposes. When ABAC is disabled for the general purpose buckets, you can only use tags for cost tracking purposes. For more information, see [Using tags with S3 general purpose buckets](https://docs.aws.amazon.com/AmazonS3/latest/userguide/buckets-tagging.html).")

	// assert.Equal(t, expectedRequest.GetBodyMediaType(), "application/xml")

	assert.Equal(t, svc.GetName(), "s3")

	expectedHost := "stackql-trial-bucket-02.s3-us-east-1.amazonaws.com"

	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		io.Copy(io.Discard, r.Body)
		assert.Equal(t, r.Host, expectedHost, "expected host does not match actual host")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(tlsServer.Close)

	baseTransport := tlsServer.Client().Transport.(*http.Transport)
	dummyClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   baseTransport.TLSClientConfig,
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("tcp", tlsServer.Listener.Addr().String())
			},
		},
	}

	authCtx := dto.GetAuthCtx([]string{}, "./testdata/dummy_credentials/dummy-sa-key.json", "null_auth")

	configurator := NewAnySdkClientConfigurator(
		dto.RuntimeCtx{
			AllowInsecure: true,
		},
		"aws",
		dummyClient,
	)
	// client, clientErr := configurator.Auth(
	// 	authCtx,
	// 	dto.AuthNullStr,
	// 	false,
	// )
	// if clientErr != nil {
	// 	t.Fatalf("Test failed: could not create client, error: %v", clientErr)
	// }
	// if client == nil {
	// 	t.Fatalf("Test failed: client is nil")
	// }
	httpPreparator := NewHTTPPreparator(
		prov,
		svc,
		method,
		map[int]map[string]interface{}{
			0: {
				"Bucket":       "my-test-bucket",
				"created_date": "2024-01-01T00:00:00Z",
			},
		},
		nil,
		nil,
		logrus.StandardLogger(),
	)
	armoury, armouryErr := httpPreparator.BuildHTTPRequestCtx(NewHTTPPreparatorConfig(false))
	if armouryErr != nil {
		t.Fatalf("Test failed: could not build HTTP preparator armoury, error: %v", armouryErr)
	}
	reqParams := armoury.GetRequestParams()
	if len(reqParams) < 1 {
		t.Fatalf("Test failed: no request parameters found")
	}

	for _, v := range reqParams {

		argList := v.GetArgList()

		// response, apiErr := CallFromSignature(
		// 		cc, payload.rtCtx, authCtx, authCtx.Type, false, os.Stderr, prov, anysdk.NewAnySdkOpStoreDesignation(opStore), argList)

		response, apiErr := CallFromSignature(
			configurator,
			dto.RuntimeCtx{
				AllowInsecure: true,
			},
			authCtx,
			authCtx.Type,
			false,
			nil,
			prov,
			NewAnySdkOpStoreDesignation(method),
			argList, // TODO: abstract
		)
		if apiErr != nil {
			t.Fatalf("Test failed: API call error: %v", apiErr)
		}
		if response.IsErroneous() {
			t.Fatalf("Test failed: API call returned erroneous response")
		}
		t.Logf("Test passed: received response: %+v", response)
	}

}
