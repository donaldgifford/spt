---
id: ADR-0001
title: "Use Go for the backend"
status: Proposed
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0001. Use Go for the backend

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

spt's backend is a service-oriented system: API server, scheduler, workers, all sharing a domain model and clients (eBay, Postgres, Valkey, Meilisearch, ClickHouse). RFC-0001 calls for a single binary that selects its role at startup, embeds cleanly into a container image, and instruments itself with OTel and Prometheus.

We also have a strong preference for the standard library over heavy frameworks — `net/http`, `database/sql` + `pgx`, `log/slog`, hand-wired OTel and metrics.

## Decision

We will write the backend in **Go**, targeting the version pinned in `go.mod` and `mise.toml`. The build artifact is a single static binary (`./cmd/spt`) that selects its role via subcommand (`spt api`, `spt scheduler`, `spt worker`).

## Consequences

### Positive

- Strong fit for the std-lib-first preference; `net/http`, `database/sql`, `log/slog`, and `context` cover most of what we need.
- Single static binary with trivial cross-compilation (linux/darwin × amd64/arm64) — already wired in `.goreleaser.yml`.
- Strong concurrency primitives for the scheduler and worker roles.
- First-class OTel and Prometheus client libraries.
- Existing repo scaffolding (golangci-lint, goreleaser, docker bake, just recipes) is Go-shaped.

### Negative

- Slower iteration on data-shape changes than a dynamic language; accepted in exchange for type safety.
- Generics are usable but conservative — some patterns from other languages don't translate directly.

### Neutral

- Commits us to the Uber-style lint posture already configured in `.golangci.yml`.

## Alternatives Considered

- **Rust** — strong systems-language fit, but the eBay-polling + analytics workload doesn't need its safety guarantees, and iteration cost on a small team is significant.
- **TypeScript / Node** — sharing language with the frontend is appealing, but operational footprint (process management, memory) and instrumentation maturity for an agentic backend lag behind Go.
- **Python** — best agentic-library ecosystem, but weak on the scheduler/worker concurrency model and packaging is painful.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
