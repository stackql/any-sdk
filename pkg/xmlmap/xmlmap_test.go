package xmlmap_test

import (
	"path/filepath"
	"testing"

	"gotest.tools/assert"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stackql/any-sdk/pkg/fileutil"
	. "github.com/stackql/any-sdk/pkg/xmlmap"
	"github.com/stackql/any-sdk/test/pkg/testutil"
)

func getFileRoot(t *testing.T) string {
	rv, err := fileutil.GetFilePathUnescapedFromRepositoryRoot(filepath.Join("test", "registry", "src"))
	assert.NilError(t, err)
	return rv
}

func TestAwareListVolumesMulti(t *testing.T) {

	fr := getFileRoot(t)

	l := openapi3.NewLoader()
	svc, err := l.LoadFromFile(filepath.Join(fr, "aws", "v0.1.0", "services", "ec2.yaml"))
	assert.NilError(t, err)
	assert.Assert(t, svc != nil)

	sc := svc.Components.Schemas["VolumeList"].Value

	m, doc, err := GetSubObjTyped(testutil.GetAwsEc2ListMultiResponseReader(), "/DescribeVolumesResponse/volumeSet/item", sc)

	assert.NilError(t, err)
	assert.Assert(t, m != nil)
	assert.Assert(t, doc != nil)

	mc, ok := m.([]map[string]interface{})
	assert.Assert(t, ok)
	assert.Assert(t, len(mc) == 2)
	assert.Assert(t, mc[1]["iops"] == 100)
	assert.Assert(t, mc[1]["size"] == 8)

}

func TestXMLMArshal(t *testing.T) {
	input := map[string]interface{}{
		"XX": "yy",
	}
	b, err := MarshalXMLUserInput(input, "Input", "", "", `xmlns="https://route53.amazonaws.com/doc/2013-04-01/"`)
	assert.NilError(t, err)
	s := string(b)
	assert.Assert(t, s == `<Input xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><XX>yy</XX></Input>`)
}
