package cli

import (
	"github.com/spf13/cobra"

	"github.com/donaldgifford/spt/internal/app/api"
	"github.com/donaldgifford/spt/internal/config"
)

func newAPICmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "api",
		Short: "Run the HTTP API role (CRUD + Meilisearch-backed search)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return api.Run(cmd.Context(), cfg)
		},
	}
}
