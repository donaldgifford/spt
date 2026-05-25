---
id: ADR-0004
title: "Use PostgreSQL as the canonical relational store"
status: Proposed
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0004. Use PostgreSQL as the canonical relational store

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

spt needs a transactional store for canonical state: users, watches, listings, scores, notification configuration. The workload is read-heavy with append-dominated writes (each polling cycle inserts listings), occasional bulk updates (re-scoring), and a mix of OLTP-style point reads and small-to-medium analytical queries. Analytics at scale moves to ClickHouse ([ADR-0008](0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)); Postgres handles everything that's not "hot analytical."

## Decision

We will use **PostgreSQL** (16+) as the canonical relational store. Access from Go is via `database/sql` with the **pgx** driver in `database/sql` mode — no ORM. Migrations are managed via a tool TBD (likely `goose` or `tern`; not blocking on this ADR).

## Consequences

### Positive

- Mature, predictable, and well-understood operationally.
- Strong fit with std-lib-first preference via `database/sql` + `pgx`.
- Excellent JSON support if we need semi-structured columns for eBay listing payloads.
- Materialized views, partial indexes, and window functions give us a long runway before analytics has to leave Postgres.
- Wide hosted-option availability (RDS, Cloud SQL, Supabase, self-hosted) for users running the Helm chart.

### Negative

- Vertical scaling has a ceiling; very high-volume analytical workloads belong elsewhere (ClickHouse).
- Schema migrations need discipline; no ORM means we hand-roll DDL.

### Neutral

- pgx is the de facto Go Postgres driver; choice is uncontroversial.

## Alternatives Considered

- **MySQL/MariaDB** — viable, but Postgres's JSON, partial indexes, and analytical SQL surface are better fits for this workload.
- **SQLite** — too constrained for a multi-role deployment with concurrent writers.
- **ORM (gorm, ent, bun)** — rejected per RFC-0001's std-lib preference; we want hand-tuned queries against Postgres without an abstraction layer.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0008 — Use OTel + ClickHouse + Langfuse for agent observability and evals](0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)
- pgx: <https://github.com/jackc/pgx>
