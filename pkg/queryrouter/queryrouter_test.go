package queryrouter

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

/*
CASE 1 — WORKS
https://{Bucket}.s3.{region}.amazonaws.com
*/

func TestS3Host_DotRegion_Works(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Servers: openapi3.Servers{
			{
				URL: "https://{Bucket}.s3.{region}.amazonaws.com",
				Variables: map[string]*openapi3.ServerVariable{
					"Bucket": {
						Default: "example-bucket",
					},
					"region": {
						Default: "ap-southeast-2",
					},
				},
			},
		},
		Paths: openapi3.Paths{
			"/": &openapi3.PathItem{},
		},
	}

	_, err := NewRouter(doc)
	if err != nil {
		t.Fatalf("expected dot-region host to work, got error: %v", err)
	}
}

/*
CASE 2 — DOES NOT WORK
https://{Bucket}.s3-{region}.amazonaws.com
*/

func TestS3Host_DashRegion_Fails(t *testing.T) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Servers: openapi3.Servers{
			{
				URL: "https://{Bucket}.s3-{region}.amazonaws.com",
				Variables: map[string]*openapi3.ServerVariable{
					"Bucket": {
						Default: "example-bucket",
					},
					"region": {
						Default: "ap-southeast-2",
					},
				},
			},
		},
		Paths: openapi3.Paths{
			"/": &openapi3.PathItem{},
		},
	}

	_, err := NewRouter(doc)
	if err == nil {
		t.Fatalf("expected dash-region host to fail, but got nil error")
	}
}
