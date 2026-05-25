package cli

import (
	"github.com/spf13/cobra"

	"github.com/donaldgifford/spt/internal/config"
)

// BuildInfo carries the ldflags-injected identity from main into the
// cobra tree so subcommands (notably `spt version`) can report it.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// NewRootCmd builds the top-level cobra command for the spt binary.
//
// The bare `spt` command has no Run function, so cobra's built-in help
// dispatch handles `spt` with no subcommand (IMPL-0001 Resolved Decision
// #1). Persistent flags bind to a shared *config.Config that Phase 3 will
// also populate from HCL2 files via a PersistentPreRunE hook.
func NewRootCmd(info BuildInfo) *cobra.Command {
	cfg := &config.Config{}

	root := &cobra.Command{
		Use:   "spt",
		Short: "Server Price Tracker",
		Long: "spt polls eBay queries, scores listings, derives market analytics, " +
			"and surfaces alerts on hardware that matches user-defined watches.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	flags := root.PersistentFlags()
	flags.StringSliceVar(&cfg.ConfigFiles, "config", nil,
		"Path to an HCL config file (repeatable; later files override earlier).")
	flags.StringVar(&cfg.ConfigDir, "config-dir", "",
		"Directory of HCL config files loaded in lexical order (before --config files).")
	flags.StringVar(&cfg.Log.Format, "log-format", logFormatAuto,
		`Log output format ("text", "json", or "auto" — TTY-detected on stderr).`)
	flags.StringVar(&cfg.Log.Level, "log-level", "info",
		`Log level ("debug", "info", "warn", "error").`)
	flags.StringVar(&cfg.Admin.Addr, "admin-addr", ":9090",
		"Address for the admin server (/healthz, /readyz, /metrics).")

	// Install structured logging once flags have parsed. Subcommands inherit
	// the configured slog logger. Phase 3 will extend this hook to also
	// load HCL config; Phase 4 will replace installSlog with obs.Setup.
	root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		return installSlog(cfg.Log.Format, cfg.Log.Level)
	}

	root.AddCommand(
		newVersionCmd(info),
		newAPICmd(cfg),
		newSchedulerCmd(cfg),
		newWorkerCmd(cfg),
		newMigrateCmd(cfg),
	)

	return root
}
