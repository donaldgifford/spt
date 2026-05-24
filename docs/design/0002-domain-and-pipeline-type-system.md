---
id: DESIGN-0002
title: "Domain and pipeline type system"
status: Draft
author: Donald Gifford
created: 2026-05-24
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0002: Domain and pipeline type system

**Status:** Draft **Author:** Donald Gifford **Date:** 2026-05-24

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [Naming and ID strategy](#naming-and-id-strategy)
  - [Domain types](#domain-types)
    - [Watch](#watch)
    - [Listing](#listing)
    - [PriceObservation](#priceobservation)
    - [Alert](#alert)
    - [Component](#component)
    - [Score](#score)
    - [Judgment](#judgment)
    - [MarketSignal](#marketsignal)
    - [Notification](#notification)
  - [Pipeline types](#pipeline-types)
    - [Job](#job)
    - [Task](#task)
    - [Stage](#stage)
  - [Infrastructure interfaces](#infrastructure-interfaces)
    - [Scheduler](#scheduler)
    - [Queue](#queue)
    - [Datastore](#datastore)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [Resolved](#resolved)
  - [Still open](#still-open)
  - [Owned elsewhere](#owned-elsewhere)
- [References](#references)
<!--toc:end-->

## Overview

Defines the core domain types (Watch, Listing, Component, Score, Judgment,
MarketSignal, Notification) and the pipeline types (Job, Task) that flow through
the orchestrator, plus the infrastructure interfaces (Scheduler, Queue,
Datastore) the rest of the codebase consumes. This is the type contract every
other package builds against.

## Goals and Non-Goals

### Goals

- A coherent type vocabulary that doesn't change as the codebase grows.
- Clear separation between **domain types** (what the user cares about),
  **pipeline types** (how work flows through the system), and **infrastructure
  interfaces** (how we talk to storage / queue / scheduling).
- A Component model expressive enough for server hardware (CPU, RAM, drives,
  NICs, chassis, PSU, motherboard, GPU) without becoming a generic kitchen-sink
  schema.
- Pipeline types that match the DAG model in
  [ADR-0012](../adr/0012-build-a-custom-scheduler-and-pipeline-orchestrator.md)
  — Jobs as DAG executions, Tasks as individual stage runs.

### Non-Goals

- The orchestrator's executor implementation (deferred to a follow-up DESIGN
  doc).
- API request/response shapes — those are derived from these types via the
  OpenAPI layer
  ([ADR-0010](../adr/0010-generate-the-frontend-api-client-from-openapi.md)) but
  live in `internal/app/api/`.
- Authoritative DDL — schemas live with their migrations under
  `internal/datastore/migrations/`. This doc shows representative SQL for
  context.
- The scoring algorithm itself (deferred until we resolve the
  analytics-computation open question in RFC-0001).

## Background

[DESIGN-0001](0001-go-application-layout-and-conventions.md) establishes that
domain types live in `internal/domain/` and are pure data — no DB tags, no JSON
tags that leak into the public API, no methods that touch infrastructure.
Everything else depends on these types.

The shape of the type system is the API contract between the orchestrator, the
agentic layer, and the persistence layer. Getting it right here makes everything
downstream cleaner; getting it wrong forces refactors across every package.

## Detailed Design

### Naming and ID strategy

IDs are **typed**, not bare strings. Each entity has its own ID type so the
compiler catches `GetListing(watchID)` mistakes:

```go
type WatchID        string
type ListingID      string
type ComponentID    string
type ScoreID        string
type JudgmentID     string
type AlertID        string
type NotificationID string
type JobID          string
type TaskID         string
```

`UserID` is intentionally absent. spt is single-tenant in v1; the API auth
boundary (when added) will not depend on a per-user identity model. See
[Open Questions](#open-questions) for the auth-vs-multi-tenancy stance.

ID generation: **UUIDv7** for everything. UUIDv7 is time-sortable (which helps
Postgres index locality and trace correlation) and globally unique without
coordination. We never expose auto-increment integer IDs externally — they leak
volume.

The string representation is the canonical wire format. Postgres stores them as
`uuid` (typed column), not `text`, so the DB can validate.

### Domain types

#### Watch

A user-defined eBay query plus everything needed to schedule and act on it.

```go
type Watch struct {
    ID            WatchID

    // Query
    Query         string            // free-text eBay search
    CategoryID    string            // optional eBay category filter
    Filters       map[string]string // pass-through Browse API filters

    // Scheduling
    Cadence       time.Duration     // poll interval
    NextRunAt     time.Time         // computed; orchestrator reads this
    LastRunAt     time.Time
    Enabled       bool

    // Notification baseline
    NotifyConfig  NotifyConfig      // see Notification section

    // Sampling
    JudgeSampleRate float64         // 0.0-1.0; fraction of scores that trigger judge

    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

`Cadence` is `time.Duration` (not seconds-as-int) so the compiler enforces
units. `NextRunAt` is the column the scheduler reads on its tick — it's
denormalized state, recomputed after every successful run.

#### Listing

A single eBay listing as we observed it. Raw API payload preserved for
reprocessing.

```go
type Listing struct {
    ID            ListingID         // our ID
    WatchID       WatchID           // which watch surfaced it
    EbayItemID    string            // eBay's ID, deduped against this

    Title         string
    Description   string            // optional; not always fetched
    URL           string
    ImageURLs     []string

    Price         Money             // current observed price; see PriceObservation for history
    Condition     Condition         // enum
    ListingType   ListingType       // enum: BuyItNow, Auction, BestOffer
    Seller        Seller

    State         ListingState      // Live | Sold | EndedNoSale | NotFound
    SoldPrice     *Money            // populated only when State == Sold
    SoldAt        *time.Time        // populated only when State == Sold
    EndedAt       *time.Time        // populated when State == Sold | EndedNoSale | NotFound

    ListedAt      time.Time
    EndsListedAt  *time.Time        // eBay's auction end time, when known; nil for non-auctions
    FirstSeenAt   time.Time         // when we first observed it
    LastSeenAt    time.Time         // updated on every observation (poll or reconciliation)

    RawPayload    json.RawMessage   // original Browse API payload, untouched

    Components    []Component       // resolved by the extract stage; empty until then
}

type Money struct {
    Amount    apd.Decimal           // never float64; precision matters
    Currency  string                // ISO 4217
}

type ListingState int
const (
    ListingStateLive ListingState = iota   // currently listed on eBay
    ListingStateSold                       // ended via sale; SoldPrice + SoldAt populated
    ListingStateEndedNoSale                // auction expired without bid, or seller cancelled
    ListingStateNotFound                   // 404 on eBay; removed from platform
)

type Condition int
const (
    ConditionUnknown Condition = iota
    ConditionNew
    ConditionOpenBox
    ConditionRefurbished
    ConditionUsed
    ConditionForParts
)

type ListingType int
const (
    ListingTypeFixedPrice ListingType = iota
    ListingTypeAuction
    ListingTypeBestOffer
)

type Seller struct {
    Username           string
    FeedbackScore      int
    FeedbackPercentage float64
    TopRated           bool
}
```

**`Money` uses `cockroachdb/apd.Decimal`**, not `float64` and not
`shopspring/decimal`. `apd` is actively maintained, has explicit
precision/rounding context (important for percentile and moving-average math),
and is the Go decimal library Cockroach itself uses for its `DECIMAL` type — so
the semantics are battle-tested. eBay returns prices as strings; we parse via
`apd.NewFromString` and keep arithmetic exact. We learned the float-precision
lesson the hard way in the prior version where float rounding made percentile
bands shift between runs.

**`State` is a four-valued enum**, not a nullable `SoldAt`. The prior version
conflated "we don't know" with "still live"; this version makes each terminal
state explicit and pairs it with the right populated fields:

| State         | SoldPrice | SoldAt | EndedAt | Meaning                                                       |
| ------------- | :-------: | :----: | :-----: | ------------------------------------------------------------- |
| `Live`        |    nil    |  nil   |   nil   | Currently listed on eBay                                      |
| `Sold`        |    set    |  set   |   set   | Ended via sale — gives us a sell-price datapoint              |
| `EndedNoSale` |    nil    |  nil   |   set   | Auction expired without a bid, or seller cancelled            |
| `NotFound`    |    nil    |  nil   |   set   | eBay returned 404 on a follow-up fetch — pulled from platform |

Only `Sold` yields a sell-price observation for analytics. The other terminal
states are excluded from sell-price aggregates but retained for "what fraction
of listings actually sell?" telemetry.

**Price history lives in a separate `PriceObservation` table.** `Listing.Price`
is always the most-recent observation; the history is queryable for "did the
price drop after we alerted?" analytics. See the `PriceObservation` section
below.

**`RawPayload` is preserved** so we can re-run extraction with an improved
extractor without re-fetching from eBay.

#### PriceObservation

A single price datapoint for a listing. One row per observation (initial fetch,
watch re-poll if the listing re-appears, alert-driven re-fetch, bulk 12h
reconciliation).

```go
type PriceObservation struct {
    ListingID    ListingID
    Price        Money
    ObservedAt   time.Time
    Source       PriceObservationSource
}

type PriceObservationSource int
const (
    PriceObservationSourceInitialFetch PriceObservationSource = iota
    PriceObservationSourceWatchRepoll                          // listing reappeared in a watch poll with different price
    PriceObservationSourceAlertReconcile                       // alert-driven per-cycle reconciliation
    PriceObservationSourceBulkReconcile                        // 12h bulk reconciliation
)
```

PriceObservations are the source of truth for "did the price change?" —
`Listing.Price` is a denormalized cache of the latest one. Sell-price aggregates
use the `Listing.SoldPrice` field on `Sold` listings; the PriceObservation log
tracks the _pre-sale_ price trajectory.

#### Alert

An open notification trigger on a (Watch, Listing) pair. One Alert per (Watch,
Listing) at most — re-triggers update the existing Alert rather than opening a
new one.

```go
type Alert struct {
    ID                 AlertID
    WatchID            WatchID
    ListingID          ListingID

    TriggeredByScoreID ScoreID            // the score that opened the alert
    LatestScoreID      ScoreID            // updated as recalcs re-score the listing
    LatestScoreValue   apd.Decimal        // denormalized for fast threshold checks

    State              AlertState         // Open | Closed
    OpenedAt           time.Time          // creation time; never updated
    ClosedAt           *time.Time
    CloseReason        AlertCloseReason   // populated when State == Closed

    // Lifecycle counters
    WatchCyclesObserved int               // count of watcher re-evaluation cycles this alert has survived
    NotificationsSent   int               // total notifications fired for this alert
    LastNotifiedAt      *time.Time        // last time a Notification fired; nil if never (e.g. notifications disabled)

    // Reconciliation freshness (see DESIGN-0004)
    Stale              bool               // true if the most recent reconciliation attempt failed
    LastReconciledAt   *time.Time         // wall time of the last reconciliation attempt (success or failure)
    LastReconcileError string             // populated when Stale = true; cleared on next success
}

type AlertState int
const (
    AlertStateOpen AlertState = iota
    AlertStateClosed
)

type AlertCloseReason int
const (
    AlertCloseReasonUnknown AlertCloseReason = iota
    AlertCloseReasonBelowThreshold   // recalc moved the alert under the threshold
    AlertCloseReasonListingSold      // reconciliation found the listing sold
    AlertCloseReasonListingRemoved   // reconciliation found the listing ended-no-sale or not-found
    AlertCloseReasonManual           // operator dismissed
)
```

**Lifecycle** (full flow lives in [DESIGN-0004 — Alert and reconciliation pipeline](0004-alert-and-reconciliation-pipeline.md)):

1. **Open**: a `score` stage produces a Score that crosses a Watch's
   `NotifyConfig.Thresholds`. An Alert is opened with
   `TriggeredByScoreID = LatestScoreID = score.ID`.
2. **Re-score**: next watcher cycle, after reconciliation has run for alerting
   listings and the baseline has been recomputed, every still-open Alert is
   re-scored against the new baseline. `LatestScoreID` and `LatestScoreValue`
   update; if the score no longer crosses the threshold, the Alert closes
   (`BelowThreshold`).
3. **Reconciliation close**: if the per-alert reconciliation finds the listing
   sold or removed, the Alert closes (`ListingSold` or `ListingRemoved`).
4. **Manual close**: operator API call.

Re-evaluation in step 2 is arithmetic only — no new scoring pass, no LLM call.
It compares the existing `Score.Value` to the _new_ `MarketSignal` percentile
bands.

**Lifecycle counters:**

| Field                  | What it tracks                                  | Why it matters |
|------------------------|-------------------------------------------------|----------------|
| `OpenedAt`             | Creation time of the alert                      | Audit; time-since-open is itself a useful signal ("this alert has been open for 14 days — is the listing stale?"). |
| `WatchCyclesObserved`  | Watcher cycles the alert survived re-evaluation | A high count means the deal really is good across multiple baseline recomputations; consumers may escalate notification cadence or de-emphasize fresh-but-shallow alerts. |
| `NotificationsSent`    | Total notifications fired for this alert        | Cap-based throttling ("don't send more than N notifications for the same alert") and audit. |
| `LastNotifiedAt`       | When the last notification fired                | Used by the re-notification policy in DESIGN-0004 — e.g., "don't re-notify within 24h even if the score improves." |

The `(WatchID, ListingID)` UNIQUE constraint stays — at most one **open** Alert per pair (enforced via partial unique index `WHERE state = 0` on the `alerts` table). Re-triggering after a close is allowed (opens a new row), but a Listing is never simultaneously the subject of two open Alerts on the same Watch. Historical closed Alerts on the same pair remain queryable for audit.

#### Component

The parsed hardware components extracted from a listing. **This is where most of
the design effort is.**

Server listings contain heterogeneous hardware. A common shape:

> "Dell PowerEdge R730xd 12LFF | 2x E5-2680v4 | 256GB DDR4 | 2x 1TB SSD | H730
> RAID | iDRAC8 Enterprise"

We need a model that captures these structured enough for analytics (e.g.,
"what's the average $/GB of DDR4 ECC RAM in the last 30 days?") without forcing
every listing into a single rigid schema.

**Shape: structured common fields + kind-typed Spec.**

```go
type Component struct {
    ID           ComponentID
    ListingID    ListingID
    Kind         ComponentKind     // enum
    Manufacturer string            // "Intel", "Samsung", "Dell"
    Model        string            // "E5-2680v4", "MZ7LH960HAJR-00005"
    Quantity     int               // 2x E5-2680v4 → Quantity: 2

    Spec         ComponentSpec     // typed per Kind; see below

    Confidence   apd.Decimal       // 0.0-1.0; LLM extractor's self-reported confidence
    ExtractedAt  time.Time
    ExtractorVer string            // version of the extractor (model + prompt) that produced this
}

type ComponentKind int
const (
    ComponentKindUnknown ComponentKind = iota
    ComponentKindCPU
    ComponentKindRAM
    ComponentKindDrive
    ComponentKindNIC
    ComponentKindChassis
    ComponentKindMotherboard
    ComponentKindPSU
    ComponentKindGPU
    ComponentKindRAIDController
)

// ComponentSpec is a sealed interface — only the types defined in this
// package satisfy it. Pattern-match via type switch.
type ComponentSpec interface {
    isComponentSpec()
}

type CPUSpec struct {
    Cores         int
    Threads       int
    BaseClockGHz  decimal.Decimal
    TurboClockGHz decimal.Decimal
    TDPWatts      int
    Socket        string            // "FCLGA2011-3"
    Generation    string            // "Broadwell-EP"
    Year          int               // release year, for age-based scoring
}
func (CPUSpec) isComponentSpec() {}

type RAMSpec struct {
    SizeGB     int                   // single stick size, not aggregate
    Type       RAMType               // DDR3, DDR4, DDR5
    SpeedMHz   int
    ECC        bool
    Registered bool                  // RDIMM vs UDIMM
    LoadReduced bool                 // LRDIMM
    RankConfig string                // "2Rx4"
}
func (RAMSpec) isComponentSpec() {}

type DriveSpec struct {
    SizeGB     int
    Medium     DriveMedium           // HDD, SSD, NVMe
    Interface  DriveInterface        // SAS, SATA, NVMe-U.2, NVMe-M.2, etc.
    RPMs       int                   // 0 for SSD/NVMe
    FormFactor string                // "2.5in", "3.5in", "M.2 22110"
    Endurance  string                // "1 DWPD", optional
}
func (DriveSpec) isComponentSpec() {}

type NICSpec struct {
    PortCount int
    SpeedGbps int                    // per port
    Interface NICInterface           // RJ45, SFP, SFP+, SFP28, QSFP+, QSFP28
}
func (NICSpec) isComponentSpec() {}

type ChassisSpec struct {
    FormFactor   string              // "2U", "4U", "Tower"
    DriveBays    int                 // total bays
    BayFormFactor string             // "2.5in" or "3.5in" or "mixed"
    HotSwap      bool
}
func (ChassisSpec) isComponentSpec() {}

// MotherboardSpec, PSUSpec, GPUSpec, RAIDControllerSpec follow the same shape.
```

**Why a sealed interface, not a `OneOf` JSON blob:**

- The Go type system catches "this CPU has a SizeGB field" mistakes at compile
  time.
- Pattern-matching on `Spec` via type switch is idiomatic Go and reads naturally
  in handler code.
- We still serialize to JSONB in Postgres (single `spec` column,
  kind-discriminated), but in-process we have full type safety.

**Why an aggregated `Quantity` field rather than N separate Component rows:**

- A listing "2x Xeon E5-2680v4" is _one_ SKU on the listing, not two physical
  objects we can disambiguate. Modeling it as one row with `Quantity: 2`
  preserves that.
- Aggregate price-per-unit calculations stay clean:
  `listing.Price / Σ(component.Quantity)` for that kind.

**`ExtractorVer` is critical.** Extraction is going to improve over time.
Tagging every Component with the extractor version that produced it lets us
re-run extraction on the preserved `RawPayload` and compare populations.

**`Confidence` is the LLM's self-reported certainty that the extracted component
faithfully reflects the listing.** Range 0.0–1.0. The signal we want is
two-fold:

1. **A high mean clustered near 1.0** with a small spread (target spread < 0.2
   across the population for a given Kind). Wide spread is a prompt-quality
   signal — re-tune.
2. **A hard floor for inclusion in analytics.** Components with
   `Confidence < 0.5` are persisted (so we can audit and judge them) but are
   **excluded from baseline calculations** and tagged for manual review / judge
   evaluation.

Extraction also runs **pre-filters and post-filters** around the LLM call to
keep load down and catch common error modes:

- **Pre-filters** reject obvious non-server listings before the LLM ever sees
  them. Example: a listing titled "Dell R740 power cable" would extract as a
  "Dell R740 server" without a pre-filter, polluting the price baseline.
  Pre-filter rules live in `internal/extract/prefilter.go`.
- **Post-filters** check the LLM output for known failure modes (e.g., a chassis
  listing that somehow extracted a CPU; component counts that exceed the
  chassis's bay count) and either flag for judge review or hard-reject.

The pre/post filter logic itself isn't a type concern — it lives in the
extraction pipeline DESIGN doc (forthcoming). What matters here: `Confidence` is
the contract the extractor publishes, and downstream consumers (scoring,
baseline recalc) respect the 0.5 floor.

#### Score

The computed score for a listing.

```go
type Score struct {
    ID            ScoreID
    ListingID     ListingID
    WatchID       WatchID

    Value         apd.Decimal       // raw score; semantics owned by scorer
    Percentile    apd.Decimal       // 0.0-100.0 within the watch's recent listings
    Components    map[string]apd.Decimal // sub-scores (e.g., "price_vs_market", "spec_quality")
    Reasoning     string            // optional: human/agent-readable rationale

    ScorerVersion string            // version of the scoring model
    ScoredAt      time.Time
}
```

`Components` (lowercase here, not the hardware Component type — these are
scoring sub-components) lets the UI break down _why_ a listing scored what it
did. `ScorerVersion` parallels `ExtractorVer` so we can recompute on the same
listing data.

#### Judgment

The LLM-as-judge result on a (Score, Listing) pair. Sampled — not every Score
gets a Judgment.

```go
type Judgment struct {
    ID              JudgmentID
    ScoreID         ScoreID
    ListingID       ListingID

    Verdict         JudgmentVerdict   // enum
    Confidence      apd.Decimal       // 0.0-1.0
    Critique        string            // free-text from the judge model

    JudgeVersion    string            // model + prompt version
    JudgedAt        time.Time
}

type JudgmentVerdict int
const (
    VerdictUnknown JudgmentVerdict = iota
    VerdictAgrees                    // judge agrees with the score
    VerdictDisagrees                 // judge disagrees
    VerdictUncertain
)
```

Judgments feed Langfuse evals (ADR-0008). They are evidence about scoring
quality, not a replacement for scores.

#### MarketSignal

Derived market-level analytics. Computed per (Watch, Window).

```go
type MarketSignal struct {
    WatchID         WatchID
    Window          time.Duration     // 7*24h, 30*24h
    AsOf            time.Time         // when this snapshot was computed

    SampleCount     int
    PriceMin        Money
    PriceMax        Money
    PriceMedian     Money
    PriceMean       Money
    PriceStdDev     apd.Decimal

    Percentiles     map[int]Money     // p10, p25, p50, p75, p90
    MovingAvg       Money             // window's moving average

    MedianTimeToSell time.Duration    // for listings observed both live and sold within window
    AvgListingLengthDays apd.Decimal
}
```

A MarketSignal is the output of the "analytics computation" question RFC-0001
flagged. **Storage decision: Postgres.** Specifically a `market_signals` table
keyed by `(watch_id, window, as_of)`, populated by the baseline recalc step at
the end of each watcher cycle (see the alert/reconciliation flow below). The
previous version stored these in Prometheus, which was a category mistake —
Prometheus is for system-health time series, not for application-derived
analytics with a long retention requirement. ClickHouse is purpose-built for
trace and event analytics, not for the relatively small (per-watch × per-window)
signal table. Postgres is exactly the right shape: low volume, transactional,
joinable with `listings`, and we already have it.

If signal volume ever grows past Postgres's comfort zone (we'd need to be
tracking thousands of watches with second-resolution windows before this
matters), a materialized view in ClickHouse fed by CDC is the natural next step.
Today: a plain Postgres table.

#### Notification

```go
type NotifyConfig struct {
    Enabled      bool
    Channels     []NotifyChannel       // email, webhook, etc.
    Thresholds   NotifyThresholds
}

type NotifyThresholds struct {
    MinScore        *apd.Decimal       // notify if Score.Value >= MinScore
    MaxPercentile   *apd.Decimal       // notify if Score.Percentile <= MaxPercentile (good deal)
    JudgeAgrees     bool               // require judge verdict = Agrees
}

type Notification struct {
    ID            NotificationID
    AlertID       AlertID               // every Notification is tied to an Alert
    WatchID       WatchID
    ListingID     ListingID
    ScoreID       ScoreID               // the score in effect when the notification fired

    Channel       NotifyChannel
    SentAt        time.Time
    DeliveryStatus DeliveryStatus
    DeliveryError  string
}
```

`Thresholds` use `*apd.Decimal` so "not set" and "set to 0" are distinguishable.

`Notification` is the delivery record for one outbound message tied to an Alert.
An Alert can produce multiple Notifications over its lifecycle (initial open,
status changes if we add re-notification, manual replay). The Alert is the
durable state; Notifications are the audit log of sends.

### Pipeline types

#### Job

A Job is one DAG execution for one watch trigger. The orchestrator creates a
Job, walks the DAG, and the Job is complete when every reachable Task is either
Done or Failed.

```go
type Job struct {
    ID          JobID
    WatchID     WatchID
    Trigger     JobTrigger          // Scheduled, Manual, Backfill

    State       JobState            // Pending, Running, Succeeded, Failed, Cancelled
    StartedAt   time.Time
    FinishedAt  *time.Time

    // Roll-up counters; the source of truth is the Task rows
    TaskCounts  map[TaskState]int

    LastError   string              // populated on Failed
    TraceID     string              // OTel trace ID for the whole pipeline run
}

type JobState int
const (
    JobStatePending JobState = iota
    JobStateRunning
    JobStateSucceeded
    JobStateFailed
    JobStateCancelled
)

type JobTrigger int
const (
    JobTriggerScheduled JobTrigger = iota
    JobTriggerManual
    JobTriggerBackfill
)
```

#### Task

A Task is one stage execution within a Job. Tasks are the unit the worker pool
consumes.

```go
type Task struct {
    ID           TaskID
    JobID        JobID
    Stage        Stage              // see below
    Sequence     int                // ordering hint within a Job

    Input        json.RawMessage    // stage-specific; typed by Stage
    Output       json.RawMessage    // populated on success

    State        TaskState
    Attempts     int
    MaxAttempts  int
    NextAttemptAt *time.Time        // populated on retry

    EnqueuedAt   time.Time
    StartedAt    *time.Time
    FinishedAt   *time.Time

    LastError    string
    SpanID       string             // OTel span for this task
}

type TaskState int
const (
    TaskStatePending TaskState = iota
    TaskStateRunning
    TaskStateSucceeded
    TaskStateFailed
    TaskStateSkipped               // conditional edge evaluated false (e.g., not sampled for judge)
    TaskStateCancelled
)
```

**`Input` and `Output` are `json.RawMessage`**, not `any`. This means:

- The Task table is a single shape regardless of stage.
- Stage handlers know their own `Input` type and assert on it.
- The orchestrator doesn't need to know about stage-specific types to schedule
  them.

There's a typed helper layer per stage so handlers don't write JSON
deserialization boilerplate:

```go
type ExtractInput struct { ListingID ListingID; RawPayload json.RawMessage }
type ExtractOutput struct { Components []Component }

func RunExtract(ctx context.Context, t Task) (ExtractOutput, error) {
    var in ExtractInput
    if err := json.Unmarshal(t.Input, &in); err != nil {
        return ExtractOutput{}, fmt.Errorf("decode extract input: %w", err)
    }
    // ...
}
```

#### Stage

Stage is an enum identifying the function to run. The DAG topology in
`internal/pipeline/` is the source of truth for which stages exist and how they
connect.

```go
type Stage string

const (
    StagePoll    Stage = "poll"    // fetch new listings from eBay for a watch
    StageExtract Stage = "extract" // parse a listing into Components
    StageScore   Stage = "score"   // compute a Score for a listing
    StageJudge   Stage = "judge"   // sampled; LLM-as-judge on a Score
    StageIndex   Stage = "index"   // push to Meilisearch
    StageNotify  Stage = "notify"  // conditional; deliver Notification
    // Reconciliation and alert-eval stages defined in DESIGN-0004:
    // StageReconcileAlerts, StageReconcileBulk, StageEvalAlerts
)
```

The full Stage enum (including the reconciliation stages added by [DESIGN-0004](0004-alert-and-reconciliation-pipeline.md)) lives in `internal/domain/stage.go`; per-DESIGN docs document the subset they introduce.

The core DAG:

```
poll ──▶ extract ──┬──▶ score ──┬──▶ judge   [sampled]
                   │            │
                   └──▶ index   └──▶ notify  [threshold]
```

### Infrastructure interfaces

Per [DESIGN-0001](0001-go-application-layout-and-conventions.md), every
infrastructure service has an interface. Here are the shapes for the three
central ones.

#### Scheduler

```go
package pipeline

type Scheduler interface {
    // Run blocks until ctx is cancelled. It ticks on Cadence,
    // queries the Datastore for Watches whose NextRunAt has elapsed,
    // and creates Jobs for each.
    Run(ctx context.Context) error

    // TriggerWatch creates an ad-hoc Job for a Watch outside the normal cadence.
    TriggerWatch(ctx context.Context, id domain.WatchID, trigger domain.JobTrigger) (domain.JobID, error)
}
```

#### Queue

```go
package queue

type Queue interface {
    Enqueue(ctx context.Context, task domain.Task) error
    Dequeue(ctx context.Context, stages []domain.Stage) (domain.Task, error) // blocks until a task is available
    Ack(ctx context.Context, id domain.TaskID) error
    Nack(ctx context.Context, id domain.TaskID, retryAfter time.Duration) error

    Ping(ctx context.Context) error
}
```

`Dequeue` takes a `stages` filter so worker pools can subscribe to specific
stages (e.g., a worker pool dedicated to `extract` because it's CPU-heavy).

#### Datastore

```go
package datastore

type Datastore interface {
    // Watches
    GetWatch(ctx context.Context, id domain.WatchID) (domain.Watch, error)
    ListWatches(ctx context.Context, f WatchFilter) ([]domain.Watch, error)
    UpsertWatch(ctx context.Context, w domain.Watch) error
    DueWatches(ctx context.Context, now time.Time, limit int) ([]domain.Watch, error)

    // Listings
    GetListing(ctx context.Context, id domain.ListingID) (domain.Listing, error)
    UpsertListing(ctx context.Context, l domain.Listing) error
    GetListingByEbayItemID(ctx context.Context, ebayID string) (domain.Listing, error) // for dedup

    // Components
    ReplaceComponents(ctx context.Context, listingID domain.ListingID, components []domain.Component) error

    // Scores, Judgments, MarketSignals, Notifications — same shape
    // ... see follow-up datastore DESIGN doc

    // Jobs and Tasks
    CreateJob(ctx context.Context, j domain.Job) error
    UpdateJobState(ctx context.Context, id domain.JobID, state domain.JobState) error
    UpsertTask(ctx context.Context, t domain.Task) error

    Ping(ctx context.Context) error
}
```

The Datastore interface is big; that's a known cost. The alternative — splitting
it into `WatchStore`, `ListingStore`, `JobStore` — is ceremony without payoff
when the same Postgres backend implements all of them. We split if and when a
different backend implements only a subset.

## API / Interface Changes

This document defines net-new types. No existing API changes.

## Data Model

Domain types map to Postgres tables in `internal/datastore/migrations/`.
Representative excerpts:

```sql
CREATE TABLE listings (
    id              uuid PRIMARY KEY,
    watch_id        uuid NOT NULL REFERENCES watches(id),
    ebay_item_id    text NOT NULL UNIQUE,
    title           text NOT NULL,
    price_amount    numeric(14,4) NOT NULL,
    price_currency  text NOT NULL,
    condition       smallint NOT NULL,
    listing_type    smallint NOT NULL,
    seller          jsonb NOT NULL,
    state           smallint NOT NULL DEFAULT 0,    -- ListingState enum
    sold_price_amount   numeric(14,4),               -- populated when state = Sold
    sold_price_currency text,
    sold_at         timestamptz,                    -- populated when state = Sold
    ended_at        timestamptz,                    -- populated when state in (Sold, EndedNoSale, NotFound)
    listed_at       timestamptz NOT NULL,
    ends_listed_at  timestamptz,
    first_seen_at   timestamptz NOT NULL,
    last_seen_at    timestamptz NOT NULL,
    raw_payload     jsonb NOT NULL
);
CREATE INDEX listings_watch_first_seen_idx ON listings (watch_id, first_seen_at DESC);
CREATE INDEX listings_ebay_item_idx ON listings (ebay_item_id);
CREATE INDEX listings_reconcile_idx ON listings (state, last_seen_at)
    WHERE state = 0;   -- drives the 12h bulk reconciliation query

CREATE TABLE price_observations (
    listing_id      uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    observed_at     timestamptz NOT NULL,
    price_amount    numeric(14,4) NOT NULL,
    price_currency  text NOT NULL,
    source          smallint NOT NULL,
    PRIMARY KEY (listing_id, observed_at)
);
CREATE INDEX price_observations_listing_idx ON price_observations (listing_id, observed_at DESC);

CREATE TABLE alerts (
    id                    uuid PRIMARY KEY,
    watch_id              uuid NOT NULL REFERENCES watches(id),
    listing_id            uuid NOT NULL REFERENCES listings(id),
    triggered_by_score_id uuid NOT NULL REFERENCES scores(id),
    latest_score_id       uuid NOT NULL REFERENCES scores(id),
    latest_score_value    numeric(14,6) NOT NULL,
    state                 smallint NOT NULL DEFAULT 0,
    opened_at             timestamptz NOT NULL,
    closed_at             timestamptz,
    close_reason          smallint,
    watch_cycles_observed integer NOT NULL DEFAULT 0,
    notifications_sent    integer NOT NULL DEFAULT 0,
    last_notified_at      timestamptz,
    stale                 boolean NOT NULL DEFAULT false,
    last_reconciled_at    timestamptz,
    last_reconcile_error  text
);
-- Drive the stale-alert Prometheus gauge cheaply.
CREATE INDEX alerts_stale_idx ON alerts (watch_id) WHERE state = 0 AND stale = true;
-- At most one OPEN alert per (Watch, Listing). Closed alerts remain for audit
-- and can coexist with new open alerts on the same pair.
CREATE UNIQUE INDEX alerts_open_unique_idx ON alerts (watch_id, listing_id) WHERE state = 0;
CREATE INDEX alerts_open_by_watch_idx ON alerts (watch_id) WHERE state = 0;

CREATE TABLE components (
    id              uuid PRIMARY KEY,
    listing_id      uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    kind            smallint NOT NULL,
    manufacturer    text,
    model           text,
    quantity        integer NOT NULL DEFAULT 1,
    spec            jsonb NOT NULL,
    confidence      numeric(4,3) NOT NULL,
    extractor_ver   text NOT NULL,
    extracted_at    timestamptz NOT NULL
);
CREATE INDEX components_listing_idx ON components (listing_id);
CREATE INDEX components_kind_idx ON components (kind);
CREATE INDEX components_high_confidence_idx ON components (kind, listing_id)
    WHERE confidence >= 0.5;   -- baseline-eligible components

CREATE TABLE market_signals (
    watch_id        uuid NOT NULL REFERENCES watches(id),
    window_seconds  integer NOT NULL,
    as_of           timestamptz NOT NULL,
    sample_count    integer NOT NULL,
    price_currency  text NOT NULL,
    price_min       numeric(14,4) NOT NULL,
    price_max       numeric(14,4) NOT NULL,
    price_median    numeric(14,4) NOT NULL,
    price_mean      numeric(14,4) NOT NULL,
    price_stddev    numeric(14,6) NOT NULL,
    percentiles     jsonb NOT NULL,   -- {"p10":..., "p25":..., ...}
    moving_avg      numeric(14,4) NOT NULL,
    median_time_to_sell_seconds  bigint,
    avg_listing_length_days      numeric(10,4),
    PRIMARY KEY (watch_id, window_seconds, as_of)
);
CREATE INDEX market_signals_latest_idx ON market_signals (watch_id, window_seconds, as_of DESC);
```

Authoritative DDL lives in migrations. This is illustrative — especially the
index choices, which we'll tune against real query patterns.

## Testing Strategy

Domain types are pure data; tests are minimal — mostly serialization round-trips
and enum/string conversion. The real testing target is downstream code that
consumes these types, covered by per-package tests.

One thing worth a dedicated test: the `ComponentSpec` JSON round-trip. The
sealed-interface pattern means encoding needs a discriminator tag (`kind`) and
decoding needs a type switch. A table-driven test per Spec type ensures every
Kind round-trips through `(json.Marshal → json.Unmarshal → Equal)` cleanly.

## Migration / Rollout Plan

This is greenfield. No migration needed. As Phase 1 packages land, they import
these types directly.

## Open Questions

The first round of open questions has been resolved. Decisions and the reasoning
behind them:

### Resolved

- **✅ Decimal library: `cockroachdb/apd`.** Picked over `shopspring/decimal` on
  activity grounds (apd has more-recent maintenance and is the library Cockroach
  uses for its `DECIMAL` type). The explicit precision/rounding context that
  `apd.Context` provides is a fit for percentile and moving-average math where
  rounding behavior needs to be deterministic.

- **✅ `Listing` state detection and sell-price capture.** Resolved by a
  four-state `ListingState` enum (`Live`, `Sold`, `EndedNoSale`, `NotFound`)
  plus a paired set of nullable terminal-state fields. _How_ we detect these
  transitions is via two reconciliation flows:
  1. **Alert-driven reconciliation** at the start of the _next_ watcher cycle
     after an Alert is opened, before baseline recalc. The flow re-fetches each
     alerting listing via the eBay `getItem` endpoint (added to DESIGN-0003),
     updates `PriceObservation`s if the price changed, and transitions
     `ListingState` if the listing has ended.
  2. **12-hour bulk reconciliation**, skewed 2 hours under eBay's 24h rate
     window so we have headroom. Re-fetches any listing with
     `State == Live AND LastSeenAt < now - 12h`, subject to a separate quota
     budget that defers to watch polls when quota is tight.

  Full flow — including baseline recalc, alert re-evaluation, the bulk-vs-alert quota split, and re-notification policy — lives in [DESIGN-0004 — Alert and reconciliation pipeline](0004-alert-and-reconciliation-pipeline.md).

- **✅ MarketSignal storage: Postgres.** A `market_signals` table keyed by
  `(watch_id, window, as_of)`. NOT Prometheus (system-metrics only — that was
  the prior version's mistake) and NOT ClickHouse (reserved for traces + agent
  observability). Low-volume application-derived analytics belong in the
  transactional store next to the data they aggregate.

- **✅ `Component.Confidence` source: LLM self-report, with a hard floor.**
  Range 0.0–1.0, emitted by the extractor. Components with `Confidence < 0.5`
  are persisted but **excluded from baseline calculations** and tagged for
  manual review / judge evaluation. The signal we tune extractor prompts against
  is mean-near-1.0 and spread-under-0.2 per Kind.

- **✅ Multi-tenancy and `UserID`: removed.** No `UserID` on `Watch` or
  `Notification`. spt is single-tenant in v1. The API auth boundary, when added,
  will most likely be a shared bearer token (env-configurable, set in the Helm
  chart) — sufficient to keep a self-hosted spt off the public internet without
  dragging in user identity. If we ever add OIDC/SSO for an SaaS or multi-user
  deployment, that's a separate ADR plus a `UserID` retrofit, but the type
  system shouldn't pre-empt it.

### Still open

- **GPU and exotic components.** GPUs are listed as `ComponentKindGPU` but server listings often include them ambiguously ("with GPU support" vs. "GPU included"). The extractor's job, not the type system's, to disambiguate.

### Owned elsewhere

The following items appeared on earlier revisions of this doc but are owned by [DESIGN-0004 — Alert and reconciliation pipeline](0004-alert-and-reconciliation-pipeline.md):

- Alert re-notification policy (resolved in DESIGN-0004: simple open/close transitions, no cooldown).
- Bulk reconciliation quota budget and prioritization.
- `Alert.Stale` state and the associated stale-alert metric.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0004 — Use PostgreSQL as the canonical relational store](../adr/0004-use-postgresql-as-the-canonical-relational-store.md)
- [ADR-0008 — Use OTel + ClickHouse + Langfuse for agent observability and evals](../adr/0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)
- [ADR-0010 — Generate the frontend API client from OpenAPI](../adr/0010-generate-the-frontend-api-client-from-openapi.md)
- [ADR-0012 — Build a custom scheduler and pipeline orchestrator](../adr/0012-build-a-custom-scheduler-and-pipeline-orchestrator.md)
- [DESIGN-0001 — Go application layout and conventions](0001-go-application-layout-and-conventions.md)
- cockroachdb/apd: <https://github.com/cockroachdb/apd>
