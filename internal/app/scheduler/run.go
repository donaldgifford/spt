package scheduler

import (
	"context"
	"log/slog"

	"github.com/donaldgifford/spt/internal/config"
)

// Run starts the spt scheduler role and blocks until ctx is cancelled.
//
// Phase 2 (IMPL-0001) ships a stub that logs a startup line, waits on
// ctx, and returns ctx.Err() on shutdown. Later phases wire the trigger
// loop, DAG walker, eBay Sync cron, bulk reconcile cron, and the
// Postgres advisory-lock leader from DESIGN-0005.
func Run(ctx context.Context, cfg *config.Config) error {
	slog.InfoContext(ctx, "scheduler role starting",
		slog.String("admin_addr", cfg.Admin.Addr),
	)
	<-ctx.Done()
	slog.InfoContext(ctx, "scheduler role stopped")
	return ctx.Err()
}
