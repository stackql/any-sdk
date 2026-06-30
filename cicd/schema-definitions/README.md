# any-sdk schema definitions

JSON Schema (Draft 2020-12) definitions for the stackql provider document model. These are the canonical, publishable artefacts; CI keeps them in lockstep with the loader code.

## Artefacts

- `provider.schema.json` - the provider document (`provider.yaml`): `id`, `name`, `version`, `providerServices`, and `config`. Stable `$id`: `https://schemas.stackql.io/any-sdk/provider.schema.json`. This is the primary artefact to register with SchemaStore (matches `**/provider.yaml`).
- `stackql-config.schema.json` - the stackql `config` block (also surfaced as `x-stackQL-config` at service / resource / method levels). Referenced from `provider.schema.json` via `$ref`. Rejects unknown keys so mistyped or misplaced config keys fail validation rather than being silently ignored.
- `service-resources.schema.json`, `resources-core.schema.json`, `fragmented-resources.schema.json`, `local-templated.service-resources.schema.json` - service / resource document schemas.
- `openapi-3.0.schema.json` - vendored OpenAPI 3.0 meta-schema used by the service schemas.

## Keeping the schema in step with the code (CI)

`internal/anysdk/config_schema_test.go` asserts struct-schema agreement: every yaml key on the loader's `standardStackQLConfig` must appear in `stackql-config.schema.json` and vice versa, that the config object rejects unknown keys, and that a sample provider document validates end to end (exercising the `provider -> config` `$ref`). These run under `go test ./...` in CI, so a config field added in code without a matching schema change fails the build.

## Coverage status

Fully modelled today: the provider document envelope and the top-level `config` keys (typos at this level are caught). Deeper structures referenced from `config` (`pagination`, `queryParamPushdown`, `retry`, `variations`, `views`, `sqlExternalTables`, `auth`) are currently typed as permissive objects; tightening these, plus modelling method-level `request.nativeCasing` and `x-stackQL-graphQL`, is follow-up work.

## SchemaStore registration

Registration is performed manually (out of scope of this repo's automation). To register, add a catalog entry to https://github.com/SchemaStore/schemastore mapping provider documents to the published `$id`, for example:

```json
{
  "name": "stackql any-sdk provider document",
  "description": "stackql any-sdk provider.yaml",
  "fileMatch": ["**/provider.yaml"],
  "url": "https://schemas.stackql.io/any-sdk/provider.schema.json"
}
```

The referenced `stackql-config.schema.json` must be published alongside `provider.schema.json` at the same base URL so the relative `$ref` resolves.
