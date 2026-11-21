---
id: ADR-0007
title: "The operator UI is a React SPA behind a Bun BFF"
status: Proposed
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0007: The operator UI is a React SPA behind a Bun BFF

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Summary](#summary)
- [Context](#context)
- [Decision](#decision)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:summary:start-->
## Summary

The operator UI is a React single-page application served by a Bun
process running Hono that owns the OIDC session, proxies the API with an
explicit write allowlist, and serves static assets under a strict content
security policy. Not HTML over the wire from the Go binary, because the
views are forms, drawers and charts that need a component model and
because the API accepts writes and therefore needs real authentication.
Not a Go-embedded SPA, because the session and proxy logic already exist
in TypeScript on this cluster and rewriting them in Go would fork the
shape. It ships as its own image in the design language of
repo-guardian's UI.

<!--docz:summary:end-->

<!--docz:context:start-->
## Context

The operator's half of the label loop is a small application: an alert
list with filters and a detail drawer; a why panel that shows every
decision's probability distribution; a label form whose controls are
generated from the registry; a review queue; a catalog page with proposal
approval and row editing; a runs view over Temporal visibility;
calibration charts; a public status page.

The forces:

1. **Authentication is mandatory.** The API accepts writes (labels,
   approvals, watch edits). Tokens must not live in the browser, writes
   must be confined to a known list, and the public status page must be
   the only unauthenticated route.
2. **Client-side state and components.** A registry-driven form, a
   distribution popover, an editable proposal row and a recharts panel
   are component work, not page renders.
3. **Typed from the contract.** View code should be generated from the
   OpenAPI document (ADR-0006) and fail CI on drift.
4. **Strict CSP.** No inline scripts, no inline styles, no `data:`
   sources, enforced by a build check, because the UI renders
   seller-written text.
5. **One family of UIs.** repo-guardian's UI runs on the same cluster for
   the same operator. Same shell, tokens, primitives, auth flow, CSP,
   tests and Dockerfile means one set of habits and one place to fix a
   bug.
6. **Small operational cost.** One operator, no SSR needs, polling is
   enough. The runtime should be a single small image on a distroless
   base with a read-only root filesystem.

<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

- The UI lives in `ui/`: a Hono BFF under `server/`, a Vite React SPA
  under `web/`, Playwright under `e2e/`, and build checks under
  `scripts/`.
- The BFF does OIDC authorization-code with PKCE as a confidential
  client, holds tokens in a sealed cookie with an absolute TTL, refreshes
  the access token and forwards it as a bearer to the Go API. The proxy is
  GET-only except an Origin-checked allowlist, one entry per write.
- The SPA is React 19, TanStack Router and Query, Tailwind v4 with the
  shared token set, radix primitives, recharts, and an `openapi-fetch`
  client typed from `api/openapi.yaml`.
- CSP is `default-src 'self'; connect-src 'self'; script-src 'self';
  frame-ancestors 'none'`; nothing is inlined; a build check enforces it.
- The image is `ghcr.io/donaldgifford/spt-ui` on distroless Bun, with its
  own Deployment, Service and Ingress under the chart's `ui:` values.
- Live data is polling with React Query. The views are specified in
  DESIGN-0007.

<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- Authentication, CSP, caching, proxy hardening, e2e and accessibility
  tests arrive as a shape with a production history rather than a design
  exercise.
- One design language and component set across the operator's UIs.
- The Go binary stays an API and a worker; no template toolchain.
- Views are typed end to end from the contract.

<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- A second image, Deployment, Ingress and CI pipeline.
- Every write is work in two places (proxy allowlist and Go handler). This
  friction is deliberate.
- Form components (registry-driven label form, proposal editor) are new.
- An OIDC issuer is needed even locally; a mock issuer covers tests and
  development.

<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- The UI changelog and status-page conventions are adopted as they are.
- If `@donaldgifford/design-system` later absorbs these tokens and
  primitives, both UIs switch together.

<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **HTML over the wire from the Go binary (templ and HTMX).** The first
  implementation's choice: about 440 lines of templ covering list,
  detail, dismiss, restore and retry, with no authentication. Smallest
  footprint and one image. It fails force 1 outright (an OIDC
  authorization-code flow, session sealing and refresh would be written in
  Go from scratch) and force 2 (a registry-generated form with live
  validation, a distribution popover and charts are client state that
  HTMX fragments handle by round-tripping to the server). Force 5 fails
  too: the design language would be re-created in templ.
- **SPA embedded in the Go binary with `embed.FS`, OIDC in Go.** One
  image and one process. The session cookie sealing, token refresh,
  Origin-checked allowlist, static caching and CSP would be rewritten in
  Go, diverging from the TypeScript implementation that already runs and
  is tested on the cluster. The Go binary would also need to serve a
  browser session, which it does not need to know about otherwise.
- **Server-rendered React (Next.js, Remix, TanStack Start).** SSR,
  streaming and route loaders for a UI that polls a dozen endpoints for
  one operator. A heavier runtime, a framework-shaped BFF, and the CSP
  story is harder because these frameworks inline by default. Nothing in
  force 2 needs SSR.
- **Svelte or Vue instead of React.** Smaller bundles. Force 5 decides
  it: the primitives, data table, drawer, charts and auth shell exist in
  React, and a second framework in the fleet costs more than the bundle
  size saves.
- **Node and Express, or Bun and Elysia, for the BFF.** Express needs a
  build step for TypeScript and a Node compatibility layer for the Web
  `Request` and `Response` types Hono uses natively. Elysia is Bun-only
  and would differ from the running reference. Bun runs TypeScript
  directly, installs fast, ships as a distroless image, and is already
  pinned in `mise.toml` across the fleet. Hono is small, uses web-standard
  types, and is the BFF already in production.
- **Build on `@donaldgifford/design-system`.** A smaller primitive set
  with no production consumer, and a registry token to install. Revisit
  when it covers what repo-guardian's primitives cover.
- **Streaming updates (SSE or websockets).** Nothing in the operator loop
  needs sub-minute latency; polling is simpler, works through the proxy
  unchanged, and matches the reference.

What tipped it: force 1 and force 5 together. The write surface makes
authentication non-negotiable, and the only implementation of that
authentication the operator already runs is the Bun BFF.

<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [ADR-0006 The API is a spec-first OpenAPI contract served on net/http](../adr/0006-the-api-is-a-spec-first-openapi-contract-served-on-nethttp.md)
- [DESIGN-0007 Operator UI: alerts, review, catalog and runs](../design/0007-operator-ui-alerts-review-catalog-and-runs.md)
- [repo-guardian INV-0009 Read-only status API and web UI options](https://github.com/donaldgifford/repo-guardian/blob/main/docs/investigation/0009-read-only-status-api-and-web-ui-options.md) (the options evaluated the first time)
- [repo-guardian DESIGN-0027 v2 read-only API, business UI and status page](https://github.com/donaldgifford/repo-guardian/blob/main/docs/design/0027-v2-read-only-api-business-ui-and-status-page.md); [DESIGN-0036 Frontend design cleanup and refresh for the v2 UI](https://github.com/donaldgifford/repo-guardian/blob/main/docs/design/0036-frontend-design-cleanup-and-refresh-for-the-v2-ui.md)
- [repo-guardian `ui/`](https://github.com/donaldgifford/repo-guardian/tree/main/ui) (`server/*.ts` BFF, `web/src`, `Dockerfile`, `package.json`)
- [server-price-tracker `internal/api/web`](https://github.com/donaldgifford/server-price-tracker/tree/main/internal/api/web) (the templ and HTMX UI)
- [Hono](https://hono.dev); [Bun](https://bun.sh); [TanStack Router](https://tanstack.com/router); [openapi-typescript](https://openapi-ts.dev)

<!--docz:references:end-->
