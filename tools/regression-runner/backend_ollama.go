package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/donaldgifford/spt/internal/domain"
)

// OllamaBackend invokes a local Ollama HTTP API. We stub the HTTP
// dance to keep this tool buildable without a running Ollama — when
// the agent IMPL ships its production extractor, plug it in here.
type OllamaBackend struct {
	Endpoint string // defaults to http://127.0.0.1:11434 if empty
	Model    string // e.g. "llama3"
}

// Name is the backend's --backend value.
func (OllamaBackend) Name() string { return "ollama" }

// Extract performs one extraction round-trip. The current
// implementation is a placeholder; replace it with the real Ollama
// /api/generate call when the agent IMPL provides a stable prompt
// shape.
func (b OllamaBackend) Extract(_ context.Context, l domain.Listing) ([]domain.Component, error) {
	if os.Getenv("OLLAMA_HOST") == "" && b.Endpoint == "" {
		return nil, fmt.Errorf("%w: OllamaBackend needs --endpoint or $OLLAMA_HOST",
			ErrBackendUnconfigured)
	}
	// Placeholder behavior: return a deterministic stub so the
	// matcher can be exercised end-to-end.
	time.Sleep(time.Millisecond)
	return []domain.Component{{ListingID: l.ID, Kind: "CPU", Confidence: 0.9}}, nil
}
