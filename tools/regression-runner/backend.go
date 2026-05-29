package main

import (
	"context"
	"errors"
	"time"

	"github.com/donaldgifford/spt/internal/domain"
)

// Backend is the per-model contract. Each provider (Ollama,
// Anthropic, OpenAI) implements this against its native HTTP/SDK.
type Backend interface {
	Name() string
	Extract(ctx context.Context, listing domain.Listing) ([]domain.Component, error)
}

// ErrBackendUnconfigured signals that the backend requires a
// credential or endpoint that isn't set in the environment. Callers
// (the runner) skip the backend in the report rather than aborting
// the whole run.
var ErrBackendUnconfigured = errors.New("regression-runner: backend unconfigured")

// MatchOutcome reflects the matcher's classification of a single
// extracted Component against the baseline truth.
type MatchOutcome int

// MatchOutcome enumeration per DESIGN-0006.
const (
	// NoMatch — the extracted Component doesn't match the baseline on
	// even the partial key.
	NoMatch MatchOutcome = iota
	// PartialMatch — Kind+Model+Manufacturer agree, but Quantity/Spec
	// differ.
	PartialMatch
	// ExactMatch — Kind+Model+Manufacturer+Quantity+Spec all agree.
	ExactMatch
)

// Result is one (backend, listing) extraction outcome.
type Result struct {
	Backend   string
	ListingID domain.ListingID
	Outcome   MatchOutcome
	Latency   time.Duration
	// Got/Expected are stored for the JSON report; tests inspect
	// these to verify aggregation correctness.
	Got      []domain.Component
	Expected []domain.Component
}

// BackendReport aggregates Results for one backend.
type BackendReport struct {
	Name            string             `json:"name"`
	Accuracy        float64            `json:"accuracy"`
	PerKindAccuracy map[string]float64 `json:"per_kind_accuracy"`
	LatencyP50      time.Duration      `json:"latency_p50"`
	LatencyP95      time.Duration      `json:"latency_p95"`
	Counts          OutcomeCounts      `json:"counts"`
	Results         []Result           `json:"results,omitempty"`
}

// OutcomeCounts is the per-outcome tally for a single BackendReport.
type OutcomeCounts struct {
	NoMatch      int `json:"no_match"`
	PartialMatch int `json:"partial_match"`
	ExactMatch   int `json:"exact_match"`
}

// Report is the top-level shape written to disk (or printed).
type Report struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Dataset     string          `json:"dataset"`
	Backends    []BackendReport `json:"backends"`
}
