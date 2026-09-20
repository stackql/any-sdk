# Changelog

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
