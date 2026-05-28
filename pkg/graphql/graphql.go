package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/template"

	"github.com/stackql/any-sdk/pkg/client"
	"github.com/stackql/any-sdk/pkg/jsonpath"
	"github.com/stackql/any-sdk/pkg/stream_transform"
)

// httpLoggerCtxKey is the context key under which an optional io.Writer is
// attached so the GraphQL reader can emit the wire request body and the raw
// pre-transform response. Mirrors the REST acquire path which writes the same
// shape of lines to runtimeCtx.outErrFile when --http.log.enabled is set.
type httpLoggerCtxKey struct{}

// ContextWithHTTPLogger returns a derived context that carries w as the sink
// for GraphQL wire request / raw response log lines. Consumers (e.g. stackql)
// should attach the same writer they use for REST HTTP logging when
// --http.log.enabled is true. Passing a nil writer is equivalent to not
// attaching one.
func ContextWithHTTPLogger(ctx context.Context, w io.Writer) context.Context {
	if w == nil {
		return ctx
	}
	return context.WithValue(ctx, httpLoggerCtxKey{}, w)
}

func httpLoggerFromContext(ctx context.Context) io.Writer {
	if ctx == nil {
		return nil
	}
	v := ctx.Value(httpLoggerCtxKey{})
	if v == nil {
		return nil
	}
	w, _ := v.(io.Writer)
	return w
}

var (
	_ template.ExecError = template.ExecError{}
)

// CursorStrategy enumerates the pluggable pagination shapes supported by
// StandardGQLReader. The empty string is treated as CursorStrategyAfter to
// preserve back-compat with specs that predate this field.
type CursorStrategy string

const (
	// CursorStrategyAfter is the default Relay-style ", after: \"<value>\""
	// cursor splice. Termination is signalled by an empty/non-scalar cursor.
	CursorStrategyAfter CursorStrategy = "cursor_after"
	// CursorStrategyKeyset injects a filter comparator (typically "_gt") on
	// the last row's sort key. Used by Cloudflare GraphQL Analytics and
	// similar APIs that do not support cursors. Termination is signalled by
	// an empty response row array.
	CursorStrategyKeyset CursorStrategy = "keyset"
	// CursorStrategyOffset synthesises a running row count client-side and
	// substitutes it as ", offset: <count>". Termination is signalled by an
	// empty response row array (or a short page if PageSize is configured).
	CursorStrategyOffset CursorStrategy = "offset"
	// CursorStrategyPageInfo reads ", after: \"<endCursor>\"" from a
	// jsonpath but terminates on a separate boolean termination flag
	// (typically `pageInfo.hasNextPage`) — required for Relay-strict
	// endpoints where the cursor remains non-empty on the final page.
	CursorStrategyPageInfo CursorStrategy = "page_info"
)

// CursorConfig configures a pagination strategy for StandardGQLReader. An
// empty Strategy defaults to CursorStrategyAfter, which is byte-identical to
// the pre-strategy behaviour.
type CursorConfig struct {
	// Strategy selects the pagination shape. Empty == CursorStrategyAfter.
	Strategy CursorStrategy
	// JSONPath is the value-source path for the cursor. Required for
	// cursor_after, keyset, and page_info; ignored for offset.
	JSONPath string
	// FormatTemplate is an optional Go text/template rendered into
	// iterativeInput["cursor"] each iteration. Bindings depend on strategy:
	//   - cursor_after / offset / page_info: {{ .value }}
	//   - keyset:                            {{ .value }} when the JSON path
	//                                        resolves to a scalar; the
	//                                        object's fields by name when it
	//                                        resolves to a map
	// When empty, strategy-specific defaults apply (see strategy docs).
	FormatTemplate string
	// TerminateOnJSONPath is the jsonpath inspected for a termination flag
	// after each page. Only used by CursorStrategyPageInfo.
	TerminateOnJSONPath string
	// PageSize, when > 0, terminates CursorStrategyOffset early when a page
	// returns fewer rows than this value. Ignored by other strategies.
	PageSize int
}

// NewStandardGQLReader constructs a StandardGQLReader with the default
// CursorStrategyAfter strategy. Behaviour is identical to the pre-strategy
// reader.
func NewStandardGQLReader(
	anySdkClient client.AnySdkClient,
	request *http.Request,
	httpPageLimit int,
	baseQuery string,
	constInput map[string]interface{},
	initialCursor string,
	responseJsonPath string,
	latestCursorJsonPath string,
) (GQLReader, error) {
	return NewStandardGQLReaderFull(
		anySdkClient,
		request,
		httpPageLimit,
		baseQuery,
		constInput,
		initialCursor,
		responseJsonPath,
		CursorConfig{Strategy: CursorStrategyAfter, JSONPath: latestCursorJsonPath},
		"",
		"",
	)
}

// NewStandardGQLReaderWithTransform constructs a StandardGQLReader that optionally
// applies a stream_transform template to the raw response body before the existing
// responseJsonPath / latestCursorJsonPath selection runs. Passing "" for both
// transformType and transformBody yields behavior identical to NewStandardGQLReader.
func NewStandardGQLReaderWithTransform(
	anySdkClient client.AnySdkClient,
	request *http.Request,
	httpPageLimit int,
	baseQuery string,
	constInput map[string]interface{},
	initialCursor string,
	responseJsonPath string,
	latestCursorJsonPath string,
	transformType string,
	transformBody string,
) (GQLReader, error) {
	return NewStandardGQLReaderFull(
		anySdkClient,
		request,
		httpPageLimit,
		baseQuery,
		constInput,
		initialCursor,
		responseJsonPath,
		CursorConfig{Strategy: CursorStrategyAfter, JSONPath: latestCursorJsonPath},
		transformType,
		transformBody,
	)
}

// NewStandardGQLReaderWithCursor constructs a StandardGQLReader using the
// supplied CursorConfig. Supplying CursorConfig{} (Strategy == "") yields the
// default cursor_after behaviour.
func NewStandardGQLReaderWithCursor(
	anySdkClient client.AnySdkClient,
	request *http.Request,
	httpPageLimit int,
	baseQuery string,
	constInput map[string]interface{},
	initialCursor string,
	responseJsonPath string,
	cursor CursorConfig,
) (GQLReader, error) {
	return NewStandardGQLReaderFull(
		anySdkClient,
		request,
		httpPageLimit,
		baseQuery,
		constInput,
		initialCursor,
		responseJsonPath,
		cursor,
		"",
		"",
	)
}

// NewStandardGQLReaderFull is the most general constructor; the narrower
// constructors are thin wrappers around it.
func NewStandardGQLReaderFull(
	anySdkClient client.AnySdkClient,
	request *http.Request,
	httpPageLimit int,
	baseQuery string,
	constInput map[string]interface{},
	initialCursor string,
	responseJsonPath string,
	cursor CursorConfig,
	transformType string,
	transformBody string,
) (GQLReader, error) {
	tmpl, err := template.New("gqlTmpl").Parse(baseQuery)
	if err != nil {
		return nil, err
	}
	if cursor.Strategy == "" {
		cursor.Strategy = CursorStrategyAfter
	}
	if err := validateCursorConfig(cursor); err != nil {
		return nil, err
	}
	var cursorTmpl *template.Template
	if cursor.FormatTemplate != "" {
		cursorTmpl, err = template.New("gqlCursorTmpl").Parse(cursor.FormatTemplate)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor.format template: %w", err)
		}
	}
	rv := &StandardGQLReader{
		anySdkClient:     anySdkClient,
		baseQuery:        baseQuery,
		httpPageLimit:    httpPageLimit,
		constInput:       constInput,
		responseJsonPath: responseJsonPath,
		queryTemplate:    tmpl,
		request:          request,
		pageCount:        1,
		iterativeInput:   make(map[string]interface{}),
		transformType:    transformType,
		transformBody:    transformBody,
		cursorConfig:     cursor,
		cursorTemplate:   cursorTmpl,
	}
	for k, v := range constInput {
		rv.iterativeInput[k] = v
	}
	rv.iterativeInput["cursor"] = initialCursor
	return rv, nil
}

func validateCursorConfig(c CursorConfig) error {
	switch c.Strategy {
	case CursorStrategyAfter, CursorStrategyKeyset, CursorStrategyOffset, CursorStrategyPageInfo:
		// known
	default:
		return fmt.Errorf("unknown cursor strategy: %q", c.Strategy)
	}
	if c.Strategy == CursorStrategyKeyset && c.FormatTemplate == "" {
		return fmt.Errorf("cursor strategy %q requires cursor.format template", c.Strategy)
	}
	if c.Strategy == CursorStrategyPageInfo && c.TerminateOnJSONPath == "" {
		return fmt.Errorf("cursor strategy %q requires cursor.terminateOnJsonPath", c.Strategy)
	}
	return nil
}

type StandardGQLReader struct {
	baseQuery        string
	constInput       map[string]interface{}
	iterativeInput   map[string]interface{}
	anySdkClient     client.AnySdkClient
	httpPageLimit    int
	queryTemplate    *template.Template
	responseJsonPath string
	request          *http.Request
	pageCount        int
	transformType    string
	transformBody    string
	cursorConfig     CursorConfig
	cursorTemplate   *template.Template
	rowsReturned     int
}

type anySdkGraphQLHTTPDesignation struct {
	url *url.URL
}

func newAnySdkGraphQLHTTPDesignation(url *url.URL) client.AnySdkDesignation {
	return &anySdkGraphQLHTTPDesignation{
		url: url,
	}
}

func (hd *anySdkGraphQLHTTPDesignation) GetDesignation() (interface{}, bool) {
	return hd.url, hd.url != nil
}

type graphqlAnySdkArgList struct {
	args []client.AnySdkArg
}

func (al *graphqlAnySdkArgList) GetArgs() []client.AnySdkArg {
	return al.args
}

func (al *graphqlAnySdkArgList) GetProtocolType() client.ClientProtocolType {
	return client.HTTP
}

func newGraphqlAnySdkArgList(args ...client.AnySdkArg) client.AnySdkArgList {
	return &graphqlAnySdkArgList{
		args: args,
	}
}

type anySdkHTTPArg struct {
	arg *http.Request
}

func (ha *anySdkHTTPArg) GetArg() (interface{}, bool) {
	return ha.arg, ha.arg != nil
}

func newAnySdkHTTPArg(arg *http.Request) client.AnySdkArg {
	return &anySdkHTTPArg{
		arg: arg,
	}
}

func (gq *StandardGQLReader) Read() ([]map[string]interface{}, error) {
	if gq.httpPageLimit > 0 && gq.pageCount >= gq.httpPageLimit {
		return nil, io.EOF
	}
	req := gq.request.Clone(gq.request.Context())
	rb, err := gq.renderQuery()
	if err != nil {
		return nil, err
	}
	req.Body = rb
	req.URL.RawQuery = ""
	req.Header.Set("Content-Type", "application/json")
	if logger := httpLoggerFromContext(req.Context()); logger != nil {
		bodyBytes, readErr := io.ReadAll(req.Body)
		if readErr == nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			fmt.Fprintf(logger, "http request url: '%s', method: '%s'\n", req.URL.String(), req.Method)
			fmt.Fprintf(logger, "http request body = '%s'\n", string(bodyBytes))
		}
	}
	r, err := gq.anySdkClient.Do(
		newAnySdkGraphQLHTTPDesignation(req.URL),
		newGraphqlAnySdkArgList(newAnySdkHTTPArg(req)),
	)
	if err != nil {
		return nil, err
	}
	httpResponse, httpResponseErr := r.GetHttpResponse()
	if httpResponseErr != nil {
		return nil, httpResponseErr
	}
	if logger := httpLoggerFromContext(req.Context()); logger != nil && httpResponse != nil && httpResponse.Body != nil {
		respBytes, readErr := io.ReadAll(httpResponse.Body)
		if readErr == nil {
			httpResponse.Body = io.NopCloser(bytes.NewReader(respBytes))
			fmt.Fprintf(logger, "%s\n", string(respBytes))
		}
	}
	gq.pageCount++
	var target map[string]interface{}
	err = json.NewDecoder(httpResponse.Body).Decode(&target)
	if err != nil {
		return nil, err
	}
	if gqlErr := extractGraphQLErrors(target); gqlErr != nil {
		return nil, gqlErr
	}
	if gq.transformType != "" && gq.transformBody != "" {
		target, err = gq.applyResponseTransform(target)
		if err != nil {
			return nil, err
		}
	}
	var returnErr error
	if len(target) == 0 {
		returnErr = io.EOF
	}
	processedResponse, err := jsonpath.Get(gq.responseJsonPath, target)
	if err != nil {
		return nil, err
	}
	var rv []map[string]interface{}
	switch pr := processedResponse.(type) {
	case []interface{}:
		for _, v := range pr {
			switch v := v.(type) {
			case map[string]interface{}:
				rv = append(rv, v)
			default:
				return nil, fmt.Errorf("cannot accommodate GraphQL processed response item of type = '%T'", v)
			}
		}
	default:
		return nil, fmt.Errorf("cannot accommodate GraphQL processed response of type = '%T'", pr)
	}
	gq.rowsReturned += len(rv)
	if returnErr == nil {
		if cursorErr := gq.advanceCursor(target, len(rv)); cursorErr != nil {
			returnErr = cursorErr
		}
	}
	return rv, returnErr
}

// advanceCursor dispatches to the per-strategy helper. It either updates
// gq.iterativeInput["cursor"] for the next page, returns io.EOF to terminate
// iteration, or returns a non-EOF error to abort with a hard failure.
func (gq *StandardGQLReader) advanceCursor(target map[string]interface{}, rowCount int) error {
	switch gq.cursorConfig.Strategy {
	case CursorStrategyAfter:
		return gq.advanceCursorAfter(target)
	case CursorStrategyKeyset:
		return gq.advanceKeyset(target, rowCount)
	case CursorStrategyOffset:
		return gq.advanceOffset(rowCount)
	case CursorStrategyPageInfo:
		return gq.advancePageInfo(target)
	}
	return fmt.Errorf("unknown cursor strategy: %q", gq.cursorConfig.Strategy)
}

func (gq *StandardGQLReader) advanceCursorAfter(target map[string]interface{}) error {
	cursorRaw, err := jsonpath.Get(gq.cursorConfig.JSONPath, target)
	if err != nil {
		return io.EOF
	}
	ct, ok := cursorRaw.([]interface{})
	if !ok || len(ct) != 1 {
		return io.EOF
	}
	if gq.cursorTemplate != nil {
		rendered, rerr := gq.renderCursorTemplate(map[string]interface{}{"value": ct[0]})
		if rerr != nil {
			return rerr
		}
		gq.iterativeInput["cursor"] = rendered
		return nil
	}
	switch c := ct[0].(type) {
	case string:
		gq.iterativeInput["cursor"] = fmt.Sprintf(`, after: "%s"`, c)
	default:
		gq.iterativeInput["cursor"] = fmt.Sprintf(`, after: %v`, c)
	}
	return nil
}

func (gq *StandardGQLReader) advanceKeyset(target map[string]interface{}, rowCount int) error {
	if rowCount == 0 {
		return io.EOF
	}
	cursorRaw, err := jsonpath.Get(gq.cursorConfig.JSONPath, target)
	if err != nil {
		return io.EOF
	}
	tmplCtx, ok := keysetTemplateContext(cursorRaw)
	if !ok {
		return io.EOF
	}
	rendered, err := gq.renderCursorTemplate(tmplCtx)
	if err != nil {
		return err
	}
	gq.iterativeInput["cursor"] = rendered
	return nil
}

// keysetTemplateContext turns a jsonpath result into the template binding map
// used by the keyset format template. It supports three shapes:
//   - scalar value             -> {"value": <v>}
//   - map of field-name->value -> field names + a "value" alias for single-field maps
//   - 1-element slice wrapping either of the above (the common JSONPath slice form)
func keysetTemplateContext(raw interface{}) (map[string]interface{}, bool) {
	switch v := raw.(type) {
	case []interface{}:
		if len(v) == 0 {
			return nil, false
		}
		return keysetTemplateContext(v[0])
	case map[string]interface{}:
		if len(v) == 0 {
			return nil, false
		}
		ctx := make(map[string]interface{}, len(v)+1)
		for k, vv := range v {
			ctx[k] = vv
		}
		if len(v) == 1 {
			for _, vv := range v {
				ctx["value"] = vv
			}
		}
		return ctx, true
	default:
		return map[string]interface{}{"value": v}, true
	}
}

func (gq *StandardGQLReader) advanceOffset(rowCount int) error {
	if rowCount == 0 {
		return io.EOF
	}
	if gq.cursorConfig.PageSize > 0 && rowCount < gq.cursorConfig.PageSize {
		return io.EOF
	}
	tmplCtx := map[string]interface{}{"value": gq.rowsReturned}
	if gq.cursorTemplate != nil {
		rendered, err := gq.renderCursorTemplate(tmplCtx)
		if err != nil {
			return err
		}
		gq.iterativeInput["cursor"] = rendered
		return nil
	}
	gq.iterativeInput["cursor"] = fmt.Sprintf(`, offset: %d`, gq.rowsReturned)
	return nil
}

func (gq *StandardGQLReader) advancePageInfo(target map[string]interface{}) error {
	hasNext, err := jsonpath.Get(gq.cursorConfig.TerminateOnJSONPath, target)
	if err != nil {
		return io.EOF
	}
	if !isPageInfoContinue(hasNext) {
		return io.EOF
	}
	cursorRaw, err := jsonpath.Get(gq.cursorConfig.JSONPath, target)
	if err != nil {
		return io.EOF
	}
	val, ok := scalarFromJSONPath(cursorRaw)
	if !ok {
		return io.EOF
	}
	if gq.cursorTemplate != nil {
		rendered, rerr := gq.renderCursorTemplate(map[string]interface{}{"value": val})
		if rerr != nil {
			return rerr
		}
		gq.iterativeInput["cursor"] = rendered
		return nil
	}
	switch c := val.(type) {
	case string:
		gq.iterativeInput["cursor"] = fmt.Sprintf(`, after: "%s"`, c)
	default:
		gq.iterativeInput["cursor"] = fmt.Sprintf(`, after: %v`, c)
	}
	return nil
}

// isPageInfoContinue interprets the value at TerminateOnJSONPath. A boolean
// false or a nil value means terminate; anything else (including unwrapping
// a 1-element slice) means continue. Treating "absent" as terminate is the
// conservative choice — if the spec writer pointed us at a flag they expected
// to flip, we'd rather stop than spin forever.
func isPageInfoContinue(v interface{}) bool {
	switch t := v.(type) {
	case []interface{}:
		if len(t) == 0 {
			return false
		}
		return isPageInfoContinue(t[0])
	case bool:
		return t
	case nil:
		return false
	default:
		return true
	}
}

// scalarFromJSONPath extracts a single scalar value from a jsonpath result,
// transparently unwrapping the 1-element slice form that arises from
// last-row selectors like `[-1:].field`.
func scalarFromJSONPath(v interface{}) (interface{}, bool) {
	switch t := v.(type) {
	case []interface{}:
		if len(t) != 1 {
			return nil, false
		}
		return t[0], true
	default:
		return v, true
	}
}

func (gq *StandardGQLReader) renderCursorTemplate(ctx map[string]interface{}) (string, error) {
	var buf bytes.Buffer
	if err := gq.cursorTemplate.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("failed to render cursor.format template: %w", err)
	}
	return buf.String(), nil
}

// extractGraphQLErrors returns a non-nil error if the decoded response body
// carries a non-empty top-level `errors` array per the GraphQL spec response
// format. Strict policy: any non-empty `errors` array is treated as a hard
// failure, even when `data` is also populated (partial-failure case).
func extractGraphQLErrors(target map[string]interface{}) error {
	rawErrs, ok := target["errors"]
	if !ok {
		return nil
	}
	errs, ok := rawErrs.([]interface{})
	if !ok || len(errs) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		if em, ok := e.(map[string]interface{}); ok {
			if m, ok := em["message"].(string); ok && m != "" {
				msgs = append(msgs, m)
				continue
			}
		}
		b, _ := json.Marshal(e)
		msgs = append(msgs, string(b))
	}
	return fmt.Errorf("graphql error: %s", strings.Join(msgs, "; "))
}

func (gq *StandardGQLReader) applyResponseTransform(target map[string]interface{}) (map[string]interface{}, error) {
	factory := stream_transform.NewStreamTransformerFactory(gq.transformType, gq.transformBody)
	if !factory.IsTransformable() {
		return nil, fmt.Errorf("unsupported response.transform type for graphql: %s", gq.transformType)
	}
	inputBytes, err := json.Marshal(target)
	if err != nil {
		return nil, err
	}
	tfm, err := factory.GetTransformer(string(inputBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to build graphql response transformer: %w", err)
	}
	if err := tfm.Transform(); err != nil {
		return nil, fmt.Errorf("graphql response transform failed: %w", err)
	}
	outBytes, err := io.ReadAll(tfm.GetOutStream())
	if err != nil {
		return nil, err
	}
	var transformed map[string]interface{}
	if err := json.Unmarshal(outBytes, &transformed); err != nil {
		return nil, fmt.Errorf("graphql response transform produced invalid JSON: %w", err)
	}
	return transformed, nil
}

func (gq *StandardGQLReader) renderQuery() (io.ReadCloser, error) {
	var tplWr bytes.Buffer
	if err := gq.queryTemplate.Execute(&tplWr, gq.iterativeInput); err != nil {
		return nil, err
	}
	s := strings.ReplaceAll(tplWr.String(), "\n", "")
	payload := fmt.Sprintf(`{ "query": "%s" }`, strings.ReplaceAll(s, `"`, `\"`))
	return io.NopCloser(bytes.NewReader([]byte(payload))), nil
}
