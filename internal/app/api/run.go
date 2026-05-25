package api

import (
	"context"
	"time"

	"github.com/donaldgifford/spt/internal/config"
	"github.com/donaldgifford/spt/internal/obs"
)

// shutdownTimeout caps how long observability flush + future server
// shutdown can take after ctx cancels. 5s matches the per-role bound
// in DESIGN-0001 § "Process lifecycle and graceful shutdown".
const shutdownTimeout = 5 * time.Second

// Run starts the spt api role and blocks until ctx is cancelled.
//
// Phase 4 (IMPL-0001) wires obs.Setup for structured logging + OTel
// tracing + Prometheus metrics. The role still has no HTTP server; the
// api IMPL replaces the ctx.Wait with the full http.Server wiring,
// OpenAPI handlers, and admin port.
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

	o.Logger.InfoContext(ctx, "api role starting",
		"admin_addr", cfg.Admin.Addr,
	)
	<-ctx.Done()
	o.Logger.InfoContext(ctx, "api role stopped")
	return ctx.Err()
}
