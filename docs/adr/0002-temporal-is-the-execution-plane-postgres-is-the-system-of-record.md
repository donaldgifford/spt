---
id: ADR-0002
title: "Temporal is the execution plane; Postgres is the system of record"
status: Proposed
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0002: Temporal is the execution plane; Postgres is the system of record

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

Everything about *running* the pipeline (scheduling a watch, retrying an
eBay call, pacing calls to a rate limit, waiting days for an operator,
deduplicating an item, asking where a listing is) is a Temporal primitive.
Everything the pipeline *knows* (watches, listings, decisions, labels,
catalog, baselines, alerts, spend counters) is a Postgres row. Not a
hand-built plane on Postgres, because that was built twice and both times
it was where the operational pain lived. Not a job queue, because a queue
gives retries and nothing else on the list. Not workflow state as the
database, because histories are not queryable by business attributes.

<!--docz:summary:end-->

<!--docz:context:start-->
## Context

The work for one listing is a sequence of external calls, each of which
can fail, be rate-limited or be billed: fetch from eBay, two or three
decider calls, a catalog lookup, possibly a text-model escalation, a store
write, a score, a verdict, a Discord post. Some sequences pause for days
while a human labels. A watch runs every fifteen minutes and must not
overlap itself. eBay allows a few calls per second and a few thousand per
day; the models have daily spend ceilings.

The forces:

1. **Per-item durable state across many steps.** A listing's progress
   must survive a worker restart mid-sequence without a table that
   records which step it reached.
2. **Declared retries, timeouts and backoff per external call**, not
   coded per client.
3. **Rate limits as settings.** eBay's per-second limit and Discord's
   bucket limits should be one knob each, not a token bucket in a table.
4. **Humans in the loop.** A listing waiting on an operator must park for
   up to a week and resume when the label arrives, with the label and the
   resumed execution in one record.
5. **Deduplication by identity.** The same item delivered twice must be a
   no-op; the same item under a new registry version must be new work.
6. **Schedules that do not overlap**, with jitter.
7. **Answerable from the outside.** "Where is item X and why is it
   there" must be a query, not a SQL session over five tables.
8. **Business data queryable by SQL.** The UI, the calibration report
   and the baselines need joins over decisions, labels and listings.
9. **Operable by one person on this cluster.** Whatever runs the plane
   must already be running, monitored and authenticated here.

Two implementations of this product built the plane by hand. The first
had a cron scheduler, a database-backed queue, a database-backed token
bucket, a judge budget loop, five polled tables and a 581-line SQL
runbook. The second set out to do it again with Postgres and Valkey and
stalled there. In both, the plane was where the pain lived and none of
the product's novelty.

<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

**Temporal owns execution.** One Schedule per watch with overlap `SKIP`
and jitter; an `IngestWorkflow` per tick and an `ExtractWorkflow` per
listing as a child with ID `extract:<ebay-item-id>:<schema-version>` and
reject-duplicate reuse; every external call an activity with timeout and
retry declared in one central table; task queues whose worker options
carry the rate and concurrency limits; `operator-label` and catalog
signals for humans; worker deployment versioning for rollouts; search
attributes for stage, watch, type and registry version.

**Postgres owns knowledge.** Watches, listings, decisions (one row per
question per listing per registry version), labels, catalog and
proposals, baselines, alerts with delivery state, and the eBay quota and
model-spend counters, incremented in the same transaction as the result
they account for. Persist activities are idempotent upserts. No table has
a status column that a loop polls.

**Two roles from one binary.** `spt api` starts, signals and queries
workflows by name and holds no workflow code; `spt worker` hosts
workflows and activities; `spt all` runs both. Workflows, activities,
queues, signals and plumbing are specified in DESIGN-0002.

<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- There is no scheduler, queue, token bucket, lock table or budget loop
  to write, and no migration for any of them.
- "Why is this listing stuck" is a workflow in retry with the error
  attached, visible in the runs view and the Temporal UI.
- Deduplication is the workflow ID. Re-extraction under a new registry
  version is a new ID.
- Operator labelling is synchronous with the pipeline.
- The same operational model, auth and test harness as the other
  services on the cluster.

<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- Nothing runs without a Temporal server. Unit tests use the SDK test
  suite; integration tests use the cached dev server; development uses
  `spt all` against the dev server.
- One child workflow per listing produces history. Above roughly 100k
  listings per day, batch items per child.
- Persist activities must be idempotent and counters transactional; this
  is a discipline the store layer has to keep.
- Notification must be batched by the parent tick to respect Discord's
  ten embeds per post.

<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- `POST /api/v1/ingest` remains a manual trigger; it starts a workflow
  instead of running inline.
- Pipeline metrics come from the worker's metrics handler and the
  workflows' metric handles rather than from an engine loop.

<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **A hand-built plane on Postgres, with or without Valkey.** Done twice.
  The first implementation's `internal/engine` was nine files and about
  4,700 lines, plus migrations for `scheduler_locks`, `job_runs`,
  `extraction_queue`, `rate_limiter_state` and `judge_scores`, plus the
  SQL runbook. Every item on that list is a Temporal primitive here. It
  fails force 7 by design (every operational question is SQL), force 1
  only with a status column per step, and force 4 with a parked row that
  something has to poll. Its one advantage, no new dependency, is void
  on a cluster where Temporal already runs (force 9).
- **A Postgres job queue (River).** Removes the polled queue table and
  gives retries with backoff per job (force 2). It does not give durable
  state across several steps of one item (force 1), signals (force 4),
  schedules with overlap control (force 6), or a history to inspect
  (force 7). The scheduler, the limiter and the parked-for-a-week row
  would still be hand-built around it.
- **NATS JetStream work queues.** Same gap as a job queue, with a second
  messaging system to operate and no answer to force 4 or 7.
- **Kubernetes CronJobs running a stateless binary.** Removes the
  scheduler (force 6, without jitter or overlap control beyond
  `concurrencyPolicy`). Every other piece of state returns to tables,
  and a run that dies mid-listing starts over from the top.
- **Workflow state as the database.** Keep listing records as workflow
  memory and answer the UI through workflow queries. Histories are
  replayed and archived, cannot be joined, and are not queryable by
  business attributes (force 8). Search attributes exist for listing
  work by state, not for calibration reports.
- **Entity workflows holding budgets as leases.** The pattern the other
  services on the cluster use for per-installation GitHub budgets. eBay
  has one application and one budget, so a task-queue rate limit plus a
  transactional counter covers forces 2 and 3 with less machinery. The
  lease pattern is the fallback if per-watch fairness is ever needed.

What tipped it: forces 1, 4 and 7 together. Only an engine that keeps
per-item execution state, can park on a signal, and shows the history
removes the status-column-plus-poller shape, and that engine is already
running and authenticated on the cluster.

<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [ADR-0005 Configuration is HCL decoded with hclkit](../adr/0005-configuration-is-hcl-decoded-with-hclkit.md) (`TEMPORAL_*` environment)
- [ADR-0006 The API is a spec-first OpenAPI contract served on net/http](../adr/0006-the-api-is-a-spec-first-openapi-contract-served-on-nethttp.md) (api role)
- [DESIGN-0001 System architecture](../design/0001-system-architecture-roles-configuration-deployment-and-data.md); [DESIGN-0002 Durable execution: workflows, activities, queues and signals](../design/0002-durable-execution-workflows-activities-queues-and-signals.md)
- [server-price-tracker `internal/engine`](https://github.com/donaldgifford/server-price-tracker/tree/main/internal/engine) and [`docs/SQL_HELPERS.md`](https://github.com/donaldgifford/server-price-tracker/blob/main/docs/SQL_HELPERS.md) (the hand-built plane); [DESIGN-0001 Server Price Tracker architecture](https://github.com/donaldgifford/server-price-tracker/blob/main/docs/design/0001-server-price-tracker-architecture.md)
- [homelab ADR-0013 Use Temporal (self-hosted) for non-infra automation](https://github.com/donaldgifford/homelab/blob/main/docs/adr/0013-use-temporal-self-hosted-for-non-infra-automation.md)
- [boop ADR-0001 Use Temporal as the control plane](https://github.com/donaldgifford/boop/blob/main/docs/adr/0001-use-temporal-as-the-control-plane.md); [boop ADR-0002 One entity workflow per repository](https://github.com/donaldgifford/boop/blob/main/docs/adr/0002-one-entity-workflow-per-repository.md)
- [repo-guardian DESIGN-0026 v2 Temporal control plane and role split](https://github.com/donaldgifford/repo-guardian/blob/main/docs/design/0026-v2-temporal-control-plane-and-role-split.md)
- [Temporal Go SDK](https://docs.temporal.io/develop/go); [River](https://riverqueue.com)

<!--docz:references:end-->
