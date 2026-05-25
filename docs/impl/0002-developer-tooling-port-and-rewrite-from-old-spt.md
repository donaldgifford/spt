---
id: IMPL-0002
title: "Developer tooling port and rewrite from old spt"
status: Draft
author: Donald Gifford
created: 2026-05-25
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0002: Developer tooling port and rewrite from old spt

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-25

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: mock-server](#phase-1-mock-server)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: docgen (inline as spt gen-docs)](#phase-2-docgen-inline-as-spt-gen-docs)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: dataset-bootstrap](#phase-3-dataset-bootstrap)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: dataset-upload](#phase-4-dataset-upload)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: judge-bootstrap](#phase-5-judge-bootstrap)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: regression-runner](#phase-6-regression-runner)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: dashgen](#phase-7-dashgen)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Resolved Decisions](#resolved-decisions)
- [References](#references)
<!--toc:end-->

## Objective

Implement the seven developer tools designed in [DESIGN-0006](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md), porting and refactoring from prior-version spt (`donaldgifford/server-price-tracker/tools/`). Each tool lands in the spt repo at the v1 platform phase that natively depends on it, per [INV-0002](../investigation/0002-old-spt-tools-triage-port-priorities-for-v1.md).

**Implements:** [DESIGN-0006](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md)

## Scope

### In Scope

- All seven tools from prior-version `tools/`: mock-server, docgen, dataset-bootstrap, dataset-upload, judge-bootstrap, regression-runner, dashgen.
- Per-tool: package layout, cobra entry points, unit tests, README, and `just`/`docker.just` recipes as applicable.
- CI gating where appropriate (mock-server unit tests on PRs, docgen drift check, dashgen `-validate` check). Explicitly **excludes** regression-runner from CI per its preserved anti-CI rationale.
- The shared `tools/` directory conventions established in [DESIGN-0006 — Repo layout for tools](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#repo-layout-for-tools).

### Out of Scope

- The platform features each tool supports (the eBay client, the agentic framework, the Helm chart). Those are tracked in their own IMPL docs.
- Fixtures content beyond an initial seed set per scenario — operators add more over time as real eBay payloads accumulate.
- Operational runbooks for using the tools in production (these are dev-facing utilities).
- Migrating any running infrastructure from the old repo. We port patterns, not state.

## Implementation Phases

Each phase below corresponds to one tool. Phases are **not strictly sequential** — they each gate on the v1 platform phase that depends on the tool, per the cross-reference column below. Within a phase, tasks are ordered from foundation outward and should be checked off as completed.

| IMPL Phase | Tool | Ships in RFC-0001 phase | DESIGN-0006 section |
|------------|------|-------------------------|---------------------|
| 1 | mock-server | [Phase 1 — Foundation / ingestion](../rfc/0001-server-price-tracker-platform.md) | [mock-server](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#mock-server) |
| 2 | docgen (inline) | [Phase 1 — Foundation, once cobra root exists](../rfc/0001-server-price-tracker-platform.md) | [docgen](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#docgen) |
| 3 | dataset-bootstrap | [Phase 3 — Agentic workflows and evals](../rfc/0001-server-price-tracker-platform.md) | [dataset-bootstrap](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#dataset-bootstrap) |
| 4 | dataset-upload | [Phase 3 — Agentic workflows and evals](../rfc/0001-server-price-tracker-platform.md) | [dataset-upload](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#dataset-upload) |
| 5 | judge-bootstrap | [Phase 3 — Agentic workflows and evals](../rfc/0001-server-price-tracker-platform.md) | [judge-bootstrap](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#judge-bootstrap) |
| 6 | regression-runner | [Phase 3 — Agentic workflows and evals](../rfc/0001-server-price-tracker-platform.md) | [regression-runner](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#regression-runner) |
| 7 | dashgen | [Phase 5 — Packaging and Helm release](../rfc/0001-server-price-tracker-platform.md) | [dashgen](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#dashgen) |

---

### Phase 1: mock-server

**Ships in:** RFC-0001 Phase 1 (Foundation / ingestion). Blocking dependency for the eBay client unit tests ([DESIGN-0003](../design/0003-ebay-api-client.md)) and the reconciliation integration tests ([DESIGN-0004](../design/0004-alert-and-reconciliation-pipeline.md)).

**Reference design:** [DESIGN-0006 — mock-server](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#mock-server).
**Prior art:** `donaldgifford/server-price-tracker/tools/mock-server`.

#### Tasks

- [ ] Scaffold `tools/mock-server/` with `main.go` containing a cobra root command (subcommand `serve`).
- [ ] Add flags: `--port`, `--scenario`, `--log-format`, `--log-level`, `--fixtures-dir` (override embedded fixtures during local fixture iteration).
- [ ] Define `Server` struct in `server.go` wrapping `*http.Server`, scenario registry, fault injector, quota state, logger.
- [ ] Wire `Server.Routes()` returning an `http.Handler` with all real-eBay-shape and `/admin/*` endpoints from DESIGN-0006.
- [ ] Set up `//go:embed fixtures` in `server.go`; create `fixtures/default/` with seed search response and a handful of item JSON files.
- [ ] Implement `ScenarioRegistry` in `scenarios.go`: loads all subdirectories of `fixtures/` at startup; `Resolve(active, itemID)` walks the active scenario then falls back to `default/`.
- [ ] Implement `POST /identity/v1/oauth2/token` (static `Bearer` token, configurable expiry, exact response shape from prior art).
- [ ] Implement `GET /buy/browse/v1/item_summary/search`:
  - [ ] Port `containsAllWords` filter from prior art.
  - [ ] Port lowercased-title cache pattern (compute at fixture load, reuse per request).
  - [ ] Support `q`, `category_ids`, `limit`, `offset`, `sort`, `filter` query params.
  - [ ] Respect `X-EBAY-C-MARKETPLACE-ID` header (default `EBAY_US`).
- [ ] Implement `GET /buy/browse/v1/item/{item_id}` consulting `ScenarioRegistry.Resolve`; return 404 with eBay-shaped error body when not found.
- [ ] Implement `GET /developer/analytics/v1_beta/rate_limit/?api_context=buy&api_name=browse` returning `QuotaState` snapshot.
- [ ] Implement `QuotaState` (concurrent-safe via `sync.Mutex`) with `count`, `limit`, `resetAt`, `timeWindow`, `autoIncr` toggle.
- [ ] Implement `QuotaState.Middleware` that increments `count` on every successful eBay-shape response and stamps `X-EBAY-API-Call-Limit`, `X-EBAY-API-Calls-Made`, `X-EBAY-API-Calls-Remaining` response headers.
- [ ] Implement `FaultInjector` in `faultinject.go` with `[]FaultRule` (regex pattern, latency ms, fail rate); `Middleware` wraps the entire mux and applies rules to matching paths.
- [ ] Wire admin endpoints: `POST /admin/scenario`, `POST /admin/quota`, `POST /admin/fault`. JSON request bodies per DESIGN-0006.
- [ ] Build initial scenario set: `default/`, `sold-listings/`, `ended-no-sale/`. No static `quota-tight/` fixture — tests exercise quota-tight behavior via `POST /admin/quota` at runtime.
- [ ] Multi-stage `Dockerfile` (`golang:1.26-alpine` → `alpine:3.21`), lifted from prior art with version bumps.
- [ ] Add `tools/mock-server/README.md` covering: what it does, invocation, admin endpoints, scenario authoring guide.
- [ ] Add generic `just tool <name> -- <args>` recipe in `justfile` (used by every tool, not just mock-server).
- [ ] Add `just -f docker.just tool-image mock-server` recipe.
- [ ] CI: add `docker/build-push-action` step on main-branch merges that publishes `ghcr.io/donaldgifford/spt-mock-server:<sha>` and `:latest`. Same channel as the main `spt` image.
- [ ] Unit test: `ScenarioRegistry.Resolve` — active hit, fallback hit, double-miss.
- [ ] Unit test: `FaultInjector` rule matching — single rule, multiple rules, no-match passthrough, latency timing within tolerance.
- [ ] Unit test: `QuotaState` — concurrent `INCR` under race, header values match snapshot, reset rolls correctly.
- [ ] Unit test: `containsAllWords` — multi-word query, case-insensitive, no false positives.
- [ ] End-to-end smoke test: start `Server` on a `net.Listen("tcp", ":0")` port; point a real `internal/ebay/Client` at it; perform a search + getItem; assert payloads round-trip.
- [ ] CI: add `go test ./tools/mock-server/...` to the `just test` invocation so it runs on every PR.
- [ ] Confirm DESIGN-0004 integration test references (alert opens → reconcile → sold → close) pass when pointed at the mock-server in Compose.

#### Success Criteria

- `just tool mock-server -- serve --scenario=default` starts cleanly on `:8080` and responds to every documented endpoint with eBay-shaped JSON.
- The Docker image built from `tools/mock-server/Dockerfile` runs the same scenarios in a `docker run` invocation.
- The integration tests in [DESIGN-0004 — Testing Strategy](../design/0004-alert-and-reconciliation-pipeline.md#testing-strategy) (`alert opens → reconcile → sold → alert closes`, `bulk sweep deferral when quota tight`, `Stale set on simulated GetItem 5xx`) pass against the mock-server.
- All unit tests pass under `go test -race ./tools/mock-server/...`.
- The mock-server's `internal/ebay/` smoke test passes from a clean checkout with no manual fixture editing.

---

### Phase 2: docgen (inline as `spt gen-docs`)

**Ships in:** RFC-0001 Phase 1, once the cobra root and at least two real subcommands exist (otherwise the generated tree is too thin to be worth gating CI on).

**Prerequisite:** [IMPL-0001 Phase 2 (Cobra root and role scaffolding)](0001-foundation-go-layout-cli-config-observability-and-migrations.md#phase-2-cobra-root-and-role-scaffolding) is complete — the cobra root and subcommands this phase generates docs against must exist first.

**Reference design:** [DESIGN-0006 — docgen](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#docgen).
**Prior art:** `donaldgifford/server-price-tracker/tools/docgen` (~30 LOC).

This phase deliberately produces **no `tools/docgen/`** — the implementation collapses into a hidden cobra subcommand on the main `spt` binary.

#### Tasks

- [ ] Create `internal/app/cli/docs.go` defining the hidden `docsCmd`:
  - `Use: "gen-docs <output-dir>"`
  - `Hidden: true`
  - `Args: cobra.ExactArgs(1)`
  - `RunE` calls `doc.GenMarkdownTree(rootCmd, args[0])`.
- [ ] Register `docsCmd` on the root cobra tree (in whatever file does root-command registration once cobra lands).
- [ ] Add `just docs-cli` recipe: `go run ./cmd/spt gen-docs docs/cli/`.
- [ ] Generate the initial `docs/cli/*.md` tree and commit it.
- [ ] Add CI step in `.github/workflows/ci.yml` after `just check`: `just docs-cli && git diff --exit-code docs/cli/`. Step fails on drift.
- [ ] Unit test in `internal/app/cli/docs_test.go`: build a minimal cobra tree, invoke `docsCmd.RunE` against `t.TempDir()`, assert at least one expected file exists.
- [ ] Confirm `spt --help` does **not** list `gen-docs` (Hidden: true); confirm `spt gen-docs --help` still works.

#### Success Criteria

- Running `just docs-cli` after adding a new cobra subcommand produces a corresponding `docs/cli/spt_<command>.md` file.
- An intentional manual edit to a committed `docs/cli/*.md` file causes CI to fail.
- `spt --help` output does not surface `gen-docs`.
- Unit test in `docs_test.go` passes.

---

### Phase 3: dataset-bootstrap

**Ships in:** RFC-0001 Phase 3 (Agentic workflows and evals). Pairs with `dataset-upload` and the Langfuse eval harness.

**Reference design:** [DESIGN-0006 — dataset-bootstrap](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#dataset-bootstrap).
**Prior art:** `donaldgifford/server-price-tracker/tools/dataset-bootstrap`.

**Prerequisite:** [DESIGN-0002](../design/0002-domain-and-pipeline-type-system.md)'s Component/Score/Confidence types implemented; `internal/datastore/` interface exposed with read methods for listings and components.

#### Tasks

- [ ] Scaffold `tools/dataset-bootstrap/` with cobra root and a single `sample` subcommand.
- [ ] Define `StratificationConfig` struct per DESIGN-0006: `SinceDuration`, `PerKind`, `PerConfidenceBucket`, `TotalCap`, `Seed`.
- [ ] Define output `Sample` struct: `Listings`, `Scores` (map keyed by `ListingID`), `Components` (map keyed by `ListingID`).
- [ ] Implement `sampler.go` performing stratified selection:
  - [ ] Query candidate listings via `datastore.ListingsSince(ctx, sinceDuration)` (add the method if it doesn't yet exist; document the addition in the `internal/datastore/` IMPL).
  - [ ] Group by `(ComponentKind, ConfidenceBucket, ExtractorVer)`.
  - [ ] Sample `PerKind` from each kind, partition across `PerConfidenceBucket` map.
  - [ ] Enforce `TotalCap` after stratification.
  - [ ] Use `math/rand.New(rand.NewSource(Seed))` for deterministic reproducibility.
- [ ] Implement JSON writer with versioned header: `{"version": "v1", "generatedAt": "...", "config": {...}, "sample": {...}}`.
- [ ] Add flags: `--since=30d`, `--per-kind=10`, `--per-confidence-bucket='<0.5:5,0.5-0.8:10,0.8-1.0:10'`, `--total-cap=200`, `--seed=42`, `--out=regression-<UTC-timestamp>.json` (default uses `time.Now().UTC().Format("20060102T150405Z")` for second-resolution to avoid same-day collisions; operator can override with explicit `--out`).
- [ ] Add `tools/dataset-bootstrap/README.md`.
- [ ] Add `just tool dataset-bootstrap -- <args>` recipe (covered by Phase 1's generic `just tool` recipe).
- [ ] Unit test: mocked `Datastore` returns a known population; verify stratification proportions hold within tolerance.
- [ ] Unit test: same `Seed` produces byte-identical output across two runs.
- [ ] Unit test: JSON output round-trips through `encoding/json` cleanly.

#### Success Criteria

- Invoked against a seeded Postgres (per [DESIGN-0004](../design/0004-alert-and-reconciliation-pipeline.md#migration--rollout-plan)'s optional seeding path), produces a `regression-<date>.json` of the configured size.
- Stratification proportions across `(ComponentKind, ConfidenceBucket)` are within ±5% of requested counts.
- Identical `--seed` produces identical output across runs.
- All unit tests pass.

---

### Phase 4: dataset-upload

**Ships in:** RFC-0001 Phase 3. Pairs with `dataset-bootstrap` and the Langfuse eval harness.

**Reference design:** [DESIGN-0006 — dataset-upload](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#dataset-upload).
**Prior art:** `donaldgifford/server-price-tracker/tools/dataset-upload`.

**Prerequisite:** Langfuse credentials available in environment.

#### Tasks

- [ ] Scaffold `tools/dataset-upload/` with cobra root and a single `upload` subcommand.
- [ ] Implement `IDFor(content []byte) string`: `SHA256(content)`, take first 8 bytes, hex-encode (16 hex chars). Document collision math in a code comment.
- [ ] Implement a minimal internal Langfuse HTTP client in `tools/dataset-upload/langfuse.go`:
  - [ ] `Client` interface exposing only `UpsertDatasetItem(ctx, datasetID, itemID, content) error` (the entire surface this tool needs).
  - [ ] Concrete `httpClient` implementation using `net/http` against Langfuse's REST API — no third-party SDK dependency.
  - [ ] At minimum: basic auth via `LANGFUSE_PUBLIC_KEY` / `LANGFUSE_SECRET_KEY`, content-type JSON, retries on 5xx, surface 4xx errors directly.
  - [ ] Re-check if Langfuse has published an official Go SDK by the time this phase starts; if so, swap the internal client for the SDK behind the same `Client` interface.
- [ ] Implement `Uploader.Upsert(ctx, items []DatasetItem) error`:
  - [ ] For each item, compute `ID = IDFor(canonicalContent)`.
  - [ ] Call Langfuse upsert with that ID.
  - [ ] Log idempotent no-ops (same content → unchanged) at DEBUG.
- [ ] Add flags: `--dataset-id`, `--input=regression-<date>.json`, `--dry-run`.
- [ ] `--dry-run` mode: print planned `(action, ID, title)` for each item; perform zero HTTP calls.
- [ ] Auth: read Langfuse credentials from env (`LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`, `LANGFUSE_HOST`); fail fast at startup if missing.
- [ ] Add `tools/dataset-upload/README.md`.
- [ ] Unit test: `IDFor` determinism (same content → same ID; one-byte change → different ID).
- [ ] Unit test: `--dry-run` produces zero HTTP calls against a mocked client.
- [ ] Unit test: upsert with same input twice produces same client calls both times (idempotency).

#### Success Criteria

- Uploading the same `regression-<date>.json` twice produces the same Langfuse dataset state both times (verified by inspecting Langfuse via UI or API after each run).
- `--dry-run` mode reports planned actions without any network calls.
- Missing Langfuse credentials → process exits non-zero with a clear error message before any work.
- All unit tests pass.

---

### Phase 5: judge-bootstrap

**Ships in:** RFC-0001 Phase 3. Pairs with the LLM-as-judge layer ([ADR-0008](../adr/0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)).

**Reference design:** [DESIGN-0006 — judge-bootstrap](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#judge-bootstrap).
**Prior art:** `donaldgifford/server-price-tracker/tools/judge-bootstrap`.

**Prerequisite:** `internal/agent/judge/` package exists (or its expected location is decided); `Score`, `Judgment`, `Component` types in `internal/domain/`.

#### Tasks

- [ ] Scaffold `tools/judge-bootstrap/` with cobra root and two subcommands: `list` and `apply`.
- [ ] Define `Candidate` struct: `ListingID`, `ScoreID`, `ScoreValue`, `Components`, `Reasoning`, `Why`, `Accepted` (bool, default false), `Notes` (string, operator-editable; **required when `Accepted == true`** — validated by `apply`).
- [ ] Define `SurfaceStrategy` interface: `Name() string`, `Surface(ctx, ds, n) ([]Candidate, error)`.
- [ ] Implement `AmbiguousStrategy`: surfaces Scores whose `Value` is within ±5% of a percentile-band boundary in the most recent `MarketSignal`.
- [ ] Implement `LowConfidenceStrategy`: surfaces Scores whose Listings contain at least one `Component` with `Confidence < 0.5`.
- [ ] Implement `DisagreementStrategy`: surfaces Scores referenced by past `Judgment`s with `Verdict ∈ {Disagrees, Uncertain}`.
- [ ] Implement `HighStakesStrategy`: surfaces Scores in the top decile of `Percentile`.
- [ ] Register strategies in a `var strategies = map[string]SurfaceStrategy{...}` lookup.
- [ ] `list` mode:
  - [ ] Flags: `--since=30d`, `--candidates=50`, `--strategy=ambiguous`, `--out=candidates.json`.
  - [ ] Resolves strategy, calls `Surface`, writes JSON with `Accepted: false` defaults.
- [ ] `apply` mode:
  - [ ] Flags: `--input=accepted.json` (freeform path, operator-supplied), `--output=internal/agent/judge/examples.json`.
  - [ ] Filters to `Accepted: true`.
  - [ ] **Validates each accepted candidate has a non-empty `Notes` field**; exit non-zero with a per-candidate error listing which `ScoreID`s are missing justification.
  - [ ] Writes the few-shots file in the format the judge prompt consumes.
- [ ] Add `tools/judge-bootstrap/README.md` including the operator workflow: `list` → manual review/edit → `apply`.
- [ ] Unit test per strategy: synthetic dataset, expected candidates returned.
- [ ] Unit test: `apply` rejects an input with `Accepted: true` and missing `Notes` (exits non-zero, error message names the offending `ScoreID`s).
- [ ] Unit test: `apply` produces an `examples.json` that the judge prompt's loader can parse without modification.

#### Success Criteria

- `judge-bootstrap list --strategy=<each>` produces a candidates JSON for every registered strategy.
- After manual operator edits flipping `Accepted: true`, `judge-bootstrap apply` writes a valid `internal/agent/judge/examples.json`.
- The judge prompt's `examples.json` loader consumes the output without modification.
- All strategy unit tests pass.

---

### Phase 6: regression-runner

**Ships in:** RFC-0001 Phase 3. Pairs with the eval datasets and judge.

**Reference design:** [DESIGN-0006 — regression-runner](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#regression-runner).
**Prior art:** `donaldgifford/server-price-tracker/tools/regression-runner`.

**This tool MUST NOT be wired into CI.** The anti-CI rationale (fork PRs can exfil API keys) is preserved verbatim from prior art.

#### Tasks

- [ ] Scaffold `tools/regression-runner/` with cobra root and a single `run` subcommand.
- [ ] At the top of `main.go`, lift the anti-CI comment verbatim from prior art, including the reasoning: fork PRs could exfiltrate API keys; release gating happens via the maintainer's local invocation, not PR CI.
- [ ] Mirror the same notice in `tools/regression-runner/README.md`.
- [ ] Define `Backend` interface: `Name() string`, `Extract(ctx, listing) ([]Component, error)`.
- [ ] Implement `OllamaBackend` calling the local Ollama HTTP API; configurable endpoint and model.
- [ ] Implement `AnthropicBackend` using the Anthropic Go SDK; reads `ANTHROPIC_API_KEY`.
- [ ] Implement `OpenAIBackend` using the OpenAI Go SDK; reads `OPENAI_API_KEY`.
- [ ] Define `Result`, `BackendReport`, `Report` types per DESIGN-0006.
- [ ] Define `MatchOutcome` enum: `ExactMatch`, `PartialMatch`, `NoMatch`. Implement the matcher per DESIGN-0006 (`ExactMatch` = `(Kind, Model, Manufacturer, Quantity, Spec)`; `PartialMatch` = `(Kind, Model, Manufacturer)` only).
- [ ] Implement aggregation: per-Kind accuracy, overall accuracy, p50/p95 latency (stdlib sort + index for percentiles; no extra dependency needed).
- [ ] Implement stdout report formatter (table-shaped, human-readable).
- [ ] Implement JSON report formatter (full `Report` struct serialized; suitable for diffing between runs).
- [ ] Add `--langfuse` flag that additionally writes each `Result` to Langfuse as a Trace (reuses the `Client` interface from Phase 4 where possible).
- [ ] Commit a small in-tree baseline regression dataset under `tools/regression-runner/testdata/baseline/` (target ~50 listings — small enough for PR review, large enough to surface obvious regressions). This is the default `--dataset` value if no flag passed.
- [ ] Document that the full regression set lives in Langfuse and is fetched via `--dataset=langfuse://<dataset-id>` (URL scheme parsing in the dataset loader).
- [ ] Add flags: `--backend=ollama,anthropic,openai`, `--dataset=<path-or-langfuse-uri>` (default `tools/regression-runner/testdata/baseline/`), `--format=text|json` (default `text`), `--out=report.txt` (writes to the chosen format when set; stdout otherwise), `--langfuse` (logs traces).
- [ ] Add `tools/regression-runner/README.md` documenting both the in-tree baseline and the Langfuse-fetched workflow.
- [ ] Unit test: mocked `Backend` impls, verify `MatchOutcome` math.
- [ ] Unit test: report aggregation (per-Kind accuracy proportions, p50/p95 latency math).
- [ ] Unit test: `--format=json` produces a valid `Report` JSON that round-trips.
- [ ] **Audit:** grep the repo for `regression-runner` references in `.github/workflows/`. The expected count is zero.

#### Success Criteria

- Invoked against a regression dataset, produces a per-backend report with accuracy (overall and per-Kind) and latency percentiles.
- Re-run with a different prompt/model version surfaces measurable accuracy deltas in the report.
- The anti-CI comment is preserved at the top of `main.go` and in the README.
- No CI workflow references `regression-runner`.
- All unit tests pass.

---

### Phase 7: dashgen

**Ships in:** RFC-0001 Phase 5 (Packaging and Helm release). Dashboards and rules become Helm-chart assets.

**Reference design:** [DESIGN-0006 — dashgen](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#dashgen).
**Prior art:** `donaldgifford/server-price-tracker/tools/dashgen`.

**Prerequisite:** Helm chart skeleton exists at `charts/spt/` with `files/` subdirectory.

#### Tasks

- [ ] Scaffold `tools/dashgen/` under the main `go.mod` (no separate module — contrary to prior version).
- [ ] Implement a thin internal builder package in `tools/dashgen/internal/grafana/` that emits raw Grafana JSON via typed Go (~200 LOC target). Only the panel/row/variable types we actually use; no dependency on a third-party Grafana SDK. If the dashboard count grows past ~10 distinct dashboards, revisit and consider `grafana-foundation-sdk`.
- [ ] Hardcode the Prometheus datasource (no `$datasource` templating in v1). Datasource templating becomes a follow-up if a user requests multi-cluster support.
- [ ] Define `DashboardSpec` (`Name`, `File`, `Build func() any`) and `RuleGroupSpec` types.
- [ ] Implement dashboards (one Go builder per file):
  - [ ] `buildAPIOverview` — request rate, latency p50/p95/p99, error rate per `internal/app/api/` handler.
  - [ ] `buildWorkerPools` — using `spt_worker_pool_inflight`, `spt_worker_pool_queue_depth`, `spt_worker_task_duration_seconds` (from [DESIGN-0005](../design/0005-pipeline-orchestrator-and-worker-model.md)).
  - [ ] `buildEbayQuota` — using `spt_ebay_api_calls_total`, `spt_ebay_quota_remaining`, `spt_ebay_quota_exhausted` (from [DESIGN-0003](../design/0003-ebay-api-client.md)).
  - [ ] `buildAlertsDashboard` — using `spt_alerts_open_total`, `spt_alerts_stale_total`, `spt_reconcile_alerts_total`, `spt_reconcile_bulk_total` (from [DESIGN-0004](../design/0004-alert-and-reconciliation-pipeline.md)).
- [ ] Define recording rules (e.g., per-watch open alerts gauge if computed via aggregation).
- [ ] Define alert rules:
  - [ ] `spt_ebay_quota_exhausted == 1 for 30m` (from [DESIGN-0003](../design/0003-ebay-api-client.md#observability)).
  - [ ] `spt_alerts_stale_total > 0 for 30m` (from [DESIGN-0004](../design/0004-alert-and-reconciliation-pipeline.md#stale-alert-detection)).
  - [ ] `sum(spt_scheduler_role{role="leader"}) != 1 for 60s` (from [DESIGN-0005](../design/0005-pipeline-orchestrator-and-worker-model.md#metrics)).
  - [ ] `rate(spt_scheduler_sweep_recovered_total[5m]) > 0 for 15m` (from [DESIGN-0005](../design/0005-pipeline-orchestrator-and-worker-model.md#metrics)).
- [ ] Define `Mode` enum (`ModeWrite`, `ModeValidate`) and `-validate` flag.
- [ ] In `ModeValidate`: regenerate to memory, compare byte-for-byte against on-disk files at the target directory, print a unified diff for each mismatch, exit non-zero on any drift.
- [ ] In `ModeWrite`: overwrite files atomically (write to `.tmp` + `os.Rename`).
- [ ] Wire output to `charts/spt/files/dashboards/<name>.json` and `charts/spt/files/rules/<name>.yml`.
- [ ] Add `just dashboards-gen` recipe: `go run ./tools/dashgen ./charts/spt/files/`.
- [ ] Add `just validate-dashboards` recipe: `go run ./tools/dashgen -validate ./charts/spt/files/`.
- [ ] Add CI step in `.github/workflows/ci.yml` (after Phase 7 of this IMPL completes): run `just validate-dashboards`.
- [ ] Add `tools/dashgen/README.md` covering: how to add a new dashboard, how to add a new alert rule, how the `-validate` gate works.
- [ ] Unit test: `ModeValidate` correctly detects drift (write file, mutate one byte, validate exits non-zero).
- [ ] Unit test: each dashboard builder produces JSON that parses back as `map[string]any` cleanly.
- [ ] Confirm Helm chart references `charts/spt/files/dashboards/*.json` (as ConfigMaps or values) and `charts/spt/files/rules/*.yml` (as PrometheusRule resources).

#### Success Criteria

- `just dashboards-gen` produces every dashboard and rule file referenced by the Helm chart.
- `just validate-dashboards` exits 0 on a clean tree.
- A manual edit to a committed `charts/spt/files/dashboards/*.json` causes CI to fail.
- Helm chart packages cleanly with the generated files included.
- All unit tests pass.

---

## File Changes

| File / Directory | Action | Phase | Description |
|------------------|--------|-------|-------------|
| `tools/mock-server/` | Create | 1 | New package: cobra entry, server, routes, scenarios, fault injection, embed.FS fixtures, Dockerfile, README. |
| `internal/app/cli/docs.go` | Create | 2 | Hidden `gen-docs` cobra subcommand. |
| `docs/cli/*.md` | Create | 2 | Generated CLI documentation tree. |
| `tools/dataset-bootstrap/` | Create | 3 | New package: stratified sampler, JSON writer, cobra entry, README. |
| `internal/datastore/` | Modify | 3 | Add `ListingsSince(ctx, dur)` read method if not already present. |
| `tools/dataset-upload/` | Create | 4 | New package: Langfuse upsert with SHA256-truncated IDs, cobra entry, README. |
| `tools/judge-bootstrap/` | Create | 5 | New package: `list` + `apply` subcommands, four surface strategies, README. |
| `internal/agent/judge/examples.json` | Create | 5 | Output of `judge-bootstrap apply`; consumed by the judge prompt. |
| `tools/regression-runner/` | Create | 6 | New package: backend interface, three impls, report aggregation, anti-CI notice, README. |
| `tools/dashgen/` | Create | 7 | New package: dashboards, rules, `-validate` mode, README. |
| `charts/spt/files/dashboards/*.json` | Create | 7 | Generated Grafana dashboards. |
| `charts/spt/files/rules/*.yml` | Create | 7 | Generated Prometheus recording + alert rules. |
| `justfile` | Modify | 1, 2, 7 | Add `tool <name>` generic recipe (Phase 1); add `docs-cli` (Phase 2); add `dashboards-gen` + `validate-dashboards` (Phase 7). |
| `docker.just` | Modify | 1 | Add `tool-image mock-server` recipe. |
| `.github/workflows/ci.yml` | Modify | 1, 2, 7 | Add mock-server unit tests (Phase 1); docs drift check (Phase 2); dashboards validate (Phase 7). |

## Testing Plan

Per-tool unit and integration tests are enumerated in each phase's Tasks section. The conventions (mockery, testify/require, integration build tags, Compose stack) are established once in [IMPL-0001 Phase 7 (Testing infrastructure)](0001-foundation-go-layout-cli-config-observability-and-migrations.md#phase-7-testing-infrastructure) and inherited here; tools should not redefine the harness. Cross-cutting testing notes:

- [ ] Every new package gets unit tests with `>80%` coverage of exported functions.
- [ ] All table-driven tests use `testify/require` per [DESIGN-0001](../design/0001-go-application-layout-and-conventions.md) and [IMPL-0001 Phase 7](0001-foundation-go-layout-cli-config-observability-and-migrations.md#phase-7-testing-infrastructure).
- [ ] Mocks generated via `mockery` (config lives at the repo root per [IMPL-0001 Phase 7](0001-foundation-go-layout-cli-config-observability-and-migrations.md#phase-7-testing-infrastructure)) where interfaces cross package boundaries (e.g., `Backend`, `SurfaceStrategy`, `Client`).
- [ ] All filesystem-touching tests use `t.TempDir()`.
- [ ] `go test -race ./tools/...` clean as a precondition for merging any tool's PR.
- [ ] **CI coverage decisions per tool:**
  - mock-server, dataset-bootstrap, dataset-upload, judge-bootstrap, dashgen → unit tests run on every PR.
  - docgen (inline) → unit test runs on every PR; drift check runs on every PR.
  - regression-runner → unit tests run on every PR; the tool itself is **never invoked in CI**.

## Dependencies

**Foundation prerequisites (from [IMPL-0001](0001-foundation-go-layout-cli-config-observability-and-migrations.md)):**

- Phase 2 of this IMPL (docgen) requires [IMPL-0001 Phase 2 (Cobra root and role scaffolding)](0001-foundation-go-layout-cli-config-observability-and-migrations.md#phase-2-cobra-root-and-role-scaffolding).
- All testing tasks in this IMPL rely on [IMPL-0001 Phase 7 (Testing infrastructure)](0001-foundation-go-layout-cli-config-observability-and-migrations.md#phase-7-testing-infrastructure) — mockery setup, testify/require, integration build-tag pattern, Compose stack.
- The generic `just tool <name> -- <args>` recipe lands in this IMPL's Phase 1; per [IMPL-0001 — Dependencies](0001-foundation-go-layout-cli-config-observability-and-migrations.md#dependencies), Phase 1 of this IMPL (mock-server) can run in parallel with the foundation since it's a standalone HTTP server under `tools/`.

**Cross-phase prerequisites within this IMPL:**

- Phase 2 (docgen) requires the cobra root to exist in `cmd/spt/main.go` and at least two real subcommands (otherwise the generated tree is too thin to gate on).
- Phases 3–6 (agentic tools) require `internal/datastore/`, `internal/agent/`, and the relevant `internal/domain/` types from [DESIGN-0002](../design/0002-domain-and-pipeline-type-system.md) to exist.
- Phase 7 (dashgen) requires the Helm chart skeleton at `charts/spt/` and the metric names defined in DESIGN-0003 through DESIGN-0005 to be implemented in their respective packages.

**External dependencies:**

- cobra (`github.com/spf13/cobra`), already used by the main binary.
- Phase 4 (`dataset-upload`): no external SDK — internal HTTP client against Langfuse REST (decision #4 in [Resolved Decisions](#resolved-decisions)).
- Phase 6 (`regression-runner`): Anthropic Go SDK and OpenAI Go SDK.
- Phase 7 (`dashgen`): no external dashboard library — internal builder (decision #9 in [Resolved Decisions](#resolved-decisions)).

**Non-blocking but relevant:** [INV-0002](../investigation/0002-old-spt-tools-triage-port-priorities-for-v1.md) provides the triage and "what to preserve" notes per tool; consult it before starting each phase.

## Resolved Decisions

These were open questions during drafting; recommendations were accepted and the tasks above already reflect the outcomes. Captured here for traceability — if any of these need to be revisited mid-implementation, edit the relevant phase tasks and add a note here explaining why.

| # | Decision | Affects |
|---|----------|---------|
| 1 | **No static `quota-tight/` fixture.** Tests flip quota state at runtime via `POST /admin/quota`; the admin endpoint covers what a static fixture would, plus mid-test mutability. | Phase 1 |
| 2 | **Publish `mock-server` Docker image to GHCR** as `ghcr.io/donaldgifford/spt-mock-server:{sha,latest}`, gated on main-branch merges. Same release channel as the main `spt` image. Integration tests use the in-process server; the image is for `docker compose` ergonomics. | Phase 1 |
| 3 | **`dataset-bootstrap` default filename: second-resolution UTC timestamp** (`regression-20260525T143015Z.json`). Operator can always override via `--out`. | Phase 3 |
| 4 | **Langfuse: write a minimal internal HTTP client** wrapped behind a `Client` interface (re-check for an official Langfuse Go SDK when Phase 4 starts; swap behind the same interface if one exists). Avoids dependency drift on still-maturing community SDKs; our surface area is one endpoint. | Phase 4 |
| 5 | **`judge-bootstrap` requires non-empty `Notes` on `Accepted: true`.** Forces the operator to justify each few-shot; `apply` exits non-zero listing the offending `ScoreID`s if any are missing. | Phase 5 |
| 6 | **Accepted-file path is freeform via `--input`.** No standardized location; the operator workflow is iterative. | Phase 5 |
| 7 | **`regression-runner` supports `--format=text|json`**, default `text`. JSON output enables run-over-run diffing. | Phase 6 |
| 8 | **Regression dataset split: in-tree baseline + Langfuse for full.** `tools/regression-runner/testdata/baseline/` holds ~50 listings (committed, PR-reviewable) and is the default `--dataset`. The full set lives in Langfuse and is opted into via `--dataset=langfuse://<dataset-id>`. | Phase 6 |
| 9 | **`dashgen` uses a thin internal Grafana builder** (~200 LOC under `tools/dashgen/internal/grafana/`) emitting raw JSON via typed Go. Avoids a dependency on `grafana-foundation-sdk` or similar still-maturing libraries. Revisit if dashboard count grows past ~10. | Phase 7 |
| 10 | **Hardcode the Prometheus datasource in v1 dashboards.** No `$datasource` templating yet; add it as a follow-up if multi-cluster users request it. | Phase 7 |
| 11 | **The generic `just tool <name> -- <args>` recipe lands in Phase 1** alongside mock-server. Used by every subsequent tool. | All phases |

## References

- [DESIGN-0006 — Developer tooling: porting and refactoring from old spt](../design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md) — the design this implements
- [INV-0002 — Old-spt tools triage: port priorities for v1](../investigation/0002-old-spt-tools-triage-port-priorities-for-v1.md) — the triage that established the phasing
- [IMPL-0001 — Foundation: Go layout, CLI, config, observability, and migrations](0001-foundation-go-layout-cli-config-observability-and-migrations.md) — the foundation this IMPL depends on (Phase 2 cobra for docgen; Phase 7 testing conventions for every tool)
- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md) — defines RFC-0001 phases 1, 3, and 5 this IMPL cross-references
- [DESIGN-0001 — Go application layout and conventions](../design/0001-go-application-layout-and-conventions.md)
- [DESIGN-0002 — Domain and pipeline type system](../design/0002-domain-and-pipeline-type-system.md)
- [DESIGN-0003 — eBay API client](../design/0003-ebay-api-client.md) (Phase 1 consumer of mock-server)
- [DESIGN-0004 — Alert and reconciliation pipeline](../design/0004-alert-and-reconciliation-pipeline.md) (Phase 1 consumer of mock-server)
- [DESIGN-0005 — Pipeline orchestrator and worker model](../design/0005-pipeline-orchestrator-and-worker-model.md) (Phase 7 dashgen consumes metric names from here)
- [ADR-0008 — Use OTel + ClickHouse + Langfuse for agent observability and evals](../adr/0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)
- Prior-version tools directory: <https://github.com/donaldgifford/server-price-tracker/tree/main/tools>
- Cobra docs generator: <https://pkg.go.dev/github.com/spf13/cobra/doc>
