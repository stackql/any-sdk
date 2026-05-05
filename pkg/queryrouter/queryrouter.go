package queryrouter

/*
 * Abstraction layer for variations / extensions on
 * gorillamux routers as leveraged by "github.com/getkin/kin-openapi/openapi3":
 *   - Shim for routing with server variables.
 *   - Open for extension as per open/closed principle.
 *
 * This is highly derivative of "github.com/getkin/kin-openapi/openapi3",
 * and in particular https://github.com/getkin/kin-openapi/blob/c95dd68aef43fa9ac8c1f52f169b387d7681626a/routers/gorillamux/router.go
 *
 * "github.com/getkin/kin-openapi/openapi3" is
 * fully acknowledged (and greatly appreciated) in our licensing pages and README.
 */

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	"github.com/gorilla/mux"

	"github.com/stackql/any-sdk/pkg/urltranslate"
)

var _ routers.Router = &Router{}

type Router struct {
	muxes  []*mux.Route
	routes []*routers.Route
}

type srv struct {
	schemes []string
	host    urltranslate.QueryElement
	base    string
	server  *openapi3.Server
}

func NewRouter(doc *openapi3.T) (routers.Router, error) {
	docLevelServers, err := extractScopedSrvSlice(doc.Servers)
	if err != nil {
		return nil, err
	}
	if len(docLevelServers) == 0 {
		docLevelServers = append(docLevelServers, srv{})
	}
	muxRouter := mux.NewRouter().UseEncodedPath()
	r := &Router{}
	for _, path := range orderedPaths(doc.Paths) {
		pathItem := doc.Paths[path]
		pathLevelServers, err := extractScopedSrvSlice(pathItem.Servers)
		if err != nil {
			return nil, err
		}

		operations := pathItem.Operations()
		methods := make([]string, 0, len(operations))
		for method, operation := range operations {
			if operation.Servers != nil && len(*operation.Servers) > 0 {
				opServers, err := extractScopedSrvSlice(*operation.Servers)
				if err != nil {
					return nil, err
				}
				err = r.addRoutes(muxRouter, doc, opServers, path, pathItem, method)
				if err != nil {
					return nil, err
				}
			} else {
				methods = append(methods, method)
			}
		}
		sort.Strings(methods)

		servers := docLevelServers
		if len(pathLevelServers) > 0 {
			servers = pathLevelServers
		}

		err = r.addRoutes(muxRouter, doc, servers, path, pathItem, methods...)
		if err != nil {
			return nil, err
		}

	}
	return r, nil
}

func (r *Router) addRoutes(
	muxRouter *mux.Router,
	doc *openapi3.T,
	servers []srv,
	path string,
	pathItem *openapi3.PathItem,
	methods ...string,
) error {
	for _, s := range servers {
		qmIxd := strings.Index(path, "?")
		strippedPath := path
		if qmIxd > -1 {
			strippedPath = path[:qmIxd]
		}
		muxRoute := muxRouter.Path(s.base + permitSlashesInPathParams(strippedPath)).Methods(methods...)
		if qmIxd > -1 && len(path) > qmIxd {
			var pairs []string
			kvs := strings.Split(path[qmIxd+1:], "&")
			for _, v := range kvs {
				pair := strings.Split(v, "=")
				if len(pair) == 2 {
					pairs = append(pairs, pair...)
				}
			}
			muxRoute = muxRoute.Queries(pairs...).Methods(methods...)
		}
		if schemes := s.schemes; len(schemes) != 0 {
			muxRoute.Schemes(schemes...)
		}
		if host := s.host; host != nil && host.FullString() != "" {
			muxRoute.Host(host.FullString())
		}
		if err := muxRoute.GetError(); err != nil {
			return err
		}
		r.muxes = append(r.muxes, muxRoute)
		r.routes = append(r.routes, &routers.Route{
			Spec:      doc,
			Server:    s.server,
			Path:      path,
			PathItem:  pathItem,
			Method:    "",
			Operation: nil,
		})
	}
	return nil
}

func (r *Router) FindRoute(req *http.Request) (*routers.Route, map[string]string, error) {
	for i, muxRoute := range r.muxes {
		var match mux.RouteMatch
		if muxRoute.Match(req, &match) {
			if err := match.MatchErr; err != nil {
				// What then?
			}
			route := *r.routes[i]
			route.Method = req.Method
			route.Operation = route.Spec.Paths[route.Path].GetOperation(route.Method)
			return &route, match.Vars, nil
		}
		switch match.MatchErr {
		case nil:
		case mux.ErrMethodMismatch:
			// return nil, nil, routers.ErrMethodNotAllowed
		default: // What then?
		}
	}
	return nil, nil, routers.ErrPathNotFound
}

func orderedPaths(paths map[string]*openapi3.PathItem) []string {
	// https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.3.md#pathsObject
	// When matching URLs, concrete (non-templated) paths would be matched
	// before their templated counterparts.
	// NOTE: sorting by number of variables ASC then by descending lexicographical
	// order seems to be a good heuristic.
	vars := make(map[int][]string)
	max := 0
	for path := range paths {
		count := strings.Count(path, "}")
		vars[count] = append(vars[count], path)
		if count > max {
			max = count
		}
	}
	ordered := make([]string, 0, len(paths))
	for c := 0; c <= max; c++ {
		if ps, ok := vars[c]; ok {
			sort.Sort(sort.Reverse(sort.StringSlice(ps)))
			ordered = append(ordered, ps...)
		}
	}
	return ordered
}

// permitSlashesInPathParams rewrites OpenAPI-style `{name}` placeholders in a
// path template to the gorilla/mux form `{name:[^?#]+}`, which allows the
// captured value to span `/` characters. This is what enables path-parameter
// values like `projects/foo/locations/us` (a single OpenAPI parameter whose
// value happens to contain slashes) to route correctly through gorilla/mux —
// mux's default per-segment regex (`[^/]+`) would otherwise reject them.
//
// Placeholders are NOT rewritten when they participate in an ambiguous
// adjacency pair — i.e. two placeholders separated only by `/` characters
// (or nothing at all). For those, mux's greedy regex cannot disambiguate
// across slashes, so we preserve the default `[^/]+` matcher: existing
// slash-free values keep routing exactly as before, and slashy values still
// fail with `ErrPathNotFound` at runtime. The aot static analyser emits a
// warning (`path-param-adjacent` bin) for these templates so authors are
// notified — see docs/parameters_containing_slash.md.
//
// Placeholders that already carry an explicit regex (`{name:...}`) are left
// untouched, as are stray braces that are not well-formed placeholders.
func permitSlashesInPathParams(path string) string {
	type span struct{ start, end int } // inclusive indices of '{' and '}'
	var spans []span
	for i := 0; i < len(path); {
		open := strings.IndexByte(path[i:], '{')
		if open < 0 {
			break
		}
		open += i
		closeIdx := strings.IndexByte(path[open:], '}')
		if closeIdx < 0 {
			break
		}
		closeIdx += open
		spans = append(spans, span{open, closeIdx})
		i = closeIdx + 1
	}
	if len(spans) == 0 {
		return path
	}
	ambiguous := make([]bool, len(spans))
	for k := 0; k < len(spans)-1; k++ {
		between := path[spans[k].end+1 : spans[k+1].start]
		if strings.Trim(between, "/") == "" {
			ambiguous[k] = true
			ambiguous[k+1] = true
		}
	}
	var b strings.Builder
	b.Grow(len(path) + 8*len(spans))
	cursor := 0
	for k, sp := range spans {
		b.WriteString(path[cursor:sp.start])
		body := path[sp.start+1 : sp.end]
		simpleName := body != "" && !strings.ContainsRune(body, ':') && !strings.ContainsRune(body, '{')
		if simpleName && !ambiguous[k] {
			b.WriteByte('{')
			b.WriteString(body)
			b.WriteString(":[^?#]+}")
		} else {
			b.WriteString(path[sp.start : sp.end+1])
		}
		cursor = sp.end + 1
	}
	b.WriteString(path[cursor:])
	return b.String()
}

// Magic strings that temporarily replace "{}" so net/url.Parse() works
var blURL, brURL = strings.Repeat("-", 50), strings.Repeat("_", 50)

func bEncode(s string) string {
	s = strings.Replace(s, "{", blURL, -1)
	s = strings.Replace(s, "}", brURL, -1)
	return s
}
func bDecode(s string) string {
	s = strings.Replace(s, blURL, "{", -1)
	s = strings.Replace(s, brURL, "}", -1)
	return s
}

func permutePart(part0 string, srv *openapi3.Server) []string {
	type mapAndSlice struct {
		m map[string]struct{}
		s []string
	}
	var2val := make(map[string]mapAndSlice)
	max := 0
	for name0, v := range srv.Variables {
		name := "{" + name0 + "}"
		if !strings.Contains(part0, name) {
			continue
		}
		m := map[string]struct{}{v.Default: {}}
		for _, value := range v.Enum {
			m[value] = struct{}{}
		}
		if l := len(m); l > max {
			max = l
		}
		s := make([]string, 0, len(m))
		for value := range m {
			s = append(s, value)
		}
		var2val[name] = mapAndSlice{m: m, s: s}
	}
	if len(var2val) == 0 {
		return []string{part0}
	}

	partsMap := make(map[string]struct{}, max*len(var2val))
	for i := 0; i < max; i++ {
		part := part0
		for name, mas := range var2val {
			part = strings.Replace(part, name, mas.s[i%len(mas.s)], -1)
		}
		partsMap[part] = struct{}{}
	}
	parts := make([]string, 0, len(partsMap))
	for part := range partsMap {
		parts = append(parts, part)
	}
	sort.Strings(parts)
	return parts
}

func extractScopedSrvSlice(serverSlice openapi3.Servers) ([]srv, error) {
	rv := make([]srv, 0, len(serverSlice))
	for _, server := range serverSlice {
		enrichedServer, err := extractSrv(server)
		if err != nil {
			return nil, err
		}
		rv = append(rv, enrichedServer)
	}
	return rv, nil
}

func extractSrv(server *openapi3.Server) (srv, error) {
	var retVal srv
	serverURLParameterised, err := urltranslate.ExtractParameterisedURL(server.URL)
	if err != nil {
		return retVal, err
	}
	serverURL := serverURLParameterised.String()
	var schemes []string

	// --- NEW: derive HOST from String(), not net/url ---
	hostPort := serverURLParameterised.String()
	if i := strings.Index(hostPort, "://"); i >= 0 {
		hostPort = hostPort[i+3:]
	}
	if j := strings.Index(hostPort, "/"); j >= 0 {
		hostPort = hostPort[:j]
	}
	urlHost, err := urltranslate.ParseURLHost(hostPort)
	if err != nil {
		return retVal, err
	}
	// --- END NEW ---

	// --- KEEP net/url.Parse ONLY for PATH ---
	var u *url.URL
	if strings.Contains(serverURL, "://") {
		scheme0 := strings.Split(serverURL, "://")[0]
		schemes = permutePart(scheme0, server)
		u, err = url.Parse(bEncode(strings.Replace(serverURL, scheme0+"://", schemes[0]+"://", 1)))
	} else {
		u, err = url.Parse(bEncode(serverURL))
	}
	if err != nil {
		return retVal, err
	}
	path := bDecode(u.EscapedPath())
	if len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	// --- END PATH ---

	hostElem, ok := serverURLParameterised.GetElementByString(urlHost.GetHost())

	if !ok {
		return retVal, fmt.Errorf("element = '%s' unavailable in URL = '%s'", hostElem.FullString(), serverURLParameterised.Raw())
	}
	return srv{
		host:    hostElem, //u.Hostname()?
		base:    path,
		schemes: schemes, // scheme: []string{scheme0}, TODO: https://github.com/gorilla/mux/issues/624
		server:  server,
	}, nil
}
