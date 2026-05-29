//go:build integration

package datastore

import (
	"database/sql"
	"errors"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

const composeDSN = "postgres://spt:spt@127.0.0.1:55432/spt?sslmode=disable"

// openTestDB connects to the Compose Postgres and resets the goose
// bookkeeping table so each test starts from a clean migration state.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", composeDSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.PingContext(t.Context()))

	// Strip both the goose table and the migration's target so reruns
	// see a virgin database.
	_, _ = db.ExecContext(t.Context(), `DROP TABLE IF EXISTS _spt_meta`)
	_, _ = db.ExecContext(t.Context(), `DROP TABLE IF EXISTS goose_db_version`)
	return db
}

func TestMigratorUpAppliesAll(t *testing.T) {
	db := openTestDB(t)

	m, err := NewMigrator(db, nil)
	require.NoError(t, err)

	require.NoError(t, m.Up(t.Context()))

	// The placeholder migration creates _spt_meta with a single row.
	var v string
	require.NoError(t, db.QueryRowContext(t.Context(),
		`SELECT value FROM _spt_meta WHERE key = 'schema_version'`,
	).Scan(&v))
	require.Equal(t, "0.0.1", v)
}

func TestMigratorStatusReportsApplied(t *testing.T) {
	db := openTestDB(t)
	m, err := NewMigrator(db, nil)
	require.NoError(t, err)

	// Pre-apply Up so Status should report zero pending.
	require.NoError(t, m.Up(t.Context()))

	statuses, err := m.Status(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, statuses)

	var pending int
	for _, s := range statuses {
		if !s.Applied {
			pending++
		}
	}
	require.Zero(t, pending, "Status reports pending after Up: %+v", statuses)
}

func TestMigratorStatusFlagsPending(t *testing.T) {
	db := openTestDB(t)
	m, err := NewMigrator(db, nil)
	require.NoError(t, err)

	// Brand-new DB — every migration is pending.
	_, err = m.Status(t.Context())
	require.ErrorIs(t, err, ErrPendingMigrations)
}

func TestMigratorDownRollsBack(t *testing.T) {
	db := openTestDB(t)
	m, err := NewMigrator(db, nil)
	require.NoError(t, err)

	require.NoError(t, m.Up(t.Context()))
	require.NoError(t, m.Down(t.Context()))

	var exists bool
	require.NoError(t, db.QueryRowContext(t.Context(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '_spt_meta')`,
	).Scan(&exists))
	require.False(t, exists, "Down should have dropped _spt_meta")
}

func TestCheckPendingMigrationsEmptyDSN(t *testing.T) {
	// Empty DSN is the local-dev path: warn-and-skip, not error.
	err := CheckPendingMigrations(t.Context(), "", nopLogger())
	require.NoError(t, err)
}

func TestCheckPendingMigrationsFailsWhenPending(t *testing.T) {
	db := openTestDB(t)
	_ = db // openTestDB resets state; we just need the side effect

	err := CheckPendingMigrations(t.Context(), composeDSN, nopLogger())
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrPendingMigrations),
		"got %v, want ErrPendingMigrations", err)
}
