# Retry Policy

`any-sdk` retries transient HTTP failures with configurable backoff. Policies
are declared in spec YAML and resolved per request, so the same provider can
mix lenient and strict retry behaviour across resources without code changes.

## Where to declare

A `retry` block lives under a `config` (or `x-stackQL-config`) object at any
of five levels. The first level that declares one wins; nothing inherits
piecewise from a parent.

| Level | YAML location |
|---|---|
| Operation | `paths.<path>.<verb>.x-stackQL-config.retry` |
| Resource | `components.x-stackQL-resources.<name>.config.retry` |
| Service | top-level `x-stackQL-config.retry` of the service spec |
| Provider service | `providerServices.<name>.x-stackQL-config.retry` |
| Provider | `x-stackQL-config.retry` on the provider doc |

Resolution order is operation -> resource -> service -> providerService ->
provider, then a built-in default if nothing is declared.

## Example

```yaml
components:
  x-stackQL-resources:
    recoverable_configured:
      id: retrytestprovider.flaky.recoverable_configured
      name: recoverable_configured
      title: Recoverable using configured 5-attempt policy
      config:
        retry:
          algorithm: exponential
          max_attempts: 5
          initial_delay_ms: 5
          max_delay_ms: 25
          multiplier: 2
          jitter_fraction: 0.1
          retryable_methods:
            - GET
          retryable_conditions:
            status_codes:
              - 503
      methods:
        get:
          operation:
            $ref: '#/paths/~1flaky~1configured-recover/get'
          response:
            mediaType: application/json
            openAPIDocKey: '200'
```

## Fields

| Field | Type | Default | Notes |
|---|---|---|---|
| `algorithm` | string | `exponential` | Only `exponential` is implemented; unknown values fall back to a flat `initial_delay_ms`. |
| `max_attempts` | integer | `3` | Total attempts including the first try. `1` disables retry. Values `< 1` snap to the default. |
| `initial_delay_ms` | integer | `500` | Delay before the second attempt. Values `<= 0` snap to the default. |
| `max_delay_ms` | integer | `10000` | Per-attempt ceiling after exponential growth. Values `<= 0` snap to the default. |
| `multiplier` | number | `2.0` | Exponential growth factor. Values `<= 1` snap to the default. |
| `jitter_fraction` | number | `0` | Symmetric jitter band as a fraction of the computed delay (e.g. `0.2` = +/-20%). Clamped to `[0, 1]`. |
| `retryable_methods` | string[] | `["GET", "HEAD"]` | Methods eligible for retry. Use `"*"` to match every method. |
| `retryable_conditions.status_codes` | integer[] | `[408, 429, 502, 503, 504]` | HTTP statuses treated as transient. |

A request whose method is not in `retryable_methods` always makes exactly one
attempt regardless of `max_attempts`.

## Backoff calculation

For attempt _n_ (1-indexed, where _n_=1 is the wait before the second try):

```
delay = min(initial_delay_ms * multiplier^(n-1), max_delay_ms)
if jitter_fraction > 0:
    delay *= 1 + uniform(-jitter_fraction, +jitter_fraction)
    delay = clamp(delay, 0, max_delay_ms)
```

`max_delay_ms` caps both the pre-jitter exponential and the post-jitter
result, so jitter cannot push a wait past the ceiling.

## Request body handling

When `max_attempts > 1` and the request has a body, `any-sdk` reads it into
memory once and rebuilds `req.Body` (and `req.GetBody`) from the buffered
bytes for each attempt. This means:

- The body is consumed only once from the caller's reader.
- Streaming bodies of unknown size are buffered in full before the first
  attempt.
- The same bytes are replayed on every retry; transformations applied between
  the caller and `any-sdk` (translate, body rewrite) run before buffering and
  are not re-evaluated per attempt.

## Cancellation

Each backoff wait runs against `req.Context()`. If the context is cancelled
during a wait, the loop exits and returns the last response/error.

## Defaults

Omitting the `retry` block entirely (or declaring it nowhere in the chain)
yields the policy returned by `DefaultRetryPolicy()`:

- `algorithm: exponential`
- `max_attempts: 3`
- `initial_delay_ms: 500`
- `max_delay_ms: 10000`
- `multiplier: 2.0`
- `jitter_fraction: 0`
- `retryable_methods: [GET, HEAD]`
- `retryable_conditions.status_codes: [408, 429, 502, 503, 504]`

## Schema

The JSON Schema for the `retry` block lives in
[`cicd/schema-definitions/resources-core.schema.json`](../cicd/schema-definitions/resources-core.schema.json)
under `$defs/RetryPolicy` and `$defs/RetryConditions`.

## Tests

End-to-end behaviour is exercised by the Robot suite in
[`test/robot/cli/mocked/adhoc.robot`](../test/robot/cli/mocked/adhoc.robot)
against the Flask retry mock at
[`test/python/any_sdk_test_utils/mocks/retry_app.py`](../test/python/any_sdk_test_utils/mocks/retry_app.py).
The fixture provider lives at
[`test/registry-mocked/src/retrytestprovider/v0.1.0/`](../test/registry-mocked/src/retrytestprovider/v0.1.0/)
and covers four scenarios: default policy recovers after transient 503s, a
configured 5-attempt policy recovers on the fifth try, a 1-attempt policy
issues exactly one request, and a tight retry budget surfaces the final 503.
