package agent

import (
	"context"
	"errors"
)

// Sentinel errors implementations return.
var (
	// ErrAgentUnavailable signals the underlying LLM provider is
	// unreachable; callers typically retry with backoff.
	ErrAgentUnavailable = errors.New("agent: provider unavailable")
)

// Agent is the placeholder contract for the LLM-touching stage
// handlers (extract, judge). Phase 6 ships the minimum surface the
// orchestrator's stage handlers will call against; the agentic IMPL
// fleshes out the actual prompt + tool wiring.
//
// Calls into Agent are the spans that get obs.SpanCategoryAgent so
// they fork to Langfuse via the agent/system split (DESIGN-0005 §
// "OTel span categories"). The default category is system; agent
// handlers SetCategory at span-start time.
type Agent interface {
	// Complete runs a single text-in / text-out interaction. Used by
	// stage handlers that need a single LLM step; multi-step or
	// tool-use flows live in the agentic IMPL.
	Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error)
}

// CompleteRequest is the input shape for Agent.Complete. Fields are
// the minimum cross-provider subset; provider-specific knobs land
// when the concrete client does.
type CompleteRequest struct {
	System      string
	Prompt      string
	Model       string
	Temperature float64
	MaxTokens   int
}

// CompleteResponse carries the model's reply plus usage accounting
// so callers can attach Prometheus metrics and Langfuse costs.
type CompleteResponse struct {
	Text         string
	Model        string
	InputTokens  int
	OutputTokens int
}
