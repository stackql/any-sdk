# any-sdk: Future API Shape & Stability Guidelines

## 1. Goals

- Provide a **small, stable public surface** for:
  - Runtime execution of provider documents.
  - Provider/registry access.
  - Static analysis / address-space for provider-doc inference.
- Avoid **API sprawl**: no new exported types or functions unless they fit the agreed profiles.
- Make it possible to reimplement the core behind a **C ABI / Rust** implementation later.

## 2. Public API Profiles

### 2.1 Runtime Profile

**Purpose:** execute provider operations and stream results.

**Target packages (public):**

- `runtime`  
  - `Client` interface (`Exec`, `StreamRows`, `Close`)
  - `ExecRequest`, `ExecResult`, `Row`, `RowStream`
- `provider`  
  - `Registry`, `Provider`
  - `ProviderDescriptor`, `Capabilities`

**Rules:**

- No globals; everything hangs off `Client` or `Registry`.
- All operations take `context.Context`.
- Configuration only via functional options (`NewClient(opts...)`).

### 2.2 Analysis / Inference Profile

**Purpose:** support provider-doc inference tooling.

**Target package (public):**

- `analysis`
  - Address-space & graph abstractions
  - Static analyzer interfaces
  - Key DTOs (`AnalyzedFullHierarchy`, etc.)

Existing types such as `AddressSpace`, `AddressSpaceGrammar`, `AddressSpaceFormulator`, `StaticAnalyzer`, `AnalyzedInput/Partial/FullHierarchy`, `BrickMap` are part of this profile and may be re-exported via type aliases.

### 2.3 Persistence / SQL (Non-Core)

**Purpose:** optional helpers to persist streamed data to RDBMS systems.

- Implemented **on top of** `runtime.RowStream`.
- May live in `persistence/sql` or a separate module.
- Not part of the core runtime stability guarantees.

## 3. API Sprawl Policy

- **Do not introduce new exported functions/types** outside `runtime`, `provider`, or `analysis` without an explicit design decision.
- Prefer:
  - Adding methods to `Client` / `Registry` / `analysis.Engine` rather than top-level functions.
  - Adding fields to existing DTOs over creating new public structs.

### 3.1 When New Public API Is Allowed

New exported API is allowed only when:

1. It fits clearly into one of the profiles above; and
2. It is documented in this file (or a related ADR) before being merged.

### 3.2 Deprecation & Removal

- Mark old entry points as `// Deprecated:` with a pointer to the new API.
- Breaking removals are allowed **before v1.0.0**, but must be recorded in `CHANGELOG.md`.

## 4. CI Enforcement

- CI will run an **API growth check** that fails if new exported functions, interfaces, or structs are added compared to `main`.
- Intentional new API must:
  1. Update the API snapshot files.
  2. Update this document to justify the addition.

