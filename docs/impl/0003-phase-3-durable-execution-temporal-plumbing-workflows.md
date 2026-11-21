---
id: IMPL-0003
title: "Phase 3 Durable execution: Temporal plumbing, workflows, activities, API role, CLI"
status: Draft
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0003: Phase 3 Durable execution: Temporal plumbing, workflows, activities, API role, CLI

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
  - [Phase 1: Temporal plumbing](#phase-1-temporal-plumbing)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 2: Workflows and activities](#phase-2-workflows-and-activities)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 3: API role](#phase-3-api-role)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 4: CLI client](#phase-4-cli-client)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 5: Integration and hardening](#phase-5-integration-and-hardening)
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

Put the pipeline on Temporal and expose it through the API and the CLI.
This document delivers `internal/temporal` (client, worker, versioning,
schedules, search attributes, test harness), the four workflow types and
every activity from DESIGN-0002 with their single options table, the
OpenAPI contract and the `spt api` role from DESIGN-0006, and the `spt`
client verbs. At the end a full tick runs against the mock eBay server
and mock deciders on the Temporal CLI dev server, a label sent through
the API resumes a waiting listing, and every workflow replays from a
checked-in history in CI. Nothing here needs a route to the cluster.

**Implements:** RFC-0001 (Delivery phase 3); DESIGN-0002 in full;
DESIGN-0006 in full; the CLI tree from DESIGN-0001; the Temporal
plumbing deferred from IMPL-0001.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- `internal/temporal`: `ConfigFromEnv`, `Dial` (mTLS and OIDC
  client-credentials), server-version check, `NewWorker` with per-queue
  options from the `queue` blocks, deployment versioning with
  `PromoteBuild` and `RequireCurrentVersion`, `EnsureSchedule`,
  `RemoveSchedule`, `EnsureSearchAttributes`, `DescribeBacklog`, the slog
  adapter, the metrics handler, the tracing interceptor,
  `temporaltest.Start(t)` and the env-gated history recorder.
- `internal/workflows`: `names`, `options.go`, `register.go`,
  `IngestWorkflow`, `ExtractWorkflow`, `BaselineWorkflow`,
  `CatalogProposalWorkflow`, `CatalogDigestWorkflow`, replay tests.
- `internal/activities`: one struct per group (ebay, decide, resolve,
  persist, score, notify, escalate, catalog) wrapping the IMPL-0001 and
  IMPL-0002 packages, with error-type mapping, heartbeats and budget and
  quota checks.
- `api/openapi.yaml`, `api/oapi-codegen.yaml`, `api/vacuum.yaml`;
  `internal/api` (server, handlers per resource, problems, cursors,
  authn, authz, middleware, limiter); the `spt api` role wired to the
  store and the Temporal client by workflow name.
- `internal/api/client` generated from the spec; every client verb in
  the DESIGN-0001 subcommand tree; the OIDC device flow and token
  storage.
- Integration tests on the dev server and testcontainers Postgres; the
  versioning test; contract tests; the chart's `worker.queues` split.

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Any model call beyond the mock and recorded fixtures (the real
  deciders exist from IMPL-0002 and are wired, but no live key is used
  here).
- The cluster: namespace, mTLS material, OIDC issuer, real eBay
  (IMPL-0004). The mock OIDC issuer from the test harness stands in.
- The UI and its BFF (IMPL-0005). The `schema.gen.ts` drift check is
  wired here so IMPL-0005 inherits it.
- Advanced visibility, batch-per-child workflows, Temporal Updates or
  Nexus (DESIGN-0002 non-goals).

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

Pin the Temporal toolchain and confirm the cluster facts that the
plumbing assumes. Nothing here produces product code.

<!--docz:tasks:start-->
#### Tasks

- [ ] Pin the Temporal CLI in `mise.toml` and prove the dev server starts
  with an in-memory SQLite store and a fresh namespace.
  verify: `temporal server start-dev --headless --namespace spt-test`
  reaches `Server: localhost:7233` within 10s with no database file.
- [ ] Pin `go.temporal.io/sdk` and `go.temporal.io/sdk/contrib/opentelemetry`
  at the minor the cluster's server is tested with (recorded from
  repo-guardian's `go.mod` and the homelab Temporal release); record both
  in Dependencies.
  verify: `go list -m go.temporal.io/sdk` prints the pinned version;
  `go build ./...` succeeds.
- [ ] Confirm the cluster Temporal server version and that worker
  deployment versioning is enabled on the frontend (the dynamic config
  keys for worker versioning), from the homelab repository or
  `temporal operator cluster describe` through a port-forward; record
  the minimum server version for `CheckServerVersion`.
  verify: `temporal operator cluster describe --address <cluster>` (or
  the homelab values file) shows a version at or above the pin.
- [ ] Confirm the IMPL-0001 and IMPL-0002 mocks exist and regenerate
  them: `StoreMock`, `DeciderMock`, `EscalatorMock`, `EbayClientMock`,
  `NotifierMock`.
  verify: `just mocks && git diff --exit-code '**/mocks'` is clean for
  the store, decider, eBay and notifier mocks.
- [ ] Pin `oapi-codegen` and `vacuum` in `mise.toml`; add
  `just generate-api` and `just lint-api` recipes that fail on a missing
  binary.
  verify: `mise ls --current | grep -E 'oapi-codegen|vacuum'` shows both;
  `just lint-api` runs against an empty-but-valid spec stub.
- [ ] Add a mock OIDC issuer to the test harness (`internal/testutil/
  oidc`) that serves discovery and a JWKS and mints RS256 and ES256
  tokens with configurable claims.
  verify: `go test ./internal/testutil/oidc` mints a token the verifier
  accepts and rejects an expired one.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `temporaltest.Start(t)` is usable from any package test in under 3s
  warm and the dev server is pinned in `mise.toml`.
- The SDK version, the cluster server version and the minimum version
  constant are recorded in Dependencies.
- `just mocks`, `just generate-api` and `just lint-api` exist and run.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 1: Temporal plumbing

`internal/temporal` as DESIGN-0002 specifies it: everything the worker
and the api role need to dial, register, version, schedule and observe,
with no workflow code in it.

<!--docz:tasks:start-->
#### Tasks

- [ ] `ConfigFromEnv`: reads `TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`,
  `TEMPORAL_TASK_QUEUE_PREFIX` (default `spt`), `TEMPORAL_TLS_*`,
  `TEMPORAL_OIDC_*`, `TEMPORAL_BUILD_ID`; validates that TLS on means
  exactly one of mTLS or OIDC is configured; `QueueName(q)` returns
  `<prefix>-<q>`.
  verify: `go test ./internal/temporal -run ConfigFromEnv` covers every
  combination, including the invalid both-and-neither cases.
- [ ] `Dial(ctx, cfg, opts)`: `client.Dial` with TLS, an OIDC
  client-credentials `HeadersProvider` that caches the bearer and
  refreshes before expiry (single-flight), the slog adapter, the metrics
  handler and the tracing interceptor; then `CheckServerVersion` against
  the Phase 0 minimum.
  verify: `go test ./internal/temporal -run Dial` against
  `temporaltest.Start(t)`; the headers provider is tested with a fake
  token endpoint for refresh and failure.
- [ ] `NewWorker(client, queue, cfg.Queues[q], opts)`: applies
  `TaskQueueActivitiesPerSecond` or `MaxConcurrentActivityExecutionSize`
  from the `queue` block, `WorkerStopTimeout`, deployment versioning
  (`UseVersioning`, build ID from `TEMPORAL_BUILD_ID`, pinned default
  behaviour), `Identity` as `<POD_NAME>@<build>`, workflow polling only
  on `default`.
  verify: `go test ./internal/temporal -run NewWorker` asserts the
  options struct per queue from the DESIGN-0001 example config.
- [ ] `PromoteBuild(ctx, client, deployment, build)`: describes the
  deployment, compares semver, sets current with the conflict token,
  no-ops when already current or older.
  verify: `go test -tags integration ./internal/temporal -run Promote`
  promotes `1.2.0` over `1.1.0` and refuses `1.0.0`.
- [ ] `RequireCurrentVersion` as a health check and `Ping` as a second.
  verify: `go test -tags integration ./internal/temporal -run Health`
  flips from failing to passing after promotion.
- [ ] `EnsureSchedule(ctx, client, spec)` and `RemoveSchedule`: interval
  with jitter, cron for the digest, overlap `SKIP`, catch-up window,
  paused flag, idempotent create-or-update by ID.
  verify: `go test -tags integration ./internal/temporal -run Schedule`
  creates, updates the interval, pauses and removes `watch:test`.
- [ ] `EnsureSearchAttributes` adding only missing keys (`SptStage`,
  `SptWatch`, `SptType`, `SptSchemaVersion`, `SptComponentKey`) through
  the operator service.
  verify: `go test -tags integration ./internal/temporal -run SearchAttributes`
  is idempotent on a second call.
- [ ] `DescribeBacklog(ctx, client, queues)` with
  `DescribeTaskQueueEnhanced`, returning per-queue backlog counts for the
  status endpoint.
  verify: `go test -tags integration ./internal/temporal -run Backlog`
  sees a non-zero count after scheduling an activity with no worker.
- [ ] `temporaltest.Start(t)`: one dev server per test binary (cached
  across tests with `sync.Once`), SQLite in memory, a namespace per test,
  a client with the slog adapter, cleanup on `t.Cleanup`.
  verify: `go test -tags integration ./internal/temporal -count=1 -v`
  shows one server start for the package.
- [ ] History recorder: when `SPT_RECORD_HISTORIES=1`, integration tests
  export each completed workflow's history as JSON to
  `testdata/histories/<WorkflowType>/<test-name>.json`.
  verify: `SPT_RECORD_HISTORIES=1 go test -tags integration ./internal/workflows -run TestIngest`
  writes a file; without the variable it writes nothing.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `go test -tags integration ./internal/temporal` passes on the dev
  server with no other services; unit coverage at or above 80%.
- Every function in the DESIGN-0002 plumbing table exists with the
  stated behaviour, and `internal/temporal` imports nothing from
  `internal/workflows`, `internal/activities` or `internal/api`.
- `TEMPORAL_*` handling matches the DESIGN-0001 environment table exactly
  (a test enumerates the variable names).

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Workflows and activities

The four workflow types, every activity, the options table, and the
replay tests, all under the SDK test suite with mocks.

<!--docz:tasks:start-->
#### Tasks

- [ ] `internal/workflows/names`: string constants for workflow types,
  signal names (`operator-label`, `catalog-approve`, `catalog-reject`,
  `catalog-resolved`, `cancel-tick`), query names (`stage`, `progress`),
  task queues, search-attribute keys and ID builders (`IngestID`,
  `ExtractID`, `BaselineID`, `ProposalID`, `DigestID`).
  verify: `go test ./internal/workflows/names` asserts every ID format
  from the DESIGN-0002 table; the depguard rule still passes with
  `internal/api` importing only this package.
- [ ] `internal/workflows/options.go`: the activity options table with
  one entry per row of the DESIGN-0002 table (queue, start-to-close,
  heartbeat, retry max and backoff, non-retryable types), and
  `ErrType*` constants (`ErrTypeQuotaExhausted`, `ErrTypeAuth`,
  `ErrTypeSchema`, `ErrTypeBudget`, `ErrTypeRefusal`, `ErrTypeGlobal429`,
  `ErrTypeWebhookGone`).
  verify: `go test ./internal/workflows -run OptionsTable` checks every
  registered activity has an entry, every entry names a registered
  activity, and any entry on a rate-limited queue with a timeout over one
  minute has a heartbeat.
- [ ] `internal/activities`: structs `Ebay`, `Decide`, `Resolve`,
  `Persist`, `Score`, `Notify`, `Escalate`, `Catalog`, each with
  dependencies in fields and a `Register(w worker.Registry)` method;
  activity errors wrapped with
  `temporal.NewApplicationErrorWithOptions` and the right type;
  heartbeats in `FetchListings` (per page) and `Escalate` (per call).
  verify: `go test ./internal/activities` under
  `TestActivityEnvironment` with mocks covers every activity's success,
  retryable and non-retryable paths.
- [ ] Budget and quota checks: decide and escalate activities read the
  day's `budgets` row before the call and return `ErrTypeBudget` when
  spent; `FetchListings` reads `quota_usage` before each page and returns
  `ErrTypeQuotaExhausted` with the reset time; `Persist` writes decisions
  and spend in one transaction through `PersistDecisionsWithSpend`.
  verify: `go test ./internal/activities -run Budget` and `-run Quota`
  with the store mock assert the error types and that spend is written
  only with the decisions.
- [ ] `IngestWorkflow`: `LoadWatch`, `FetchListings`, child start per
  new item under the `max_concurrent_children` semaphore with
  `REJECT_DUPLICATE` and `ParentClosePolicy: ABANDON`,
  `WorkflowExecutionAlreadyStarted` counted as seen, await until
  `tick_deadline`, `UndeliveredAlerts`, `Notify` in batches of ten,
  `RecordTick`; quota sleep-to-reset once then `QuotaDeferred`;
  `cancel-tick` handling; `progress` query.
  verify: `go test ./internal/workflows -run TestIngest` covers the
  deadline, the duplicate child, the quota sleep and the cancel path.
- [ ] `ExtractWorkflow`: the DESIGN-0002 state machine with every
  transition upserting `SptStage` and calling `SetStage`; `awaitLabel`
  as written in DESIGN-0002 (drain buffered, selector over signal and
  timer); `AwaitingCatalog` waiting on `catalog-resolved`; `label_timeout`
  and `proposal_timeout` expiry; `stage` query; result struct.
  verify: `go test ./internal/workflows -run TestExtract` has one test
  per transition in the state diagram, including signal-before-wait,
  signal-after-wait, both timeouts and the rejected-proposal path.
- [ ] `BaselineWorkflow`: `MarkInactive`, `RecomputeBaselines`,
  `EmitBaselineMetrics` in sequence; auto-upgrade versioning behaviour.
  verify: `go test ./internal/workflows -run TestBaseline` asserts the
  order and that an activity failure stops the sequence.
- [ ] `CatalogProposalWorkflow`: started with signal-with-start by
  `ProposeCatalog` keyed by the dedupe key; waiter list in workflow
  state and in `catalog_proposal_waiters`; waits on `catalog-approve`
  with edits or `catalog-reject` with reason up to `proposal_timeout`;
  `ApplyCatalogDecision`; signals `catalog-resolved` to every waiter.
  verify: `go test ./internal/workflows -run TestProposal` covers a
  second signal-with-start attaching a waiter, approve with edits fanning
  out, reject, and timeout.
- [ ] `CatalogDigestWorkflow`: lists pending proposals and posts one
  summary embed; holds no state.
  verify: `go test ./internal/workflows -run TestDigest` asserts one
  `Notify` call with the pending count.
- [ ] `register.go`: `RegisterAPIRole` (nothing), `RegisterWorker`
  (all workflows and activities per queue), `RegisterForReplay` (the
  union); a test that the api role registers no workflows.
  verify: `go test ./internal/workflows -run TestRegistration` passes;
  `just lint` passes the depguard rule.
- [ ] `GetVersion` convention: a `versions.go` listing change IDs named
  after the document that introduced them; the first entry is
  `impl-0003-initial`.
  verify: `go test ./internal/workflows -run TestVersions` asserts every
  change ID in code is listed.
- [ ] Replay tests: `replay_test.go` replays every file under
  `testdata/histories` with the union registration; initial histories
  recorded through the Phase 1 recorder for each workflow type.
  verify: `go test ./internal/workflows -run TestReplay` replays at
  least five histories (one per workflow type) and fails on a
  deliberately edited history.
- [ ] Worker wiring in `spt worker`: register search attributes,
  register workflows and activities, start one worker per queue,
  `PromoteBuild`, `EnsureSchedule` for `baseline` and `catalog-digest`,
  readiness checks (Temporal ping, current version, Postgres), drain on
  SIGTERM for `WorkerStopTimeout`.
  verify: `go test -tags integration ./cmd/spt -run TestWorkerBoot`
  asserts the boot order against the dev server and a readiness flip.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `go test ./internal/workflows ./internal/activities` passes with no
  services; coverage at or above 80% in both packages.
- `go test ./internal/workflows -run TestReplay` replays at least five
  checked-in histories.
- The options table test proves the DESIGN-0002 activity table and the
  code agree row for row.
- `spt worker` boots against the dev server and reports ready.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: API role

The contract first, then the generated strict server, the middleware
chain, authentication, and handlers that reach Postgres and Temporal.

<!--docz:tasks:start-->
#### Tasks

- [ ] `api/openapi.yaml` written resource by resource from the
  DESIGN-0006 endpoint table: watches, listings (with decisions and
  label), alerts (dismiss and restore), baselines (refresh), catalog
  (rows, proposals, approve, reject), runs (list, journal), decisions
  (stats, calibration, agreement), registry, ingest (start, cancel),
  quota, status, the served spec; `Problem`, `Page_*` envelopes, cursor
  and limit parameters, `security: []` on the public operations.
  verify: `just lint-api` passes vacuum at `--fail-severity warn`.
- [ ] `api/oapi-codegen.yaml` (`std-http-server`, `strict-server`,
  `models`, `embedded-spec`, `skip-prune`) generating
  `internal/api/gen/api.gen.go`; `just generate-api`; a CI drift check.
  verify: `just generate-api && git diff --exit-code internal/api/gen`
  is clean; `go build ./...` succeeds.
- [ ] `internal/api/problem.go`: RFC 9457 writer with the type URIs from
  DESIGN-0006; `cursor.go`: HMAC-signed opaque keyset cursors with per-list
  sort keys and a page-size cap.
  verify: `go test ./internal/api -run Problem` and `-run Cursor`
  (round trip, tamper detection, cap).
- [ ] `authn.go`: OIDC discovery and JWKS cache with refresh, RS256 and
  ES256, `iss`, `aud`, `exp`, `nbf` with clock skew; `authz.go`: the
  operator group claim; `--insecure-no-auth` refuses to start unless the
  listen address is loopback.
  verify: `go test ./internal/api -run Authn` against the mock issuer
  covers valid, expired, wrong audience, wrong issuer, unknown key, and
  `-run Insecure` asserts the loopback guard.
- [ ] `middleware.go` and `server.go`: outermost first `otelhttp`,
  request ID and access log, recovery, authn, authz; two muxes for
  public and protected routes; `r.Pattern` labelling on the otelhttp
  labeler; `/api/v1` base URL.
  verify: `go test ./internal/api -run Middleware` asserts the order by
  observing a recorded sequence; an anonymous request to a protected
  route gets 401 before parameter binding (a malformed body still gets
  401, not 400).
- [ ] Store-backed handlers: watches CRUD (create and delete also call
  `EnsureSchedule` and `RemoveSchedule`; enable and disable pause and
  unpause), listings list and get and decisions, alerts list and get and
  dismiss and restore (writing `labels` and the Langfuse score through
  an interface), baselines, catalog rows and proposals, registry,
  quota, decisions stats, calibration and agreement.
  verify: `go test ./internal/api` with `StoreMock` covers every
  operation in the strict interface (a test enumerates the interface
  methods and fails on an untested one).
- [ ] Temporal-backed handlers by workflow name: `POST /ingest` starts
  `IngestWorkflow` and returns 202 with the ID; `DELETE /ingest/{id}`
  signals `cancel-tick`; `POST /baselines/refresh` starts
  `BaselineWorkflow`; `POST /listings/{id}/label` validates against the
  registry, signals `operator-label`, and on `NotFound` writes the
  `labels` row directly and returns 200 instead of 202;
  `POST /catalog/proposals/{id}/approve` and `/reject` signal the
  proposal workflow; Temporal unreachable maps to a 503 problem.
  verify: `go test ./internal/api -run Temporal` with a fake client
  interface covers each path, including the `NotFound` fallback and the
  503 mapping.
- [ ] Runs: `GET /runs` from `ListWorkflow` with a query built from the
  `SptStage`, `SptWatch`, `SptType`, `SptSchemaVersion`, `WorkflowType`
  and `ExecutionStatus` filters and a visibility page token as the
  cursor; `GET /runs/{id}/journal` from `GetWorkflowHistory` filtered to
  activity scheduled, completed and failed events and signals.
  verify: `go test -tags integration ./internal/api -run Runs` against
  the dev server lists a started `ExtractWorkflow` by stage and returns
  its journal.
- [ ] `GET /status`: component health from the readiness registry,
  backlog per queue from `DescribeBacklog`, `awaiting_label` and pending
  proposal counts, last tick per watch, eBay quota.
  verify: `go test ./internal/api -run Status` with mocks; `curl
  :8080/api/v1/status` on `spt all` returns the fields.
- [ ] `POST /decide`: budget check on `budgets('decide')`, then the
  Postgres-backed per-subject limiter (a `rate_limits(subject, window,
  count)` row updated in one statement; DESIGN-0006 Q7 option b), then
  the cascade; 429 problem with `Retry-After` when limited.
  verify: `go test -tags integration ./internal/api -run DecideLimiter`
  on testcontainers Postgres asserts the N+1th call in a window is 429
  and the window rolls.
- [ ] `spt api` role wiring: config, store, Temporal client, OIDC
  verifier, server on `LISTEN_ADDR`, readiness checks (Postgres ping,
  Temporal ping, registry loaded, migrations current), graceful
  shutdown.
  verify: `go test -tags integration ./cmd/spt -run TestAPIBoot`
  boots against the dev server and testcontainers Postgres and serves
  `/readyz` 200.
- [ ] Serve the spec at `/api/v1/openapi.yaml` from the embedded copy.
  verify: `curl :8080/api/v1/openapi.yaml | vacuum lint -` passes.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just lint-api` and the generation drift check pass in CI; every
  operation in the spec has a handler test.
- Authentication is enforced on every non-public route from the first
  commit of the role; `--insecure-no-auth` cannot bind a non-loopback
  address.
- `spt all` serves the API, starts a tick through `POST /ingest`, and
  `POST /listings/{id}/label` resumes a waiting `ExtractWorkflow` on
  the dev server.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: CLI client

The `spt` client verbs, generated from the same contract, with token
acquisition that works for an operator at a terminal and for CI.

<!--docz:tasks:start-->
#### Tasks

- [ ] Generate `internal/api/client` with oapi-codegen (`client` and
  `models`) from `api/openapi.yaml`; wire it into `just generate-api`
  and the drift check.
  verify: `just generate-api && git diff --exit-code internal/api/client`
  is clean.
- [ ] Client configuration: `SPT_API_URL` (default
  `http://127.0.0.1:8080`), `SPT_TOKEN` (bearer, used as is), otherwise
  the OIDC device-authorisation flow against `SPT_OIDC_ISSUER` and
  `SPT_OIDC_CLIENT_ID` with the refresh token stored in the OS keyring
  and refreshed silently; `spt auth login`, `spt auth logout`,
  `spt auth status`.
  verify: `go test ./cmd/spt -run Auth` against the mock issuer's device
  endpoints; `SPT_TOKEN=x spt watches list` sends `Authorization: Bearer
  x` to an `httptest` server.
- [ ] Verbs wired to the client: `watches list|create|update|enable|
  disable|delete`, `listings list|get|label`, `alerts list|get|dismiss|
  restore`, `catalog list|approve|reject`, `runs list|journal`,
  `decisions calibration|agreement|stats`, `ingest [--watch] [--full]
  [--schema-version]`, `ingest cancel`, `decide` (through
  `POST /decide`); `--output table|json|yaml`; problem documents
  rendered as `error: <title>: <detail>` with the exit code from the
  status class.
  verify: `go test ./cmd/spt -run Verbs` runs every verb against the
  strict server over `httptest` with a fake store; `spt --help` lists no
  `ErrNotImplemented` verbs except `catalog seed` and
  `registry diff` (IMPL-0002 owns those).
- [ ] Cursor paging in list verbs: `--limit`, `--cursor`, `--all` to
  follow pages.
  verify: `go test ./cmd/spt -run Paging` follows three pages with
  `--all`.
- [ ] Regenerate the cobra reference under `docs/cli/` and the drift
  check.
  verify: `just docs-cli && git diff --exit-code docs/cli` is clean.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every client verb in the DESIGN-0001 tree except the two IMPL-0002
  verbs works against `spt all` with `SPT_TOKEN` and with the device
  flow against the mock issuer.
- `go test ./cmd/spt` covers each verb's happy path and one problem
  response.
- `docs/cli/` is current.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: Integration and hardening

End-to-end on the dev server and testcontainers Postgres, the
versioning proof, contract tests, and the chart's queue split.

<!--docz:tasks:start-->
#### Tasks

- [ ] Full-tick integration test: `spt all` wiring in-process with the
  dev server, testcontainers Postgres, `tools/mock-ebay` on the `paged`
  scenario, `ScriptedDecider` answers, the notifier against the capture
  endpoint; one watch, one tick, assert listings, decisions, alerts and
  captured embeds.
  verify: `just test-integration` includes `TestFullTick` and it passes
  in under 60s.
- [ ] Scenario tests: `429-then-ok`, `5xx-then-ok`, `quota-exhausted`
  and `slow-page` scenarios through `/__admin/scenario`, asserting the
  retry, the quota sleep and the heartbeat behaviour from DESIGN-0002.
  verify: `go test -tags integration ./internal/workflows -run TestScenario`
  passes all four.
- [ ] Signals through the real client: a listing scripted to
  `AwaitingLabel`, labelled through `POST /listings/{id}/label`, resumes
  and alerts; a proposal approved through the API fans out
  `catalog-resolved`; `cancel-tick` abandons children without losing
  them.
  verify: `go test -tags integration ./internal/api -run TestSignals`
  passes all three.
- [ ] Schedule behaviour: a watch Schedule with overlap `SKIP` and a
  long-running tick skips the next fire; enable and disable pause and
  unpause it; deleting the watch removes it.
  verify: `go test -tags integration ./internal/workflows -run TestSchedule`
  asserts the skipped action in the Schedule description.
- [ ] Versioning test: start a long `ExtractWorkflow` on build `A`,
  start workers for build `B`, `PromoteBuild(B)`, assert the first run
  completes on `A` and a new run starts on `B`.
  verify: `go test -tags integration ./internal/temporal -run TestVersioningAB`
  passes.
- [ ] Contract tests: the generated client against the strict server
  over `httptest` with a fake store, one request per operation,
  asserting the response decodes into the generated model.
  verify: `go test ./internal/api -run TestContract` enumerates the
  spec's operations and fails on an uncovered one.
- [ ] `ui/scripts/check-api.ts` stub and the `check:api` script that
  regenerates `schema.gen.ts` with openapi-typescript and diffs it, wired
  into CI behind a `ui/` paths filter, so IMPL-0005 inherits the drift
  check.
  verify: `bun run check:api` passes on a fresh generation and fails
  after editing the spec without regenerating.
- [ ] Record replay histories for every workflow type from the
  integration run and commit them.
  verify: `SPT_RECORD_HISTORIES=1 just test-integration` writes the
  files and `go test ./internal/workflows -run TestReplay` replays them.
- [ ] Chart: `worker.queues` as a list of queue groups, each rendering a
  Deployment with `args: [worker, --queues, <group>]`; default one group
  with all queues; helm-unittest for one and two groups.
  verify: `just helm-unittest` passes; `helm template` with two groups
  renders two worker Deployments.
- [ ] CI: `test-integration` runs the dev server and testcontainers in
  the job; `lint-api`, `generate-api` drift, `check:api` added.
  verify: `gh pr checks --watch` reports every check passing.
- [ ] Open the PR with a `minor` label (first release that runs a tick).
  verify: `gh pr view --json labels` shows `minor`; CI green.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just test-integration` runs a full tick, every scenario, every
  signal path, the schedule test and the versioning test, green, with
  Docker as the only external requirement.
- Contract tests cover every operation; `check:api` is in CI.
- Replay histories exist for all five workflow types.
- The release train cuts a version whose worker and api boot on k3d
  (`just k3d-install`) and complete a tick against the mock eBay server.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `mise.toml` | Modify | Temporal CLI, oapi-codegen, vacuum pins |
| `go.mod` | Modify | `go.temporal.io/sdk`, contrib opentelemetry, oapi-codegen runtime, keyring library |
| `internal/temporal/` | Create | config, dial, worker, deployment, schedules, searchattributes, backlog, metrics, tracing, temporaltest, recorder |
| `internal/testutil/oidc/` | Create | mock OIDC issuer for tests |
| `internal/workflows/names/` | Create | workflow, signal, query, queue and search-attribute constants; ID builders |
| `internal/workflows/` | Create | options, register, versions, ingest, extract, baseline, proposal, digest, replay tests |
| `internal/activities/` | Create | ebay, decide, resolve, persist, score, notify, escalate, catalog |
| `testdata/histories/` | Create | replay histories per workflow type |
| `api/openapi.yaml`, `api/oapi-codegen.yaml`, `api/vacuum.yaml` | Create | the contract, generation config, lint ruleset |
| `internal/api/gen/`, `internal/api/client/` | Create | generated server and client |
| `internal/api/` | Create | server, handlers per resource, problem, cursor, authn, authz, middleware, limiter |
| `internal/store/migrations/0NN_rate_limits.sql` | Create | limiter table for `POST /decide` |
| `cmd/spt/api.go`, `cmd/spt/worker.go`, `cmd/spt/all.go`, `cmd/spt/auth.go`, `cmd/spt/client_*.go` | Create/Modify | role wiring and client verbs |
| `docs/cli/` | Modify | regenerated reference |
| `ui/scripts/check-api.ts`, `ui/package.json` | Create | `check:api` stub for IMPL-0005 |
| `charts/spt/templates/deployment-worker.yaml`, `values.yaml`, `values.schema.json`, `tests/` | Modify | `worker.queues` groups |
| `justfile`, `.github/workflows/ci.yml` | Modify | `generate-api`, `lint-api`, `test-integration`, `check:api` |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Unit tests under the SDK test suite for every workflow branch and
  under `TestActivityEnvironment` for every activity path, with mockery
  mocks.
- [ ] Options-table, registration, versions and replay tests in
  `internal/workflows`.
- [ ] Handler tests for every strict-server operation with `StoreMock`
  and a fake Temporal client; authn tests against the mock issuer;
  cursor and problem tests; contract tests with the generated client.
- [ ] Integration tests (tag `integration`) on the dev server and
  testcontainers Postgres: plumbing, full tick, scenarios, signals,
  schedules, versioning, runs, limiter.
- [ ] Coverage target 80% across `internal/temporal`,
  `internal/workflows`, `internal/activities`, `internal/api`.

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- IMPL-0001 complete: config, store, eBay client and mock server,
  notifier, scorer, health, observability, chart topology.
- IMPL-0002 complete: `pkg/decide` with `ScriptedDecider` and mocks,
  `pkg/registry`, `pkg/resolve`, the escalator.
- Cluster Temporal server version and worker-versioning dynamic config
  (recorded in Phase 0); `go.temporal.io/sdk` pinned at the minor the
  cluster is tested with.
- Temporal CLI dev server (pinned in `mise.toml`); Docker for
  testcontainers.
- A mock OIDC issuer in the test harness; no real issuer until
  IMPL-0004.
- oapi-codegen, vacuum, openapi-typescript (through bun) for the drift
  checks.

<!--docz:dependencies:end-->

## Open Questions

All questions were decided on 2026-11-14: option a in every case. The
bodies above reflect the decisions.

**Q1. Temporal Go SDK version pin policy.** Decided: option a. Pin to
the minor the cluster's server is tested with (the version repo-guardian
and boop run), bump through Renovate, with the replay tests and the
versioning integration test as the merge gate. Alternative: track the
latest SDK release.

**Q2. How replay histories are refreshed.** Decided: option a. The
env-gated recorder writes histories during integration runs; a new set
is committed with every release that adds a `GetVersion` change ID.
Alternative: manual export with `temporal workflow show`.

**Q3. The api role when Temporal is unreachable.** Decided: option a.
Start anyway: readiness fails, store-backed endpoints keep serving,
start and signal endpoints return a 503 problem until the client
reconnects. Alternative: refuse to start.

**Q4. CLI token storage.** Decided: option a, with a note. The question
is where the CLI keeps the operator's OIDC refresh token between runs:
the OS keyring through a Go keyring library (macOS Keychain, Secret
Service, Windows Credential Manager), falling back to a 0600 file only
when no keyring is available and saying so. The answer given pointed at
`temporal-oidc`; that is the Secret and mount repo-guardian uses for the
worker's Temporal client credentials, which is the service-side auth
already decided in DESIGN-0001 Q1 and not what this question is about.
Option a stands unless the operator wants the file instead. Alternative:
a 0600 file under the XDG config directory always.

**Q5. CLI client generation.** Decided: option a. Generate with
oapi-codegen `client` from the same spec, so the CLI cannot drift from
the contract. Alternative: hand-write a thin client.

**Q6. Contract test approach.** Decided: option a. The generated client
against the strict server over `httptest` with a fake store, one request
per operation, enumerated from the spec. Alternative: schema-driven
fuzzing in CI.

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [ADR-0002 Temporal is the execution plane; Postgres is the system of record](../adr/0002-temporal-is-the-execution-plane-postgres-is-the-system-of-record.md); [ADR-0006 The API is a spec-first OpenAPI contract served on net/http](../adr/0006-the-api-is-a-spec-first-openapi-contract-served-on-nethttp.md)
- [DESIGN-0001 System architecture](../design/0001-system-architecture-roles-configuration-deployment-and-data.md); [DESIGN-0002 Durable execution](../design/0002-durable-execution-workflows-activities-queues-and-signals.md); [DESIGN-0006 API](../design/0006-api-the-openapi-contract-authentication-and-clients.md)
- [IMPL-0001 Foundation](../impl/0001-phase-1-foundation-roles-configuration-store-ebay-client.md); [IMPL-0002 Typed extraction](../impl/0002-phase-2-typed-extraction-deciders-registry-catalog-escalator.md); [IMPL-0004 Integration](../impl/0004-phase-4-integration-cluster-dev-namespace-live-providers-side.md); [IMPL-0005 Operator UI](../impl/0005-phase-5-operator-ui-bff-alerts-review-catalog-runs-decisions.md)
- Re-read, not imported: [repo-guardian `internal/temporal`](https://github.com/donaldgifford/repo-guardian/tree/main/internal/temporal), [repo-guardian `internal/workflows`](https://github.com/donaldgifford/repo-guardian/tree/main/internal/workflows), [repo-guardian `internal/api`](https://github.com/donaldgifford/repo-guardian/tree/main/internal/api), [boop `internal/worker/worker.go`](https://github.com/donaldgifford/boop/blob/main/internal/worker/worker.go)
- [Temporal Go SDK developer guide](https://docs.temporal.io/develop/go); [child workflows](https://docs.temporal.io/develop/go/child-workflows); [schedules](https://docs.temporal.io/develop/go/schedules); [worker versioning](https://docs.temporal.io/worker-versioning); [message passing](https://docs.temporal.io/develop/go/message-passing); [testing and replay](https://docs.temporal.io/develop/go/testing-suite); [`worker.Options`](https://pkg.go.dev/go.temporal.io/sdk/worker#Options); [OpenTelemetry contrib](https://pkg.go.dev/go.temporal.io/sdk/contrib/opentelemetry); [Temporal CLI dev server](https://docs.temporal.io/cli/server#start-dev)
- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen); [vacuum](https://github.com/daveshanley/vacuum); [openapi-typescript](https://openapi-ts.dev); [RFC 9457 Problem Details](https://www.rfc-editor.org/rfc/rfc9457); [OAuth 2.0 Device Authorization Grant](https://www.rfc-editor.org/rfc/rfc8628); [Go 1.22 routing enhancements](https://go.dev/blog/routing-enhancements)

<!--docz:references:end-->
