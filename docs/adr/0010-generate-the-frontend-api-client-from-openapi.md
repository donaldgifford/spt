---
id: ADR-0010
title: "Generate the frontend API client from OpenAPI"
status: Proposed
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0010. Generate the frontend API client from OpenAPI

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

The UI is a thin wrapper around the backend API ([RFC-0001](../rfc/0001-server-price-tracker-platform.md)). Two failure modes to avoid:

1. **API drift** — frontend code hand-written against an API shape that's since changed; bugs surface only at runtime.
2. **Duplicated type definitions** — the same response shape declared twice (once in Go, once in TypeScript), with no compiler enforcement that they agree.

We need a single source of truth for the API contract that both sides consume, with type-safe TypeScript clients generated for the frontend.

## Decision

The backend will publish an **OpenAPI 3.x** spec for its HTTP API. The TypeScript client and request/response types for the frontend are **generated from that spec** as part of the frontend build.

The Go side authors the spec. The exact authoring approach (handwritten YAML, code-first via `huma` / `oapi-codegen`'s server side, or annotation-based) is deferred to a follow-up DESIGN doc; the contract here is "OpenAPI is the boundary."

Generator candidates for the TypeScript side: `openapi-typescript` (types only), `openapi-fetch` (typed fetch wrapper), or `orval` (typed client + React Query hooks). Selection deferred to the same DESIGN doc.

## Consequences

### Positive

- Type-safe frontend calls; renaming a Go field that breaks the contract surfaces as a TypeScript compile error in CI.
- One source of truth for API shape — no manual sync between Go structs and TypeScript interfaces.
- OpenAPI spec doubles as machine-readable API documentation.
- Backwards-compat checks (e.g., `oasdiff`) become feasible.

### Negative

- Codegen step in the frontend build; adds tooling and a CI gate that the spec is in-tree and current.
- OpenAPI authoring has friction; if we go code-first we accept a generator dependency, if we go spec-first we accept handwriting effort.

### Neutral

- Commits us to OpenAPI-compatible API design (no GraphQL, no gRPC at the public boundary).

## Alternatives Considered

- **Hand-written TS types** — fastest to start, lowest type safety guarantee. The bug class it permits (silent drift) is exactly what we want to prevent.
- **tRPC** — superb DX when both sides are TypeScript; not applicable with a Go backend.
- **gRPC + gRPC-Web** — strong contract, generated clients, but the browser-side story is worse and we lose easy human-readable API browsing.
- **GraphQL** — overkill for our API shape (CRUD-ish over watches + listings + market signals); adds a server-side resolver layer for no real gain.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0002 — Use Bun as the frontend runtime and bundler](0002-use-bun-as-the-frontend-runtime-and-bundler.md)
- OpenAPI: <https://www.openapis.org>
