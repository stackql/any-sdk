# Work order: migrate any-sdk embedded SQLite engine to modernc.org/sqlite

## Objective

Replace `github.com/mattn/go-sqlite3` in the embedded SQLite engine
(`public/sqlengine/sqlite_embedded.go`) with `modernc.org/sqlite` (pure Go,
CGo-free), and implement the StackQL extension functions in pure Go as a new
`public/sqlfuncs` package registered into that engine. This makes any-sdk the
sole owner of the driver and the functions. Downstream, stackql will bump to
the resulting release and drop its go.mod replace of mattn to the cgo fork -
that is a separate follow-up PR in the stackql repo, not part of this work.

## Context

- Prior diligence lives in stackql draft PR #783
  (github.com/stackql/stackql/pull/783): a `PRAGMA compile_options` diff
  showed the cgo-only options (ENABLE_FTS3(+PARENTHESIS),
  ENABLE_UPDATE_DELETE_LIMIT, OMIT_DEPRECATED) are unused by stackql code
  paths, and golden output samples for all six extension functions were
  captured from the live cgo build. Those samples, plus the test suite in
  github.com/stackql/sqlite-ext-functions (`test/`), are the compatibility
  contract for the Go port.
- The six functions - `split_part`, `regexp_like`, `regexp_substr`,
  `regexp_replace`, `json_equal`, `aws_policy_equal` - are all deterministic
  scalars. Their C reference implementations are in
  github.com/stackql/sqlite-ext-functions (`src/`) and in the stackql cgo
  fork's amalgamation additions (`-DSQLITE_ENABLE_STACKQL`, build tag
  `sqlite_stackql`).
- Conventions for the new package are in `public/sqlfuncs/CLAUDE.md`
  (already on this branch). Treat it as authoritative.

## Hard constraints

- Pin `modernc.org/sqlite` at the current release (>= v1.57.0 is the floor
  for caller-constructed Driver registration; v1.59.0 was verified viable in
  the #783 diligence). Do not fork or vendor-patch it.
- Construct a `*sqlite.Driver`, register the functions on it via
  `sqlfuncs.Register(drv)`, and register it under the driver name
  `stackql-sqlite`. No package-global function registration.
- The functions register unconditionally in the embedded engine. The
  `sqlite_stackql` build tag mechanism is retired; do not introduce a
  replacement build tag. This is a deliberate, documented behavior change:
  any-sdk consumers now always get the functions.
- After this PR, no reference to `mattn/go-sqlite3` remains anywhere in
  any-sdk, including the module graph: `go mod graph | grep mattn` must
  return nothing.
- Do not touch the postgres engine or any non-sqlite sqlengine code.
- Do not change the public API of `dto.SQLBackendCfg` or other exported
  types. The driver swap is internal to the engine.
- Do not create, move, or delete any git tags. Release tagging is done by a
  human after merge.
- Deliver one PR structured as the three commit groups below so the revert
  path is `git revert`.

## Stop conditions - halt and report, do not work around

1. Re-verify the compile_options diff from #783 against the pinned modernc
   version. If any newly missing `SQLITE_ENABLE_*` option is actually used
   by any-sdk or stackql code paths, STOP - the fix is an upstream request
   to the modernc maintainer.
2. If any golden vector (from sqlite-ext-functions tests or the #783 cgo
   samples) cannot be satisfied by a Go implementation without ambiguity
   about intended behavior, STOP and list the cases for human decision.
3. If the swap cannot be completed without modifying the postgres engine or
   breaking the exported API surface, STOP and describe the minimal seam
   change needed.
4. If an existing test fails for what you believe is a legitimate
   expected-behavior change, STOP - never skip, weaken, or delete a test to
   get green.

## Commit group 1: public/sqlfuncs

1. First commit: copy `public/sqlfuncs/CLAUDE.md` byte-identically to
   `public/sqlfuncs/AGENTS.md`.
2. Import the golden vectors: the test cases from
   github.com/stackql/sqlite-ext-functions `test/`, plus the cgo-build
   samples recorded in stackql PR #783, converted to table-driven format in
   `public/sqlfuncs/testdata/`.
3. Implement the six functions in pure Go per the CLAUDE.md:
   - `split_part`: 1-based indexing, negative indexes from the end,
     out-of-range behavior per the vectors.
   - `regexp_like` / `regexp_substr` / `regexp_replace`: Go `regexp` (RE2).
     Determine the C engine's replacement token syntax and translate. Every
     pattern-class divergence (backreferences, lookaround) goes in
     `public/sqlfuncs/DIVERGENCES.md`.
   - `json_equal`: canonicalized deep comparison; number forms and
     invalid-JSON behavior per the vectors.
   - `aws_policy_equal`: port the normalization logic (scalar vs array
     forms, statement order insensitivity, IAM case rules).
   - NULL propagation per function, verified against vectors, not assumed.
4. `register.go`: `func Register(drv *sqlite.Driver) error` registering all
   six as deterministic scalars.
5. Native Go fuzz targets per function, seeded from the vectors; run each
   briefly (30s) and commit findings as regression cases.

## Commit group 2: sqlite_embedded.go swap

1. Remove the mattn import from `public/sqlengine/sqlite_embedded.go`
   first; fix every resulting compile error as the mechanical inventory of
   touchpoints (error assertions, driver name, DSN construction, pooling).
2. Grep for what the compiler misses: string matching on driver error
   messages, DSN fragments built by concatenation, the literal `sqlite3`
   driver name anywhere in code, config handling, or docs.
3. Centralize DSN construction in one `BuildDSN(opts)` function translating
   mattn-style params to modernc `_pragma=` syntax
   (`_busy_timeout=5000` -> `_pragma=busy_timeout(5000)`, etc.). modernc
   silently ignores unknown mattn-style params - a missed translation fails
   silently, hence step 5.
4. Wrap driver error inspection behind internal predicates (`isBusy(err)`,
   `isConstraintViolation(err)`) mapping `*sqlite.Error` codes; no
   `*sqlite.Error` assertions outside that file.
5. Startup pragma assertion: after opening the engine, query every pragma
   the DSN sets and fail fast on mismatch. Permanent, not scaffolding.
6. Verify in-memory database semantics under the engine's existing pooling
   policy (with database/sql pooling, each new connection to a plain
   `:memory:` DSN is a separate database); preserve the current effective
   behavior, single writer connection as the safe default.
7. If timestamps are stored by the engine or its consumers' control-plane
   tables, add a round-trip test and set `_time_format` if needed.
8. Add a concurrency stress test (N reader goroutines plus a writer,
   asserting no unhandled SQLITE_BUSY) and one integration test invoking
   each of the six functions through the engine via SQL.

## Commit group 3: CI, docs, release prep

1. Set `CGO_ENABLED=0` in CI and remove any C toolchain setup.
2. `go mod tidy`; confirm `go mod graph | grep mattn` is empty.
3. Changelog / release notes draft for the next alpha (per the existing
   versioning scheme; do NOT tag): modernc driver, extension functions now
   built in unconditionally, `sqlite_stackql` build tag retired, FTS3 /
   UPDATE_DELETE_LIMIT / OMIT_DEPRECATED no longer compiled in, previous
   release is the rollback point.
4. Root-level CLAUDE.md / AGENTS.md (create or append): embedded backend is
   modernc.org/sqlite, driver name `stackql-sqlite`, DSN only via BuildDSN,
   errors only via the predicates, functions only via public/sqlfuncs,
   never reintroduce mattn or cgo.
5. Final commit: delete this work order file
   (`docs/work-orders/modernc-migration-work-order.md`) so it does not
   reach main; reproduce the acceptance checklist in the PR description
   first.

## Acceptance criteria

- [ ] Full any-sdk test suite green, no test skipped/weakened/deleted
- [ ] public/sqlfuncs unit tests green, all golden vectors passing
- [ ] Fuzz targets run clean for the smoke duration
- [ ] Integration test exercises all six functions through the engine
- [ ] Startup pragma assertions in place and passing
- [ ] Concurrency stress test passing
- [ ] `go mod graph | grep mattn` empty; `grep -ri "mattn" --exclude-dir=.git` clean outside changelog
- [ ] CI green with CGO_ENABLED=0
- [ ] DIVERGENCES.md complete; changelog drafted; no tags created
- [ ] Acceptance checklist reproduced in the PR description with per-item status

## Out of scope

- The stackql repo (bump, replace-directive removal, robot suite, and the
  performance comparison against the #783 baselines happen in a follow-up
  stackql PR against the released alpha)
- omni-sdk
- The postgres engine
- Creating releases or tags
- Cross-repo validation: do not attempt to clone or build stackql from this
  environment; the downstream robot suite is the follow-up PR's gate
