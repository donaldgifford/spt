---
id: IMPL-0002
title: "Phase 2 Typed extraction: deciders, registry, catalog, escalator, regression"
status: Draft
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0002: Phase 2 Typed extraction: deciders, registry, catalog, escalator, regression

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 0: Prerequisites](#phase-0-prerequisites)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 1: Contract, schema compiler, mock, contract suite](#phase-1-contract-schema-compiler-mock-contract-suite)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 2: Registry](#phase-2-registry)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 3: Decider kinds, cascade, cross-check, Langfuse](#phase-3-decider-kinds-cascade-cross-check-langfuse)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 4: Catalog and resolver](#phase-4-catalog-and-resolver)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 5: Escalator, verdict, calibration, regression](#phase-5-escalator-verdict-calibration-regression)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Build the middle of the pipeline as pure packages that a workflow can
call and a regression runner can measure: the `Decider` contract and its
provider kinds (Jev directly from TypeSafe; Anthropic and OpenAI
text models through one compiled schema; Ollama; mock), the registry
that declares the domain, the catalog resolver, the escalator, the
cross-check, the Langfuse recording, the calibration report and the
regression runner. At the end of this document the golden dataset runs
through every enabled decider and prints accuracy and calibration
tables, and the registry thresholds are set from them.

**Implements:** RFC-0001 (Delivery phase 2); DESIGN-0003, DESIGN-0004
and DESIGN-0005 in full; ADR-0001, ADR-0003 and ADR-0004.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- `pkg/decide`: contract, `State`, question kinds, `CompileSchema` and
  `Decode`, `decidertest`, kinds `jev`, `anthropic`, `openai`, `ollama`,
  `mock`, `Cascade`, `CrossCheck`, `Escalator`.
- `pkg/registry`: grammar, loader, validators, `schema_version`,
  question-set derivation, product-key rendering, `Diff`, the registry
  file for `ram`, `drive`, `server`, `cpu`, `gpu`, the CLI verbs, the
  reference-table generator, an hclkit lint schema.
- `pkg/resolve`: normalisation, retrieval (through the store from
  IMPL-0001), the chooser with its companion question, proposal dedupe,
  seeding tool, alias proposals.
- `pkg/langfuse`: ingestion client for traces, generations, scores and
  dataset runs.
- Calibration and agreement queries and the `spt decisions` verbs;
  `tools/regression-runner`; `just test-regression`.
- Live admission of every configured decider route, including the
  OpenCode translated routes, with a capped budget.

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Sequencing the calls (activities and `ExtractWorkflow`, IMPL-0003).
  This document delivers functions and their tests; IMPL-0003 wraps them.
- The API endpoints that expose decisions, calibration and the registry
  (IMPL-0003); the UI (IMPL-0005).
- Images to Jev, trained classifiers, price prediction.
- `nic`, `workstation` and `desktop` registry entries (follow as
  separate PRs once the first five are in prod, with their own
  regression tables).

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its
tasks are checked off and its success criteria are met. Every task
carries a `verify:` line naming the command or observation that proves
it done.

Live calls in this document send public eBay listing titles, item
specifics, condition and price to TypeSafe, OpenCode, Anthropic and
OpenAI. Seller names and descriptions are never in the decider state;
descriptions reach the escalator only. Every live task runs under the
budget in Open Question Q4 and uses the keys from the spt 1Password
Environment through `op run`.

---

<!--docz:phase:start-->
### Phase 0: Prerequisites

Facts to confirm before any decider code is written. The first task is
the one this document cannot proceed without: whether the OpenCode
translated routes enforce structured output the way the direct
providers do.

<!--docz:tasks:start-->
#### Tasks

- [ ] Confirm that OpenCode's translated Anthropic and OpenAI routes
  pass structured-output parameters through and enforce them. Write
  `tools/probe-structured`: it sends a three-question schema (one enum,
  one boolean, one integer enum) with `output_config.format` to
  `https://opencode.ai/inference/anthropic/v1/messages` and with
  `text.format` (`strict: true`) to
  `https://opencode.ai/inference/openai/v1/responses`, for
  `claude-haiku-5-5` and `gpt-5.4-mini`, and for the same models direct
  from Anthropic and OpenAI as controls; it reports whether every
  response parses against the schema, whether an out-of-enum value ever
  appears over 20 runs with adversarial state, and the latency and cost.
  Record the result table in this document's Dependencies section.
  verify: `op run --environment $op_env -- go run ./tools/probe-structured --all` prints `enforced` for all four routes, or the open question below is answered.
- [ ] Read the TypeSafe live docs the skill points at (models, HTTP API,
  primitives, structured instructions and criteria, jaggedness,
  patterns, the extraction-cascade cookbook) and note in
  `pkg/decide/jev/doc.go` the limits and behaviours the client relies
  on: 64k per request, 32k per question, 1–255 options, 1–10 score
  levels, `retry-after` on 429, versioned ids, the first-option lean.
  verify: `go doc ./pkg/decide/jev` prints the notes with the doc URLs.
- [ ] Jev live probe: `GET /v1/models` with the spt key; one
  `POST /v1/systemone` to `jev-1.14.0` with a real listing state object
  and five questions (a choice, two nouls, a score, a companion noul);
  save the exact request and response under `testdata/jev/probe/`.
  verify: `op run --environment $op_env -- go run ./tools/probe-jev` writes the pair and prints `model: jev-1.14.0`.
- [ ] Langfuse: create the `spt` project's dataset for regression runs
  (or confirm the golden dataset is the one to record runs against) and
  confirm the keys in the 1Password Environment reach it.
  verify: `op run --environment $op_env -- go run ./tools/probe-langfuse` creates and reads back a trace.
- [ ] Golden dataset present from IMPL-0001 phase 5 with hard cases.
  verify: `go test ./testdata/golden` passes and `wc -l testdata/golden/*.jsonl` matches the README.
- [ ] Pin `hclkit` CLI in `mise.toml` and write `pkg/registry/lint.hcl`,
  an hclkit lint schema for the registry grammar (block kinds, label
  counts, required attributes, uniqueness of component keys).
  verify: `hclkit lint --schema=pkg/registry/lint.hcl pkg/registry/registry.hcl` passes on the fixture registry and fails on a fixture with a duplicate component.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The OpenCode pass-through result is recorded with numbers; if any
  route is not enforced, the open question is answered before Phase 3.
- The Jev probe payload is committed as the first wire
  fixtures.
- Langfuse and the golden dataset are reachable from `just`
  recipes.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 1: Contract, schema compiler, mock, contract suite

Everything a workflow or a test needs from `pkg/decide` without a
network.

<!--docz:tasks:start-->
#### Tasks

- [ ] `pkg/decide` types from DESIGN-0003: `Kind`, `Question`, `Option`,
  `QuestionSet`, `State` (structured, with `Scoring` optional), `Answer`,
  `Answers`, `Usage`, `Decider` interface with `Name` and `Kind`, the
  error values.
  verify: `go build ./pkg/decide && go vet ./pkg/decide` clean.
- [ ] `State.Render()` deterministic text (sorted specifics, collapsed
  whitespace, control characters stripped) and `State.Tokens()` estimate;
  `State.JSON()` for Jev with the field names from the design.
  verify: `go test ./pkg/decide -run Render` with golden output; two
  renders of a shuffled specifics map are byte-identical.
- [ ] `CompileSchema(qs)`: one JSON schema for every text kind (`enum`
  for choice, `boolean` for bool, integer `enum` of level indices for
  score, a required `confidence` object, `additionalProperties: false`
  everywhere, every property required, no `minimum`, `maximum`,
  `pattern`, `$ref` or recursion).
  verify: `go test ./pkg/decide -run CompileSchema` checks each unsupported feature is absent and the golden schema for the fixture registry matches.
- [ ] `Decode(qs, raw)`: case-insensitive enum match canonicalised to the
  declared key, level index range, confidence range, `ErrNotAnOption` on
  anything else; never repairs.
  verify: `go test ./pkg/decide -run Decode` includes an `ECC_REG` versus `ecc_reg` case and a value outside the set.
- [ ] `mock`: a mockery v3 mock of `Decider` under `pkg/decide/mocks/`
  and a `ScriptedDecider` answering from a fixture map keyed by question
  set name and item id.
  verify: `just mocks && git diff --exit-code pkg/decide/mocks` clean; `go test ./pkg/decide/mock` passes.
- [ ] `decidertest.Run(t, d, fixtures)`: the contract suite (option
  guarantee, distribution sums within 0.01, confidence equals the
  probability of the value, purity on fixtures, `ErrStateTooLarge` on an
  oversized state). Shared by every kind and by the live admission verb.
  verify: `go test ./pkg/decide/decidertest` passes against `ScriptedDecider`.
- [ ] Fixed system prompt for text kinds as one exported constant with a
  test that it contains no model-specific wording.
  verify: `go test ./pkg/decide -run SystemPrompt` passes.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `pkg/decide` builds with no dependency on the store, Temporal or any
  provider SDK.
- The compiled schema for the fixture registry is a committed golden
  file.
- `decidertest` is the single admission test every later kind runs.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Registry

The grammar, the loader, the version, what is derived from it, and the
first five component types written with the TypeSafe guidance.

<!--docz:tasks:start-->
#### Tasks

- [ ] `pkg/registry` structs with hclkit tags for `registry`,
  `classify`, `component`, `choice`, `bool`, `score` (`levels`),
  `derived`, `resolve`, `verdict`; `Load()` from the embedded file and
  `LoadFile(path)`.
  verify: `go test ./pkg/registry -run Load` on a two-component fixture.
- [ ] Validators from DESIGN-0004, one diagnostic each with position.
  verify: `go test ./pkg/registry -run Validate` has one invalid fixture per rule under `testdata/registry/invalid/`.
- [ ] `schema_version`: SHA-256 over the hclkit-formatted canonical bytes,
  first 12 hex; exposed on `Registry` and by `spt registry version`.
  verify: `spt registry version` equals `hclkit fmt pkg/registry/registry.hcl | sha256sum | cut -c1-12`.
- [ ] Derivations: `Registry.Classify()`, `Component.Fields()`,
  `Component.Choose(candidates)` (choice over candidates plus the
  companion `matches_catalog` bool), `Registry.Verdict()`,
  `Component.RenderKey(answers, derived, row)` with `unknown` handling.
  verify: `go test ./pkg/registry -run 'Fields|Choose|RenderKey'` with golden keys per component.
- [ ] `Diff(a, b)` listing added, removed and changed components,
  questions, options, thresholds and templates; `spt registry diff
  <git-ref>` reads the other version with `git show`.
  verify: `spt registry diff HEAD~1` on a fixture change prints the expected list.
- [ ] `tools/registry-docs` rendering one markdown table per component
  into the chart README section; drift-checked.
  verify: `just registry-docs && git diff --exit-code charts/spt/README.md` clean.
- [ ] Write `pkg/registry/registry.hcl` for `ram`, `drive`, `server`,
  `cpu`, `gpu` and the `classify` and `verdict` blocks, using the
  TypeSafe skill while writing: one judgment per question, options named
  for content, criteria as concrete situations, the confusing case named
  in the criteria, a companion bool wherever a choice has a "none"
  meaning (`is_accessory` and `is_bundle` on classify, `<field>_listed`
  on every bounded numeric, per Q2 and Q7), the five verdict judgments
  as bools (Q1), `quantity` as a choice.
  verify: `spt registry validate` passes; `hclkit lint --schema=pkg/registry/lint.hcl pkg/registry/registry.hcl` passes.
- [ ] Human-test review: for each component, pick ten golden listings
  and check by hand that a person shown only the rendered state and
  each question could answer at a glance; record changes made.
  verify: `git log --oneline -- pkg/registry/registry.hcl` shows the review commit with the notes in its message.
- [ ] CLI: `spt registry validate [file]`, `spt registry diff <ref>`,
  `spt registry version`; cobra docs regenerated.
  verify: `just docs-cli && git diff --exit-code docs/cli` clean.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The embedded registry loads, validates and lints; its version is
  reproducible from the file.
- The compiled Claude schema for the real registry is a committed golden
  file and changes show in review.
- Five components are authored with their human-test review recorded.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: Decider kinds, cascade, cross-check, Langfuse

Every provider route behind the contract, admitted by the same suite.

<!--docz:tasks:start-->
#### Tasks

- [ ] `jev` kind: HTTP client for `POST {base_url}/v1/systemone`
  building `choice`, `noul` and `score` questions from a `QuestionSet`
  (state as a JSON object; `instructions` from the question text;
  option criteria; score `criteria` as the ordered levels), parsing
  `probabilities`, `confidence`, `noul` and `score`, recording the
  response `model`, token usage and cost; bearer auth; two retries with
  backoff honouring `retry-after`; 400 and 422 surfaced with `detail`;
  input-limit check before the call.
  verify: `go test ./pkg/decide/jev` against the probe fixtures and an `httptest` server for 429 and 422.
- [ ] Record Jev fixtures for every question set in the registry
  (classify with its companions, fields per component, a chooser, the
  verdict judgments) under `testdata/jev/`, through an env-gated
  recorder.
  verify: `SPT_RECORD=1 op run --environment $op_env -- go test ./pkg/decide/jev -run Record` writes them; a second run without `SPT_RECORD` passes from the files.
- [ ] `anthropic` kind: Messages API with `output_config.format` from
  `CompileSchema`, the fixed system prompt, `stop_reason: refusal` →
  `ErrRefusal`, usage and cost from the response; `base_url` and `model`
  from config so the same code serves Anthropic direct and OpenCode.
  verify: `go test ./pkg/decide/anthropic` against recorded fixtures for both routes.
- [ ] `openai` kind: Responses API with `text.format` strict,
  `refusal` → `ErrRefusal`, `status: incomplete` → `ErrStateTooLarge`;
  `base_url` and `model` from config for OpenAI direct and OpenCode.
  verify: `go test ./pkg/decide/openai` against recorded fixtures for both routes.
- [ ] `ollama` kind: `/api/chat` with `format` set to the compiled
  schema; disabled by default; no live test in this IMPL.
  verify: `go test ./pkg/decide/ollama` against one recorded fixture.
- [ ] Cost table in config (`decider.price_in_usd_per_mtok`,
  `price_out_usd_per_mtok`) and `Usage.CostUSD` computed per call.
  verify: `go test ./pkg/decide -run Cost` on known usage numbers.
- [ ] `Cascade`: per-question re-ask of sub-threshold questions, merged
  answers with `Decider` recorded, summed usage, comma-joined
  `ModelVersion`.
  verify: `go test ./pkg/decide/cascade` table-driven with `ScriptedDecider`s.
- [ ] `CrossCheck`: deterministic sampling by item-id hash, second
  decider answers the same set, both answer sets returned with roles,
  agreement per question computed; `on_disagreement` policy returned to
  the caller as a decision, not acted on here.
  verify: `go test ./pkg/decide/crosscheck` asserts sampling is stable and agreement is computed per question.
- [ ] `pkg/langfuse`: ingestion client (batch endpoint) for traces,
  generations, scores and dataset runs with the fields from DESIGN-0003;
  a `Recorder` that wraps any `Decider` and records each call under a
  session; failures are logged and never fail the call.
  verify: `go test ./pkg/langfuse` against an `httptest` server; `op run --environment $op_env -- go test -tags live ./pkg/langfuse` writes one trace.
- [ ] Live admission verb: `spt decide --decider <name> --suite` runs
  `decidertest` against the configured route with 20 golden listings,
  prints pass or fail per guarantee, latency and cost.
  verify: `op run --environment $op_env -- spt decide --decider jev --suite` passes; the same for `haiku`, `sonnet-opencode`, `gpt-mini-opencode`.
- [ ] Option-order check: the regression runner's `--reorder` flag runs
  each choice question with reversed option order on a sample and
  reports the flip rate per question (Open Question Q3).
  verify: `go run ./tools/regression-runner --decider jev --reorder --sample 50` prints a flip-rate table.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every configured route passes `decidertest` live, with the results
  table pasted into this document's Dependencies section.
- Every kind has recorded fixtures for every question set, and
  `just test` runs offline.
- One compiled schema serves every text route; `git grep -n "model ==" pkg/decide` finds nothing.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: Catalog and resolver

Open strings made finite with data.

<!--docz:tasks:start-->
#### Tasks

- [ ] `pkg/resolve.Normalise`: lowercase, strip the vendor prefix the
  component implies, collapse separators and spaces, trim; nothing else.
  verify: `go test ./pkg/resolve -run Normalise` table-driven on spellings from the golden dataset.
- [ ] `pkg/resolve.Retrieve` over the store's `RetrieveCandidates` with
  `similarity_threshold`, `candidate_limit`, vendor narrowing at the cap,
  and the exact and alias match short-circuit.
  verify: `go test -tags integration ./pkg/resolve -run Retrieve` on the 200-row fixture catalog reports recall at 50 on the golden server, CPU and GPU listings.
- [ ] Chooser: `Component.Choose(candidates)` renders criteria from
  `display_name` and the per-type attribute template; the three-outcome
  table (`τ_match` 0.70, 0.30) implemented as a pure function
  `resolve.Outcome(answers)`.
  verify: `go test ./pkg/resolve -run Outcome` covers match, no-match and in-between.
- [ ] Proposals: `resolve.Dedupe(proposal)` key; proposal struct with
  `evidence`; alias-proposal builder from a label whose string did not
  match the chosen row.
  verify: `go test ./pkg/resolve -run 'Dedupe|AliasProposal'` passes.
- [ ] `spt catalog seed --from-v1 <dsn> [--component t]` reading the
  first implementation's distinct product keys and item specifics,
  grouping by normalised model, writing proposals (never rows), idempotent.
  verify: `go test -tags integration ./cmd/spt -run CatalogSeed` on a fixture export produces the expected proposal set twice with no duplicates.
- [ ] `spt catalog list | approve <id> | --all --component <t> | reject
  <id>` against the store directly (the API path arrives in IMPL-0003).
  verify: `go test -tags integration ./cmd/spt -run Catalog` passes.
- [ ] Nearest rows for a proposal (top three by trigram) as a store
  query used by the CLI output and later the UI.
  verify: `go test -tags integration ./internal/store -run NearestRows` passes.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Retrieval recall at 50 on the golden server, CPU and GPU listings is
  reported and above 0.95 on the fixture catalog.
- Seeding from a fixture export is idempotent and writes proposals only.
- No string handling exists outside `Normalise`; `git grep -n "strings.Replace" pkg/resolve` shows only that file.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: Escalator, verdict, calibration, regression

The text-model path, the alert gate, and the measurement that sets the
thresholds.

<!--docz:tasks:start-->
#### Tasks

- [ ] `Escalator` with `EscalateInput` and `EscalateResult` from
  DESIGN-0003: any text kind, schema from `CompileSchema` extended with
  proposal and bundle shapes, the `<<<UNTRUSTED>>>` wrapping,
  description truncation, validation of `Fields` against the registry,
  `Unsure` handling, budget check before the call.
  verify: `go test ./pkg/decide/escalate` against recorded fixtures; the injection fixtures produce only declared options.
- [ ] Verdict judgment set (five bools) from the registry with
  `State.Scoring` rendered deterministically; the alert gate as a pure
  function `decide.AlertGate(score, threshold, judgments, baselineN)`
  returning deal, edge or noise from the truth table in DESIGN-0003
  (Q1).
  verify: `go test ./pkg/decide -run 'Verdict|AlertGate'` covers every table row, the band case and the thin-baseline case.
- [ ] Calibration: the store query from IMPL-0001 wired to
  `spt decisions calibration --since --question --decider` printing per
  question per decider the bin table and the threshold proposal;
  `spt decisions agreement` printing agreement per question per decider
  pair; `spt decisions stats`.
  verify: `go test -tags integration ./cmd/spt -run Decisions` reproduces a known table from a synthetic set.
- [ ] `tools/regression-runner`: runs `testdata/golden/*.jsonl` through
  each enabled decider (classify, fields, chooser against the fixture
  catalog, verdict with a synthetic baseline), prints accuracy per
  component type and the calibration table per decider, writes a
  Langfuse dataset run per decider per commit, supports `--reorder` and
  `--sample`; `just test-regression` wraps it under `op run`.
  verify: `just test-regression` prints tables for `jev` and `haiku` and the Langfuse UI shows two dataset runs for the commit.
- [ ] Set the registry thresholds from Jev's calibration table (type,
  field, quantity, verdict, match) and record the table in the PR.
  verify: `spt registry diff main` shows only threshold changes; the PR body contains the table.
- [ ] Budget guard: `decide.Budget` interface with the store's
  transactional increment; both budgets checked before a call; a test
  that an exhausted budget returns `ErrBudget` before any HTTP request.
  verify: `go test ./pkg/decide -run Budget` asserts no request is made.
- [ ] PR with the regression tables, labelled `dont-release`.
  verify: `gh pr view --json labels` shows `dont-release`; `gh pr checks` green.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just test-regression` reports per-type accuracy for `jev-1.14.0` and
  `claude-haiku-5-5` on the golden dataset, and both runs exist in
  Langfuse.
- Registry thresholds are set from measured calibration, not defaults,
  and the table is in the PR.
- The escalator never emits a value outside the registry on the
  injection fixtures.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `pkg/decide/` | Create | contract, `State`, `CompileSchema`, `Decode`, system prompt, errors, cost |
| `pkg/decide/{jev,anthropic,openai,ollama,mock,cascade,crosscheck,escalate,decidertest}/` | Create | kinds, composition, escalator, contract suite |
| `pkg/decide/mocks/` | Create | mockery v3 mocks |
| `pkg/registry/` | Create | grammar structs, loader, validators, version, derivations, diff, `registry.hcl`, `lint.hcl` |
| `pkg/resolve/` | Create | normalise, retrieve, chooser outcome, dedupe, alias proposals |
| `pkg/langfuse/` | Create | ingestion client and recorder |
| `internal/store/queries/{calibration,agreement,nearest}.sql` | Create | reporting queries |
| `cmd/spt/{registry,catalog,decide,decisions}.go` | Create | CLI verbs |
| `tools/{probe-structured,probe-jev,probe-langfuse,regression-runner,registry-docs}/` | Create | probes and tools |
| `testdata/{jev,anthropic,openai,ollama}/` | Create | recorded wire fixtures per route |
| `testdata/registry/` | Create | fixture registries, invalid cases, golden schema and keys |
| `testdata/catalog/` | Create | 200-row fixture catalog, v1 export fixture |
| `justfile`, `mise.toml` | Modify | `test-regression`, `registry-docs`, `mocks`; `hclkit` pin |
| `charts/spt/README.md` | Modify | generated registry reference section |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] `decidertest` run by every kind offline (fixtures) and live
  (admission verb).
- [ ] Unit tests for `CompileSchema`, `Decode`, `State.Render`,
  `Cascade`, `CrossCheck`, `Escalator`, `AlertGate`, every registry
  validator, every resolver function.
- [ ] Golden tests: compiled schema, product keys, registry reference
  table, `schema_version`.
- [ ] Integration tests (tag `integration`): retrieval recall, seeding,
  nearest rows, calibration and agreement queries.
- [ ] Live tests (tag `live`, under `op run`): Langfuse write, admission
  of each route; never in the default `just check`.
- [ ] `just test-regression` as the gate for any PR touching
  `pkg/registry` or `pkg/decide`.

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- IMPL-0001 complete: store with catalog and reporting queries,
  `testdata/golden/`, `op run` in `just`, mockery v3.
- TypeSafe key (`JEV_API_KEY`), OpenCode Console key
  (`OPENCODE_API_KEY`), Anthropic key (`ANTHROPIC_API_KEY`) and, if
  OpenAI direct is admitted, `OPENAI_API_KEY`, all in the spt 1Password
  Environment.
- Langfuse project and keys.
- The TypeSafe agent skill, installed in this repository's Claude Code
  setup, used while writing the registry and reviewing questions.
- Phase 0 result table for the OpenCode pass-through probe (to be
  filled in):

  | Route | Model | Schema enforced | Out-of-enum over 20 runs | p50 latency | Cost per call |
  | --- | --- | --- | --- | --- | --- |
  | Anthropic direct | `claude-haiku-5-5` | | | | |
  | OpenCode anthropic | `claude-haiku-5-5` | | | | |
  | OpenAI direct | `gpt-5.4-mini` | | | | |
  | OpenCode openai | `gpt-5.4-mini` | | | | |

- Phase 3 admission table (to be filled in) with the same columns per
  configured decider plus the `decidertest` result.

<!--docz:dependencies:end-->

## Open Questions

All questions were decided on 2026-11-14: option a in every case, with
one direction on Q6. The bodies above reflect the decisions, and
DESIGN-0003 and DESIGN-0004 were updated for Q1, Q2 and Q7.

**Q1. Verdict: one `choice`, or atomic judgments combined in code?**
Decided: option a. Replace the single `deal | edge | noise` choice with
companion `bool` questions in the registry's `verdict` block
(`is_lot_or_multiple`, `is_part_or_accessory`, `title_contradicts_key`,
`scam_pattern`, `baseline_applies`) and compute deal, edge or noise in
`decide.AlertGate` from those probabilities, the score and the baseline
sample count. Each row of the table is testable and the why panel can
show every input. Alternative: keep the single choice, which puts the
decision in the model.

**Q2. Classify: extras as options, or as companion judgments?**
Decided: option a. `choice` over component keys only, plus companion
`bool` questions `is_accessory` and `is_bundle`; `unknown` is the case
where the top component is below threshold. Alternative: keep
`accessory`, `bundle` and `unknown` as options of the one choice.

**Q3. Option-order bias.** Decided: option a. The regression runner's
`--reorder` check reports a flip rate per choice question; a question
above 5% gets its options or criteria revised before thresholds are set.
Option order in the registry stays fixed and documented. Alternative:
ignore until a calibration problem appears.

**Q4. Live budget for this document.** Decided: option a. Caps per
provider for probes, admission and regression runs: $5 TypeSafe, $5
OpenCode, $5 Anthropic, $5 OpenAI if admitted; the regression runner
refuses to start when a run's estimate exceeds the remaining cap,
tracked in `testdata/budget.json`. Alternative: no cap.

**Q5. Fixture recording and refresh.** Decided: option a. Env-gated
recorder (`SPT_RECORD=1`) writing request and response pairs under
`testdata/<kind>/`, refreshed deliberately when a model version or the
registry changes; the files are reviewed in the PR. Alternative: a
cassette library replaying HTTP by URL match.

**Q6. OpenAI direct in this document.** Decided: option a, with
direction. Admit OpenAI models through OpenCode only; add
`OPENAI_API_KEY` and a direct route later if OpenCode's translation
shows a gap in the phase 0 probe. Direction: Jev is the opposite case.
TypeSafe's own endpoint is the only Jev route; Jev through OpenCode is
not configured, probed or recorded. OpenCode serves the text models.
Alternative: admit OpenAI direct now as the control route.

**Q7. `other` on bounded numerics.** Decided: option a. Replace `other`
with a companion `bool` named `<field>_listed` per bounded numeric (the
guidance prefers a companion question to a "none" option); below 0.3 the
value is `null` plus the metric. Alternative: keep `other` as an option.

**Q8. If the phase 0 probe shows an OpenCode route does not enforce the
schema.** Decided: option a. Disable that route's deciders in
configuration, keep the direct routes, and re-probe at each OpenCode
release; the kind code does not change. Alternative: client-side
retry-on-invalid for that route only, rejected by the
no-per-model-branches rule.

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [ADR-0001](../adr/0001-every-finite-question-is-a-typed-decision-with-a-calibrated.md); [ADR-0003](../adr/0003-the-domain-schema-is-data-an-hcl-registry-embedded-in-the-binary.md); [ADR-0004](../adr/0004-canonical-identities-come-from-a-catalog-grown-by-proposal-and.md)
- [DESIGN-0003 Decisions](../design/0003-decisions-the-decider-contract-cascade-escalation-and.md); [DESIGN-0004 Registry](../design/0004-registry-the-domain-schema-as-an-embedded-hcl-file.md); [DESIGN-0005 Catalog](../design/0005-catalog-canonical-identities-retrieval-and-proposals.md)
- [IMPL-0001 Foundation](../impl/0001-phase-1-foundation-roles-configuration-store-ebay-client.md); [IMPL-0003 Durable execution](../impl/0003-phase-3-durable-execution-temporal-plumbing-workflows.md); [IMPL-0004 Integration](../impl/0004-phase-4-integration-cluster-dev-namespace-live-providers-side.md)
- [TypeSafe docs index](https://docs.typesafe.ai/llms.txt); [models](https://docs.typesafe.ai/go/models.md); [HTTP API](https://docs.typesafe.ai/go/api.md); [OpenAPI spec](https://api.typesafe.ai/openapi.json); [primitives](https://docs.typesafe.ai/go/primitives.md); [structured instructions and criteria](https://docs.typesafe.ai/go/advanced.md); [jaggedness](https://docs.typesafe.ai/go/jaggedness.md); [patterns](https://docs.typesafe.ai/go/patterns.md); [extraction cascade cookbook](https://docs.typesafe.ai/cookbooks/sde_cascade.md); [agent skill](https://docs.typesafe.ai/introduction/coding-agents)
- [OpenCode Console](https://opencode.ai/docs/zen/)
- [Claude API, Structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs); [OpenAI, Structured outputs](https://developers.openai.com/api/docs/guides/structured-outputs); [Ollama structured outputs](https://docs.ollama.com/capabilities/structured-outputs)
- [Langfuse ingestion API](https://langfuse.com/docs/api); [Langfuse datasets](https://langfuse.com/docs/datasets/overview)
- [hclkit lint schema](https://github.com/donaldgifford/hclkit); [PostgreSQL `pg_trgm`](https://www.postgresql.org/docs/current/pgtrgm.html)
- References from the first implementation (re-read, not imported): [`pkg/extract`](https://github.com/donaldgifford/server-price-tracker/tree/main/pkg/extract), [`internal/regression`](https://github.com/donaldgifford/server-price-tracker/tree/main/internal/regression)

<!--docz:references:end-->
