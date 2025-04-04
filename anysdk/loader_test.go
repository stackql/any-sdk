package anysdk

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"

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

func setupFileRoot(t *testing.T) {
	var err error
	OpenapiFileRoot, err = fileutil.GetFilePathFromRepositoryRoot(path.Join("test", "registry", "src"))
	assert.NilError(t, err)
}

func TestSimpleOktaApplicationServiceRead(t *testing.T) {
	setupFileRoot(t)
	for _, vr := range oktaTestableVersions {

		pr, err := LoadProviderByName("okta", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		ps, err := pr.GetProviderService("application")
		if err != nil {
			t.Fatalf("Test failed: could not locate ProviderService for okta.application")
		}

		b, err := GetServiceDocBytes(fmt.Sprintf("okta/%s/services/Application.yaml", vr))
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		svc, err := LoadServiceDocFromBytes(ps, b)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Equal(t, svc.GetName(), "application")
	}

	t.Logf("TestSimpleOktaServiceRead passed")
}

func TestSimpleOktaApplicationServiceReadAndDump(t *testing.T) {
	setupFileRoot(t)

	for _, vr := range oktaTestableVersions {
		b, err := GetServiceDocBytes(fmt.Sprintf("okta/%s/services/Application.yaml", vr))
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		l := newLoader()

		svc, err := l.loadFromBytes(b)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Equal(t, svc.GetName(), "application")

		_, err = fileutil.GetFilePathFromRepositoryRoot("test/_output/Application.spew.raw.txt")

		assert.NilError(t, err)
	}

	t.Logf("TestSimpleOktaApplicationServiceReadAndDump passed\n")
}

func TestSimpleOktaApplicationServiceReadAndDumpString(t *testing.T) {
	setupFileRoot(t)
	for _, vr := range oktaTestableVersions {
		b, err := GetServiceDocBytes(fmt.Sprintf("okta/%s/services/Application.yaml", vr))
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		l := newLoader()

		svc, err := l.loadFromBytes(b)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Equal(t, svc.GetName(), "application")

		outFile, err := fileutil.GetFilePathFromRepositoryRoot("test/_output/Application.spew.go")

		assert.NilError(t, err)

		f, err := os.OpenFile(outFile, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0666)
		assert.NilError(t, err)

		f.WriteString("package main\n\n")
		f.WriteString("import(\n")
		f.WriteString(`  "encoding/json"` + "\n\n")
		f.WriteString(`  "github.com/getkin/kin-openapi/openapi3"` + "\n")
		f.WriteString(`  "github.com/stackql/anysdk"` + "\n")
		f.WriteString(")\n\n")
	}

	t.Logf("TestSimpleOktaApplicationServiceReadAndDump passed\n")
}

func TestSimpleOktaApplicationServiceJsonReadAndDumpString(t *testing.T) {
	setupFileRoot(t)
	for _, vr := range oktaTestableVersions {
		b, err := GetServiceDocBytes(fmt.Sprintf("okta/%s/services/Application.yaml", vr))
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		l := newLoader()

		svc, err := l.loadFromBytes(b)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Equal(t, svc.GetName(), "application")

		outFile, err := fileutil.GetFilePathFromRepositoryRoot("test/_output/Application.json")

		assert.NilError(t, err)

		b, err = json.MarshalIndent(svc, "", "  ")

		assert.NilError(t, err)

		assert.Assert(t, b != nil)

		f, err := os.OpenFile(outFile, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0666)
		assert.NilError(t, err)

		f.Write(b)
		f.Close()

	}

	t.Logf("TestSimpleOktaApplicationServiceReadAndDump passed\n")
}

func TestSimpleAWSec2ServiceJsonReadAndDumpString(t *testing.T) {
	setupFileRoot(t)
	for _, vr := range awsTestableVersions {
		b, err := GetServiceDocBytes(fmt.Sprintf("aws/%s/services/ec2.yaml", vr))
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		l := newLoader()

		svc, err := l.loadFromBytes(b)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Equal(t, svc.GetName(), "ec2")

		outFile, err := fileutil.GetFilePathFromRepositoryRoot("test/_output/ec2.json")

		assert.NilError(t, err)

		inst, err := svc.GetResource("volumes")

		assert.NilError(t, err)

		opStore, err := inst.FindMethod("describeVolumes")

		assert.NilError(t, err)

		assert.Assert(t, opStore != nil)

		rscs, err := svc.GetResources()

		assert.NilError(t, err)

		assert.Assert(t, rscs != nil)

		b, err = json.MarshalIndent(svc, "", "  ")

		assert.NilError(t, err)

		assert.Assert(t, b != nil)

		f, err := os.OpenFile(outFile, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0666)
		assert.NilError(t, err)

		f.Write(b)
		f.Close()

	}

	t.Logf("TestSimpleOktaApplicationServiceReadAndDump passed\n")
}

func TestSimpleGoogleComputeServiceJsonReadAndDumpString(t *testing.T) {
	setupFileRoot(t)
	for _, vr := range googleTestableVersions {
		pr, err := LoadProviderByName("googleapis.com", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		ps, err := pr.GetProviderService("compute")
		if err != nil {
			t.Fatalf("Test failed: could not locate ProviderService for google.compute")
		}

		b, err := GetServiceDocBytes(fmt.Sprintf("googleapis.com/%s/services-split/compute/compute-v1.yaml", vr))
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		br, err := GetServiceDocBytes(fmt.Sprintf("googleapis.com/%s/resources/compute-v1.yaml", vr))
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		l := newLoader()

		rr, err := LoadResourcesShallow(ps, br)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		svc, err := l.loadFromBytesAndResources(rr, "subnetworks", b)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Equal(t, svc.GetName(), "compute")

		outFile, err := fileutil.GetFilePathFromRepositoryRoot("test/_output/Compute.json")

		assert.NilError(t, err)

		b, err = json.MarshalIndent(svc, "", "  ")

		assert.NilError(t, err)

		assert.Assert(t, b != nil)

		f, err := os.OpenFile(outFile, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0666)
		assert.NilError(t, err)

		f.Write(b)
		f.Close()

	}

	t.Logf("TestSimpleGoogleComputeServiceJsonReadAndDumpString passed\n")
}

func TestSimpleGoogleComputeResourcesJsonRead(t *testing.T) {
	setupFileRoot(t)

	for _, vr := range googleTestableVersions {

		pr, err := LoadProviderByName("googleapis.com", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		ps, err := pr.GetProviderService("compute")
		if err != nil {
			t.Fatalf("Test failed: could not locate ProviderService for google.compute")
		}

		b, err := GetServiceDocBytes(fmt.Sprintf("googleapis.com/%s/resources/compute-v1.yaml", vr))
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		rr, err := LoadResourcesShallow(ps, b)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, rr != nil)
		atTypes, ok := rr.GetResource("acceleratorTypes")
		if !ok {
			t.Fatalf("Test failed: could not locate resource acceleratorTypes")
		}
		assert.Equal(t, atTypes.GetID(), "google.compute.acceleratorTypes")

		t.Logf("TestSimpleGoogleComputeResourcesJsonRead passed\n")
	}
}

func TestIndirectGoogleComputeResourcesJsonRead(t *testing.T) {

	setupFileRoot(t)

	for _, vr := range googleTestableVersions {
		pr, err := LoadProviderByName("googleapis.com", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		rr, err := pr.GetResourcesShallow("compute")
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, rr != nil)
		atTypes, ok := rr.GetResource("acceleratorTypes")
		if !ok {
			t.Fatalf("Test failed: could not locate resource acceleratorTypes")
		}
		assert.Equal(t, atTypes.GetID(), "google.compute.acceleratorTypes")
	}

	t.Logf("TestSimpleGoogleComputeResourcesJsonRead passed\n")
}

func TestIndirectGoogleComputeServiceSubsetJsonRead(t *testing.T) {

	setupFileRoot(t)

	for _, vr := range googleTestableVersions {

		pr, err := LoadProviderByName("googleapis.com", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		rr, err := pr.GetResourcesShallow("compute")
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, rr != nil)
		atTypes, ok := rr.GetResource("acceleratorTypes")
		if !ok {
			t.Fatalf("Test failed: could not locate resource acceleratorTypes")
		}
		assert.Equal(t, atTypes.GetID(), "google.compute.acceleratorTypes")

		getMethod, err := atTypes.FindMethod("get")
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}
		sb, err := GetServiceDocBytes(getMethod.GetOperationRef().ExtractServiceDocPath())
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		sv, err := LoadServiceSubsetDocFromBytes(rr, "instances", sb)

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}
		assert.Assert(t, sv != nil)

		sn := sv.GetName()

		assert.Equal(t, sn, "compute")
	}

	t.Logf("TestIndirectGoogleComputeServiceSubsetJsonRead passed\n")
}

func TestIndirectGoogleComputeServiceSubsetAccess(t *testing.T) {

	setupFileRoot(t)

	for _, vr := range googleTestableVersions {

		pr, err := LoadProviderByName("googleapis.com", vr)
		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		sh, err := pr.GetProviderService("compute")

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, sh != nil)

		sv, err := sh.GetServiceFragment("instances")

		if err != nil {
			t.Fatalf("Test failed: %v", err)
		}

		assert.Assert(t, sv != nil)

		sn := sv.GetName()

		assert.Equal(t, sn, "compute")
	}

	t.Logf("TestIndirectGoogleComputeServiceSubsetAccess passed\n")
}
