package cli

import (
	"github.com/spf13/cobra"

	"github.com/donaldgifford/spt/internal/config"
)

// newMigrateCmd returns the `spt migrate` parent command with up/down/status
// child stubs.
//
// Phase 2 (IMPL-0001) ships placeholder children that print a "not yet
// implemented" message. Phase 8 wires goose against the embedded
// migrations under internal/datastore/migrations.
func newMigrateCmd(_ *config.Config) *cobra.Command {
	migrate := &cobra.Command{
		Use:   "migrate",
		Short: "Run SQL migrations against the canonical Postgres store",
	}

	migrate.AddCommand(
		newMigrateChildCmd("up", "Apply all pending migrations"),
		newMigrateChildCmd("down", "Roll back the most recently applied migration"),
		newMigrateChildCmd("status", "Print applied/pending migration state"),
	)

	return migrate
}

func newMigrateChildCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := cmd.OutOrStderr().Write([]byte("migrate " + use + ": not yet implemented (IMPL-0001 Phase 8)\n"))
			return err
		},
	}
}
