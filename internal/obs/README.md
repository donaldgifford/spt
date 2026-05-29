# `internal/obs`

Structured logging (`log/slog`), distributed tracing (OpenTelemetry
with the agent/system span-category split), and Prometheus metrics for
every `spt` role. Initialized once per role via `obs.Setup`.

## Public surface

- `obs.NewLogger(w, format, level)` — slog handler factory. Format
  `"auto"` picks JSON for non-TTY writers, text for TTYs.
- `obs.WithLogger(ctx, l)` / `obs.LoggerFromContext(ctx)` — request-
  scoped logger plumbing. `LoggerFromContext` attaches `trace_id` /
  `span_id` attributes when an OTel span is active on ctx.
- `obs.NewTracerProvider(ctx, opts)` — OTel `sdktrace.TracerProvider`
  with the OTLP HTTP exporter (system spans) plus an optional
  agent-only `LangfuseExporter` (filtered via
  `categoryFilterProcessor`). Returns a shutdown fn that flushes both.
- `obs.SetCategory(span, cat)` — tag a span with
  `spt.span_category = "agent"` to route it through the Langfuse path.
- `obs.NewRegistry()` — Prometheus registry pre-loaded with Go runtime
  and process collectors.
- `obs.WithInstance(reg, instance)` — `prometheus.Registerer` wrapper
  that pins the `instance` label on every metric registered through it
  (DESIGN-0005 multi-instance scaling).
- `obs.Setup(ctx, cfg, serviceName) (*Obs, shutdown, error)` — one-call
  init for a role's `Run`.

## Span category split

The Langfuse processor only forwards spans whose `spt.span_category`
attribute equals `"agent"`. Everything else is dropped on the Langfuse
side; the system OTLP exporter receives every span. This implements
the agent/system separation from DESIGN-0005 § OTel Span Categories
without forking the trace context — one tree, two destinations.

Set the category in a handler:

```go
ctx, span := tracer.Start(ctx, "extract-listing")
obs.SetCategory(span, obs.SpanCategoryAgent)
defer span.End()
```

## Phase 4 deferrals

- **Langfuse exporter is not wired yet.** `obs.Setup` constructs the
  OTLP exporter and the agent/system filter machinery, but leaves
  `TracerOptions.LangfuseExporter` nil. The Langfuse OTel client lands
  with the agent IMPL — when it ships, drop it into `setup.go` and
  agent-tagged spans route automatically. Tests already exercise the
  filter against an in-process recording exporter.
- **`/metrics` HTTP endpoint** is Phase 5's responsibility
  (`internal/health`). Phase 4 owns the registry; Phase 5 serves it.

## Manual smoke-test: OTLP

The TracerProvider sends to `cfg.Obs.OTLPEndpoint` (e.g.,
`localhost:4318` for the OTLP HTTP collector). With a local collector
running:

```sh
spt --config=test/config/example.hcl api &
# Trigger a request; check the collector's stdout or downstream Tempo.
```

There is no toy-span emitter yet — Phase 5+ instruments the role's
handlers; until then, the smoke test verifies pipeline construction
only.
