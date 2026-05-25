---
id: DESIGN-0006
title: "Developer tooling — porting and refactoring from old spt"
status: Draft
author: Donald Gifford
created: 2026-05-25
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0006: Developer tooling — porting and refactoring from old spt

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-25

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [Repo layout for tools](#repo-layout-for-tools)
  - [mock-server](#mock-server)
  - [docgen](#docgen)
  - [dataset-bootstrap](#dataset-bootstrap)
  - [dataset-upload](#dataset-upload)
  - [judge-bootstrap](#judge-bootstrap)
  - [regression-runner](#regression-runner)
  - [dashgen](#dashgen)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Overview

Specifies the new shapes of the seven developer-facing tools that prior-version spt shipped under `tools/`. Each tool is treated as a problem the old version already solved well enough to use as **prior art** — we don't re-discover the right shape for an eBay mock or a Langfuse dataset uploader; we lift what the old code got right, fix what it got wrong, and update for the new codebase's types and conventions. The phase-ordering follows [INV-0002](../investigation/0002-old-spt-tools-triage-port-priorities-for-v1.md): mock-server lands in Phase 1, docgen recreates inline when cobra arrives, the agentic tools cluster in Phase 3, dashgen lifts wholesale in Phase 5.

## Goals and Non-Goals

### Goals

- One coherent reference for every dev tool: what it is, why it exists, what we lift from prior art, what changes, and which v1 phase ships it.
- Concrete enough new shapes (package layout, key types/interfaces, fixtures organization) that the IMPL doc per tool is just "implement what's spec'd here."
- Preserve the non-obvious decisions from the prior version (deterministic IDs, anti-CI rationale, `-validate` no-write mode, list-then-apply UX) so they don't get rediscovered the hard way.
- A consistent repo layout for `tools/` so future tools land in the obvious place.

### Non-Goals

- The user-facing `spt` CLI — that's `cmd/spt/main.go` and the cobra subcommands defined in [DESIGN-0001](0001-go-application-layout-and-conventions.md), not `tools/`.
- Phase timelines and ownership — those belong in IMPL docs per tool and the master PLAN doc, not here.
- Eval methodology, prompt content, or scoring algorithm — those are the responsibility of the respective DESIGN docs (extraction, scoring, judge) that the agentic tools support, not this doc.
- Replacing tools we don't need anymore — none of the seven are categorically obsolete; the question is "when," not "whether."

## Background

The prior-version spt repo (`donaldgifford/server-price-tracker`) shipped seven tools under `tools/`. [INV-0002](../investigation/0002-old-spt-tools-triage-port-priorities-for-v1.md) triaged them and concluded: port mock-server now (Phase 1 dependency), recreate docgen inline when cobra lands, port the four agentic tools at Phase 3, lift dashgen wholesale at Phase 5. This document is the design follow-up — translating that triage into concrete tool designs.

Treating the old code as **prior art rather than legacy** changes the framing. We're not migrating production tooling; we're using the previous solution as a high-quality reference design. That means:

- We can drop incidental complexity (the bind-mount-fixture-path pattern, ad-hoc shell wrappers) without ceremony.
- We can change shape where the new architecture warrants (mock-server gets `embed.FS` and a scenario engine; dataset-bootstrap gets schema-aware stratification matching [DESIGN-0002](0002-domain-and-pipeline-type-system.md)'s Component model).
- We must explicitly preserve the non-obvious wins (per [INV-0002](../investigation/0002-old-spt-tools-triage-port-priorities-for-v1.md)'s "what to preserve" notes) so they aren't lost in the rewrite.

## Detailed Design

### Repo layout for tools

```
tools/
  mock-server/
    main.go                  # cobra-based CLI entry
    server.go                # http.Server wiring
    routes.go                # endpoint handlers
    fixtures/                # embed.FS root for fixtures
      default/
        search.json
        items/
      sold-listings/
      ended-no-sale/
    scenarios.go             # scenario engine: per-item, per-scenario directories
    faultinject.go           # latency + 5xx injection
    Dockerfile               # multi-stage; lifted from prior version
    README.md
  docgen/                    # only present after cobra lands; ~30 LOC main
    main.go
    README.md
  dataset-bootstrap/         # Phase 3
    main.go
    sampler.go               # stratified sampling
    README.md
  dataset-upload/            # Phase 3
    main.go
    langfuse.go              # SHA256-truncated-ID upsert
    README.md
  judge-bootstrap/           # Phase 3
    main.go
    list.go                  # `list` mode
    apply.go                 # `apply` mode
    README.md
  regression-runner/         # Phase 3
    main.go
    backends.go              # Ollama / Anthropic / OpenAI adapters
    report.go                # accuracy + latency reporting
    README.md
  dashgen/                   # Phase 5
    main.go
    dashboards.go            # Grafana dashboards
    rules.go                 # Prometheus recording + alert rules
    README.md
```

**Conventions:**

- **One Go module for the whole repo.** All tools live in `github.com/donaldgifford/spt/tools/<name>` under the main `go.mod`. The prior version had per-tool `go.mod`s for `dashgen`; that's bookkeeping overhead without payoff at our scale. We split only if a tool's dependency tree is genuinely incompatible with the main module (none of the seven currently are).
- **Cobra for every tool entry point that takes more than two flags.** Same library as the main `spt` binary; consistency over creativity. Trivial tools (docgen) can use stdlib `flag` or no flag parsing at all.
- **Tool internal code stays in `tools/<name>/`.** No `tools/internal/` or `tools/shared/` package for v1. If two tools need the same helper, we promote it to `internal/` proper.
- **No tool imports concrete infrastructure from `internal/app/`** — tools are dev utilities; they may import `internal/domain/`, `internal/datastore/` (for tools that touch the DB like `dataset-bootstrap`), or `internal/ebay/types.go`, but they don't reach into the role wiring.
- **Each tool has a README** with: what it does, when to use it, example invocation, any prerequisites (env vars, fixtures, DB connection).
- **Build via `just`:** add a generic `just tool <name> -- <args>` recipe that does `go run ./tools/<name>/ -- "$@"`. Tools with Docker images get a `just -f docker.just tool-image <name>` recipe.

### mock-server

**Phase 1. Port and extend.** This is the only tool whose absence blocks Phase 1 work: the eBay client unit tests ([DESIGN-0003](0003-ebay-api-client.md)) and the reconciliation integration tests ([DESIGN-0004](0004-alert-and-reconciliation-pipeline.md)) both depend on it.

**Prior art:** `donaldgifford/server-price-tracker/tools/mock-server`. The old version implemented OAuth token + Browse search against a bind-mounted single fixture file. It got the multi-stage Docker shape right (`golang:1.26-alpine` → `alpine:3.21`), the multi-word query matching right (`containsAllWords` filter + lowercased title cache), and the OAuth response shape right. It missed: `GetItem`, Analytics, scriptable scenarios, fault injection, and embedded fixtures.

**New shape:**

```go
// tools/mock-server/server.go
package main

import (
    "embed"
    "net/http"
)

//go:embed fixtures
var fixturesFS embed.FS

type Server struct {
    addr        string
    scenarios   *ScenarioRegistry          // active scenario by name
    activeScenario string                  // mutable; set via /admin/scenario
    fault       *FaultInjector             // per-request latency + 5xx
    quota       *QuotaState                // mutable; set via /admin/quota
    logger      *slog.Logger
}

func (s *Server) Routes() http.Handler {
    mux := http.NewServeMux()

    // Real-eBay-shape endpoints.
    mux.HandleFunc("POST /identity/v1/oauth2/token", s.handleOAuth)
    mux.HandleFunc("GET /buy/browse/v1/item_summary/search", s.handleSearch)
    mux.HandleFunc("GET /buy/browse/v1/item/{item_id}", s.handleGetItem)
    mux.HandleFunc("GET /developer/analytics/v1_beta/rate_limit/", s.handleAnalytics)

    // Admin endpoints (mock-only, prefixed with /admin).
    mux.HandleFunc("POST /admin/scenario", s.handleSetScenario)   // body: {"name": "sold-listings"}
    mux.HandleFunc("POST /admin/quota", s.handleSetQuota)         // body: {"count": 4500, "limit": 5000}
    mux.HandleFunc("POST /admin/fault", s.handleSetFault)         // body: {"endpoint": "/buy/browse/v1/item/.*", "latency_ms": 1000, "fail_rate": 0.1}

    return s.fault.Middleware(s.quota.Middleware(mux))
}
```

**Scenarios.** A scenario is a named directory under `fixtures/`:

```
fixtures/
  default/                          # the baseline scenario, used when nothing else is set
    search.json                     # search response template
    items/
      v1|151234567890.json          # full Item response, filename is URL-encoded itemID
      v1|151234567891.json
  sold-listings/                    # all items return AvailabilityStatus=OUT_OF_STOCK
    items/
      v1|151234567890.json
  ended-no-sale/
    items/
      v1|151234567890.json
  quota-tight/
    quota.json                      # initial state override
    items/
```

A scenario directory may override any subset of fixtures from `default/`. Resolution walks `<active>` first, falls back to `default/`. This keeps scenarios small — a "sold-listings" scenario only ships the items whose responses differ.

```go
// tools/mock-server/scenarios.go
type ScenarioRegistry struct {
    scenarios map[string]*Scenario
    defaultS  *Scenario
}

type Scenario struct {
    Name   string
    Items  map[string]json.RawMessage   // ebayItemID → full Item JSON
    Search json.RawMessage              // search response template
    Quota  *QuotaSnapshot               // optional initial state
}

func (r *ScenarioRegistry) Resolve(active, itemID string) (json.RawMessage, bool) {
    if s, ok := r.scenarios[active]; ok {
        if item, ok := s.Items[itemID]; ok {
            return item, true
        }
    }
    if item, ok := r.defaultS.Items[itemID]; ok {
        return item, true
    }
    return nil, false
}
```

**Fault injection.** Per-pattern latency and failure-rate, configurable at runtime via `POST /admin/fault`:

```go
type FaultRule struct {
    EndpointPattern *regexp.Regexp   // matches request URL path
    LatencyMs       int              // sleep before responding
    FailRate        float64          // 0.0-1.0; HTTP 503 with eBay-shaped error body
}

type FaultInjector struct {
    mu    sync.RWMutex
    rules []FaultRule
}
```

Why runtime-configurable rather than fixture-only: integration tests want to flip "now all requests to GetItem time out" mid-test to exercise the reconciler's stale-detection path. A static fixture can't model that.

**Quota state.** Mutable counter served via `/developer/analytics/v1_beta/rate_limit/`:

```go
type QuotaState struct {
    mu          sync.Mutex
    count       int64
    limit       int64
    resetAt     time.Time            // next 24h reset
    timeWindow  string               // mimics eBay's "DAY" string
    autoIncr    bool                 // if true, every successful eBay call increments count; if false, only /admin/quota mutates
}
```

The `Middleware` wraps the eBay-shaped routes, increments `count` on every successful response, and stamps `X-EBAY-API-Call-Limit`, `X-EBAY-API-Calls-Made`, and `X-EBAY-API-Calls-Remaining` headers — so the rate-limiter under test sees realistic header values.

**What's lifted verbatim from prior art:**

- Multi-stage Dockerfile shape (`golang:1.26-alpine` → `alpine:3.21`), updated to current versions.
- `containsAllWords(title, queryTerms)` filter for multi-word query matching against fixture titles.
- Lowercased-title cache pattern (compute once at fixture load, reuse across requests).
- OAuth token response shape (`access_token`, `expires_in`, `token_type=Bearer`, single static token for the mock's lifetime).

**What's new vs. prior art:**

- `GET /buy/browse/v1/item/{item_id}` (new endpoint).
- `GET /developer/analytics/v1_beta/rate_limit/` (new endpoint).
- `embed.FS` fixtures, multi-scenario layout (replaces single bind-mounted file).
- Scenario engine with default-fallback resolution.
- Fault injection (runtime-configurable via `/admin/fault`).
- Rate-limit response headers on every eBay-shaped response.
- Cobra entry point with `--port`, `--scenario`, `--log-format`, etc.

**Invocation patterns:**

```bash
# Default usage; serves on :8080 with the 'default' scenario.
just tool mock-server -- serve

# Specific scenario.
just tool mock-server -- serve --scenario=sold-listings --port=8081

# Inside Compose for integration tests.
docker run --rm -p 8080:8080 ghcr.io/donaldgifford/spt-mock-server:latest serve --scenario=default

# Mid-test: flip scenario.
curl -X POST http://localhost:8080/admin/scenario -d '{"name": "ended-no-sale"}'
```

**Acceptance:** the integration tests in [DESIGN-0004](0004-alert-and-reconciliation-pipeline.md) (`alert opens → reconcile → sold → alert closes`, `bulk sweep deferral when quota tight`, `Stale set on simulated GetItem 5xx`) pass when pointed at this mock-server in a Compose stack.

### docgen

**Phase 1 (after cobra lands). Recreate inline — not a separate tool worth carrying.**

**Prior art:** ~30 lines using `github.com/spf13/cobra/doc.GenMarkdownTree(rootCmd, "docs/cli")`.

**New shape:** **not** a separate tool. Add it as a hidden cobra subcommand under the main binary:

```go
// internal/app/cli/docs.go
var docsCmd = &cobra.Command{
    Use:    "gen-docs <output-dir>",
    Hidden: true,                            // not surfaced in `spt --help`
    Short:  "Regenerate the docs/cli/ markdown tree from the live command set",
    Args:   cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return doc.GenMarkdownTree(rootCmd, args[0])
    },
}
```

Invocation: `spt gen-docs docs/cli/`. Wired into a `just docs-cli` recipe and into the CI doc-drift check (CI runs `just docs-cli && git diff --exit-code docs/cli/` to fail on stale docs).

**Why inline, not a tool:** the implementation is one function call from an existing dependency we already use. A `tools/docgen/main.go` would duplicate cobra import paths and add a separate build target for no gain. The "tools port" framing breaks down for things that are this small; better to absorb them into the main binary where they have access to the live command tree.

**Acceptance:** running `spt gen-docs docs/cli/` after adding a new cobra subcommand produces an updated docs tree; CI catches drift.

### dataset-bootstrap

**Phase 3. Port at Phase 3 (lift the idea, rewrite for our schema).**

**Prior art:** `tools/dataset-bootstrap` in the prior version. CLI that pulls a stratified sample of recent listings from Postgres into a regression JSON for human audit. The stratification dimensions were tied to the old Component model.

**New shape:**

```go
// tools/dataset-bootstrap/main.go
// Pulls a stratified sample of recent Listings (and their Components, Scores) into
// a regression JSON suitable for human audit and dataset-upload.

type Sample struct {
    Listings []domain.Listing
    Scores   map[domain.ListingID]domain.Score
    Components map[domain.ListingID][]domain.Component
}

type StratificationConfig struct {
    SinceDuration   time.Duration            // default 30 days
    PerKind         int                      // sample N per ComponentKind
    PerConfidenceBucket map[string]int       // sample N per ("<0.5", "0.5-0.8", "0.8-1.0") bucket
    TotalCap        int                      // hard cap regardless of strata
    Seed            int64                    // for reproducibility
}
```

**Stratification dimensions for the new schema:** by `ComponentKind` (CPU, RAM, Drive, ...), by `Confidence` bucket (`<0.5` flagged-for-review band, `0.5-0.8` mid-confidence, `0.8-1.0` high-confidence), by `ExtractorVer` (so version transitions are covered). The buckets come from [DESIGN-0002](0002-domain-and-pipeline-type-system.md)'s Confidence contract.

**Output:** a single JSON file (`regression-<date>.json`) with the sample, ready to feed into `dataset-upload` or for manual audit. Format is internal; pinned at v1 of the tool, versioned in the file's header field so future schema changes don't silently break old datasets.

**What's preserved from prior art:**

- The stratified-sampling pattern itself — random sampling of recent listings is not a substitute for stratified sampling across known dimensions of interest. Don't lose this.
- The "human-auditable JSON for spot-checking" output format — operators want to eyeball samples before pushing to Langfuse.

**What's new vs. prior art:**

- Stratification dimensions match [DESIGN-0002](0002-domain-and-pipeline-type-system.md)'s Component / Score / Confidence model.
- Uses the same `internal/datastore/` interface as the rest of the codebase (no raw SQL strings duplicated from the old version).
- Cobra entry point.

**Acceptance:** invoked against a Postgres seeded with old-spt data (per [DESIGN-0004](0004-alert-and-reconciliation-pipeline.md)'s optional seeding path), produces a stratified sample JSON of the configured size.

### dataset-upload

**Phase 3. Port at Phase 3 (lift the idea + the SHA256-truncated-ID trick).**

**Prior art:** `tools/dataset-upload`. Small CLI; uploads regression JSON to Langfuse as a DatasetItem set. The non-obvious trick: deterministic SHA256-truncated IDs make re-uploads idempotent — the second upload of the same content is a no-op, not a duplicate.

**New shape:**

```go
// tools/dataset-upload/langfuse.go

// IDFor produces a deterministic, Langfuse-friendly DatasetItem ID from the
// item's canonical content. SHA256 → first 16 hex chars → fits Langfuse ID
// constraints with negligible collision risk for our scale.
func IDFor(content []byte) string {
    sum := sha256.Sum256(content)
    return hex.EncodeToString(sum[:8])  // 16 hex chars
}

type Uploader struct {
    client     *langfuse.Client
    datasetID  string                  // existing Langfuse dataset
    dryRun     bool                    // print actions, don't call API
    log        *slog.Logger
}

func (u *Uploader) Upsert(ctx context.Context, items []DatasetItem) error
```

The deterministic-ID trick is the entire insight: Langfuse's API is idempotent on ID, so SHA256-truncated content hashes give us "if the content is the same, do nothing; if it changed, update in place." This means re-running `dataset-upload` after a `dataset-bootstrap` re-sample is safe by default — no duplicates accumulating, no need for cleanup tooling.

**What's preserved from prior art:**

- The SHA256-truncated-ID pattern (16 hex chars; collision risk vanishingly small at expected scale).
- The dry-run mode.

**What's new vs. prior art:**

- Whatever the current Langfuse Go SDK looks like (likely different from the old version's bindings; adapter is straightforward).
- Cobra entry point.

**Acceptance:** uploading the same regression JSON twice produces the same Langfuse dataset state both times (no duplicates, no errors).

### judge-bootstrap

**Phase 3. Port at Phase 3.**

**Prior art:** `tools/judge-bootstrap`. Two-mode CLI:

- **`list`** — surface candidate few-shot examples (existing Score/Listing pairs with operator-relevant signals: high-confidence-but-disagreement-prone, low-confidence-but-likely-correct, etc.). Operator reviews and accepts the ones to include.
- **`apply`** — write the accepted few-shots to `pkg/judge/examples.json` for the judge prompt to consume.

The workflow ("operator audits existing classifications rather than labeling from scratch") is the keeper. Cold-labeling is slow and prone to operator drift; classification-audit is fast and grounds the few-shots in real model behavior.

**New shape:**

```go
// tools/judge-bootstrap/main.go
// Two subcommands:
//   spt-judge-bootstrap list   --since=30d --candidates=50 --strategy=ambiguous
//   spt-judge-bootstrap apply  --input=accepted.json --output=internal/agent/judge/examples.json

type Candidate struct {
    ListingID   domain.ListingID
    ScoreID     domain.ScoreID
    ScoreValue  apd.Decimal
    Components  []domain.Component
    Reasoning   string                    // from Score.Reasoning
    Why         string                    // why this candidate was surfaced (strategy explanation)
}

type SurfaceStrategy interface {
    Name() string
    Surface(ctx context.Context, ds datastore.Datastore, n int) ([]Candidate, error)
}

// Strategies:
//   AmbiguousStrategy   - Scores near percentile-band boundaries (most likely to flip on baseline drift)
//   LowConfidenceStrategy - Components with confidence < 0.5
//   DisagreementStrategy - past Judgments where verdict was Disagrees or Uncertain
//   HighStakesStrategy  - top-percentile scores (these are the alerting ones; getting them right matters most)
```

The `Why` field is critical — it's how the operator knows *why* a candidate was surfaced, which informs whether it's a good few-shot for the judge prompt.

**Output of `list`:** a JSON file of candidates with an `accepted: false` flag per item. Operator edits the file (flips `accepted: true` on the ones to keep, optionally adds a `notes` field), then runs `apply` to materialize the final few-shots file.

**What's preserved from prior art:**

- The list-then-apply UX (don't auto-commit; let the operator filter).
- The strategy-pluggable surfacing — different judge prompts will want different candidate distributions.
- Output path: `internal/agent/judge/examples.json` (was `pkg/judge/examples.json` in the old version; updated for new layout per [DESIGN-0001](0001-go-application-layout-and-conventions.md)).

**What's new vs. prior art:**

- Strategy interface formalized (was implicit in the old version).
- Operates over the new [DESIGN-0002](0002-domain-and-pipeline-type-system.md) types (Score, Component, Confidence, Judgment).
- Reads from `internal/datastore/` rather than raw SQL.

**Acceptance:** `list` produces a candidates JSON; manual operator edits; `apply` writes the final few-shots file; the judge prompt consumes it without modification.

### regression-runner

**Phase 3. Port at Phase 3.**

**Prior art:** `tools/regression-runner`. Runs the golden classifications dataset through Ollama / Anthropic / OpenAI backends; reports per-component accuracy + p50/p95 latency; optional Langfuse logging. **Explicitly not in CI** — the source had a documented rationale: fork PRs could exfil API keys, so this is local-only.

**New shape:**

```go
// tools/regression-runner/backends.go
type Backend interface {
    Name() string
    Extract(ctx context.Context, listing domain.Listing) ([]domain.Component, error)
}

type OllamaBackend struct {
    endpoint string
    model    string
}
type AnthropicBackend struct {
    apiKey string
    model  string
}
type OpenAIBackend struct {
    apiKey string
    model  string
}

// tools/regression-runner/report.go
type Result struct {
    Backend     string
    Listing     domain.Listing
    Got         []domain.Component
    Want        []domain.Component
    Match       MatchOutcome             // ExactMatch | PartialMatch | NoMatch
    LatencyMs   int64
}

type Report struct {
    PerBackend map[string]*BackendReport
}

type BackendReport struct {
    Name              string
    TotalListings     int
    AccuracyByKind    map[domain.ComponentKind]float64
    OverallAccuracy   float64
    LatencyP50Ms      int64
    LatencyP95Ms      int64
    Failures          []FailureSummary
}
```

**Match semantics.** `ExactMatch` requires same `(Kind, Model, Manufacturer, Quantity, Spec)`; `PartialMatch` is `(Kind, Model, Manufacturer)` only; `NoMatch` is anything less. We track all three because partial-match rates indicate prompt drift (model gets the SKU right but botches the spec extraction).

**What's preserved from prior art:**

- The pluggable-backend pattern — local Ollama for fast iteration, paid models for ground-truth comparison.
- **The explicit anti-CI comment.** Lift it verbatim with its reasoning intact: "This tool intentionally does not run in CI. Fork PRs could exfiltrate API keys via the test invocation; we run regression checks locally and gate releases on the maintainer's local run, not on PR CI." Put it at the top of `main.go` and in the README, not just in commit history.
- Optional Langfuse logging (results go to a Langfuse dataset as Traces for cross-run comparison).

**What's new vs. prior art:**

- `Backend` interface unified for new providers (Ollama, Anthropic, OpenAI as starters; the interface admits more).
- Output format aligns with `dataset-bootstrap`'s sample shape — the same JSON can be the input to `regression-runner` and the output of a fresh `dataset-bootstrap` sample.
- Match outcomes are explicit (`ExactMatch | PartialMatch | NoMatch`), not just a boolean.

**Acceptance:** invoked against a known regression dataset, produces per-backend accuracy + latency reports; rerunning with a different prompt version surfaces measurable accuracy deltas.

### dashgen

**Phase 5. Lift wholesale.**

**Prior art:** `tools/dashgen` in the prior version. Self-contained Go module (had its own `go.mod`); generates Grafana dashboards + Prometheus recording/alert rules from typed Go code; supports `-validate` mode (no-write; exits non-zero on drift). The `-validate` mode is the gem — it gives us a CI gate that committed dashboards haven't drifted from the source-of-truth Go code without diffing committed JSON.

**New shape:**

```go
// tools/dashgen/main.go
//
// Generates Grafana dashboards and Prometheus rules from typed Go code into
// charts/spt/files/dashboards/ and charts/spt/files/rules/. The Helm chart
// references these files; the generator is the source of truth.

type Mode int
const (
    ModeWrite    Mode = iota   // default: write files, overwriting existing
    ModeValidate                // no write; compare against on-disk and exit non-zero on drift
)

// Top-level dashboards: API latency, worker queue depths, eBay quota, alert pipeline health.
var dashboards = []DashboardSpec{
    {Name: "api-overview", File: "api-overview.json", Build: buildAPIOverview},
    {Name: "worker-pools", File: "worker-pools.json", Build: buildWorkerPools},
    {Name: "ebay-quota",   File: "ebay-quota.json",   Build: buildEbayQuota},
    {Name: "alerts",       File: "alerts.json",       Build: buildAlertsDashboard},
}

// Recording + alert rules; e.g. the spt_ebay_quota_exhausted alert from
// DESIGN-0003, the spt_alerts_stale_total alert from DESIGN-0004, the
// spt_scheduler_role split-brain alert from DESIGN-0005.
var ruleGroups = []RuleGroupSpec{...}
```

**`-validate` mode in CI:** add a `just validate-dashboards` recipe that runs `go run ./tools/dashgen -validate ./charts/spt/files/`. CI fails if the committed files differ from the regenerated output. Same gate as the docgen drift check, same justification: source-of-truth in code, committed files for human review, CI proves they match.

**Same go.mod as everything else.** The prior version had `dashgen` as a separate module to isolate the grafonnet-like dependencies. With Grafana dashboards now generated using lighter-weight libraries (or no external lib at all — the JSON shape is small enough to construct directly), the separate-module overhead isn't justified. Drop it.

**What's preserved from prior art:**

- The code-as-source-of-truth approach (typed Go over hand-edited JSON).
- The `-validate` mode and the CI gate it enables.
- Per-dashboard file output (one JSON per dashboard, easy to review in PRs).

**What's new vs. prior art:**

- Dashboards target the new metric names from [DESIGN-0003](0003-ebay-api-client.md), [DESIGN-0004](0004-alert-and-reconciliation-pipeline.md), and [DESIGN-0005](0005-pipeline-orchestrator-and-worker-model.md) (not the old `metrics.EbayDailyLimitHits.Inc()` naming).
- Output goes to `charts/spt/files/` for direct Helm-chart consumption (the old version wrote to a sibling repo).
- Same go.mod as the main module.

**Acceptance:** running `go run ./tools/dashgen ./charts/spt/files/` produces dashboards and rules referenced by the Helm chart; `go run ./tools/dashgen -validate ./charts/spt/files/` exits 0 on a clean tree and non-zero after a manual edit of a committed dashboard.

## API / Interface Changes

The tools introduce no public-API surface changes. They consume:

- `internal/domain/` types (read-only).
- `internal/datastore/` interface for tools that touch the DB (`dataset-bootstrap`, `judge-bootstrap`).
- `internal/ebay/types.go` for fixture shapes (`mock-server`).

Tools that produce on-disk output (`dashgen`, `docgen`, `dataset-bootstrap`) write to well-known paths:

| Tool | Output path | Consumed by |
|------|-------------|-------------|
| `docgen` (inline as `spt gen-docs`) | `docs/cli/*.md` | docs site; CI drift check |
| `dataset-bootstrap` | `regression-<date>.json` (operator-supplied path) | `dataset-upload`, `regression-runner` |
| `dataset-upload` | (Langfuse remote) | Langfuse dataset; `regression-runner --langfuse` |
| `judge-bootstrap apply` | `internal/agent/judge/examples.json` | judge prompt at compile/runtime |
| `regression-runner` | stdout report + optional Langfuse traces | operator |
| `dashgen` | `charts/spt/files/dashboards/*.json`, `charts/spt/files/rules/*.yml` | Helm chart; CI drift check |

## Data Model

Tools own no DB tables. Two tools read from `internal/datastore/`:

- `dataset-bootstrap` reads `listings`, `scores`, `components` (stratified sample).
- `judge-bootstrap` reads `scores`, `components`, `judgments` (candidate surfacing).

Neither writes. The mock-server owns its `embed.FS` fixtures (no DB). The Langfuse-touching tools (`dataset-upload`, `regression-runner --langfuse`) own no local persistence — Langfuse is the store.

## Testing Strategy

**Per-tool unit tests** (standard package layout, `_test.go` next to source). Focus areas per tool:

- **mock-server:** scenario resolution (default fallback), fault injection (rule matching), quota state (concurrent-safe counter, header stamping). Plus a single end-to-end smoke test that runs the server, points the actual `internal/ebay/` client at it, and verifies a search + getItem round-trip.
- **docgen** (inline): one test that builds the cobra tree, generates docs to a temp dir, and asserts a known command's file exists.
- **dataset-bootstrap:** mocked Datastore returning a known population; verify stratification proportions hold against the requested config.
- **dataset-upload:** SHA256 ID determinism (same content → same ID; different content → different ID); dry-run produces no API calls.
- **judge-bootstrap:** strategy correctness (ambiguous, low-confidence, disagreement, high-stakes return the expected candidates from a synthetic dataset).
- **regression-runner:** mocked Backend impls; verify report aggregation (per-Kind accuracy, p50/p95 latency math).
- **dashgen:** `-validate` correctly detects drift introduced by a synthetic edit.

**Integration tests** (only mock-server has them): the Compose-stack tests in [DESIGN-0004](0004-alert-and-reconciliation-pipeline.md) and [DESIGN-0003](0003-ebay-api-client.md) point at mock-server.

**CI gating:**

- `tools/mock-server` builds and unit-tests run on every PR (it's a Phase-1 dependency).
- `tools/docgen` (inline `spt gen-docs`) runs in a `just docs-cli && git diff --exit-code docs/cli/` step.
- `tools/dashgen -validate` runs in a `just validate-dashboards` step once dashgen exists (Phase 5).
- `tools/regression-runner` **does not** run in CI per the preserved anti-CI rationale.
- The other agentic tools (`dataset-bootstrap`, `dataset-upload`, `judge-bootstrap`) are operator-invoked; their unit tests run in CI but the tools themselves aren't exercised end-to-end there.

## Migration / Rollout Plan

Phase-aligned per [INV-0002](../investigation/0002-old-spt-tools-triage-port-priorities-for-v1.md):

| Phase | Tool | Action |
|-------|------|--------|
| **Phase 1** | mock-server | Build per [mock-server](#mock-server) above. Land alongside the first cut of `internal/ebay/`. IMPL doc tracks the work. |
| **Phase 1** (after cobra) | docgen | Add as `spt gen-docs` hidden subcommand. Wire `just docs-cli` recipe and CI drift check. |
| **Phase 3** | dataset-bootstrap, dataset-upload, judge-bootstrap, regression-runner | Build together when the agentic layer (`internal/agent/`) lands. Single PLAN entry covers the cluster. |
| **Phase 5** | dashgen | Lift wholesale into `tools/dashgen/` when the Helm chart needs bundled dashboards + rules. Wire the `-validate` CI gate. |

**Why phase-aligned and not pull-forward:** porting tools whose phase hasn't started means maintaining unused code. The mock-server is the only exception because it's directly *unblocking* Phase 1 testing.

**Per-tool IMPL docs** scheduled at their phase boundaries — not all upfront. The mock-server IMPL doc is the first one to write; the rest follow when their phase opens.

**Old-spt repository remains available** as reference; no migration of binaries or operational state is required. We are porting *patterns*, not running systems.

## Open Questions

None blocking. Tool-internal questions (e.g., "what's the exact list of fault-injection knobs mock-server needs") are scoped to each tool's IMPL doc and surface during implementation.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0008 — Use OTel + ClickHouse + Langfuse for agent observability and evals](../adr/0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)
- [DESIGN-0001 — Go application layout and conventions](0001-go-application-layout-and-conventions.md)
- [DESIGN-0002 — Domain and pipeline type system](0002-domain-and-pipeline-type-system.md)
- [DESIGN-0003 — eBay API client](0003-ebay-api-client.md)
- [DESIGN-0004 — Alert and reconciliation pipeline](0004-alert-and-reconciliation-pipeline.md)
- [DESIGN-0005 — Pipeline orchestrator and worker model](0005-pipeline-orchestrator-and-worker-model.md)
- [INV-0002 — Old-spt tools triage: port priorities for v1](../investigation/0002-old-spt-tools-triage-port-priorities-for-v1.md)
- Prior-version tools directory: <https://github.com/donaldgifford/server-price-tracker/tree/main/tools>
- Cobra docs generator: <https://pkg.go.dev/github.com/spf13/cobra/doc>
