package datastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"

	"github.com/donaldgifford/spt/internal/datastore/migrations"
)

// ErrPendingMigrations signals there are migrations applied to the
// embedded set but not to the database. Each role's Run reports this
// at startup so the operator runs `spt migrate up` before pods serve
// traffic — matches the Kubernetes Job/initContainer deployment
// pattern (per IMPL-0001 Resolved Decision #12, no auto-migrate).
var ErrPendingMigrations = errors.New("datastore: pending migrations")

// Migrator runs spt's SQL migrations against a Postgres database. The
// migration set is embedded at build time via
// internal/datastore/migrations/embed.go; tests and `--migrations-dir`
// supply an alternate fs.FS for local iteration.
type Migrator struct {
	db       *sql.DB
	provider *goose.Provider
}

// NewMigrator returns a Migrator that reads migrations from filesys.
// Pass migrations.FS (the embedded set) in production; pass
// os.DirFS(dir) when --migrations-dir is set or in tests.
func NewMigrator(db *sql.DB, filesys fs.FS) (*Migrator, error) {
	if filesys == nil {
		filesys = migrations.FS
	}
	p, err := goose.NewProvider(goose.DialectPostgres, db, filesys)
	if err != nil {
		return nil, fmt.Errorf("datastore: build goose provider: %w", err)
	}
	return &Migrator{db: db, provider: p}, nil
}

// Up applies every pending migration in order. Idempotent — if the
// database is already at HEAD, returns nil with no side effects.
func (m *Migrator) Up(ctx context.Context) error {
	if _, err := m.provider.Up(ctx); err != nil {
		return fmt.Errorf("datastore: migrate up: %w", err)
	}
	return nil
}

// Down rolls back the most recently applied migration.
func (m *Migrator) Down(ctx context.Context) error {
	if _, err := m.provider.Down(ctx); err != nil {
		return fmt.Errorf("datastore: migrate down: %w", err)
	}
	return nil
}

// MigrationStatus is one row in Status's report.
type MigrationStatus struct {
	Version int64
	Source  string
	Applied bool
}

// Status returns the per-migration applied/pending state in version
// order. Returns ErrPendingMigrations alongside the report when one
// or more migrations are pending, so callers can both display the
// table and fail-fast.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	statuses, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("datastore: migrate status: %w", err)
	}
	out := make([]MigrationStatus, 0, len(statuses))
	pending := false
	for _, s := range statuses {
		applied := !s.AppliedAt.IsZero()
		if !applied {
			pending = true
		}
		out = append(out, MigrationStatus{
			Version: s.Source.Version,
			Source:  s.Source.Path,
			Applied: applied,
		})
	}
	if pending {
		return out, ErrPendingMigrations
	}
	return out, nil
}
