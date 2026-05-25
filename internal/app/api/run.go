package api

import (
	"context"
	"log/slog"

	"github.com/donaldgifford/spt/internal/config"
)

// Run starts the spt api role and blocks until ctx is cancelled.
//
// Phase 2 (IMPL-0001) ships a stub that logs a startup line, waits on
// ctx, and returns ctx.Err() on shutdown. The api IMPL replaces this
// with the full http.Server wiring, OpenAPI handlers, and admin port.
func Run(ctx context.Context, cfg *config.Config) error {
	slog.InfoContext(ctx, "api role starting",
		slog.String("admin_addr", cfg.Admin.Addr),
	)
	<-ctx.Done()
	slog.InfoContext(ctx, "api role stopped")
	return ctx.Err()
}
