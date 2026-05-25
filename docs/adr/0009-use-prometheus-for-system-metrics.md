---
id: ADR-0009
title: "Use Prometheus for system metrics"
status: Proposed
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0009. Use Prometheus for system metrics

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

spt needs system-level metrics — queue depth, scheduler tick lag, HTTP request rates and latency, polling error rates, DB connection pool state — that answer "is the system healthy?" These are distinct from agent-quality metrics ([ADR-0008](0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)) and benefit from a separate, purpose-built tool.

This split is intentional. Mixing health-signal metrics into the eval pipeline produces high-cardinality noise; mixing eval data into a Prometheus instance produces an expensive cardinality bill. Two surfaces, two tools.

## Decision

We will use **Prometheus** as the system metrics backend. The Go binary exposes a `/metrics` endpoint in each role via the official `prometheus/client_golang` library. A Prometheus server (user-provided or bundled in the Helm chart's optional dependencies) scrapes it. Dashboards target Grafana but are not bundled in v1.

Cardinality discipline is enforced via convention: label values come from a small, bounded set per metric.

## Consequences

### Positive

- Industry-standard for system metrics; every K8s operator already has a Prometheus.
- Pull-based scrape model is friendly to ephemeral worker pods.
- `client_golang` is mature and integrates cleanly with `net/http`.
- Clean split from agent-quality observability keeps both surfaces clean.

### Negative

- Two observability stacks (Prometheus + OTel/ClickHouse/Langfuse) to operate.
- Cardinality discipline requires ongoing attention — bad labels are easy to add and expensive to remove.

### Neutral

- We commit to documenting the metric names, types, and label conventions for downstream Grafana dashboards.

## Alternatives Considered

- **OTel metrics + same backend as traces** — possible (OTel has a metrics SDK), but the Prometheus ecosystem (PromQL, Alertmanager, hosted-Prometheus offerings) is overwhelmingly the de facto standard for system metrics. Going OTel-only sacrifices that maturity.
- **StatsD / DogStatsD** — push-based and tied historically to specific vendors; weaker fit for K8s.
- **Roll metrics into ClickHouse** — possible but Prometheus's purpose-built model (PromQL, recording rules, alerting) is worth keeping separate.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0008 — Use OTel + ClickHouse + Langfuse for agent observability and evals](0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)
- Prometheus: <https://prometheus.io>
- `client_golang`: <https://github.com/prometheus/client_golang>
