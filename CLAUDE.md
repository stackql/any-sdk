# CLAUDE.md

Coding guidelines and conventions for working in `any-sdk`. Read these before making changes.

For domain context (what an `any-sdk` provider document is, the provider/service/resource/method hierarchy, SQL-verb mapping, validation) see @AGENTS.md. This file is about *how to write the Go code*.

## Architecture and where code goes

Three layers, with a strict separation of concerns:

- `internal/anysdk/` - the real implementation. All logic lives here. Foreign-system semantics (OData `$filter` syntax, AWS XML envelope shapes, pagination dialects, casing rules, etc.) belong here, never in a downstream consumer such as `stackql`.
- `pkg/` - shared, dependency-light libraries (e.g. `pkg/casing`, `pkg/stream_transform`). A `pkg/` package must not import `internal/anysdk` (that is an import cycle). If such a package needs a type from internal, define a minimal neutral interface in the `pkg/` package and have the caller adapt to it.
- `public/formulation/` - the PUBLIC FACADE. It only wraps `internal/anysdk`. It contains no logic.

### The facade is sacred (`public/formulation`)

This is the rule most often violated. The facade exists to wrap internal types so internal structs never leak across the public boundary.

- There are exactly THREE files: `interfaces.go`, `wrappers.go`, `formulation.go`. Do not add a fourth (not even a `*_test.go`). The logic and its tests live in `internal/anysdk`; the facade is mechanical wrapping and is covered by internal tests plus integration.
  - `interfaces.go` - public interface declarations.
  - `wrappers.go` - the `wrapped*` impls and `wrapSlice_*` / `unwrapSlice_*` helpers.
  - `formulation.go` - constructors (`New*`) and free functions (e.g. `ApplyPushdown`).
- Never introduce a public struct in the facade. Every public type is an interface.
- Never vary the wrapping pattern. Follow exactly what is already there:
  - `type wrappedX struct { inner anysdk.X }`, methods delegate to `w.inner.Method()`.
  - `func (w *wrappedX) unwrap() anysdk.X { return w.inner }` when the value is passed back into internal.
  - `wrapSlice_X(in []anysdk.X) []X` and `unwrapSlice_X(in []X) []anysdk.X`.
  - Constructors build the internal value via `anysdk.NewX(...)` and wrap it: `&wrappedX{inner: anysdk.NewX(...)}`.
- A public wrapper WRAPS the internal type. It must not re-declare the internal type's fields. Hold `inner anysdk.X` and delegate; do not copy `field-by-field` translations into the facade.
- No "creativity" in the facade - no bespoke conversion logic, no helper structs, no logic that should have lived in `internal/anysdk`. If you find yourself writing real logic in the facade, it belongs in internal.
- Mirror internal method names on the public interface (e.g. internal `QueryParams()` -> public `QueryParams()`), so wrapping is a one-line delegation.

When adding a public method that surfaces an internal one, add it to the public interface and the wrapper, mirroring an existing example (e.g. how `GetPaginationResponseTerminatorTokenSemantic` is surfaced on both wrapped operation-store types).

## Type and struct conventions

- Structures are lowercase (unexported) by default. The standard internal pattern is: exported interface + lowercase `standard*` implementation + `New*` constructor. Example: `QueryParamPushdown` (interface) / `standardQueryParamPushdown` (impl) / and a constructor or `GetTesting*` helper.
- If a type needs to be published (consumed across a package boundary), publish an INTERFACE, not a struct.
- The only acceptable exported struct is a data-transfer object for serde - i.e. a type with `json`/`yaml` tags that is unmarshalled from a provider document (e.g. `RegistryConfig`, the `standard*` config structs whose exported fields exist so the serializer can populate them). A plain value-carrier struct that is not deserialized should be an interface.
- Do not expose `internal/anysdk` structs through the public API. Wrap them.

## Additive and backward compatible

- New behavior is opt-in via an OpenAPI/`provider.yaml` flag, a method extension, or an explicit builder/argument. With the flag/config absent, behavior must be byte-for-byte unchanged.
- Do not change existing exported signatures of stable APIs. Add new methods/constructors instead.
- New extension keys go in `internal/anysdk/const.go`.
- Reuse existing constants (e.g. `ODataDialect`) rather than re-declaring literals.

## Testing

- Add a test for everything you implement. Put the test where the logic is (almost always `internal/anysdk` or `pkg/...`), never in the facade.
- Do not break existing tests.
- White-box tests (`package anysdk`) can exercise unexported helpers; external tests (`package anysdk_test`) use the exported surface. An external test can still pass a fake that structurally satisfies an unexported interface parameter.
- For config-driven types, build fixtures with the `GetTesting*` helper plus `yaml.Unmarshal` (see existing `query_param_pushdown` / `pagination` tests).

## Formatting and lint

`gofmt`/`goimports` clean and `golangci-lint run` with zero issues is a hard requirement.

- Tabs for indentation.
- `gofmt` does NOT column-align consecutive single-line function declarations. Write one-line getters one-per-line with a single space before `{` (do not pad them into a column, or gofmt will reformat and the lint check fails).
- `gofmt` DOES align struct fields and `const`/`var` blocks - keep those aligned.
- Prefer leading doc comments over trailing aligned inline comments on struct fields (trailing-comment alignment is fragile to hand-format).
- Do not orphan unexported functions; an unused unexported func trips `staticcheck` (U1000). If a refactor leaves a helper unused, route a caller through it or delete it.
- One `init()` per file.

## Footprint

- Touch as few files as practical. Make the change, not adjacent "improvements".
- Keep changes cohesive and minimal so the diff is easy to review.

## Build and verification

Before considering work done (and always before a release tag), run:

```
go build ./...
go test ./...
golangci-lint run
```

## Workflow, git, and releases

- Make changes in the current long-lived feature branch. The maintainer stages, commits, pushes, and raises the PR.
- Provide the PR title and body fenced with `~~~` (not triple backticks) so copy-paste into VS Code does not break.
- An unrelated change should be a branch off `main`, not stacked on an open PR.
- Releases use the `v0.5.3-alphaNN` tag scheme. Tag the squash-merge commit on `main` (verify `git rev-parse <tag>` == `git rev-parse origin/main`) only after the PR is merged and CI is green. The maintainer cuts/pushes tags.
