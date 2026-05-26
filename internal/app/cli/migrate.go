package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"text/tabwriter"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx stdlib driver for database/sql
	"github.com/spf13/cobra"

	"github.com/donaldgifford/spt/internal/config"
	"github.com/donaldgifford/spt/internal/datastore"
)

// newMigrateCmd returns the `spt migrate` parent command with up/down/status
// children. Migrations are embedded into the binary via
// internal/datastore/migrations; --migrations-dir on any child swaps
// in a real filesystem path for local dev.
func newMigrateCmd(cfg *config.Config) *cobra.Command {
	migrate := &cobra.Command{
		Use:   "migrate",
		Short: "Run SQL migrations against the canonical Postgres store",
	}

	var migrationsDir string
	migrate.PersistentFlags().StringVar(&migrationsDir, "migrations-dir", "",
		"Override the embedded migration set with files from this directory.")

	migrate.AddCommand(
		newMigrateUpCmd(cfg, &migrationsDir),
		newMigrateDownCmd(cfg, &migrationsDir),
		newMigrateStatusCmd(cfg, &migrationsDir),
	)

	return migrate
}

func newMigrateUpCmd(cfg *config.Config, dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMigrate(cmd.Context(), cfg, *dir, func(ctx context.Context, m *datastore.Migrator) error {
				return m.Up(ctx)
			})
		},
	}
}

func newMigrateDownCmd(cfg *config.Config, dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Roll back the most recently applied migration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMigrate(cmd.Context(), cfg, *dir, func(ctx context.Context, m *datastore.Migrator) error {
				return m.Down(ctx)
			})
		},
	}
}

func newMigrateStatusCmd(cfg *config.Config, dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print applied/pending migration state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMigrate(cmd.Context(), cfg, *dir, func(ctx context.Context, m *datastore.Migrator) error {
				statuses, err := m.Status(ctx)
				// Status returns ErrPendingMigrations alongside the report
				// — render the table either way, then surface the error
				// to set the exit code.
				renderStatus(cmd.OutOrStdout(), statuses)
				if err != nil && !errors.Is(err, datastore.ErrPendingMigrations) {
					return err
				}
				return nil
			})
		},
	}
}

// runMigrate opens a Postgres connection, builds a Migrator, and runs fn.
// Closes the connection unconditionally on return.
func runMigrate(
	ctx context.Context, cfg *config.Config, migrationsDir string,
	fn func(context.Context, *datastore.Migrator) error,
) error {
	if cfg.Postgres.DSN == "" {
		return errors.New("cli: postgres.dsn is required for migrate (set via --postgres-dsn, env DATABASE_URL, or HCL)")
	}

	db, err := sql.Open("pgx", cfg.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("cli: open postgres: %w", err)
	}
	defer func() { _ = db.Close() }() //nolint:errcheck // close error in defer is not actionable

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("cli: ping postgres: %w", err)
	}

	var filesys fs.FS
	if migrationsDir != "" {
		filesys = os.DirFS(migrationsDir)
	}

	mig, err := datastore.NewMigrator(db, filesys)
	if err != nil {
		return err
	}
	return fn(ctx, mig)
}

func renderStatus(w fileWriter, statuses []datastore.MigrationStatus) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	// Writes go to cobra's stdout writer; the only failure mode is a
	// closed pipe (operator piped to head, etc.) and there's nothing
	// useful to recover.
	defer func() { _ = tw.Flush() }()                  //nolint:errcheck // best-effort flush
	_, _ = fmt.Fprintln(tw, "VERSION\tSTATUS\tSOURCE") //nolint:errcheck // best-effort write
	for _, s := range statuses {
		state := "pending"
		if s.Applied {
			state = "applied"
		}
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\n", s.Version, state, s.Source) //nolint:errcheck // best-effort write
	}
}

// fileWriter narrows io.Writer enough for tabwriter while letting
// cobra's OutOrStdout (which returns io.Writer) flow through.
type fileWriter interface {
	Write(p []byte) (n int, err error)
}
