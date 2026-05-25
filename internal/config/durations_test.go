package config

import (
	"errors"
	"testing"
	"time"
)

func TestParseOptionalDurationEmpty(t *testing.T) {
	d, err := parseOptionalDuration("field", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 0 {
		t.Errorf("empty input: got %v want 0", d)
	}
}

func TestParseOptionalDurationValid(t *testing.T) {
	d, err := parseOptionalDuration("field", "5m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 5*time.Minute {
		t.Errorf("got %v want 5m", d)
	}
}

func TestParseOptionalDurationInvalid(t *testing.T) {
	_, err := parseOptionalDuration("field", "5 minutes")
	if !errors.Is(err, ErrInvalidDuration) {
		t.Errorf("got %v, want ErrInvalidDuration", err)
	}
}

func TestSchedulerParsedAccessors(t *testing.T) {
	s := SchedulerConfig{
		TickInterval:          "1s",
		BulkReconcileInterval: "2h",
		SyncInterval:          "30m",
	}
	tick, err := s.ParsedTickInterval()
	if err != nil || tick != time.Second {
		t.Errorf("tick: got %v, %v", tick, err)
	}
	bulk, err := s.ParsedBulkReconcileInterval()
	if err != nil || bulk != 2*time.Hour {
		t.Errorf("bulk: got %v, %v", bulk, err)
	}
	sync, err := s.ParsedSyncInterval()
	if err != nil || sync != 30*time.Minute {
		t.Errorf("sync: got %v, %v", sync, err)
	}
}

func TestAPIConfigParsedAccessors(t *testing.T) {
	a := APIConfig{ReadTimeout: "10s", WriteTimeout: "20s"}
	r, err := a.ParsedReadTimeout()
	if err != nil || r != 10*time.Second {
		t.Errorf("read: got %v, %v", r, err)
	}
	w, err := a.ParsedWriteTimeout()
	if err != nil || w != 20*time.Second {
		t.Errorf("write: got %v, %v", w, err)
	}
}

func TestWatchParsedCadence(t *testing.T) {
	w := WatchConfig{Name: "w", Cadence: "15m"}
	c, err := w.ParsedCadence()
	if err != nil || c != 15*time.Minute {
		t.Errorf("cadence: got %v, %v", c, err)
	}
}
