package worker

import (
	"context"
	"log/slog"

	"github.com/donaldgifford/spt/internal/config"
)

// Run starts the spt worker role and blocks until ctx is cancelled.
//
// Phase 2 (IMPL-0001) ships a stub that logs a startup line, waits on
// ctx, and returns ctx.Err() on shutdown. Later phases wire the per-stage
// worker pools from DESIGN-0005 — Worker pool model.
func Run(ctx context.Context, cfg *config.Config) error {
	slog.InfoContext(ctx, "worker role starting",
		slog.String("admin_addr", cfg.Admin.Addr),
	)
	<-ctx.Done()
	slog.InfoContext(ctx, "worker role stopped")
	return ctx.Err()
}
