---
id: IMPL-0005
title: "Phase 5 Operator UI: BFF, alerts, review, catalog, runs, decisions"
status: Draft
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0005: Phase 5 Operator UI: BFF, alerts, review, catalog, runs, decisions

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 0: Prerequisites](#phase-0-prerequisites)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 1: Workspace, BFF and shell](#phase-1-workspace-bff-and-shell)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 2: Alerts and the why panel](#phase-2-alerts-and-the-why-panel)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 3: Label form and review queue](#phase-3-label-form-and-review-queue)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 4: Catalog, runs, watches, decisions](#phase-4-catalog-runs-watches-decisions)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 5: Build, image, chart, CI, e2e and deployment](#phase-5-build-image-chart-ci-e2e-and-deployment)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Build the operator UI: a Bun process running Hono that owns the OIDC
session and proxies the API through a write allowlist, and a React SPA
that gives the operator the alerts list and why panel, the registry-driven
label form and review queue, catalog and proposal review, the runs view
over Temporal visibility, watches, the decisions page, and a public status
page. It ships as its own image and chart Deployment and is verified by
`bun test`, Playwright with an axe audit, the generated-type drift check
and the CSP dist check. The UI is in the first release because the review
queue and the label form are how thresholds get set from the first
dev-namespace tick.

**Implements:** RFC-0001 (Delivery phase 5); DESIGN-0007 in full; the
UI-facing parts of DESIGN-0006 (status fields, label and proposal
endpoints, calibration and agreement endpoints); ADR-0007.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- `ui/` workspace: `server/` (Hono BFF), `web/` (Vite React SPA),
  `e2e/` (Playwright and the stack script), `scripts/` (generated-type
  and dist checks, changelog generation).
- BFF: fail-fast configuration, OIDC authorization-code with PKCE as a
  confidential client against the cluster's issuer, sealed session cookie
  with refresh, GET-only proxy with the Origin-checked write allowlist
  from DESIGN-0007, static serving with caching rules and a traversal
  guard, health endpoints, strict CSP.
- SPA: shell (sidebar, top bar, command palette), primitives vendored
  from repo-guardian's UI and re-themed only where the product name
  appears, and every view in DESIGN-0007: `/status`, `/` alerts with the
  drawer and why panel, `/listings/$id`, `/review`, `/catalog` with
  proposals, `/runs`, `/watches`, `/decisions`, `/access`.
- Build and release: Dockerfile on distroless Bun, bake targets, CI jobs
  behind a paths filter, release workflow push of
  `ghcr.io/donaldgifford/spt-ui`, chart `ui:` values and templates,
  deployment to `spt-dev`.
- Tests: `bun test` for the BFF and components, Playwright e2e with axe,
  `check:api` and `check:dist`.

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- API endpoints, their handlers and the OpenAPI document (IMPL-0003);
  this document consumes `api/openapi.yaml` and files gaps against
  IMPL-0003.
- The cluster issuer itself, ingress class and TLS issuance
  (IMPL-0004); this document registers one client and sets values.
- Editing the registry or thresholds from the UI (ADR-0003).
- Multi-user roles, a light theme, live push updates.
- A shared design-system package; primitives are vendored here and a
  later extraction is noted in Open Questions.

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its
tasks are checked off and its success criteria are met. Every task
carries a `verify:` line naming the command or observation that proves
it done.

---

<!--docz:phase:start-->
### Phase 0: Prerequisites

Pin the toolchain, settle the test issuer, register the dev client, and
read the reference UI before writing any of it.

<!--docz:tasks:start-->
#### Tasks

- [ ] Pin `bun` in `mise.toml` and add `ui-install` to `justfile`
  running `bun install --frozen-lockfile` in `ui/`.
  verify: `mise install && just ui-install` succeeds on a clean checkout.
- [ ] Pin the Playwright container image tag used by `ui-e2e` in
  `.github/workflows/ci.yml` and `justfile`, matching the
  `@playwright/test` version in `ui/package.json`.
  verify: `grep -n "mcr.microsoft.com/playwright" .github/workflows/ci.yml justfile` shows one tag that equals the package version.
- [ ] Choose and scaffold the mock OIDC issuer for tests per Open
  Question Q1 (default: `ui/e2e/mock-issuer.ts`, a Hono server that
  serves discovery, JWKS, the authorization and token endpoints, and
  signs with a committed test-only key).
  verify: `bun run ui/e2e/mock-issuer.ts & curl -s localhost:4444/.well-known/openid-configuration | jq .issuer` prints the issuer URL.
- [ ] Register a confidential client `spt-ui` for the dev deployment in
  the cluster issuer with redirect URI `https://spt-dev.<domain>/auth/callback`
  and the operator group claim mapped; store the secret in the spt
  1Password Environment as `UI_OIDC_CLIENT_SECRET`.
  verify: `op run --environment $op_env -- env | grep -c UI_OIDC_CLIENT_SECRET` prints 1.
- [ ] Confirm `api/openapi.yaml` from IMPL-0003 covers every endpoint
  DESIGN-0007 consumes (alerts filters, `listings?stage=`, catalog
  `q=` and `nearest[]`, runs and journal, status fields, calibration and
  agreement); file gaps as tasks against IMPL-0003 before Phase 2.
  verify: `bunx openapi-typescript ../api/openapi.yaml -o /tmp/schema.ts && grep -c "operations\[" /tmp/schema.ts` succeeds and the gap list is empty or filed.
- [ ] Re-read repo-guardian's `ui/server/*.ts`, `ui/web/src/{components,api,shell}`,
  `ui/Dockerfile`, `ui/e2e/stack.ts` and `scripts/` and write the list of
  files to re-create here, with what changes, into `ui/README.md`.
  verify: `test -s ui/README.md` and the list names every file created in
  Phase 1.
- [ ] Add `@fontsource/ibm-plex-sans` and `@fontsource/jetbrains-mono`
  and `@tabler/icons-react` to the planned dependency list; confirm
  the CSP from ADR-0007 (`default-src 'self'; connect-src 'self';
  script-src 'self'; frame-ancestors 'none'`) needs no `data:` or
  `font-src` exception with bundled fonts.
  verify: `grep -n "data:" ui/server/static.ts ui/web/vite.config.ts` finds nothing.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ui-install` is reproducible; `bun --version` matches the pin.
- The mock issuer serves discovery and JWKS and signs a token the BFF
  can verify.
- The dev client exists in the issuer and its secret is in the 1Password
  Environment.
- Every endpoint DESIGN-0007 needs is present in `api/openapi.yaml` or
  filed against IMPL-0003.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 1: Workspace, BFF and shell

The `ui/` workspace, the Bun BFF with session, proxy, static serving and
CSP, and the SPA shell with the two pages that need no business data.

<!--docz:tasks:start-->
#### Tasks

- [ ] `ui/package.json` (workspace scripts `dev`, `build`, `test`,
  `lint`, `typecheck`, `check:api`, `check:dist`, `e2e`),
  `ui/tsconfig.json` (`strict`, `noUncheckedIndexedAccess`,
  `exactOptionalPropertyTypes`, `verbatimModuleSyntax`),
  `ui/eslint.config.js` (typescript-eslint, react-hooks, a rule banning
  `dangerouslySetInnerHTML` and `innerHTML`, zero warnings),
  `ui/web/vite.config.ts` (`assetsInlineLimit: 0`, dev proxy for `/api`,
  `/auth`, `/ui`).
  verify: `cd ui && bun run lint && bun run typecheck` pass on the empty
  workspace.
- [ ] `server/config.ts`: reads `OIDC_ISSUER`, `OIDC_CLIENT_ID`,
  `OIDC_CLIENT_SECRET`, `OIDC_SCOPES`, `OIDC_OPERATOR_GROUP`,
  `API_UPSTREAM`, `PUBLIC_URL`, `UI_SESSION_KEYS`, `UI_SESSION_TTL`,
  `PORT`, `UI_DOCS_URL`, `TEMPORAL_UI_URL`, `LANGFUSE_URL`,
  `STATUS_PUBLIC`; fails at start listing every missing or malformed
  variable together.
  verify: `cd ui && bun test server/config.test.ts` covers missing,
  malformed and complete environments.
- [ ] `server/oidc.ts`: discovery with caching, authorization-code with
  PKCE as a confidential client, token exchange, refresh, JWKS fetch for
  ID-token verification, the operator group claim check.
  verify: `cd ui && bun test server/oidc.test.ts` against the mock
  issuer.
- [ ] `server/session.ts`: sealed cookie (AEAD with `UI_SESSION_KEYS`
  rotation), absolute TTL, `HttpOnly; Secure; SameSite=Lax`, refresh of
  the access token on expiry inside the proxy.
  verify: `cd ui && bun test server/session.test.ts` covers seal,
  unseal, rotation and expiry.
- [ ] `server/auth.ts`: `/auth/login`, `/auth/callback`, `/auth/logout`
  (Origin-checked), `/ui/me` returning subject, groups and whether the
  operator group is present.
  verify: `cd ui && bun test server/auth.test.ts` runs the full login
  round trip with the mock issuer.
- [ ] `server/proxy.ts`: forwards `/api/*` to `API_UPSTREAM` with the
  bearer token; GET-only by default; the allowlist from DESIGN-0007
  (dismiss, restore, label, catalog create and edit, proposal approve and
  reject, watch create, update, enabled, delete, ingest, baselines
  refresh, decide), each entry Origin-checked against `PUBLIC_URL`;
  everything else 405; problem documents passed through unchanged.
  verify: `cd ui && bun test server/proxy.test.ts` has one test per
  allowlist entry (allowed with Origin, 403 without, 405 for any other
  method on any other path).
- [ ] `server/static.ts`: serves `web/dist`; `/assets/*` immutable for
  one year, root files one hour, `index.html` no-cache; `/api/`,
  `/auth/`, `/ui/`, `/assets/` never fall through to the SPA shell; a
  path-traversal guard; CSP, `X-Content-Type-Options: nosniff` and
  `Referrer-Policy: same-origin` on every response.
  verify: `cd ui && bun test server/static.test.ts` covers caching
  headers, fall-through rules, traversal and CSP.
- [ ] `server/health.ts`: `/healthz` (process up) and `/readyz` (issuer
  discovery and `API_UPSTREAM/readyz` reachable, cached 10s).
  verify: `cd ui && bun test server/health.test.ts`.
- [ ] `server/main.ts` wiring with structured request logging
  (`server/log.ts`).
  verify: `cd ui && OIDC_ISSUER=http://localhost:4444 ... bun run server/main.ts` serves `/healthz` 200.
- [ ] SPA shell: TanStack Router with file routes, React Query provider,
  `openapi-fetch` client typed from `web/src/generated/schema.gen.ts`,
  the sidebar from a nav data array (Alerts, Review, Catalog, Runs,
  Watches, Decisions, Status, Docs), top bar, command palette, Tailwind
  v4 `@theme` tokens, `@fontsource` fonts, Tabler icons, radix
  primitives (data table, detail drawer inline above 1600px and overlay
  below, badges, tabs, dialog, dropdown, kpi, meter, stat-strip, filter
  chips, cursor pager) vendored into `web/src/components` and re-themed
  only where the product name appears.
  verify: `cd ui && bun run build` produces `web/dist` with no inline
  scripts or styles (`bun run check:dist` passes).
- [ ] `scripts/check-api.ts`: regenerates `schema.gen.ts` from
  `../api/openapi.yaml` and fails on diff; `scripts/check-dist.ts`:
  fails if `web/dist` contains inline `<script>` or `style=` or `data:`
  URLs.
  verify: `cd ui && bun run check:api && bun run check:dist` pass;
  editing the spec makes `check:api` fail.
- [ ] `/status` view (public): component health, backlog per task queue,
  pending labels and proposals, last tick per watch, eBay quota, the
  changelog card; `/access` view for a signed-in user outside the
  operator group.
  verify: `cd ui && bun test web/src/views/status.test.tsx web/src/views/access.test.tsx` pass with happy-dom.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `cd ui && bun test` passes with at least one test per allowlist entry,
  per config failure mode and per static-serving rule.
- The login round trip works against the mock issuer and against the
  cluster issuer in dev; tokens never appear in browser storage or
  responses.
- `bun run build`, `check:api`, `check:dist`, `lint` and `typecheck`
  are all green and wired into `just ui-lint`.
- `/status` renders against `spt api` with no session.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Alerts and the why panel

The view the operator opens first: the alert list with its drawer, and
the why panel that explains every decision behind a listing.

<!--docz:tasks:start-->
#### Tasks

- [ ] `/` alerts list: filters (watch, type, verdict, score range,
  delivered, dismissed) as chips bound to `GET /api/v1/alerts`
  parameters; keyset paging through the cursor pager; 60s polling and
  refetch on window focus.
  verify: `cd ui && bun test web/src/views/alerts.test.tsx` covers
  filter-to-query mapping and cursor navigation.
- [ ] Alert drawer: listing summary with link to eBay, score and the
  breakdown from `score_breakdown`, verdict and `verdict_p`, delivery
  and dismissal state, Dismiss and Restore actions with optimistic update
  and invalidation.
  verify: `cd ui && bun test web/src/views/alerts/drawer.test.tsx`
  asserts the two mutations hit the allowlisted paths.
- [ ] Why panel component from `GET /api/v1/listings/{id}/decisions`:
  one row per question (question text from the registry, chosen value,
  probability meter, popover with the full distribution, answering
  decider, threshold with green at or above and amber below); derived
  fields and the rendered key with each placeholder's source; resolve
  steps (specifics hit, candidates, chooser distribution,
  `matches_catalog` probability); escalation record; cross-check rows
  with agreement; labels with who, when and what changed.
  verify: `cd ui && bun test web/src/components/why-panel.test.tsx`
  covers the popover, threshold colouring and every section against a
  fixture.
- [ ] Re-ask action: `POST /api/v1/decide` with the listing's state and
  component; fresh answers rendered side by side with the stored ones;
  a 429 or budget problem shown inline.
  verify: `cd ui && bun test web/src/components/why-panel.test.tsx -t reask` asserts the call and the side-by-side render.
- [ ] `/listings/$id` view: the full record across registry versions
  (newest first, older collapsed), the why panel, catalog ref, product
  key, baseline context, links to the Temporal UI and the Langfuse
  session.
  verify: `cd ui && bun test web/src/views/listing.test.tsx`.
- [ ] Playwright: sign in, open an alert, dismiss it, restore it, open
  the why panel popover, open the listing page.
  verify: `cd ui && bun run e2e -- --grep alerts` passes against
  `e2e/stack.ts`.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every decision in a fixture listing is visible with its distribution,
  decider and threshold without leaving the drawer.
- Dismiss and Restore round-trip through the BFF and are reflected on
  the next poll.
- The alerts e2e spec passes with zero serious or critical axe
  violations.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: Label form and review queue

The operator's half of the label loop: a form generated from the
registry, submitted as a signal, and the queue of listings waiting on it.

<!--docz:tasks:start-->
#### Tasks

- [ ] Registry client: `GET /api/v1/registry` cached per
  `schema_version`; a `useRegistry(schemaVersion)` hook.
  verify: `cd ui && bun test web/src/api/registry.test.ts`.
- [ ] Label form generated from the registry: component select; per
  question of that component a select for `choice`, a toggle for `bool`,
  a level picker showing each level's description for `score`;
  prefilled from current decisions; catalog-ref search box backed by
  `GET /api/v1/catalog?component=&q=`; free-text note; client validation
  mirroring the registry (declared option keys, level range, required
  fields).
  verify: `cd ui && bun test web/src/components/label-form.test.tsx`
  generates controls from a fixture registry and rejects an undeclared
  value.
- [ ] Submit: `POST /api/v1/listings/{id}/label`; on `202` poll
  `GET /api/v1/listings/{id}` every 2s for up to 30s until `stage`
  changes; on `200` show "recorded, nothing to resume"; the form is
  available from any alert drawer and any listing page.
  verify: `cd ui && bun test web/src/components/label-form.test.tsx -t submit` covers 202 with stage change, 202 timeout and 200.
- [ ] Registry version mismatch: when the listing's `schema_version`
  differs from the current registry, the form uses the listing's version
  and shows a banner (Open Question Q3).
  verify: `cd ui && bun test web/src/components/label-form.test.tsx -t version` asserts the banner and the version used.
- [ ] `/review` queue: `GET /api/v1/listings?stage=awaiting_label`
  oldest first, the label form inline per row, 30s polling; sidebar
  badge with the count from `GET /api/v1/status`.
  verify: `cd ui && bun test web/src/views/review.test.tsx` and the
  badge test in `web/src/shell/nav.test.tsx`.
- [ ] Playwright: label a listing from the review queue and see its
  stage change; label a finished listing from an alert and see the
  "recorded" message.
  verify: `cd ui && bun run e2e -- --grep label` passes.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Adding a question to the fixture registry adds a control with no UI
  code change.
- A label submitted from the queue moves the listing out of
  `AwaitingLabel` within the poll window in e2e.
- The review badge count matches the queue length in e2e.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: Catalog, runs, watches, decisions

The remaining views: growing the catalog, seeing where work is, managing
watches, and reading calibration and agreement.

<!--docz:tasks:start-->
#### Tasks

- [ ] `/catalog`: rows by component with search (`q=`), keyset paging,
  a row editor (every attribute, aliases as chips) saving through
  `PUT /api/v1/catalog/{id}`, and a merge action that sends `merge_into`.
  verify: `cd ui && bun test web/src/views/catalog.test.tsx` covers
  search, edit and merge payloads.
- [ ] Proposals tab: pending proposals from
  `GET /api/v1/catalog/proposals?status=pending` with the proposed row,
  evidence, originating listing title and specifics, and `nearest[]`
  rows; alias proposals in the same list with a smaller editor; Approve
  opens the editable row and sends
  `POST /api/v1/catalog/proposals/{id}/approve` with the edits; Reject
  sends `/reject` with a reason; sidebar badge from the pending count.
  verify: `cd ui && bun test web/src/views/catalog/proposals.test.tsx`
  asserts both payloads and the nearest-rows render.
- [ ] `/runs`: list from `GET /api/v1/runs` with filters for stage,
  watch, type and registry version; workflow ID, type, stage, started,
  last event, status badge; 30s polling; a journal drawer from
  `GET /api/v1/runs/{id}/journal` polling 15s while open; "Open in
  Temporal" deep link built from `TEMPORAL_UI_URL`.
  verify: `cd ui && bun test web/src/views/runs.test.tsx` covers filters,
  the journal render and the link.
- [ ] `/watches`: list, create and edit form (name, component select from
  the registry, query, filters, threshold), enable and disable through
  `PUT /api/v1/watches/{id}/enabled`, delete with confirmation, Run now
  through `POST /api/v1/ingest` showing the returned workflow ID with a
  link to `/runs`.
  verify: `cd ui && bun test web/src/views/watches.test.tsx` covers every
  mutation path.
- [ ] `/decisions`: calibration chart per question per decider from
  `GET /api/v1/decisions/calibration` (predicted versus observed over
  ten bins with sample counts), agreement per decider pair from
  `GET /api/v1/decisions/agreement`, escalation rate, fallback rate and
  `other` rate over time from `GET /api/v1/decisions/stats`; filters for
  question, decider and period; refetch on filter change only.
  verify: `cd ui && bun test web/src/views/decisions.test.tsx` renders
  each chart from fixtures and maps filters to query parameters.
- [ ] Playwright: approve a proposal with an edit, reject one with a
  reason, open a run journal, create a watch and run it, open the
  decisions page.
  verify: `cd ui && bun run e2e -- --grep "catalog|runs|watches|decisions"` passes.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- A proposal approved with edits appears as a catalog row with those
  edits in e2e, and the originating listing's stage moves.
- "Where is item X" is answerable from `/runs` by workflow ID or filter
  without opening the Temporal UI.
- Every chart on `/decisions` renders from the API with no client-side
  recomputation beyond formatting.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: Build, image, chart, CI, e2e and deployment

Package the UI, gate it in CI, render it from the chart, and deploy it
to the dev namespace.

<!--docz:tasks:start-->
#### Tasks

- [ ] `ui/Dockerfile`: a Bun build stage, a production-dependencies
  stage, and a distroless Bun final image running as user 65532 with
  `BUN_RUNTIME_TRANSPILER_CACHE_PATH=0`, a read-only root filesystem, no
  shell and `HEALTHCHECK NONE`; `VERSION`, `COMMIT`, `DATE` build
  arguments stamped into `/ui/version`.
  verify: `docker buildx bake ui && docker run --rm --read-only ghcr.io/donaldgifford/spt-ui:dev bun --version` succeeds.
- [ ] `docker-bake.hcl` targets `ui`, `ui-ci`, `ui-release` mirroring the
  binary's three targets; `justfile` recipes `ui-install`,
  `generate-ui-api`, `ui-lint` (typecheck, eslint at zero warnings,
  `check:api`, `check:dist`), `ui-test`, `ui-build`, `ui-e2e`,
  `ui-docker-build`.
  verify: `just ui-lint && just ui-test && just ui-build && just ui-docker-build` pass.
- [ ] `e2e/stack.ts`: starts Postgres (testcontainers), the mock issuer,
  `spt api --insecure-no-auth` off, the BFF, seeds a fixture database
  (alerts across stages, an `AwaitingLabel` listing, pending proposals,
  runs with recorded journals, watches, decisions with labels), and
  exposes the base URL to Playwright; teardown on exit.
  verify: `cd ui && bun run e2e` runs the whole suite green locally.
- [ ] Playwright config: the pinned image, a project per viewport
  (1280 and 1920), screenshot diffs stored under `e2e/__screenshots__`,
  an axe audit on every page asserting zero serious or critical
  violations.
  verify: `cd ui && bun run e2e -- --reporter=list | grep -c "axe"` is
  at least the number of pages.
- [ ] `scripts/changelog.ts`: generates `web/src/generated/changelog.json`
  from `charts/spt/CHANGELOG.md` for the status card (Open Question Q6);
  drift-checked in `ui-lint`.
  verify: `cd ui && bun run scripts/changelog.ts && git diff --exit-code web/src/generated/changelog.json` is clean.
- [ ] CI: `ui` job (install, lint, test, build, `check:api`,
  `check:dist`) and `ui-e2e` job (Playwright in the pinned image)
  behind a paths filter on `ui/**` and `api/openapi.yaml`, plus a
  nightly full run (Open Question Q5); the release workflow builds and
  pushes `ghcr.io/donaldgifford/spt-ui` at the tag with cosign and SLSA
  like the binary image.
  verify: `actionlint` passes; a PR touching only `ui/` runs both jobs;
  a PR touching only Go skips them.
- [ ] Chart: `templates/ui.yaml` (Deployment with the environment from
  DESIGN-0007, Service, ServiceAccount, PodDisruptionBudget, probes on
  `/healthz` and `/readyz`, read-only root filesystem, non-root) and
  `templates/ingress-ui.yaml`; `ui:` values (`enabled`, `image`,
  `replicas`, `publicUrl`, `oidc.{issuer, clientId, existingSecret}`,
  `sessionKeysSecret`, `links.{temporalUi, langfuse}`, `ingress`,
  `statusPublic`); `values.schema.json` updated; helm-unittest suites
  for enabled and disabled, ingress on and off, secret references.
  verify: `just helm-test` passes with the new suites.
- [ ] Deploy to `spt-dev`: `ui.enabled: true`, the dev client from Phase
  0, session keys Secret created from the 1Password Environment, ingress
  host set; sign in with the cluster issuer; label a real
  `AwaitingLabel` listing.
  verify: `curl -s https://spt-dev.<domain>/healthz` is 200 and the
  listing's stage changes in `/runs`.
- [ ] Open the PR with a `minor` label (the UI image is a new release
  artifact).
  verify: `gh pr view --json labels -q '.labels[].name'` includes
  `minor`; CI green.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ui-lint`, `just ui-test`, `just ui-build` and `just ui-e2e` are
  green locally and in CI; e2e reports zero serious or critical axe
  violations on every page.
- `ghcr.io/donaldgifford/spt-ui` is published at the release tag with
  the same provenance as the binary image.
- The UI runs in `spt-dev` behind the cluster issuer with the strict
  CSP and no console errors on any view.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `mise.toml`, `justfile` | Modify | bun pin; `ui-*` recipes |
| `ui/package.json`, `ui/bun.lock`, `ui/tsconfig.json`, `ui/eslint.config.js`, `ui/README.md` | Create | workspace and tooling |
| `ui/server/{config,oidc,session,auth,proxy,static,health,log,main,redirect}.ts` and `*.test.ts` | Create | Hono BFF |
| `ui/web/vite.config.ts`, `ui/web/index.html`, `ui/web/src/{main.tsx,router.tsx,app.css}` | Create | SPA entry and tokens |
| `ui/web/src/generated/{schema.gen.ts,changelog.json}` | Create | generated, drift-checked |
| `ui/web/src/api/*` | Create | `openapi-fetch` client, registry hook, query keys |
| `ui/web/src/shell/*` | Create | sidebar, top bar, command palette, nav data |
| `ui/web/src/components/*` | Create | vendored primitives, why panel, label form, proposal editor, probability meter, distribution popover, charts wrappers |
| `ui/web/src/views/{status,access,alerts,alerts/drawer,listing,review,catalog,catalog/proposals,runs,runs/drawer,watches,decisions}.tsx` and tests | Create | views |
| `ui/scripts/{check-api,check-dist,changelog}.ts` | Create | build checks |
| `ui/e2e/{stack.ts,mock-issuer.ts,playwright.config.ts,specs/*.spec.ts,fixtures/*}` | Create | e2e harness |
| `ui/Dockerfile`, `docker-bake.hcl` | Create/Modify | UI image and bake targets |
| `.github/workflows/ci.yml`, `.github/workflows/release.yml` | Modify | `ui` and `ui-e2e` jobs; UI image publish |
| `charts/spt/templates/{ui.yaml,ingress-ui.yaml}`, `values.yaml`, `values.schema.json`, `tests/ui_test.yaml` | Create/Modify | UI Deployment, Service, PDB, Ingress, values, suites |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] `bun test` on `ui/server/*.test.ts`: one test per allowlist entry
  (allowed with Origin, 403 without, 405 otherwise), config failure
  modes, session seal and rotation, static caching and traversal, CSP
  headers, health.
- [ ] `bun test` with happy-dom and Testing Library on components and
  views, the DOM registrator imported before anything that loads radix:
  why panel, label form, proposal editor, each view's query mapping and
  mutations.
- [ ] Playwright e2e against `e2e/stack.ts` with the mock issuer and the
  seeded fixture database: sign in, alerts, label, catalog, runs,
  watches, decisions; screenshot diffs; axe audit on every page.
- [ ] `check:api` (generated types match `api/openapi.yaml`) and
  `check:dist` (no inline script, style or `data:` in the build) in
  `just ui-lint` and CI.
- [ ] helm-unittest suites for the UI templates; `ct lint`.

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- IMPL-0003: `api/openapi.yaml` and the api role serving every endpoint
  DESIGN-0007 consumes, including `--insecure-no-auth` for the e2e
  stack and the problem-document shapes the BFF passes through.
- IMPL-0004: the `spt-dev` release, ingress class, TLS and the cluster
  issuer with the `spt-ui` confidential client; the Temporal UI and
  Langfuse URLs for deep links.
- IMPL-0001: the chart's value conventions, bake file, CI layout and
  the `op run` convention for local secrets.
- Bun pinned in `mise.toml`; the Playwright container image; Docker for
  the e2e stack's Postgres.
- Library choices: React 19, Vite, TanStack Router and Query, Tailwind
  v4, radix primitives, recharts, `openapi-fetch` and
  `openapi-typescript`, Hono, `@fontsource` fonts, Tabler icons,
  Playwright with `@axe-core/playwright`.

<!--docz:dependencies:end-->

## Open Questions

**Q1. Mock OIDC issuer for tests.**
a. A small Hono server in `ui/e2e/mock-issuer.ts` that serves discovery,
   JWKS, authorization and token endpoints and signs with a committed
   test-only key; starts in milliseconds inside `stack.ts` and needs no
   container.
b. A containerised issuer (Dex or Keycloak) in the e2e stack; closer to
   production behaviour, slower to start and one more image to pin.
Other: ____

**Q2. Charts library.**
a. recharts, as the reference UI uses; the fleet's chart wrappers and
   tokens carry over unchanged.
b. visx; lower-level and smaller bundles, but every chart is written
   from primitives and the wrappers are new.
Other: ____

**Q3. Label form when the registry is newer than the listing.**
a. Generate the form from the listing's own `schema_version` (the API
   serves any version it has decisions for) and show a banner noting the
   current version; the label stays comparable to the decisions it
   corrects.
b. Always generate from the current registry; simpler, but a label can
   reference questions the listing was never asked.
Other: ____

**Q4. Theme.**
a. Dark-only, matching the reference UI and ADR-0007's non-goal; one
   token set to maintain.
b. Add a light theme now through a second `@theme` block and a toggle.
Other: ____

**Q5. When e2e runs.**
a. On PRs behind a paths filter (`ui/**`, `api/openapi.yaml`) plus a
   nightly full run on `main`, so Go-only PRs stay fast and drift is
   still caught daily.
b. On every PR regardless of paths.
Other: ____

**Q6. UI changelog source.**
a. Generated from `charts/spt/CHANGELOG.md` by `scripts/changelog.ts`
   into `web/src/generated/changelog.json`, drift-checked, so the status
   card tracks releases without a second changelog.
b. A hand-maintained `ui/CHANGELOG.md` rendered directly.
Other: ____

**Q7. Primitives: vendor now, extract later?**
a. Vendor the shell and primitives into `ui/web/src/components` now and
   open an issue to extract a shared package once both UIs have diverged
   enough to show what is common.
b. Extract `@donaldgifford/ui-primitives` first and consume it from both
   UIs before building spt's views.
Other: ____

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [ADR-0006 The API is a spec-first OpenAPI contract served on net/http](../adr/0006-the-api-is-a-spec-first-openapi-contract-served-on-nethttp.md); [ADR-0007 The operator UI is a React SPA behind a Bun BFF](../adr/0007-the-operator-ui-is-a-react-spa-behind-a-bun-bff.md)
- [DESIGN-0007 Operator UI](../design/0007-operator-ui-alerts-review-catalog-and-runs.md); [DESIGN-0006 API](../design/0006-api-the-openapi-contract-authentication-and-clients.md); [DESIGN-0003 Decisions](../design/0003-decisions-the-decider-contract-cascade-escalation-and.md) (why panel data, agreement); [DESIGN-0004 Registry](../design/0004-registry-the-domain-schema-as-an-embedded-hcl-file.md) (label form); [DESIGN-0005 Catalog](../design/0005-catalog-canonical-identities-retrieval-and-proposals.md) (proposals, nearest rows, merge)
- [IMPL-0001 Foundation](../impl/0001-phase-1-foundation-roles-configuration-store-ebay-client.md); [IMPL-0003 Durable execution](../impl/0003-phase-3-durable-execution-temporal-plumbing-workflows.md); [IMPL-0004 Integration](../impl/0004-phase-4-integration-cluster-dev-namespace-live-providers-side.md)
- References from repo-guardian (re-read, not imported): [`ui/`](https://github.com/donaldgifford/repo-guardian/tree/main/ui), [DESIGN-0027 v2 read-only API, business UI and status page](https://github.com/donaldgifford/repo-guardian/blob/main/docs/design/0027-v2-read-only-api-business-ui-and-status-page.md), [DESIGN-0036 Frontend design cleanup and refresh](https://github.com/donaldgifford/repo-guardian/blob/main/docs/design/0036-frontend-design-cleanup-and-refresh-for-the-v2-ui.md), [DESIGN-0037 Controls UI](https://github.com/donaldgifford/repo-guardian/blob/main/docs/design/0037-controls-ui-org-tree-findings-fleet-panels-and-status-changelog.md)
- [Hono](https://hono.dev); [Bun](https://bun.sh); [TanStack Router](https://tanstack.com/router); [TanStack Query](https://tanstack.com/query); [openapi-typescript](https://openapi-ts.dev); [Playwright](https://playwright.dev); [axe-core for Playwright](https://github.com/dequelabs/axe-core-npm/tree/develop/packages/playwright); [Tailwind v4 theme](https://tailwindcss.com/docs/theme)

<!--docz:references:end-->
