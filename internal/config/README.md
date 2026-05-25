# `internal/config`

Typed configuration model and HCL2 loader for `spt`. Owns the
`config.Config` struct that every role's `Run(ctx, *Config)` receives.

## Layering precedence

1. **Defaults** — `config.Defaults()`. Documented per field below.
2. **HCL files** — `--config <path>` (repeatable) and `--config-dir <dir>`
   (every `*.hcl` in lexical order). Loaded in this order, with each
   file's blocks overriding the previous file's via
   `hcl.MergeFiles`.
3. **Env vars** — accessed inline through the `env("VAR_NAME")` HCL
   function, evaluated at decode time. There is no implicit
   `SPT_SECTION_FIELD` env-var mapping; the operator decides which
   fields read from env by writing the call in their config.
4. **CLI flags** — `--log-format`, `--log-level`, `--admin-addr`,
   `--ebay-app-id`, `--ebay-cert-id`, `--postgres-dsn`,
   `--valkey-addr`, `--meili-url`. Only flags the user actually typed
   override; flag *defaults* do not (we use cobra's `Flag.Changed`).

## Discovery: explicit-only

There is no automatic XDG / `/etc/spt/` / `$SPT_CONFIG` lookup. If you
want a config file loaded, name it. Per [IMPL-0001 Resolved Decision
#3](../../docs/impl/0001-foundation-go-layout-cli-config-observability-and-migrations.md#resolved-decisions),
this avoids the "which config did I just load?" footgun. With no
`--config` and no `--config-dir`, the loader returns
`Defaults()` + env (via flag overrides) + CLI flags.

## Block reference

See [`test/config/example.hcl`](../../test/config/example.hcl) for a
complete annotated example. The schema lives in
[`internal/config/types.go`](./types.go); the rest of this section
summarizes per-block intent.

| Block         | Purpose                                                                            |
|---------------|------------------------------------------------------------------------------------|
| `log`         | Output format (`text`/`json`/`auto`) and level (`debug`/`info`/`warn`/`error`).    |
| `admin`       | Per-role admin server address (`/healthz`, `/readyz`, `/metrics`).                 |
| `ebay`        | eBay Browse/Marketplace API credentials and rate-limit (DESIGN-0003).              |
| `postgres`    | Canonical store DSN + connection-pool sizing (ADR-0004).                           |
| `valkey`      | Queue + cache (ADR-0005).                                                          |
| `meilisearch` | Search index (ADR-0006).                                                           |
| `obs`         | OTel OTLP endpoint + Langfuse credentials + span sampling (ADR-0008/0009).         |
| `api`         | `api` role HTTP server: address and read/write timeouts.                           |
| `scheduler`   | Orchestrator cron intervals (DESIGN-0005).                                         |
| `worker`      | One `pools "<stage>" { concurrency = N }` block per stage (DESIGN-0005).           |
| `watch`       | Bootstrap Watch declarations seeded into the datastore at startup (see below).    |

## Watch blocks: parsed in Phase 3, seeded later

Per [IMPL-0001 Resolved Decisions #4 and
#5](../../docs/impl/0001-foundation-go-layout-cli-config-observability-and-migrations.md#resolved-decisions),
the HCL `watch "<name>" { ... }` block is the bootstrap-and-seed
surface. The runtime CRUD path is the API. Phase 3 only parses and
validates these blocks; the seed logic (read `cfg.Watches`,
upsert into the `watches` table) lives in the datastore IMPL that owns
that table.

## Validation

`config.Validate` aggregates every problem into a single
`*config.ValidationError` so one run surfaces every issue. Phase 3
catches:

- Malformed `log.format` / `log.level` values.
- Malformed Go duration strings (`scheduler.*`, `api.*_timeout`,
  `watch[*].cadence`).
- `obs.span_sampling` outside `[0, 1]`.
- `watch` blocks missing `query`.
- `worker.pools[*].concurrency <= 0`.
- `judge_sample_rate` outside `[0, 1]`.

Required-field enforcement for production dependencies (Postgres DSN,
eBay credentials) is deferred until those clients land in Phase 4+ —
forcing them now would block local development on stub roles that
don't open the connections.

## Errors

Sentinel errors live in [`errors.go`](./errors.go):

- `ErrParse` — HCL syntax failure (diagnostics wrapped).
- `ErrDecode` — gohcl could not map the body onto `Config`.
- `ErrInvalidDuration` — malformed Go duration string.
- `ErrRequired` — empty required field.
- `ErrOutOfRange` — numeric outside its documented range.
- `ErrReadFile` — couldn't read a file from disk.

`*ValidationError` implements `Is(target error) bool`, so
`errors.Is(err, config.ErrRequired)` works on the aggregate.
