package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/donaldgifford/spt/internal/domain"
)

// AnthropicBackend invokes the Anthropic Messages API. We stub the
// SDK call to keep the tool buildable without the SDK dependency
// pulled in for tests; the agent IMPL wires the real client.
type AnthropicBackend struct {
	Model string // e.g. "claude-haiku-4-5-20251001"
}

// Name is the backend's --backend value.
func (AnthropicBackend) Name() string { return "anthropic" }

// Extract performs one extraction round-trip. Placeholder until the
// agent IMPL ships the production prompt + parsing.
func (AnthropicBackend) Extract(_ context.Context, l domain.Listing) ([]domain.Component, error) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return nil, fmt.Errorf("%w: AnthropicBackend needs $ANTHROPIC_API_KEY",
			ErrBackendUnconfigured)
	}
	time.Sleep(time.Millisecond)
	return []domain.Component{{ListingID: l.ID, Kind: "CPU", Confidence: 0.95}}, nil
}
