---
id: ADR-0012
title: "Build a custom scheduler and pipeline orchestrator"
status: Accepted
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0012. Build a custom scheduler and pipeline orchestrator

<!--toc:start-->
- [Status](#status)
- [Context](#context)
- [Decision](#decision)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)
<!--toc:end-->

## Status

Accepted (resolved by [INV-0001](../investigation/0001-scheduler-river-vs-custom-vs-alternatives.md))

## Context

spt's "scheduler" role is misleadingly named. Its real job is to **orchestrate a DAG**: a watch trigger fans out into a sequence of stages — extract, score, judge, index, notify — with conditional edges (judge is sampled; notify fires on threshold) and stage-to-stage data passing.

```
poll(watch) ──▶ extract(listing) ──┬──▶ score(listing) ──┬──▶ judge(score)   [sampled]
                                   │                     │
                                   └──▶ index(listing)   └──▶ notify(score)  [if threshold]
```

[INV-0001](../investigation/0001-scheduler-river-vs-custom-vs-alternatives.md) examined the off-the-shelf options:

- **River** and **Asynq** are flat job queues with first-class single-job semantics. Neither models a DAG. The pattern of "job A enqueues job B with its result" works for linear chains but degrades on fan-out, conditional edges, and aggregation — and pushes pipeline topology into handler code where it's untestable.
- **Temporal** would fit the DAG shape but was rejected upstream in RFC-0001 as operationally overweight for spt's scale.

The mismatch is structural: a flat job queue is the wrong primitive for a DAG. Wrapping it in a thin DAG layer leaves us owning the hard part (the orchestration logic) while still paying the framework's operational cost — at which point the framework is essentially just an enqueue/dequeue primitive that Valkey already gives us.

The scheduler role also needs to handle requirements adjacent to orchestration: per-watch cadence triggering, global rate-pacing against eBay, and leader election across replicas. These compound naturally with custom DAG execution.

## Decision

We will build a **custom scheduler and pipeline orchestrator** as a role within the spt binary (`spt scheduler`). It is responsible for:

1. **Triggering** — read watches from Postgres on a tick loop and fire those whose `next_run_at` has elapsed.
2. **DAG execution** — evaluate the pipeline DAG (extract → score → {judge, index} → notify) for each triggered watch. Stage transitions are explicit; conditional edges and sampling live in the orchestrator, not in stage handlers.
3. **Rate-pacing** — share a global eBay token-bucket via Valkey ([ADR-0005](0005-use-valkey-for-queues-and-caching.md)) across all in-flight pipelines.
4. **Dispatch** — push individual stage executions into Valkey for the worker pool to consume.
5. **Leader election** — Postgres advisory locks ensure a single active orchestrator across replicas.

The DAG topology is defined explicitly in Go (one place, reviewable, testable). Stage handlers themselves are simple functions that take inputs and produce outputs; they don't know about the DAG.

A follow-up DESIGN doc will cover the executor model (in-process state machine vs. fully Valkey-mediated stage handoff), failure/retry semantics per stage, OTel span shape for end-to-end traces, and the sampling logic for `judge`.

## Consequences

### Positive

- DAG topology is explicit, reviewable, and unit-testable independent of handlers.
- Stage handlers stay simple — they're functions over typed inputs and outputs, not framework-aware code.
- Rate-pacing and eval-trigger requirements drop in naturally; they were always application logic.
- End-to-end OTel trace shape matches the DAG shape — operators see the pipeline as it actually executes.
- We own a critical-path component fully; no framework upgrades break our orchestration.
- The "we want to build it" factor is honest: this is a component the team will own for the life of the project, and we want to understand it deeply.

### Negative

- We own all of it: failure semantics, retry policy, leader election, observability, crash recovery. No upstream community is fixing scheduler bugs for us.
- Risk of scope creep into a generic workflow engine. Mitigation: hold the line at "this specific DAG and the primitives it needs." Generic workflow features only ship when a second pipeline demands them.
- Less battle-tested than River/Asynq at v1.

### Neutral

- Complexity ceiling: ~1k LOC of core orchestrator logic is the trip-wire to revisit. Beyond that we're likely reinventing something that exists.
- We commit to documenting the DAG topology in code and in a DESIGN doc; both must stay in sync.

## Alternatives Considered

See [INV-0001](../investigation/0001-scheduler-river-vs-custom-vs-alternatives.md) for the full comparison. Summary:

- **River** — Postgres-backed job queue. Excellent for what it is; not a DAG orchestrator. Rejected: shape mismatch.
- **Asynq** — Redis/Valkey-backed job queue. Same shape mismatch as River.
- **Temporal** — DAG-capable but operationally overweight for spt's scale; rejected upstream in RFC-0001.
- **gocron / robfig/cron** — too thin (no persistence, no cross-replica coordination, no DAG).

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [INV-0001 — Scheduler — River vs custom vs alternatives](../investigation/0001-scheduler-river-vs-custom-vs-alternatives.md)
- [ADR-0004 — Use PostgreSQL as the canonical relational store](0004-use-postgresql-as-the-canonical-relational-store.md)
- [ADR-0005 — Use Valkey for queues and caching](0005-use-valkey-for-queues-and-caching.md)
