# spt-dashgen

Generates Grafana dashboards and Prometheus rules into `charts/spt/files/` so the Helm chart ships them as ConfigMaps and PrometheusRule resources. `-validate` mode regenerates to memory and diffs against the on-disk files — CI fails on drift.

Designed in [DESIGN-0006 — dashgen](../../docs/design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#dashgen); built per [IMPL-0002 Phase 7](../../docs/impl/0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-7-dashgen).

## Invocation

```bash
# Regenerate files into the chart.
just dashboards-gen

# CI gate: fail if files are stale.
just validate-dashboards
```

## Adding a new dashboard

1. Write the builder function in `dashboards.go` (`func buildXxx() any`).
2. Register it in `DashboardSpecs()` with a name + file path under `dashboards/`.
3. Run `just dashboards-gen` and commit the generated JSON.

The builder uses the in-house `internal/grafana/` package — a ~200-LOC typed-Go shape that emits the Grafana JSON we actually use. No third-party SDK dependency (Resolved Decision #9 in IMPL-0002 Phase 7). Revisit if dashboard count grows past ~10 distinct dashboards.

## Adding a new alert rule

1. Append a `Rule{Alert: ..., Expr: ..., For: ..., Labels: ..., Annotations: ...}` to the appropriate `RuleGroupSpec` in `rules.go`.
2. Or create a new `RuleFileSpec` if the rules belong to a separate group/file.
3. Run `just dashboards-gen` and commit.

## Why hardcoded Prometheus datasource

Resolved Decision #10 in IMPL-0002: `$datasource` templating becomes a follow-up if a multi-cluster user requests it. Today every panel pins `Datasource{Type: "prometheus", UID: "prometheus"}`.

## How `-validate` works

`Generate(..., ModeValidate)` walks `DashboardSpecs()` + `RuleFiles()`, marshals each spec to bytes, and `bytes.Equal`-compares against the on-disk file. Any drift produces a per-file list printed to stdout and a non-zero exit. Atomic writes (`.tmp` + `os.Rename`) keep the on-disk tree consistent under concurrent runs.
