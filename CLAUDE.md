# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project state

`spt` (Server Price Tracker) is an early-stage Go CLI scaffold. Per [IMPL-0001](docs/impl/0001-foundation-go-layout-cli-config-observability-and-migrations.md), Phases 1–4 are complete:

- **Phase 1**: package tree from [DESIGN-0001](docs/design/0001-go-application-layout-and-conventions.md) in place with `doc.go` placeholders.
- **Phase 2**: cobra root + role scaffolding. `spt` runs with subcommands `api`, `scheduler`, `worker`, `migrate {up,down,status}`, and `version` (`--json` for machine-readable). Roles log a startup line, block on `ctx.Done()`, and exit clean on SIGINT/SIGTERM.
- **Phase 3**: HCL2 config loader (`internal/config/`). Layering precedence: defaults → HCL files (lexical `--config-dir`, then explicit `--config`) → env vars (via the `env("VAR")` HCL function) → CLI flags (`--ebay-app-id`, `--postgres-dsn`, …). Validation aggregates every problem into a single `*config.ValidationError`. Sample config at `test/config/example.hcl`; schema doc at `internal/config/README.md`.
- **Phase 4**: observability core (`internal/obs/`). `obs.Setup(ctx, cfg, serviceName)` returns a `*Obs{Logger, TracerProvider, Registry}` plus a shutdown fn. OTel TracerProvider wires the system OTLP exporter and an agent-only filter (`categoryFilterProcessor`) for Langfuse — the Langfuse exporter is plumbed but nil until the agent IMPL lands. Use `obs.SetCategory(span, obs.SpanCategoryAgent)` to route a span. `obs.LoggerFromContext(ctx)` attaches `trace_id`/`span_id` when a span is active.
- **Phase 5**: admin endpoints (`internal/health/`). Every role serves `/healthz`, `/readyz`, and `/metrics` on `cfg.Admin.Addr` (default `:9090`). `health.New(registry)` + `RegisterReadiness(name, probe)` then `Serve(ctx, addr)`. `/readyz` runs every probe with a 2s timeout and returns per-probe JSON status. Listener opens synchronously so `Addr()` is reliable for `":0"`-bound test servers.
- **Phase 6**: service interface skeletons. `queue.Queue`, `datastore.Datastore`, `search.Search`, `cache.Cache`, `pipeline.Scheduler`, `agent.Agent`, and `ebay.{Client, RateLimiter, TokenProvider, ListingChecker}` are declared with sentinel errors but no implementations. Minimal `internal/domain/` types (IDs, Stage enum, lifecycle enums, placeholder structs) seed the interface signatures; field sets are placeholders the per-table IMPLs will flesh out.
- **Phase 7**: testing infrastructure. `testify/require` for assertions, mockery v3 generates `<package>/mocks/` for every service interface (regenerate via `just mocks-generate`; config in `.mockery.yaml`). Integration tests under `test/integration/` are guarded by `//go:build integration` and run via `just test-integration` against a Postgres + Valkey + Meilisearch Compose stack on deterministic local ports. CI integration job at `.github/workflows/integration.yml` is label-gated (`run-integration`) on PRs + nightly cron on main. Conventions documented in `docs/testing.md`.
- **Phase 8**: SQL migrations (`internal/datastore/migrations/`). Goose-backed `Migrator{Up,Down,Status}` with the migration set embedded into the binary via `embed.FS`. `spt migrate {up,down,status}` is the operator surface; `--migrations-dir` swaps in a filesystem path for dev iteration. Per Resolved Decision #12 there is no auto-migrate — each role's `Run` calls `datastore.CheckPendingMigrations` at startup and fails fast on pending migrations (warn-and-skip when `cfg.Postgres.DSN` is empty). `just db-{up,down,status}` wraps `spt migrate` against the Compose Postgres.

All IMPL-0001 phases complete. The binary builds, lints clean, all unit and integration tests pass; the package tree is ready for the per-component IMPLs (datastore, queue, ebay, agent, etc.) to drop in concrete implementations against the established interfaces. When asked to add features, work the next unchecked task in IMPL-0001.

- Module: `github.com/donaldgifford/spt`
- Go: pinned to the version in `go.mod` (`mise.toml` also pins the toolchain)
- Entry point: `./cmd/spt` — `main` is a thin wrapper around `cli.NewRootCmd` + `signal.NotifyContext`; build identity (`version`/`commit`/`date`) is injected via `-ldflags -X`.
- Package tree: `internal/{app/{api,scheduler,worker,cli},domain,pipeline,queue,datastore,search,cache,ebay,agent,health,obs,config,httpx}/` + `pkg/` (intentionally empty)
- Per-role `Run` signature: `func Run(ctx context.Context, cfg *config.Config) error` (pointer satisfies `gocritic`'s `hugeParam` since the struct grew in Phase 3).
- Config duration fields are strings (`"5s"`, `"15m"`) because gohcl doesn't decode `time.Duration` natively; use the `Parsed*` helpers in `internal/config/durations.go` to consume them.

## Task runner: just (not make)

The canonical task runner is `just`. There is **no `Makefile`** in this repo — CI installs `just` via `jdx/mise-action@v3` and invokes the recipes directly. Use `just` locally:

```
just build             # → build/bin/spt with version/commit/date ldflags
just test              # go test -v -race ./...
just test-pkg ./pkg/foo
just test-coverage     # writes coverage.out
just test-integration  # docker compose up, go test -tags=integration, down -v
just lint              # golangci-lint run ./...
just lint-fix
just fmt               # gofmt + goimports -local github.com/donaldgifford
just mocks-generate    # mockery v3 → <package>/mocks/
just check             # lint + test  (pre-commit gate)
just ci                # lint + test + build + license-check
just release-local     # goreleaser snapshot, no publish
just release v0.1.0    # tags and pushes
```

Docker targets live in `docker.just` (not imported by the main `justfile` — invoke with `just -f docker.just <recipe>` or import it if you need it regularly):

```
just -f docker.just docker-build   # local single-arch via buildx bake
just -f docker.just docker-ci      # linux/amd64 only (matches PR CI)
```

## Toolchain via mise

All dev tools — `golangci-lint`, `goreleaser`, `goimports`, `git-cliff`, `yq`, `prettier`, `actionlint`, `mockery`, `go-licenses`, `govulncheck`, `docz`, etc. — are version-pinned in `mise.toml`. Run `mise install` to materialize them. Versions tagged with `# renovate:` comments are auto-bumped by Renovate's custom-manager regex; preserve those comments when editing.

## Lint configuration is strict (Uber style)

`.golangci.yml` is a v2 config modeled on Uber's Go Style Guide. Notable enforced rules:

- **Imports** are grouped by `gci`: stdlib → third-party → `github.com/donaldgifford/*`. Run `just fmt` after touching imports.
- **Complexity ceilings**: `gocyclo` 15, `gocognit` 30, `funlen` 100 lines / 50 statements, `nestif` 4. Refactor rather than nolint.
- **Errors**: `errcheck` with `check-blank: true` and `check-type-assertions: true`; `errorlint` requires `%w` wrapping; error names must end in `Error`.
- **Style**: `revive` enforces early returns, indent-error-flow, context-as-first-arg, typed context keys, no dot imports, no naked returns past 5 lines.
- **nolint** directives must be specific (`//nolint:linter`) and carry an explanation (`nolintlint` requires it).
- Test files (`_test.go`), `main.go`, and `mock_*.go` have relaxed rules — see the exclusions block before adding new exclusions.

## Documentation: docz

The `docs/` tree (`adr/`, `rfc/`, `design/`, `impl/`, `plan/`, `investigation/`) is managed by the `docz` CLI (`.docz.yaml` defines the types, statuses, and ID prefixes). Create docs with `docz create <type> "<title>"` rather than hand-editing — it updates the per-type README index. The `docz` plugin is enabled in `.claude/settings.json` and provides the canonical workflow.

## Docker / release pipeline

`docker-bake.hcl` defines three groups:

- `default` / `spt`: local linux/amd64 build.
- `ci` / `spt-ci`: linux/amd64 only — multi-arch is **deliberately** excluded from PR CI because emulated arm64 builds on `ubuntu-latest` runners take ~25 min.
- `release` / `spt-release`: multi-arch (amd64 + arm64), tags supplied at runtime by `docker/metadata-action`'s bake-file output. `spt-release` intentionally declares no `tags` — HCL inheritance replaces rather than extends, so omitting them lets the metadata-action override take effect.

`goreleaser` (`.goreleaser.yml`) handles binary releases for linux/darwin × amd64/arm64 and signs checksums with GPG.

## Branch naming → PR labels

`.github/labeler.yml` auto-labels PRs by head-branch prefix: `feature/`, `fix/`, `chore/`, `docs/`, `security/`. Use these prefixes when creating branches so labels apply automatically.
