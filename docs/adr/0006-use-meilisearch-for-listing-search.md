---
id: ADR-0006
title: "Use Meilisearch for listing search"
status: Proposed
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0006. Use Meilisearch for listing search

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

The UI surfaces a search experience over listings — by title, description, seller, condition, and other faceted attributes ([RFC-0001](../rfc/0001-server-price-tracker-platform.md)). Postgres full-text search is viable for small volumes but degrades on ranking quality, typo tolerance, and faceted UX as the listing corpus grows.

Requirements:

- Sub-100ms search latency.
- Typo tolerance and prefix matching out of the box.
- Faceted filtering (condition, price range, seller rating).
- Simple operational footprint — we're shipping a Helm chart users will run.

## Decision

We will use **Meilisearch** as the search index for listings. Postgres remains the source of truth; an indexer worker syncs listing changes into Meilisearch (initially fire-and-forget on listing insert/update, with a periodic full-rebuild fallback).

## Consequences

### Positive

- Typo tolerance, prefix matching, and faceted search are first-class and require minimal configuration.
- Single-binary deployment; trivially easy to run in our Helm chart and in dev.
- Fast indexing and fast queries on the listing volumes we expect.
- Good Go client (`meilisearch-go`).

### Negative

- Adds a fourth datastore (Postgres + Valkey + ClickHouse + Meilisearch) to operate.
- Index sync from Postgres adds a consistency dimension; we accept eventual consistency for search.
- Less flexible than Elasticsearch for unusual query shapes (rare for our use case).

### Neutral

- Schema/index settings live in code (init-time bootstrap), versioned with the application.

## Alternatives Considered

- **Postgres FTS** — zero new infra, but the UX (typo tolerance, faceting, ranking) is noticeably worse and gets harder to tune as corpus grows.
- **Elasticsearch / OpenSearch** — more powerful and more flexible, but heavier to run (JVM, multi-node cluster) and overkill for our listing volume and query shape.
- **Typesense** — direct competitor to Meilisearch with similar ergonomics. Meilisearch's Go client and Helm-friendly deploy story tip the choice; revisitable if it underperforms.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0004 — Use PostgreSQL as the canonical relational store](0004-use-postgresql-as-the-canonical-relational-store.md)
- Meilisearch: <https://www.meilisearch.com>
