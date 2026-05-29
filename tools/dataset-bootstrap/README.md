# spt-dataset-bootstrap

Pulls a stratified sample of recent Listings, Components, and Scores from the canonical Postgres datastore into a regression JSON file. The output is suitable for human audit or for uploading to Langfuse via `spt-dataset-upload`.

Designed in [DESIGN-0006 — dataset-bootstrap](../../docs/design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#dataset-bootstrap); built per [IMPL-0002 Phase 3](../../docs/impl/0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-3-dataset-bootstrap).

## Invocation

```bash
just tool dataset-bootstrap -- sample \
    --since=30d \
    --per-kind=10 \
    --per-confidence-bucket='<0.5:5,0.5-0.8:10,0.8-1.0:10' \
    --total-cap=200 \
    --seed=42
```

Output filename defaults to `regression-<UTC-timestamp>.json` (second resolution) — override with `--out`.

## Stratification

Listings are grouped by `(ComponentKind, ConfidenceBucket, ExtractorVer)` using each listing's primary component. The three confidence buckets follow DESIGN-0002's contract:

| Bucket | Confidence range | Meaning |
|--------|------------------|---------|
| `<0.5` | `[0.0, 0.5)` | needs-review band |
| `0.5-0.8` | `[0.5, 0.8)` | mid-confidence |
| `0.8-1.0` | `[0.8, 1.0]` | high-confidence |

A listing with no components lands in the special `<no-kind>` kind so the operator can spot-check unparseable titles.

## Determinism

Two runs with the same `--seed` and the same population must produce byte-identical output. Internally this is achieved by:

1. Seeding `math/rand/v2.NewPCG` from `cfg.Seed`.
2. Sorting strata keys (kind, bucket, extractor) before selecting.
3. Sorting the final picked listings by `ListingID` so the JSON written to disk hashes identically.

## Status

The Postgres `Datastore` implementation lands with the datastore IMPL; until then `sample` exits non-zero with a clear message and the unit tests exercise `Sampler` against a fake `Reader`. The `Reader` interface in `sampler.go` is a deliberately narrow subset of `datastore.Datastore` so test fakes don't have to satisfy the full CRUD contract.

## Output schema

```json
{
  "version": "v1",
  "generatedAt": "2026-05-28T00:00:00Z",
  "config": {
    "SinceDuration": 2592000000000000,
    "PerKind": 10,
    "PerConfidenceBucket": {"<0.5": 5, "0.5-0.8": 10, "0.8-1.0": 10},
    "TotalCap": 200,
    "Seed": 42,
    "OutputPath": "regression-20260528T000000Z.json"
  },
  "sample": {
    "listings": [...],
    "scores": {...},
    "components": {...}
  }
}
```

Future shape changes bump the `version` field; older datasets remain parseable by tooling that branches on the version.
