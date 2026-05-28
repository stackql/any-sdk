package graphql

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stackql/any-sdk/pkg/client"
)

// fakeAnySdkResponse implements client.AnySdkResponse and returns a canned http.Response.
type fakeAnySdkResponse struct {
	resp *http.Response
}

func (f *fakeAnySdkResponse) IsErroneous() bool                       { return false }
func (f *fakeAnySdkResponse) GetHttpResponse() (*http.Response, error) { return f.resp, nil }

// fakeAnySdkClient implements client.AnySdkClient by returning a canned response body.
type fakeAnySdkClient struct {
	bodyJSON string
}

func (f *fakeAnySdkClient) Do(client.AnySdkDesignation, client.AnySdkArgList) (client.AnySdkResponse, error) {
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(f.bodyJSON)),
		Header:     http.Header{},
	}
	return &fakeAnySdkResponse{resp: resp}, nil
}

// cloudflareLikeBody is a minimal fixture mimicking the nested shape of a Cloudflare
// GraphQL Analytics httpRequestsAdaptiveGroups response: one row under
// data.viewer.zones[0].httpRequestsAdaptiveGroups with dimensions{}, sum{}, count.
const cloudflareLikeBody = `{
  "data": {
    "viewer": {
      "zones": [
        {
          "httpRequestsAdaptiveGroups": [
            {
              "dimensions": {
                "datetime": "2026-05-28T00:00:00Z",
                "clientCountryName": "AU"
              },
              "sum": {
                "requests": 42,
                "threats": 1,
                "bytes": 1024
              },
              "count": 7
            }
          ]
        }
      ]
    }
  }
}`

// flattenTransform lifts the leaves of httpRequestsAdaptiveGroups[*] into a top-level rows array.
const flattenTransform = `{
  "rows": [
    {{- $s := separator ", " -}}
    {{- range $i, $row := index . "data" "viewer" "zones" 0 "httpRequestsAdaptiveGroups" -}}
    {{- call $s }}
    {
      "datetime": {{ toJson (index $row "dimensions" "datetime") }},
      "clientCountryName": {{ toJson (index $row "dimensions" "clientCountryName") }},
      "requests": {{ index $row "sum" "requests" }},
      "threats": {{ index $row "sum" "threats" }},
      "count": {{ index $row "count" }}
    }
    {{- end }}
  ]
}`

func newTestRequest(t *testing.T) *http.Request {
	t.Helper()
	u, _ := url.Parse("https://api.example.test/graphql")
	req, err := http.NewRequest("POST", u.String(), strings.NewReader(""))
	if err != nil {
		t.Fatalf("failed to construct test request: %v", err)
	}
	return req
}

// TestRead_AppliesResponseTransform asserts that when transformType/transformBody
// are supplied, the nested Cloudflare-shaped response is flattened before
// responseJsonPath selection runs.
func TestRead_AppliesResponseTransform(t *testing.T) {
	c := &fakeAnySdkClient{bodyJSON: cloudflareLikeBody}
	req := newTestRequest(t)

	r, err := NewStandardGQLReaderWithTransform(
		c,
		req,
		0,
		`{ ignored }`,
		map[string]interface{}{},
		"",
		"$.rows[*]",
		"$.__no_cursor[*]",
		"golang_template_json_v0.3.0",
		flattenTransform,
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReaderWithTransform: %v", err)
	}

	rows, err := r.Read()
	if err != nil && err != io.EOF {
		t.Fatalf("Read returned unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d (rows=%v)", len(rows), rows)
	}
	row := rows[0]
	if _, has := row["dimensions"]; has {
		t.Errorf("transformed row should not retain nested 'dimensions' key, got: %v", row)
	}
	if _, has := row["sum"]; has {
		t.Errorf("transformed row should not retain nested 'sum' key, got: %v", row)
	}
	if got, want := row["datetime"], "2026-05-28T00:00:00Z"; got != want {
		t.Errorf("datetime: got %v, want %v", got, want)
	}
	if got, want := row["clientCountryName"], "AU"; got != want {
		t.Errorf("clientCountryName: got %v, want %v", got, want)
	}
	// JSON numbers decode as float64 in map[string]interface{}.
	if got, want := row["requests"], float64(42); got != want {
		t.Errorf("requests: got %v (%T), want %v", got, got, want)
	}
	if got, want := row["threats"], float64(1); got != want {
		t.Errorf("threats: got %v (%T), want %v", got, got, want)
	}
	if got, want := row["count"], float64(7); got != want {
		t.Errorf("count: got %v (%T), want %v", got, got, want)
	}
}

// TestRead_BackCompatNoTransform asserts that when transformType is "", behavior
// is unchanged: rows from the original responseJsonPath are returned with their
// nested wrappers intact.
func TestRead_BackCompatNoTransform(t *testing.T) {
	c := &fakeAnySdkClient{bodyJSON: cloudflareLikeBody}
	req := newTestRequest(t)

	r, err := NewStandardGQLReader(
		c,
		req,
		0,
		`{ ignored }`,
		map[string]interface{}{},
		"",
		"$.data.viewer.zones[0].httpRequestsAdaptiveGroups[*]",
		"$.__no_cursor[*]",
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReader: %v", err)
	}

	rows, err := r.Read()
	if err != nil && err != io.EOF {
		t.Fatalf("Read returned unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if _, has := row["dimensions"]; !has {
		t.Errorf("expected nested 'dimensions' key to be preserved when no transform is set; row=%v", row)
	}
	if _, has := row["sum"]; !has {
		t.Errorf("expected nested 'sum' key to be preserved when no transform is set; row=%v", row)
	}
}

// TestRead_UnsupportedTransformType ensures a misconfigured transform type yields
// a clear error rather than silent fallthrough.
func TestRead_UnsupportedTransformType(t *testing.T) {
	c := &fakeAnySdkClient{bodyJSON: cloudflareLikeBody}
	req := newTestRequest(t)

	r, err := NewStandardGQLReaderWithTransform(
		c,
		req,
		0,
		`{ ignored }`,
		map[string]interface{}{},
		"",
		"$.rows[*]",
		"$.__no_cursor[*]",
		"not_a_real_transform_v0.0.0",
		"irrelevant",
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReaderWithTransform: %v", err)
	}

	_, err = r.Read()
	if err == nil {
		t.Fatalf("expected error for unsupported transform type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported response.transform type") {
		t.Errorf("expected unsupported-transform error, got: %v", err)
	}
}
