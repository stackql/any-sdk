# Changelog

## Unreleased

### Fixes from stackql robot-test feedback on the pure Go SQLite migration

Two upstream defects root-caused by the stackql robot test suite running
against `v0.5.6-alpha01-pure-go-sqlite-migration`:

- **`aws_policy_equal` fork-only behaviors restored**: the pure Go port was
  written from `github.com/stackql/sqlite-ext-functions`, but the C code
  that actually shipped (the stackql-go-sqlite3 fork's
  `SQLITE_ENABLE_STACKQL` amalgamation additions) carried two modifications
  never backported to that repo: `Tags` and `tags` are members of the
  unordered-comparison field set, and two top-level array documents (array
  vs array at the document root) compare unordered. Both are restored,
  pinned by new golden vectors (`fork_*` in
  `public/sqlfuncs/testdata/aws_policy_equal.json`), and recorded as the
  authoritative contract in `public/sqlfuncs/DIVERGENCES.md`.
- **Declared-boolean columns render as Go bool again**: mattn/go-sqlite3
  converted INTEGER values to Go `bool` when a column's declared type was
  boolean (case-insensitive decltype check, `val > 0`);
  modernc.org/sqlite performs decltype-based conversion for timestamps but
  not booleans, so such columns surfaced as 0/1. The embedded engine now
  wraps the driver so declared-boolean columns return Go `bool` for
  INTEGER values, NULL passes through unchanged, and all other columns are
  untouched.

## v0.5.5-alpha02 (draft — not yet tagged)

### Embedded SQLite backend migrated to modernc.org/sqlite (CGO-free)

The embedded SQLite engine now uses the pure Go driver
[`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) v1.59.0,
registered under the driver name `stackql-sqlite`. The cgo driver
`github.com/mattn/go-sqlite3` (via the stackql fork) is removed from the
module graph entirely.

#### Highlights

- **CGO-free builds**: `CGO_ENABLED=0` everywhere, including CI. No C
  toolchain is required to build or cross-compile any-sdk.
- **StackQL extension functions built in unconditionally**: `split_part`,
  `regexp_like`, `regexp_substr`, `regexp_replace`, `json_equal` and
  `aws_policy_equal` are now implemented in pure Go in `public/sqlfuncs`
  and registered on every embedded connection. The `sqlite_stackql` build
  tag is retired; no build tag is needed to get the functions.
- **SQL `REGEXP` operator now works**: a `regexp(pattern, source)` function
  (SQLite's hook for the `X REGEXP Y` operator) is registered alongside the
  `regexp_*` family. The retired cgo builds never provided it, so the
  operator documented in the stackql language spec previously failed with
  "no such function: regexp".
- **DSN handling centralized**: all embedded-engine DSNs pass through
  `sqlengine.BuildDSN`, which translates legacy `mattn`-style parameters
  (for example `_busy_timeout=5000`) to modernc `_pragma=name(value)`
  syntax and rejects unknown underscore parameters instead of silently
  ignoring them. The 5000 ms default busy timeout of the previous driver
  is preserved.
- **Startup pragma assertions**: after opening a database, the engine
  reads back every pragma the DSN set and fails fast on mismatch.
- **Driver error inspection wrapped**: SQLITE_BUSY / SQLITE_LOCKED and
  constraint-violation detection go through predicates in the engine
  package; no `*sqlite.Error` assertions exist outside it.

#### Behavior notes

- The regular-expression functions are now backed by Go `regexp` (RE2)
  instead of the tiny C `re.c` engine. Supported syntax is a strict
  superset for common patterns; divergences are catalogued in
  `public/sqlfuncs/DIVERGENCES.md`.
- SQLite compile-time options differ from the retired custom
  amalgamation: FTS3, UPDATE_DELETE_LIMIT and OMIT_DEPRECATED are no
  longer compiled in. modernc ships its own option set (including FTS5).
- In-memory semantics are preserved: the default DSN remains
  `file::memory:?cache=shared`; plain `:memory:` DSNs are capped to a
  single pooled connection so they keep addressing one database.

#### Rollback

The previous release, `v0.5.5-alpha01` (also tagged
`v0.5.5-alpha01-final-cgo`), is the last cgo-based release and is the
rollback point if a regression is discovered.

## v0.5.5-alpha01

Final release built on the cgo driver `github.com/mattn/go-sqlite3`
(stackql fork) with the `SQLITE_ENABLE_STACKQL` amalgamation extensions.
