package config

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateAcceptsDefaults(t *testing.T) {
	cfg := Defaults()
	if err := Validate(&cfg); err != nil {
		t.Errorf("Defaults() must validate, got: %v", err)
	}
}

func TestValidateLogFormat(t *testing.T) {
	cfg := Defaults()
	cfg.Log.Format = "yaml"

	err := Validate(&cfg)
	if !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("got %v, want ErrOutOfRange", err)
	}
	if !strings.Contains(err.Error(), "log.format") {
		t.Errorf("error should mention log.format: %v", err)
	}
}

func TestValidateLogLevel(t *testing.T) {
	cfg := Defaults()
	cfg.Log.Level = "trace"

	err := Validate(&cfg)
	if !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("got %v, want ErrOutOfRange", err)
	}
}

func TestValidateObsSpanSamplingOutOfRange(t *testing.T) {
	cfg := Defaults()
	cfg.Obs.SpanSampling = 2.0

	err := Validate(&cfg)
	if !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("got %v, want ErrOutOfRange", err)
	}
}

func TestValidateBadDuration(t *testing.T) {
	cfg := Defaults()
	cfg.Scheduler.TickInterval = "15 minutes" // not a Go duration string

	err := Validate(&cfg)
	if !errors.Is(err, ErrInvalidDuration) {
		t.Fatalf("got %v, want ErrInvalidDuration", err)
	}
}

func TestValidateWorkerConcurrency(t *testing.T) {
	cfg := Defaults()
	cfg.Worker.Pools = []PoolConfig{{Stage: "extract", Concurrency: 0}}

	err := Validate(&cfg)
	if !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("got %v, want ErrOutOfRange", err)
	}
	if !strings.Contains(err.Error(), `worker.pools["extract"].concurrency`) {
		t.Errorf("error should include pool stage: %v", err)
	}
}

func TestValidateWatchRequiresQuery(t *testing.T) {
	cfg := Defaults()
	cfg.Watches = []WatchConfig{{Name: "w1"}}

	err := Validate(&cfg)
	if !errors.Is(err, ErrRequired) {
		t.Fatalf("got %v, want ErrRequired", err)
	}
}

func TestValidateWatchJudgeSampleRange(t *testing.T) {
	cfg := Defaults()
	cfg.Watches = []WatchConfig{{Name: "w1", Query: "q", JudgeSampleRate: 1.5}}

	err := Validate(&cfg)
	if !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("got %v, want ErrOutOfRange", err)
	}
}

func TestValidateAggregatesMultipleProblems(t *testing.T) {
	cfg := Defaults()
	cfg.Log.Format = "yaml"
	cfg.Log.Level = "trace"
	cfg.Obs.SpanSampling = -1

	err := Validate(&cfg)
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("got %T, want *ValidationError", err)
	}
	if len(vErr.Problems) != 3 {
		t.Errorf("problems: got %d want 3 (%v)", len(vErr.Problems), vErr.Problems)
	}
}

func TestValidationErrorIsUnwraps(t *testing.T) {
	v := &ValidationError{
		Problems: []FieldProblem{
			{Field: "log.format", Err: ErrOutOfRange},
		},
	}
	if !errors.Is(v, ErrOutOfRange) {
		t.Error("errors.Is on ValidationError should find ErrOutOfRange")
	}
	if errors.Is(v, ErrRequired) {
		t.Error("errors.Is on ValidationError should not find unrelated sentinel")
	}
}
