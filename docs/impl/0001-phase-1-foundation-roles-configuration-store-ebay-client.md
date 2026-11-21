---
id: IMPL-0001
title: "Phase 1 Foundation: roles, configuration, store, eBay client, notifier, scorer, observability"
status: Draft
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0001: Phase 1 Foundation: roles, configuration, store, eBay client, notifier, scorer, observability

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
  - [Phase 1: Binary, roles, configuration, health](#phase-1-binary-roles-configuration-health)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 2: Store and migrations](#phase-2-store-and-migrations)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 3: eBay client, mock server, notifier, scorer](#phase-3-ebay-client-mock-server-notifier-scorer)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 4: Chart topology, build and development loop](#phase-4-chart-topology-build-and-development-loop)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 5: Golden dataset and scaffold cleanup](#phase-5-golden-dataset-and-scaffold-cleanup)
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

Turn the forge-templated scaffold into the foundation every later phase
builds on: the one binary with its role and client subcommands, the HCL
configuration loader, the Postgres store and migrations, the eBay client
with a mock server, the Discord notifier, the composite scorer, health
and observability, the chart's role topology, the development loop with
secrets from 1Password, and the golden dataset exported into the
repository. Everything in this document is verified with `just check`
and `just test-integration` and needs no route to the cluster.

**Implements:** RFC-0001 (Delivery phase 1); DESIGN-0001 in full except
the Temporal plumbing; the store, eBay, notifier and scorer pieces that
DESIGN-0002 and DESIGN-0003 call.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- Toolchain pins and the depguard boundary between roles.
- `cmd/spt` subcommand tree with role commands that start and stop
  cleanly, and client commands that fail with "not implemented" until
  IMPL-0003 generates the API client.
- `internal/config`: hclkit loader, defaults, `env()`, secret guard,
  validators, `spt config validate`.
- `internal/store`: pgx pool, Goose migrations `001`–`00n` for every
  table in DESIGN-0001, idempotent upserts, transactional counters,
  `spt migrate` and `migrate --check`.
- `internal/ebay`: Browse search with paging, OAuth client-credentials
  token provider, Analytics rate-limit sync, typed errors for 429, 5xx
  and quota; `tools/mock-ebay` with scenario files.
- `internal/notify`: Discord embeds, ten-per-post chunking, bucket-aware
  retry delays, a capture endpoint for tests.
- `pkg/scorer`: the composite score with configurable weights.
- `internal/health` and `internal/observability`: `/healthz`, `/readyz`,
  `/metrics`, OTel SDK with OTLP export, Prometheus bridge, slog JSON, the
  app metric set from DESIGN-0001 pinned by a test.
- Chart: `topology`, `api` and `worker` Deployments, `migrate` hook Job,
  `temporal` and `database` values, the ConfigMap-mounted `spt.hcl`,
  extended collision helper, helm-unittest suites.
- `just dev` with compose and `op run`; `testdata/golden/` exported from
  the Langfuse dataset; scaffold cleanup (README, CLAUDE.md,
  catalog-info, chart README).

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Anything that calls a model (IMPL-0002).
- Temporal: client, worker, workflows, activities, schedules
  (IMPL-0003). The `queue` and `workflows` config blocks are decoded and
  validated here but consumed there.
- The OpenAPI contract and handlers (IMPL-0003); the UI (IMPL-0005).
- Cluster deployment, real credentials against production eBay
  (IMPL-0004).

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

Confirm the external facts the rest of the document assumes, and pin the
tools. Nothing here produces product code.

<!--docz:tasks:start-->
#### Tasks

- [ ] Confirm the spt 1Password Environment exists and record its ID in
  `justfile` as `op_env`; confirm it carries `DATABASE_URL`,
  `EBAY_CLIENT_ID`, `EBAY_CLIENT_SECRET`, `DISCORD_WEBHOOK_URL`,
  `JEV_API_KEY`, `OPENCODE_API_KEY`, `ANTHROPIC_API_KEY`,
  `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`.
  verify: `op run --environment $op_env -- env | grep -c '^EBAY_\|^JEV_\|^OPENCODE_\|^ANTHROPIC_\|^LANGFUSE_\|^DISCORD_'` prints 8 or more.
- [ ] Confirm the golden dataset exists in Langfuse and note its dataset
  name and item count.
  verify: `grep -c '^- ' testdata/golden/README.md` matches the Langfuse item count recorded there.
- [ ] Confirm read access to the first implementation's Postgres (needed
  by IMPL-0002 seeding) or record that it is unavailable.
  verify: `psql "$V1_DATABASE_URL" -c 'select count(*) from listings'`
  returns, or the open question below is answered.
- [ ] Pin in `mise.toml`: `oapi-codegen`, `vacuum`, `goose`, `temporal`
  (CLI), `bun`, `hclkit`, `postgres` client (`psql`), `docker compose`
  availability noted. Keep the absolute mockery v3 path convention.
  verify: `mise install && mise ls --current` shows every pin; `just --list`
  runs.
- [ ] Add a depguard rule to `.golangci.yml` denying
  `internal/workflows` and `internal/activities` to `internal/api/**` and
  `cmd/spt/api*.go`, and denying `internal/api` to `internal/workflows/**`
  and `internal/activities/**`.
  verify: a throwaway import in `internal/api` fails `just lint`; remove
  it.
- [ ] Decide the Go module layout for `pkg/` versus `internal/` per
  DESIGN-0001 and create the empty package directories with `doc.go`
  files.
  verify: `go build ./...` succeeds; `go vet ./...` clean.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `mise install` is reproducible on a clean machine and `just check`
  passes on the scaffold with the new pins.
- The depguard rule exists and is proven to fire.
- The 1Password Environment, golden dataset and v1 database facts are
  recorded in this document's Dependencies section.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 1: Binary, roles, configuration, health

The subcommand tree, the configuration loader and the process skeleton
that every role shares.

<!--docz:tasks:start-->
#### Tasks

- [ ] Add cobra and build the subcommand tree from DESIGN-0001 (`api`,
  `worker`, `all`, `migrate`, `config validate`, `registry ...`,
  `catalog ...`, `watches ...`, `listings ...`, `alerts ...`, `runs ...`,
  `decisions ...`, `ingest ...`, `decide`, `version`). Client and
  not-yet-built commands return `ErrNotImplemented` with the IMPL that
  delivers them.
  verify: `spt --help` lists every group; `spt version` prints the three
  stamped values; `spt watches list` exits 2 with the message.
- [ ] Generate committed cobra markdown under `docs/cli/` with
  `DisableAutoGenTag` set recursively, and a `just docs-cli` recipe with a
  drift check in CI.
  verify: `just docs-cli && git diff --exit-code docs/cli` is clean.
- [ ] `internal/config`: `Config` structs for every block in DESIGN-0001
  with hclkit tags; `Load(path, opts...)` applying defaults, the file,
  `env()` through `funcs.Env`, then flag overrides; `Diagnostics`
  aggregated with positions.
  verify: `go test ./internal/config` with table-driven cases for each
  block.
- [ ] Secret guard: fields tagged `secret:"true"` reject any expression
  that is not an `env()` call; the diagnostic names the field and line.
  verify: a fixture with a literal `api_key` fails with that diagnostic.
- [ ] Validators: cascade names resolve to declared deciders; at least
  one decider enabled; `kind` enum; budgets non-negative; queue names in
  the known set; durations within bounds; `ebay.quota_reserve` in [0, 1).
  verify: one failing fixture per validator in `testdata/config/invalid/`.
- [ ] Environment overrides: `LISTEN_ADDR`, `METRICS_ADDR`, `LOG_LEVEL`
  override their fields; `POD_NAME` feeds the identity.
  verify: `go test ./internal/config -run EnvOverride` passes.
- [ ] `spt config validate <file>` runs the loader with a fake
  environment in which every `env()` resolves to a placeholder, prints
  diagnostics GCC-style, exits 1 on any error.
  verify: `spt config validate testdata/config/example.hcl` exits 0;
  the invalid fixtures exit 1.
- [ ] Commit `testdata/config/example.hcl` equal to the DESIGN-0001
  example and a golden test that it loads to the expected struct.
  verify: `go test ./internal/config -run Example`.
- [ ] `internal/health`: `/healthz` (process up), `/readyz` (a registry of
  named checks; ready only when all pass), `/metrics` (Prometheus
  handler), served on `METRICS_ADDR` with its own `http.Server`.
  verify: `go test ./internal/health`; `curl :9090/readyz` returns 503
  until checks pass.
- [ ] `internal/observability`: OTel SDK with OTLP gRPC exporter to
  `observability.otlp_endpoint`, resource attributes (service name,
  version, role, pod), Prometheus exporter bridged to `/metrics`, slog
  JSON handler with trace and span IDs injected, `LOG_LEVEL` honoured.
  verify: `go test ./internal/observability`; `spt all` with an
  unreachable collector logs one warning and keeps serving.
- [ ] App metric set: a `Metrics` struct with every instrument from
  DESIGN-0001 and an exact-set test over `/metrics` output names.
  verify: `go test ./internal/observability -run MetricSet`.
- [ ] Role skeleton: `spt api`, `spt worker` and `spt all` load config,
  start health, log a ready line, and shut down on SIGTERM within
  `server.shutdown_timeout`; `all` runs both in one process.
  verify: `spt all --config testdata/config/example.hcl` starts and exits
  0 on SIGTERM within the timeout; a test asserts the order.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just check` passes; `internal/config`, `internal/health` and
  `internal/observability` are at or above 80% coverage.
- `spt config validate` is wired into `just lint` against
  `testdata/config/example.hcl` and every chart values file's rendered
  ConfigMap (see Phase 4).
- The CLI reference under `docs/cli/` is committed and drift-checked.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Store and migrations

Every table in DESIGN-0001, the migrator, and the store methods later
phases call, with the idempotency and transaction rules the design
requires.

<!--docz:tasks:start-->
#### Tasks

- [ ] Goose SQL migrations embedded under `internal/store/migrations`:
  `001_extensions.sql` (`pg_trgm`), then one file per table group:
  watches; listings; decisions (with `role`); labels; catalog and
  `catalog_proposals` and `catalog_proposal_waiters`; `price_baselines`;
  alerts; `quota_usage` and `budgets`; indexes from DESIGN-0001 and
  DESIGN-0005.
  verify: `spt migrate` on an empty database applies all; `spt migrate
  --check` exits 0; a second `spt migrate` is a no-op.
- [ ] `internal/store`: `New(ctx, dsn, opts)` with pgx pool, `otelpgx`
  tracing, `statement_timeout` from config; `Migrate` and `PendingMigrations`.
  verify: `go test -tags integration ./internal/store` on testcontainers
  Postgres.
- [ ] Watches: `CreateWatch`, `UpdateWatch`, `SetWatchEnabled`,
  `DeleteWatch`, `GetWatch`, `ListWatches(cursor)`.
  verify: `go test -tags integration ./internal/store -run Watches` passes, including the keyset cursor round trip.
- [ ] Listings: `UpsertListing` keyed by `(ebay_item_id, schema_version)`
  returning whether it was new; `TouchLastSeen` batch; `SetStage`;
  `MarkInactive(watch, cutoff)`; `GetListing`; `ListListings(filter,
  cursor)`.
  verify: `go test -tags integration ./internal/store -run UpsertListingIdempotent` asserts a second upsert changes no row and keeps `first_seen`.
- [ ] Decisions and labels: `InsertDecisions(batch)` append-only with
  `role`; `InsertLabel`; `ListDecisions(item, schema)`; the calibration
  query (`decisions` joined to `labels`, binned) as SQL under
  `internal/store/queries/`.
  verify: `go test -tags integration ./internal/store -run Calibration` reproduces a known bin table from a synthetic set.
- [ ] Catalog: `UpsertCatalogRow`, `MergeCatalogRows(into, from)`
  (repoint listings, union aliases), `RetrieveCandidates(component, q,
  limit, threshold)` with the trigram SQL from DESIGN-0005,
  `ListCatalog(filter, cursor)`; proposals: `UpsertProposal` with the
  dedupe key partial unique index, `AddProposalWaiter`,
  `DecideProposal`, `ListProposals(status, cursor)`, `NearestRows`.
  verify: `go test -tags integration ./internal/store -run Catalog` on a 200-row fixture catalog; retrieval returns the seeded aliases.
- [ ] Baselines: `RecomputeBaselines(window)` as one transaction (upsert
  then orphan delete); `GetBaseline`, `ListBaselines`.
  verify: `go test -tags integration ./internal/store -run Baselines` seeds, recomputes, removes a listing, recomputes, asserts the orphan is gone.
- [ ] Alerts: `InsertAlert`, `MarkDelivered(ids)`, `Dismiss`, `Restore`,
  `UndeliveredAlerts(watch)`, `ListAlerts(filter, cursor)`.
  verify: `go test -tags integration ./internal/store -run Alerts` passes.
- [ ] Counters: `IncrementQuota(day, calls)` and `IncrementBudget(name,
  day, usd)` exposed as functions that take a `pgx.Tx`, and
  `PersistDecisionsWithSpend` that runs decisions insert and budget
  increment in one transaction.
  verify: `go test -tags integration ./internal/store -run CounterTransactional` fails the insert and asserts the counter did not move.
- [ ] `spt migrate [--check] [--to N]` subcommand using the embedded
  migrations; `--check` exits 3 when pending.
  verify: `go test -tags integration ./cmd/spt -run Migrate` passes against testcontainers.
- [ ] Mockery v3 config generating `StoreMock` from the `Store` interface
  into `internal/store/mocks/`; `just mocks` recipe with the absolute v3
  path.
  verify: `just mocks && git diff --exit-code internal/store/mocks`.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just test-integration` passes on a clean machine with Docker and no
  other services.
- Every upsert is proven idempotent and every counter increment is
  proven transactional by a test.
- The schema matches the DESIGN-0001 ER diagram; a doc check compares
  table and column names in the migrations with the diagram.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: eBay client, mock server, notifier, scorer

The edges of the pipeline that are pure Go clients, each testable
without a network.

<!--docz:tasks:start-->
#### Tasks

- [ ] `internal/ebay`: client-credentials token provider with early
  refresh and single-flight; Browse `Search(ctx, query, filters, page)`
  returning typed item summaries (title, item id, price, currency,
  condition, listing type, quantity, item specifics, seller, URL);
  pagination helper that stops at `pages_per_tick`.
  verify: unit tests against recorded fixtures under `testdata/ebay/`.
- [ ] Typed errors: `ErrRateLimited` (429, with `Retry-After`),
  `ErrServer` (5xx), `ErrQuotaExhausted` (from the Analytics limit or an
  explicit quota response, carrying the reset time), `ErrAuth`.
  verify: `go test ./internal/ebay -run Errors` maps every fixture response to its error.
- [ ] Analytics `GetRateLimits` and a `QuotaSync` helper that writes
  `quota_usage` for the day.
  verify: `go test -tags integration ./internal/ebay -run QuotaSync` writes the row.
- [ ] `tools/mock-ebay`: an HTTP server with scenario files
  (`testdata/ebay/scenarios/*.yaml`): paged results, empty page, 429 with
  `Retry-After`, 5xx then success, quota exhausted, slow page; an
  `/__admin/scenario` endpoint to switch scenarios; a Dockerfile target
  for k3d.
  verify: `go run ./tools/mock-ebay --scenario paged` and the client's
  integration tests pass against it.
- [ ] `internal/notify`: Discord embed builder from an alert record,
  chunking at ten embeds per POST, `inter_chunk_delay`, bucket-aware
  retry delay from `X-RateLimit-Reset-After`, typed `ErrGlobal429` and
  `ErrWebhookGone`; `summary_only` mode.
  verify: unit tests with an `httptest` webhook; a 429 fixture produces
  the computed delay.
- [ ] `tools/mock-ebay` gains a `/discord` capture endpoint (or a separate
  `tools/capture`) that records posted embeds for e2e assertions.
  verify: `go test -tags integration ./internal/notify -run Capture` sees the posted embeds in the capture list.
- [ ] `pkg/scorer`: composite score 0–100 from price-versus-baseline,
  seller, condition, quantity and quality factors with `scoring.weights`;
  `min_baseline_samples` gate; a breakdown struct for the why panel.
  verify: `go test ./pkg/scorer` passes, including thin-baseline and lot cases and golden breakdowns.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The eBay client passes its integration tests against every mock
  scenario, including resuming after 429 and stopping on quota.
- The notifier never sends more than ten embeds per POST and honours
  bucket delays in tests.
- `pkg/scorer` has no dependency on the store or any client.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: Chart topology, build and development loop

The chart renders the roles, the migration hook and the configuration
file; the development loop runs everything locally with secrets from
1Password.

<!--docz:tasks:start-->
#### Tasks

- [ ] Chart values and templates from DESIGN-0001: `topology`,
  `api.*`, `worker.*`, `temporal.*`, `database.existingSecret`,
  `config.file`, `migrate.enabled`; `deployment-api.yaml`,
  `deployment-worker.yaml` (or one Deployment for `all`), `job-migrate.yaml`
  with hook annotations, `configmap-spt-hcl.yaml` mounted at
  `/etc/spt/spt.hcl`, ServiceMonitor per role.
  verify: `just helm-template` renders both topologies without error.
- [ ] Extend `spt.validateEnvCollisions` to `DATABASE_URL`, `TEMPORAL_*`
  and `OIDC_*`; update `values.schema.json`.
  verify: a values file colliding on `TEMPORAL_ADDRESS` fails
  `helm template`.
- [ ] helm-unittest suites: split and all topologies, migrate hook on and
  off, collision helper, ServiceMonitor per role, ConfigMap content.
  verify: `just helm-unittest` passes.
- [ ] PrometheusRule additions from DESIGN-0001 with `promtool check
  rules` in `just helm-lint`.
  verify: `just helm-lint` passes.
- [ ] `tools/dashgen` producing Grafana dashboard JSON from the metric
  set into `charts/spt/dashboards/`, committed and drift-checked.
  verify: `just dashgen && git diff --exit-code charts/spt/dashboards`.
- [ ] `compose.yaml` with Postgres 17 and the Temporal CLI dev server;
  `just dev` runs `op run --environment $op_env -- spt all --config
  dev.hcl --insecure-no-auth` against them; `dev.hcl` committed with
  `env()` for every secret.
  verify: `just dev` reaches ready; `curl :9090/readyz` is 200.
- [ ] `just k3d-install` builds the image, imports it, and installs the
  chart with `topology: all` and a dev values file that enables Postgres
  and the Temporal dev server as sub-charts or sidecars.
  verify: `kubectl get pods` shows spt ready; `/readyz` 200 through a
  port-forward.
- [ ] CI: `test-integration` job with Docker; `helm-unittest` covers the
  new suites; `config-validate` step renders the ConfigMap for each
  values file and runs `spt config validate` on it.
  verify: `gh pr checks` reports every check green.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just helm-test` passes; both topologies render and install on k3d.
- `just dev` brings up a ready `spt all` with no `.env` file on disk.
- CI runs unit, integration and chart gates on every PR.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: Golden dataset and scaffold cleanup

The dataset later phases measure against, and the repository's
self-description brought in line with what it now is.

<!--docz:tasks:start-->
#### Tasks

- [ ] `tools/dataset-export`: pulls the Langfuse dataset into
  `testdata/golden/listings.jsonl` (one object per listing: the eBay
  item summary as `State`, the labelled component type, fields and
  catalog key, the source and date) and `testdata/golden/README.md`
  with counts per component type.
  verify: `go run ./tools/dataset-export` is idempotent; the README
  counts match `wc -l`.
- [ ] Add 20 hand-written hard cases per component type to
  `testdata/golden/hard.jsonl` (bundles, accessories, lots, wrong
  vendor, thin titles) with labels.
  verify: `go test ./testdata/golden` validates every record against the
  schema.
- [ ] Rewrite `README.md` for spt (what it is, the pipeline, how to run
  `just dev`, where the docs are); update `CLAUDE.md` with the roles,
  the depguard boundary, the `op run` convention, the mockery path and
  the IMPL workflow; update `catalog-info.yaml` description and links;
  regenerate the chart README.
  verify: `just helm-docs && markdownlint-cli2 "**/*.md"` clean;
  `git grep -i "templated go repo"` finds nothing.
- [ ] Remove scaffold placeholders (`run()` printing the version, generic
  "Platform" descriptions, unused values).
  verify: `git grep -n "Platform"` in chart and docs finds nothing stale.
- [ ] Open the PR with a `dont-release` label (no release is cut for
  foundation).
  verify: `gh pr view --json labels` shows `dont-release`; `gh pr checks` is green.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `testdata/golden/` holds the exported dataset plus hard cases, with a
  schema test.
- A newcomer can run `just dev` from `README.md` alone.
- `just check` and `just test-integration` are green with no services
  beyond Docker.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `mise.toml` | Modify | pins for oapi-codegen, vacuum, goose, temporal CLI, bun, hclkit, psql |
| `.golangci.yml` | Modify | depguard rules for the role boundary |
| `cmd/spt/*.go` | Create | cobra root and one file per subcommand group |
| `docs/cli/*.md` | Create | generated CLI reference |
| `internal/config/` | Create | hclkit loader, validators, env overrides, `validate` command |
| `internal/health/` | Create | healthz, readyz checks, metrics server |
| `internal/observability/` | Create | OTel, Prometheus bridge, slog, metric set |
| `internal/store/` | Create | pgx store, embedded Goose migrations, queries, mocks |
| `internal/ebay/` | Create | Browse client, token provider, Analytics, errors, fixtures |
| `internal/notify/` | Create | Discord notifier |
| `pkg/scorer/` | Create | composite score |
| `tools/mock-ebay/`, `tools/dashgen/`, `tools/dataset-export/` | Create | development and CI tools |
| `charts/spt/templates/*` | Modify/Create | role Deployments, migrate Job, ConfigMap, ServiceMonitors, rules |
| `charts/spt/values.yaml`, `values.schema.json`, `tests/*` | Modify | new values and suites |
| `compose.yaml`, `dev.hcl`, `justfile` | Create/Modify | development loop, `op run`, new recipes |
| `testdata/config/`, `testdata/ebay/`, `testdata/golden/` | Create | fixtures and dataset |
| `README.md`, `CLAUDE.md`, `catalog-info.yaml` | Modify | scaffold cleanup |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Unit tests for every exported function in `internal/config`,
  `internal/health`, `internal/observability`, `internal/ebay`,
  `internal/notify`, `pkg/scorer`; table-driven where inputs vary.
- [ ] Integration tests (tag `integration`) for `internal/store` on
  testcontainers Postgres and for `internal/ebay` against `mock-ebay`.
- [ ] Exact-set test for the metric names; golden test for the example
  configuration; schema test for the golden dataset.
- [ ] helm-unittest suites and `ct lint` for the chart; `promtool` for
  rules.
- [ ] Coverage target 80% across `internal/` and `pkg/`, reported in CI.

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- Docker locally and in CI (testcontainers, compose, k3d).
- 1Password CLI with access to the spt Environment (ID recorded in Phase
  0); a 1Password service account scoped to it for any CI job that needs
  live keys (none in this IMPL).
- Langfuse project with the golden dataset (confirmed in Phase 0).
- Read access to the first implementation's database for IMPL-0002
  seeding (confirmed or declined in Phase 0).
- Library choices recorded for later IMPLs: cobra, hclkit v0.2.x, pgx v5
  with `otelpgx`, Goose v3, testcontainers-go, OTel Go SDK with the
  Prometheus exporter, mockery v3.

<!--docz:dependencies:end-->

## Open Questions

All questions were decided on 2026-11-14: option a in every case. The
bodies above reflect the decisions.

**Q1. CLI framework.** Decided: option a. cobra, with generated markdown
under `docs/cli/` and `DisableAutoGenTag` set recursively (the
convention the other repos settled on after footer drift broke CI).
Alternative: standard-library `flag` with hand-rolled dispatch.

**Q2. SQL layer.** Decided: option a. pgx v5 with hand-written SQL in
`internal/store/queries/*.sql`, embedded and loaded by name.
Alternative: sqlc generating typed Go from the same files.

**Q3. Migration tool.** Decided: option a. Goose v3 with embedded SQL
files, run by `spt migrate` and checked at start. Alternative:
golang-migrate.

**Q4. Mock eBay server shape.** Decided: option a. A small Go server
under `tools/mock-ebay` with YAML scenario files and an admin endpoint,
built into an image for k3d and e2e. Alternative: WireMock with JSON
mappings.

**Q5. Local Postgres and Temporal for `just dev`.** Decided: option a.
`compose.yaml` with Postgres and the Temporal CLI dev server; k3d stays
for chart verification. Alternative: k3d from the start.

**Q6. Golden dataset format on disk.** Decided: option a. JSONL with one
object per listing carrying the state and the labels, plus a README with
counts. Alternative: one JSON file per listing.

**Q7. If the first implementation's database is unreachable for
seeding.** Decided: option a. Export its `listings` and item specifics
to a CSV once, by hand, commit it under `testdata/seed/` (no personal
data in it), and seed from the file. Alternative: build the catalog from
proposals alone.

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [DESIGN-0001 System architecture](../design/0001-system-architecture-roles-configuration-deployment-and-data.md); [DESIGN-0002 Durable execution](../design/0002-durable-execution-workflows-activities-queues-and-signals.md); [DESIGN-0005 Catalog](../design/0005-catalog-canonical-identities-retrieval-and-proposals.md) (retrieval SQL, tables)
- [ADR-0002](../adr/0002-temporal-is-the-execution-plane-postgres-is-the-system-of-record.md); [ADR-0005](../adr/0005-configuration-is-hcl-decoded-with-hclkit.md)
- [IMPL-0002 Typed extraction](../impl/0002-phase-2-typed-extraction-deciders-registry-catalog-escalator.md); [IMPL-0003 Durable execution](../impl/0003-phase-3-durable-execution-temporal-plumbing-workflows.md)
- [hclkit](https://github.com/donaldgifford/hclkit); [pgx](https://github.com/jackc/pgx); [Goose](https://github.com/pressly/goose); [testcontainers-go](https://golang.testcontainers.org); [mockery v3](https://vektra.github.io/mockery/latest/); [1Password `op run`](https://www.1password.dev/cli/reference/commands/run)
- [eBay Browse API](https://developer.ebay.com/api-docs/buy/browse/overview.html); [eBay Analytics rate limits](https://developer.ebay.com/api-docs/developer/analytics/resources/rate_limit/methods/getRateLimits); [Discord webhook rate limits](https://discord.com/developers/docs/topics/rate-limits)
- References from the first implementation (re-read, not imported): [`internal/ebay`](https://github.com/donaldgifford/server-price-tracker/tree/main/internal/ebay), [`internal/notify`](https://github.com/donaldgifford/server-price-tracker/tree/main/internal/notify), [`pkg/scorer`](https://github.com/donaldgifford/server-price-tracker/tree/main/pkg/scorer); [boop `internal/config`](https://github.com/donaldgifford/boop/tree/main/internal/config)

<!--docz:references:end-->
