package cli

import (
	"fmt"
	"os"

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

// rootFlags holds the raw cobra-bound values for every persistent flag.
// PersistentPreRunE translates these into config.FlagOverrides — but
// only for flags the user actually set on the command line, so the
// friendly help-string defaults don't clobber HCL config values.
type rootFlags struct {
	logFormat   string
	logLevel    string
	adminAddr   string
	ebayAppID   string
	ebayCertID  string
	postgresDSN string
	valkeyAddr  string
	meiliURL    string
}

// NewRootCmd builds the top-level cobra command for the spt binary.
//
// The bare `spt` command has no Run function, so cobra's built-in help
// dispatch handles `spt` with no subcommand (IMPL-0001 Resolved Decision
// #1). Persistent flags are layered onto the HCL config by
// PersistentPreRunE via config.Load — CLI > env > HCL > defaults.
func NewRootCmd(info BuildInfo) *cobra.Command {
	cfg := &config.Config{}
	flags := &rootFlags{}

	root := &cobra.Command{
		Use:   "spt",
		Short: "Server Price Tracker",
		Long: "spt polls eBay queries, scores listings, derives market analytics, " +
			"and surfaces alerts on hardware that matches user-defined watches.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	bindRootFlags(root, cfg, flags)

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		overrides := overridesFrom(cmd, flags)

		loaded, err := config.Load(
			cfg.ConfigFiles, cfg.ConfigDir,
			config.EnvSliceToMap(os.Environ()), &overrides,
		)
		if err != nil {
			return fmt.Errorf("cli: load config: %w", err)
		}
		// Preserve the path fields the CLI populated; everything else
		// comes from the loader.
		loaded.ConfigFiles = cfg.ConfigFiles
		loaded.ConfigDir = cfg.ConfigDir
		*cfg = loaded
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

// bindRootFlags registers every persistent flag. The "config" /
// "config-dir" flags route directly to cfg (they tell the loader what
// files to read); the rest route to the flags struct and are translated
// to FlagOverrides only when the user actually typed them.
func bindRootFlags(root *cobra.Command, cfg *config.Config, flags *rootFlags) {
	pf := root.PersistentFlags()

	pf.StringSliceVar(&cfg.ConfigFiles, "config", nil,
		"Path to an HCL config file (repeatable; later files override earlier).")
	pf.StringVar(&cfg.ConfigDir, "config-dir", "",
		"Directory of HCL config files loaded in lexical order (before --config files).")

	pf.StringVar(&flags.logFormat, "log-format", "auto",
		`Log output format ("text", "json", or "auto" — TTY-detected on stderr).`)
	pf.StringVar(&flags.logLevel, "log-level", "info",
		`Log level ("debug", "info", "warn", "error").`)
	pf.StringVar(&flags.adminAddr, "admin-addr", ":9090",
		"Address for the admin server (/healthz, /readyz, /metrics).")

	pf.StringVar(&flags.ebayAppID, "ebay-app-id", "",
		"eBay App ID (overrides ebay.app_id from config / EBAY_APP_ID).")
	pf.StringVar(&flags.ebayCertID, "ebay-cert-id", "",
		"eBay Cert ID (overrides ebay.cert_id from config / EBAY_CERT_ID).")
	pf.StringVar(&flags.postgresDSN, "postgres-dsn", "",
		"Postgres DSN (overrides postgres.dsn from config / DATABASE_URL).")
	pf.StringVar(&flags.valkeyAddr, "valkey-addr", "",
		"Valkey address (overrides valkey.addr from config / VALKEY_ADDR).")
	pf.StringVar(&flags.meiliURL, "meili-url", "",
		"Meilisearch URL (overrides meilisearch.url from config / MEILI_URL).")
}

// overridesFrom builds a FlagOverrides containing only the fields the
// user actually set on the command line. cobra's Flag.Changed bit
// distinguishes "user supplied a value" from "flag default applied" —
// the former overrides HCL/env, the latter does not.
func overridesFrom(cmd *cobra.Command, flags *rootFlags) config.FlagOverrides {
	pf := cmd.Flags()
	var o config.FlagOverrides
	if pf.Changed("log-format") {
		o.LogFormat = flags.logFormat
	}
	if pf.Changed("log-level") {
		o.LogLevel = flags.logLevel
	}
	if pf.Changed("admin-addr") {
		o.AdminAddr = flags.adminAddr
	}
	if pf.Changed("ebay-app-id") {
		o.EbayAppID = flags.ebayAppID
	}
	if pf.Changed("ebay-cert-id") {
		o.EbayCertID = flags.ebayCertID
	}
	if pf.Changed("postgres-dsn") {
		o.PostgresDSN = flags.postgresDSN
	}
	if pf.Changed("valkey-addr") {
		o.ValkeyAddr = flags.valkeyAddr
	}
	if pf.Changed("meili-url") {
		o.MeiliURL = flags.meiliURL
	}
	return o
}
