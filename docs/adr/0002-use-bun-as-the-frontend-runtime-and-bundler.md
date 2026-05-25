---
id: ADR-0002
title: "Use Bun as the frontend runtime and bundler"
status: Proposed
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0002. Use Bun as the frontend runtime and bundler

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

The frontend ([RFC-0001](../rfc/0001-server-price-tracker-platform.md)) is a React Router single-page app that wraps the backend API. We need a JS runtime + package manager + bundler that:

- Builds quickly enough that the dev loop stays tight.
- Has a single canonical CLI rather than the Node + npm/yarn/pnpm + Vite/Webpack pile.
- Plays nicely with generated TypeScript clients ([ADR-0010](0010-generate-the-frontend-api-client-from-openapi.md)) and shadcn/ui ([ADR-0003](0003-use-shadcnui-with-recharts-for-the-web-ui.md)).
- Doesn't add operational complexity at deploy time — the production artifact is a static bundle served behind the API.

## Decision

We will use **Bun** as the frontend runtime, package manager, and bundler. The frontend is a Bun + React Router project; production builds emit a static asset bundle that the Go backend serves (or fronts via CDN).

## Consequences

### Positive

- Single CLI for install, run, test, and build — fewer moving parts than Node + pnpm + Vite.
- Native TypeScript execution; no separate transpile step in dev.
- Fast install and fast bundling shorten the dev loop.
- First-class workspace support if/when the frontend splits into packages.

### Negative

- Smaller ecosystem maturity than Node; occasional library incompatibilities require fallbacks.
- Less battle-tested at scale than Node; production deployment risk is low because we ship a static bundle (Bun isn't in the request path at runtime).
- Team Node-native muscle memory takes a small adjustment.

### Neutral

- Lockfile is `bun.lockb` (binary); review tooling needs to handle it.

## Alternatives Considered

- **Node + pnpm + Vite** — the conservative choice. Strong ecosystem and tooling, but more pieces to manage and a slower dev loop.
- **Deno** — comparable single-CLI ergonomics but a smaller React ecosystem and a different import model that complicates shadcn integration.
- **Node + npm + Webpack** — incumbent default; rejected for build speed and config complexity.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0003 — Use shadcn/ui with Recharts for the web UI](0003-use-shadcnui-with-recharts-for-the-web-ui.md)
- [ADR-0010 — Generate the frontend API client from OpenAPI](0010-generate-the-frontend-api-client-from-openapi.md)
- Bun: <https://bun.sh>
