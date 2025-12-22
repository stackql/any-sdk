package queryrouter

import (
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
