# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project state

`spt` (Server Price Tracker) is an early-stage Go CLI scaffold. `cmd/spt/main.go` is currently an empty `package main` — there is no application logic yet. Most of the repo is the surrounding toolchain: build, lint, release, docs, and CI scaffolding. When asked to add features, you will likely be creating the first real code in a given area.

- Module: `github.com/donaldgifford/spt`
- Go: pinned to the version in `go.mod` (`mise.toml` also pins the toolchain)
- Entry point: `./cmd/spt`

## Task runner: just (not make)

The canonical task runner is `just`. There is **no `Makefile`** in this repo — CI installs `just` via `jdx/mise-action@v3` and invokes the recipes directly. Use `just` locally:

```
just build           # → build/bin/spt with version/commit ldflags
just test            # go test -v -race ./...
just test-pkg ./pkg/foo
just test-coverage   # writes coverage.out
just lint            # golangci-lint run ./...
just lint-fix
just fmt             # gofmt + goimports -local github.com/donaldgifford
just check           # lint + test  (pre-commit gate)
just ci              # lint + test + build + license-check
just release-local   # goreleaser snapshot, no publish
just release v0.1.0  # tags and pushes
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
