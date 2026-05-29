# spt-regression-runner

Runs a regression dataset against one or more model backends (Ollama, Anthropic, OpenAI) and reports per-Kind accuracy + p50/p95 latency.

Designed in [DESIGN-0006 — regression-runner](../../docs/design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#regression-runner); built per [IMPL-0002 Phase 6](../../docs/impl/0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-6-regression-runner).

## ⚠ DO NOT WIRE INTO CI

Verbatim from prior art (`donaldgifford/server-price-tracker/tools/regression-runner`):

> regression-runner needs `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` to invoke the production model backends. Wiring this into PR CI exposes those keys to fork-PR contributors via workflow logs and `env:` echoing, since `pull_request_target` workflows or even careless `pull_request` workflows can leak secrets. Release-gating against accuracy regression happens via the maintainer's local invocation, not PR CI.

The audit `grep -r regression-runner .github/workflows/` should always return zero results.

## Invocation

```bash
# Default: ollama against the in-tree baseline.
just tool regression-runner -- run

# Multi-backend, JSON output for diffing across runs.
ANTHROPIC_API_KEY=sk-... OPENAI_API_KEY=sk-... \
  just tool regression-runner -- run \
    --backend=anthropic,openai \
    --format=json --out=report.json

# Against the full set in Langfuse (TODO: wiring lands with agent IMPL).
just tool regression-runner -- run \
    --dataset=langfuse://regression-eval-v3 \
    --backend=anthropic
```

## Datasets

- **In-tree baseline** at `tools/regression-runner/testdata/baseline/` — ~50 listings, committed, PR-reviewable. Default `--dataset` value.
- **Full Langfuse set** via `--dataset=langfuse://<dataset-id>` — fetched at run time; not committed. The Langfuse wiring lands with the agent IMPL and currently returns `ErrLangfuseDatasetNotWired`.

## Match outcomes

| Outcome | Means |
|---------|-------|
| `ExactMatch` | `(Kind, Model, Manufacturer, Quantity, Spec)` all agree. |
| `PartialMatch` | `(Kind, Model, Manufacturer)` agree; quantity/spec differ. |
| `NoMatch` | Even the partial key disagrees, OR the backend returned an error. |

The placeholder `domain.Component` only has `Kind` today; ExactMatch will tighten when the extract IMPL adds the other fields.

## Status

The backend `Extract` methods are placeholders that return deterministic stubs when the relevant env var is set — the matcher and report aggregation are exercised end-to-end by the unit tests, but the production Ollama/Anthropic/OpenAI HTTP calls land with the agent IMPL.
