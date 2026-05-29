# spt — task runner
#
# Canonical task runner for spt. There is no Makefile; every contributor
# (and CI) goes through `just`.

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

project_name      := "spt"
project_owner     := "donaldgifford"
go_package        := "github.com/" + project_owner + "/" + project_name
build_dir         := "build"
bin_dir           := build_dir + "/bin"
coverage_out      := "coverage.out"
allowed_licenses  := "Apache-2.0,MIT,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0"
goimports_local   := "github.com/" + project_owner

# Version info derived from git; falls back to dev when not in a repo or tag-less.
commit_hash := `git rev-parse --short HEAD 2>/dev/null || echo unknown`
version     := `git describe --tags --always --dirty 2>/dev/null || echo dev`
build_date  := `date -u +%Y-%m-%dT%H:%M:%SZ`

# Default: list recipes
_default:
    @just --list --unsorted

# ─── Build ──────────────────────────────────────────────────────────

# Build everything (core)
[group('build')]
build: build-core

# Build the core CLI binary into build/bin/spt
[group('build')]
build-core:
    @mkdir -p {{ bin_dir }}
    @go build -ldflags "-X main.version={{ version }} -X main.commit={{ commit_hash }} -X main.date={{ build_date }}" \
        -o {{ bin_dir }}/{{ project_name }} ./cmd/{{ project_name }}
    @echo "✓ Core binaries built"

# Remove build artifacts and the Go build cache
[group('build')]
clean:
    @rm -rf {{ bin_dir }}/
    @rm -f {{ coverage_out }}
    @go clean -cache
    @find . -name "*.test" -delete
    @echo "✓ Cleaned build artifacts"

# ─── Run ────────────────────────────────────────────────────────────

# Build then run the CLI
[group('run')]
run: build
    @{{ bin_dir }}/{{ project_name }}

# Build then run the CLI from the local bin
[group('run')]
run-local: build
    @{{ bin_dir }}/{{ project_name }}

# ─── Tools ──────────────────────────────────────────────────────────

# Generic recipe to invoke any tool under tools/<name>/ via `go run`.
# Usage: just tool mock-server -- serve --scenario=default
[group('tools')]
tool name *args:
    @go run ./tools/{{ name }} {{ args }}

# Regenerate the CLI markdown reference tree under docs/cli/.
# CI runs `just docs-cli && git diff --exit-code docs/cli/` to fail on drift.
[group('tools')]
docs-cli:
    @mkdir -p docs/cli
    @go run ./cmd/{{ project_name }} gen-docs docs/cli/

# ─── Test ───────────────────────────────────────────────────────────

# Run all tests with the race detector
[group('test')]
test:
    @go test -v -race ./...

# Run all tests (core + plugins)
[group('test')]
test-all: test

# Run tests for a single package: just test-pkg ./pkg/foo
[group('test')]
test-pkg pkg:
    @go test -v -race {{ pkg }}

# Run tests with a coverage profile written to coverage.out
[group('test')]
test-coverage:
    @go test -v -race -coverprofile={{ coverage_out }} ./...

# Run tests and open the HTML coverage report
[group('test')]
test-report:
    @go test -coverprofile={{ coverage_out }} ./...
    @go tool cover -html={{ coverage_out }}

# Run the build-tagged integration suite against the Compose stack
[group('test')]
test-integration:
    @docker compose -f test/integration/docker-compose.yml up -d --wait
    @go test -tags=integration -race ./test/integration/... ./internal/datastore/...
    @docker compose -f test/integration/docker-compose.yml down -v

# ─── Migrations (against the Compose Postgres from test/integration) ─

# DSN for the Compose Postgres; override with $SPT_DSN for other targets.
db_dsn := env_var_or_default("SPT_DSN", "postgres://spt:spt@127.0.0.1:55432/spt?sslmode=disable")

# Apply all pending migrations against the Compose Postgres
[group('db')]
db-up: build
    @{{ bin_dir }}/{{ project_name }} --postgres-dsn={{ db_dsn }} migrate up

# Roll back the most recent migration
[group('db')]
db-down: build
    @{{ bin_dir }}/{{ project_name }} --postgres-dsn={{ db_dsn }} migrate down

# Print applied/pending migration table
[group('db')]
db-status: build
    @{{ bin_dir }}/{{ project_name }} --postgres-dsn={{ db_dsn }} migrate status

# ─── Lint & format ─────────────────────────────────────────────────

# Run golangci-lint
[group('lint')]
lint:
    @golangci-lint run ./...

# Run golangci-lint with --fix
[group('lint')]
lint-fix:
    @golangci-lint run --fix ./...

# Verify the golangci-lint configuration
[group('lint')]
lint-config:
    @golangci-lint config verify

# Lint GitHub Actions workflows
[group('lint')]
lint-actions:
    @actionlint

# Format code with gofmt + goimports
[group('lint')]
fmt:
    @gofmt -s -w .
    @goimports -w -local {{ goimports_local }} .

# ─── Mocks ─────────────────────────────────────────────────────────

# Generate mocks for every interface listed in .mockery.yaml
[group('mocks')]
mocks-generate:
    @mockery

# ─── License compliance ─────────────────────────────────────────────

# Check dependency licenses against the allow list
[group('license')]
license-check:
    @go-licenses check ./... --allowed_licenses={{ allowed_licenses }}

# Generate CSV report of all dependency licenses
[group('license')]
license-report:
    @go-licenses report ./... --template=.github/licenses-csv.tpl

# ─── Release ────────────────────────────────────────────────────────

# Validate the goreleaser config
[group('release')]
release-check:
    @goreleaser check

# Snapshot release locally (no publish, no sign)
[group('release')]
release-local:
    @goreleaser release --snapshot --clean --skip=publish --skip=sign

# Tag and push a new release: just release v0.1.0
[group('release')]
release tag:
    @git tag -a {{ tag }} -m "Release {{ tag }}"
    @git push origin {{ tag }}

# ─── Composite gates ────────────────────────────────────────────────

# Pre-commit gate: lint + test
[group('gate')]
check: lint test
    @echo "✓ Pre-commit checks passed"

# Full CI gate: lint + test + build + license-check
[group('gate')]
ci: lint test build license-check
    @echo "✓ CI pipeline complete"
