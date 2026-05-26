package datastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx stdlib driver for database/sql
)

// CheckPendingMigrations is the fail-fast guard every role's Run calls
// at startup. Per IMPL-0001 Resolved Decision #12 there is no
// auto-migrate — operators run `spt migrate up` explicitly as a
// Kubernetes Job / initContainer before deploying role pods.
//
// When dsn is empty (the local-dev case) the function logs a warning
// and returns nil so stub roles still start. When dsn is set and
// there are pending migrations, the function returns the wrapped
// ErrPendingMigrations so the role exits non-zero with a clear
// operator-facing message.
func CheckPendingMigrations(ctx context.Context, dsn string, logger *slog.Logger) error {
	if dsn == "" {
		logger.WarnContext(ctx,
			"postgres DSN not configured; skipping migration check",
		)
		return nil
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("datastore: open postgres for migration check: %w", err)
	}
	defer func() { _ = db.Close() }() //nolint:errcheck // close error in defer is not actionable

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("datastore: ping postgres for migration check: %w", err)
	}

	m, err := NewMigrator(db, nil)
	if err != nil {
		return err
	}

	_, err = m.Status(ctx)
	if errors.Is(err, ErrPendingMigrations) {
		return fmt.Errorf("%w (run `spt migrate up` before starting the role)", err)
	}
	return err
}
