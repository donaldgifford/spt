# Testing conventions

The repo enforces a small set of testing conventions so every package
reads the same way. Pickup these patterns when adding a new test file
or starting a new package.

## Unit tests

- **Assertion library: `github.com/stretchr/testify/require`.** No
  `testify/assert` — one style per repo. `require.NoError`,
  `require.Equal`, `require.Contains` are the most common helpers.
- **Table-driven by default.** When you have more than one case for
  one function, write a slice of structs and iterate. Subtests run in
  parallel via `t.Parallel()` where safe.
- **No live external calls.** Network, eBay, LLM, Postgres, Valkey,
  and Meilisearch all go through their interface and use a mock
  (see "Mocks" below). Real services land in the integration suite.

```go
func TestThing(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr error
    }{
        {name: "ok", input: "x", want: "X"},
        {name: "empty", input: "", wantErr: ErrEmpty},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            got, err := Thing(tt.input)
            if tt.wantErr != nil {
                require.ErrorIs(t, err, tt.wantErr)
                return
            }
            require.NoError(t, err)
            require.Equal(t, tt.want, got)
        })
    }
}
```

## Mocks: mockery v3

Every service interface (`Queue`, `Datastore`, `Search`, `Cache`,
`Scheduler`, `Agent`, `ebay.Client`, `ebay.RateLimiter`,
`ebay.TokenProvider`) gets a mockery-generated mock in
`<package>/mocks/<interface_name>.go`.

Regenerate after changing an interface:

```sh
just mocks-generate
```

Config lives in `.mockery.yaml` at the repo root. Mocks are
checked-in (not generated at test time) so unit tests stay hermetic
and reviewers can read the generated code in PRs.

`.golangci.yml` excludes `**/mocks/*.go` from style + complexity
rules — generated code shouldn't fight the linter.

Use in a test:

```go
import (
    "github.com/donaldgifford/spt/internal/queue"
    queuemocks "github.com/donaldgifford/spt/internal/queue/mocks"
)

func TestThingThatNeedsQueue(t *testing.T) {
    q := queuemocks.NewQueue(t)
    q.EXPECT().Enqueue(mock.Anything, mock.Anything).Return(nil)
    // ... rest of test
}
```

## Integration tests

Tests guarded by `//go:build integration` live under
`test/integration/` and run against the Compose stack in
`test/integration/docker-compose.yml` (Postgres, Valkey,
Meilisearch on deterministic local ports). They never run from
`just test` — only `just test-integration`:

```sh
just test-integration
```

This brings the stack up with `--wait` (blocks on healthchecks),
runs `go test -tags=integration -race ./test/integration/...`, then
tears the stack down with `down -v` (volumes purged).

### When to add an integration test

- A new datastore migration or query.
- A new queue / cache interaction with semantic edges (TTL, BLMOVE,
  atomicity).
- A new Meilisearch index/query shape.

If the test can be expressed with a mock, write a unit test instead.
Integration tests pay a Compose-stack startup cost (~10–20 s) and
run on a separate CI job, so reserve them for cases where the real
backend's semantics matter.

## CI

- **Unit tests** run on every push and PR via `.github/workflows/ci.yml`
  (`test-go` job).
- **Integration tests** run via `.github/workflows/integration.yml`:
  - Pull requests carrying the `run-integration` label.
    `.github/labeler.yml` auto-applies the label when
    `test/integration/**` changes; operators can also apply it
    manually for cross-cutting changes.
  - A nightly cron at 03:00 UTC against main, so drift is caught
    even on PRs that don't carry the label.

> If the integration suite stays under ~5 minutes once real backends
> land, revisit moving it to the PR fast-path (no label gating). Until
> then, the label keeps PR signal cheap.
