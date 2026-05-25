---
id: INV-0002
title: "Old-spt tools triage — port priorities for v1"
status: In Progress
author: Donald Gifford
created: 2026-05-24
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0002: Old-spt tools triage — port priorities for v1

**Status:** In Progress
**Author:** Donald Gifford
**Date:** 2026-05-24

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Evaluation criteria](#evaluation-criteria)
- [Findings](#findings)
  - [mock-server](#mock-server)
  - [dashgen](#dashgen)
  - [dataset-bootstrap](#dataset-bootstrap)
  - [dataset-upload](#dataset-upload)
  - [judge-bootstrap](#judge-bootstrap)
  - [regression-runner](#regression-runner)
  - [docgen](#docgen)
- [Cross-cutting observations](#cross-cutting-observations)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

The prior version of spt (`donaldgifford/server-price-tracker`) shipped a `tools/` directory with seven developer-facing utilities. Which of these are worth porting **early** (now or during Phase 1) to accelerate v1 development, versus deferring until the phase they natively belong to, versus skipping entirely?

Concretely: **what's the smallest set of tool-porting work that meaningfully accelerates v1, and when should the rest land?**

## Hypothesis

The mock-server is the only Phase-1-blocking entry — it directly accelerates eBay client unit tests and the reconciliation integration tests required by DESIGN-0003 and DESIGN-0004. The rest are valuable but naturally align with later phases (agentic, packaging) and porting them now would be pull-forward work without a velocity payoff.

## Context

**Triggered by:** Surveys conducted while drafting DESIGN-0003 (eBay client) and DESIGN-0004 (alert + reconciliation pipeline). The surveys found that the prior version had a useful `tools/` directory; DESIGN-0004's Testing Strategy already references the mock-server rewrite as a needed dependency.

This investigation triages the full tools inventory so we don't lose track of useful prior art while still keeping v1 scope tight.

## Approach

1. Catalogue every tool from the prior version's `tools/` directory (already done via background survey — see References).
2. For each tool, score on the evaluation criteria below.
3. Map each tool to the v1 phase that depends on it.
4. Produce a ranked port-or-defer list with concrete recommendations.

## Evaluation criteria

| Criterion | Why it matters |
|-----------|----------------|
| **Phase dependency** | Which v1 phase actually exercises this tool? Tools whose phase is far out shouldn't drag Phase 1. |
| **Port effort** | Low / Medium / High — Low means days, High means weeks. Includes "rewrite for new stack" cost. |
| **Velocity payoff** | What's blocked (or slowed) without it? Sharp answers favor porting early; vague answers favor deferral. |
| **Pre-existing reuse** | Can the old tool run unmodified against our new code? If yes, port-cost is near zero. |
| **Phase fit** | Does the tool naturally cluster with other work in a phase, so porting it then is cheap? |

Recommendation values:

- **Port now (Phase 1)** — block on it; needed to unblock other Phase 1 work.
- **Port at Phase N** — defer until that phase begins; trivially scoped to land alongside that phase's work.
- **Recreate inline** — port effort > rewrite-from-scratch effort; not worth the carry.
- **Lift wholesale** — copy in essentially unchanged; near-zero porting cost.
- **Skip** — no longer relevant.

## Findings

### mock-server

| Field | Value |
|---|---|
| Old shape | OAuth token + Browse search; single fixture file; bind-mount fixture path |
| Phase dependency | **Phase 1** — eBay client unit tests + reconciliation integration tests |
| Port effort | **Medium** — old surface is small but missing capabilities are real |
| Velocity payoff | **High** — without it, eBay client tests use ad-hoc `httptest.NewServer` mocks per test; reconciliation integration tests have nothing to point at |
| Pre-existing reuse | None — needs new endpoints (`GetItem`, Analytics) and scenario semantics |
| Recommendation | **Port now (Phase 1)** |

**What's needed beyond the old version:**

- `GET /buy/browse/v1/item/{item_id}` with scriptable responses per `item_id` (Live current price, Live changed price, Sold + final price, EndedNoSale, NotFound/404).
- `GET /developer/analytics/v1_beta/rate_limit/` returning controllable quota values.
- Latency injection (per-request, configurable via header or query).
- 5xx fault injection (same channel).
- Multi-fixture loading via `embed.FS` (per-scenario fixture directories), not a bind-mounted single file.
- Rate-limit-header support (`X-EBAY-API-Call-Limit`, `X-EBAY-API-Calls-Made`).

**What to lift verbatim from the old version:**

- Docker shape (multi-stage `golang:1.26-alpine` → `alpine:3.21`).
- `containsAllWords` filter trick for multi-word query matching against fixture titles.
- Closure-based title lower-casing for cheap repeated filtering.

### dashgen

| Field | Value |
|---|---|
| Old shape | Self-contained Go module; generates Grafana dashboards + Prometheus recording/alert rules from typed Go code; has its own `go.mod`; supports `-validate` no-write mode |
| Phase dependency | **Phase 5** — packaging, when the Helm chart needs canonical dashboards + alert rules to bundle |
| Port effort | **Low** — self-contained, copy in essentially as-is and update import paths |
| Velocity payoff | Medium when Phase 5 lands. Code-as-config beats hand-edited JSON dashboards. |
| Pre-existing reuse | High — the metric names will differ from old-spt but the *generation pattern* is independent |
| Recommendation | **Port at Phase 5** (lift wholesale) |

The `-validate` mode is the gem here — it gives us a CI gate that the generated dashboards haven't drifted from the source code without diffing committed JSON. Worth preserving.

### dataset-bootstrap

| Field | Value |
|---|---|
| Old shape | CLI that pulls a stratified sample of recent listings from Postgres into a regression JSON for human audit |
| Phase dependency | **Phase 3** — agentic workflows and evals; needs the extractor to produce data worth sampling |
| Port effort | **Low–Medium** — pattern is reusable, but the SQL queries are tied to the new schema and the stratification dimensions (per-Kind, per-Confidence-bucket) need to match our types |
| Velocity payoff | Medium when Phase 3 lands — accelerates eval dataset curation significantly |
| Pre-existing reuse | Idea only; code needs adaptation |
| Recommendation | **Port at Phase 3** (lift the idea, rewrite for our schema) |

Stratified sampling for golden test sets is the right pattern. Don't rebuild the design; just rebuild the implementation.

### dataset-upload

| Field | Value |
|---|---|
| Old shape | Uploads curated regression JSON to Langfuse as a DatasetItem set, using SHA256-truncated deterministic IDs for idempotent upsert |
| Phase dependency | **Phase 3** — pairs with dataset-bootstrap and the Langfuse eval harness |
| Port effort | **Low** — small CLI; the deterministic-ID pattern is the whole insight |
| Velocity payoff | Medium when Phase 3 lands — re-uploading datasets without duplication is the difference between "iterate freely" and "be careful" |
| Pre-existing reuse | Idea preservable; code is small enough to recreate |
| Recommendation | **Port at Phase 3** (lift the idea + the SHA256-truncated-ID trick) |

### judge-bootstrap

| Field | Value |
|---|---|
| Old shape | Two-mode CLI (list/apply) to build few-shot examples for an LLM-as-judge alert classifier; emits `pkg/judge/examples.json` |
| Phase dependency | **Phase 3** — only relevant once we have a judge layer ([ADR-0008](../adr/0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)) |
| Port effort | **Medium** — non-trivial CLI with stratification + score-validation logic |
| Velocity payoff | High when Phase 3 lands — bootstrapping few-shots is otherwise tedious manual work |
| Pre-existing reuse | Idea + workflow shape (list-then-apply) are the keepers |
| Recommendation | **Port at Phase 3** |

The "operator audits existing classifications rather than labels from scratch" workflow is meaningfully cheaper than cold labeling. Don't lose that.

### regression-runner

| Field | Value |
|---|---|
| Old shape | Runs the golden classifications dataset through Ollama / Anthropic / OpenAI backends; reports per-component accuracy + p50/p95 latency; optional Langfuse logging; **explicitly not in CI** (fork-PR / API-key exfil risks documented in source) |
| Phase dependency | **Phase 3** — pairs with the eval datasets and judge |
| Port effort | **Medium** — backend-pluggable design is the meat; need to adapt for our agent framework |
| Velocity payoff | High when Phase 3 lands — automating extractor-quality regression checks is foundational for the agentic-eval discipline |
| Pre-existing reuse | Idea + the explicit anti-CI stance |
| Recommendation | **Port at Phase 3** |

Preserve the explicit anti-CI comment with its reasoning when porting — the decision (run-locally-only because forks could exfil API keys) is worth keeping documented at the call site, not just in commit history.

### docgen

| Field | Value |
|---|---|
| Old shape | ~30 lines of `cobra/doc.GenMarkdownTree` over the spt command tree → `docs/cli/*.md` |
| Phase dependency | **Phase 1** (once cobra commands exist) |
| Port effort | **Trivial** |
| Velocity payoff | Low — auto-generated CLI docs are nice but not blocking |
| Pre-existing reuse | The pattern is one library call; the code itself isn't worth lifting |
| Recommendation | **Recreate inline** when Phase 1 cobra commands exist (likely 5 minutes of work, no separate tool) |

## Cross-cutting observations

**The agentic tools (`dataset-*`, `judge-*`, `regression-runner`) cluster naturally with Phase 3.** Porting them all together with the agentic framework lands them in one focused chunk of work rather than scattered effort. Don't pull them forward.

**`dashgen` is the inverse case** — it could land in any phase, costs near nothing to port, but the *use* of it (dashboards bundled in the Helm chart) is Phase 5. Porting earlier would mean maintaining it while it's unused.

**The mock-server is unique** in that Phase 1 testing genuinely needs it. Without a real mock, the eBay client tests and reconciliation integration tests devolve into per-test `httptest.NewServer` boilerplate, and the integration test in DESIGN-0004 (alert opens → reconcile → sold → alert closes) is hard to write at all.

## Conclusion

**Answer:** Port one tool now, defer the rest to their natural phases.

| Phase | Tool | Action |
|-------|------|--------|
| **Phase 1 (now)** | mock-server | **Port and extend** — needed to unblock eBay client + reconciliation testing |
| **Phase 1 (when cobra lands)** | docgen | Recreate inline — ~30 LOC, not worth a separate tool |
| **Phase 3** | dataset-bootstrap, dataset-upload, judge-bootstrap, regression-runner | Port together when agentic layer comes online |
| **Phase 5** | dashgen | Lift wholesale when Helm chart needs bundled dashboards |

The hypothesis held: mock-server is the only tool whose absence slows Phase 1 work directly.

## Recommendation

1. **Open an IMPL doc — `tools/mock-server` rewrite** — track the port as a Phase 1 work item with the extended-capability list captured under [mock-server](#mock-server) above. Should land in the same window as the first cut of `internal/ebay/`.

2. **Track the deferred tools** in a single PLAN doc (or just in each phase's IMPL when those phases start) so they don't get forgotten. Suggested PLAN entry: "Port deferred old-spt tools at their phase boundaries (Phase 3: agentic tooling; Phase 5: dashgen)."

3. **Capture the per-tool "what to preserve"** notes from this investigation in the IMPL/PLAN docs as they get scheduled — specifically:
   - mock-server: extended endpoint + scenario list (now)
   - dataset-upload: the SHA256-truncated-ID pattern for idempotent Langfuse upserts (Phase 3)
   - regression-runner: the explicit anti-CI rationale (Phase 3)
   - dashgen: the `-validate` no-write CI mode (Phase 5)

4. **Mark this investigation Concluded** once the user accepts these recommendations.

## References

- [DESIGN-0003 — eBay API client](../design/0003-ebay-api-client.md)
- [DESIGN-0004 — Alert and reconciliation pipeline](../design/0004-alert-and-reconciliation-pipeline.md)
- [ADR-0008 — Use OTel + ClickHouse + Langfuse for agent observability and evals](../adr/0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)
- Prior-version tools directory: <https://github.com/donaldgifford/server-price-tracker/tree/main/tools>
