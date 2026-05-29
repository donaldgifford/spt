package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/donaldgifford/spt/internal/config"
	"github.com/donaldgifford/spt/internal/datastore"
	"github.com/donaldgifford/spt/internal/health"
	"github.com/donaldgifford/spt/internal/obs"
)

// shutdownTimeout caps the obs/server flush window during graceful
// shutdown — see DESIGN-0001 § "Process lifecycle and graceful shutdown".
const shutdownTimeout = 5 * time.Second

// Run starts the spt worker role and blocks until ctx is cancelled.
//
// Phase 5 (IMPL-0001) adds the admin port (cfg.Admin.Addr) serving
// /healthz, /readyz, and /metrics. Later phases wire the per-stage
// worker pools from DESIGN-0005 — Worker pool model.
func Run(ctx context.Context, cfg *config.Config) error {
	o, shutdown, err := obs.Setup(ctx, cfg, "spt-worker")
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := shutdown(shutdownCtx); err != nil {
			o.Logger.WarnContext(shutdownCtx, "obs shutdown returned error", "error", err)
		}
	}()

	if err := datastore.CheckPendingMigrations(ctx, cfg.Postgres.DSN, o.Logger); err != nil {
		return fmt.Errorf("worker: %w", err)
	}

	h := health.New(o.Registry)

	o.Logger.InfoContext(ctx, "worker role starting",
		"admin_addr", cfg.Admin.Addr,
	)

	adminErr := make(chan error, 1)
	go func() { adminErr <- h.Serve(ctx, cfg.Admin.Addr) }()

	select {
	case <-ctx.Done():
		if err := <-adminErr; err != nil && !errors.Is(err, health.ErrServerClosed) {
			o.Logger.ErrorContext(ctx, "admin server shutdown error", "error", err)
		}
		o.Logger.InfoContext(ctx, "worker role stopped")
		return ctx.Err()
	case err := <-adminErr:
		o.Logger.ErrorContext(ctx, "admin server failed", "error", err)
		return err
	}
}
