package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/donaldgifford/spt/internal/config"
)

func TestRunReturnsCanceledOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Run(ctx, &config.Config{Admin: config.AdminConfig{Addr: ":0"}})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run: got %v, want context.Canceled", err)
	}
}
