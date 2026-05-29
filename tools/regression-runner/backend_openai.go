package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/donaldgifford/spt/internal/domain"
)

// OpenAIBackend invokes the OpenAI Chat Completions API. Stub for
// the same reason as the other two.
type OpenAIBackend struct {
	Model string // e.g. "gpt-4.1-mini"
}

// Name is the backend's --backend value.
func (OpenAIBackend) Name() string { return "openai" }

// Extract performs one extraction round-trip. Placeholder until the
// agent IMPL ships the production prompt + parsing.
func (OpenAIBackend) Extract(_ context.Context, l domain.Listing) ([]domain.Component, error) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		return nil, fmt.Errorf("%w: OpenAIBackend needs $OPENAI_API_KEY",
			ErrBackendUnconfigured)
	}
	time.Sleep(time.Millisecond)
	return []domain.Component{{ListingID: l.ID, Kind: "CPU", Confidence: 0.85}}, nil
}
