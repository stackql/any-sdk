package radix_tree_address_space_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/discovery"
	"github.com/stackql/any-sdk/public/radix_tree_address_space"
	"github.com/stackql/stackql-provider-registry/signing/Ed25519/app/edcrypto"
)

func getNewTestDataMockRegistry(relativePath string) (anysdk.RegistryAPI, error) {
	return anysdk.NewRegistry(
		anysdk.RegistryConfig{
			RegistryURL:      fmt.Sprintf("file://%s", relativePath),
			LocalDocRoot:     relativePath,
			AllowSrcDownload: false,
			VerifyConfig: &edcrypto.VerifierConfig{
				NopVerify: true,
			},
		},
		nil)
}

type dummyRoundTripper struct {
	resp *http.Response
	err  error
}

func (drt *dummyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return drt.resp, drt.err
}

func getDummyRoundTripper(resp *http.Response, err error) http.RoundTripper {
	return &dummyRoundTripper{
		resp: resp,
		err:  err,
	}
}

func TestNewAddressSpace(t *testing.T) {
	addressSpace := radix_tree_address_space.NewAddressSpaceAnalyzer(
		radix_tree_address_space.NewAddressSpaceGrammar(),
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	if addressSpace == nil {
		t.Fatalf("expected non-nil address space")
	}
}

func TestBasicAddressSpaceGoogleCurrent(t *testing.T) {
	registryLocalPath := "./testdata/registry/basic"
	googleProviderPath := "testdata/registry/basic/src/googleapis.com/v0.1.2/provider.yaml"
	// expectedErrorCount := 282
	analyzerFactory := discovery.NewSimpleSQLiteAnalyzerFactory(registryLocalPath, dto.RuntimeCtx{})
	staticAnalyzer, analyzerErr := analyzerFactory.CreateProviderServiceLevelStaticAnalyzer(
		googleProviderPath,
		"compute",
	)
	if analyzerErr != nil {
		t.Fatalf("Failed to create static analyzer: %v", analyzerErr)
	}
	err := staticAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Static analysis failed: %v", err)
	}
	errorSlice := staticAnalyzer.GetErrors()
	for _, err := range errorSlice {
		t.Logf("Static analysis error: %v", err)
	}
	// these are shallow
	resources := staticAnalyzer.GetResources()
	t.Logf("Discovered %d resources", len(resources))
	if len(resources) == 0 {
		t.Fatalf("Static analysis failed: expected non-zero resources but got %d", len(resources))
	}
	imagesResource, imagesResourceExists := resources["images"]
	if !imagesResourceExists {
		t.Fatalf("Static analysis failed: expected 'images' resource to exist")
	}
	selectImagesMethod, _, selectImagesMethodExists := imagesResource.GetFirstMethodFromSQLVerb("select")
	if !selectImagesMethodExists {
		t.Fatalf("Static analysis failed: expected 'select' method to exist on 'images' resource")
	}
	prov, hasProv := imagesResource.GetProvider()
	if !hasProv {
		t.Fatalf("Static analysis failed: expected provider to exist on 'images' resource")
	}
	registryAPI, hasRegistryAPI := staticAnalyzer.GetRegistryAPI()
	if !hasRegistryAPI {
		t.Fatalf("Static analysis failed: expected registry API to exist on static analyzer")
	}
	if registryAPI == nil {
		t.Fatalf("Static analysis failed: expected non-nil registry API to exist on static analyzer")
	}
	providerService, providerServiceErr := prov.GetProviderService("compute")
	if providerServiceErr != nil {
		t.Fatalf("Static analysis failed: expected 'compute' service to exist on provider")
	}
	svc, svcErr := registryAPI.GetServiceFragment(providerService, "images")
	if svcErr != nil {
		t.Fatalf("Static analysis failed: expected 'images' service to exist on provider")
	}
	rsc, rscErr := svc.GetResource("images")
	if rscErr != nil {
		t.Fatalf("Static analysis failed: expected 'images' resource to exist on service")
	}
	if rsc == nil {
		t.Fatalf("Static analysis failed: expected non-nil 'images' resource to exist")
	}
	selectImagesMethod, _, selectImagesMethodExists = rsc.GetFirstMethodFromSQLVerb("select")
	if !selectImagesMethodExists {
		t.Fatalf("Static analysis failed: expected 'select' method to exist on 'images' resource")
	}

	addressSpaceAnalyzer := radix_tree_address_space.NewAddressSpaceAnalyzer(
		radix_tree_address_space.NewAddressSpaceGrammar(),
		prov,
		svc,
		rsc,
		selectImagesMethod,
		map[string]string{
			"amalgam": "response.body.$.items",
		},
	)
	err = addressSpaceAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Address space analysis failed: %v", err)
	}
	addressSpace := addressSpaceAnalyzer.GetAddressSpace()
	if addressSpace == nil {
		t.Fatalf("Address space analysis failed: expected non-nil address space")
	}
	simpleSelectSchema := addressSpace.GetSimpleSelectSchema()
	if simpleSelectSchema == nil {
		t.Fatalf("Address space analysis failed: expected non-nil simple select schema")
	}
	unionSelectSchemas := addressSpace.GetUnionSelectSchemas()
	if len(unionSelectSchemas) != 1 {
		t.Fatalf("Address space analysis failed: expected 2 union select schemas but got %d", len(unionSelectSchemas))
	}
	for k, v := range unionSelectSchemas {
		t.Logf("Union select schema key: %s, schema title: %s", k, v.GetTitle())
	}
	requestBody, requestBodyOk := addressSpace.DereferenceAddress("request.body")
	if !requestBodyOk {
		t.Fatalf("Address space analysis failed: expected to dereference 'request.body'")
	}
	if requestBody != nil {
		t.Fatalf("Address space analysis failed: expected nil 'request.body'")
	}
	responseBody, responseBodyOk := addressSpace.DereferenceAddress("response.body")
	if !responseBodyOk {
		t.Fatalf("Address space analysis failed: expected to dereference 'response.body'")
	}
	if responseBody == nil {
		t.Fatalf("Address space analysis failed: expected non-nil 'response.body'")
	}
	projectParam, projectParamOk := addressSpace.DereferenceAddress(".project")
	if !projectParamOk {
		t.Fatalf("Address space analysis failed: expected to dereference '.project'")
	}
	if projectParam == nil {
		t.Fatalf("Address space analysis failed: expected non-nil '.project'")
	}
	mutateProjectErr := addressSpace.WriteToAddress(".project", "my-test-project")
	if mutateProjectErr != nil {
		t.Fatalf("Address space analysis failed: expected to write to address '.project'")
	}
	projectVal, projectValOk := addressSpace.ReadFromAddress(".project")
	if !projectValOk {
		t.Fatalf("Address space analysis failed: expected to read from address '.project'")
	}
	if projectVal == nil {
		t.Fatalf("Address space analysis failed: expected non-nil value from address '.project'")
	}
	if projectVal != "my-test-project" {
		t.Fatalf("Address space analysis failed: expected 'my-test-project' from address '.project' but got '%v'", projectVal)
	}
	isResolved := addressSpace.ResolveSignature(map[string]any{
		"project": "my-test-project",
	})
	if !isResolved {
		t.Fatalf("Address space analysis failed: expected signature to be resolved")
	}
	dummyReq := &http.Request{
		Method: "GET",
		URL: &url.URL{
			Scheme:   "https",
			Host:     "www.googleapis.com",
			Path:     "/compute/v1/projects/my-test-project/global/images",
			RawQuery: "filter=name+eq+my-test-image",
		},
		Header: http.Header{
			"Content-Type":  []string{"application/json"},
			"Accept":        []string{"application/json"},
			"User-Agent":    []string{"stackql"},
			"Host":          []string{"www.googleapis.com"},
			"Authorization": []string{"Bearer ya.yb.c"},
		},
	}
	dummyClient := &http.Client{
		Transport: getDummyRoundTripper(
			&http.Response{
				StatusCode: 200,
				Body:       http.NoBody,
			},
			nil,
		),
	}
	invocationErr := addressSpace.Invoke(dummyClient, dummyReq)
	if invocationErr != nil {
		t.Fatalf("Address space analysis failed: expected invocation to succeed: %v", invocationErr)
	}
	mappedNamsespace, mapErr := addressSpace.ToMap()
	if mapErr != nil {
		t.Fatalf("Address space analysis failed: expected to map namespace: %v", mapErr)
	}
	if mappedNamsespace == nil {
		t.Fatalf("Address space analysis failed: expected non-nil mapped namespace")
	}
}

func TestAliasedAddressSpaceGoogleCurrent(t *testing.T) {
	registryLocalPath := "./testdata/registry/basic"
	googleProviderPath := "testdata/registry/basic/src/googleapis.com/v0.1.2/provider.yaml"
	// expectedErrorCount := 282
	analyzerFactory := discovery.NewSimpleSQLiteAnalyzerFactory(registryLocalPath, dto.RuntimeCtx{})
	staticAnalyzer, analyzerErr := analyzerFactory.CreateProviderServiceLevelStaticAnalyzer(
		googleProviderPath,
		"compute",
	)
	if analyzerErr != nil {
		t.Fatalf("Failed to create static analyzer: %v", analyzerErr)
	}
	err := staticAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Static analysis failed: %v", err)
	}
	errorSlice := staticAnalyzer.GetErrors()
	for _, err := range errorSlice {
		t.Logf("Static analysis error: %v", err)
	}
	// these are shallow
	resources := staticAnalyzer.GetResources()
	t.Logf("Discovered %d resources", len(resources))
	if len(resources) == 0 {
		t.Fatalf("Static analysis failed: expected non-zero resources but got %d", len(resources))
	}
	instanceGroupsResource, instanceGroupsResourceExists := resources["instanceGroups"]
	if !instanceGroupsResourceExists {
		t.Fatalf("Static analysis failed: expected 'images' resource to exist")
	}
	selectInstanceGroupMethod, selectInstanceGroupMethodErr := instanceGroupsResource.FindMethod("aggregatedList")
	if selectInstanceGroupMethodErr != nil {
		t.Fatalf("Static analysis failed: expected 'select' method to exist on 'instanceGroups' resource")
	}
	prov, hasProv := instanceGroupsResource.GetProvider()
	if !hasProv {
		t.Fatalf("Static analysis failed: expected provider to exist on 'instanceGroups' resource")
	}
	registryAPI, hasRegistryAPI := staticAnalyzer.GetRegistryAPI()
	if !hasRegistryAPI {
		t.Fatalf("Static analysis failed: expected registry API to exist on static analyzer")
	}
	if registryAPI == nil {
		t.Fatalf("Static analysis failed: expected non-nil registry API to exist on static analyzer")
	}
	providerService, providerServiceErr := prov.GetProviderService("compute")
	if providerServiceErr != nil {
		t.Fatalf("Static analysis failed: expected 'compute' service to exist on provider")
	}
	svc, svcErr := registryAPI.GetServiceFragment(providerService, "instanceGroups")
	if svcErr != nil {
		t.Fatalf("Static analysis failed: expected 'instanceGroups' service to exist on provider")
	}
	rsc, rscErr := svc.GetResource("instanceGroups")
	if rscErr != nil {
		t.Fatalf("Static analysis failed: expected 'instanceGroups' resource to exist on service")
	}
	if rsc == nil {
		t.Fatalf("Static analysis failed: expected non-nil 'instanceGroups' resource to exist")
	}
	selectInstanceGroupMethod, selectInstanceGroupMethodErr = rsc.FindMethod("aggregatedList")
	if selectInstanceGroupMethodErr != nil {
		t.Fatalf("Static analysis failed: expected 'select' method to exist on 'instanceGroups' resource")
	}

	addressSpaceAnalyzer := radix_tree_address_space.NewAddressSpaceAnalyzer(
		radix_tree_address_space.NewAddressSpaceGrammar(),
		prov,
		svc,
		rsc,
		selectInstanceGroupMethod,
		map[string]string{
			"amalgam": "response.body.$.items",
			"name":    "response.body.$.items[*].instanceGroups[*].name",
		},
	)
	err = addressSpaceAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Address space analysis failed: %v", err)
	}
	addressSpace := addressSpaceAnalyzer.GetAddressSpace()
	if addressSpace == nil {
		t.Fatalf("Address space analysis failed: expected non-nil address space")
	}
	simpleSelectSchema := addressSpace.GetSimpleSelectSchema()
	if simpleSelectSchema == nil {
		t.Fatalf("Address space analysis failed: expected non-nil simple select schema")
	}
	unionSelectSchemas := addressSpace.GetUnionSelectSchemas()
	if len(unionSelectSchemas) != 2 {
		t.Fatalf("Address space analysis failed: expected 2 union select schemas but got %d", len(unionSelectSchemas))
	}
	for k, v := range unionSelectSchemas {
		t.Logf("Union select schema key: %s, schema title: %s", k, v.GetTitle())
	}
	requestBody, requestBodyOk := addressSpace.DereferenceAddress("request.body")
	if !requestBodyOk {
		t.Fatalf("Address space analysis failed: expected to dereference 'request.body'")
	}
	if requestBody != nil {
		t.Fatalf("Address space analysis failed: expected nil 'request.body'")
	}
	responseBody, responseBodyOk := addressSpace.DereferenceAddress("response.body")
	if !responseBodyOk {
		t.Fatalf("Address space analysis failed: expected to dereference 'response.body'")
	}
	if responseBody == nil {
		t.Fatalf("Address space analysis failed: expected non-nil 'response.body'")
	}
	projectParam, projectParamOk := addressSpace.DereferenceAddress(".project")
	if !projectParamOk {
		t.Fatalf("Address space analysis failed: expected to dereference '.project'")
	}
	if projectParam == nil {
		t.Fatalf("Address space analysis failed: expected non-nil '.project'")
	}
	mutateProjectErr := addressSpace.WriteToAddress(".project", "my-test-project")
	if mutateProjectErr != nil {
		t.Fatalf("Address space analysis failed: expected to write to address '.project'")
	}
	projectVal, projectValOk := addressSpace.ReadFromAddress(".project")
	if !projectValOk {
		t.Fatalf("Address space analysis failed: expected to read from address '.project'")
	}
	if projectVal == nil {
		t.Fatalf("Address space analysis failed: expected non-nil value from address '.project'")
	}
	if projectVal != "my-test-project" {
		t.Fatalf("Address space analysis failed: expected 'my-test-project' from address '.project' but got '%v'", projectVal)
	}
}

func TestConfigDrivenAliasedAddressSpaceGoogleCurrent(t *testing.T) {
	registryLocalPath := "./testdata/registry/basic"
	googleProviderPath := "testdata/registry/basic/src/googleapis.com/v0.1.2/provider.yaml"
	// expectedErrorCount := 282
	analyzerFactory := discovery.NewSimpleSQLiteAnalyzerFactory(registryLocalPath, dto.RuntimeCtx{})
	staticAnalyzer, analyzerErr := analyzerFactory.CreateProviderServiceLevelStaticAnalyzer(
		googleProviderPath,
		"compute",
	)
	if analyzerErr != nil {
		t.Fatalf("Failed to create static analyzer: %v", analyzerErr)
	}
	err := staticAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Static analysis failed: %v", err)
	}
	staticAnalyzer.GetResources()
	errorSlice := staticAnalyzer.GetErrors()
	for _, err := range errorSlice {
		t.Logf("Static analysis error: %v", err)
	}
	// these are shallow
	resources := staticAnalyzer.GetResources()
	t.Logf("Discovered %d resources", len(resources))
	if len(resources) == 0 {
		t.Fatalf("Static analysis failed: expected non-zero resources but got %d", len(resources))
	}
	svcFrags := staticAnalyzer.GetServiceFragments()
	svc, hasSvc := svcFrags["instanceGroups"]
	if !hasSvc {
		t.Fatalf("Static analysis failed: expected 'compute' service to exist")
	}
	rsc, hasRsc := resources["instanceGroups"]
	if !hasRsc {
		t.Fatalf("Static analysis failed: expected 'instanceGroups' resource to exist on service")
	}
	if rsc == nil {
		t.Fatalf("Static analysis failed: expected non-nil 'instanceGroups' resource to exist")
	}
	selectInstanceGroupMethod, selectInstanceGroupMethodErr := rsc.FindMethod("aggregatedList")
	if selectInstanceGroupMethodErr != nil {
		t.Fatalf("Static analysis failed: expected 'select' method to exist on 'instanceGroups' resource")
	}
	prov, hasProv := rsc.GetProvider()
	if !hasProv {
		t.Fatalf("Static analysis failed: expected provider to exist on 'images' resource")
	}

	addressSpaceAnalyzer := radix_tree_address_space.NewAddressSpaceAnalyzer(
		radix_tree_address_space.NewAddressSpaceGrammar(),
		prov,
		svc,
		rsc,
		selectInstanceGroupMethod,
		map[string]string{
			"amalgam": "response.body.$.items",
			"name":    "response.body.$.items[*].instanceGroups[*].name",
		},
	)
	err = addressSpaceAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Address space analysis failed: %v", err)
	}
	addressSpace := addressSpaceAnalyzer.GetAddressSpace()
	if addressSpace == nil {
		t.Fatalf("Address space analysis failed: expected non-nil address space")
	}
	simpleSelectSchema := addressSpace.GetSimpleSelectSchema()
	if simpleSelectSchema == nil {
		t.Fatalf("Address space analysis failed: expected non-nil simple select schema")
	}
	unionSelectSchemas := addressSpace.GetUnionSelectSchemas()
	if len(unionSelectSchemas) != 2 {
		t.Fatalf("Address space analysis failed: expected 2 union select schemas but got %d", len(unionSelectSchemas))
	}
	for k, v := range unionSelectSchemas {
		t.Logf("Union select schema key: %s, schema title: %s", k, v.GetTitle())
	}
	requestBody, requestBodyOk := addressSpace.DereferenceAddress("request.body")
	if !requestBodyOk {
		t.Fatalf("Address space analysis failed: expected to dereference 'request.body'")
	}
	if requestBody != nil {
		t.Fatalf("Address space analysis failed: expected nil 'request.body'")
	}
	responseBody, responseBodyOk := addressSpace.DereferenceAddress("response.body")
	if !responseBodyOk {
		t.Fatalf("Address space analysis failed: expected to dereference 'response.body'")
	}
	if responseBody == nil {
		t.Fatalf("Address space analysis failed: expected non-nil 'response.body'")
	}
	projectParam, projectParamOk := addressSpace.DereferenceAddress(".project")
	if !projectParamOk {
		t.Fatalf("Address space analysis failed: expected to dereference '.project'")
	}
	if projectParam == nil {
		t.Fatalf("Address space analysis failed: expected non-nil '.project'")
	}
	mutateProjectErr := addressSpace.WriteToAddress(".project", "my-test-project")
	if mutateProjectErr != nil {
		t.Fatalf("Address space analysis failed: expected to write to address '.project'")
	}
	projectVal, projectValOk := addressSpace.ReadFromAddress(".project")
	if !projectValOk {
		t.Fatalf("Address space analysis failed: expected to read from address '.project'")
	}
	if projectVal == nil {
		t.Fatalf("Address space analysis failed: expected non-nil value from address '.project'")
	}
	if projectVal != "my-test-project" {
		t.Fatalf("Address space analysis failed: expected 'my-test-project' from address '.project' but got '%v'", projectVal)
	}
}

func TestFatConfigDrivenAliasedAddressSpaceGoogleCurrent(t *testing.T) {
	registryLocalPath := "./testdata/registry/basic"
	providerPath := "testdata/registry/basic/src/googleapis.com/v0.1.2/provider.yaml"
	serviceName := "compute"
	resourceName := "firewallPolicies"
	methodName := "insert"
	expectedUnionProjectionCount := 4
	// expectedErrorCount := 282
	analyzerFactory := discovery.NewSimpleSQLiteAnalyzerFactory(registryLocalPath, dto.RuntimeCtx{})
	staticAnalyzer, analyzerErr := analyzerFactory.CreateProviderServiceLevelStaticAnalyzer(
		providerPath,
		"compute",
	)
	if analyzerErr != nil {
		t.Fatalf("Failed to create static analyzer: %v", analyzerErr)
	}
	err := staticAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Static analysis failed: %v", err)
	}
	staticAnalyzer.GetResources()
	errorSlice := staticAnalyzer.GetErrors()
	for _, err := range errorSlice {
		t.Logf("Static analysis error: %v", err)
	}
	// these are shallow
	resources := staticAnalyzer.GetResources()
	t.Logf("Discovered %d resources", len(resources))
	if len(resources) == 0 {
		t.Fatalf("Static analysis failed: expected non-zero resources but got %d", len(resources))
	}
	svcFrags := staticAnalyzer.GetServiceFragments()
	svc, hasSvc := svcFrags[resourceName]
	if !hasSvc {
		t.Fatalf("Static analysis failed: expected '%s' service to exist and discoverable from resource '%s'", serviceName, resourceName)
	}
	rsc, hasRsc := resources[resourceName]
	if !hasRsc {
		t.Fatalf("Static analysis failed: expected '%s' resource to exist on service", resourceName)
	}
	if rsc == nil {
		t.Fatalf("Static analysis failed: expected non-nil 'instanceGroups' resource to exist")
	}
	method, methodErr := rsc.FindMethod(methodName)
	if methodErr != nil {
		t.Fatalf("Static analysis failed: expected '%s' method to exist on 'instanceGroups' resource", methodName)
	}
	prov, hasProv := rsc.GetProvider()
	if !hasProv {
		t.Fatalf("Static analysis failed: expected provider to exist on '%s' resource", resourceName)
	}

	addressSpaceAnalyzer := radix_tree_address_space.NewAddressSpaceAnalyzer(
		radix_tree_address_space.NewAddressSpaceGrammar(),
		prov,
		svc,
		rsc,
		method,
		map[string]string{
			"short_name":           "request.body.$.shortName",
			"input_description":    "request.body.$.description",
			"operation_status":     "response.body.$.status",
			"operation_start_time": "response.body.$.startTime",
		},
	)
	err = addressSpaceAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Address space analysis failed: %v", err)
	}
	addressSpace := addressSpaceAnalyzer.GetAddressSpace()
	if addressSpace == nil {
		t.Fatalf("Address space analysis failed: expected non-nil address space")
	}
	simpleSelectSchema := addressSpace.GetSimpleSelectSchema()
	if simpleSelectSchema == nil {
		t.Fatalf("Address space analysis failed: expected non-nil simple select schema")
	}
	unionSelectSchemas := addressSpace.GetUnionSelectSchemas()
	if len(unionSelectSchemas) != expectedUnionProjectionCount {
		t.Fatalf("Address space analysis failed: expected %d union select schemas but got %d", expectedUnionProjectionCount, len(unionSelectSchemas))
	}
	globalSelectSchemas := addressSpace.GetGlobalSelectSchemas()
	if len(globalSelectSchemas) < expectedUnionProjectionCount {
		t.Fatalf("Address space analysis failed: expected >= %d union select schemas but got %d", expectedUnionProjectionCount, len(unionSelectSchemas))
	}
	for k, v := range unionSelectSchemas {
		t.Logf("Union select schema key: %s, schema title: %s", k, v.GetTitle())
	}
	requestBody, requestBodyOk := addressSpace.DereferenceAddress("request.body")
	if !requestBodyOk {
		t.Fatalf("Address space analysis failed: expected to dereference 'request.body'")
	}
	if requestBody == nil {
		t.Fatalf("Address space analysis failed: expected non nil 'request.body'")
	}
	responseBody, responseBodyOk := addressSpace.DereferenceAddress("response.body")
	if !responseBodyOk {
		t.Fatalf("Address space analysis failed: expected to dereference 'response.body'")
	}
	if responseBody == nil {
		t.Fatalf("Address space analysis failed: expected non-nil 'response.body'")
	}
	projectParam, projectParamOk := addressSpace.DereferenceAddress(".requestId")
	if !projectParamOk {
		t.Fatalf("Address space analysis failed: expected to dereference '.requestId'")
	}
	if projectParam == nil {
		t.Fatalf("Address space analysis failed: expected non-nil '.requestId'")
	}
	mutateProjectErr := addressSpace.WriteToAddress(".requestId", "my-test-id")
	if mutateProjectErr != nil {
		t.Fatalf("Address space analysis failed: expected to write to address '.requestId'")
	}
	projectVal, projectValOk := addressSpace.ReadFromAddress(".requestId")
	if !projectValOk {
		t.Fatalf("Address space analysis failed: expected to read from address '.requestId'")
	}
	if projectVal == nil {
		t.Fatalf("Address space analysis failed: expected non-nil value from address '.requestId'")
	}
	if projectVal != "my-test-id" {
		t.Fatalf("Address space analysis failed: expected 'my-test-id' from address '.requestId' but got '%v'", projectVal)
	}
}

func TestBasicAddressSpaceAWSCurrent(t *testing.T) {
	registryLocalPath := "./testdata/registry/basic"
	googleProviderPath := "testdata/registry/basic/src/aws/v0.1.0/provider.yaml"
	// expectedErrorCount := 282
	analyzerFactory := discovery.NewSimpleSQLiteAnalyzerFactory(registryLocalPath, dto.RuntimeCtx{})
	staticAnalyzer, analyzerErr := analyzerFactory.CreateProviderServiceLevelStaticAnalyzer(
		googleProviderPath,
		"ec2",
	)
	if analyzerErr != nil {
		t.Fatalf("Failed to create static analyzer: %v", analyzerErr)
	}
	err := staticAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Static analysis failed: %v", err)
	}
	errorSlice := staticAnalyzer.GetErrors()
	for _, err := range errorSlice {
		t.Logf("Static analysis error: %v", err)
	}
	// these are shallow
	resources := staticAnalyzer.GetResources()
	t.Logf("Discovered %d resources", len(resources))
	if len(resources) == 0 {
		t.Fatalf("Static analysis failed: expected non-zero resources but got %d", len(resources))
	}
	volumesResource, volumesResourceExists := resources["volumes"]
	if !volumesResourceExists {
		t.Fatalf("Static analysis failed: expected 'instances' resource to exist")
	}
	volumesResourceMethod, _, volumesResourceMethodExists := volumesResource.GetFirstMethodFromSQLVerb("select")
	if !volumesResourceMethodExists {
		t.Fatalf("Static analysis failed: expected 'select' method to exist on 'images' resource")
	}
	prov, hasProv := volumesResource.GetProvider()
	if !hasProv {
		t.Fatalf("Static analysis failed: expected provider to exist on 'images' resource")
	}
	registryAPI, hasRegistryAPI := staticAnalyzer.GetRegistryAPI()
	if !hasRegistryAPI {
		t.Fatalf("Static analysis failed: expected registry API to exist on static analyzer")
	}
	if registryAPI == nil {
		t.Fatalf("Static analysis failed: expected non-nil registry API to exist on static analyzer")
	}
	providerService, providerServiceErr := prov.GetProviderService("ec2")
	if providerServiceErr != nil {
		t.Fatalf("Static analysis failed: expected 'compute' service to exist on provider")
	}
	svc, svcErr := registryAPI.GetServiceFragment(providerService, "")
	if svcErr != nil {
		t.Fatalf("Static analysis failed: expected 'images' service to exist on provider")
	}
	// rsc, rscErr := svc.GetResource("images")
	// if rscErr != nil {
	// 	t.Fatalf("Static analysis failed: expected 'images' resource to exist on service")
	// }
	// if rsc == nil {
	// 	t.Fatalf("Static analysis failed: expected non-nil 'images' resource to exist")
	// }

	addressSpaceAnalyzer := radix_tree_address_space.NewAddressSpaceAnalyzer(
		radix_tree_address_space.NewAddressSpaceGrammar(),
		prov,
		svc,
		volumesResource,
		volumesResourceMethod,
		map[string]string{
			"amalgam": "response.body./Volumes",
			"vol":     "response.body./*/volumeSet/item",
		},
	)
	err = addressSpaceAnalyzer.Analyze()
	if err != nil {
		t.Fatalf("Address space analysis failed: %v", err)
	}
	addressSpace := addressSpaceAnalyzer.GetAddressSpace()
	if addressSpace == nil {
		t.Fatalf("Address space analysis failed: expected non-nil address space")
	}
	simpleSelectSchema := addressSpace.GetSimpleSelectSchema()
	if simpleSelectSchema == nil {
		t.Fatalf("Address space analysis failed: expected non-nil simple select schema")
	}
	unionSelectSchemas := addressSpace.GetUnionSelectSchemas()
	if len(unionSelectSchemas) != 2 {
		t.Fatalf("Address space analysis failed: expected 2 union select schemas but got %d", len(unionSelectSchemas))
	}
	for k, v := range unionSelectSchemas {
		t.Logf("Union select schema key: %s, schema title: %s", k, v.GetTitle())
	}
}
