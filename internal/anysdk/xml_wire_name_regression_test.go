package anysdk

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// xmlStringProperty builds a string property carrying an xml: name override (the
// wire element name), as an xml provider document does for a column whose display
// name differs from the wire name.
func xmlStringProperty(xmlName string) *openapi3.SchemaRef {
	return &openapi3.SchemaRef{Value: &openapi3.Schema{
		Type: "string",
		XML:  map[string]interface{}{"name": xmlName},
	}}
}

// allOfXMLStringProperty builds a property whose xml: name override lives in an
// allOf member (the shape real provider documents use, e.g. AWS EC2 Volume:
// allOf: [ {$ref: String}, { xml: { name: availabilityZone } } ]).
func allOfXMLStringProperty(xmlName string) *openapi3.SchemaRef {
	return &openapi3.SchemaRef{Value: &openapi3.Schema{
		AllOf: openapi3.SchemaRefs{
			{Value: &openapi3.Schema{Type: "string"}},
			{Value: &openapi3.Schema{XML: map[string]interface{}{"name": xmlName}}},
		},
	}}
}

// assertColumnNaming checks a single column's display name (GetName) and wire name
// (GetWireName).
func assertColumnNaming(t *testing.T, cols map[string]ColumnDescriptor, display, wire string) {
	t.Helper()
	col, ok := cols[display]
	if !ok {
		t.Fatalf("expected a column with display name %q; got %v", display, keysOf(cols))
	}
	if got := col.GetName(); got != display {
		t.Errorf("GetName() = %q, want display name %q", got, display)
	}
	if got := col.GetWireName(); got != wire {
		t.Errorf("column %q: GetWireName() = %q, want wire name %q", display, got, wire)
	}
}

// resourceAndSelectItemsCols returns the column descriptors for the two paths a
// consumer uses: the resource-schema path (plain Tabulate) and the
// response/select-items path (Tabulate then RenameColumnsToXml, as the XML media
// type branch does). They must agree on every column's display and wire name.
func resourceAndSelectItemsCols(s Schema) (resource, selectItems map[string]ColumnDescriptor) {
	resource = columnsByName(s.Tabulate(false, "").GetColumns())
	selectItems = columnsByName(s.Tabulate(false, "").RenameColumnsToXml().GetColumns())
	return resource, selectItems
}

// TestXMLNameOverrideColumnNaming covers the alpha09 regression (Case A): an xml:
// name override is the wire name, so it must land in GetWireName and never in
// GetName, and the resource and response/select-items descriptor paths must agree.
// Before the fix, RenameColumnsToXml overwrote GetName with the wire name, creating
// the column under the wire name on a case-sensitive backend (postgres 42703).
func TestXMLNameOverrideColumnNaming(t *testing.T) {
	sc := &openapi3.Schema{
		Type: "object",
		Properties: openapi3.Schemas{
			"AvailabilityZone": allOfXMLStringProperty("availabilityZone"), // real allOf shape
			"SnapshotId":       xmlStringProperty("snapshotId"),            // direct xml shape
			"name":             stringProperty(),                           // no override: key == wire == display
		},
	}
	svc := &standardService{Provider: &standardProvider{StackQLConfig: &standardStackQLConfig{}}}
	s := newStandardSchema(sc, svc, "Volume", "")

	resource, selectItems := resourceAndSelectItemsCols(s)
	for _, cols := range []map[string]ColumnDescriptor{resource, selectItems} {
		assertColumnNaming(t, cols, "AvailabilityZone", "availabilityZone")
		assertColumnNaming(t, cols, "SnapshotId", "snapshotId")
		assertColumnNaming(t, cols, "name", "name") // no-divergence case
	}
	if resource["AvailabilityZone"].GetName() != selectItems["AvailabilityZone"].GetName() {
		t.Errorf("display name diverges across paths: resource %q vs select-items %q",
			resource["AvailabilityZone"].GetName(), selectItems["AvailabilityZone"].GetName())
	}
}

// TestSnakeCaseAliasColumnNamingAcrossPaths covers Case B: under snake_case_aliases
// the display name is the snake alias and the wire name is the xml: name (the
// original), consistent across both descriptor paths.
func TestSnakeCaseAliasColumnNamingAcrossPaths(t *testing.T) {
	sc := &openapi3.Schema{
		Type: "object",
		Properties: openapi3.Schemas{
			"cidrBlock": xmlStringProperty("cidrBlock"),
			"VolumeId":  stringProperty(), // snake-aliased, wire == property key
		},
	}
	svc := &standardService{Provider: &standardProvider{StackQLConfig: &standardStackQLConfig{SnakeCaseAliases: true}}}
	s := newStandardSchema(sc, svc, "Volume", "")

	resource, selectItems := resourceAndSelectItemsCols(s)
	for _, cols := range []map[string]ColumnDescriptor{resource, selectItems} {
		assertColumnNaming(t, cols, "cidr_block", "cidrBlock")
		assertColumnNaming(t, cols, "volume_id", "VolumeId")
	}
}
