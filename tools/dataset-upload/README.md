# spt-dataset-upload

Uploads a regression JSON (produced by `spt-dataset-bootstrap`) to Langfuse as a DatasetItem set. Deterministic SHA256-truncated IDs make re-uploads idempotent — re-uploading the same content is a no-op, not a duplicate.

Designed in [DESIGN-0006 — dataset-upload](../../docs/design/0006-developer-tooling-porting-and-refactoring-from-old-spt.md#dataset-upload); built per [IMPL-0002 Phase 4](../../docs/impl/0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-4-dataset-upload).

## Invocation

```bash
export LANGFUSE_HOST="https://cloud.langfuse.com"
export LANGFUSE_PUBLIC_KEY="pk-..."
export LANGFUSE_SECRET_KEY="sk-..."

just tool dataset-upload -- upload \
    --dataset-id=regression-eval \
    --input=regression-20260528T000000Z.json

# Preview without hitting Langfuse:
just tool dataset-upload -- upload --dry-run \
    --dataset-id=regression-eval \
    --input=regression-20260528T000000Z.json
```

The tool exits non-zero with `dataset-upload: missing Langfuse credentials` if any of the three env vars is empty when `--dry-run` is off.

## ID scheme

```
ID = hex(SHA256(content)[:8])     →  16 hex chars
```

Collision math: birthday bound at 2^64 IDs gives 50% collision probability around 2^32 (~4 billion) items. At expected scale (≤10^6 items per dataset) collision risk is ~10^-8 — negligible for an eval dataset.

## Why an internal client rather than a third-party SDK

Resolved Decision #4 in IMPL-0002: our surface area is one endpoint (`POST /api/public/dataset-items`). A community SDK would add dependency drift for negligible benefit. Re-check for an official Langfuse Go SDK at each release — the `Client` interface here lets us swap implementations without touching `Uploader`.

## Testing

```bash
just test-pkg ./tools/dataset-upload/...
```

Unit tests cover ID determinism, dry-run silence, upsert idempotency, and 4xx error surfacing against an in-process `httptest.Server`.
