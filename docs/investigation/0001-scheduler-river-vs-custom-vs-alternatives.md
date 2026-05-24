---
id: INV-0001
title: "Scheduler — River vs custom vs alternatives"
status: Concluded
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0001: Scheduler — River vs custom vs alternatives

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-05-23

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Candidates](#candidates)
- [Approach](#approach)
- [Evaluation criteria](#evaluation-criteria)
- [Environment](#environment)
- [Findings](#findings)
  - [River and Asynq](#river-and-asynq)
  - [Temporal](#temporal)
  - [Custom](#custom)
  - [Cross-cutting observation](#cross-cutting-observation)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

For spt's scheduling needs — periodic eBay polling per watch, global rate-pacing against the eBay API, and selective triggering of eval workloads — should we adopt an existing Go job/scheduler framework (River, Asynq, Temporal, gocron, etc.), or build a purpose-built scheduler?

Concretely: does any off-the-shelf option give us **(a)** per-watch cadence configuration, **(b)** a global token-bucket or leaky-bucket rate limiter shared across all watches, and **(c)** the ability to mark a subset of jobs as eval-triggering without bolting a parallel system on top?

## Hypothesis

A purpose-built scheduler will be smaller and clearer than retrofitting any of the above frameworks. Two reasons:

1. The dominant complexity isn't job execution (workers consume from Valkey for that). It's *pacing* — coordinating cadence and a shared rate budget — which most frameworks don't handle directly.
2. The eval-trigger hook is application-domain logic that doesn't belong inside a generic job framework.

We expect to confirm a custom scheduler is the right call, but want to validate by actually trying River specifically — it's the strongest contender and the one we'd regret skipping without a fair look.

## Context

**Triggered by:** [RFC-0001](../rfc/0001-server-price-tracker-platform.md) — RFC-0001 took a tentative position in favor of a custom scheduler but flagged this as worth re-checking. [ADR-0012](../adr/0012-build-a-custom-scheduler-and-pipeline-orchestrator.md) was blocked on this investigation's conclusion (now resolved).

The scheduler is on the critical path for Phase 1 (ingestion). Picking wrong here is recoverable but expensive — either we throw away a custom scheduler and rewrite onto River, or we rip out River when its abstractions stop fitting.

## Candidates

| Candidate | Type | Persistence | Notes |
|-----------|------|-------------|-------|
| **River** | Postgres-backed job queue | Postgres | Modern, idiomatic Go, active development. Our top off-the-shelf candidate |
| **Asynq** | Redis-backed job queue | Redis/Valkey | Mature, simpler than River. Aligns with our Valkey choice |
| **Temporal** | Workflow engine | Postgres/Cassandra | Heavyweight; overkill for our needs but worth a paragraph to dismiss |
| **gocron / robfig/cron** | In-process scheduler | None | Too thin — no persistence, no cross-replica coordination |
| **Custom (in-tree)** | Purpose-built | Postgres (next-run-at column on `watches`) | Hypothesis favorite |

## Approach

1. **Read** River and Asynq docs end-to-end; identify the closest equivalent to "per-job cadence + shared rate budget + eval hook" in each.
2. **Spike River**: minimal cmd that enqueues a `poll_watch` job per row in a mock `watches` table, runs it through a fake eBay client wrapped in a shared rate limiter, and emits a span when the eval flag is set. Measure: lines of glue code, clarity of the rate-limit integration, friction of the eval hook.
3. **Spike Asynq**: same exercise. Compare against River, especially on the Valkey-native angle.
4. **Sketch the custom scheduler**: design doc only, no code. Identify the data model (likely `next_run_at` + `cadence_seconds` columns on `watches`) and the tick loop. Count expected LOC honestly.
5. **Compare** all three on the evaluation criteria below.

## Evaluation criteria

| Criterion | Weight | Why it matters |
|-----------|--------|----------------|
| Per-watch cadence configurability | High | Core requirement; user-facing |
| Shared rate-budget integration | High | eBay rate limits are a hard constraint |
| Eval-trigger hook ergonomics | High | Couples scheduler to agentic layer |
| Operational footprint (extra services, schemas, migrations) | Medium | We already have Postgres + Valkey; adding more is a cost |
| LOC / complexity of integration glue | Medium | Proxy for ongoing maintenance |
| Cross-replica safety (we run multiple scheduler pods?) | Medium | Affects Helm chart and HA story |
| Community/maturity | Low | All three options are credible; this is a tiebreaker |
| Std-lib alignment | Low | Soft preference, not a deal-breaker |

## Environment

| Component | Version / Value |
|-----------|----------------|
| Go        | as pinned in `go.mod` |
| River     | latest at investigation time |
| Asynq     | latest at investigation time |
| Postgres  | 16+ |
| Valkey    | 8+ |

## Findings

The investigation was cut short by a reframing of the requirements rather than completed via the spike plan above. The original framing treated the scheduler as a job-pacing problem with three secondary requirements (cadence, rate budget, eval flag). On closer look, the agentic pipeline is fundamentally a **DAG**, not a flat job stream:

```
poll(watch) ──▶ extract(listing) ──┬──▶ score(listing) ──┬──▶ judge(score)   [sampled]
                                   │                     │
                                   └──▶ index(listing)   └──▶ notify(score)  [if threshold]
```

- `poll` fans out to N `extract` jobs (one per listing returned).
- Each `extract` fans out to `score` + `index`.
- `score` conditionally fans out to `judge` (sampling rate) and `notify` (threshold).
- Downstream stages need upstream outputs (extracted listing → score input; score → judge input).

That's a workflow DAG, not a job queue. The implication:

### River and Asynq

Both are flat job queues with first-class single-job semantics: enqueue, retry, schedule. Neither has native DAG support. The pattern people use to fake it — "in job A, enqueue job B with A's result as the payload" — works for linear chains but degrades badly for fan-out, conditional edges, and aggregation. It also pushes the *pipeline topology* into the job handler code, where it's invisible to operators and untestable in isolation.

We'd end up writing a thin DAG layer on top of River/Asynq. At that point we own the orchestration anyway; the framework is just an enqueue/dequeue primitive that we could trivially replace with `BLPOP` on Valkey.

### Temporal

Temporal *is* a DAG/workflow engine and would technically fit. Rejected upstream in RFC-0001 as operationally overweight (separate cluster, schema migrations, SDK surface area). That dismissal stands — Temporal is the right tool for a much larger system than spt is.

### Custom

A purpose-built orchestrator gives us:

- DAG topology defined explicitly in Go (testable, reviewable, visible).
- Direct ownership of fan-out, conditional edges, and sampling logic — these are *domain* concerns, not infrastructure.
- The rate-pacing and eval-trigger requirements drop in naturally; they were always going to be application logic anyway.
- A clear ceiling: tick loop + DAG executor + Postgres lock + Valkey enqueue. Small, knowable, fully ours.

### Cross-cutting observation

The scheduler role in spt isn't really a scheduler in the cron sense — it's a **pipeline orchestrator** that happens to also handle periodic triggering. The role's responsibilities are: trigger watches on cadence, evaluate the DAG, pace external calls, dispatch units of work to the worker pool. Calling it "the scheduler" is a useful shorthand but understates the scope.

## Conclusion

**Answer:** Build a custom scheduler / orchestrator.

The deciding factor was not implementation cost — both River and Asynq would have been quick to wire up for *job execution* — but the **shape mismatch** between a flat job queue and the extract → score → judge DAG that actually models the work. Adding DAG semantics on top of a job queue is more code, less clarity, and worse observability than owning the orchestrator outright. Temporal would fit the shape but at an operational cost we already rejected.

There is also an explicit "we want to build this" component to the choice. That's a legitimate input for a long-lived component the team will own indefinitely; documented here for honesty rather than hidden.

## Recommendation

1. Flip **ADR-0012** to **Accepted**, update its Context to lead with the DAG framing, and broaden its scope from "scheduler" to "scheduler + pipeline orchestrator."
2. Update **RFC-0001**'s scheduler section to reflect the resolved decision (drop the "tentative position" hedge).
3. Open a **DESIGN** doc for the orchestrator before Phase 1 implementation begins. It should cover: DAG definition format, executor model (in-process vs Valkey-mediated handoff between stages), failure/retry semantics per stage, sampling logic for `judge`, leader election, and OTel span shape for end-to-end pipeline traces.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0012 — Build a custom scheduler and pipeline orchestrator](../adr/0012-build-a-custom-scheduler-and-pipeline-orchestrator.md)
- River: <https://github.com/riverqueue/river>
- Asynq: <https://github.com/hibiken/asynq>
- Temporal: <https://temporal.io/>
