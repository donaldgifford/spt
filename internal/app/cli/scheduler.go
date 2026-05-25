package cli

import (
	"github.com/spf13/cobra"

	"github.com/donaldgifford/spt/internal/app/scheduler"
	"github.com/donaldgifford/spt/internal/config"
)

func newSchedulerCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "scheduler",
		Short: "Run the orchestrator role (cadence trigger + DAG walker + crons)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return scheduler.Run(cmd.Context(), cfg)
		},
	}
}
