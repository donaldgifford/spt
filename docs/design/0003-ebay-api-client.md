---
id: DESIGN-0003
title: "eBay API client"
status: Draft
author: Donald Gifford
created: 2026-05-24
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0003: eBay API client

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-24

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [Package layout](#package-layout)
  - [Endpoints used](#endpoints-used)
  - [Auth: app token via OAuth client-credentials](#auth-app-token-via-oauth-client-credentials)
  - [Rate-limit tracking](#rate-limit-tracking)
  - [Search and pagination](#search-and-pagination)
  - [Decoupling from domain types](#decoupling-from-domain-types)
  - [Observability](#observability)
  - [HTTP client basics](#http-client-basics)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [Resolved](#resolved)
- [References](#references)
<!--toc:end-->

## Overview

The eBay API client (`internal/ebay/`) is the only place spt talks to eBay. Its job: authenticate via OAuth, search the Browse API for listings, paginate through results, and track 24-hour rate-limit usage with persistence across restarts. The structure is lifted from the prior version of this project; the new version closes the in-memory-only quota persistence gap and decouples the client from outer domain types.

## Goals and Non-Goals

### Goals

- Single chokepoint for all eBay traffic — every eBay call goes through this package.
- Accurate 24-hour rate-limit tracking that survives process restarts.
- Periodic reconciliation against eBay's authoritative quota via the Developer Analytics API.
- Pagination that stops on the first known listing for incremental polls, with a different cap for first-run/backfill.
- Pure data types in this package (no domain leakage); domain conversion lives in a separate mapper.
- OTel-instrumented from the first call.

### Non-Goals

- The Trading API, Finding API, Inventory API, Feed API, or anything user-context (user-token) eBay APIs. App token + Browse search is the whole surface for spt.
- The extraction step (`extract` stage) — that lives in `internal/extract/` and consumes the eBay client's output.
- User-facing search inside spt — that's Meilisearch, not eBay.
- "Is this listing sold" detection — separate concern, downstream of the client.

## Background

The prior version of this project (`donaldgifford/server-price-tracker`, `internal/ebay/`) was structurally sound. A survey of that code found two endpoints used, OAuth client-credentials for auth, a two-layer rate limiter (per-second token bucket + daily counter), and a clever `Sync()` handshake against eBay's Analytics API for ground-truth quota state.

The single biggest gap in the old code: **quota state was in-memory only.** A process restart reset the daily counter and effectively re-burned quota until the next `Sync()`. For a 24-hour rolling-window service that may restart for any reason (deploy, OOM, node move), this is a real problem. The rewrite fixes it by persisting quota state to Valkey ([ADR-0005](../adr/0005-use-valkey-for-queues-and-caching.md)).

Two other things from the survey worth lifting forward:

1. **`Sync()` as a quota-truth handshake.** Local counters drift from eBay's midnight-Pacific reset; periodic Analytics API calls reconcile them. This pattern is clean and we keep it.
2. **"Stop on known listing" pagination.** Walk eBay's `newlyListed` sort until we hit a listing we've already seen; treat the sort as a monotonic stream. Cheap and effective.

Two things to NOT carry forward:

1. **Global Prometheus registry coupling.** The old code called `metrics.EbayDailyLimitHits.Inc()` directly. The new client takes an `Observer` interface (or just uses OTel spans + a Prometheus counter wired in `internal/obs/`).
2. **Domain-type coupling in `convert.go`.** The old `convert.ToListings` imported `pkg/types` (`domain.Listing`). The new eBay package is domain-agnostic; conversion to `domain.Listing` lives in a dedicated mapper outside `internal/ebay/`.

## Detailed Design

### Package layout

```
internal/ebay/
  client.go            # Client interface + shared types (SearchRequest, SearchResponse)
  browse.go            # BrowseClient: the Client implementation
  auth.go              # OAuthTokenProvider: client-credentials token cache
  ratelimit.go         # RateLimiter: per-second + 24h daily, Valkey-persisted
  analytics.go         # AnalyticsClient: ground-truth quota fetch + Sync trigger
  paginator.go         # Paginator: walks pages with stop-on-known semantics
  types.go             # Raw API response types (ItemSummary etc.) — no domain
  errors.go            # ErrDailyLimitReached, ErrUnauthorized, etc.
  *_test.go
  integration/         # //go:build integration
  mocks/               # mockery-generated
```

The package follows [DESIGN-0001](0001-go-application-layout-and-conventions.md): one interface (`Client`), one implementation (`BrowseClient`), constructor-injected dependencies, no globals.

### Endpoints used

Four eBay endpoints. One more than the prior version — `getItem` is added for the reconciliation flows in DESIGN-0002.

| Purpose            | Method + URL                                                                                  | Notes |
|--------------------|-----------------------------------------------------------------------------------------------|-------|
| OAuth token        | `POST https://api.ebay.com/identity/v1/oauth2/token`                                          | `grant_type=client_credentials`, scope `https://api.ebay.com/oauth/api_scope`. Basic-auth header with `base64(appID:certID)`. |
| Browse search      | `GET https://api.ebay.com/buy/browse/v1/item_summary/search`                                  | Query params: `q`, `category_ids`, `limit`, `offset`, `sort`, `filter` (pass-through). Required header: `X-EBAY-C-MARKETPLACE-ID: EBAY_US` (configurable). |
| Item fetch         | `GET https://api.ebay.com/buy/browse/v1/item/{item_id}`                                       | Fetch a single item by eBay item ID. Used by both reconciliation flows. Returns full `Item` shape (richer than `ItemSummary` from search) — including current `availabilityStatus` and pricing. 404 → listing pulled from eBay. |
| Quota truth        | `GET https://api.ebay.com/developer/analytics/v1_beta/rate_limit/?api_context=buy&api_name=browse` | Returns `Count`, `Limit`, `Remaining`, `ResetAt`, `TimeWindow` for the `buy.browse` resource. |

No Finding, Trading, Inventory, or Feed APIs. If we ever need them, they get their own files (`finding.go`, etc.) and their own quota tracking.

**`getItem` consumes the same `buy.browse` quota as `Search`.** The two reconciliation flows therefore share a budget with the watch polls. The alert-driven reconciliation is unconditional (alerts are user-visible state we owe correctness on); the 12h bulk reconciliation defers when the daily budget is tight. Specific budget split lives in [DESIGN-0004](0004-alert-and-reconciliation-pipeline.md).

### Auth: app token via OAuth client-credentials

```go
type TokenProvider interface {
    Token(ctx context.Context) (string, error)
}

type OAuthTokenProvider struct {
    appID, certID string
    httpClient    *http.Client

    mu      sync.Mutex
    token   string
    expiry  time.Time

    refreshBuffer time.Duration   // default 60s
    now           func() time.Time // injectable for tests
}
```

`Token()` returns the cached token if `now < expiry - refreshBuffer`; otherwise refreshes and caches. The mutex serializes refresh so two concurrent callers don't double-fetch.

**Credentials come from env: `EBAY_APP_ID`, `EBAY_CERT_ID`.** The package reads them from typed config (`config.Config.Ebay.AppID`, etc.), not from `os.Getenv` directly — config layering is the API layer's job. The 60-second `refreshBuffer` prevents in-flight requests from racing token expiry mid-call.

We do NOT support user-context OAuth. spt operates only as an app, not on behalf of an eBay user. If that ever changes it's a separate ADR.

### Rate-limit tracking

**This is the section that mattered most in the survey, and it's the one with the biggest change from the prior version.**

Two layers as before:

1. **Per-second throttle** — `golang.org/x/time/rate.Limiter` (token bucket; rate + burst from config).
2. **Daily quota** — 24-hour rolling counter against `maxDaily`.

**The change: daily counter state lives in Valkey, not process memory.** Specifically:

| Valkey key                          | Type     | Purpose |
|-------------------------------------|----------|---------|
| `spt:ebay:quota:daily:count`        | INTEGER  | atomic counter; incremented on each successful call |
| `spt:ebay:quota:daily:window_start` | STRING   | RFC3339 timestamp of the current window |
| `spt:ebay:quota:daily:reset_at`     | STRING   | RFC3339 timestamp of next reset (from Sync) |
| `spt:ebay:quota:daily:limit`        | INTEGER  | current max (refreshed by Sync) |

The interface:

```go
type RateLimiter interface {
    Wait(ctx context.Context) error   // blocks if per-second throttled; ErrDailyLimitReached if daily exhausted
    Sync(ctx context.Context, count, limit int64, resetAt time.Time) error
    Snapshot(ctx context.Context) (Snapshot, error)
}

type Snapshot struct {
    Count       int64
    Limit       int64
    Remaining   int64
    WindowStart time.Time
    ResetAt     time.Time
}
```

**Wait flow** (unchanged ordering from the prior version, which is correct):

1. `checkDailyReset(ctx)` — if `now > resetAt`, atomically roll the window: reset `daily:count` to 0, `SET window_start = now`, `SET reset_at = now + 24h`. Use Valkey `MULTI`/`EXEC` or a Lua script for atomicity.
2. If `count >= limit`, return `ErrDailyLimitReached` (wrapped with `%w`).
3. `limiter.Wait(ctx)` — per-second token bucket.
4. `INCR daily:count`.

**Daily-check-before-per-second-wait** is preserved. The prior version observed this avoids burning context-deadline budget while throttled when the daily quota is already gone. Keep it.

**Sync flow:** an `AnalyticsClient` calls the Developer Analytics endpoint on a cron (e.g., every 15 min). It calls `RateLimiter.Sync(ctx, count, limit, resetAt)`, which overwrites the Valkey keys with authoritative values. `window_start` is back-computed as `resetAt - 24h`.

**Critical caveat from the survey:** the Analytics call itself consumes a separate quota (the `developer.analytics` resource, not `buy.browse`). It's cheap, but we shouldn't call it on every request — every 15 minutes is a reasonable default with exponential backoff on failure.

**Boot behavior:** on process start, prime the limiter via `Sync()` before serving any traffic. This handles "we crashed mid-window" cleanly — eBay's view is the source of truth.

### Search and pagination

```go
type Client interface {
    Search(ctx context.Context, req SearchRequest) (SearchResponse, error)
    GetItem(ctx context.Context, ebayItemID string) (Item, error)
}

type SearchRequest struct {
    Query       string
    CategoryID  string
    Limit       int
    Offset      int
    Sort        string                  // "newlyListed", default
    Filters     map[string]string       // pass-through Browse API filter syntax
    Marketplace string                  // default "EBAY_US"
}

type SearchResponse struct {
    Items   []ItemSummary
    Total   int
    Offset  int
    Limit   int
    HasMore bool                        // derived from `Next` link in response
}

// Item is the full /buy/browse/v1/item/{item_id} payload. Richer than ItemSummary —
// includes availability status, current bid count, and seller payment/return policies.
// Used by the reconciliation flows to detect Sold / EndedNoSale / NotFound state transitions.
type Item struct {
    ItemID               string
    Title                string
    Price                ItemPrice
    AvailabilityStatus   string         // "AVAILABLE", "OUT_OF_STOCK", or absent for ended
    EstimatedAvailabilities []EstimatedAvailability
    ItemEndDate          string         // RFC3339; populated for auctions
    BidCount             int            // auctions only
    // ... full shape mirrors eBay's response; see types.go
}
```

**Error model for `GetItem` (and the rest of the client):** every error is a wrapped sentinel. The sentinel lets callers use `errors.Is(err, ebay.ErrItemNotFound)`; the wrapping carries structured debug context for logs:

```go
// In errors.go
var (
    ErrItemNotFound       = errors.New("ebay: item not found")
    ErrItemUnavailable    = errors.New("ebay: item unavailable")
    ErrDailyLimitReached  = errors.New("ebay: daily quota exhausted")
    ErrUnauthorized       = errors.New("ebay: unauthorized")
    ErrRateLimited        = errors.New("ebay: rate limited (per-second)")
    ErrTransient          = errors.New("ebay: transient error")  // retryable 5xx
)

// Structured error carrying eBay-side state, wraps a sentinel.
type ItemStateError struct {
    ItemID             string
    AvailabilityStatus string    // raw eBay availabilityStatus
    EndDate            string    // raw eBay itemEndDate, if any
    BidCount           int       // raw, for auctions
    HTTPStatus         int       // 200, 404, etc.
    Cause              error     // sentinel: ErrItemNotFound | ErrItemUnavailable
}

func (e *ItemStateError) Error() string {
    return fmt.Sprintf(
        "ebay: item=%s http=%d availability=%q end=%q bids=%d: %v",
        e.ItemID, e.HTTPStatus, e.AvailabilityStatus, e.EndDate, e.BidCount, e.Cause,
    )
}
func (e *ItemStateError) Unwrap() error { return e.Cause }
```

The reconciliation pipeline uses `errors.Is` to branch on the sentinel and `errors.As(&itemErr)` to inspect the raw eBay state — both at once. Logs see the full `Error()` string; callers see a clean sentinel. The pattern documented here is the repo-wide error convention (per DESIGN-0001's sentinel-error rule); other packages follow the same shape.

Terminal-state inference (Sold vs. EndedNoSale) is the caller's job based on `Item.AvailabilityStatus`, `BidCount`, and `ItemEndDate` — the eBay client doesn't encode that policy; the reconciliation pipeline does.

**Pagination** lives in `paginator.go`:

```go
type Paginator struct {
    client       Client
    checker      ListingChecker         // predicate, not a DB type
    log          *slog.Logger
    maxPages     int                    // default 10
    firstRunCap  int                    // default 5
    pageSize     int                    // default 50, eBay max 200
}

type ListingChecker func(ctx context.Context, ebayItemID string) (bool, error)

func (p *Paginator) Paginate(ctx context.Context, req SearchRequest, firstRun bool) ([]ItemSummary, error)
```

**Stop conditions** (any one ends the walk):

1. `checker(ctx, item.EbayItemID)` returns `true` (we've seen this listing before).
2. Pages walked >= `maxPages` (or `firstRunCap` on the first poll of a watch).
3. eBay returns no more results (`HasMore == false`).
4. Context cancelled.
5. `RateLimiter` returns `ErrDailyLimitReached`.

**Why a `ListingChecker` predicate, not a Datastore dependency:** keeps the eBay package storage-agnostic. The orchestrator wires the predicate to call `datastore.GetListingByEbayItemID`. This was a coupling issue in the prior version's `paginator.go` that we're explicitly fixing.

**Why a different first-run cap:** the first poll of a new watch should NOT backfill the entire result set. 5 pages × 50 items = 250 listings is enough to bootstrap; subsequent polls go deeper since they short-circuit quickly on the first known listing.

**Why the `newlyListed` sort assumption:** the stop-on-known logic depends on eBay returning listings in a roughly monotonic order. If we change sort to something non-monotonic (e.g., `price`), the paginator must walk the full result every time. Document this in the code.

### Decoupling from domain types

The package returns `ItemSummary` (raw API shape), not `domain.Listing`. Conversion lives in `internal/extract/mapper.go`:

```go
package extract

func ListingFromItem(item ebay.ItemSummary, watchID domain.WatchID) (domain.Listing, error)
```

This breaks the prior version's import cycle (eBay package → domain package) and lets us version the mapper separately from the eBay client.

The `ItemSummary` type mirrors eBay's payload faithfully — prices as strings, feedback percentages as strings, RFC3339 timestamps as strings. The mapper handles the parsing (`shopspring/decimal.NewFromString` for prices, `time.Parse(time.RFC3339, ...)` for timestamps). We preserve the original payload as `json.RawMessage` on `domain.Listing.RawPayload` so we can re-run the mapper without re-fetching.

### Observability

- **OTel spans** wrap every `BrowseClient.Search`, every `OAuthTokenProvider.Token` refresh, every `AnalyticsClient.GetBrowseQuota`, and every `Paginator.Paginate`. Span attributes include `ebay.endpoint`, `ebay.query`, `ebay.page`, `ebay.items_returned`.
- **`spt.span_category` is `"system"`** for eBay spans (per DESIGN-0001 routing). LLM extraction on the resulting payloads gets `"agent"`.
- **Prometheus counters / gauges** (registered in `internal/obs/`, accessed via an injected `Observer` interface):
  - `spt_ebay_api_calls_total{endpoint, marketplace}` — counter
  - `spt_ebay_daily_limit_hits_total{marketplace}` — counter; every `ErrDailyLimitReached` increments this
  - `spt_ebay_request_duration_seconds{endpoint}` — histogram
  - `spt_ebay_quota_remaining{marketplace}` — gauge, updated on Sync
  - `spt_ebay_quota_exhausted{marketplace}` — gauge, 0 or 1; set to 1 whenever a request fails with `ErrDailyLimitReached`, reset to 0 on the next successful Sync that shows quota remaining
  - `spt_ebay_sync_failures_total{reason}` — counter; tracks Analytics-endpoint reconciliation failures

**The `quota_exhausted` gauge is the alertable signal.** Quota exhaustion is a real operational incident — we lose ingestion freshness until reset, and any reconciliation-driven alert correctness work also stalls. Prometheus alert rule belongs in the published Helm chart with a sensible default (e.g., "fire if exhausted for > 30m").
- **slog** logs include `trace_id`, `span_id`, `ebay.endpoint`, and the request ID eBay returns in `X-Ebay-Request-Id` (for support cases).

The package never imports `internal/obs/` directly. It takes an `Observer` interface in its constructor.

### HTTP client basics

- Single `*http.Client` shared across the package, with a sensible `Transport` (max idle conns per host, timeout). Constructed in `internal/app/<role>/run.go` and passed in.
- `Timeout: 30s` per request. Token endpoint has a separate, shorter 10s timeout because auth should never block traffic for long.
- Retries on 5xx and 429: exponential backoff (250ms, 500ms, 1s, 2s), max 4 attempts, with jitter. 429 responses also respect any `Retry-After` header eBay sends.
- On 401 from Browse: invalidate the cached token in `OAuthTokenProvider`, force a refresh, retry once. After that, propagate the error.
- All requests carry `User-Agent: spt/<version>` and `X-Ebay-Request-Id: <our uuid>` for traceability.

## API / Interface Changes

Net-new package. No existing API changes.

## Data Model

The eBay package owns no DB tables. It does own four Valkey keys (listed in the rate-limit section). Listings, components, and watches are stored by `internal/datastore/`; the eBay package returns transient API data only.

## Testing Strategy

- **Unit tests** for `OAuthTokenProvider` (clock-injected, table-driven on refresh-buffer edge cases).
- **Unit tests** for `RateLimiter` against a fake Valkey (`miniredis`) — every state transition (under limit, at limit, over limit, post-Sync rewrite, window rollover) gets a table case.
- **Unit tests** for `BrowseClient` using `httptest.NewServer` — covers URL building, header injection, marketplace handling, retry behavior on 5xx/429/401.
- **Unit tests** for `Paginator` with a mock `Client` and `ListingChecker` — every stop condition gets a case.
- **Integration tests** under `//go:build integration` against real eBay, gated on `EBAY_APP_ID` and `EBAY_CERT_ID` env vars. One canonical search ("32GB DDR4 ECC" or similar), one quota check.

The fake-Valkey choice matters: a real Valkey via Compose is more accurate but slower; `miniredis` is fast but doesn't model all Valkey semantics. We use `miniredis` for unit tests and a real Valkey for the integration suite.

## Migration / Rollout Plan

Greenfield. Land the package alongside Phase 1 ingestion. The boot-time `Sync()` priming means the first deploy starts with accurate quota state — no warm-up window where we over-call.

## Open Questions

### Resolved

- **✅ Marketplace: EBAY_US only.** Confirmed. Type-supported for future marketplaces but no plumbing or testing for EBAY_UK / EBAY_DE / etc. in v1.
- **✅ Sync backoff: non-blocking, but exposed as a metric and alerted on.** If Analytics calls fail, ingestion continues against local counter estimates (with exponential backoff on Sync retries). `spt_ebay_quota_exhausted` and `spt_ebay_sync_failures_total` are first-class metrics; a sustained quota_exhausted or repeated Sync failures should page (alert rules ship with the Helm chart).

- **✅ Shared queue across spt instances: interface designed for it now, single-Valkey implementation for v1, scaling gaps tracked separately.**

  The `Queue` interface ([DESIGN-0002](0002-domain-and-pipeline-type-system.md)) already follows a lease/claim model (`Dequeue` → process → `Ack`/`Nack`), which works for multi-instance from day one — multiple workers can each consume independently from the same Valkey because each `Dequeue` atomically claims a Task. The Valkey implementation uses `BLMOVE` with claim TTLs from v1 (~no extra cost over fire-and-forget).

  The gaps that *do* remain for true multi-instance scaling:

  - **Leader election for `Sync()`** — when multiple spt deployments share a Valkey, only one should call eBay Analytics on the cron tick. Single-leader semantics via Postgres advisory lock or Valkey `SET NX` with TTL.
  - **Per-instance Prometheus labels** — `instance` dimension on metrics so we can disambiguate.
  - **Quota state already lives in Valkey** ✓ — designed for sharing.

  These belong in a forthcoming **DESIGN-0005 — Multi-instance scaling**, to be drafted when we actually approach multi-instance deployment. Not blocking v1, and the foundations (lease semantics, shared Valkey) are in place.

- **✅ Browse API `fields` parameter: profile during implementation, automate in CI.**

  The full `ItemSummary` payload is generous and we use a fraction of it. Bandwidth + JSON-decode CPU savings could be meaningful. The spike happens during the eBay client implementation phase, not now. The output is a benchmark that becomes part of CI — a nightly (not per-PR) `just bench-ebay` recipe that hits a fixture or the real eBay endpoint (env-gated) and reports payload sizes + decode times, with a comparison-to-baseline check. Tracking this as a Phase 1 testing concern, not a v1 blocker.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0005 — Use Valkey for queues and caching](../adr/0005-use-valkey-for-queues-and-caching.md)
- [ADR-0008 — Use OTel + ClickHouse + Langfuse for agent observability and evals](../adr/0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)
- [DESIGN-0001 — Go application layout and conventions](0001-go-application-layout-and-conventions.md)
- [DESIGN-0002 — Domain and pipeline type system](0002-domain-and-pipeline-type-system.md)
- Prior version source: <https://github.com/donaldgifford/server-price-tracker/tree/main/internal/ebay>
- eBay Browse API: <https://developer.ebay.com/api-docs/buy/browse/overview.html>
- eBay Developer Analytics API: <https://developer.ebay.com/api-docs/developer/analytics/overview.html>
