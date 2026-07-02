package anysdk

import (
	"fmt"
	"io"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/stackql/any-sdk/pkg/casing"

	"gotest.tools/assert"
)

// These tests cover the casing-surface parity fixes shaken out by live AWS
// provider lifecycle testing: every column-naming path (SELECT tabulation,
// SELECT * star expansion, DESCRIBE) and every parameter-resolution path
// (method routing, request construction) must agree on the snake alias
// treatment, and request bodies must only hit the wire when they exist.

func snakeAliasedObjectSchema(flagOn bool, propNames ...string) Schema {
	prov := &standardProvider{StackQLConfig: &standardStackQLConfig{SnakeCaseAliases: flagOn}}
	svc := &standardService{Provider: prov}
	props := openapi3.Schemas{}
	for _, p := range propNames {
		props[p] = stringProperty()
	}
	sc := &openapi3.Schema{Type: "object", Properties: props}
	return newStandardSchema(sc, svc, "Row", "")
}

// TestToDescriptionMapSnakeAliases: DESCRIBE surfaces the same column names as
// SELECT. Before the fix, DESCRIBE showed wire casing (volumeId) while SELECT
// projected the snake alias (volume_id).
func TestToDescriptionMapSnakeAliases(t *testing.T) {
	s := snakeAliasedObjectSchema(true, "VpcId", "cidrBlock", "state")
	dm := s.ToDescriptionMap(true)
	for _, want := range []string{"vpc_id", "cidr_block", "state"} {
		entry, ok := dm[want]
		if !ok {
			t.Fatalf("expected DESCRIBE column %q; got %v", want, mapKeys(dm))
		}
		entryMap, isMap := entry.(map[string]interface{})
		if !isMap {
			t.Fatalf("description entry for %q is not a map: %T", want, entry)
		}
		if got := entryMap["name"]; got != want {
			t.Errorf("description entry name = %v, want %q", got, want)
		}
	}
	for _, wire := range []string{"VpcId", "cidrBlock"} {
		if _, present := dm[wire]; present {
			t.Errorf("wire-cased key %q must not appear in DESCRIBE output", wire)
		}
	}
}

// TestToDescriptionMapDisabledIsWireKeyed: default behaviour unchanged.
func TestToDescriptionMapDisabledIsWireKeyed(t *testing.T) {
	s := snakeAliasedObjectSchema(false, "VpcId", "cidrBlock")
	dm := s.ToDescriptionMap(false)
	for _, wire := range []string{"VpcId", "cidrBlock"} {
		if _, ok := dm[wire]; !ok {
			t.Errorf("expected wire-cased DESCRIBE key %q; got %v", wire, mapKeys(dm))
		}
	}
}

// TestGetAllColumnsSnakeAliases: SELECT * star expansion surfaces snake aliases.
// Before the fix, star expansion emitted wire-cased identifiers which a
// case-sensitive backend resolved as string literals - every value projected
// as its own column name.
func TestGetAllColumnsSnakeAliases(t *testing.T) {
	s := snakeAliasedObjectSchema(true, "VpcId", "cidrBlock", "state")
	got := stringSet(s.GetAllColumns(""))
	for _, want := range []string{"vpc_id", "cidr_block", "state"} {
		if _, ok := got[want]; !ok {
			t.Errorf("expected star column %q; got %v", want, got)
		}
	}
	if _, present := got["VpcId"]; present {
		t.Errorf("wire-cased star column VpcId must not appear: %v", got)
	}
}

// TestGetAllColumnsDisabledIsWireKeyed: default behaviour unchanged.
func TestGetAllColumnsDisabledIsWireKeyed(t *testing.T) {
	s := snakeAliasedObjectSchema(false, "VpcId", "cidrBlock")
	got := stringSet(s.GetAllColumns(""))
	for _, wire := range []string{"VpcId", "cidrBlock"} {
		if _, ok := got[wire]; !ok {
			t.Errorf("expected wire-cased star column %q; got %v", wire, got)
		}
	}
}

func requiredQueryParameterRef(name string) *openapi3.ParameterRef {
	return &openapi3.ParameterRef{Value: &openapi3.Parameter{
		Name: name, In: openapi3.ParameterInQuery, Required: true,
	}}
}

func newRequiredParamOperationStore(nativeCasing string) *standardOpenAPIOperationStore {
	op := &standardOpenAPIOperationStore{
		OperationRef: &OperationRef{Value: &openapi3.Operation{
			Parameters: openapi3.Parameters{requiredQueryParameterRef("VpcId")},
		}},
	}
	if nativeCasing != "" {
		op.Request = &standardExpectedRequest{NativeCasing: nativeCasing}
	}
	return op
}

// TestParameterMatchReverseCasing: a snake_case SQL key must satisfy its
// wire-name REQUIRED parameter during method routing. Before the fix,
// `DELETE ... WHERE vpc_id = ...` reported "no appropriate method" even though
// the casing engine could resolve vpc_id -> VpcId at request-build time.
func TestParameterMatchReverseCasing(t *testing.T) {
	op := newRequiredParamOperationStore(casing.Pascal)
	remaining, ok := op.parameterMatch(map[string]interface{}{"vpc_id": "vpc-1"})
	if !ok {
		t.Fatalf("snake key vpc_id should satisfy required wire param VpcId")
	}
	if len(remaining) != 0 {
		t.Errorf("matched snake key should be consumed; remaining = %v", remaining)
	}
	// The wire form must still match too.
	if _, ok := op.parameterMatch(map[string]interface{}{"VpcId": "vpc-1"}); !ok {
		t.Errorf("wire key VpcId must still satisfy its own parameter")
	}
	// namespaceParameterMatch shares the semantics.
	if _, ok := op.namespaceParameterMatch(map[string]interface{}{"vpc_id": "vpc-1"}); !ok {
		t.Errorf("namespaceParameterMatch must accept the snake alias too")
	}
}

// TestParameterMatchReverseCasingAbsentIsUnchanged: without request
// nativeCasing a snake key does NOT satisfy the wire param (no behaviour change
// for providers that have not opted in).
func TestParameterMatchReverseCasingAbsentIsUnchanged(t *testing.T) {
	op := newRequiredParamOperationStore("")
	if _, ok := op.parameterMatch(map[string]interface{}{"vpc_id": "vpc-1"}); ok {
		t.Fatalf("snake key must not satisfy wire param absent nativeCasing")
	}
}

// TestGetOperationParameterReverseCasing: request construction resolves a
// snake key to its wire-name operation parameter, returning the Addressable
// under its wire name (HttpParameters.StoreParameter then re-keys the value to
// the wire form). Before the fix, the value fell through to the server-param
// branch and never reached the wire ("The request must contain the parameter
// vpcId").
func TestGetOperationParameterReverseCasing(t *testing.T) {
	op := newRequiredParamOperationStore(casing.Pascal)
	addr, ok := op.GetOperationParameter("vpc_id")
	if !ok {
		t.Fatalf("expected vpc_id to resolve to the VpcId operation parameter")
	}
	if got := addr.GetName(); got != "VpcId" {
		t.Errorf("resolved Addressable name = %q, want wire name VpcId", got)
	}
	// Absent native casing, unchanged.
	opPlain := newRequiredParamOperationStore("")
	if _, ok := opPlain.GetOperationParameter("vpc_id"); ok {
		t.Errorf("snake key must not resolve absent nativeCasing")
	}
}

// TestParameterizeNilBodySendsNoBody: a nil body must not marshal to literal
// "null" bytes (S3 CreateBucket rejects them with MalformedXML).
func TestParameterizeNilBodySendsNoBody(t *testing.T) {
	setupFileRoot(t)
	ops, svc := loadK8sNodeSelectOp(t)
	ops.setRequest(&standardExpectedRequest{BodyMediaType: "application/json"})

	params := NewHttpParameters(ops)
	err := params.IngestMap(map[string]interface{}{"cluster_addr": "k8shost"})
	assert.NilError(t, err)

	rvi, err := ops.parameterize(dummmyK8sProv, svc, params, nil)
	assert.NilError(t, err)
	assert.Assert(t, rvi != nil)
	if rvi.Request.Body != nil {
		b, _ := io.ReadAll(rvi.Request.Body)
		t.Fatalf("nil body must not produce request bytes; got %q", string(b))
	}
}

// TestParameterizeBaseFallbackBody: with request.base declared, an empty body
// map sends the base bytes verbatim with the declared Content-Type (AWS json
// requires literal '{}' on no-input ops).
func TestParameterizeBaseFallbackBody(t *testing.T) {
	setupFileRoot(t)
	ops, svc := loadK8sNodeSelectOp(t)
	ops.setRequest(&standardExpectedRequest{
		BodyMediaType: "application/x-amz-json-1.0",
		Base:          "{}",
	})

	params := NewHttpParameters(ops)
	err := params.IngestMap(map[string]interface{}{"cluster_addr": "k8shost"})
	assert.NilError(t, err)

	rvi, err := ops.parameterize(dummmyK8sProv, svc, params, map[string]interface{}{})
	assert.NilError(t, err)
	assert.Assert(t, rvi != nil)
	assert.Assert(t, rvi.Request.Body != nil)
	b, _ := io.ReadAll(rvi.Request.Body)
	if string(b) != "{}" {
		t.Fatalf("base fallback body = %q, want {}", string(b))
	}
	if got := rvi.Request.Header.Get("Content-Type"); got != "application/x-amz-json-1.0" {
		t.Errorf("Content-Type = %q, want application/x-amz-json-1.0", got)
	}
}

func loadK8sNodeSelectOp(t *testing.T) (StandardOperationStore, Service) {
	t.Helper()
	b, err := GetServiceDocBytes(fmt.Sprintf("k8s/%s/services/core_v1.yaml", "v0.1.0"), "")
	assert.NilError(t, err)
	l := newLoader()
	svc, err := l.loadFromBytes(b)
	assert.NilError(t, err)
	rsc, err := svc.GetResource("node")
	assert.NilError(t, err)
	ops, _, ok := rsc.GetFirstNamespaceMethodMatchFromSQLVerb("select", nil)
	assert.Assert(t, ok)
	return ops, svc
}

func mapKeys(m map[string]interface{}) []string {
	rv := make([]string, 0, len(m))
	for k := range m {
		rv = append(rv, k)
	}
	return rv
}

func stringSet(ss []string) map[string]struct{} {
	rv := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		rv[s] = struct{}{}
	}
	return rv
}
