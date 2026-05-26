---
id: IMPL-0001
title: "Foundation: Go layout, CLI, config, observability, and migrations"
status: Draft
author: Donald Gifford
created: 2026-05-25
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0001: Foundation: Go layout, CLI, config, observability, and migrations

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-25

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Repo skeleton and module hygiene](#phase-1-repo-skeleton-and-module-hygiene)
  - [Phase 2: Cobra root and role scaffolding](#phase-2-cobra-root-and-role-scaffolding)
  - [Phase 3: HCL2 config loader](#phase-3-hcl2-config-loader)
  - [Phase 4: Observability core](#phase-4-observability-core)
  - [Phase 5: Health and admin endpoints](#phase-5-health-and-admin-endpoints)
  - [Phase 6: Service interface skeletons](#phase-6-service-interface-skeletons)
  - [Phase 7: Testing infrastructure](#phase-7-testing-infrastructure)
  - [Phase 8: SQL migration scaffolding](#phase-8-sql-migration-scaffolding)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Resolved Decisions](#resolved-decisions)
- [References](#references)
<!--toc:end-->

## Objective

Land the foundational scaffolding described in [DESIGN-0001](../design/0001-go-application-layout-and-conventions.md): directory layout, cobra-based role selection, HCL2 typed config, slog/OTel/Prometheus wiring, health and metrics endpoints, service interface skeletons (without concrete implementations), testing infrastructure, and SQL migration scaffolding via `goose`. Every later IMPL doc — eBay client, orchestrator, agentic layer, tools — depends on this scaffolding existing.

**Implements:** [DESIGN-0001](../design/0001-go-application-layout-and-conventions.md)

## Scope

### In Scope

- Directory layout per [DESIGN-0001 — Directory layout](../design/0001-go-application-layout-and-conventions.md#directory-layout).
- `spt` binary entry, cobra root, and the five subcommands (`api`, `scheduler`, `worker`, `migrate`, `version`) with stub `Run` functions that log a startup line and return cleanly on `ctx.Done()`.
- HCL2 config loader: typed `config.Config` struct, file → env → flag precedence, validation.
- Observability initialization shared across roles: `slog` (text/JSON), OTel tracer provider, Prometheus registry, the agent-vs-system span-category split per [ADR-0008](../adr/0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md).
- `/healthz`, `/readyz`, `/metrics` endpoints on a configurable admin port (default `:9090`), served by every long-running role.
- Service interface declarations only — `Queue`, `Datastore`, `Search`, `Cache`, `ebay.Client`, `Scheduler`, `Agent` — to establish the import shape. Concrete implementations are deferred to per-package IMPL docs.
- Testing infrastructure: `testify/require`, `mockery` mock generation, integration build-tag pattern, Compose stack for integration tests, CI wiring.
- `goose`-based migration scaffolding and `spt migrate {up,down,status}` subcommands with embedded migrations.

### Out of Scope

- **Concrete implementations of the service interfaces.** `ValkeyQueue`, `PostgresDatastore`, `MeilisearchSearch`, `BrowseClient`, the pipeline orchestrator, and the agent adapter all land in their own IMPL docs (one per DESIGN). This IMPL only declares the interfaces.
- The actual eBay schema migrations and DDL — DESIGN-0002 owns those, landed via the per-package IMPL.
- Production deployment, Helm chart, Docker image hardening — those land in the packaging IMPL paired with RFC-0001 Phase 5.
- Per-stage handlers and DAG topology code — those belong to the orchestrator IMPL paired with [DESIGN-0005](../design/0005-pipeline-orchestrator-and-worker-model.md).
- Watch-definition HCL syntax and runtime semantics beyond what's needed to validate config layering. (See [Resolved Decisions](#resolved-decisions) #4 and #5.)

## Implementation Phases

Each phase ships independently and can be reviewed/merged as its own PR. Earlier phases unblock later ones — the dependency order is roughly the phase order, with the exceptions noted. **Every phase except Phase 1 depends on Phase 1.** Phase 2 unblocks the docgen tool in [IMPL-0002 Phase 2](0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-2-docgen-inline-as-spt-gen-docs); Phase 7 establishes the conventions every tool in [IMPL-0002](0002-developer-tooling-port-and-rewrite-from-old-spt.md) should follow.

| Phase | Theme | Unblocks |
|-------|-------|----------|
| 1 | Repo skeleton | All other phases; all subsequent IMPLs |
| 2 | Cobra root + role stubs | [IMPL-0002 Phase 2 (docgen)](0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-2-docgen-inline-as-spt-gen-docs); per-role IMPLs |
| 3 | HCL2 config | Any code that needs typed config (every role) |
| 4 | Observability core | Any code that needs slog/OTel/Prometheus (every package) |
| 5 | Health endpoints | Helm chart readiness probes; per-role liveness |
| 6 | Service interfaces | Per-package IMPLs (eBay client, orchestrator, datastore, ...) |
| 7 | Testing infrastructure | Every IMPL's testing tasks, including [IMPL-0002](0002-developer-tooling-port-and-rewrite-from-old-spt.md) |
| 8 | SQL migration scaffolding | Per-package IMPLs that own schemas (datastore) |

---

### Phase 1: Repo skeleton and module hygiene

**Reference design:** [DESIGN-0001 — Directory layout](../design/0001-go-application-layout-and-conventions.md#directory-layout).

Establish the empty package tree from DESIGN-0001 with placeholder `doc.go` files so subsequent phases land into the right locations.

#### Tasks

- [x] Create the directory tree from [DESIGN-0001 — Directory layout](../design/0001-go-application-layout-and-conventions.md#directory-layout):
  - [x] `internal/app/{api,scheduler,worker,cli}/`
  - [x] `internal/{domain,pipeline,queue,datastore,search,cache,ebay,agent,health,obs,config,httpx}/`
  - [x] `pkg/` (intentionally empty for now; add `.gitkeep` to track)
- [x] Add `doc.go` in each new package with a one-line package comment describing the package's responsibility.
- [x] Confirm `cmd/spt/main.go` exists and is a `package main` stub. _(Pre-existing stub had no `main()` so `go build` failed; added `func main() {}` to make Phase 1's build gate pass. Phase 2 expands this.)_
- [x] Update top-level `CLAUDE.md` if the layout deviates from what's documented there. _(Updated Project state to reference IMPL-0001 and list the package tree.)_
- [x] Run `go mod tidy` to confirm no spurious deps were added.
- [x] Run `just build` to confirm the binary still builds against the empty packages. _(Fixed pre-existing justfile bug — every recipe used `{{ "{{" }} var {{ "}}" }}` which outputs literal `{{var}}` instead of substituting. Replaced with correct just template syntax `{{ var }}` across all 15 instances; recipes were not functional before this fix.)_

#### Success Criteria

- `go build ./...` succeeds with all new packages present.
- `go vet ./...` clean.
- Directory tree matches the DESIGN-0001 layout table.
- `git status` shows only intended additions (no accidental file moves).

---

### Phase 2: Cobra root and role scaffolding

**Reference design:** [DESIGN-0001 — Role selection](../design/0001-go-application-layout-and-conventions.md#role-selection).

Build the cobra command tree and stub `Run` functions for each role. Roles do not yet do anything — they log a startup line, block on `ctx.Done()`, and exit cleanly. This phase unblocks [IMPL-0002 Phase 2 (docgen)](0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-2-docgen-inline-as-spt-gen-docs) because docgen needs the cobra root + at least two real subcommands to gate against.

#### Tasks

- [x] Add cobra dependency (`github.com/spf13/cobra`) to `go.mod`; confirm `mise.toml` pins `cobra-cli`.
- [x] `cmd/spt/main.go`: thin entry that builds `rootCmd` from `internal/app/cli/` and calls `cobra.Command.ExecuteContext(ctx)` with a `signal.NotifyContext` for SIGINT/SIGTERM.
- [x] `internal/app/cli/root.go`: `NewRootCmd()` returns the root `*cobra.Command` with persistent flags `--config`, `--log-format`, `--log-level`, `--admin-addr`. **No `Run` function on `rootCmd`** so `spt` with no subcommand prints help and exits 0 (per [Resolved Decisions](#resolved-decisions) #1).
- [x] `--log-format` default behavior: if unset, the value resolves to `text` when `os.Stderr.Fd()` is a TTY (`golang.org/x/term.IsTerminal`), `json` otherwise. Explicit `--log-format=json|text` always wins (per [Resolved Decisions](#resolved-decisions) #2).
- [x] `internal/app/cli/version.go`: `spt version` subcommand prints version, commit, build date, Go version; `--json` flag emits structured JSON.
- [x] `internal/app/cli/api.go`: `spt api` subcommand calls `internal/app/api.Run(ctx, cfg)`.
- [x] `internal/app/cli/scheduler.go`: `spt scheduler` subcommand calls `internal/app/scheduler.Run(ctx, cfg)`.
- [x] `internal/app/cli/worker.go`: `spt worker` subcommand calls `internal/app/worker.Run(ctx, cfg)`.
- [x] `internal/app/cli/migrate.go`: `spt migrate` parent subcommand with `up`/`down`/`status` child stubs (full implementation in Phase 8).
- [x] `internal/app/api/run.go`: `Run(ctx, cfg) error` logs `slog.Info("api role starting")`, blocks on `ctx.Done()`, returns `ctx.Err()` on shutdown.
- [x] `internal/app/scheduler/run.go`: same shape.
- [x] `internal/app/worker/run.go`: same shape.
- [x] Wire `ldflags -X` for version/commit/date into `justfile`'s `build` recipe (verify the existing recipe already does this; document in code comments).
- [x] Unit tests: `version --json` produces valid JSON with expected fields; `spt --help` lists all subcommands; signal handling produces clean exit on SIGINT.
- [x] Update [IMPL-0002 Phase 2](0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-2-docgen-inline-as-spt-gen-docs) prerequisite note to point at this phase's completion as the unblocking event.

> **Phase 2 implementation notes (deltas from spec):**
> - `Run(ctx, cfg)` signatures take `*config.Config` rather than `config.Config` to satisfy `gocritic`'s `hugeParam` rule once Phase 3 expands the struct (~88 bytes today, larger soon).
> - `main()` returns through `os.Exit(run())` so the `signal.NotifyContext` cleanup runs before exit (avoids `exitAfterDefer`).
> - `context.Canceled` from `ExecuteContext` is swallowed by `main` — SIGINT/SIGTERM during a role's `Run` is the normal shutdown path and exits 0.
> - `spt version` uses `PersistentPreRunE = noopPreRun` to skip slog installation; version output is the only command that needs to succeed regardless of logger flags.

#### Success Criteria

- `just build && ./build/bin/spt --help` lists `api`, `scheduler`, `worker`, `migrate`, `version`.
- `spt version --json` emits `{"version": "...", "commit": "...", "date": "...", "go": "..."}`.
- `spt api`, `spt scheduler`, `spt worker` each start, log one line, exit 0 on SIGINT.
- `spt --help` does NOT surface `gen-docs` (hidden flag will be added in IMPL-0002 Phase 2).
- All unit tests pass.

---

### Phase 3: HCL2 config loader

**Reference design:** [DESIGN-0001 — Standard-library defaults (Configuration: HCL2)](../design/0001-go-application-layout-and-conventions.md#standard-library-defaults).

Implement the typed config system: HCL files → env vars → CLI flags, decoded into a `config.Config` struct via `gohcl`.

#### Tasks

- [x] Add `github.com/hashicorp/hcl/v2` dependency.
- [x] `internal/config/types.go`: define `Config` root struct and nested config types — at minimum:
  - [x] `LogConfig{Format, Level}`
  - [x] `AdminConfig{Addr}`
  - [x] `EbayConfig{AppID, CertID, Marketplace, RateLimit}` (sourced from [DESIGN-0003](../design/0003-ebay-api-client.md))
  - [x] `PostgresConfig{DSN, MaxOpenConns, MaxIdleConns}`
  - [x] `ValkeyConfig{Addr, DB, Password}`
  - [x] `MeilisearchConfig{URL, APIKey}`
  - [x] `ObsConfig{OTLPEndpoint, LangfuseHost, LangfusePublicKey, LangfuseSecretKey, SpanSampling}` (sourced from [DESIGN-0005](../design/0005-pipeline-orchestrator-and-worker-model.md))
  - [x] `ApiConfig{Addr, ReadTimeout, WriteTimeout}`
  - [x] `SchedulerConfig{TickInterval, BulkReconcileInterval, SyncInterval}`
  - [x] `WorkerConfig{Pools map[string]PoolConfig}` per [DESIGN-0005 — Worker pool model](../design/0005-pipeline-orchestrator-and-worker-model.md#worker-pool-model)
- [x] Add `hcl:"<name>,attr|block"` struct tags throughout.
- [x] `internal/config/loader.go`: `Load(paths []string, env map[string]string, flags FlagOverrides) (Config, error)`:
  - [x] Parse each HCL file in declaration order via `hclparse.NewParser`.
  - [x] Merge files: later files override earlier (block-by-block, attribute-by-attribute).
  - [x] Apply env var overrides for scalar fields (decoded via the HCL eval context's `env()` function, evaluated at decode time).
  - [x] Apply CLI flag overrides last (mutates the decoded `Config`).
- [x] `internal/config/validate.go`: per-field required/range validation; aggregate errors into a single `*config.ValidationError` listing every problem.
- [x] `internal/config/loader.go`: support a `--config-dir` flag in addition to `--config` (single-file convenience); when both supplied, dir's files load first then explicit files override.
- [x] **Config discovery: explicit-only.** No XDG/`/etc/spt/`/`$SPT_CONFIG` auto-discovery; if no file-based config is needed (all required values supplied via env/flags), `--config` may be omitted. Document this explicitly in `internal/config/README.md` (per [Resolved Decisions](#resolved-decisions) #3).
- [x] Define the `watch "<name>" { ... }` HCL block in `internal/config/types.go` and validate it parses cleanly (per [Resolved Decisions](#resolved-decisions) #5). **Seeding behavior is NOT in this IMPL** — the seed-from-HCL logic lives in the datastore IMPL that owns the Watch table (per [Resolved Decisions](#resolved-decisions) #4 — HCL is bootstrap-and-seed; runtime CRUD goes through the API). Document the deferred behavior in `internal/config/README.md`.
- [x] Provide a sample config under `test/config/example.hcl` exercising every documented block.
- [x] Document the schema in `internal/config/README.md` with a complete annotated example.
- [x] Wire `Load` into `cmd/spt/main.go` (or `internal/app/cli/root.go`) so every role's `Run` receives a validated `Config`.
- [x] Unit tests:
  - [x] Single file parses and validates.
  - [x] Multi-file precedence (later overrides earlier).
  - [x] Env var override of a scalar.
  - [x] CLI flag override of an env var override.
  - [x] Missing required field produces a clear error mentioning the field path.
  - [x] Invalid HCL syntax produces a diagnostic with file + line.

> **Phase 3 implementation notes (deltas from spec):**
> - **Duration fields are strings** (e.g., `tick_interval = "5s"`) rather than `time.Duration`. gohcl's cty decoder doesn't support `time.Duration`; helpers in `internal/config/durations.go` (`ParsedTickInterval`, etc.) expose them as `time.Duration` with field-path-tagged errors.
> - **Loader uses an intermediate `parseSchema`** with `*BlockConfig` pointers so the user can omit any section. The public `Config` keeps value semantics; the projection step copies non-nil pointer sections onto the value type.
> - **Per-file decode + sequential projection** is the merge strategy. `hcl.MergeFiles` rejects duplicate single-instance blocks across files, which would forbid the file→file override pattern.
> - **Required-field enforcement** for production dependencies (Postgres DSN, eBay credentials, Valkey, Meilisearch) is **deferred** to Phase 4+. Enforcing them now would block local development on stub roles that don't open those connections. The role-aware required-field matrix is documented in `internal/config/README.md`.
> - **CLI flag override semantics** use `pflag.Flag.Changed` so that the friendly default values shown in `spt --help` don't always override HCL — only flags the user actually typed propagate.
> - **`WorkerConfig.Pools`** is a `[]PoolConfig` slice (not a `map[string]PoolConfig`), matching gohcl's labelled-block decoding model. Each pool is a `pools "<stage>" { concurrency = N }` block.

#### Success Criteria

- `spt --config=test/config/example.hcl api` parses and runs (the stub `Run`).
- Setting `EBAY_APP_ID=foo` in env, then passing `--ebay-app-id=bar` via flag, results in `cfg.Ebay.AppID == "bar"`.
- Missing required field (e.g., `postgres.dsn`) produces a `ValidationError` listing the field path and aborts before any role starts.
- All unit tests pass under `go test -race ./internal/config/...`.

---

### Phase 4: Observability core

**Reference design:** [DESIGN-0001 — Observability: OTel from day one](../design/0001-go-application-layout-and-conventions.md#observability-otel-from-day-one), [ADR-0008](../adr/0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md), [ADR-0009](../adr/0009-use-prometheus-for-system-metrics.md).

Wire structured logging, distributed tracing, and metrics. The agent-vs-system span split makes Langfuse and ClickHouse/Tempo addressable from one trace producer.

#### Tasks

- [x] `internal/obs/slog.go`: `NewLogger(format string, level slog.Level) *slog.Logger`. Format `"json"` → `slog.NewJSONHandler`; `"text"` → `slog.NewTextHandler`. Default level `slog.LevelInfo`.
- [x] `internal/obs/context.go`: `LoggerFromContext(ctx)` returning the logger with `trace_id`/`span_id` attributes attached when a span is active.
- [x] `internal/obs/tracing.go`: `NewTracerProvider(ctx, cfg ObsConfig) (*sdktrace.TracerProvider, func(context.Context) error, error)`. Sets up:
  - [x] OTLP/HTTP exporter targeting `cfg.OTLPEndpoint` for system spans. **Direct OTLP, no embedded collector** — operators run their own collector if they want one (per [Resolved Decisions](#resolved-decisions) #6).
  - [x] A custom `SpanProcessor` that ALSO publishes spans with `spt.span_category = "agent"` to Langfuse (HTTP POST per Langfuse's ingestion API).
  - [x] `TraceIDRatioBased(cfg.SpanSampling)` sampler with `SpanSampling` defaulting to `1.0` (100%, per [Resolved Decisions](#resolved-decisions) #7). Reads from `cfg.Obs.SpanSampling` in the typed config.
  - [x] Returns a shutdown function for clean flush at process exit.
- [x] `internal/obs/span_category.go`:
  - [x] Constants `SpanCategorySystem = "system"`, `SpanCategoryAgent = "agent"`.
  - [x] Helper `SetCategory(span trace.Span, cat string)` setting the `spt.span_category` attribute. (Per [DESIGN-0001 — Still open](../design/0001-go-application-layout-and-conventions.md#still-open), the exact Langfuse-compatible attribute name is implementation-time — confirm during prototyping; expose as a const.)
- [x] `internal/obs/metrics.go`:
  - [x] `NewRegistry() *prometheus.Registry` with default Go + process collectors registered.
  - [x] Helper to inject `instance` label (per [DESIGN-0005 — Multi-instance scaling](../design/0005-pipeline-orchestrator-and-worker-model.md#multi-instance-scaling-and-leader-election)) onto every metric registered via a `prometheus.WrapRegistererWith`.
- [x] `internal/obs/setup.go`: `Setup(ctx, cfg) (*Obs, func(context.Context) error, error)` one-call init returning a struct carrying `Logger`, `TracerProvider`, `Registry`, plus a shutdown that flushes everything in order.
- [x] Wire `obs.Setup` into each role's `Run` (replaces the bare `slog.Info` from Phase 2).
- [x] Unit tests:
  - [x] Logger format selection (json vs text).
  - [x] `LoggerFromContext` picks up `trace_id`/`span_id` when a span is active.
  - [x] `SetCategory` sets the expected attribute.
  - [x] Span-category-split SpanProcessor publishes agent spans to a mock Langfuse exporter AND to the system exporter (the system one gets every span; Langfuse gets only agent).
  - [x] Prometheus registry exposes Go + process collectors.

> **Phase 4 implementation notes (deltas from spec):**
> - **Langfuse exporter is plumbed but unwired.** `obs.Setup` constructs the OTLP exporter and the agent/system filter (`categoryFilterProcessor`) but leaves `TracerOptions.LangfuseExporter` nil. The Langfuse OTel client lands with the agent IMPL — drop it into `setup.go` then and agent-tagged spans route automatically. Tests exercise the filter against an in-process `recordingExporter`.
> - **Graceful shutdown uses `context.WithoutCancel`** (Go 1.21+) so the bounded 5s `shutdownTimeout` runs even when ctx was cancelled by SIGINT.
> - **`installSlog` from Phase 2 now delegates to `obs.NewLogger`** so there's one logger factory. `cli/logger.go` exists only for the PreRun pass that gives short-lived subcommands (migrate stubs, the help dispatch) a configured handler; long-running roles let `obs.Setup` install the full bundle.
> - **Service name passed per-role** (`spt-api`, `spt-scheduler`, `spt-worker`) so OTel `service.name` resource distinguishes the role in collector queries.

#### Success Criteria

- `spt --log-format=json api` produces JSON log lines including `trace_id` and `span_id` when a span is active.
- A toy trace started in `Run` reaches a local OTLP collector (manual smoke-test, documented in `internal/obs/README.md`).
- An agent-categorized span lands in Langfuse (manual smoke-test against a Langfuse dev project; documented).
- Prometheus registry can be scraped over HTTP (exposed by Phase 5's `/metrics` handler).
- Process flushes traces and logs cleanly on SIGINT.
- All unit tests pass.

---

### Phase 5: Health and admin endpoints

**Reference design:** [DESIGN-0001 — Health and metrics endpoints](../design/0001-go-application-layout-and-conventions.md#health-and-metrics-endpoints).

Stand up `/healthz`, `/readyz`, `/metrics` on the admin port. Roles register readiness probes for their dependencies at startup.

#### Tasks

- [x] `internal/health/health.go`: `Health` type with `RegisterReadiness(name string, probe func(ctx) error)` and `Serve(ctx, addr) error`.
- [x] `/healthz` handler: returns `200 OK` always (process is alive if it can respond).
- [x] `/readyz` handler: invokes every registered probe with a short timeout (default `2s`); returns `200` if all pass, `503` otherwise; response body is JSON listing per-probe status (`{"postgres": "ok", "valkey": "error: connection refused"}`).
- [x] `/metrics` handler: backed by the Prometheus registry from Phase 4 via `promhttp.HandlerFor`.
- [x] Wire `health.Serve` into each role's `Run`, with a separate `http.Server` listening on `cfg.Admin.Addr` (default `:9090`).
- [x] Graceful shutdown: `health.Server.Shutdown(ctx)` is called when role `Run` returns; bounded by a 5s timeout.
- [x] Unit tests:
  - [x] `/healthz` returns 200 with no probes registered.
  - [x] `/readyz` returns 200 when all probes pass.
  - [x] `/readyz` returns 503 when any probe fails; body lists the failing probe.
  - [x] Probe timeout fires within the configured window.
  - [x] `/metrics` returns a valid Prometheus exposition format response.

> **Phase 5 implementation notes (deltas from spec):**
> - **Listener is opened synchronously** in `Serve` (via `net.ListenConfig.Listen(ctx, ...)`) before the serve goroutine starts. This way `Server.Addr()` is populated by the time callers see the listener active — necessary for tests using `":0"`.
> - **Type is `*Server`, not `*Health`** — the package name `health` already carries the role; calling the type `Server` matches stdlib conventions (`http.Server`, `grpc.Server`).
> - **Probes register at construction**, not at `Serve` time. Roles instantiate `health.New(o.Registry)`, attach probes, then hand the server to a goroutine that calls `Serve`.

#### Success Criteria

- `spt api &; curl http://localhost:9090/healthz` returns 200.
- `curl http://localhost:9090/readyz` returns the correct code based on registered probes.
- `curl http://localhost:9090/metrics` returns Prometheus exposition output including Go runtime metrics.
- Admin port is separable from the API role's business port (Phase 2's `cfg.Api.Addr` would be different).
- All unit tests pass.

---

### Phase 6: Service interface skeletons

**Reference design:** [DESIGN-0001 — Interface-driven services](../design/0001-go-application-layout-and-conventions.md#interface-driven-services).

Declare the seven core service interfaces in their respective packages. No concrete implementations — those land in per-package IMPL docs. This phase is the import-shape contract every later IMPL relies on.

#### Tasks

- [x] `internal/queue/queue.go`: `Queue` interface per [DESIGN-0002](../design/0002-domain-and-pipeline-type-system.md#queue) + [DESIGN-0005 additions](../design/0005-pipeline-orchestrator-and-worker-model.md#api--interface-changes). Sentinel errors (`ErrQueueClosed`, etc.). Shared types only.
- [x] `internal/datastore/datastore.go`: `Datastore` interface per [DESIGN-0002](../design/0002-domain-and-pipeline-type-system.md#datastore). Sentinel errors (`ErrNotFound`, etc.).
- [x] `internal/search/search.go`: `Search` interface (read/write operations against the listings index).
- [x] `internal/cache/cache.go`: `Cache` interface (get/set/del/expire).
- [x] `internal/ebay/client.go`: `Client` interface per [DESIGN-0003](../design/0003-ebay-api-client.md#search-and-pagination). Plus `RateLimiter`, `TokenProvider`, `ListingChecker` from the same DESIGN.
- [x] `internal/ebay/errors.go`: sentinel errors per [DESIGN-0003 — Search and pagination](../design/0003-ebay-api-client.md#search-and-pagination) (`ErrItemNotFound`, `ErrItemUnavailable`, `ErrDailyLimitReached`, `ErrUnauthorized`, `ErrRateLimited`, `ErrTransient`) and the `ItemStateError` structured type.
- [x] `internal/pipeline/scheduler.go`: `Scheduler` interface per [DESIGN-0002](../design/0002-domain-and-pipeline-type-system.md#scheduler) + [DESIGN-0005 additions (`CancelJob`)](../design/0005-pipeline-orchestrator-and-worker-model.md#api--interface-changes).
- [x] `internal/agent/agent.go`: `Agent` interface placeholder — minimum surface the orchestrator's `extract`/`judge` stage handlers will call. Real shape lands with the agentic IMPL.
- [x] Each package gets a `doc.go` with a one-paragraph description of the package's role and a pointer to the relevant DESIGN doc.
- [x] No implementations in this phase — every interface has at least one declaration, no `var _ Queue = (*ValkeyQueue)(nil)` style compile-time guards yet (those land in the per-package IMPL).
- [x] Compile check: `go build ./internal/...` succeeds with all interfaces present and no circular imports.

> **Phase 6 implementation notes (deltas from spec):**
> - **Domain types are seeded as placeholders.** `internal/domain/{ids,stage,state,types}.go` ships the strongly-typed IDs (`WatchID`, `ListingID`, …), the `Stage` enum (including the DESIGN-0004 reconciliation stages), the lifecycle enums (`JobState`, `TaskState`, `JobTrigger`), and minimal struct shapes (`Watch`, `Listing`, `Component`, `Job`, `Task`, `WatchFilter`). Full field sets land with each per-table IMPL — Phase 6 only needs enough surface for the interfaces to compile.
> - **`datastore.WatchFilter` is a type alias to `domain.WatchFilter`.** Avoids a parallel type when the shapes don't yet diverge; later phases can swap in a separate type if query-shape and data-shape need to differ.
> - **No `var _ Queue = (*ValkeyQueue)(nil)` guards yet** — the spec explicitly defers those to per-package IMPLs.

#### Success Criteria

- `go build ./internal/...` succeeds.
- Every interface from [DESIGN-0001 — Interface-driven services](../design/0001-go-application-layout-and-conventions.md#interface-driven-services) (Queue, Datastore, Search, Cache, ebay.Client, Scheduler, Agent) is declared and importable.
- Sentinel errors per DESIGN-0003 are declared in `internal/ebay/errors.go`.
- `mockery` can generate mocks for every interface (validated in Phase 7).
- No package has a circular import.

---

### Phase 7: Testing infrastructure

**Reference design:** [DESIGN-0001 — Testing](../design/0001-go-application-layout-and-conventions.md#testing).

Stand up the testing conventions: `testify/require`, table-driven tests, `mockery` mocks per interface, an integration build-tag pattern, and a Compose stack for integration tests. Every later IMPL (including [IMPL-0002](0002-developer-tooling-port-and-rewrite-from-old-spt.md)) follows these conventions.

#### Tasks

- [x] Confirm `testify` and `mockery` are pinned in `mise.toml` (`mockery` is already pinned).
- [x] Add `testify/require` dependency to `go.mod` (used in tests; not required by non-test code).
- [x] Create `.mockery.yaml` at the repo root listing every interface from Phase 6:
  - `internal/queue.Queue`
  - `internal/datastore.Datastore`
  - `internal/search.Search`
  - `internal/cache.Cache`
  - `internal/ebay.{Client,RateLimiter,TokenProvider,ListingChecker}`
  - `internal/pipeline.Scheduler`
  - `internal/agent.Agent`
  - Output to `<package>/mocks/`.
- [x] Add `just mocks-generate` recipe: `mockery`.
- [x] Run `just mocks-generate` and commit the generated mocks.
- [x] Widen `.golangci.yml`'s `mock_*.go` exclusion to also match `mocks/` directories under any package.
- [x] Create `test/integration/docker-compose.yml` bringing up: Postgres (matching the production version), Valkey, Meilisearch. Use deterministic ports + healthchecks.
- [x] Add `just test-integration` recipe: `docker compose -f test/integration/docker-compose.yml up -d --wait && go test -tags=integration -race ./... ; docker compose -f test/integration/docker-compose.yml down -v`.
- [x] Add one smoke integration test that asserts each service in the Compose stack responds to a ping (proves the harness works end-to-end).
- [x] Add a CI workflow job for integration tests (per [Resolved Decisions](#resolved-decisions) #9):
  - [x] Separate job from the PR fast-path; runs `docker compose` directly.
  - [x] **PR trigger:** runs only when a `run-integration` label is applied to the PR (gh actions: `if: contains(github.event.pull_request.labels.*.name, 'run-integration')`).
  - [x] **Scheduled trigger:** nightly cron (`0 3 * * *` UTC) against `main` to catch drift even without label-gated PR runs.
  - [x] Add the `run-integration` label to `.github/labeler.yml` definitions so operators can apply it manually or via branch convention.
  - [x] Note in `docs/testing.md`: "If the integration suite stays under ~5 minutes, revisit moving to PR fast-path."
- [x] Document the testing conventions (table-driven pattern, `require` over `assert`, integration tag usage) in `docs/testing.md` (or fold into `CLAUDE.md`).
- [x] Update [IMPL-0002](0002-developer-tooling-port-and-rewrite-from-old-spt.md)'s testing tasks to reference this phase as the baseline (mockery, testify/require, build tags).

> **Phase 7 implementation notes (deltas from spec):**
> - **Mockery bumped from v2.53.6 → v3.7.0.** The v2 binary (built against Go 1.25) cannot parse Go 1.26 source. The v3 config format is different — `template: testify` replaces the old `with-expecter` knob, and `template-data` schema is per-template. Pinned in `mise.toml`.
> - **`ListingChecker` is NOT mocked.** It's a function type, not an interface — tests pass a closure directly.
> - **Labeler auto-applies `run-integration`** when `test/integration/**` is touched or the branch starts with `integration/`, in addition to manual apply. Matches the spec's "operators can apply it manually or via branch convention" requirement.

#### Success Criteria

- `just mocks-generate` produces compilable mocks under every `<package>/mocks/` directory.
- `just test` runs the unit suite cleanly.
- `just test-integration` brings up the Compose stack, runs the integration smoke test, and tears down without leftover containers or volumes.
- `golangci-lint run` does not flag generated mocks.
- The CI integration job runs successfully on a sample PR (or on main-branch merge depending on the gating decision).

---

### Phase 8: SQL migration scaffolding

**Reference design:** [DESIGN-0001 — SQL migrations](../design/0001-go-application-layout-and-conventions.md#sql-migrations).

Wire `goose` into the binary via `spt migrate up | down | status`. Migrations are embedded into the binary via `embed.FS`.

#### Tasks

- [ ] Add `github.com/pressly/goose/v3` dependency.
- [ ] `internal/datastore/migrations/` directory with `embed.FS`.
- [ ] First migration file: `internal/datastore/migrations/00001_initial.sql` containing a minimal placeholder schema (e.g., a `_spt_meta` table the migrator can use as a smoke target). Real DDL ships with the datastore IMPL.
- [ ] `internal/datastore/migrate.go`:
  - [ ] `Migrator` struct with `Up(ctx) error`, `Down(ctx) error`, `Status(ctx) (Status, error)` methods wrapping `goose`.
  - [ ] Constructor accepts `*sql.DB` and an `fs.FS` (defaulting to the embedded one; allows override via `--migrations-dir` flag for dev workflows).
  - [ ] Uses goose's timestamp filename pattern: `YYYYMMDDHHMMSS_<snake_name>.sql`.
- [ ] Expand the stubs from Phase 2's `internal/app/cli/migrate.go`:
  - [ ] `spt migrate up` — apply all pending migrations.
  - [ ] `spt migrate down` — roll back the last migration.
  - [ ] `spt migrate status` — print applied/pending list as a table.
  - [ ] All take `--migrations-dir` to override the embedded FS during dev.
- [ ] **No auto-migrate on role startup** (per [Resolved Decisions](#resolved-decisions) #12). Each role's `Run` calls `Migrator.Status` at startup and fails fast with a clear error if there are pending migrations; the operator must run `spt migrate up` explicitly. Matches the Kubernetes Job/initContainer deployment pattern.
- [ ] Document the operator workflow in `internal/datastore/README.md`: standalone migration step before deploying role pods; reference in the Helm chart README when packaging lands.
- [ ] `just db-up`, `just db-down`, `just db-status` recipes that wrap `spt migrate` against the Compose Postgres from Phase 7.
- [ ] Integration test (`//go:build integration`) under `internal/datastore/migrate_test.go`:
  - [ ] `Up` against a fresh Postgres applies all migrations.
  - [ ] `Status` reports the correct count.
  - [ ] `Down` rolls back the last one.
- [ ] Confirm embedded migrations are present in the built binary (`strings build/bin/spt | grep 00001_initial`).

#### Success Criteria

- `spt migrate up` applies the placeholder migration against the Compose Postgres.
- `spt migrate status` reports `1 applied, 0 pending`.
- `spt migrate down` rolls back successfully.
- Embedded migrations are bundled into the production binary (no separate file shipped).
- Integration test passes under `just test-integration`.

---

## File Changes

| File / Directory | Action | Phase | Description |
|------------------|--------|-------|-------------|
| `internal/app/{api,scheduler,worker,cli}/`, `internal/{domain,pipeline,queue,datastore,search,cache,ebay,agent,health,obs,config,httpx}/`, `pkg/.gitkeep` | Create | 1 | Empty package tree with `doc.go` placeholders. |
| `cmd/spt/main.go` | Modify | 2 | Expand from stub to cobra entry. |
| `internal/app/cli/{root,version,api,scheduler,worker,migrate}.go` | Create | 2 | Cobra command tree. |
| `internal/app/{api,scheduler,worker}/run.go` | Create | 2 | Role `Run(ctx, cfg)` entry points (stub bodies in Phase 2; filled by per-role IMPLs). |
| `internal/config/{types,loader,validate}.go`, `internal/config/README.md` | Create | 3 | HCL2 config system. |
| `test/config/example.hcl` | Create | 3 | Sample config exercising every block. |
| `internal/obs/{slog,context,tracing,span_category,metrics,setup}.go` | Create | 4 | Observability initialization. |
| `internal/health/health.go` | Create | 5 | `/healthz`, `/readyz`, `/metrics` server. |
| `internal/{queue,datastore,search,cache,ebay,pipeline,agent}/*.go` | Create | 6 | Service interface declarations + sentinel errors. |
| `.mockery.yaml` | Create | 7 | Mockery generation config. |
| `internal/**/mocks/` | Create | 7 | Generated mocks (committed). |
| `test/integration/docker-compose.yml` | Create | 7 | Postgres + Valkey + Meilisearch for integration tests. |
| `docs/testing.md` (or `CLAUDE.md` section) | Create / Modify | 7 | Testing conventions. |
| `internal/datastore/migrations/00001_initial.sql` | Create | 8 | Placeholder migration. |
| `internal/datastore/migrate.go` | Create | 8 | `Migrator` wrapping goose. |
| `justfile` | Modify | 2, 7, 8 | Add/update `build` ldflags (Phase 2), `mocks-generate` + `test-integration` (Phase 7), `db-up/down/status` (Phase 8). |
| `.github/workflows/ci.yml` | Modify | 7 | Add integration test job. |
| `.golangci.yml` | Modify | 7 | Widen `mocks/` exclusion. |

## Testing Plan

- [ ] Every new package in Phases 3–6 ships with `_test.go` files using `testify/require` and table-driven tests.
- [ ] Phase 7's integration smoke test validates the harness end-to-end before any later IMPL tries to use it.
- [ ] `go test -race ./...` clean as a precondition for merging any phase.
- [ ] CI matrix: PR job runs unit tests; integration job runs on `run-integration` label + nightly cron against main (per [Resolved Decisions](#resolved-decisions) #9).
- [ ] Coverage target: >70% for `internal/config/`, `internal/obs/`, `internal/health/`, `internal/datastore/` (the foundation packages that are tested in this IMPL). Per-package targets land with each IMPL.

## Dependencies

**Cross-phase prerequisites within this IMPL:**

- Phase 2 depends on Phase 1.
- Phase 3 depends on Phase 2 (config integration into `Run`).
- Phase 4 depends on Phase 3 (Observability config block).
- Phase 5 depends on Phase 4 (Prometheus registry).
- Phase 6 is independent of Phases 2–5 once Phase 1 is in place, but Phase 7's mock generation needs Phase 6's interfaces to exist.
- Phase 8 depends on Phase 2 (migrate subcommand stubs) and Phase 7 (Compose Postgres for integration test).

**External dependencies:**

- `github.com/spf13/cobra` (Phase 2)
- `github.com/hashicorp/hcl/v2` (Phase 3)
- OTel Go SDK + OTLP HTTP exporter, `log/slog` (stdlib), Prometheus client_golang (Phase 4)
- `github.com/stretchr/testify`, `github.com/vektra/mockery` (Phase 7, mockery already pinned via mise)
- `github.com/pressly/goose/v3` (Phase 8)

**Cross-IMPL relationships:**

- This IMPL unblocks every per-package IMPL (eBay, datastore, orchestrator, agentic, packaging).
- Phase 2 specifically unblocks [IMPL-0002 Phase 2 (docgen)](0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-2-docgen-inline-as-spt-gen-docs).
- Phase 7's testing conventions are referenced from [IMPL-0002's Testing Plan](0002-developer-tooling-port-and-rewrite-from-old-spt.md#testing-plan).
- [IMPL-0002 Phase 1 (mock-server)](0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-1-mock-server) can land in parallel with most of this IMPL — the mock-server is a standalone HTTP server under `tools/mock-server/` that doesn't depend on `internal/` scaffolding.

## Resolved Decisions

These were open questions during drafting; recommendations were accepted and the tasks above already reflect the outcomes. Captured here for traceability — if any need to be revisited mid-implementation, edit the relevant phase tasks and add a note here explaining why.

| # | Decision | Affects |
|---|----------|---------|
| 1 | **`spt` with no subcommand prints help and exits 0.** No default role. `rootCmd` has no `Run` function; cobra's built-in help dispatch handles the bare invocation. | Phase 2 |
| 2 | **`--log-format` defaults to TTY-detected:** `text` when `os.Stderr` is a terminal, `json` otherwise. Explicit `--log-format=json\|text` always overrides. | Phase 2, Phase 4 |
| 3 | **Config discovery is explicit-only.** No `$SPT_CONFIG` env, no XDG search, no `/etc/spt/` fallback. `--config` and `--config-dir` are the only sources; omit them if all required values come from env/flags. | Phase 3 |
| 4 | **HCL `watch` blocks are bootstrap-and-seed**, not runtime source-of-truth. First boot inserts a Watch if it doesn't already exist; subsequent boots are no-ops. Runtime CRUD goes through the API. | Phase 3 (parsing only) + datastore IMPL (seeding) |
| 5 | **The `watch` HCL block parsing lands in this IMPL; the seed behavior defers to the datastore IMPL.** Phase 3 proves the shape parses cleanly via `internal/config/types.go`. | Phase 3 |
| 6 | **Direct OTLP HTTP/gRPC exporters in v1.** No embedded OTel collector; operators run their own collector via Helm values if they want one. | Phase 4 |
| 7 | **Tracing sampling rate: config knob `obs.span_sampling`, default `1.0`** (100%). Tunable down once volume warrants. Implemented via `TraceIDRatioBased(cfg.SpanSampling)`. | Phase 4 |
| 8 | **`Agent` interface declared in this IMPL as a tiny placeholder** with `Extract`/`Judge` methods over typed I/O. Real shape refines in the agentic IMPL. Lets the orchestrator IMPL begin without waiting. | Phase 6 |
| 9 | **Integration test CI: label-gated (`run-integration`) + nightly cron against main.** Not on PR fast-path. Revisit moving to PR fast-path if suite stays under ~5 minutes. | Phase 7 |
| 10 | **Generated mocks are committed** under `<package>/mocks/`. Interface changes and mock diffs land in the same PR for reviewability. | Phase 7 |
| 11 | **First "real" migration lands in the datastore IMPL**, not this one. This IMPL ships only the `_spt_meta` placeholder migration that proves the migrator works. | Phase 8 |
| 12 | **No auto-migrate on role startup.** `api`/`scheduler`/`worker` roles call `Migrator.Status` at startup and fail-fast on pending migrations. Operator runs `spt migrate up` explicitly. Matches the Kubernetes Job/initContainer deployment pattern. | Phase 8 |

## References

- [DESIGN-0001 — Go application layout and conventions](../design/0001-go-application-layout-and-conventions.md) — the design this implements
- [DESIGN-0002 — Domain and pipeline type system](../design/0002-domain-and-pipeline-type-system.md) — source of `Queue`, `Datastore`, `Scheduler` interfaces in Phase 6
- [DESIGN-0003 — eBay API client](../design/0003-ebay-api-client.md) — source of `ebay.Client` interface + sentinel errors in Phase 6
- [DESIGN-0005 — Pipeline orchestrator and worker model](../design/0005-pipeline-orchestrator-and-worker-model.md) — source of Scheduler additions and observability metric labels
- [ADR-0001 — Use Go for the backend](../adr/0001-use-go-for-the-backend.md)
- [ADR-0008 — Use OTel + ClickHouse + Langfuse for agent observability and evals](../adr/0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)
- [ADR-0009 — Use Prometheus for system metrics](../adr/0009-use-prometheus-for-system-metrics.md)
- [ADR-0011 — Use sdk-booty-sh as the agentic framework](../adr/0011-use-sdk-booty-sh-as-the-agentic-framework.md)
- [ADR-0012 — Build a custom scheduler and pipeline orchestrator](../adr/0012-build-a-custom-scheduler-and-pipeline-orchestrator.md)
- [IMPL-0002 — Developer tooling port and rewrite from old spt](0002-developer-tooling-port-and-rewrite-from-old-spt.md) — depends on Phase 2 (docgen unblocking) and Phase 7 (testing conventions)
- goose: <https://github.com/pressly/goose>
- cobra: <https://github.com/spf13/cobra>
- HCL2: <https://github.com/hashicorp/hcl>
- mockery: <https://github.com/vektra/mockery>
