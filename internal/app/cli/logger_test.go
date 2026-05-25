package cli

import (
	"errors"
	"testing"

	"github.com/donaldgifford/spt/internal/obs"
)

func TestInstallSlogValid(t *testing.T) {
	if err := installSlog("json", "info"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallSlogInvalidFormat(t *testing.T) {
	err := installSlog("yaml", "info")
	if !errors.Is(err, obs.ErrUnknownLogFormat) {
		t.Errorf("got %v, want obs.ErrUnknownLogFormat", err)
	}
}

func TestInstallSlogInvalidLevel(t *testing.T) {
	err := installSlog("json", "trace")
	if !errors.Is(err, obs.ErrUnknownLogLevel) {
		t.Errorf("got %v, want obs.ErrUnknownLogLevel", err)
	}
}
