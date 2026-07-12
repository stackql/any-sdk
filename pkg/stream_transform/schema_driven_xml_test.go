package stream_transform

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

// fakeSchema is a minimal SchemaTree for tests.
type fakeSchema struct {
	typ     string
	items   *fakeSchema
	props   map[string]*fakeSchema
	xmlName string
}

func (f *fakeSchema) Type() string { return f.typ }

func (f *fakeSchema) XMLName() string { return f.xmlName }

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

// runWalkerEnvelope returns the full output envelope, which may carry scalar
// siblings (pagination tokens, requestId) alongside the row list.
func runWalkerEnvelope(t *testing.T, override *fakeSchema, protocol, xml string) map[string]interface{} {
	t.Helper()
	tr, err := newSchemaDrivenXMLTransformer(xml, override, protocol, "line_items", bytes.NewBuffer(nil))
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	if err := tr.Transform(); err != nil {
		t.Fatalf("transform: %v", err)
	}
	out, _ := io.ReadAll(tr.GetOutStream())
	var env map[string]interface{}
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("bad envelope json %q: %v", string(out), err)
	}
	return env
}

func runWalker(t *testing.T, override *fakeSchema, protocol, xml string) []map[string]interface{} {
	t.Helper()
	env := runWalkerEnvelope(t, override, protocol, xml)
	raw, ok := env["line_items"].([]interface{})
	if !ok {
		t.Fatalf("line_items should be a list, got %#v", env["line_items"])
	}
	rows := make([]map[string]interface{}, 0, len(raw))
	for _, e := range raw {
		rm, ok := e.(map[string]interface{})
		if !ok {
			t.Fatalf("row should be an object, got %#v", e)
		}
		rows = append(rows, rm)
	}
	return rows
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

func TestWalker_XMLNameOverrideProjection(t *testing.T) {
	// EC2's Volume.Attachments member is serialised as <attachmentSet>: the
	// schema carries the member name as the property key and the wire element
	// name as an xml: name override. The walker must extract by the override
	// and key the projected row by it (value extraction downstream resolves
	// GetWireName first).
	row := &fakeSchema{typ: "object", props: map[string]*fakeSchema{
		"VolumeId":    {typ: "string", xmlName: "volumeId"},
		"Attachments": {typ: "array", xmlName: "attachmentSet"},
		"Size":        {typ: "integer", xmlName: "size"},
	}}
	list := &fakeSchema{typ: "array", items: row}
	override := &fakeSchema{typ: "object", props: map[string]*fakeSchema{"line_items": list}}
	xml := `<DescribeVolumesResponse><volumeSet>` +
		`<item><volumeId>vol-1</volumeId><size>8</size>` +
		`<attachmentSet><item><instanceId>i-1</instanceId></item></attachmentSet></item>` +
		`</volumeSet></DescribeVolumesResponse>`
	rows := runWalker(t, override, XProtocolEC2, xml)
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d (%v)", len(rows), rows)
	}
	r := rows[0]
	if r["volumeId"] != "vol-1" || r["size"] != float64(8) {
		t.Fatalf("scalar wire-key projection mismatch: %v", r)
	}
	att, ok := r["attachmentSet"].(string)
	if !ok || !bytes.Contains([]byte(att), []byte("i-1")) {
		t.Fatalf("attachmentSet should be a JSON string containing i-1: %v", r["attachmentSet"])
	}
	if _, present := r["Attachments"]; present {
		t.Fatalf("projected row must be keyed by wire name, not property name: %v", r)
	}
}

func TestWalker_ComplexValueUnderStringSchemaIsJSON(t *testing.T) {
	// Display schemas type complex columns as "string" so users decompose
	// them with JSON_EXTRACT. A nested XML structure landing under such a
	// column must serialise as JSON, not Go fmt map notation.
	override := overrideWith(map[string]string{"AttributeName": "string", "AttributeValues": "string"})
	xml := `<DescribeAccountAttributesResponse><accountAttributeSet>` +
		`<item><AttributeName>max-instances</AttributeName>` +
		`<AttributeValues><item><attributeValue>20</attributeValue></item></AttributeValues></item>` +
		`</accountAttributeSet></DescribeAccountAttributesResponse>`
	rows := runWalker(t, override, XProtocolEC2, xml)
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d (%v)", len(rows), rows)
	}
	av, ok := rows[0]["AttributeValues"].(string)
	if !ok {
		t.Fatalf("AttributeValues should be a string: %#v", rows[0]["AttributeValues"])
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(av), &decoded); err != nil {
		t.Fatalf("AttributeValues should be valid JSON, got %q: %v", av, err)
	}
	if !bytes.Contains([]byte(av), []byte(`"attributeValue"`)) {
		t.Fatalf("expected attributeValue key in JSON: %q", av)
	}
}

func TestWalker_EC2SingletonUnwrap(t *testing.T) {
	// CreateVpc (ec2 protocol) returns {requestId, vpc: {...}} - the row lives
	// under a named wrapper member. The walker must unwrap one level when the
	// payload root does not itself carry the row fields.
	override := overrideWith(map[string]string{"vpcId": "string", "state": "string", "cidrBlock": "string"})
	xml := `<CreateVpcResponse><requestId>r-9</requestId>` +
		`<vpc><vpcId>vpc-123</vpcId><state>pending</state><cidrBlock>10.99.0.0/16</cidrBlock></vpc>` +
		`</CreateVpcResponse>`
	rows := runWalker(t, override, XProtocolEC2, xml)
	if len(rows) != 1 {
		t.Fatalf("want 1 unwrapped singleton row, got %d (%v)", len(rows), rows)
	}
	if rows[0]["vpcId"] != "vpc-123" || rows[0]["cidrBlock"] != "10.99.0.0/16" {
		t.Fatalf("unwrap mismatch: %v", rows[0])
	}
}

func TestWalker_EmptyBodyYieldsNoRows(t *testing.T) {
	// S3 CreateBucket (and friends) return 200 with an empty body; the walker
	// must emit an empty row set rather than an mxj EOF error.
	override := overrideWith(map[string]string{"Location": "string"})
	rows := runWalker(t, override, XProtocolRestXML, "")
	if len(rows) != 0 {
		t.Fatalf("want 0 rows for empty body, got %d (%v)", len(rows), rows)
	}
}

func TestWalker_ScalarSiblingPassthrough(t *testing.T) {
	// Pagination response tokens (<nextToken> et al.) are scalar siblings of
	// the row carrier: they must survive into the output envelope so the
	// pagination machinery can extract them from the transformed document.
	override := overrideWith(map[string]string{"volumeId": "string", "size": "integer"})
	xml := `<DescribeVolumesResponse><requestId>r-1</requestId><volumeSet>` +
		`<item><volumeId>vol-1</volumeId><size>8</size></item>` +
		`</volumeSet><nextToken>tok-page-2</nextToken></DescribeVolumesResponse>`
	env := runWalkerEnvelope(t, override, XProtocolEC2, xml)
	if env["nextToken"] != "tok-page-2" {
		t.Errorf("nextToken = %#v, want \"tok-page-2\"", env["nextToken"])
	}
	if env["requestId"] != "r-1" {
		t.Errorf("requestId = %#v, want \"r-1\"", env["requestId"])
	}
	rows, ok := env["line_items"].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("line_items should carry 1 row, got %#v", env["line_items"])
	}
	row, _ := rows[0].(map[string]interface{})
	if row["volumeId"] != "vol-1" || row["size"] != float64(8) {
		t.Fatalf("row projection changed by passthrough: %v", row)
	}
	if _, present := row["nextToken"]; present {
		t.Fatalf("token must live on the envelope, not the row: %v", row)
	}
}

func TestWalker_ScalarSiblingPassthroughQueryResultWrapper(t *testing.T) {
	// query-protocol tokens live inside the *Result wrapper (the payload map),
	// e.g. CloudFormation's <NextToken> sibling of <Stacks>.
	override := overrideWith(map[string]string{"StackName": "string"})
	xml := `<DescribeStacksResponse><DescribeStacksResult>` +
		`<Stacks><member><StackName>s1</StackName></member></Stacks>` +
		`<NextToken>tok-42</NextToken>` +
		`</DescribeStacksResult></DescribeStacksResponse>`
	env := runWalkerEnvelope(t, override, XProtocolQuery, xml)
	if env["NextToken"] != "tok-42" {
		t.Errorf("NextToken = %#v, want \"tok-42\"", env["NextToken"])
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
