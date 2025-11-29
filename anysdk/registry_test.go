package anysdk_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	. "github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/fileutil"

	"gotest.tools/assert"
)

var (
	awsTestableVersions = []string{
		"v0.1.0",
	}
	oktaTestableVersions = []string{
		"v0.1.0",
	}
	googleTestableVersions = []string{
		// "v0.1.0",
		"v0.1.2",
	}
)

const (
	individualDownloadAllowedRegistryCfgStr string = `{"allowSrcDownload": true,  "verifyConfig": { "nopVerify": true } }`
	pullProvidersRegistryCfgStr             string = `{"srcPrefix": "test-src",  "verifyConfig": { "nopVerify": true }   }`
	deprecatedRegistryCfgStr                string = `{"srcPrefix": "deprecated-src",  "verifyConfig": { "nopVerify": true }   }`
	unsignedProvidersRegistryCfgStr         string = `{"srcPrefix": "unsigned-src",  "verifyConfig": { "nopVerify": true }  }`
)

func init() {
	var err error
	OpenapiFileRoot, err = fileutil.GetFilePathFromRepositoryRoot("test/registry/src")
	if err != nil {
		os.Exit(1)
	}
}

func TestRegistrySimpleOktaApplicationServiceRead(t *testing.T) {
	execLocalAndRemoteRegistryTests(t, individualDownloadAllowedRegistryCfgStr, execTestRegistrySimpleOktaApplicationServiceRead)
}

func TestRegistryIndirectGoogleComputeResourcesJsonRead(t *testing.T) {
	execLocalAndRemoteRegistryTests(t, individualDownloadAllowedRegistryCfgStr, execTestRegistryIndirectGoogleComputeResourcesJsonRead)
}

func TestRegistryIndirectGoogleComputeServiceSubsetAccess(t *testing.T) {
	execLocalAndRemoteRegistryTests(t, individualDownloadAllowedRegistryCfgStr, execTestRegistryIndirectGoogleComputeServiceSubsetAccess)
}

func TestLocalRegistryIndirectGoogleComputeServiceSubsetAccess(t *testing.T) {
	execLocalAndRemoteRegistryTests(t, individualDownloadAllowedRegistryCfgStr, execTestRegistryIndirectGoogleComputeServiceSubsetAccess)
}

func TestProviderPull(t *testing.T) {
	execLocalAndRemoteRegistryTests(t, pullProvidersRegistryCfgStr, execTestRegistrySimpleOktaPull)
}

func TestProviderPullAndPersist(t *testing.T) {
	execRemoteRegistryTestOnly(t, pullProvidersRegistryCfgStr, execTestRegistrySimpleOktaPullAndPersist)
}

func TestRegistryIndirectGoogleComputeServiceMethodResolutionSeparateDocs(t *testing.T) {
	execLocalRegistryTestOnly(t, unsignedProvidersRegistryCfgStr, execTestRegistryIndirectGoogleComputeServiceMethodResolutionSeparateDocs)
}

func TestRegistryArrayTopLevelResponse(t *testing.T) {
	execLocalRegistryTestOnly(t, unsignedProvidersRegistryCfgStr, execTestRegistryCanHandleArrayResponts)
}

func TestRegistryCanHandleUnspecifiedResponseWithDefaults(t *testing.T) {
	execLocalRegistryTestOnly(t, unsignedProvidersRegistryCfgStr, execTestRegistryCanHandleUnspecifiedResponseWithDefaults)
}

func TestRegistryCanHandlePolymorphismAllOf(t *testing.T) {
	execLocalRegistryTestOnly(t, unsignedProvidersRegistryCfgStr, execTestRegistryCanHandlePolymorphismAllOf)
}

func TestListProvidersRegistry(t *testing.T) {
	execRemoteRegistryTestOnly(t, unsignedProvidersRegistryCfgStr, execTestRegistryProvidersList)
}

func TestListProviderVersionsRegistry(t *testing.T) {
	execRemoteRegistryTestOnly(t, unsignedProvidersRegistryCfgStr, execTestRegistryProviderVersionsList)
}

func execLocalAndRemoteRegistryTests(t *testing.T, registryConfigStr string, tf func(t *testing.T, r RegistryAPI)) {

	rc, err := getRegistryCfgFromString(registryConfigStr)

	assert.NilError(t, err)

	runRemote(t, rc, tf)

	runLocal(t, rc, tf)
}

func execLocalRegistryTestOnly(t *testing.T, registryConfigStr string, tf func(t *testing.T, r RegistryAPI)) {

	rc, err := getRegistryCfgFromString(registryConfigStr)

	assert.NilError(t, err)

	runLocal(t, rc, tf)
}

func execRemoteRegistryTestOnly(t *testing.T, registryConfigStr string, tf func(t *testing.T, r RegistryAPI)) {

	rc, err := getRegistryCfgFromString(registryConfigStr)

	assert.NilError(t, err)

	runRemote(t, rc, tf)
}

func getRegistryCfgFromString(registryConfigStr string) (RegistryConfig, error) {
	var rc RegistryConfig
	if registryConfigStr != "" {
		err := json.Unmarshal([]byte(registryConfigStr), &rc)
		return rc, err
	}
	return rc, fmt.Errorf("could not compose registry config")
}

func runLocal(t *testing.T, rc RegistryConfig, tf func(t *testing.T, r RegistryAPI)) {
	r, err := GetMockLocalRegistry(rc)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	tf(t, r)
}

func runRemote(t *testing.T, rc RegistryConfig, tf func(t *testing.T, r RegistryAPI)) {
	r, err := GetMockRegistry(rc)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	tf(t, r)
}

func execTestRegistrySimpleOktaApplicationServiceRead(t *testing.T, r RegistryAPI) {
	for _, vr := range oktaTestableVersions {
		pr, err := LoadProviderByName("okta", vr, "")
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		ps, err := pr.GetProviderService("application")
		if err != nil {
			t.Fatalf("Test failed: could not locate ProviderService for okta.application")
		}
		svc, err := r.GetService(ps)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Equal(t, svc.GetName(), "application")
	}

	t.Logf("TestSimpleOktaServiceRead passed")
}

func execTestRegistryProvidersList(t *testing.T, r RegistryAPI) {

	pr, err := r.ListAllAvailableProviders()
	assert.NilError(t, err)

	assert.Assert(t, len(pr) > 0)
	assert.Assert(t, len(pr["google"].Versions) == 1)
	assert.Assert(t, len(pr["okta"].Versions) == 1)
	assert.Assert(t, pr["google"].Versions[0] == "v2.0.1")
	assert.Assert(t, pr["okta"].Versions[0] == "v2.0.1")

	t.Logf("execTestRegistryProvidersList passed")
}

func execTestRegistryProviderVersionsList(t *testing.T, r RegistryAPI) {

	pr, err := r.ListAllProviderVersions("google")
	assert.NilError(t, err)

	assert.Assert(t, len(pr) == 1)
	assert.Assert(t, len(pr["google"].Versions) == 2)

	t.Logf("execTestRegistryProviderVersionsList passed")
}

func execTestRegistryIndirectGoogleComputeResourcesJsonRead(t *testing.T, r RegistryAPI) {

	for _, vr := range googleTestableVersions {
		pr, err := r.LoadProviderByName("google", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		rr, err := r.GetResourcesShallowFromProvider(pr, "compute")
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, rr != nil)
		aTypes, ok := rr.GetResource("acceleratorTypes")
		if !ok {
			t.Fatalf("Test failed: could not locate resource acceleratorTypes")
		}
		assert.Equal(t, aTypes.GetID(), "google.compute.acceleratorTypes")
	}
	t.Logf("TestSimpleGoogleComputeResourcesJsonRead passed\n")
}

func execTestRegistryIndirectGoogleComputeServiceSubsetJsonRead(t *testing.T, r RegistryAPI) {

	for _, vr := range googleTestableVersions {
		pr, err := r.LoadProviderByName("google", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		rr, err := r.GetResourcesShallowFromProvider(pr, "compute")
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, rr != nil)
		aTypes, ok := rr.GetResource("acceleratorTypes")
		if !ok {
			t.Fatalf("Test failed: could not locate resource acceleratorTypes")
		}
		assert.Equal(t, aTypes.GetID(), "google.compute.acceleratorTypes")

		m, err := aTypes.FindMethod("get")
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		psv, err := r.GetService(m.GetProviderService())

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}
		assert.Assert(t, psv != nil)

		sn := psv.GetName()

		assert.Equal(t, sn, "compute")
	}

	t.Logf("TestIndirectGoogleComputeServiceSubsetJsonRead passed\n")
}

func execTestRegistryIndirectGoogleComputeServiceSubsetAccess(t *testing.T, r RegistryAPI) {

	for _, vr := range googleTestableVersions {
		pr, err := r.LoadProviderByName("google", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		sh, err := pr.GetProviderService("compute")

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, sh != nil)

		sv, err := r.GetServiceFragment(sh, "instances")

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, sv != nil)

		sn := sv.GetName()

		assert.Equal(t, sn, "compute")
	}

	t.Logf("TestIndirectGoogleComputeServiceSubsetAccess passed\n")
}

func execTestRegistrySimpleOktaPull(t *testing.T, r RegistryAPI) {

	for _, vr := range oktaTestableVersions {
		arc, err := r.PullProviderArchive("okta", vr)

		assert.NilError(t, err)

		assert.Assert(t, arc != nil)
	}

}

func execTestRegistrySimpleOktaPullAndPersist(t *testing.T, r RegistryAPI) {
	for _, vr := range oktaTestableVersions {
		err := r.PullAndPersistProviderArchive("okta", vr)

		assert.NilError(t, err)

		pr, err := LoadProviderByName("okta", vr, "")
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		ps, err := pr.GetProviderService("application")
		if err != nil {
			t.Fatalf("Test failed: could not locate ProviderService for okta.application")
		}
		svc, err := r.GetService(ps)

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Equal(t, svc.GetName(), "application")
	}

	t.Logf("TestRegistrySimpleOktaPullAndPersist passed")

}

func execTestRegistryIndirectGoogleComputeServiceMethodResolutionSeparateDocs(t *testing.T, r RegistryAPI) {

	for _, vr := range googleTestableVersions {
		pr, err := r.LoadProviderByName("google", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		sh, err := pr.GetProviderService("compute")

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, sh != nil)

		sv, err := r.GetServiceFragment(sh, "acceleratorTypes")

		assert.NilError(t, err)

		assert.Assert(t, sv != nil)

		sn := sv.GetName()

		assert.Equal(t, sn, "compute")

		rsc, err := sv.GetResource("acceleratorTypes")

		assert.NilError(t, err)

		matchParams := map[string]interface{}{
			"project": struct{}{},
		}

		os, remainingParams, ok := rsc.GetFirstNamespaceMethodMatchFromSQLVerb("select", matchParams)

		assert.Assert(t, ok)

		assert.Assert(t, len(remainingParams) == 0)

		assert.Equal(t, os.GetOperationRef().Value.OperationID, "compute.acceleratorTypes.aggregatedList")
	}

	t.Logf("TestRegistryIndirectGoogleComputeServiceMethodResolutionSeparateDocs passed\n")
}

func execTestRegistryCanHandleArrayResponts(t *testing.T, r RegistryAPI) {

	for _, vr := range []string{"v1"} {
		pr, err := r.LoadProviderByName("github", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		sh, err := pr.GetProviderService("repos")

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, sh != nil)

		sv, err := r.GetServiceFragment(sh, "repos")

		assert.NilError(t, err)

		assert.Assert(t, sv != nil)

		// sn := sv.GetName()

		// assert.Equal(t, sn, "repos")

		rsc, err := sv.GetResource("repos")

		assert.NilError(t, err)

		matchParams := map[string]interface{}{
			"org": struct{}{},
		}

		os, remainingParams, ok := rsc.GetFirstNamespaceMethodMatchFromSQLVerb("select", matchParams)

		assert.Assert(t, ok)

		assert.Assert(t, len(remainingParams) == 0)

		assert.Equal(t, os.GetOperationRef().Value.OperationID, "repos/list-for-org")

		assert.Equal(t, os.GetOperationRef().Value.Responses["200"].Value.Content["application/json"].Schema.Value.Type, "array")

		props := os.GetOperationRef().Value.Responses["200"].Value.Content["application/json"].Schema.Value.Items.Value.Properties

		name, nameExists := props["name"]

		assert.Assert(t, nameExists)

		assert.Equal(t, name.Value.Type, "string")

		sshUrl, sshUrlExists := props["ssh_url"]

		assert.Assert(t, sshUrlExists)

		assert.Equal(t, sshUrl.Value.Type, "string")
	}

}

func execTestRegistryCanHandleUnspecifiedResponseWithDefaults(t *testing.T, r RegistryAPI) {

	for _, vr := range []string{"v0.1.2"} {
		pr, err := r.LoadProviderByName("google", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		sh, err := pr.GetProviderService("compute")

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, sh != nil)

		sv, err := r.GetServiceFragment(sh, "disks")

		assert.NilError(t, err)

		assert.Assert(t, sv != nil)

		sn := sv.GetName()

		assert.Equal(t, sn, "compute")

		rsc, err := sv.GetResource("disks")

		assert.NilError(t, err)

		matchParams := map[string]interface{}{
			"project": struct{}{},
			"zone":    struct{}{},
		}

		os, remainingParams, ok := rsc.GetFirstNamespaceMethodMatchFromSQLVerb("select", matchParams)

		assert.Assert(t, ok)

		assert.Assert(t, len(remainingParams) == 0)

		assert.Equal(t, os.GetOperationRef().Value.OperationID, "compute.disks.list")

		sc, _, err := os.GetResponseBodySchemaAndMediaType()

		assert.NilError(t, err)

		assert.Equal(t, sc.GetType(), "object")

		items, _ := sc.GetSelectListItems("items")

		assert.Assert(t, items != nil)

		name, nameExists := items.GetItemProperty("name")

		assert.Assert(t, nameExists)

		assert.Equal(t, name.GetType(), "string")

		id, idExists := items.GetItemProperty("id")

		assert.Assert(t, idExists)

		assert.Equal(t, id.GetType(), "string")
	}

}

func execTestRegistryCanHandlePolymorphismAllOf(t *testing.T, r RegistryAPI) {

	for _, vr := range []string{"v1"} {
		pr, err := r.LoadProviderByName("github", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		sh, err := pr.GetProviderService("apps")

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, sh != nil)

		sv, err := r.GetServiceFragment(sh, "apps")

		assert.NilError(t, err)

		assert.Assert(t, sv != nil)

		// sn := sv.GetName()

		// assert.Equal(t, sn, "repos")

		rsc, err := sv.GetResource("apps")

		assert.NilError(t, err)

		os, ok := rsc.GetMethods().FindMethod("create_from_manifest")

		assert.Assert(t, ok)

		assert.Equal(t, os.GetOperationRef().Value.OperationID, "apps/create-from-manifest")

		assert.Equal(t, os.GetOperationRef().Value.Responses["201"].Value.Content["application/json"].Schema.Value.Type, "")

		sVal := NewTestSchema(os.GetOperationRef().Value.Responses["201"].Value.Content["application/json"].Schema.Value, sv, "", os.GetOperationRef().Value.Responses["201"].Value.Content["application/json"].Schema.Ref)

		tab := sVal.Tabulate(false, "")

		colz := tab.GetColumns()

		for _, expectedProperty := range []string{"pem", "description"} {
			found := false
			for _, col := range colz {
				if col.GetName() == expectedProperty {
					found = true
					break
				}
			}
			assert.Assert(t, found)
		}
	}

}

func TestRegistryLocalTemplated(t *testing.T) {
	execLocalRegistryTestOnly(t, individualDownloadAllowedRegistryCfgStr, execTestRegistryLocalTemplated)
}

func execTestRegistryLocalTemplated(t *testing.T, r RegistryAPI) {

	for _, vr := range []string{"v0.1.0"} {
		pr, err := r.LoadProviderByName("local_openssl", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		sh, err := pr.GetProviderService("keys")

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, sh != nil)

		sv, err := r.GetServiceFragment(sh, "keys")

		assert.NilError(t, err)

		assert.Assert(t, sv != nil)

		// sn := sv.GetName()

		// assert.Equal(t, sn, "repos")

		rsc, err := sv.GetResource("rsa")

		assert.NilError(t, err)

		method, ok := rsc.GetMethods().FindMethod("create_key_pair")

		assert.Assert(t, ok)

		assert.Assert(t, method != nil)

		// assert.Equal(t, os.GetOperationRef().Value.OperationID, "apps/create-from-manifest")

		// assert.Equal(t, os.GetOperationRef().Value.Responses["201"].Value.Content["application/json"].Schema.Value.Type, "")

		// sVal := NewTestSchema(os.GetOperationRef().Value.Responses["201"].Value.Content["application/json"].Schema.Value, sv, "", os.GetOperationRef().Value.Responses["201"].Value.Content["application/json"].Schema.Ref)

		// tab := sVal.Tabulate(false, "")

		// colz := tab.GetColumns()

		// for _, expectedProperty := range []string{"pem", "description"} {
		// 	found := false
		// 	for _, col := range colz {
		// 		if col.GetName() == expectedProperty {
		// 			found = true
		// 			break
		// 		}
		// 	}
		// 	assert.Assert(t, found)
		// }
	}

}

func TestRegistryProviderLatestVersion(t *testing.T) {

	rc, err := getRegistryCfgFromString(individualDownloadAllowedRegistryCfgStr)
	assert.NilError(t, err)
	r, err := GetMockLocalRegistry(rc)
	assert.NilError(t, err)
	v, err := r.GetLatestAvailableVersion("google")
	assert.NilError(t, err)
	assert.Equal(t, v, "v0.1.2")
	vo, err := r.GetLatestAvailableVersion("okta")
	assert.NilError(t, err)
	assert.Equal(t, vo, "v0.1.0")

	rc, err = getRegistryCfgFromString(deprecatedRegistryCfgStr)
	assert.NilError(t, err)
	r, err = GetMockLocalRegistry(rc)
	assert.NilError(t, err)
	v, err = r.GetLatestAvailableVersion("google")
	assert.NilError(t, err)
	assert.Equal(t, v, "v1")
	vo, err = r.GetLatestAvailableVersion("okta")
	assert.NilError(t, err)
	assert.Equal(t, vo, "v1")

	t.Logf("TestRegistryProviderLatestVersion passed\n")
}

func TestQueryParamPushdownConfig(t *testing.T) {
	execLocalRegistryTestOnly(t, unsignedProvidersRegistryCfgStr, execTestQueryParamPushdownConfig)
}

func execTestQueryParamPushdownConfig(t *testing.T, r RegistryAPI) {
	// Test using OData TripPin reference service - supports full OData query capabilities
	pr, err := r.LoadProviderByName("odata_trippin", "v00.00.00000")
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	sh, err := pr.GetProviderService("main")
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	assert.Assert(t, sh != nil)

	// Test 'people' resource - full OData support with all columns
	sv, err := r.GetServiceFragment(sh, "people")
	assert.NilError(t, err)
	assert.Assert(t, sv != nil)

	rsc, err := sv.GetResource("people")
	assert.NilError(t, err)

	method, ok := rsc.GetMethods().FindMethod("list")
	assert.Assert(t, ok)
	assert.Assert(t, method != nil)

	cfg := method.GetStackQLConfig()
	assert.Assert(t, cfg != nil)

	// Get queryParamPushdown config from the method's StackQL config
	qpp, ok := cfg.GetQueryParamPushdown()
	assert.Assert(t, ok, "expected queryParamPushdown config to exist")

	// Test filter config - full OData operator support
	filterPD, ok := qpp.GetFilter()
	assert.Assert(t, ok, "expected filter pushdown config to exist")
	assert.Equal(t, filterPD.GetDialect(), "odata")
	assert.Equal(t, filterPD.GetParamName(), "$filter") // OData default
	assert.Assert(t, filterPD.IsOperatorSupported("eq"))
	assert.Assert(t, filterPD.IsOperatorSupported("ne"))
	assert.Assert(t, filterPD.IsOperatorSupported("gt"))
	assert.Assert(t, filterPD.IsOperatorSupported("lt"))
	assert.Assert(t, filterPD.IsOperatorSupported("contains"))
	assert.Assert(t, filterPD.IsOperatorSupported("startswith"))
	assert.Assert(t, filterPD.IsOperatorSupported("endswith"))
	assert.Assert(t, filterPD.IsOperatorSupported("and"))
	assert.Assert(t, filterPD.IsOperatorSupported("or"))
	assert.Assert(t, !filterPD.IsOperatorSupported("like"), "OData doesn't support 'like'")
	// No supportedColumns specified = all columns supported
	assert.Assert(t, filterPD.IsColumnSupported("FirstName"))
	assert.Assert(t, filterPD.IsColumnSupported("anyColumn"))

	// Test select config
	selectPD, ok := qpp.GetSelect()
	assert.Assert(t, ok, "expected select pushdown config to exist")
	assert.Equal(t, selectPD.GetDialect(), "odata")
	assert.Equal(t, selectPD.GetParamName(), "$select") // OData default
	assert.Equal(t, selectPD.GetDelimiter(), ",")       // OData default
	// No supportedColumns = all columns supported
	assert.Assert(t, selectPD.IsColumnSupported("FirstName"))
	assert.Assert(t, selectPD.IsColumnSupported("anyColumn"))

	// Test orderBy config
	orderByPD, ok := qpp.GetOrderBy()
	assert.Assert(t, ok, "expected orderBy pushdown config to exist")
	assert.Equal(t, orderByPD.GetDialect(), "odata")
	assert.Equal(t, orderByPD.GetParamName(), "$orderby") // OData default
	assert.Equal(t, orderByPD.GetSyntax(), "odata")       // OData default

	// Test top config
	topPD, ok := qpp.GetTop()
	assert.Assert(t, ok, "expected top pushdown config to exist")
	assert.Equal(t, topPD.GetDialect(), "odata")
	assert.Equal(t, topPD.GetParamName(), "$top") // OData default
	assert.Equal(t, topPD.GetMaxValue(), 0)       // No maxValue set

	// Test count config
	countPD, ok := qpp.GetCount()
	assert.Assert(t, ok, "expected count pushdown config to exist")
	assert.Equal(t, countPD.GetDialect(), "odata")
	assert.Equal(t, countPD.GetParamName(), "$count")         // OData default
	assert.Equal(t, countPD.GetParamValue(), "true")          // OData default
	assert.Equal(t, countPD.GetResponseKey(), "@odata.count") // OData default

	// Test 'airports' resource - OData support with restricted columns and maxValue
	svAirports, err := r.GetServiceFragment(sh, "airports")
	assert.NilError(t, err)

	rscAirports, err := svAirports.GetResource("airports")
	assert.NilError(t, err)

	methodAirports, ok := rscAirports.GetMethods().FindMethod("list")
	assert.Assert(t, ok)

	cfgAirports := methodAirports.GetStackQLConfig()
	assert.Assert(t, cfgAirports != nil)

	qppAirports, ok := cfgAirports.GetQueryParamPushdown()
	assert.Assert(t, ok)

	// Test filter with restricted columns
	filterAirports, ok := qppAirports.GetFilter()
	assert.Assert(t, ok)
	assert.Assert(t, filterAirports.IsColumnSupported("Name"))
	assert.Assert(t, filterAirports.IsColumnSupported("IcaoCode"))
	assert.Assert(t, !filterAirports.IsColumnSupported("Location"), "Location is not in supportedColumns")

	// Test select with restricted columns
	selectAirports, ok := qppAirports.GetSelect()
	assert.Assert(t, ok)
	assert.Assert(t, selectAirports.IsColumnSupported("Name"))
	assert.Assert(t, selectAirports.IsColumnSupported("IataCode"))
	assert.Assert(t, !selectAirports.IsColumnSupported("Location"))

	// Test top with maxValue
	topAirports, ok := qppAirports.GetTop()
	assert.Assert(t, ok)
	assert.Equal(t, topAirports.GetMaxValue(), 100)

	t.Logf("TestQueryParamPushdownConfig passed")
}
