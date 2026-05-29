package cli

import (
	"github.com/spf13/cobra"

	"github.com/donaldgifford/spt/internal/app/worker"
	"github.com/donaldgifford/spt/internal/config"
)

func newWorkerCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Run the worker role (per-stage Task executor pools)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return worker.Run(cmd.Context(), cfg)
		},
	}
}
