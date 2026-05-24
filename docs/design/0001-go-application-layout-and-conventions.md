---
id: DESIGN-0001
title: "Go application layout and conventions"
status: Draft
author: Donald Gifford
created: 2026-05-24
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0001: Go application layout and conventions

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
  - [Directory layout](#directory-layout)
  - [Role selection](#role-selection)
  - [Interface-driven services](#interface-driven-services)
  - [Standard-library defaults](#standard-library-defaults)
  - [Health and metrics endpoints](#health-and-metrics-endpoints)
  - [Observability: OTel from day one](#observability-otel-from-day-one)
  - [Testing](#testing)
  - [SQL migrations](#sql-migrations)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [Resolved](#resolved)
  - [Still open](#still-open)
- [References](#references)
<!--toc:end-->

## Overview

Defines the Go project layout, package conventions, and repo-level rules for spt — on top of what `.golangci.yml` already enforces. The headline rules: every infrastructure service has an interface (even when there's one implementation), the standard library is the default, and every long-running process exposes `/healthz`, `/readyz`, and `/metrics`.

## Goals and Non-Goals

### Goals

- A single, opinionated layout that every contributor can navigate without asking.
- A repo-level rule set that complements (not duplicates) `.golangci.yml`.
- Make swapping any major infrastructure component (queue, datastore, search, scheduler) mechanical, not architectural.
- Establish the observability and health-endpoint contract every role must satisfy.

### Non-Goals

- Defining individual service designs. Those get their own DESIGN docs (scheduler/orchestrator, eBay client, datastore schema, etc.).
- Mandating a specific HTTP routing helper beyond "stdlib first, reach for `chi` only if we hit something stdlib can't do cleanly."
- Performance tuning guidance.

## Background

RFC-0001 establishes the single-binary multi-role backend. ADR-0001 commits us to Go and a std-lib-first preference. ADR-0009 commits us to Prometheus. ADR-0008 commits us to OTel + ClickHouse + Langfuse. ADR-0012 commits us to a custom scheduler/orchestrator. This document is the connective tissue: how the binary is laid out so all of those decisions cohere into one shippable codebase.

The interface-first rule is not premature abstraction. We've been burned before by reaching the "we need to swap this" moment and finding application code that imports concrete types directly. The cost of writing an interface upfront for a single-implementation service is low (one file, ~20 lines); the cost of retrofitting one across a grown codebase is high. We are accepting a known-small cost to avoid a known-large one.

## Detailed Design

### Directory layout

```
cmd/
  spt/
    main.go              # entry; wires cobra root + version/commit
internal/
  app/                   # role wiring — composes services for each cobra subcommand
    api/                 # `spt api` role
    scheduler/           # `spt scheduler` role
    worker/              # `spt worker` role
  domain/                # pure types: Watch, Listing, Component, Score, Job, Task (DESIGN-0002)
  pipeline/              # orchestrator: DAG executor, stage handlers
  queue/                 # Queue interface + Valkey implementation
  datastore/             # Datastore interface + Postgres implementation
  search/                # Search interface + Meilisearch implementation
  cache/                 # Cache interface + Valkey implementation
  ebay/                  # eBay API client (DESIGN-0003)
  agent/                 # agentic framework adapter (ADR-0011)
  health/                # /healthz, /readyz, /metrics handlers (shared)
  obs/                   # OTel + slog + Prometheus wiring (shared)
  config/                # typed config loader (env + flags + file)
  httpx/                 # small net/http helpers (middleware, error mapping)
pkg/                     # intentionally empty for now; promote out of internal/ only when external consumers exist
```

**`internal/` by default.** Packages live under `internal/` unless there's a concrete external consumer. `pkg/` is reserved for packages we intentionally publish for downstream import. Today there are none.

**`internal/app/<role>`** is the only place that imports concrete infrastructure constructors. Everywhere else, code imports interfaces from `internal/queue`, `internal/datastore`, etc., and receives concrete implementations via constructor injection.

### Role selection

The binary uses **cobra** for CLI structure (already pinned in `mise.toml` as `cobra-cli`). Subcommands map 1:1 to roles:

```
spt api          # HTTP API server (CRUD + search; Meilisearch-backed search endpoint exposed to both UI and external API consumers)
spt scheduler    # orchestrator (DAG executor + cadence trigger)
spt worker       # stage executor (consumes from Valkey)
spt migrate up   # SQL migrations (goose under the hood)
spt version
```

**The `api` role exposes the Meilisearch-backed listing search to all callers**, not just the UI. The UI consumes it via the generated OpenAPI client (ADR-0010); external API users get the same surface. Search is a first-class API capability, not a UI-private concern.

Cobra is the choice on familiarity grounds. We're not going to be clever here; cobra works, we know it, the cost of switching to `urfave/cli` or stdlib `flag` is real and the upside is nil.

Each role's entry point lives in `internal/app/<role>/run.go` exposing a `Run(ctx context.Context, cfg config.Config) error`. The cobra layer is thin — it parses flags, constructs config, and calls `Run`. No business logic in the cobra files.

### Interface-driven services

**Rule: every infrastructure service has an interface in its own package, even with one implementation.**

The package shape is:

```
internal/queue/
  queue.go         # the Queue interface, error vars, shared types
  valkey.go        # ValkeyQueue implements Queue
  valkey_test.go
  mocks/
    queue.go       # mockery-generated
```

The interface lives in the same package as the implementation. We do not split into `queue/queue` and `queue/valkey` subpackages — that's ceremony without payoff. Consumers import `internal/queue` and depend on `queue.Queue`.

Services that get this treatment in v1:

| Package              | Interface       | Implementation        |
|----------------------|-----------------|------------------------|
| `internal/queue`     | `Queue`         | `ValkeyQueue`         |
| `internal/datastore` | `Datastore`    | `PostgresDatastore`    |
| `internal/search`    | `Search`        | `MeilisearchSearch`   |
| `internal/cache`     | `Cache`         | `ValkeyCache`         |
| `internal/ebay`      | `Client`        | `BrowseClient`        |
| `internal/pipeline`  | `Scheduler`, `Executor` | concrete impls |
| `internal/agent`     | `Agent`         | `<framework>Agent`    |

Where an interface only has one method, prefer a function type (`type Fn func(...) ...`) over an interface. Interfaces are for shaped surfaces, not for ceremony.

**Composition rule:** services accept dependencies via constructor injection only. No package-level singletons, no `init()` magic, no service-locator. Constructors return concrete types; consumers store interfaces.

### Standard-library defaults

| Concern                | Default                                    | Escape hatch |
|------------------------|--------------------------------------------|--------------|
| HTTP server / routing  | `net/http` + stdlib `http.ServeMux`        | `chi` if we need middleware chaining we can't write in 20 lines |
| HTTP client            | `net/http` with a shared base `http.Client` | none |
| Logging                | `log/slog` with a JSON handler in prod, text in dev | none |
| Errors                 | `errors` + `fmt.Errorf("...: %w", err)`    | none |
| Configuration          | **HCL2** (`github.com/hashicorp/hcl/v2`) for file config + env/flag overrides, parsed into a typed `config.Config` struct | none |
| JSON                   | `encoding/json/v2`                         | fall back to `encoding/json` if v2 surfaces a blocking issue at our Go version |
| Time                   | `time` with explicit `time.Now` injection in scheduler code | none |
| SQL                    | `database/sql` + `pgx` (per ADR-0004); no ORM by default | a query-builder (e.g., `sqlc` for type-safe generated query funcs) is fair game if hand-rolled queries get repetitive |
| CLI                    | `cobra`                                    | none |

**The std-lib-first rule is a default, not dogma.** If the stdlib forces obviously bad code, we add a library — and the library choice gets a one-paragraph note in the PR description. We don't add libraries on aesthetic preference.

**JSON: try `encoding/json/v2` first.** Go 1.26 ships v2 as the recommended default for new code. We start there. If we hit a blocking issue at the version we're pinned to (`1.26.2`), fall back to `encoding/json` for the specific call site, leave a `// TODO: revert to json/v2` comment with a one-line reason, and note it in the PR.

**Configuration: HCL2.** Not YAML, not TOML. HCL2 (`github.com/hashicorp/hcl/v2`) pays back the slightly-heavier-than-YAML parsing cost because spt's configuration isn't just a flat key/value file — we want to declaratively define watches, pipeline workflows, and notification channels with the expression and block semantics HCL2 gives us natively:

```hcl
# config.hcl
log_level = "info"

ebay {
  app_id   = env("EBAY_APP_ID")
  cert_id  = env("EBAY_CERT_ID")
}

watch "dell_r730xd" {
  query    = "Dell PowerEdge R730xd"
  cadence  = "15m"
  judge_sample_rate = 0.1

  notify {
    channel = "webhook"
    threshold {
      max_percentile = 25.0
    }
  }
}
```

This is the same shape as Terraform / Packer / Nomad — blocks, labels, typed expressions, environment-variable interpolation, and the ability to split config across multiple files. YAML would force us to model the same structures as nested maps with string keys, losing the type-checked block model. The HCL2 library is a single dependency, well-maintained, with clean Go ergonomics (`gohcl.DecodeBody` into a typed struct).

Layering: HCL files (in declaration order across the configured config dir) → env vars (override scalar values) → CLI flags (final override). The loader lives in `internal/config/`.

**Errors: sentinel + wrap, and never lost.** Three rules:

1. **Every package that returns errors defines named sentinel values** in its package (`var ErrFoo = errors.New("pkg: foo")`) or named error types implementing `error`. No bare `errors.New(...)` inline from a returning function — sentinels let callers use `errors.Is` / `errors.As`.
2. **Wrapping uses `fmt.Errorf("...context: %w", err)`** following Go idioms. Wrap when adding context, return the original when context adds nothing. The Uber-style golangci rules (`errorlint`) already enforce `%w`.
3. **No error is silently dropped.** Every error is either (a) handled, (b) returned with wrap, or (c) logged at `slog.LevelError` with the wrapped error attached. `errcheck` with `check-blank: true` and `check-type-assertions: true` (already configured) catches accidental drops. The `_ = thing()` pattern requires a comment explaining why.

For structured error information (e.g., HTTP status, eBay item state), define a typed error that *wraps* a sentinel — so callers can use `errors.Is(err, ebay.ErrItemNotFound)` AND `errors.As(err, &itemErr)` to pull the structured fields. See DESIGN-0003 for the pattern.

### Health and metrics endpoints

Every long-running role (`api`, `scheduler`, `worker`) serves three HTTP endpoints on a separate admin port (default `9090`):

| Endpoint    | Purpose                                                              | Response                            |
|-------------|----------------------------------------------------------------------|-------------------------------------|
| `/healthz`  | Process is alive; the binary is running and accepting connections    | `200 OK` always (when serving)      |
| `/readyz`   | Process is ready to do work — dependencies (DB, Valkey, etc.) reachable | `200 OK` ready, `503` not ready  |
| `/metrics`  | Prometheus scrape endpoint                                           | text/plain Prometheus exposition    |

Implementation lives in `internal/health/`. Roles register readiness probes for their dependencies at construction time:

```go
h := health.New()
h.RegisterReadiness("postgres", ds.Ping)
h.RegisterReadiness("valkey", q.Ping)
h.Serve(ctx, ":9090")
```

The admin port is separate from any business port (e.g., the API role's `:8080`). This keeps scrape and probe traffic off the main listener and lets us protect the business port behind auth without compromising health checks.

### Observability: OTel from day one

OTel is wired in `internal/obs/` and initialized in every role's `Run`. There is no "we'll add tracing later" path; spans wrap HTTP handlers, DB queries, queue operations, and external API calls from v1.

**Trace routing splits by trace category:**

| Category                                    | Destination                       |
|---------------------------------------------|-----------------------------------|
| LLM calls + agent reasoning + extraction    | ClickHouse + Langfuse             |
| Everything else (API requests, DB, queue, scheduler ticks) | ClickHouse, or OTel collector → Grafana Tempo |

The category is determined by span attribute (`spt.span_category = "agent" | "system"`) set at span-start time. The OTel exporter is a custom `SpanProcessor` that dual-publishes agent spans to Langfuse and sends everything to the primary ClickHouse exporter. The general-trace destination is configurable so users running the Helm chart can plug into their existing observability stack.

Prometheus and slog wiring also live in `internal/obs/`. Convention: `log/slog` writes structured logs with `trace_id` and `span_id` attributes pulled from `context.Context`, so logs and traces correlate automatically.

### Testing

- **`testify/require`** for assertions in tests. We do not also pull in `testify/assert`; one assertion style per repo.
- **Table-driven tests** are the default. Each test is `tests := []struct{ name string; ...; want ...; wantErr ... }{...}` followed by `for _, tt := range tests { t.Run(tt.name, func(t *testing.T) {...}) }`. Subtests run in parallel where safe (`t.Parallel()`).
- **`mockery`** generates mocks for every interface. Mocks live next to the interface in a `mocks/` subpackage; the package is excluded from coverage and most lint rules (see `.golangci.yml`'s `mock_*.go` exclusion — we'll widen it to the `mocks/` directory).
- **Integration tests** use the `//go:build integration` tag and run against real services (a `docker compose` stack defined in `test/integration/`). They are not part of `just test` — they run via `just test-integration` and in a separate CI job.
- **No live external calls in unit tests.** eBay and LLM calls in unit tests go through their interface and use a mock. Integration tests can use real eBay (gated on env vars).

### SQL migrations

Use **`github.com/pressly/goose`**. Migrations live in `internal/datastore/migrations/` as numbered `.sql` files (`20260524120000_create_watches.sql`). `goose` is invoked via `spt migrate up | down | status` subcommands that wrap the library directly — no separate `goose` binary in the production image.

**Choice: goose over `golang-migrate/migrate`.** Three reasons:

1. **SQL-first with optional Go migrations.** goose treats `.sql` files as the primary artifact and lets us drop down to Go when we need transactional schema changes that mix DDL and data. golang-migrate's Go-migration story is weaker.
2. **Single dependency.** goose ships as a Go library we can embed in the `spt migrate` subcommand. golang-migrate works the same way but its driver model is more complex.
3. **Activity and governance.** Both are maintained, but golang-migrate has had stretches of slow maintenance and contested PRs. goose has been steady. Neither is a blocker; this is a tiebreaker.

If we ever need migration features goose doesn't have (e.g., complex rollback graphs), revisit. We're not blocking on this — the migration format is small enough that switching tools is a mechanical port.

## API / Interface Changes

This document **establishes** the rules; it doesn't change an existing API. The shape of each major interface (`Queue`, `Datastore`, `Search`, etc.) is captured in [DESIGN-0002](0002-domain-and-pipeline-type-system.md).

## Data Model

N/A. Data model lives in DESIGN-0002 (types) and individual schema docs.

## Testing Strategy

The testing rules above ARE the strategy. To restate:

- **Unit tests** — `_test.go` next to source, table-driven, mocked external dependencies, run via `just test`.
- **Integration tests** — `//go:build integration`, real services via Compose, run via `just test-integration`.
- **End-to-end / pipeline tests** — defer to a per-pipeline test doc; not in scope here.

## Migration / Rollout Plan

This doc lands first; subsequent code reviews check against it. Adoption is mechanical: as we land Phase 1 scaffolding, every new package follows the layout. Any deviation gets called out in PR review with a link back to this doc.

## Open Questions

### Resolved

- **✅ HTTP routing: stdlib `http.ServeMux`.** Go 1.22+ pattern matching covers everything we need today; `chi` stays as an escape hatch only if middleware composition gets demonstrably ugly.
- **✅ Go version: `1.26.2`** (already pinned in `go.mod` and `mise.toml`).
- **✅ JSON: `encoding/json/v2` first**, fall back per call site if blocked. Documented in the std-lib defaults section above.
- **✅ Config layering: HCL2.** Single dependency (`github.com/hashicorp/hcl/v2`), file→env→flag precedence, types decoded into a typed `config.Config` struct via `gohcl`. Details in the std-lib defaults section above.

### Still open

- **Span-category attribute name.** `spt.span_category` is a placeholder. **Langfuse has its own conventions** for span naming (it expects specific span types like `generation`, `span`, `event`, with attributes like `langfuse.observation.type`). The agent-category spans must conform to whatever Langfuse expects to ingest; system spans can use our own convention. Concrete attribute names land once we've prototyped against Langfuse — implementation-time decision, not design-time.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0001 — Use Go for the backend](../adr/0001-use-go-for-the-backend.md)
- [ADR-0004 — Use PostgreSQL as the canonical relational store](../adr/0004-use-postgresql-as-the-canonical-relational-store.md)
- [ADR-0008 — Use OTel + ClickHouse + Langfuse for agent observability and evals](../adr/0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)
- [ADR-0009 — Use Prometheus for system metrics](../adr/0009-use-prometheus-for-system-metrics.md)
- [ADR-0011 — Use sdk-booty-sh as the agentic framework](../adr/0011-use-sdk-booty-sh-as-the-agentic-framework.md)
- [ADR-0012 — Build a custom scheduler and pipeline orchestrator](../adr/0012-build-a-custom-scheduler-and-pipeline-orchestrator.md)
- [DESIGN-0002 — Domain and pipeline type system](0002-domain-and-pipeline-type-system.md)
- goose: <https://github.com/pressly/goose>
- cobra: <https://github.com/spf13/cobra>
- mockery: <https://github.com/vektra/mockery>
- HCL2: <https://github.com/hashicorp/hcl>
