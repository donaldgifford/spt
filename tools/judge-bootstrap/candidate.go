package main

import (
	"github.com/donaldgifford/spt/internal/domain"
)

// Candidate is one Score surfaced by a strategy for operator review.
// Operators flip Accepted: true and fill Notes; `apply` then writes
// the accepted set to internal/agent/judge/examples.json.
//
// Notes is required when Accepted is true — the apply path validates
// this and exits non-zero with the offending ScoreIDs if violated.
type Candidate struct {
	ListingID  domain.ListingID   `json:"listing_id"`
	ScoreID    domain.ScoreID     `json:"score_id"`
	ScoreValue float64            `json:"score_value"`
	Components []domain.Component `json:"components"`
	Reasoning  string             `json:"reasoning,omitempty"`
	Why        string             `json:"why"`
	Accepted   bool               `json:"accepted"`
	Notes      string             `json:"notes,omitempty"`
}
