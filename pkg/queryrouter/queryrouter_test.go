package queryrouter

import (
	"net/http"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

/*
CASE 1
Dot-separated S3 region
https://{Bucket}.s3.{region}.amazonaws.com
EXPECTED: works
*/
func TestServerTemplate_S3_DotRegion_Works(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Servers: openapi3.Servers{
			{
				URL: "https://{Bucket}.s3.{region}.amazonaws.com",
				Variables: map[string]*openapi3.ServerVariable{
					"Bucket": {Default: "example-bucket"},
					"region": {Default: "ap-southeast-2"},
				},
			},
		},
		Paths: openapi3.Paths{
			"/": &openapi3.PathItem{},
		},
	}

	if _, err := NewRouter(doc); err != nil {
		t.Fatalf("dot-region S3 template should work, got error: %v", err)
	}
}

/*
CASE 2
Dash-separated S3 region
https://{Bucket}.s3-{region}.amazonaws.com
EXPECTED: works
*/
func TestServerTemplate_S3_DashRegion_Fails(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Servers: openapi3.Servers{
			{
				URL: "https://{Bucket}.s3-{region}.amazonaws.com",
				Variables: map[string]*openapi3.ServerVariable{
					"Bucket": {Default: "example-bucket"},
					"region": {Default: "ap-southeast-2"},
				},
			},
		},
		Paths: openapi3.Paths{
			"/": &openapi3.PathItem{},
		},
	}

	if _, err := NewRouter(doc); err != nil {
		t.Fatalf("dash-region S3 template should work, got error: %v", err)
	}
}

/*
CASE 3
Regex-based cluster address template
{protocol}://{cluster_addr:^(?:[^\:/]+(?:\:[0-9]+)?|[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+(?:\:[0-9]+)?)$}/
EXPECTED: works
*/
func TestServerTemplate_RegexClusterAddr_Works(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Servers: openapi3.Servers{
			{
				URL: "{protocol}://{cluster_addr:^(?:[^\\:/]+(?:\\:[0-9]+)?|[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+(?:\\:[0-9]+)?)$}/",
				Variables: map[string]*openapi3.ServerVariable{
					"cluster_addr": {
						Default: "localhost",
					},
					"protocol": {
						Default: "https",
						Enum:    []string{"https", "http"},
					},
				},
			},
		},
		Paths: openapi3.Paths{
			"/": &openapi3.PathItem{},
		},
	}

	if _, err := NewRouter(doc); err != nil {
		t.Fatalf("regex cluster_addr server template should work, got error: %v", err)
	}
}

/*
CASE 4
Path parameter value contains forward slashes (resource-name style).
Template: /v1/{name}/keys
Request:  /v1/projects/p1/locations/us/keyRings/r1/keys
Expected: matches; name = "projects/p1/locations/us/keyRings/r1"
*/
func TestPathParam_SingleSlashyParam_Matches(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Servers: openapi3.Servers{{URL: "https://example.com"}},
		Paths: openapi3.Paths{
			"/v1/{name}/keys": &openapi3.PathItem{
				Get: &openapi3.Operation{
					Parameters: openapi3.Parameters{
						{Value: &openapi3.Parameter{Name: "name", In: "path", Required: true}},
					},
					Responses: openapi3.Responses{"200": &openapi3.ResponseRef{Value: &openapi3.Response{}}},
				},
			},
		},
	}
	r, err := NewRouter(doc)
	if err != nil {
		t.Fatalf("NewRouter failed: %v", err)
	}
	req, _ := http.NewRequest("GET", "https://example.com/v1/projects/p1/locations/us/keyRings/r1/keys", nil)
	route, vars, err := r.FindRoute(req)
	if err != nil {
		t.Fatalf("FindRoute failed for slashy path-param value: %v", err)
	}
	if route == nil {
		t.Fatalf("expected non-nil route")
	}
	if got, want := vars["name"], "projects/p1/locations/us/keyRings/r1"; got != want {
		t.Fatalf("captured name = %q, want %q", got, want)
	}
}

/*
CASE 5
Multiple path parameters separated by a literal segment, both values containing slashes.
Template: /v1/{parent}/locations/{location}
Request:  /v1/projects/p1/folders/f2/locations/us-central1/sub
Expected: greedy backtracking lands on the literal `/locations/`,
          giving parent = "projects/p1/folders/f2", location = "us-central1/sub".
*/
func TestPathParam_TwoParams_LiteralBetween_Matches(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Servers: openapi3.Servers{{URL: "https://example.com"}},
		Paths: openapi3.Paths{
			"/v1/{parent}/locations/{location}": &openapi3.PathItem{
				Get: &openapi3.Operation{
					Parameters: openapi3.Parameters{
						{Value: &openapi3.Parameter{Name: "parent", In: "path", Required: true}},
						{Value: &openapi3.Parameter{Name: "location", In: "path", Required: true}},
					},
					Responses: openapi3.Responses{"200": &openapi3.ResponseRef{Value: &openapi3.Response{}}},
				},
			},
		},
	}
	r, err := NewRouter(doc)
	if err != nil {
		t.Fatalf("NewRouter failed: %v", err)
	}
	req, _ := http.NewRequest("GET", "https://example.com/v1/projects/p1/folders/f2/locations/us-central1/sub", nil)
	_, vars, err := r.FindRoute(req)
	if err != nil {
		t.Fatalf("FindRoute failed: %v", err)
	}
	if got, want := vars["parent"], "projects/p1/folders/f2"; got != want {
		t.Fatalf("captured parent = %q, want %q", got, want)
	}
	if got, want := vars["location"], "us-central1/sub"; got != want {
		t.Fatalf("captured location = %q, want %q", got, want)
	}
}

/*
CASE 6
permitSlashesInPathParams: pure-string transform invariants.
*/
func TestPermitSlashesInPathParams_Rewrites(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/v1/{name}/keys", "/v1/{name:[^?#]+}/keys"},
		{"/v1/{parent}/locations/{location}", "/v1/{parent:[^?#]+}/locations/{location:[^?#]+}"},
		// already-explicit regex preserved
		{"/v1/{id:[0-9]+}", "/v1/{id:[0-9]+}"},
		// no placeholders
		{"/health", "/health"},
		// empty braces left as-is
		{"/v1/{}/x", "/v1/{}/x"},
		// unmatched brace tolerated
		{"/v1/{name", "/v1/{name"},
		// Ambiguous adjacency: BOTH placeholders left at mux's default `[^/]+`.
		{"/v1/{a}/{b}", "/v1/{a}/{b}"},
		{"/v1/{a}{b}", "/v1/{a}{b}"},
		// Mixed: ambiguous pair stays restrictive, anchored neighbour gets rewritten.
		{"/v1/{a}/{b}/x/{c}", "/v1/{a}/{b}/x/{c:[^?#]+}"},
	}
	for _, c := range cases {
		if got := permitSlashesInPathParams(c.in); got != c.want {
			t.Fatalf("permitSlashesInPathParams(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

/*
CASE 7
Ambiguous adjacency template `/{a}/{b}` keeps routing slash-free values
exactly as before — proves the selective skip is non-regressing for
existing specs.
*/
func TestPathParam_AdjacentPair_NoRegressionForSlashFreeValues(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Servers: openapi3.Servers{{URL: "https://example.com"}},
		Paths: openapi3.Paths{
			"/v1/{a}/{b}": &openapi3.PathItem{
				Get: &openapi3.Operation{
					Parameters: openapi3.Parameters{
						{Value: &openapi3.Parameter{Name: "a", In: "path", Required: true}},
						{Value: &openapi3.Parameter{Name: "b", In: "path", Required: true}},
					},
					Responses: openapi3.Responses{"200": &openapi3.ResponseRef{Value: &openapi3.Response{}}},
				},
			},
		},
	}
	r, err := NewRouter(doc)
	if err != nil {
		t.Fatalf("NewRouter failed: %v", err)
	}
	// Slash-free values: must match.
	req, _ := http.NewRequest("GET", "https://example.com/v1/foo/bar", nil)
	_, vars, err := r.FindRoute(req)
	if err != nil {
		t.Fatalf("FindRoute failed for slash-free values on adjacent template: %v", err)
	}
	if got := vars["a"]; got != "foo" {
		t.Fatalf("captured a = %q, want %q", got, "foo")
	}
	if got := vars["b"]; got != "bar" {
		t.Fatalf("captured b = %q, want %q", got, "bar")
	}
	// Slashy value: must NOT match — the adjacency check is documented as
	// the boundary where slash support breaks down.
	req2, _ := http.NewRequest("GET", "https://example.com/v1/foo/extra/bar", nil)
	if _, _, err := r.FindRoute(req2); err == nil {
		t.Fatalf("expected FindRoute to fail for slashy value on adjacent template, got nil error")
	}
}
