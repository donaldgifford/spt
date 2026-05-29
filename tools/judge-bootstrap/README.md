# spt-judge-bootstrap

Two-mode CLI to bootstrap LLM-as-judge few-shots from existing Score data — operators audit existing classifications rather than labeling from scratch.

Designed in [DESIGN-0006 — judge-bootstrap](../../docs/design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#judge-bootstrap); built per [IMPL-0002 Phase 5](../../docs/impl/0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-5-judge-bootstrap).

## Operator workflow

```bash
# 1. Surface candidates with one of the four strategies.
just tool judge-bootstrap -- list \
    --strategy=ambiguous \
    --since=30d \
    --candidates=50 \
    --out=candidates.json

# 2. Hand-edit candidates.json: for each item you accept, set
#    "accepted": true and write a short "notes" explaining why it's
#    a good few-shot.

# 3. Apply: validate Notes-on-accepted and write the few-shots file.
just tool judge-bootstrap -- apply \
    --input=candidates.json \
    --output=internal/agent/judge/examples.json
```

`apply` exits non-zero with a per-candidate listing of which `ScoreID`s are missing `notes` — operators can't half-justify a few-shot.

## Strategies

| Name | Description |
|------|-------------|
| `ambiguous` | Scores within ±5% of a percentile boundary (25/50/75) of the recent population. |
| `low-confidence` | Listings with at least one Component below `Confidence < 0.5`. |
| `high-stakes` | Scores in the top decile of `Value` — biggest absolute deltas. |
| `disagreement` | Scores with prior `Judgment.Verdict ∈ {Disagrees, Uncertain}`. Requires a `JudgmentReader`. |

## Output schema

The few-shots file is a JSON array of `Candidate` records — the judge prompt loader reads it directly.

## Status

The Postgres `Datastore` lives in a future IMPL; until then `list` exits non-zero with a clear message and the unit tests exercise each strategy against a fake `Reader`. The `Reader` and `JudgmentReader` interfaces are deliberately narrow so test fakes don't have to satisfy the entire CRUD contract.
