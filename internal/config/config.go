package config

// Config is the root spt configuration shared across roles.
//
// Phase 2 (IMPL-0001) ships the minimal subset needed to wire the cobra
// tree and per-role Run signatures: log + admin + the file paths the
// loader will read. Phase 3 extends with EbayConfig, PostgresConfig,
// ValkeyConfig, MeilisearchConfig, ObsConfig, ApiConfig, SchedulerConfig,
// and WorkerConfig populated from HCL2.
type Config struct {
	// ConfigFiles holds the --config flag values (repeatable).
	ConfigFiles []string

	// ConfigDir is the --config-dir flag value; HCL files inside are
	// loaded in lexical order before any --config files.
	ConfigDir string

	Log   LogConfig
	Admin AdminConfig
}

// LogConfig controls structured-log output.
type LogConfig struct {
	// Format is one of "text", "json", or "auto" (TTY-detected).
	Format string

	// Level is one of "debug", "info", "warn", "error".
	Level string
}

// AdminConfig controls the per-role admin server (/healthz, /readyz, /metrics).
type AdminConfig struct {
	// Addr is the listen address; default ":9090".
	Addr string
}
