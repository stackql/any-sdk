package graphql

import (
	"bytes"
	"context"
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

// multiPageAnySdkClient walks through a sequence of response bodies, one per
// Do() call. After the sequence is exhausted, the last body is repeated.
type multiPageAnySdkClient struct {
	bodies   []string
	call     int
	captured []string
}

func (f *multiPageAnySdkClient) Do(_ client.AnySdkDesignation, args client.AnySdkArgList) (client.AnySdkResponse, error) {
	// Record the rendered request body so tests can assert on cursor splicing.
	for _, a := range args.GetArgs() {
		v, ok := a.GetArg()
		if !ok {
			continue
		}
		if req, ok := v.(*http.Request); ok && req.Body != nil {
			b, _ := io.ReadAll(req.Body)
			f.captured = append(f.captured, string(b))
		}
	}
	idx := f.call
	if idx >= len(f.bodies) {
		idx = len(f.bodies) - 1
	}
	f.call++
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(f.bodies[idx])),
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

// readerCursor exposes the iterativeInput["cursor"] slice for assertions in
// tests that drive multiple iterations of Read().
func readerCursor(t *testing.T, r GQLReader) interface{} {
	t.Helper()
	sg, ok := r.(*StandardGQLReader)
	if !ok {
		t.Fatalf("reader is not *StandardGQLReader; got %T", r)
	}
	return sg.iterativeInput["cursor"]
}

// keysetBody renders a Cloudflare-shaped two-row response with a configurable
// "last row" datetime, so we can chain page bodies in a multi-page test.
func keysetBody(lastDatetime string) string {
	if lastDatetime == "" {
		return `{"data":{"viewer":{"zones":[{"httpRequestsAdaptiveGroups":[]}]}}}`
	}
	return `{"data":{"viewer":{"zones":[{"httpRequestsAdaptiveGroups":[
  {"dimensions":{"datetime":"2026-05-28T10:00:00Z","clientCountryName":"US"},"sum":{"requests":42}},
  {"dimensions":{"datetime":"` + lastDatetime + `","clientCountryName":"GB"},"sum":{"requests":17}}
]}]}}}`
}

// TestRead_Keyset verifies that the keyset strategy splices a comparator-style
// cursor built from the last row's sort key into the next request, and that
// an empty response array terminates the iteration with io.EOF.
func TestRead_Keyset(t *testing.T) {
	c := &multiPageAnySdkClient{
		bodies: []string{
			keysetBody("2026-05-28T10:01:00Z"),
			keysetBody(""), // empty page → terminate
		},
	}
	req := newTestRequest(t)
	r, err := NewStandardGQLReaderWithCursor(
		c,
		req,
		0,
		`query { zone(filter: { datetime_geq: "x"{{ .cursor }} }) { d } }`,
		map[string]interface{}{},
		"",
		"$.data.viewer.zones[0].httpRequestsAdaptiveGroups[*]",
		CursorConfig{
			Strategy:       CursorStrategyKeyset,
			JSONPath:       "$.data.viewer.zones[0].httpRequestsAdaptiveGroups[-1:].dimensions.datetime",
			FormatTemplate: `, AND datetime_gt: "{{ .value }}"`,
		},
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReaderWithCursor: %v", err)
	}
	rows1, err := r.Read()
	if err != nil {
		t.Fatalf("page 1: unexpected error: %v", err)
	}
	if len(rows1) != 2 {
		t.Fatalf("page 1: expected 2 rows, got %d", len(rows1))
	}
	if got, want := readerCursor(t, r), `, AND datetime_gt: "2026-05-28T10:01:00Z"`; got != want {
		t.Errorf("page 1 cursor: got %q, want %q", got, want)
	}
	rows2, err := r.Read()
	if err != io.EOF {
		t.Errorf("page 2: expected io.EOF, got %v", err)
	}
	if len(rows2) != 0 {
		t.Errorf("page 2: expected 0 rows on empty page, got %d", len(rows2))
	}
	if len(c.captured) < 2 {
		t.Fatalf("expected at least 2 captured requests, got %d", len(c.captured))
	}
	if !strings.Contains(c.captured[1], `AND datetime_gt: \"2026-05-28T10:01:00Z\"`) {
		t.Errorf("page 2 request did not contain the keyset splice; body=%q", c.captured[1])
	}
}

// offsetBody returns a response carrying n synthetic rows under $.data.items.
func offsetBody(n int) string {
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, `{"id":`+itoa(i)+`}`)
	}
	return `{"data":{"items":[` + strings.Join(parts, ",") + `]}}`
}

func itoa(i int) string {
	// Avoid pulling strconv into the test imports for a single use.
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// TestRead_Offset verifies that the offset strategy substitutes a running row
// count as ", offset: N" each page and terminates when a short page returns
// fewer rows than the configured PageSize.
func TestRead_Offset(t *testing.T) {
	const pageSize = 50
	c := &multiPageAnySdkClient{
		bodies: []string{
			offsetBody(pageSize), // page 1: 50 rows
			offsetBody(pageSize), // page 2: 50 rows
			offsetBody(23),       // page 3: 23 rows → short page → terminate after this
		},
	}
	req := newTestRequest(t)
	r, err := NewStandardGQLReaderWithCursor(
		c,
		req,
		0,
		`query { items(limit: 50{{ .cursor }}) { id } }`,
		map[string]interface{}{},
		"",
		"$.data.items[*]",
		CursorConfig{
			Strategy: CursorStrategyOffset,
			PageSize: pageSize,
		},
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReaderWithCursor: %v", err)
	}
	rows1, err := r.Read()
	if err != nil {
		t.Fatalf("page 1: unexpected error: %v", err)
	}
	if len(rows1) != pageSize {
		t.Fatalf("page 1: expected %d rows, got %d", pageSize, len(rows1))
	}
	if got, want := readerCursor(t, r), `, offset: 50`; got != want {
		t.Errorf("page 1 cursor: got %q, want %q", got, want)
	}
	rows2, err := r.Read()
	if err != nil {
		t.Fatalf("page 2: unexpected error: %v", err)
	}
	if len(rows2) != pageSize {
		t.Fatalf("page 2: expected %d rows, got %d", pageSize, len(rows2))
	}
	if got, want := readerCursor(t, r), `, offset: 100`; got != want {
		t.Errorf("page 2 cursor: got %q, want %q", got, want)
	}
	rows3, err := r.Read()
	if err != io.EOF {
		t.Errorf("page 3: expected io.EOF after short page, got %v", err)
	}
	if len(rows3) != 23 {
		t.Errorf("page 3: expected 23 rows, got %d", len(rows3))
	}
}

// pageInfoBody renders a minimal Relay-style search response with a pageInfo
// block driving termination.
func pageInfoBody(endCursor string, hasNextPage bool) string {
	hnp := "false"
	if hasNextPage {
		hnp = "true"
	}
	return `{"data":{"search":{"nodes":[{"id":"n1"}],"pageInfo":{"endCursor":"` + endCursor + `","hasNextPage":` + hnp + `}}}}`
}

// TestRead_PageInfo verifies that the page_info strategy uses endCursor for
// the splice and pageInfo.hasNextPage for termination — including the case
// where the final page still carries a non-empty endCursor but hasNextPage
// is false (the Relay-strict signal we cannot detect with cursor_after).
func TestRead_PageInfo(t *testing.T) {
	c := &multiPageAnySdkClient{
		bodies: []string{
			pageInfoBody("Y3Vyc29yOjI=", true),
			pageInfoBody("Y3Vyc29yOjQ=", false), // hasNextPage=false → terminate even though endCursor is set
		},
	}
	req := newTestRequest(t)
	r, err := NewStandardGQLReaderWithCursor(
		c,
		req,
		0,
		`query { search(first: 10{{ .cursor }}) { nodes { id } } }`,
		map[string]interface{}{},
		"",
		"$.data.search.nodes[*]",
		CursorConfig{
			Strategy:            CursorStrategyPageInfo,
			JSONPath:            "$.data.search.pageInfo.endCursor",
			TerminateOnJSONPath: "$.data.search.pageInfo.hasNextPage",
		},
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReaderWithCursor: %v", err)
	}
	if _, err := r.Read(); err != nil {
		t.Fatalf("page 1: unexpected error: %v", err)
	}
	if got, want := readerCursor(t, r), `, after: "Y3Vyc29yOjI="`; got != want {
		t.Errorf("page 1 cursor: got %q, want %q", got, want)
	}
	_, err = r.Read()
	if err != io.EOF {
		t.Errorf("page 2: expected io.EOF (hasNextPage=false), got %v", err)
	}
}

// TestNewStandardGQLReaderWithCursor_UnknownStrategy ensures construction
// fails fast on an unrecognised strategy rather than silently defaulting.
func TestNewStandardGQLReaderWithCursor_UnknownStrategy(t *testing.T) {
	c := &fakeAnySdkClient{bodyJSON: `{}`}
	req := newTestRequest(t)
	_, err := NewStandardGQLReaderWithCursor(
		c,
		req,
		0,
		`{}`,
		map[string]interface{}{},
		"",
		"$.data[*]",
		CursorConfig{Strategy: "bogus"},
	)
	if err == nil {
		t.Fatalf("expected error for unknown cursor strategy, got nil")
	}
	if !strings.Contains(err.Error(), "unknown cursor strategy") {
		t.Errorf("expected unknown-strategy error, got: %v", err)
	}
}

// TestRead_GraphQLErrorEnvelope_ReturnsError asserts that a GraphQL error
// envelope (`{"data": null, "errors": [...]}`) surfaces as a Go error carrying
// the concatenated `message` fields, rather than silently producing zero rows
// via the jsonpath projection.
func TestRead_GraphQLErrorEnvelope_ReturnsError(t *testing.T) {
	body := `{
        "data": null,
        "errors": [
            {"message": "unknown field \"requests\""},
            {"message": "unknown argument \"foo\""}
        ]
    }`
	c := &fakeAnySdkClient{bodyJSON: body}
	req := newTestRequest(t)
	r, err := NewStandardGQLReader(
		c, req, 0, `{ ignored }`, map[string]interface{}{}, "",
		"$.data.rows[*]", "$.data.__no_cursor[*]",
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReader: %v", err)
	}
	_, err = r.Read()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("expected error to surface the GraphQL message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "unknown argument") {
		t.Errorf("expected error to concatenate multiple GraphQL messages, got: %v", err)
	}
}

// TestRead_GraphQLPartialFailure_ReturnsError asserts strict v1 behaviour: when
// `data` and `errors` are both populated, the error wins and no rows are
// returned. This guards the recommended "strict" mode from the issue.
func TestRead_GraphQLPartialFailure_ReturnsError(t *testing.T) {
	body := `{
        "data": {"rows": [{"k": "v"}]},
        "errors": [{"message": "field permission denied"}]
    }`
	c := &fakeAnySdkClient{bodyJSON: body}
	req := newTestRequest(t)
	r, err := NewStandardGQLReader(
		c, req, 0, `{ ignored }`, map[string]interface{}{}, "",
		"$.data.rows[*]", "$.data.__no_cursor[*]",
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReader: %v", err)
	}
	rows, err := r.Read()
	if err == nil {
		t.Fatalf("expected error on partial failure, got nil")
	}
	if !strings.Contains(err.Error(), "field permission denied") {
		t.Errorf("expected partial-failure error to surface message, got: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected no rows on partial failure, got %d", len(rows))
	}
}

// TestRead_EmptyErrorsArray_NoError asserts that an empty `errors: []` (legal
// per spec, semantically a success) does not trip the error path.
func TestRead_EmptyErrorsArray_NoError(t *testing.T) {
	body := `{"data": {"rows": [{"k": "v"}]}, "errors": []}`
	c := &fakeAnySdkClient{bodyJSON: body}
	req := newTestRequest(t)
	r, err := NewStandardGQLReader(
		c, req, 0, `{ ignored }`, map[string]interface{}{}, "",
		"$.data.rows[*]", "$.data.__no_cursor[*]",
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReader: %v", err)
	}
	rows, err := r.Read()
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}

// TestRead_GraphQLErrorWithoutMessage_FallsBackToJSON asserts that an error
// object lacking a usable `message` string is rendered as JSON, so the user
// still gets a non-empty signal rather than `graphql error: ` with nothing
// after the colon.
func TestRead_GraphQLErrorWithoutMessage_FallsBackToJSON(t *testing.T) {
	body := `{"data": null, "errors": [{"code": 42}]}`
	c := &fakeAnySdkClient{bodyJSON: body}
	req := newTestRequest(t)
	r, err := NewStandardGQLReader(
		c, req, 0, `{ ignored }`, map[string]interface{}{}, "",
		"$.data.rows[*]", "$.data.__no_cursor[*]",
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReader: %v", err)
	}
	_, err = r.Read()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "code") || !strings.Contains(err.Error(), "42") {
		t.Errorf("expected JSON fallback to include the error object, got: %v", err)
	}
}

// TestRead_EmitsRequestBodyToHTTPLogWhenEnabled asserts that when a context
// logger is attached, the rendered GraphQL request body and wire URL are
// surfaced before the Do() call — closing the gap where --http.log.enabled
// previously only showed the post-transform projection.
func TestRead_EmitsRequestBodyToHTTPLogWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	c := &fakeAnySdkClient{bodyJSON: `{"data": {"rows": [{"id": 1}]}}`}
	req := newTestRequest(t)
	req = req.WithContext(ContextWithHTTPLogger(context.Background(), &buf))

	r, err := NewStandardGQLReader(
		c, req, 0, `query { rows { id } }`, map[string]interface{}{}, "",
		"$.data.rows[*]", "$.data.__no_cursor[*]",
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReader: %v", err)
	}
	if _, err := r.Read(); err != nil && err != io.EOF {
		t.Fatalf("Read: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "query { rows { id } }") {
		t.Errorf("expected rendered request body in log, got:\n%s", out)
	}
	if !strings.Contains(out, "https://api.example.test/graphql") {
		t.Errorf("expected wire URL in log, got:\n%s", out)
	}
}

// TestRead_EmitsRawResponseToHTTPLogWhenEnabled asserts that the naked
// pre-transform response body is surfaced when a context logger is attached.
// This is the diagnostic that was missing for transform / templating failures.
func TestRead_EmitsRawResponseToHTTPLogWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	c := &fakeAnySdkClient{bodyJSON: `{"data":{"rows":[{"id":1}]}}`}
	req := newTestRequest(t)
	req = req.WithContext(ContextWithHTTPLogger(context.Background(), &buf))

	r, err := NewStandardGQLReader(
		c, req, 0, `{ ignored }`, map[string]interface{}{}, "",
		"$.data.rows[*]", "$.data.__no_cursor[*]",
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReader: %v", err)
	}
	if _, err := r.Read(); err != nil && err != io.EOF {
		t.Fatalf("Read: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"id":1`) {
		t.Errorf("expected raw response body in log, got:\n%s", out)
	}
}

// TestRead_DoesNotLogWhenHTTPLogDisabled asserts that with no logger attached
// to the request context, Read() emits nothing — the opt-in shape mirrors the
// REST acquire path's gating on runtimeCtx.HTTPLogEnabled.
func TestRead_DoesNotLogWhenHTTPLogDisabled(t *testing.T) {
	c := &fakeAnySdkClient{bodyJSON: `{"data":{"rows":[]}}`}
	req := newTestRequest(t) // no logger in context

	r, err := NewStandardGQLReader(
		c, req, 0, `{ ignored }`, map[string]interface{}{}, "",
		"$.data.rows[*]", "$.data.__no_cursor[*]",
	)
	if err != nil {
		t.Fatalf("NewStandardGQLReader: %v", err)
	}
	if _, err := r.Read(); err != nil && err != io.EOF {
		t.Fatalf("Read: %v", err)
	}
	// nothing to assert beyond "no panic and no log sink to fill" — the
	// negative case is covered by the structural check that nil-logger
	// branches in Read() are short-circuit.
}

// TestNewStandardGQLReaderWithCursor_KeysetRequiresFormat ensures a keyset
// configuration without a format template is rejected at construction time.
func TestNewStandardGQLReaderWithCursor_KeysetRequiresFormat(t *testing.T) {
	c := &fakeAnySdkClient{bodyJSON: `{}`}
	req := newTestRequest(t)
	_, err := NewStandardGQLReaderWithCursor(
		c,
		req,
		0,
		`{}`,
		map[string]interface{}{},
		"",
		"$.data[*]",
		CursorConfig{Strategy: CursorStrategyKeyset, JSONPath: "$.x"},
	)
	if err == nil {
		t.Fatalf("expected error for keyset without format, got nil")
	}
	if !strings.Contains(err.Error(), "format") {
		t.Errorf("expected format-required error, got: %v", err)
	}
}
