# Path parameters containing forward slashes

Several upstream APIs use **resource-name** style path parameters whose value
itself contains `/` characters — for example, GCP's `name`:

```
projects/my-project/locations/us-central1/keyRings/my-ring
```

Until now, these values could not be carried through any-sdk's request flow.
This document records what changed, why, and the one authoring constraint the
fix imposes on OpenAPI specs.

## What used to break

The validation router used by `parameterize()` is built on
`github.com/gorilla/mux`. When a route like `/v1/{name}/keys` is registered,
mux compiles the placeholder `{name}` to its default per-segment regex,
`[^/]+`. A path-parameter value containing `/` therefore failed to match,
`router.FindRoute` returned `ErrPathNotFound`, and the request never left the
client — even though the substituted URL itself was correct.

Anatomy of the failure path:

- [internal/anysdk/operation_store.go:1506](../internal/anysdk/operation_store.go#L1506) — substitutes path-param values literally; slashes survive.
- [pkg/queryrouter/queryrouter.go:107](../pkg/queryrouter/queryrouter.go#L107) — `muxRouter.Path("/v1/{name}")` registers with the default `[^/]+` regex.
- [internal/anysdk/operation_store.go:1528](../internal/anysdk/operation_store.go#L1528) — `router.FindRoute(httpReq)` returns `ErrPathNotFound`.

## The fix

`pkg/queryrouter/queryrouter.go` rewrites OpenAPI placeholders in the path
template at route-registration time. The rewrite is *selective*: it applies
only to placeholders where mux can unambiguously split the captured value:

| Source template                       | Registered with mux as                                |
| ------------------------------------- | ----------------------------------------------------- |
| `/v1/{name}/keys`                     | `/v1/{name:[^?#]+}/keys`                              |
| `/v1/{parent}/locations/{location}`   | `/v1/{parent:[^?#]+}/locations/{location:[^?#]+}`     |
| `/v1/{id:[0-9]+}`                     | unchanged (already explicit)                          |
| `/v1/{a}/{b}` (ambiguous adjacency)   | unchanged (both keep mux's default `[^/]+`)           |
| `/v1/{a}/{b}/x/{c}`                   | `/v1/{a}/{b}/x/{c:[^?#]+}` (only `{c}` is rewritten)  |

The regex `[^?#]+` permits anything except the path-terminating characters
`?` (start of query) and `#` (start of fragment) — i.e. `/` is allowed inside
a captured value. mux's greedy matcher backtracks until the surrounding
literal segments line up, so `/v1/{parent}/locations/{location}` against
`/v1/projects/p1/folders/f2/locations/us-central1/sub` correctly resolves
to `parent = "projects/p1/folders/f2"` and `location = "us-central1/sub"`.

Substitution itself is unchanged — `replaceSimpleStringVars` still emits
literal `/` into the URL. That is what most resource-name APIs (GCP, Azure,
Vault, etc.) actually expect on the wire.

The rewrite lives in `permitSlashesInPathParams` in
[pkg/queryrouter/queryrouter.go](../pkg/queryrouter/queryrouter.go); behaviour is
covered by tests in
[pkg/queryrouter/queryrouter_test.go](../pkg/queryrouter/queryrouter_test.go).

## Known limitation: ambiguous adjacency

The rewrite relies on a literal segment between any two slashy path
parameters to anchor the regex split. If the template has two placeholders
with no disambiguating literal between them — e.g.

```
/v1/{a}/{b}        # only `/` between
/v1/{a}{b}         # nothing between
/v1/{a}//{b}       # only slashes between
```

— gorilla/mux's greedy regex cannot reliably split the captured values.

**For these cases the fix simply doesn't apply.** Both placeholders keep
mux's default `[^/]+` matcher: existing slash-free values keep routing
exactly as they did before this change, and slashy values still fail with
`ErrPathNotFound` at runtime — same as the pre-fix world. Other path
parameters in the same spec, in other operations, still benefit from the
rewrite normally.

### Surfacing in `aot`

The static analyser (`aot` CLI) runs `checkPathParamAdjacency` on every
operation. A template with ambiguous adjacency produces a **warning**-level
finding tagged with bin `path-param-adjacent` and an explanatory message
naming the offending template. The finding flows through both:

- the per-finding JSONL stream on stderr (`{"level":"warning","bin":"path-param-adjacent",...}`); and
- the stdout summary, where it is counted under `total_warnings` and binned
  under `bins["path-param-adjacent"]`.

The warning does **not** drive a non-zero exit code — it is informational,
since the runtime behaviour is unchanged. CI pipelines that want to gate on
this can grep for the bin name.

The check is implemented at
[public/discovery/method_analysis_checks.go](../public/discovery/method_analysis_checks.go)
(`checkPathParamAdjacency` + `hasAdjacentPathParams`); wiring is in
[public/discovery/static_analyzer.go](../public/discovery/static_analyzer.go)
(`analyzeMethod`).

### Remediations if you hit the warning and need slashy values

1. **Insert a literal segment.** If the API actually expects a literal
   between the two values (the usual case), put it back:
   `/v1/{parent}/locations/{location}` instead of `/v1/{parent}/{location}`.
2. **Collapse to a single parameter.** If the two values are always written
   together in the URL (resource-name style), merge them at the spec level:
   `/v1/{name}` where `name = "p/q/locations/us"`.
3. **Pin an explicit regex.** If you genuinely have a constrained format —
   for example `{id}` is always `[0-9]+` — write `{id:[0-9]+}` in the
   template. The rewrite preserves explicit regexes verbatim, so mux can
   disambiguate without backtracking.

## Server-side caveat

We deliberately do **not** percent-encode `/` to `%2F` during substitution.
That alternative was considered and rejected: the encoded form satisfies the
mux router but breaks for servers fronted by stacks that reject `%2F` in
request paths (older Apache with `AllowEncodedSlashes off` is the canonical
example). Sending the literal `/` matches what GCP, Azure, AWS resource ARNs
and most modern REST APIs actually accept.

If a target API ever requires the `%2F` form, the right place to opt in is a
per-parameter flag on the OpenAPI spec, encoded into the substitution layer
in `replaceSimpleStringVars`. That's not implemented today.
