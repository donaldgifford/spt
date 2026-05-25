package worker

import (
	"context"
	"time"

	"github.com/donaldgifford/spt/internal/config"
	"github.com/donaldgifford/spt/internal/obs"
)

// shutdownTimeout caps the obs/server flush window during graceful
// shutdown — see DESIGN-0001 § "Process lifecycle and graceful shutdown".
const shutdownTimeout = 5 * time.Second

// Run starts the spt worker role and blocks until ctx is cancelled.
//
// Phase 4 (IMPL-0001) wires obs.Setup for structured logging + OTel
// tracing + Prometheus metrics. Later phases wire the per-stage
// worker pools from DESIGN-0005 — Worker pool model.
func Run(ctx context.Context, cfg *config.Config) error {
	o, shutdown, err := obs.Setup(ctx, cfg, "spt-worker")
	if err != nil {
		return err
	}
	defer func() {
		// WithoutCancel preserves the parent's deadline/values but
		// detaches from its cancellation so the bounded shutdown window
		// applies even when ctx was cancelled by SIGINT.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := shutdown(shutdownCtx); err != nil {
			o.Logger.WarnContext(shutdownCtx, "obs shutdown returned error", "error", err)
		}
	}()

	o.Logger.InfoContext(ctx, "worker role starting",
		"admin_addr", cfg.Admin.Addr,
	)
	<-ctx.Done()
	o.Logger.InfoContext(ctx, "worker role stopped")
	return ctx.Err()
}
