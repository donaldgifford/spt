# `internal/datastore`

Postgres-backed canonical store. Phase 6 (IMPL-0001) ships the
`Datastore` interface contract + sentinel errors; Phase 8 ships the
`Migrator` and the goose migration runner. The Postgres
implementation of `Datastore` lands with the datastore IMPL — Phase 8
gives that IMPL a working schema bootstrap to layer onto.

## Migrations

Migrations live in [`migrations/`](./migrations/) as numbered
`.sql` files. They are embedded into the `spt` binary at build time
(`migrations/embed.go`); `--migrations-dir` swaps in a filesystem
path for local iteration.

### Operator workflow (no auto-migrate)

Per [IMPL-0001 Resolved Decision #12](../../docs/impl/0001-foundation-go-layout-cli-config-observability-and-migrations.md#resolved-decisions),
roles do **not** auto-migrate on startup. Each role's `Run` calls
`Migrator.Status` and fails fast on pending migrations with a clear
error so the operator runs the explicit step:

```sh
spt --postgres-dsn=$DATABASE_URL migrate up
```

Then deploy the role pods. This matches the Kubernetes
Job / initContainer pattern that the Helm chart will package.

### Local dev recipes

`just` wraps the same flow against the Compose Postgres in
`test/integration/docker-compose.yml` (DSN
`postgres://spt:spt@127.0.0.1:55432/spt`):

```sh
just test-integration       # brings the Compose stack up via `up -d --wait`
just db-up                  # apply pending migrations
just db-status              # show applied/pending table
just db-down                # roll back the last migration
```

`SPT_DSN=postgres://...` overrides the recipe's default DSN to point
at another database.

### Authoring migrations

Use goose's timestamp convention so files sort lexically across
contributors:

```
internal/datastore/migrations/YYYYMMDDHHMMSS_<snake_name>.sql
```

Each file pairs an `-- +goose Up` block with `-- +goose Down`.
Phase 8 ships only `00001_initial.sql` (placeholder `_spt_meta`
table). Real DDL — watches, listings, components, jobs, tasks,
alerts, notifications, scores, judgments, market signals — lands
with the datastore IMPL.
