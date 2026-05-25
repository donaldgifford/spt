---
id: ADR-0008
title: "Use OTel + ClickHouse + Langfuse for agent observability and evals"
status: Proposed
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0008. Use OTel + ClickHouse + Langfuse for agent observability and evals

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

Proposed

## Context

spt's agentic layer ([RFC-0001](../rfc/0001-server-price-tracker-platform.md)) — listing extraction, deal-quality scoring, anomaly judges — needs observability that goes beyond "did the request succeed?":

- We need to trust the agent's outputs. That means **evals** (offline and continuous) against curated datasets, plus LLM-as-judge passes on representative samples.
- We need to debug specific runs (prompt, context, tool calls, response) when an output looks wrong.
- We need historical analytics over agent behavior (score distributions, judge regressions, prompt-version diffs).

These are different questions from "is the system healthy?" (covered by [ADR-0009](0009-use-prometheus-for-system-metrics.md)).

## Decision

We will adopt a three-part stack for agent observability and evals:

1. **OpenTelemetry** as the instrumentation API. Agentic spans (LLM calls, tool calls, eval results) are emitted from Go via the standard OTel SDK.
2. **ClickHouse** as the trace storage backend. Columnar storage is the right shape for trace-volume analytics; queries over span attributes, durations, and score columns are cheap.
3. **Langfuse** as the eval and judge harness. Langfuse ingests the same trace data (or a derived subset) and runs evals + LLM-as-judge passes against curated datasets.

The split between trace storage (ClickHouse) and eval orchestration (Langfuse) lets each tool do what it's best at: ClickHouse for ad-hoc analytics, Langfuse for eval workflows and dataset management.

## Consequences

### Positive

- OTel is the vendor-neutral standard; we're not locked into any backend.
- ClickHouse is purpose-built for trace-shaped data; expensive queries that would crater Postgres are routine here.
- Langfuse provides a turnkey eval/dataset/judge workflow we'd otherwise have to build.
- Clean separation of agent-quality concerns (this ADR) from system-health concerns ([ADR-0009](0009-use-prometheus-for-system-metrics.md)).

### Negative

- Three observability components to operate (Prometheus is a fourth — see ADR-0009). For Helm users this is significant.
- ClickHouse is operationally non-trivial; we'll need to support a "minimal mode" that disables ClickHouse-dependent features so users without it can still run spt.
- Langfuse is a hosted service or self-hosted stack; we add an integration but also an external dependency.

### Neutral

- Eval datasets become a versioned artifact we maintain alongside the code.

## Alternatives Considered

- **Generic APM (Datadog, New Relic, Honeycomb)** — strong for system traces but weak for agent-specific eval workflows; we'd still need Langfuse-equivalent tooling.
- **Roll our own eval harness on top of Postgres** — possible but a meaningful side-project; Langfuse exists for this exact use case.
- **Trace storage in Postgres or in Tempo/Jaeger** — Postgres doesn't fit the workload shape; Tempo/Jaeger are fine for trace-viewing but worse for analytical queries we want over span attributes.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0009 — Use Prometheus for system metrics](0009-use-prometheus-for-system-metrics.md)
- [ADR-0011 — Use sdk-booty-sh as the agentic framework](0011-use-sdk-booty-sh-as-the-agentic-framework.md)
- OpenTelemetry: <https://opentelemetry.io>
- ClickHouse: <https://clickhouse.com>
- Langfuse: <https://langfuse.com>
