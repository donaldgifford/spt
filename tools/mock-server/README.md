# spt-mock-server

An in-memory eBay-shaped HTTP mock used by unit tests, integration tests, and local development in place of the real eBay Browse / Identity / Analytics APIs.

Designed in [DESIGN-0006 — mock-server](../../docs/design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#mock-server); built per [IMPL-0002 Phase 1](../../docs/impl/0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-1-mock-server).

## What it does

- Serves the eBay endpoints `internal/ebay/Client` consumes:
  - `POST /identity/v1/oauth2/token` — static Bearer token.
  - `GET /buy/browse/v1/item_summary/search` — keyword search with multi-word matching against fixture titles.
  - `GET /buy/browse/v1/item/{item_id}` — per-item GET resolved through the active scenario.
  - `GET /developer/analytics/v1_beta/rate_limit/` — quota snapshot.
- Stamps `X-EBAY-API-Call-Limit`, `X-EBAY-API-Calls-Made`, and `X-EBAY-API-Calls-Remaining` headers on every successful eBay-shape response so the rate-limiter under test sees realistic header values.
- Supports runtime-configurable scenarios, quota state, and fault injection via `/admin/*` endpoints — see below.

## Invocation

```bash
# Via the just recipe (canonical):
just tool mock-server -- serve --scenario=default

# Via go run:
go run ./tools/mock-server serve --port=8080 --scenario=default

# Via the Docker image:
docker run --rm -p 8080:8080 ghcr.io/donaldgifford/spt-mock-server:latest serve
```

### Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `--port` | `8080` | TCP port to bind. |
| `--scenario` | `default` | Active scenario name. |
| `--log-format` | `auto` | `text`, `json`, or `auto` (TTY-detected). |
| `--log-level` | `info` | `debug`, `info`, `warn`, `error`. |
| `--fixtures-dir` | (embedded) | Override the embedded fixtures with an on-disk directory. |
| `--token-expires` | `2h` | OAuth token TTL reported to clients. |

## Admin endpoints

The mock exposes `/admin/*` endpoints for runtime mutability — integration tests use these to flip scenarios mid-test, mark quota tight, or inject faults.

### `POST /admin/scenario`

Switch the active scenario. The scenario directory must already be loaded (under `fixtures/`).

```bash
curl -X POST localhost:8080/admin/scenario -d '{"name": "sold-listings"}'
```

### `POST /admin/quota`

Overlay a quota snapshot onto live state. Use to mark quota tight (count near limit) for testing the eBay client's bulk-deferral logic.

```bash
curl -X POST localhost:8080/admin/quota \
  -d '{"count": 4500, "limit": 5000, "reset_after": "23h"}'
```

### `POST /admin/fault`

Append a fault rule. Endpoint is a regex matched against the request path; `latency_ms` adds delay; `fail_rate` (0.0–1.0) probability of an eBay-shaped 503.

```bash
# Slow down item lookups by 1 s with a 10 % failure rate.
curl -X POST localhost:8080/admin/fault \
  -d '{"endpoint": "/buy/browse/v1/item/.*", "latency_ms": 1000, "fail_rate": 0.1}'

# Clear all rules.
curl -X POST localhost:8080/admin/fault -d '{"clear": true}'
```

### `GET /admin/scenarios`

List loaded scenarios and which is active.

```bash
curl localhost:8080/admin/scenarios
# {"active":"default","available":["default","sold-listings","ended-no-sale"]}
```

## Scenario authoring

A scenario is a directory under `tools/mock-server/fixtures/`. Layout:

```
fixtures/
  default/
    search.json                 # search response template
    items/
      <url-encoded-item-id>.json
  sold-listings/
    items/
      <url-encoded-item-id>.json
  ended-no-sale/
    items/
      <url-encoded-item-id>.json
```

**File naming.** eBay item IDs use `|` as a separator (`v1|151234567890|0`). Go's `embed` package forbids `|` in embedded filenames, so fixtures use URL-encoded names on disk: `v1%7C151234567890%7C0.json`. The loader URL-decodes filenames back to the canonical item ID when populating the scenario map.

**Inheritance.** When the active scenario doesn't contain a fixture for the requested item ID, the resolver falls back to `default/`. This keeps scenarios small — `sold-listings/` only ships the items whose responses differ.

**Per-scenario quota.** A scenario may ship a `quota.json` with an initial `QuotaSnapshot`. Activation calls `QuotaState.Apply(snapshot)`.

## Why these design choices

- **Runtime-configurable faults rather than fixture-only:** integration tests want to flip "now all requests to GetItem time out" mid-test to exercise the reconciler's stale-detection path. A static fixture can't model that.
- **`embed.FS` rather than a bind mount:** the Docker image works the same as the binary; no fixture bind-mounts required for the common case.
- **Pure-stdlib filter cache:** `containsAllWords` plus a per-item lowercased-title cache keeps the search endpoint fast enough that test runs aren't dominated by mock CPU time.
- **First-match-wins fault rule semantics:** with overlapping rules, the earliest declared wins. This keeps behavior predictable when an operator layers a broad "slow everything down" rule under a more specific exception.

## Testing

```bash
just test-pkg ./tools/mock-server/...
```

Tests are pure-Go, no Compose required. The `fixtures/` tree is exercised both via `//go:embed` (the production loader) and `testing/fstest.MapFS` (per-test synthetic trees).
