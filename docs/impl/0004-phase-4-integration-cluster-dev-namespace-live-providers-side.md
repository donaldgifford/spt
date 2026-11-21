---
id: IMPL-0004
title: "Phase 4 Integration: cluster dev namespace, live providers, promotion to prod"
status: Draft
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0004: Phase 4 Integration: cluster dev namespace, live providers, promotion to prod

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
  - [Phase 1: Namespaces, databases, secrets and identity](#phase-1-namespaces-databases-secrets-and-identity)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 2: The spt-dev release](#phase-2-the-spt-dev-release)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 3: Live provider admission and calibration](#phase-3-live-provider-admission-and-calibration)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 4: All watches, soak and tuning](#phase-4-all-watches-soak-and-tuning)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 5: Promotion to prod and operations](#phase-5-promotion-to-prod-and-operations)
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

Take the system that IMPL-0001 through IMPL-0003 verified against mocks,
fixtures and the Temporal dev server, and run it on the cluster: a
`spt-dev` release against the real Temporal, a real CNPG database, real
eBay at a capped quota, the live decision and text-model providers, and
Langfuse. Admit each provider through the live contract suite and the
golden dataset, set the registry thresholds from measured calibration,
soak on every watch for two weeks with the operator labelling as it
runs, and promote to the `spt` release. The first implementation no
longer runs, so there is nothing to compare against or cut over from;
its database is read once to seed the catalog. Everything in this
document needs a route to the homelab; nothing in it is verifiable by
`just check` alone.

**Implements:** RFC-0001 (Delivery phase 4); DESIGN-0001 rollout steps
2–3; DESIGN-0002 rollout steps 3–4; DESIGN-0003 rollout steps 2–5;
DESIGN-0005 rollout steps 2–4.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- Cluster facts confirmed and recorded: Temporal namespaces, server
  version and worker versioning, the OIDC issuer and the mTLS pattern,
  CNPG databases, the OTel collector, Langfuse, the eBay application,
  Discord webhooks, the secrets path from the 1Password Environment to
  Kubernetes.
- Namespaces, databases, Secrets and the Temporal client identity for
  `spt-dev` and `spt`.
- The `spt-dev` Helm release with `topology: split`, the migrate hook,
  dashboards and alert rules, on one watch and then all watches.
- Live admission of every configured decider route through the
  `decidertest` suite and the golden dataset; calibration tables;
  registry thresholds set from Jev's calibration; cross-check at 10%;
  the escalator live with a low budget.
- The two-week dev soak on all watches with daily labelling; catalog
  seeding from the first implementation's database as proposals and
  the approval pass; one threshold tuning from the soak's labels.
- Promotion to the `spt` release and the post-promotion review.
- Operational notes (what to look at when something is stuck, how to
  pause a watch, how to replay a tick) written into this document's
  Phase 5 and the repository README, because the `runbook` doc type is
  disabled in `.docz.yaml`.

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Any new product code beyond environment values files and small fixes
  found during integration (those go through
  normal PRs against the IMPL that owns the code).
- The operator UI (IMPL-0005); until it ships, the CLI is the operator
  surface for labels and proposals.
- Changing cluster-wide infrastructure (Temporal, CNPG, the OIDC issuer,
  the collector). This IMPL consumes them and records what it needed.
- Multi-tenancy or a second operator.

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its
tasks are checked off and its success criteria are met. Every task
carries a `verify:` line naming the command or observation that proves
it done. Commands that touch the cluster assume a kubeconfig context
named `homelab` and the `temporal` CLI configured by the `TEMPORAL_*`
environment from the spt 1Password Environment.

---

<!--docz:phase:start-->
### Phase 0: Prerequisites

Confirm every cluster fact the later phases depend on, and record each
one in the Dependencies section. Nothing here changes the cluster.

<!--docz:tasks:start-->
#### Tasks

- [ ] Confirm rights to create the Temporal namespaces `spt-dev` and
  `spt`, record the server version, and confirm worker deployment
  versioning is enabled on the server.
  verify: `temporal operator cluster system` prints the version and
  `temporal operator namespace list` succeeds with the operator identity.
- [ ] Confirm the cluster OIDC issuer can register a confidential client
  `spt` for Temporal client-credentials, and that the OpenBao-issued mTLS
  pattern from repo-guardian DESIGN-0028 can issue a certificate for the
  `spt` identity. Record the issuer URL, the audience, and the OpenBao
  role name.
  verify: `curl -fsS "$OIDC_ISSUER/.well-known/openid-configuration" | jq .token_endpoint`
  prints the endpoint; the OpenBao role is listed by
  `bao list pki/roles`.
- [ ] Confirm rights on the existing CNPG cluster to create databases
  `spt_dev` and `spt`, each owned by its own `app` user, with `pg_trgm`
  available (DESIGN-0001 Q3, decided: new databases on the existing
  cluster).
  verify: `kubectl -n cnpg get cluster` lists the cluster and
  `psql "$CNPG_ADMIN_URL" -c "select name from pg_available_extensions where name = 'pg_trgm'"`
  returns one row.
- [ ] Record the OTel collector OTLP gRPC endpoint and confirm it is
  reachable from the `spt` namespace.
  verify: `kubectl -n spt-dev run otel-probe --rm -it --image=busybox --restart=Never -- nc -zv otel-collector.observability.svc 4317`
  reports open.
- [ ] Confirm the Langfuse project for spt, its public and secret keys in
  the 1Password Environment, and the golden dataset present in it.
  verify: `op run --environment $op_env -- curl -fsS -u "$LANGFUSE_PUBLIC_KEY:$LANGFUSE_SECRET_KEY" "$LANGFUSE_BASE_URL/api/public/datasets"`
  lists the dataset.
- [ ] Confirm the eBay production application, its daily quota and the
  per-second limit from the Analytics API, and whether its application
  can simply be reused by spt (Q2).
  verify: `op run --environment $op_env -- spt ebay quota`
  (IMPL-0001 client) prints the limit and the current usage.
- [ ] Confirm two Discord webhooks: a test channel for `spt-dev` and the
  production channel the operator already watches. Record both in
  the 1Password Environment as `DISCORD_WEBHOOK_URL_DEV` and
  `DISCORD_WEBHOOK_URL`.
  verify: `op run --environment $op_env -- curl -fsS -X POST -H 'content-type: application/json' -d '{"content":"spt-dev probe"}' "$DISCORD_WEBHOOK_URL_DEV"`
  posts to the test channel only.
- [ ] Confirm the TypeSafe key (`JEV_API_KEY`), the OpenCode Console key
  (`OPENCODE_API_KEY`) and the Anthropic key (`ANTHROPIC_API_KEY`) are in
  the 1Password Environment and valid.
  verify: `op run --environment $op_env -- curl -fsS -H "Authorization: Bearer $JEV_API_KEY" https://api.typesafe.ai/v1/models`
  lists `jev-1.14.0`;
  `op run --environment $op_env -- curl -fsS -H "Authorization: Bearer $OPENCODE_API_KEY" https://opencode.ai/inference/v1/models`
  lists `jev-1.13` and `claude-haiku-5-5`.
- [ ] Decide and record how secrets reach the cluster (Open Question
  Q1) and confirm the chosen operator or controller is installed.
  verify: `kubectl get crd | grep -E 'externalsecrets|onepassword|sealedsecrets'`
  shows the chosen CRD.
- [ ] Record the first implementation's database DSN (read-only role)
  for catalog seeding; its deployment no longer runs, so only the data
  matters (IMPL-0001 Q7 covers the case where the database is gone too).
  verify: `psql "$V1_DATABASE_URL" -c 'select count(*) from listings'`
  returns.
- [ ] Confirm the release train: a merged PR with a `patch`, `minor` or
  `major` label tags and publishes binary, image and chart; `dont-release`
  skips. Record that integration fixes merge with `patch`.
  verify: `gh release list --limit 3` shows the latest tag and
  `gh api repos/donaldgifford/spt/labels | jq -r '.[].name' | grep -E '^(major|minor|patch|dont-release)$'`
  prints all four.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every fact above is recorded in the Dependencies section with the
  value found, not a placeholder.
- The secrets controller (Q1) and the eBay application (Q2) are in
  place before Phase 1 starts.
- No cluster resource has been created.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 1: Namespaces, databases, secrets and identity

Create the per-environment resources the chart consumes, prove the
Temporal dial from inside the cluster, and register the search
attributes.

<!--docz:tasks:start-->
#### Tasks

- [ ] Create the Temporal namespaces `spt-dev` and `spt` with a 30-day
  retention and the cluster's default archival.
  verify: `temporal operator namespace describe spt-dev` and
  `temporal operator namespace describe spt` both succeed.
- [ ] Create the CNPG databases `spt_dev` and `spt` with their `app`
  users through the cluster's `Database` resources, and enable
  `pg_trgm` in each.
  verify: `kubectl -n cnpg get database spt-dev spt` shows both ready;
  `psql "$SPT_DEV_DATABASE_URL" -c '\dx'` lists `pg_trgm`.
- [ ] Register the OIDC confidential client `spt` and issue the mTLS
  certificate and key for the `spt` Temporal identity, one per
  environment, stored in the 1Password Environment.
  verify: `op run --environment $op_env -- env | grep -c '^TEMPORAL_'`
  prints the eleven `TEMPORAL_*` names from DESIGN-0001.
- [ ] Create the Kubernetes namespaces `spt-dev` and `spt` and the
  Secrets named by the chart values (`spt-db-app`, `spt-temporal-tls`,
  `spt-temporal-oidc`, `spt-providers` with the eBay, Discord, model and
  Langfuse keys) through the path chosen in Q1, sourced from the
  1Password Environment.
  verify: `kubectl -n spt-dev get secret spt-db-app spt-temporal-tls spt-temporal-oidc spt-providers`
  lists all four and
  `kubectl -n spt-dev get secret spt-providers -o json | jq -r '.data | keys[]'`
  matches the DESIGN-0001 environment table.
- [ ] Prove the Temporal dial from inside the cluster with the spt image:
  a one-off pod running `spt worker --dry-run` (dial, version check,
  exit) with the Secrets mounted.
  verify: `kubectl -n spt-dev run spt-dial --rm -it --restart=Never --image=ghcr.io/donaldgifford/spt:<tag> --overrides="$(cat hack/dial-pod.json)" -- worker --dry-run`
  logs `temporal: connected` and exits 0.
- [ ] Register the search attributes `SptStage`, `SptWatch`, `SptType`,
  `SptSchemaVersion`, `SptComponentKey` in both namespaces (the worker
  does this at start; do it once by hand so the first start is not
  racing a schedule).
  verify: `temporal operator search-attribute list --namespace spt-dev`
  shows the five keys as `Keyword`.
- [ ] Run `spt migrate` against `spt_dev` from the same one-off pod.
  verify: `spt migrate --check` exits 0 and
  `psql "$SPT_DEV_DATABASE_URL" -c '\dt'` lists every DESIGN-0001 table.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Both environments have a namespace, a database, a Temporal namespace,
  an identity and the Secrets, created from recorded values.
- The spt image dials Temporal over mTLS with the OIDC bearer from
  inside the cluster.
- `spt_dev` is migrated and empty.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: The `spt-dev` release

Deploy the chart to `spt-dev`, run one watch against real eBay under a
quota reserve, and see every signal arrive where it should.

<!--docz:tasks:start-->
#### Tasks

- [ ] Add `charts/spt/values-dev.yaml`: `topology: split`,
  `temporal.namespace: spt-dev`, `database.existingSecret: spt-db-app`,
  `migrate.enabled: true`, `ui.enabled: false` until IMPL-0005,
  `config.file` with `ebay.quota_reserve = 0.75`, `decide.cascade =
  ["jev"]`, `cross_check` disabled, `escalate.budget_usd_per_day = 0.50`,
  `notify.discord_webhook = env("DISCORD_WEBHOOK_URL_DEV")`, and the
  OTel and Langfuse endpoints from Phase 0.
  verify: `helm template spt-dev charts/spt -f charts/spt/values-dev.yaml | spt config validate --from-configmap -`
  exits 0.
- [ ] Install the release and watch the migrate hook, then both
  Deployments, reach ready.
  verify: `helm -n spt-dev upgrade --install spt-dev charts/spt -f charts/spt/values-dev.yaml --wait --timeout 5m`
  exits 0 and `kubectl -n spt-dev get pods` shows api and worker Ready.
- [ ] Confirm readiness semantics: `/readyz` on the worker is 200 only
  after the build is promoted; on the api after the Postgres and
  Temporal pings.
  verify: `kubectl -n spt-dev port-forward deploy/spt-dev-worker 9090 & curl -fsS localhost:9090/readyz`
  returns 200 and the JSON lists every check as `ok`.
- [ ] Confirm the worker promoted its build and the baseline and
  catalog-digest Schedules exist.
  verify: `temporal worker deployment describe --namespace spt-dev --name spt`
  shows the image tag as current;
  `temporal schedule list --namespace spt-dev` lists `baseline` and
  `catalog-digest`.
- [ ] Create one watch through the CLI (a RAM watch with a narrow query)
  and confirm its Schedule and first tick.
  verify: `op run --environment $op_env -- spt watches create --name ram-ddr4-32gb --component ram --query "32GB DDR4 ECC RDIMM" --threshold 70`
  prints the watch id;
  `temporal schedule describe --namespace spt-dev --schedule-id watch:<id>`
  shows the next run; after the interval
  `temporal workflow list --namespace spt-dev --query 'WorkflowType="IngestWorkflow"'`
  lists one run.
- [ ] Confirm listings were fetched and extracted and that the quota
  reserve held.
  verify: `op run --environment $op_env -- spt listings list --watch <id> --output json | jq length`
  is greater than zero and
  `op run --environment $op_env -- spt quota` shows usage below 25% of
  the daily limit.
- [ ] Confirm Discord delivery to the test channel for any alert, and
  that the batch respected the ten-embed limit.
  verify: `op run --environment $op_env -- spt alerts list --watch <id> --delivered --output json | jq length`
  matches the embeds seen in the test channel; the worker log shows
  `notify: batch size` values of at most 10.
- [ ] Confirm Langfuse traces: one trace per listing with
  `sessionId` equal to the `ExtractWorkflow` ID and one generation per
  decider call.
  verify: `op run --environment $op_env -- curl -fsS -u "$LANGFUSE_PUBLIC_KEY:$LANGFUSE_SECRET_KEY" "$LANGFUSE_BASE_URL/api/public/traces?sessionId=extract:<item>:<schema>" | jq '.data | length'`
  is 1.
- [ ] Confirm metrics and traces reach the collector and Prometheus:
  the ServiceMonitor targets are up and the SDK metrics appear.
  verify: `kubectl -n observability port-forward svc/prometheus 9091 & curl -fsS 'localhost:9091/api/v1/query?query=up{namespace="spt-dev"}' | jq '.data.result | length'`
  is 2; `temporal_request_total` and `spt_ingest_items_total` return
  series.
- [ ] Import the generated dashboards and prove one PrometheusRule fires
  by scaling the worker to zero for eleven minutes.
  verify: `kubectl -n spt-dev scale deploy/spt-dev-worker --replicas=0`
  then after eleven minutes
  `curl -fsS localhost:9091/api/v1/alerts | jq '.data.alerts[] | select(.labels.alertname=="SptWorkerNotPolling")'`
  shows the alert firing; scale back to 1 and confirm it resolves.
- [ ] Confirm a worker restart mid-tick loses nothing: delete the worker
  pod during an ingest and check the tick completes on the new pod.
  verify: `kubectl -n spt-dev delete pod -l app.kubernetes.io/component=worker`
  during a tick; `temporal workflow describe --namespace spt-dev --workflow-id ingest:<id>:<tick>`
  shows `Completed` afterwards.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The `spt-dev` release is Ready from a values file alone, with no
  manual steps beyond the Secrets from Phase 1.
- One watch produces listings, decisions, baselines (after the first
  hourly run) and at least one delivered alert in the test channel.
- Traces, metrics and Langfuse generations are visible; one alert rule
  has been proven to fire and resolve.
- eBay quota usage stays under 25% of the daily limit while the reserve
  is 75%.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: Live provider admission and calibration

Every decider route is admitted through the live contract suite and the
golden dataset before it is enabled; thresholds are set from measured
calibration, not guessed; the cross-check and the escalator go live on
`spt-dev` with budgets.

<!--docz:tasks:start-->
#### Tasks

- [ ] Run the `decidertest` live suite against TypeSafe `jev-1.14.0`
  with a recorded budget, and record the request payloads as fixtures
  under `testdata/jev/live/`.
  verify: `op run --environment $op_env -- spt decide --decider jev --suite --budget-usd 0.50`
  passes every contract check and prints the spend.
- [ ] Run the live suite against Anthropic `claude-haiku-5-5` directly,
  then against the OpenCode-translated Anthropic route, then the
  OpenCode-translated OpenAI route with `gpt-5.4-mini`. This is the
  cluster-side confirmation of IMPL-0002's phase 0 finding that the
  translated routes pass structured-output parameters through.
  verify: `op run --environment $op_env -- spt decide --decider haiku --suite --budget-usd 0.50`,
  `op run --environment $op_env -- spt decide --decider sonnet-opencode --suite --budget-usd 0.50`
  and `op run --environment $op_env -- spt decide --decider gpt-mini-opencode --suite --budget-usd 0.50`
  all pass with zero `ErrNotAnOption`.
- [ ] Run the golden dataset regression through every admitted decider
  and record a Langfuse dataset run per decider at the current commit.
  verify: `op run --environment $op_env -- just test-regression`
  prints accuracy per component type and a calibration table per
  decider;
  `curl -fsS -u "$LANGFUSE_PUBLIC_KEY:$LANGFUSE_SECRET_KEY" "$LANGFUSE_BASE_URL/api/public/dataset-runs?datasetName=spt-golden" | jq '.data | length'`
  has grown by the number of deciders.
- [ ] Set the registry thresholds (`τ_type`, `τ_field`, `τ_quantity`,
  `τ_match`, and the band edges for the companion and verdict bools if
  the table says 0.3 and 0.7 are wrong) from Jev's calibration table:
  the lowest bin
  meeting each question's target. Open a registry PR with the tables
  pasted; merge with `patch`.
  verify: `spt registry diff main` shows only threshold changes and
  `just test-regression` is pasted into the PR body.
- [ ] Enable the cross-check at 10% with `haiku` and
  `on_disagreement = "log"` in `values-dev.yaml`; upgrade the release.
  verify: `op run --environment $op_env -- spt decisions agreement --since 24h`
  after one day prints agreement per question for the `jev`/`haiku` pair and
  `curl -fsS 'localhost:9091/api/v1/query?query=sum(spt_decide_agreement_total)'`
  is non-zero.
- [ ] Enable the escalator live with `budget_usd_per_day = 0.50` and
  confirm the budget guard: exhaust it with manual `spt decide` calls
  and see listings go to `AwaitingLabel`.
  verify: `temporal workflow list --namespace spt-dev --query 'SptStage="AwaitingLabel"' | wc -l`
  increases after the budget is spent and
  `curl -fsS 'localhost:9091/api/v1/query?query=spt_budget_remaining_usd{budget="escalate"}'`
  reads 0.
- [ ] Label every `AwaitingLabel` listing from the first day through the
  CLI, so the calibration join has its first operator rows.
  verify: `op run --environment $op_env -- spt listings list --stage awaiting_label --output json | jq length`
  returns 0 afterwards and
  `psql "$SPT_DEV_DATABASE_URL" -c "select count(*) from labels where source='operator'"`
  is greater than zero.
- [ ] Reorder-consistency check on the catalog chooser and classify
  questions (the model leans toward the first option): run the golden
  set with options shuffled and compare.
  verify: `op run --environment $op_env -- just test-regression --shuffle-options`
  differs from the unshuffled run on under 2% of answers; larger
  differences are filed as registry issues before Phase 4.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Five routes (TypeSafe Jev, OpenCode Jev, Anthropic direct, OpenCode
  Anthropic, OpenCode OpenAI) pass the live contract suite; any that
  fails is left disabled and its failure is recorded.
- Registry thresholds come from a committed calibration table, with Jev
  at or above the per-question targets on the golden dataset.
- The cross-check and the escalator are live on `spt-dev` under budgets
  that have been proven to stop them.
- Escalation rate on `spt-dev` is under 10% of listings over three
  days.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: All watches, soak and tuning

All watches on `spt-dev`, two weeks of daily labelling, the catalog
seeded and approved, and one threshold tuning from the labels the soak
produces.

<!--docz:tasks:start-->
#### Tasks

- [ ] Create every watch the operator wants, through `spt watches
  create`, with the reserve from Q2.
  verify: `op run --environment $op_env -- spt watches list --output json | jq length`
  equals the intended count and
  `temporal schedule list --namespace spt-dev | grep -c '^watch:'`
  matches.
- [ ] Seed the catalog from the first implementation's data as proposals
  (`spt catalog seed --from-v1`), then do the approval pass with
  `spt catalog approve` per row or `--all --component` where obviously
  right.
  verify: `psql "$SPT_DEV_DATABASE_URL" -c "select status, count(*) from catalog_proposals group by 1"`
  shows no `pending` rows and
  `psql "$SPT_DEV_DATABASE_URL" -c "select component_type, count(*) from catalog group by 1"`
  shows rows for `server`, `cpu`, `gpu`.
- [ ] Label daily for fourteen days: every alert the dev channel posted
  (true or false positive) and a sample of ten non-alert listings per
  watch, through `spt alerts list --since 1d` and `spt listings label`;
  record each day's counts in a table under this task.
  verify: `psql "$SPT_DEV_DATABASE_URL" -c "select date(created_at), count(*) from labels group by 1 order by 1" | grep -c '2026-'`
  is 14.
- [ ] Compute alert precision from the labelled alerts and estimated
  recall from the labelled non-alert sample over the window, per watch
  and overall (Q3).
  verify: `op run --environment $op_env -- spt decisions report --since 14d`
  prints precision and recall and the table is pasted into this
  document.
- [ ] Tune thresholds once from the calibration table over the window's
  labels; open the registry PR with the tables; merge with `patch`.
  verify: `op run --environment $op_env -- spt decisions calibration --since 14d`
  is pasted into the PR and `spt registry diff main` shows thresholds
  only.
- [ ] Review the cross-check disagreements over the window and record
  which decider was right per question, from the labels.
  verify: `op run --environment $op_env -- spt decisions agreement --since 14d --with-labels`
  prints a per-question table with a "right" column for each decider.
- [ ] Confirm no workflow is stuck: nothing in `AwaitingLabel` older than
  `label_timeout`, no `IngestWorkflow` running longer than
  `tick_deadline`, no failed activity in terminal retry.
  verify: `temporal workflow list --namespace spt-dev --query 'ExecutionStatus="Running" AND StartTime < "<now minus 7d>"' | wc -l`
  is 0 and `temporal workflow list --namespace spt-dev --query 'ExecutionStatus="Failed"' | wc -l`
  is 0 for the window.
- [ ] Confirm spend: the decide and escalate budgets were never exhausted
  after Phase 3's deliberate test, and total model spend for the window
  is recorded.
  verify: `psql "$SPT_DEV_DATABASE_URL" -c "select name, sum(spent_usd) from budgets where day >= current_date - 14 group by 1"`
  is pasted into this document.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Fourteen days of labels recorded; every alert in the window labelled.
- Alert precision over the window at or above 0.80 overall, and no
  watch below 0.60; estimated recall from the non-alert sample reported.
- Escalation rate under 10%; cross-check agreement reported per
  question with which decider was right.
- Zero stuck workflows older than `label_timeout`; zero failed
  workflows; budgets never exhausted outside the deliberate test.
- Catalog seeded and approved; proposal queue empty at the end of the
  window.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: Promotion to prod and operations

Promote to the `spt` release and leave the operational notes an
operator needs.

<!--docz:tasks:start-->
#### Tasks

- [ ] Add `charts/spt/values-prod.yaml`: the dev file with
  `temporal.namespace: spt`, the prod database Secret, the prod webhook,
  `ebay.quota_reserve = 0.10`, `cross_check.sample = 0.10` (Q6), and the
  tuned registry from Phase 4 in the released image.
  verify: `diff <(yq -P charts/spt/values-dev.yaml) <(yq -P charts/spt/values-prod.yaml)`
  shows only the intended keys.
- [ ] Install the `spt` release in the `spt` namespace, migrate, and
  recreate the watches from `spt-dev` by export and import.
  verify: `helm -n spt upgrade --install spt charts/spt -f charts/spt/values-prod.yaml --wait`
  exits 0 and
  `op run --environment $op_env -- spt watches list --output json | jq length`
  against prod equals the dev count.
- [ ] Copy the approved catalog and the labels from `spt_dev` to `spt`
  (`pg_dump` of `catalog`, `labels` only) so prod starts with the seeded
  identities and the calibration history.
  verify: `psql "$SPT_DATABASE_URL" -c "select count(*) from catalog"`
  equals the dev count.
- [ ] Let prod run for one full baseline window (one hour) and one tick
  per watch; confirm alerts land in the prod channel (Q5) and record the
  date prod went live.
  verify: `op run --environment $op_env -- spt alerts list --delivered --since 2h --output json | jq length`
  against prod is greater than zero.
- [ ] Scale `spt-dev` down to its minimum (worker 1, api 1, quota
  reserve 90%) so dev keeps exercising the pipeline without competing
  for quota.
  verify: `helm -n spt-dev get values spt-dev | grep quota_reserve`
  shows 0.90.
- [ ] Write the operational notes into `README.md` under "Operating
  spt": where to look when a listing is stuck (`spt runs journal`, the
  Temporal UI link), how to pause a watch (`spt watches disable`), how to
  replay a tick (`spt ingest --watch`), how to re-extract under a new
  registry (`spt ingest --full --schema-version`), how to read the
  budgets and quota (`spt quota`, the dashboard), and how to approve
  proposals from the CLI until the UI ships.
  verify: `markdownlint-cli2 README.md` is clean and every command
  named exists in `docs/cli/`.
- [ ] Hold the post-promotion review after 14 days of prod: alert
  precision from the prod labels, escalation rate, agreement, spend,
  stuck-workflow count; record it in this document and open issues for
  anything below the Phase 4 bar.
  verify: `op run --environment $op_env -- spt decisions report --since 14d`
  against prod is pasted below this task.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The `spt` release serves the production channel from the recorded
  date.
- Prod started with the seeded catalog and the dev labels.
- `README.md` carries the operational notes and every command in them
  exists.
- The post-promotion review is recorded and meets the Phase 4 bar.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `charts/spt/values-dev.yaml` | Create | `spt-dev` release values: split topology, dev namespace and database, quota reserve, dev webhook, cascade and budgets |
| `charts/spt/values-prod.yaml` | Create | `spt` release values; differs from dev only in namespace, Secrets, webhook and reserve |
| `hack/dial-pod.json` | Create | pod overrides for the one-off dial and migrate pod |
| `README.md` | Modify | "Operating spt" section |
| `pkg/registry/registry.hcl` | Modify | thresholds set from calibration (two PRs, Phase 3 and Phase 4) |
| `testdata/jev/live/` | Create | recorded live request and response pairs from the admission suite |
| `docs/impl/0004-*.md` (this file) | Modify | recorded facts, tables and review results as phases complete |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] The `decidertest` live suite as the admission gate for each
  provider route, with recorded payloads committed as fixtures.
- [ ] `just test-regression` on the golden dataset per decider, recorded
  as Langfuse dataset runs at each registry change.
- [ ] Cluster verification by the `verify:` commands in each phase:
  readiness, promotion, schedules, quota, delivery, traces, metrics,
  alert rules, restart safety, budget guards, stuck-workflow queries.
- [ ] The fourteen-day labelled soak as the acceptance test for
  promotion.

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- IMPL-0001, IMPL-0002 and IMPL-0003 merged and released; the image tag
  used in Phase 1 is a release tag, not a dev build.
- Cluster facts recorded in Phase 0 (fill in as confirmed):
  - Temporal: address, server version, worker versioning enabled,
    namespaces `spt-dev` and `spt`.
  - OIDC issuer, audience, client `spt`; OpenBao PKI role for the `spt`
    mTLS identity.
  - CNPG cluster name; databases `spt_dev` and `spt`.
  - OTel collector endpoint.
  - Langfuse base URL, project, dataset name.
  - eBay application id, daily quota, per-second limit; whether the
    first implementation's application was reused (Q2).
  - Discord webhooks: dev test channel, production channel.
  - Secrets path (Q1) and its controller.
  - First implementation: read-only DSN for seeding.
- The spt 1Password Environment (ID in `justfile` from IMPL-0001) with
  the eleven `TEMPORAL_*` names, the provider keys and both webhooks.
- The TypeSafe agent skill installed in this repository (used when
  registry criteria are revised after calibration).

<!--docz:dependencies:end-->

## Open Questions

All questions were decided on 2026-11-14. The bodies above reflect the
decisions. Q3 and Q4 as originally posed (a comparison metric and a
cutover trigger against the first implementation) are moot: the first
implementation no longer runs, so there is no side-by-side window and
nothing to cut over from. They are recorded as re-framed.

**Q1. How do secrets reach the cluster?** Decided: option a.
External Secrets Operator with the 1Password provider, syncing items
from the spt 1Password Environment into Kubernetes Secrets named exactly
as the chart values expect; rotation is a 1Password edit. Alternatives
considered: sealed-secrets with manual rotation; the 1Password
Kubernetes operator with `OnePasswordItem` resources.

**Q2. eBay application and quota.** Decided: option b, with direction.
Reuse the first implementation's eBay application (nothing else is
consuming its quota now), with a 50% reserve on `spt-dev` during the
soak and 10% in prod. If reusing it turns out to be awkward, or a second
application is simply easier, swap to option a: a second application
registered for spt with its own quota. Alternative a as originally
posed: a second application so two running systems have independent
quotas.

**Q3. Soak metric.** Decided: re-framed.
Originally precision and recall against the labelled union of both
systems' alerts. With one system, the metric is alert precision from
the operator's labels on every alert in the window, plus estimated
recall from a daily labelled sample of non-alert listings per watch; the
same labels feed calibration.

**Q4. Promotion trigger.** Decided: re-framed.
Originally two consecutive weeks of beating the first implementation.
Now: the Phase 4 success criteria hold over the fourteen-day soak
(precision bar, escalation rate, no stuck or failed workflows, budgets
intact, proposal queue empty). The operator promotes when they hold;
there is no date-driven alternative.

**Q5. Production Discord channel.** Decided: option a.
The same channel the operator already watches; nothing else posts to it
now, so habits and history stay in one place. Alternative: a new channel
for spt.

**Q6. Cross-check sample in production.** Decided: option a.
Keep 10% in prod with the decide budget as the cap, so agreement and
per-decider calibration keep accruing on live traffic; `spt-dev` runs
100% for new-model admission. Alternative: 0% in prod and 100% in dev.

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [ADR-0001 Every finite question is a typed decision with a calibrated probability](../adr/0001-every-finite-question-is-a-typed-decision-with-a-calibrated.md); [ADR-0002 Temporal is the execution plane; Postgres is the system of record](../adr/0002-temporal-is-the-execution-plane-postgres-is-the-system-of-record.md); [ADR-0005 Configuration is HCL decoded with hclkit](../adr/0005-configuration-is-hcl-decoded-with-hclkit.md)
- [DESIGN-0001 System architecture](../design/0001-system-architecture-roles-configuration-deployment-and-data.md) (rollout, environment table, chart values); [DESIGN-0002 Durable execution](../design/0002-durable-execution-workflows-activities-queues-and-signals.md); [DESIGN-0003 Decisions](../design/0003-decisions-the-decider-contract-cascade-escalation-and.md) (admission suite, cross-check, calibration); [DESIGN-0005 Catalog](../design/0005-catalog-canonical-identities-retrieval-and-proposals.md) (seeding)
- [IMPL-0001 Foundation](../impl/0001-phase-1-foundation-roles-configuration-store-ebay-client.md); [IMPL-0002 Typed extraction](../impl/0002-phase-2-typed-extraction-deciders-registry-catalog-escalator.md); [IMPL-0003 Durable execution](../impl/0003-phase-3-durable-execution-temporal-plumbing-workflows.md); [IMPL-0005 Operator UI](../impl/0005-phase-5-operator-ui-bff-alerts-review-catalog-runs-decisions.md)
- [repo-guardian DESIGN-0028 Temporal client auth: OIDC over mTLS with OpenBao-issued certificates](https://github.com/donaldgifford/repo-guardian/blob/main/docs/design/0028-temporal-client-auth-oidc-over-mtls-with-openbao-issued.md); [repo-guardian `charts/repo-guardian`](https://github.com/donaldgifford/repo-guardian/tree/main/charts/repo-guardian)
- [homelab ADR-0013 Use Temporal (self-hosted) for non-infra automation](https://github.com/donaldgifford/homelab/blob/main/docs/adr/0013-use-temporal-self-hosted-for-non-infra-automation.md)
- [Temporal CLI](https://docs.temporal.io/cli); [Temporal worker versioning](https://docs.temporal.io/worker-versioning); [CloudNativePG `Database` resource](https://cloudnative-pg.io/documentation/current/declarative_database_management/); [External Secrets Operator 1Password provider](https://external-secrets.io/latest/provider/1password-sdk/); [1Password `op run`](https://www.1password.dev/cli/reference/commands/run)
- [TypeSafe models](https://docs.typesafe.ai/go/models.md); [OpenCode Console](https://opencode.ai/docs/zen/); [Langfuse datasets](https://langfuse.com/docs/datasets/overview)
- [eBay Analytics rate limits](https://developer.ebay.com/api-docs/developer/analytics/resources/rate_limit/methods/getRateLimits); [Discord webhook rate limits](https://discord.com/developers/docs/topics/rate-limits)

<!--docz:references:end-->
