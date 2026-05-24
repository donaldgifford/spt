---
id: RFC-0001
title: "Server Price Tracker Platform"
status: Draft
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# RFC 0001: Server Price Tracker Platform

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-23

<!--toc:start-->
- [Summary](#summary)
- [Problem Statement](#problem-statement)
- [Proposed Solution](#proposed-solution)
- [Design](#design)
  - [System overview](#system-overview)
  - [Backend: single Go binary, multiple roles](#backend-single-go-binary-multiple-roles)
  - [Datastores](#datastores)
  - [Agentic layer and observability](#agentic-layer-and-observability)
  - [Frontend](#frontend)
  - [Packaging and deployment](#packaging-and-deployment)
- [Alternatives Considered](#alternatives-considered)
- [Open Questions](#open-questions)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Ingestion and storage](#phase-1-ingestion-and-storage)
  - [Phase 2: Scoring and market analytics](#phase-2-scoring-and-market-analytics)
  - [Phase 3: Agentic workflows and evals](#phase-3-agentic-workflows-and-evals)
  - [Phase 4: UI, notifications, and search](#phase-4-ui-notifications-and-search)
  - [Phase 5: Packaging and Helm release](#phase-5-packaging-and-helm-release)
- [Risks and Mitigations](#risks-and-mitigations)
- [Success Criteria](#success-criteria)
- [References](#references)
<!--toc:end-->

## Summary

`spt` (Server Price Tracker) is an agentic platform that polls user-defined eBay queries, extracts and stores listings, scores them, and derives market analytics — moving averages, percentile pricing, time-to-sell, average listing length, and related signals. It exposes those signals through a web UI and an API, and ships as a Docker image and a Helm chart so users can run it locally or on Kubernetes.

## Problem Statement

The used and refurbished server hardware market on eBay is opaque. Pricing varies widely across sellers, conditions, and listing styles; "is this a good deal?" requires manually tracking comparable listings over weeks, eyeballing sold-listing history, and developing intuition that doesn't transfer between SKUs. No general-purpose tool surfaces structured market signals (percentile pricing, recent moving averages, expected time-to-sell) for arbitrary watch lists, and no tool offers an agentic layer for higher-level reasoning (deal-quality scoring, anomaly detection, listing-quality assessment) with the eval discipline needed to trust those outputs.

This RFC proposes the platform end-to-end. Specific component-level decisions (data model, scheduler design, scoring algorithm, eval harness) will be captured in follow-up ADRs and DESIGN docs as they solidify; this document establishes the shape of the system so those decisions have a frame to fit into.

## Proposed Solution

A service-oriented platform built as a single Go binary that runs in multiple roles (API, scheduler, worker), backed by Postgres for canonical storage, Valkey for queues and caching, Meilisearch for listing search, and ClickHouse for analytics and OTel trace storage. An agentic layer wraps scoring and analysis with Langfuse-based evals and judges. A Bun + React Router + shadcn UI consumes a generated API client to manage watches, configure notification thresholds, and explore market data.

The platform is packaged as a Docker image and a Helm chart so users can self-host on Docker Compose or Kubernetes.

## Design

### System overview

```
                  ┌────────────────┐
                  │  React UI      │
                  │ (Bun + RR +    │
                  │  shadcn)       │
                  └───────┬────────┘
                          │  generated API client
                          ▼
                  ┌────────────────┐
                  │  API           │  ◀── role: api
                  └───────┬────────┘
            ┌─────────────┼─────────────┐
            │             │             │
            ▼             ▼             ▼
      ┌──────────┐  ┌──────────┐  ┌──────────┐
      │ Postgres │  │  Valkey  │  │  Meili   │
      │ (truth)  │  │ (queue,  │  │ (search) │
      │          │  │  cache)  │  │          │
      └────┬─────┘  └─────┬────┘  └──────────┘
           │              │
           │              │ pop jobs
           │              ▼
           │       ┌────────────┐       ┌────────────┐
           │       │  Worker(s) │──────▶│  eBay API  │
           │       └─────┬──────┘       └────────────┘
           │             │
           ▼             ▼
      ┌──────────────────────────┐    ┌─────────────────┐
      │  ClickHouse              │◀───│   OTel traces   │
      │  (analytics + traces)    │    │   from all roles│
      └──────────────────────────┘    └─────────────────┘
                  ▲
                  │ evals, judges, datasets
                  ▼
            ┌────────────┐
            │  Langfuse  │
            └────────────┘

   ┌───────────────┐
   │  Scheduler    │  ◀── role: scheduler
   │  (custom)     │      enqueues poll jobs into Valkey
   └───────────────┘      based on watch cadence
```

### Backend: single Go binary, multiple roles

The backend is a single Go binary in a monorepo. Behavior is selected at startup by subcommand and configuration — for example `spt api`, `spt scheduler`, `spt worker`. This keeps build, release, and image surface area minimal while still letting each role scale independently in Kubernetes (separate Deployments backed by the same image).

Standard-library-first is a strong preference. Concretely that means `net/http` (no external HTTP framework), `database/sql` with `pgx` as the driver (no ORM beyond a thin query layer), `log/slog` for structured logging, and hand-wired OTel + Prometheus instrumentation rather than a turnkey middleware stack. The benefit is full control over tracing semantics and metric cardinality — both of which matter once the agentic layer is producing high-volume spans we want to evaluate.

The scheduler is custom because the work it orchestrates is a **DAG** (extract → score → {judge, index} → notify), not a flat job stream. Generic Go job libraries (River, Asynq) are excellent at single-job semantics but don't model DAGs; wrapping them in a thin DAG layer leaves us owning the hard part while still paying the framework's operational cost. Temporal models DAGs but is overweight for our scale. See [ADR-0012](../adr/0012-build-a-custom-scheduler-and-pipeline-orchestrator.md) and [INV-0001](../investigation/0001-scheduler-river-vs-custom-vs-alternatives.md).

### Datastores

| Store        | Role                                                      |
|--------------|-----------------------------------------------------------|
| Postgres     | Canonical truth: watches, listings, scores, users, config |
| Valkey       | Job queue (worker dispatch), short-TTL cache              |
| Meilisearch  | Full-text search over listing titles, descriptions, sellers |
| ClickHouse   | Aggregations, historical analytics, OTel trace backend    |

In-memory queues are used for short-lived, intra-process fan-out within a role (e.g., a worker's internal pipeline stages); Valkey is the queue between roles.

### Agentic layer and observability

Agentic workflows — listing extraction, deal-quality scoring, anomaly judges — are instrumented via OTel and exported to ClickHouse. Langfuse ingests the same trace data (or a derived subset) and runs evals and LLM-as-judge passes against curated datasets. The Prometheus metrics path is separate and focused on system-health signals (queue depth, scheduler tick lag, poll error rate).

This split — OTel/ClickHouse/Langfuse for *agent quality*, Prometheus for *system health* — is intentional. The two surfaces ask different questions and benefit from different storage and visualization tools.

### Frontend

A Bun + React Router single-page app, styled with shadcn/ui. The API client is generated from the backend's OpenAPI (or similar) spec so the frontend stays in lockstep with backend changes. The UI is a thin wrapper around the API:

- Manage the watch list (add, edit, delete watches; configure notification baselines per watch)
- Display market signals and listing history for each watch
- Search listings via Meilisearch

The UI deliberately holds no business logic the API doesn't expose. That keeps the API the system's contract and the UI a swappable presentation layer.

### Packaging and deployment

- Docker image: distroless, multi-arch (amd64/arm64), built via `docker buildx bake` (already scaffolded).
- Helm chart: published alongside releases. Default values target a small single-replica deployment; production values enable per-role scaling and external Postgres/Valkey/Meilisearch/ClickHouse.

## Alternatives Considered

**Multi-service backend (separate API, scheduler, worker binaries).** Rejected for v1 in favor of single-binary-multi-role: same build artifact, same dependency graph, lower release surface area. Roles still deploy independently because the role is a CLI flag, not a binary. Revisitable if roles diverge enough that they want different build-time dependencies.

**An ORM (gorm, ent) over `database/sql` + `pgx`.** Rejected for the same std-lib-first reasoning: we own enough of the query surface that an ORM's ergonomics don't pay back the abstraction cost, and we want hand-tuned analytical queries against Postgres without fighting a generic query builder.

**Generic job framework (Asynq, River, Temporal) instead of a custom scheduler/orchestrator.** Investigated in [INV-0001](../investigation/0001-scheduler-river-vs-custom-vs-alternatives.md) and resolved in favor of custom. River and Asynq are flat job queues, not DAG orchestrators, and the extract → score → judge pipeline is a DAG. Temporal would fit but is operationally overweight for spt's scale. See [ADR-0012](../adr/0012-build-a-custom-scheduler-and-pipeline-orchestrator.md).

**Datastore choices (Postgres, Valkey, Meilisearch, ClickHouse).** Each will get its own ADR. The combination is intentional: relational truth, fast queue/cache, search-tuned index, and a columnar store for analytics and traces.

## Open Questions

**How should market analytics be computed?** The headline signals (7/30-day moving averages, percentile scoring, time-to-sell, average listing length) can be implemented in several places along a spectrum:

1. **Materialized views in Postgres**, refreshed on a schedule. Simple, transactional, but expensive at high listing volume and limited in query flexibility.
2. **Continuous aggregates in ClickHouse**, with Postgres holding only the canonical listings. Cheap reads, scales well, but adds a sync step from Postgres → ClickHouse and pushes analytics queries off the primary store.
3. **On-demand in the API layer**, computed per request against Postgres with aggressive Valkey caching. Most flexible, easiest to iterate on, but hardest to keep fast as data grows.
4. **A dedicated analytics role**, pre-computing rollups on a schedule and writing them back to Postgres or ClickHouse.

This is the single largest unresolved design decision. It should be settled in an ADR before Phase 2 begins.

## Implementation Phases

### Phase 1: Ingestion and storage

- Single-binary scaffold with `api`, `scheduler`, `worker` subcommands.
- Postgres schema for watches and listings; Valkey-backed queue.
- eBay polling worker; raw listings stored canonically.
- OTel + Prometheus wiring (no eval layer yet).

### Phase 2: Scoring and market analytics

- Resolve the analytics-computation ADR (see Open Questions).
- Implement the chosen pipeline; expose signals via API.
- ClickHouse online for analytics (and ready to receive OTel traces in Phase 3).

### Phase 3: Agentic workflows and evals

- Add the agentic scoring/judging layer.
- Wire ClickHouse + Langfuse for trace storage and evals.
- Curate the first eval datasets.

### Phase 4: UI, notifications, and search

- Bun + React Router + shadcn UI; generated API client.
- Meilisearch indexing pipeline; search UI.
- Per-watch notification baselines and delivery (channel TBD).

### Phase 5: Packaging and Helm release

- Polish the existing Docker pipeline; publish Helm chart.
- Document self-hosting on Docker Compose and Kubernetes.

## Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| eBay API rate limits or ToS changes break ingestion | High | Medium | Centralize all eBay calls behind a single rate-paced client; track quota in Valkey; design watches to degrade (slower cadence) rather than fail when quota is tight |
| Agentic scoring drifts in quality over time | High | High | Langfuse evals + curated datasets gating model/prompt changes; alert on judge-score regression |
| Premature scaling complexity from the multi-role binary | Medium | Medium | Default deployment runs all roles in one Pod; per-role scaling is opt-in via Helm values |
| Wrong analytics-computation choice (see Open Questions) becomes expensive to migrate | Medium | Medium | Settle in an ADR before Phase 2; design the API layer so the compute backend is swappable |
| ClickHouse operational overhead for self-hosters | Medium | Medium | Helm chart supports external ClickHouse and a "minimal" mode that disables ClickHouse-dependent features |
| Custom scheduler reinvents a job framework poorly | Medium | Low | Keep the scheduler's surface area small; if it grows beyond ~1k LOC of core logic, revisit River |

## Success Criteria

- A user can add an eBay query as a watch and, within one polling cycle, see structured listings and at least one market signal in the UI.
- Moving averages, percentile pricing, time-to-sell, and average listing length are exposed via API and rendered in the UI.
- Agentic scores have an eval harness with a baseline dataset; eval scores are visible in Langfuse and gate releases.
- The platform deploys via `helm install` against a vanilla Kubernetes cluster using bundled defaults.
- All three roles (`api`, `scheduler`, `worker`) emit OTel traces to ClickHouse and Prometheus metrics scrapable by a standard Prometheus server.

## References

- *(forthcoming)* ADR: Single-binary, multi-role backend
- *(forthcoming)* ADR: Standard-library-first Go stack
- *(forthcoming)* ADR: Datastore selection — Postgres, Valkey, Meilisearch, ClickHouse
- *(forthcoming)* ADR: Market analytics computation strategy *(resolves the Open Question)*
- [ADR-0012 — Build a custom scheduler and pipeline orchestrator](../adr/0012-build-a-custom-scheduler-and-pipeline-orchestrator.md) *(resolved by [INV-0001](../investigation/0001-scheduler-river-vs-custom-vs-alternatives.md))*
- *(forthcoming)* DESIGN: eBay polling worker and rate-pacing
- *(forthcoming)* DESIGN: Agentic scoring pipeline and Langfuse eval harness
