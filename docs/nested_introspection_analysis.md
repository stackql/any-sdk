# Nested schema introspection for any-sdk: analysis

Status: design draft for human review. No production code in this pass.
Scope: `any-sdk` is the schema-resolution layer; `stackql` core is the SQL
caller. Paths in this document are relative to the relevant repo root,
prefixed with `any-sdk/` or `stackql/` when they cross repos.

This is a working analysis. Where the prompt's framing differs from what
the code actually does, the discrepancy is called out inline rather than
papered over.

## 1. Inventory

### 1.1 Schema model

The Go-side schema lives in [internal/anysdk/schema.go](../internal/anysdk/schema.go).
The `Schema` interface (lines 29-108) is the wide façade used by both
`any-sdk` internals and `stackql` core. The single concrete implementation
is `standardSchema` at [schema.go:254-262](../internal/anysdk/schema.go#L254-L262),
which embeds `*openapi3.Schema` from `github.com/getkin/kin-openapi/openapi3`.

Through embedding, every native OpenAPI field is reachable on
`standardSchema`: `Properties`, `Items`, `AllOf`, `AnyOf`, `OneOf`, `Not`,
`Discriminator`, `Required`, `ReadOnly`, `WriteOnly`, `Deprecated`,
`Default`, `Enum`, `Example`, `AdditionalProperties`, `XML`, and the
`Extensions` map. `copyOpenapiSchema` at [schema.go:268-316](../internal/anysdk/schema.go#L268-L316)
enumerates all of them explicitly, which is a useful reference for "what
the loader knows is there."

What the `Schema` interface actually exposes is much narrower than the
underlying type:

- Properties access: `GetProperties`, `GetProperty`, `getProperty`,
  `GetPropertySchema`, `getRawProperty`, `setRawProperty`.
- Items access: `GetItems`, `GetItemsSchema`, `GetItemProperty`,
  `getItemsRef`, `setItemsRef`.
- AdditionalProperties: `GetAdditionalProperties` / `getAdditionalProperties`.
- Polymorphism: `getAllOf`, plus the internal `hasPolymorphicProperties`
  and `getFattnedPolymorphicSchema` ([schema.go:1178-1183](../internal/anysdk/schema.go#L1178-L1183),
  [schema.go:1302-1313](../internal/anysdk/schema.go#L1302-L1313)).
  `AnyOf` and `OneOf` are visible via `getAllSchemaRefsColumns` /
  `getAnyOfColumns` / `getOneOfColumns` at [schema.go:1022-1033](../internal/anysdk/schema.go#L1022-L1033)
  but only through the column-flattening path; there is no public
  `GetAnyOf` / `GetOneOf` on the `Schema` interface.
- Required: `IsRequired(key)`, `GetRequired()`.
- Read-only: `IsReadOnly()`. There is no `IsWriteOnly`, no `IsDeprecated`,
  no enum or default accessor, and no sensitive marker on the interface
  even though the underlying openapi3 fields are populated
  ([schema.go:295-299](../internal/anysdk/schema.go#L295-L299)).
- Vendor extensions: `getExtension(k)` at [schema.go:386-395](../internal/anysdk/schema.go#L386-L395)
  reads bytes out of `Extensions`. Known keys live in
  [const.go:28-36](../internal/anysdk/const.go#L28-L36): `x-alwaysRequired`,
  `x-stackQL-graphQL`, `x-stackQL-config`, `x-stackql-provider`,
  `x-stackQL-resources`, `x-stackQL-stringOnly`, `x-stackQL-alias`.
  There is no x-stackQL-sensitive, x-stackQL-readOnly etc. — OpenAPI's
  native `readOnly`/`writeOnly`/`deprecated` are the only such markers.

Discriminators are stored (via the openapi3 embed) but not consulted by
any traversal in the package — grep for `Discriminator` returns the
single line in `copyOpenapiSchema`.

Request and response shapes live in
[internal/anysdk/expectedRequest.go](../internal/anysdk/expectedRequest.go)
and [internal/anysdk/expectedResponse.go](../internal/anysdk/expectedResponse.go).
Each is a thin wrapper holding `Schema`, media type, required-property
names, and an optional `OverrideSchema` / `AsyncOverrideSchema` for
provider authors to swap in a custom shape.

### 1.2 Reference resolution

`$ref` resolution is driven by the upstream `kin-openapi` loader in
[internal/anysdk/loader.go](../internal/anysdk/loader.go). `standardLoader`
([loader.go:55-63](../internal/anysdk/loader.go#L55-L63)) embeds
`*openapi3.Loader`, which performs eager `$ref` resolution at load time.
By the time `any-sdk` is dealing with `*openapi3.Schema`, the `Value`
behind every `*openapi3.SchemaRef` is already populated; the `Ref`
string is kept on the side purely for traceability and naming.

Cycle detection at load time is built into `kin-openapi` and not
reimplemented in any-sdk. `standardLoader` does, however, carry its own
visited-sets to avoid re-processing operations during the merge passes:

- `visitedExpectedRequest`, `visitedExpectedResponse`,
  `visitedOperation`, `visitedOpenAPIOperationStore`, `visitedPathItem`
  ([loader.go:58-62](../internal/anysdk/loader.go#L58-L62)).

These keep `resolveExpectedRequest` / `resolveExpectedResponse` from
double-walking the same component, but they do *not* protect downstream
traversal of nested schemas. There is no schema-level cycle guard in
any-sdk's traversal code — `FindByPath` at
[schema.go:1315-1365](../internal/anysdk/schema.go#L1315-L1365) carries a
`visited map[string]bool` populated with `v.Ref` strings as it descends,
but that is the only place a cycle map is threaded through. `getDescendent`
([schema.go:670-689](../internal/anysdk/schema.go#L670-L689)),
`getProperties` ([schema.go:415-436](../internal/anysdk/schema.go#L415-L436)),
and the `getFatSchema` family ([schema.go:1065-1166](../internal/anysdk/schema.go#L1065-L1166))
do not carry one. In practice, a self-referencing schema reached through
those paths would loop or stack-overflow today.

`allOf` / `oneOf` / `anyOf` resolution is lazy and merge-style.
`getFattnedPolymorphicSchema` ([schema.go:1302-1313](../internal/anysdk/schema.go#L1302-L1313))
calls `getFatSchema(srs)` for whichever of the three is non-empty,
preferring `AllOf` > `OneOf` > `AnyOf`. `getFatSchema`
([schema.go:1065-1109](../internal/anysdk/schema.go#L1065-L1109)) merges
properties into a single synthetic schema; on key collisions it disambiguates
with `<schemaName>_<propertyKey>`. There is also a separate "implicit
union" mode behind `isObjectSchemaImplicitlyUnioned`
([schema.go:408-413](../internal/anysdk/schema.go#L408-L413),
[schema.go:441-455](../internal/anysdk/schema.go#L441-L455)) — an opt-in
hack for Azure autorest documents documented as "horrendous" in the
code comments. Discriminator-aware variant selection is not implemented.

### 1.3 Current `DESCRIBE` path

End-to-end trace, from SQL down to columns:

1. Parser produces a `*sqlparser.DescribeTable` AST
   ([stackql-parser/go/vt/sqlparser/ast.go:329-333](https://github.com/stackql/stackql-parser/blob/main/go/vt/sqlparser/ast.go#L329-L333))
   with `Full`, `Extended`, `Table` fields. Grammar at
   [sql.y:2216](https://github.com/stackql/stackql-parser/blob/main/go/vt/sqlparser/sql.y#L2216).
2. stackql core: [planbuilder/plan_builder.go:115](https://github.com/stackql/stackql/blob/main/internal/stackql/planbuilder/plan_builder.go#L115)
   routes the AST node to `handleDescribe`
   ([plan_builder.go:337-385](https://github.com/stackql/stackql/blob/main/internal/stackql/planbuilder/plan_builder.go#L337-L385)).
3. `handleDescribe` calls `primitivebuilder.NewDescribeTableInstructionExecutor`
   ([primitivebuilder/shortcuts.go:433-463](https://github.com/stackql/stackql/blob/main/internal/stackql/primitivebuilder/shortcuts.go#L433-L463)).
   This is the executor. It does three things:
   - `schema, err := tbl.GetSelectableObjectSchema()` — pulls the
     post-`SelectSchemaAndObjectPath` schema. This is the "GET response
     unwrapped to selectable rows" view.
   - `descriptionMap := schema.ToDescriptionMap(extended)` — flattens.
   - Output rows = `name`, `type`, plus `description` when EXTENDED, per
     `formulation.GetDescribeHeader` (mirrors
     [any-sdk/internal/anysdk/metadata.go:7-22](../internal/anysdk/metadata.go#L7-L22)).
4. `tbl.GetSelectableObjectSchema()` resolves through
   `standardHeirarchyObjects.GetSelectableObjectSchema`
   ([stackql/tablemetadata/hierarchy_objects.go:208-223](https://github.com/stackql/stackql/blob/main/internal/stackql/tablemetadata/hierarchy_objects.go#L208-L223))
   into `OperationStore.GetSelectSchemaAndObjectPath`
   ([any-sdk/internal/anysdk/operation_store.go:1615-1624](../internal/anysdk/operation_store.go#L1615-L1624)).
5. That descends into `schema.getSelectItemsSchema(itemsKey, mediaType)`
   ([schema.go:823-883](../internal/anysdk/schema.go#L823-L883)), which
   handles JSON-path / XPath sub-selectors. Without an explicit path it
   returns either the items schema (for arrays) or the schema itself.

The "stops at object/array boundaries" behaviour lives in
`ToDescriptionMap` at [schema.go:1262-1300](../internal/anysdk/schema.go#L1262-L1300):

- For type `object` it iterates `Properties` once and calls
  `toFlatDescriptionMap(extended)` on each child
  ([schema.go:955-963](../internal/anysdk/schema.go#L955-L963)), which
  reports only `name`, `type`, and optionally `description`.
- For type `array` it recurses into items by re-calling
  `ToDescriptionMap`, but the items terminate the same way: one more
  property-level pass and stop.
- For polymorphic schemas it materialises the fat schema and does one
  property pass.

The limitation is **structural**: `toFlatDescriptionMap` deliberately
records only a leaf-style summary of each child, so what `DESCRIBE`
returns is always exactly two levels deep from the chosen anchor (the
anchor itself + its direct properties / items). There is no depth
parameter and no recursion below that. So the prompt's "stop at object
and array boundaries" framing is accurate.

### 1.4 Response selection

Selection of which response to introspect is split between load time
and call time:

- **Load time**, in `resolveExpectedResponse`
  ([loader.go:981-1040](../internal/anysdk/loader.go#L981-L1040)): if the
  provider's YAML supplies `openAPIDocKey` + `bodyMediaType`, that exact
  response is pinned. Otherwise `findBestResponseDefault`
  ([loader.go:815-837](../internal/anysdk/loader.go#L815-L837)) picks the
  numerically lowest sub-300 status code (so `200` wins over `201`
  wins over `204`), falling back to the `default` response key.
- The media type fallback inside `resolveContentDefault`
  ([loader.go:801-813](../internal/anysdk/loader.go#L801-L813)) prefers
  `application/json`, then `application/xml`, then
  `application/octet-stream`. Anything else is silently ignored.
- **Call time**, in `GetResponseBodySchemaAndMediaType` /
  `GetFinalResponseBodySchemaAndMediaType` /
  `GetSelectSchemaAndObjectPath`
  ([operation_store.go:1575-1638](../internal/anysdk/operation_store.go#L1575-L1638)):
  the chain prefers (in order) `AsyncOverrideSchema` >
  `OverrideSchema` > the resolved `Schema`. `Final*` variants give
  preference to async overrides, regular variants do not.

There is no "prefer GET" anywhere in `any-sdk`. What looks like GET-preference
is actually SQL-verb routing: when stackql encounters `SELECT`, the
hierarchy lookup eventually calls `GetFirstMethodFromSQLVerb("select")`
([resource.go:315-317, 349-354](../internal/anysdk/resource.go#L315-L354)),
which consults the `sqlVerbs` map. The default ordering for an unannotated
resource is `select`, `list`, `aggregatedList`, `get`
([resource.go:357-368](../internal/anysdk/resource.go#L357-L368)).
So `list` actually wins over `get` by default, not the other way round.
The prompt's "implicit GET 200" framing is mostly right (200-or-lowest-2xx)
but the method preference is verb-driven and slightly counter-intuitive.

### 1.5 Existing recursion

`any-sdk` itself does not have a depth-unbounded schema walker. The
in-package traversals are all shallow or path-driven:

- `getDescendent` / `getDescendentInit`
  ([schema.go:559-571, 670-689](../internal/anysdk/schema.go#L559-L689))
  walks one path segment at a time but takes the path from a caller,
  not enumerating all subtrees.
- `FindByPath` ([schema.go:1315-1365](../internal/anysdk/schema.go#L1315-L1365))
  does walk the tree breadth-style looking for a single key match. It
  threads a `visited map[string]bool` keyed on `$ref`, so it is the only
  cycle-aware traversal in the package. It also has a TODO at line 1344
  acknowledging that endless recursion is not fully prevented.
- `GetAllColumns` ([schema.go:965-991](../internal/anysdk/schema.go#L965-L991))
  enumerates immediate properties (one level) and, for arrays, recurses
  once into items. Not a tree walk.
- `Tabulate` ([schema.go:1209-1260](../internal/anysdk/schema.go#L1209-L1260))
  produces columns from properties at depth 1; for arrays it recurses
  into items. Same shape as `ToDescriptionMap`.
- `unmarshalReaderResponseAtPath` ([schema.go:1467-1505](../internal/anysdk/schema.go#L1467-L1505))
  walks to a JSON/XML subtree at parse time, again driven by an input path.

The only place in the wider codebase with real depth-unbounded recursion
over schemas is **outside** `any-sdk`, in stackql core:
[stackql/internal/stackql/metadatavisitors/requestvisitors.go:359-457](https://github.com/stackql/stackql/blob/main/internal/stackql/metadatavisitors/requestvisitors.go#L359-L457)
(`retrieveTemplateVal`). That function recurses through object
properties and array items to produce the body template for
`SHOW INSERT INTO`. It does cycle-detect, but using `schema.GetTitle()`
as the visited key, which is unreliable (titles are optional and not
unique). It also short-circuits on arrays of seen titles by emitting a
template placeholder, which is the right idea but not transferable to
DESCRIBE output. This is the natural model to copy for the new
resolver — but with proper `$ref`-keyed cycle detection.

### 1.6 Views

Views live in [internal/anysdk/view.go](../internal/anysdk/view.go) and
are pure provider-authored DDL. The `standardViewContainer` type
([view.go:36-41](../internal/anysdk/view.go#L36-L41)) holds:

- `DDL` — the literal SQL string used to materialise the view.
- `Predicate` — a guard expression like
  `sqlDialect == "postgres" && requiredParams == ["projectId"]`,
  parsed by `sqlDialectRegex` and `requiredParamRegex` at
  [view.go:11-16](../internal/anysdk/view.go#L11-L16).
- `Fallback` — a chain for predicate failover.

Views are extracted in `stackQLConfig` per resource and looked up via
`Resource.GetViewsForSqlDialect(dialect)`
([resource.go:189-194](../internal/anysdk/resource.go#L189-L194)). They
are not derived. There is no machinery in the loader to auto-flatten a
nested response into a view DDL. Each view's DDL is hand-written by the
provider author and lives in YAML alongside the resource.

So views are a workaround for the introspection gap, not a partial
solution to it. Once a proper nested resolver exists, autogenerating a
view DDL ("flatten the GET response one level into dotted columns")
becomes mechanical: walk the tree, produce a SELECT list of
`response.<path> AS <alias>`. Worth doing, but as a follow-up, not as
part of the introspection primitive itself.

### 1.7 Request shape access

The request shape is already accessible. `OperationStore.GetRequestBodySchema()`
([operation_store.go:1545-1554](../internal/anysdk/operation_store.go#L1545-L1554))
returns the resolved request schema; `Request.GetRequired()`
([expectedRequest.go:95-97](../internal/anysdk/expectedRequest.go#L95-L97))
returns required property names. Override schemas are honoured
identically to responses.

The prompt asserts that `SHOW INSERT INTO` "was reportedly working at
some point" and asks what happened to it. **It still works.** The
grammar production is live at
[stackql-parser/go/vt/sqlparser/sql.y:1980-1985](https://github.com/stackql/stackql-parser/blob/main/go/vt/sqlparser/sql.y#L1980-L1985),
the handler at
[stackql/internal/stackql/planbuilder/plan_builder.go:1277-1285](https://github.com/stackql/stackql/blob/main/internal/stackql/planbuilder/plan_builder.go#L1277-L1285)
and
[stackql/internal/stackql/primitivebuilder/shortcuts.go:130-179](https://github.com/stackql/stackql/blob/main/internal/stackql/primitivebuilder/shortcuts.go#L130-L179),
and there are passing integration tests:
[stackql/internal/stackql/driver/show_insert_integration_test.go](https://github.com/stackql/stackql/blob/main/internal/stackql/driver/show_insert_integration_test.go).
The implementation depth is the issue, not its existence: the body
template comes from `ToInsertStatement` ->
`SchemaRequestTemplateVisitor.RetrieveTemplate` ->
`retrieveTemplateVal`, which produces a JSON template, not a column-shaped
DESCRIBE. Calling that "introspection" is a stretch — it shows the
*placeholders an INSERT needs*, not the *schema structure*. The two
shapes are different artefacts even when they enumerate the same fields.

### 1.8 API boundary (what stackql core actually calls)

The exported surface stackql core depends on is mirrored in
[public/formulation/interfaces.go](../public/formulation/interfaces.go).
This file is marked "generated mechanically from wrappers.go" — so
adding a method to an `anysdk` interface that stackql needs to see also
requires touching `wrappers.go` and re-generating `interfaces.go`.

Methods relevant to schema metadata that are already in the public
surface:

| any-sdk method | Used in stackql for |
| --- | --- |
| `OperationStore.GetRequestBodySchema` | request body inspection |
| `OperationStore.GetResponseBodySchemaAndMediaType` | response body inspection |
| `OperationStore.GetSelectSchemaAndObjectPath` | DESCRIBE's anchor schema |
| `Schema.GetProperties` / `GetProperty` / `GetPropertySchema` | tabulation, request templating |
| `Schema.GetItemsSchema` / `GetAdditionalProperties` | as above |
| `Schema.ToDescriptionMap(extended bool)` | the actual DESCRIBE producer |
| `Schema.Tabulate(omitColumns bool, defaultCol string)` | SELECT projection planning |
| `Schema.GetType` / `IsRequired` / `IsReadOnly` | field metadata |
| `Schema.GetName` / `GetTitle` / `GetSelectionName` | naming |

That is the surface that needs a sibling-tree style addition.
`ToDescriptionMap` is what DESCRIBE today binds to. The new resolver
should not replace it; it should sit beside it so the default DESCRIBE
behaviour stays a one-line call.

## 2. Gaps

Mapped against the target capability set, with the change kind labelled
in parentheses:

1. **Depth-unbounded recursion.** `ToDescriptionMap` stops at depth 1.
   None of the in-package walkers produce a full tree. (Missing
   function in any-sdk: a new resolver.)
2. **Cycle-safe traversal.** Only `FindByPath` carries a visited map,
   and only over `$ref` strings. A new walker needs its own visited
   map keyed on `*openapi3.Schema` identity *and* `$ref` string (the
   former protects inline cycles, the latter named cycles). (Missing
   function; no type change required if the visited map is internal.)
3. **AnyOf/OneOf exposure.** Today `getFattnedPolymorphicSchema` merges
   them into a single fat schema with synthetic name disambiguation.
   That's lossy: a caller can't tell whether a property came from a
   particular variant. (New fields on the output node:
   `OneOf`/`AnyOf`/`AllOf` arrays of sub-trees; existing internal
   fatten path stays for column tabulation.)
4. **Symmetric request introspection.** Request schemas are reachable
   (`GetRequestBodySchema`) but not exposed through a parallel
   DESCRIBE-like path. (Grammar change + new resolver entry point;
   no new type unless we want to thin-wrap.)
5. **Per-method / per-status selection.** Today selection is implicit
   ("the selectable schema" for SELECT verb; lowest 2xx). There is no
   way to ask for a specific operation by name or a specific status
   code. (Grammar change + parameters on the new resolver. The
   underlying `OperationStore` lookup already supports this — the
   resource holds the full methods map at
   [resource.go:157-159](../internal/anysdk/resource.go#L157-L159).)
6. **Path / subtree selection.** `getSelectItemsSchema` already
   supports JSON-path subselection at
   [schema.go:823-883](../internal/anysdk/schema.go#L823-L883). The
   resolver can reuse it. (Grammar change to expose the parameter;
   resolver work to pipe it through.)
7. **Two output shapes (tree, flat).** `ToDescriptionMap` is a hybrid
   today: it's a map keyed by property name, value is a flat map of
   `{name, type, description}` — neither a nested tree nor a useful
   one-row-per-leaf flat. Both need building. (New function; the
   "flat" form needs a new column set; the "tree" form needs a JSON
   marshaller.)
8. **Vendor-extension awareness.** `WriteOnly` and `Deprecated` are
   on the underlying `*openapi3.Schema` but not surfaced on the
   `Schema` interface. There is no sensitive/computed/output-only
   marker beyond the openapi3 booleans. (Interface additions:
   `IsWriteOnly`, `IsDeprecated`. Optional new extension keys:
   `x-stackQL-sensitive`, `x-stackQL-computed`. The flag is loaded
   for free via openapi3's `Extensions` map.)
9. **Stable column output for SQL.** The current DESCRIBE shape
   (`name`, `type`, optional `description`) is fine as a default but
   does not expose `required`, `default`, `enum`, etc. (New column
   set for the FLAT form; tree form lives in a JSON column.)
10. **Public surface plumbing.** Each new exported method on an
    `anysdk` type requires a matching method on the `formulation`
    wrapper plus the regenerated interface line. (Mechanical;
    formulation/wrappers.go is the source of truth, interfaces.go is
    generated.)
11. **Cycle markers in output.** When a cycle is short-circuited the
    walker should emit a sentinel node that the renderer can show.
    Today nothing emits such a sentinel because nothing detects
    cycles. (New field on the output node: `CycleRef`.)

## 3. Proposed design

### 3.1 SQL grammar

The prompt's strawman has too many independent statements. Most of
the variation collapses into options on `DESCRIBE`, with `SHOW` reserved
for what it does today (enumerate things). Recommended grammar:

```sql
DESCRIBE <table>                                     -- unchanged
DESCRIBE EXTENDED <table>                            -- unchanged (adds description column)
DESCRIBE [EXTENDED] <table>
    [ METHOD <method-name> ]                         -- pick an operation, not a verb
    [ REQUEST | RESPONSE ]                           -- which side; default RESPONSE
    [ STATUS <code> ]                                -- e.g. 200, 201, default
    [ AT '<jsonpath>' ]                              -- subtree anchor
    [ DEPTH <n> ]                                    -- 0 = current behaviour, default; -1 = unlimited
    [ AS TREE | AS FLAT ]                            -- default FLAT
```

Rationale and tradeoffs:

- **Default `DESCRIBE <table>` does not change.** Backward compatibility
  matters; existing snapshot tests and user scripts assume the current
  two- or three-column output. `DEPTH 0` and `AS FLAT` are the
  defaults, so the unchanged form is `DEPTH 0, AS FLAT, RESPONSE,
  default status, no path, default method`.
- **`METHOD <method-name>` not `FOR <verb>`.** The verb lookup
  (`getFirstMethodFromSQLVerb`) is already what `DESCRIBE` uses
  implicitly. Operators want to be able to name a specific method —
  e.g. `get` vs `list` vs `aggregatedList` on the same resource. Names
  are unambiguous within a resource; verbs are not.
- **`REQUEST` / `RESPONSE` instead of two grammars.** A single
  grammar with a side-selector keeps the parser change small and
  lets every option apply symmetrically.
- **`STATUS` as a numeric literal or `default`.** Today the loader
  pins one response. The resolver bypasses the pinned choice when
  STATUS is given.
- **`DEPTH 0` means "today's behaviour"** — i.e. one level of
  properties from the anchor. `DEPTH -1` means unlimited (cycle-bounded).
  Positive integers mean exactly N additional levels.
- **`AS TREE` returns one JSON row.** `AS FLAT` returns one row per
  leaf. The default is FLAT because that is what existing tooling
  and `psql`-style clients render best.

`SHOW INSERT INTO <table>` stays as-is. It is not an alias of the new
grammar — it produces a jsonnet/INSERT template, not a column listing.
Co-existence is fine. The prompt's "SHOW UPDATE" and "SHOW EXEC"
proposals are not needed: those become
`DESCRIBE EXTENDED <table> METHOD update REQUEST` and
`DESCRIBE EXTENDED <table> METHOD <exec-method> REQUEST AS FLAT`
respectively. One grammar to maintain.

Optionally, accept `DESCRIBE <table>.<method>` as syntactic sugar for
`DESCRIBE <table> METHOD <method>` — it is a smaller change for users
and removes the need for keyword shuffling. Not strictly necessary.

### 3.2 Internal API in any-sdk

The output type is what most of the work is. Proposed:

```go
// In internal/anysdk/introspection.go (new file).

type IntrospectionOpts struct {
    Method     string  // method name; "" = SQL-verb default for the resource
    Side       Side    // SideRequest | SideResponse
    StatusCode string  // "" = best-2xx default; ignored for SideRequest
    At         string  // JSON-path subtree anchor; "" = root
    MaxDepth   int     // 0 = legacy one-level; -1 = unlimited; N = N additional levels
    UnwrapPoly Poly    // PolyMerge (today's fatten) | PolySplit (emit OneOf/AnyOf nodes)
}

type Side int
const (
    SideResponse Side = iota
    SideRequest
)

type Poly int
const (
    PolyMerge Poly = iota
    PolySplit
)

type MethodIntrospection struct {
    Method        string   // canonical method name (e.g. "list")
    SQLVerb       string   // "select", "insert", etc.
    HTTPVerb      string   // "GET", "POST", ...
    StatusCode    string   // for response side; "" for request
    MediaType     string   // resolved media type
    Mutates       bool
    Awaitable     bool
    Root          *SchemaNode
}

type SchemaNode struct {
    Path        string                  // dotted path from anchor; "" for root
    Name        string                  // local key
    Type        string                  // object, array, string, integer, ...
    Format      string                  // openapi format
    Title       string
    Description string
    Required    bool                    // required *within its parent*
    Default     any
    Enum        []any
    Example     any
    ReadOnly    bool
    WriteOnly   bool
    Deprecated  bool
    Sensitive   bool                    // x-stackQL-sensitive (new extension)
    Computed    bool                    // x-stackQL-computed   (new extension)
    Properties  []*SchemaNode           // ordered for stable output
    Items       *SchemaNode             // when Type == "array"
    Additional  *SchemaNode             // additionalProperties
    OneOf       []*SchemaNode           // populated when UnwrapPoly == PolySplit
    AnyOf       []*SchemaNode
    AllOf       []*SchemaNode
    RefOrigin   string                  // original $ref, for traceability
    CycleRef    string                  // set when this node is the cycle-break point
    Truncated   bool                    // set when depth limit cut here
}
```

The entry point:

```go
// On the Resource interface (so stackql can find it after resolving the
// table). Method lookup stays inside the resource.
type Resource interface {
    // ...existing methods...
    Introspect(opts IntrospectionOpts) (MethodIntrospection, error)
}
```

Rationale:

- `Properties` is `[]*SchemaNode` not `map[string]*SchemaNode`. Maps
  give unstable iteration; ordered slices let the FLAT renderer
  produce deterministic output without re-sorting in the caller.
  The slice carries `Name` per element.
- `Required` is per-node-in-parent. The boolean encodes presence in
  the parent's `required` list. The root is `Required: false` by
  convention; it is the schema being described.
- `Default` / `Enum` / `Example` are `any` (kin-openapi already stores
  them that way). The renderer stringifies for SQL.
- `OneOf` / `AnyOf` / `AllOf` populate only in `PolySplit` mode. In
  `PolyMerge` mode (the default) the merged properties land in
  `Properties` as today — matches existing `Tabulate` behaviour, so a
  caller wanting "what columns does this have" still gets the union.
- `CycleRef` is the original `$ref`. The renderer emits a row with
  type=`<cycle>` and the ref string in the description.
- `Truncated` is separate from `CycleRef`. Truncation is a user-imposed
  depth cap; cycle is a structural protection.

The actual resolver lives in `internal/anysdk/introspection.go`:

```go
func resolveIntrospection(rsc *standardResource, opts IntrospectionOpts) (MethodIntrospection, error)
func walkSchema(s *standardSchema, parentPath, name string, required bool, depth int, opts IntrospectionOpts, visited *visitMap) *SchemaNode
```

Cycle detection: `visitMap` carries both `map[*openapi3.Schema]string`
(identity, for inline cycles) and `map[string]struct{}` (`$ref` strings,
for named cycles). When `walkSchema` hits a schema already in the
visited map, it returns a `SchemaNode{CycleRef: ref, Type: "object"}`
without descending. Visited entries pop when the recursion unwinds —
i.e. the visited map tracks *the path from root*, not "anywhere ever".
A schema that appears at two unrelated subtrees of the same response
is not a cycle and should not be elided.

Reuse of existing code:

- `s.getProperties()` ([schema.go:415-436](../internal/anysdk/schema.go#L415-L436))
  for object properties. Returns the same union semantics today's code
  expects, including the implicit-union hack for Azure.
- `s.GetItems()` ([schema.go:691-706](../internal/anysdk/schema.go#L691-L706))
  for array items.
- `s.GetAdditionalProperties()` ([schema.go:345-350](../internal/anysdk/schema.go#L345-L350))
  for `additionalProperties`.
- `s.getFattnedPolymorphicSchema()` in PolyMerge mode.
- `op.GetSelectSchemaAndObjectPath()` / `GetRequestBodySchema()` to
  obtain the anchor.

### 3.3 Flat rendering

Columns, in this order:

```
path             text       -- dotted path from anchor; empty for root row
name             text       -- terminal segment of path
type             text       -- openapi type, plus "[cycle]"/"[truncated]" markers
format           text       -- openapi format; null if absent
required         boolean
default_value    text       -- json-stringified value; null if absent
enum_values      text       -- comma-joined; null if absent
description      text       -- only included when EXTENDED
read_only        boolean
write_only       boolean
deprecated       boolean
sensitive        boolean    -- from x-stackQL-sensitive
ref_origin       text       -- $ref string of the schema variant, for traceability
```

Path encoding:

- `.` separates object property descents.
- `[]` indicates "items of an array"; the items appear under the same
  path as the array itself, with `[]` suffix.
- `[*]` indicates `additionalProperties` (synthetic key).
- `~oneof:<name>~` and `~anyof:<name>~` segments appear only in
  `PolySplit` mode.

Without EXTENDED, the column list drops `description` and `ref_origin`
to keep the today's-shape muscle memory. Non-extended is the default.

`name` is redundant with the trailing segment of `path` but is included
because it is genuinely what most callers grep for and it keeps SQL
queries readable. Both columns; no clever join logic.

### 3.4 Tree rendering

One row, one column:

```
schema           json
```

The JSON shape is exactly `SchemaNode` minus internal fields. This is
intended for MCP and other JSON-native consumers; humans should prefer
FLAT.

### 3.5 Cycle handling

Contract:

- A cycle is detected when walking encounters a schema already on the
  current ancestor path (identity match on `*openapi3.Schema`, or
  `$ref` string match if the ref is non-empty).
- The walker emits a single `SchemaNode` for the cycle point with
  `CycleRef` set to the `$ref` of the originally referenced schema.
- The cycle node has `Type` propagated (object/array) but no children.
- The FLAT renderer shows `type` as `<cycle:#/components/schemas/Foo>`
  or similar; the TREE renderer keeps `CycleRef` as a first-class
  field.
- Recursion limited to a hard ceiling (e.g. 64 levels) even with
  `DEPTH -1`, as a defence against pathological non-cyclic-but-deep
  schemas like GCP IAM. Exceeding the ceiling sets `Truncated: true`.

### 3.6 View interaction

Recommendation: **ignore views**. The new primitive describes the
underlying method schema, not the rewritten view. Two reasons:

1. Views are SELECT projections; they don't apply to request bodies
   or to non-SELECT methods. A grammar that sometimes consults views
   and sometimes doesn't is harder to explain.
2. Views become uninteresting once introspection works. The goal is
   to let agents discover the structure they already would have
   built a view for. Routing them through the view defeats that.

If the existing view-based `DESCRIBE` of a view (handled at
[plan_builder.go:358-376](https://github.com/stackql/stackql/blob/main/internal/stackql/planbuilder/plan_builder.go#L358-L376))
should be preserved, that's fine — it is detected via
`md.GetHeirarchyObjects().GetHeirarchyIDs().GetView()`, which is
orthogonal to the new resolver. So: views still describe-as-they-do,
tables describe via the new resolver. The new clauses
(`METHOD`/`REQUEST`/etc.) only make sense on tables; reject them at
parse time when the target is a view.

## 4. Implementation plan

Steps are sequenced for short PRs. Each step is independently
mergeable and adds either no new public surface or one small piece of
it.

### 4.1 any-sdk: schema interface gaps

Files: [internal/anysdk/schema.go](../internal/anysdk/schema.go),
[public/formulation/wrappers.go](../public/formulation/wrappers.go),
[public/formulation/interfaces.go](../public/formulation/interfaces.go).

Add to the `Schema` interface and `standardSchema`:

```go
IsWriteOnly() bool
IsDeprecated() bool
IsSensitive() bool          // reads x-stackQL-sensitive extension
GetDefault() any
GetEnum() []any
GetFormat() string
GetExample() any
GetXStackQLExtensions() map[string]any
```

Add to the `ExtensionKey*` block in
[internal/anysdk/const.go](../internal/anysdk/const.go#L28-L36):
`ExtensionKeySensitive = "x-stackQL-sensitive"`,
`ExtensionKeyComputed = "x-stackQL-computed"`.

Regenerate `public/formulation/interfaces.go` (it's marked
"Code generated mechanically from wrappers.go"). The header comment
in interfaces.go does not name a tool; the generation is in-tree
boilerplate so the regenerate is done by hand in `wrappers.go` first,
then mirrored into `interfaces.go`.

No behaviour change. This step is a pre-req that unblocks the
resolver from having to type-assert down to `*standardSchema`.

### 4.2 any-sdk: introspection resolver

Files: new `internal/anysdk/introspection.go`,
new `internal/anysdk/introspection_test.go`.

Implement:

- `IntrospectionOpts`, `Side`, `Poly`, `MethodIntrospection`,
  `SchemaNode` (as in §3.2).
- `Resource.Introspect(opts) (MethodIntrospection, error)`. Add to
  the `Resource` interface in
  [internal/anysdk/resource.go](../internal/anysdk/resource.go#L18-L52).
  Implementation: resolve `OperationStore` via `FindMethod` when
  `opts.Method` is set, otherwise `GetFirstMethodFromSQLVerb` with
  the relevant verb implied by `opts.Side` (request -> insert/update,
  response -> select).
- Status-code override: when `opts.StatusCode != ""` and
  `opts.Side == SideResponse`, look up the response directly from
  the OpenAPI operation's `Responses` map instead of using the
  loader-pinned schema. This requires reaching into the operation's
  `openapi3.Operation` — already accessible via
  `OperationStore.GetOperationRef().Value.Responses`.
- `walkSchema` recursion as in §3.5, threading `*visitMap`.
- Path anchoring via `opts.At`: reuse `schema.getSelectItemsSchema`
  ([schema.go:823-883](../internal/anysdk/schema.go#L823-L883)) to
  resolve the anchor, then walk from there.

Tests: table-driven against fixture schemas in
[internal/anysdk/testdata](../internal/anysdk/testdata), covering:
- Simple object.
- Object inside `data` envelope.
- Array of nested objects.
- AllOf with property override.
- OneOf in both PolyMerge and PolySplit modes.
- Self-referencing schema via `$ref` (cycle marker present).
- Depth caps.

### 4.3 any-sdk: flat-rendering helper

Files: extend `internal/anysdk/introspection.go`.

Two functions:

```go
func (mi MethodIntrospection) Flatten(extended bool) []map[string]any
func (mi MethodIntrospection) TreeJSON() ([]byte, error)
```

`Flatten` produces one map per `SchemaNode`, keyed for stackql's
`util.PrepareResultSet` rowmap convention. Sort order is the natural
DFS order of the walk (children after parent, properties in slice
order). The renderer enriches `type` with `[cycle:<ref>]` and
`[truncated]` markers as appropriate.

`TreeJSON` marshals `SchemaNode` minus internal-only fields.

Tests: golden-file comparisons against committed fixtures.

### 4.4 any-sdk: public surface

Files: [public/formulation/wrappers.go](../public/formulation/wrappers.go),
[public/formulation/interfaces.go](../public/formulation/interfaces.go).

Add wrappers for:

- `Resource.Introspect`
- `MethodIntrospection` (interface mirror)
- `SchemaNode` (struct re-exported — the renderer needs to read it)

Re-generate `interfaces.go`.

### 4.5 stackql-parser: grammar change

Files:
[stackql-parser/go/vt/sqlparser/sql.y](https://github.com/stackql/stackql-parser/blob/main/go/vt/sqlparser/sql.y),
[stackql-parser/go/vt/sqlparser/ast.go](https://github.com/stackql/stackql-parser/blob/main/go/vt/sqlparser/ast.go).

Extend the `DescribeTable` production to accept the optional clauses
in §3.1. Extend the `DescribeTable` AST struct with corresponding
fields:

```go
DescribeTable struct {
    Full       string
    Extended   string
    Table      TableName
    Method     string  // empty = default
    Side       string  // "REQUEST" | "RESPONSE" | ""
    StatusCode string  // empty = default
    AtPath     string  // empty = root
    Depth      string  // "0" by default; "-1" for unlimited
    OutputForm string  // "TREE" | "FLAT" | ""
}
```

The new clauses are all optional and trailing, so the existing
production (`DESCRIBE [EXTENDED] <table>`) keeps matching with empty
fields. No breakage.

Regenerate `sql.go` from `sql.y` using the existing build target.

### 4.6 stackql core: wire the new resolver

Files:
[stackql/internal/stackql/planbuilder/plan_builder.go](https://github.com/stackql/stackql/blob/main/internal/stackql/planbuilder/plan_builder.go),
[stackql/internal/stackql/primitivebuilder/shortcuts.go](https://github.com/stackql/stackql/blob/main/internal/stackql/primitivebuilder/shortcuts.go),
[stackql/internal/stackql/tablemetadata/hierarchy_objects.go](https://github.com/stackql/stackql/blob/main/internal/stackql/tablemetadata/hierarchy_objects.go).

In `handleDescribe` ([plan_builder.go:337-385](https://github.com/stackql/stackql/blob/main/internal/stackql/planbuilder/plan_builder.go#L337-L385)):

- Detect whether any of the new clauses is set on the AST.
- If not: keep the current path (`NewDescribeTableInstructionExecutor`).
- If so: build `IntrospectionOpts` from the AST and call a new
  `NewDescribeIntrospectExecutor(handlerCtx, md, opts)`.

Implement the new executor in `shortcuts.go`. It:

1. Pulls the resource via `md.GetHeirarchyObjects().GetResource()`.
2. Calls `resource.Introspect(opts)`.
3. Renders to FLAT or TREE per `opts`.
4. Wraps with `util.PrepareResultSet` like the existing executor.

Column header sets live in `formulation.GetDescribeHeader` (extend) or a
new helper `GetIntrospectionHeader(form, extended)`.

### 4.7 Restore-as-aliases evaluation

The prompt asks for `SHOW INSERT INTO`, `SHOW UPDATE`, `SHOW EXEC` work.
`SHOW INSERT INTO` exists already (§1.7). I am not proposing
`SHOW UPDATE` / `SHOW EXEC` — those should fall out of the new
DESCRIBE grammar:

- `SHOW INSERT INTO foo` -> stays as today (jsonnet template).
- "What columns are in the update body?" ->
  `DESCRIBE EXTENDED foo METHOD update REQUEST AS FLAT`.
- "What's the exec payload look like?" ->
  `DESCRIBE EXTENDED foo METHOD <exec-method> REQUEST AS FLAT`.

If product wants the `SHOW`-shaped aliases, the cost is small: each
becomes a thin `case` branch in `NewShowInstructionExecutor` that
calls the new resolver and renders FLAT. Defer until there is user
demand.

### 4.8 Backward compatibility check

`DESCRIBE foo` with no new clauses must produce identical rows to
today. Verification:

- `default DEPTH = 0` makes the resolver descend one level (matches
  `ToDescriptionMap`'s shape).
- `default Side = RESPONSE`, `default Method = ""` (SQL-verb default)
  matches `GetSelectableObjectSchema`.
- `default OutputForm = FLAT` and default columns set to
  `{name, type}` (+ `description` when EXTENDED) matches
  `GetDescribeHeader`.

Make this an explicit test: re-run the existing `DESCRIBE` integration
tests under stackql/test against the new code path, gated on a
feature flag, and assert byte-equality of output. The feature flag
flips to "always new path" once equality is proven.

### 4.9 Provider golden tests

Pick three providers with deeply nested schemas:

- **Databricks** — heavily polymorphic, `oneOf` everywhere
  (covers PolySplit and PolyMerge).
- **GCP compute** — deep nesting through `Operation` and `Instance`
  schemas; well-known cycle through `Operation.error.errors[]`.
- **AWS CloudFormation** — extreme depth via `Resources` discriminated
  by `Type`; covers worst-case `additionalProperties` patterns.

Test layout:

```
test/golden/introspection/
    databricks_workspaces_clusters_get_response_flat.golden
    databricks_workspaces_clusters_get_response_tree.golden
    gcp_compute_instances_get_response_flat.golden
    gcp_compute_instances_insert_request_flat.golden
    aws_cfn_stacks_response_flat.golden
```

The golden files are committed; the test loads the provider, runs
`Introspect` with the documented opts, formats the output exactly as
the SQL renderer would, and diffs. Update via a `-update` flag.

### 4.10 MCP exposure

Files: stackql `internal/stackql/mcpbackend/mcp_service_stackql.go` and
`mcp_reverse_proxy_backend_service.go`.

Today the MCP layer has `DescribeTable(ctx, hI)` at
[mcp_service_stackql.go:505](https://github.com/stackql/stackql/blob/main/internal/stackql/mcpbackend/mcp_service_stackql.go#L505)
which builds a SQL string from `GetDescribeTable` and executes it. The
MCP tool surface stays the same — it just now passes the new clauses
through (or, simplest, exposes a single tool that always emits
`DESCRIBE EXTENDED ... AS TREE` and lets the MCP-side renderer reshape
to whatever the agent wants).

Add two MCP tools, both thin wrappers over `DESCRIBE`:

- `describe_table_flat(table, method?, side?, status?, at?, depth?)` ->
  emits FLAT rows.
- `describe_table_tree(table, method?, ...)` -> emits one JSON blob.

Both should construct SQL and execute through the existing interrogator
path. There should be no schema knowledge in the MCP layer itself —
it remains a presentation shim over the SQL primitive.

## Open questions for review

1. **`DESCRIBE` default behaviour.** I have kept it byte-identical to
   today. An alternative is to widen the default to `DEPTH 1` (so
   one envelope unwrap happens automatically) and accept the snapshot
   churn. Cleaner, more useful, breaks existing tests. Worth discussing.
2. **Cycle ceiling.** I proposed a hard 64-depth ceiling even at
   `DEPTH -1`. Should that be configurable?
3. **PolyMerge vs PolySplit default.** I defaulted to PolyMerge to
   match `Tabulate`. For DESCRIBE specifically PolySplit might be more
   informative. Recommend: PolyMerge for FLAT (since FLAT renders one
   column at a time), PolySplit for TREE.
4. **Sensitivity inference.** Beyond `x-stackQL-sensitive`, should
   well-known field names (`password`, `secret`, `token`,
   `*_key`, `*_token`) be auto-marked? Risk of false positives.
   Defer.
5. **View autogeneration.** Once the resolver is in, generating a
   default view DDL from the FLAT output is a few hundred lines. Want
   it scoped in or deferred?
