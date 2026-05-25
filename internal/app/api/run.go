package api

import (
	"context"
	"errors"
	"time"

	"github.com/donaldgifford/spt/internal/config"
	"github.com/donaldgifford/spt/internal/health"
	"github.com/donaldgifford/spt/internal/obs"
)

// shutdownTimeout caps how long observability flush + future server
// shutdown can take after ctx cancels. 5s matches the per-role bound
// in DESIGN-0001 § "Process lifecycle and graceful shutdown".
const shutdownTimeout = 5 * time.Second

// Run starts the spt api role and blocks until ctx is cancelled.
//
// Phase 5 (IMPL-0001) adds the admin port (cfg.Admin.Addr) serving
// /healthz, /readyz, and /metrics. The role's business HTTP server is
// still a Phase 6+ deliverable; for now Run blocks on either ctx.Done
// or an admin-server listen failure.
func Run(ctx context.Context, cfg *config.Config) error {
	o, shutdown, err := obs.Setup(ctx, cfg, "spt-api")
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

	h := health.New(o.Registry)
	// Real readiness probes (postgres, valkey, meilisearch, eBay) get
	// registered when their clients land in later IMPLs.

	o.Logger.InfoContext(ctx, "api role starting",
		"admin_addr", cfg.Admin.Addr,
	)

	adminErr := make(chan error, 1)
	go func() { adminErr <- h.Serve(ctx, cfg.Admin.Addr) }()

	select {
	case <-ctx.Done():
		if err := <-adminErr; err != nil && !errors.Is(err, health.ErrServerClosed) {
			o.Logger.ErrorContext(ctx, "admin server shutdown error", "error", err)
		}
		o.Logger.InfoContext(ctx, "api role stopped")
		return ctx.Err()
	case err := <-adminErr:
		o.Logger.ErrorContext(ctx, "admin server failed", "error", err)
		return err
	}
}
