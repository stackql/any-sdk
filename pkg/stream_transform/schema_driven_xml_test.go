package stream_transform

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

// fakeSchema is a minimal SchemaTree for tests.
type fakeSchema struct {
	typ   string
	items *fakeSchema
	props map[string]*fakeSchema
}

func (f *fakeSchema) Type() string { return f.typ }

func (f *fakeSchema) Items() (SchemaTree, bool) {
	if f.items == nil {
		return nil, false
	}
	return f.items, true
}

func (f *fakeSchema) Property(name string) (SchemaTree, bool) {
	p, ok := f.props[name]
	if !ok || p == nil {
		return nil, false
	}
	return p, true
}

func (f *fakeSchema) Properties() map[string]SchemaTree {
	out := make(map[string]SchemaTree, len(f.props))
	for k, v := range f.props {
		out[k] = v
	}
	return out
}

// overrideWith builds {line_items: [ { <field>: <type> } ]}.
func overrideWith(fields map[string]string) *fakeSchema {
	rowProps := make(map[string]*fakeSchema, len(fields))
	for k, t := range fields {
		rowProps[k] = &fakeSchema{typ: t}
	}
	row := &fakeSchema{typ: "object", props: rowProps}
	list := &fakeSchema{typ: "array", items: row}
	return &fakeSchema{typ: "object", props: map[string]*fakeSchema{"line_items": list}}
}

func runWalker(t *testing.T, override *fakeSchema, protocol, xml string) []map[string]interface{} {
	t.Helper()
	tr, err := newSchemaDrivenXMLTransformer(xml, override, protocol, "line_items", bytes.NewBuffer(nil))
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	if err := tr.Transform(); err != nil {
		t.Fatalf("transform: %v", err)
	}
	out, _ := io.ReadAll(tr.GetOutStream())
	var env map[string][]map[string]interface{}
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("bad envelope json %q: %v", string(out), err)
	}
	return env["line_items"]
}

func TestWalker_EC2List(t *testing.T) {
	override := overrideWith(map[string]string{"volumeId": "string", "size": "integer", "encrypted": "boolean"})
	xml := `<DescribeVolumesResponse><requestId>r-1</requestId><volumeSet>` +
		`<item><volumeId>vol-1</volumeId><size>8</size><encrypted>true</encrypted></item>` +
		`<item><volumeId>vol-2</volumeId><size>16</size><encrypted>false</encrypted></item>` +
		`</volumeSet></DescribeVolumesResponse>`
	rows := runWalker(t, override, XProtocolEC2, xml)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d (%v)", len(rows), rows)
	}
	if rows[0]["volumeId"] != "vol-1" || rows[0]["size"] != float64(8) || rows[0]["encrypted"] != true {
		t.Fatalf("row0 mismatch: %v", rows[0])
	}
}

func TestWalker_EC2SmallPayload(t *testing.T) {
	override := overrideWith(map[string]string{"volumeId": "string", "size": "integer"})
	xml := `<DescribeVolumesResponse><volumeSet><item><volumeId>vol-9</volumeId><size>4</size></item></volumeSet></DescribeVolumesResponse>`
	rows := runWalker(t, override, XProtocolEC2, xml)
	if len(rows) != 1 || rows[0]["volumeId"] != "vol-9" || rows[0]["size"] != float64(4) {
		t.Fatalf("unexpected rows: %v", rows)
	}
}

func TestWalker_QueryListWithResultWrapper(t *testing.T) {
	override := overrideWith(map[string]string{"StackName": "string", "StackStatus": "string"})
	xml := `<DescribeStacksResponse><DescribeStacksResult><Stacks>` +
		`<member><StackName>s1</StackName><StackStatus>OK</StackStatus></member>` +
		`<member><StackName>s2</StackName><StackStatus>BAD</StackStatus></member>` +
		`</Stacks></DescribeStacksResult></DescribeStacksResponse>`
	rows := runWalker(t, override, XProtocolQuery, xml)
	if len(rows) != 2 || rows[0]["StackName"] != "s1" || rows[1]["StackStatus"] != "BAD" {
		t.Fatalf("unexpected rows: %v", rows)
	}
}

func TestWalker_QueryEmptySelfClosingList(t *testing.T) {
	override := overrideWith(map[string]string{"StackName": "string"})
	xml := `<DescribeStacksResponse><DescribeStacksResult><Stacks/></DescribeStacksResult></DescribeStacksResponse>`
	rows := runWalker(t, override, XProtocolQuery, xml)
	if len(rows) != 0 {
		t.Fatalf("want 0 rows for empty self-closing list, got %d (%v)", len(rows), rows)
	}
}

func TestWalker_RestXMLList(t *testing.T) {
	override := overrideWith(map[string]string{"Name": "string", "CreationDate": "string"})
	xml := `<ListAllMyBucketsResult>` +
		`<Owner><ID>123</ID><DisplayName>me</DisplayName></Owner>` +
		`<Buckets>` +
		`<Bucket><Name>b1</Name><CreationDate>2020</CreationDate></Bucket>` +
		`<Bucket><Name>b2</Name><CreationDate>2021</CreationDate></Bucket>` +
		`</Buckets></ListAllMyBucketsResult>`
	rows := runWalker(t, override, XProtocolRestXML, xml)
	if len(rows) != 2 || rows[0]["Name"] != "b1" || rows[1]["Name"] != "b2" {
		t.Fatalf("unexpected rows: %v", rows)
	}
}

func TestWalker_RestXMLSingleton(t *testing.T) {
	override := overrideWith(map[string]string{"HostedZone": "object", "DelegationSet": "object"})
	xml := `<GetHostedZoneResponse>` +
		`<HostedZone><Id>/hostedzone/Z1</Id><Name>example.com</Name></HostedZone>` +
		`<DelegationSet><NameServer>ns1</NameServer></DelegationSet>` +
		`</GetHostedZoneResponse>`
	rows := runWalker(t, override, XProtocolRestXML, xml)
	if len(rows) != 1 {
		t.Fatalf("want 1 singleton row, got %d (%v)", len(rows), rows)
	}
	hz, ok := rows[0]["HostedZone"].(string)
	if !ok || !bytes.Contains([]byte(hz), []byte("example.com")) {
		t.Fatalf("HostedZone should be a JSON string containing example.com: %v", rows[0]["HostedZone"])
	}
}

func TestWalker_RestXMLSingletonWithAncillaryList(t *testing.T) {
	override := overrideWith(map[string]string{"HostedZone": "object", "DelegationSet": "object", "VPCs": "array"})
	xml := `<GetHostedZoneResponse>` +
		`<HostedZone><Id>/hostedzone/Z1</Id></HostedZone>` +
		`<DelegationSet><NameServer>ns1</NameServer></DelegationSet>` +
		`<VPCs><VPC><VPCId>vpc-1</VPCId></VPC></VPCs>` +
		`</GetHostedZoneResponse>`
	rows := runWalker(t, override, XProtocolRestXML, xml)
	if len(rows) != 1 {
		t.Fatalf("want 1 singleton row (ancillary list must not trigger list mode), got %d (%v)", len(rows), rows)
	}
	if _, ok := rows[0]["VPCs"].(string); !ok {
		t.Fatalf("VPCs should be JSON-stringified: %v", rows[0]["VPCs"])
	}
}

func TestWalker_TypeDispatch(t *testing.T) {
	override := overrideWith(map[string]string{
		"OwnerId": "string", "Count": "integer", "Enabled": "boolean", "Tags": "object",
	})
	// 12-digit OwnerId must stay a string (no float64), Tags is self-closing -> null.
	xml := `<DescribeXResponse><items>` +
		`<item><OwnerId>123456789012</OwnerId><Count>5</Count><Enabled>false</Enabled><Tags/></item>` +
		`</items></DescribeXResponse>`
	rows := runWalker(t, override, XProtocolEC2, xml)
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r["OwnerId"] != "123456789012" {
		t.Errorf("OwnerId = %#v, want string \"123456789012\"", r["OwnerId"])
	}
	if r["Count"] != float64(5) {
		t.Errorf("Count = %#v, want 5", r["Count"])
	}
	if r["Enabled"] != false {
		t.Errorf("Enabled = %#v, want false", r["Enabled"])
	}
	if r["Tags"] != nil {
		t.Errorf("Tags (self-closing) = %#v, want null", r["Tags"])
	}
}

func TestWalker_FactoryRegistration(t *testing.T) {
	override := overrideWith(map[string]string{"Name": "string"})
	f := NewSchemaDrivenXMLStreamTransformerFactory(SchemaDrivenXMLV1, override, XProtocolEC2, "line_items")
	if !f.IsTransformable() {
		t.Fatalf("SchemaDrivenXMLV1 should be transformable")
	}
	tr, err := f.GetTransformer(`<R><items><item><Name>x</Name></item></items></R>`)
	if err != nil {
		t.Fatalf("GetTransformer: %v", err)
	}
	if err := tr.Transform(); err != nil {
		t.Fatalf("Transform: %v", err)
	}
}
