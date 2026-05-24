---
id: ADR-0007
title: "Package and deploy via Docker and a Helm chart"
status: Proposed
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0007. Package and deploy via Docker and a Helm chart

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

spt is intended to be self-hosted by its users. They run the range from "single VPS with Docker Compose" to "production Kubernetes cluster." We need a packaging story that covers both without duplicating release pipelines.

The backend is a single Go binary that selects its role at startup ([ADR-0001](0001-use-go-for-the-backend.md)), and depends on Postgres ([ADR-0004](0004-use-postgresql-as-the-canonical-relational-store.md)), Valkey ([ADR-0005](0005-use-valkey-for-queues-and-caching.md)), Meilisearch ([ADR-0006](0006-use-meilisearch-for-listing-search.md)), and optionally ClickHouse + Langfuse ([ADR-0008](0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)).

## Decision

We will package spt as:

1. A **multi-arch Docker image** (`linux/amd64` + `linux/arm64`) built via `docker buildx bake`, published to GHCR on tagged releases. Distroless base. Already scaffolded in `Dockerfile` and `docker-bake.hcl`.
2. A **Helm chart** published alongside each release. Defaults are conservative (single replica running all roles in one pod); production-shaped values enable per-role Deployments (`api`, `scheduler`, `worker`) and external Postgres/Valkey/Meilisearch.

For local dev and small-scale self-hosting, a Docker Compose example file is provided but not the primary deployment target.

## Consequences

### Positive

- One image, multiple roles → minimal release surface area. Roles scale independently via separate Deployments backed by the same image.
- Helm covers the realistic self-hosted production target without forcing it on users who don't need it.
- CI already produces the right artifacts (`.github/workflows/ci.yml` and the bake file are wired).
- Distroless + nonroot user gives us a good security baseline by default.

### Negative

- Helm charts are an ongoing maintenance burden — values surface, schema, upgrade paths.
- Multi-arch builds (specifically arm64 via QEMU) are slow in CI; we've already split the pipeline (linux/amd64 only on PRs, multi-arch on release) to keep PR feedback fast.

### Neutral

- We commit to publishing OCI images to GHCR; this is conventional and easy to mirror.

## Alternatives Considered

- **Binary releases only** — viable for power users via `goreleaser` (already configured), but doesn't meet the K8s self-host requirement.
- **Operator pattern (CRDs + controller)** — overkill for v1; a Helm chart covers the same ground with far less ceremony. Revisitable if multi-tenant or fleet-scale management becomes a requirement.
- **Nix flake / OCI distroless via Bazel** — interesting but high learning-curve cost for marginal benefit at this stage.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0001 — Use Go for the backend](0001-use-go-for-the-backend.md)
- `Dockerfile`, `docker-bake.hcl`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`
