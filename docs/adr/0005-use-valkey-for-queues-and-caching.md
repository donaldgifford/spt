---
id: ADR-0005
title: "Use Valkey for queues and caching"
status: Proposed
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0005. Use Valkey for queues and caching

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

spt needs two distinct primitives:

1. **A job queue** between the scheduler and worker roles ([RFC-0001](../rfc/0001-server-price-tracker-platform.md)). The scheduler enqueues `poll_watch` jobs (and similar); workers consume and process them.
2. **A short-TTL cache** for things like eBay rate-limit token state, recent listing fingerprints (dedup), and API response caching.

Both are well-served by a Redis-compatible in-memory data store. We want the Redis API and ecosystem but prefer the BSD-3-Clause licensed fork.

## Decision

We will use **Valkey** (8+) for both job queueing and caching. Access from Go is via a Redis-compatible client library (e.g., `valkey-go` or `go-redis`, decision deferred — both speak the Valkey wire protocol).

Queue semantics layer on top of Valkey primitives (lists, streams, or a thin job-queue library). If [INV-0001](../investigation/0001-scheduler-river-vs-custom-vs-alternatives.md) concludes in favor of Asynq, Valkey doubles as its backend.

## Consequences

### Positive

- BSD-3-Clause licensed (vs. Redis's SSPL); friendlier for downstream users of our Helm chart.
- Drop-in Redis API compatibility; no library or operational lock-in.
- Active development backed by the Linux Foundation.
- Single component covers both queue and cache, reducing operational surface.

### Negative

- Newer brand; some hosted offerings still catalog it as "Redis-compatible" rather than first-class.
- In-memory persistence model means careful configuration is required if we ever treat queue state as durable.

### Neutral

- The Redis ecosystem (clients, ops tooling) applies unchanged.

## Alternatives Considered

- **Redis** — functionally equivalent but SSPL licensing concerns and the Valkey fork's better governance model push us away.
- **RabbitMQ / NATS** — proper message brokers with richer routing semantics. Rejected as over-spec for our queue shape (single producer, simple worker pool).
- **Postgres-backed queue (e.g., River)** — possible, and pulled in if INV-0001 picks River. Doesn't replace the cache need.
- **In-process channel queue** — fine within a role but doesn't cross role boundaries; we need queueing between scheduler and worker pods.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [INV-0001 — Scheduler — River vs custom vs alternatives](../investigation/0001-scheduler-river-vs-custom-vs-alternatives.md)
- Valkey: <https://valkey.io>
