// Package config defines the typed configuration model and the HCL2
// loader that populates it from files, env vars, and CLI flags.
package config

// Config is the root spt configuration shared across roles.
//
// Decoded from HCL by the loader and layered with env-var and CLI-flag
// overrides. Duration-typed fields are kept as strings (e.g., "15m")
// and parsed via the helpers in durations.go because gohcl's cty
// decoder doesn't handle time.Duration natively.
//
// Note: the loader uses a separate parse-only schema (see loader.go)
// because gohcl requires single-instance blocks to be pointers when
// the user is allowed to omit them. Keeping pointers off the public
// type means consumers don't have to nil-check every section.
type Config struct {
	// ConfigFiles holds the --config flag values (repeatable). Set by
	// the CLI layer before calling Load; not loaded from HCL.
	ConfigFiles []string

	// ConfigDir is the --config-dir flag value; HCL files inside are
	// loaded in lexical order before any --config files. Set by the
	// CLI layer before calling Load.
	ConfigDir string

	Log         LogConfig
	Admin       AdminConfig
	Ebay        EbayConfig
	Postgres    PostgresConfig
	Valkey      ValkeyConfig
	Meilisearch MeilisearchConfig
	Obs         ObsConfig
	API         APIConfig
	Scheduler   SchedulerConfig
	Worker      WorkerConfig

	// Watches are the bootstrap Watch declarations seeded into the
	// datastore at startup. Phase 3 just decodes them; the seed logic
	// lives in the datastore IMPL — see internal/config/README.md.
	Watches []WatchConfig
}

// LogConfig controls structured-log output.
type LogConfig struct {
	// Format is one of "text", "json", or "auto" (TTY-detected on stderr).
	Format string `hcl:"format,optional"`
	// Level is one of "debug", "info", "warn", "error".
	Level string `hcl:"level,optional"`
}

// AdminConfig controls the per-role admin server (/healthz, /readyz, /metrics).
type AdminConfig struct {
	// Addr is the listen address; default ":9090".
	Addr string `hcl:"addr,optional"`
}

// EbayConfig holds eBay Browse/Marketplace credentials and per-instance
// rate limits. AppID and CertID are required for the api/worker roles
// once the eBay client lands (DESIGN-0003).
type EbayConfig struct {
	AppID       string `hcl:"app_id,optional"`
	CertID      string `hcl:"cert_id,optional"`
	Marketplace string `hcl:"marketplace,optional"`
	// RateLimit is requests-per-second; 0 = unlimited (development only).
	RateLimit int `hcl:"rate_limit,optional"`
}

// PostgresConfig is the canonical store connection (ADR-0004).
type PostgresConfig struct {
	DSN          string `hcl:"dsn,optional"`
	MaxOpenConns int    `hcl:"max_open_conns,optional"`
	MaxIdleConns int    `hcl:"max_idle_conns,optional"`
}

// ValkeyConfig is the queue + cache (ADR-0005).
type ValkeyConfig struct {
	Addr     string `hcl:"addr,optional"`
	DB       int    `hcl:"db,optional"`
	Password string `hcl:"password,optional"`
}

// MeilisearchConfig is the search index (ADR-0006).
type MeilisearchConfig struct {
	URL    string `hcl:"url,optional"`
	APIKey string `hcl:"api_key,optional"`
}

// ObsConfig wires OTel exporters and Langfuse credentials (ADR-0008/0009).
type ObsConfig struct {
	OTLPEndpoint      string `hcl:"otlp_endpoint,optional"`
	LangfuseHost      string `hcl:"langfuse_host,optional"`
	LangfusePublicKey string `hcl:"langfuse_public_key,optional"`
	LangfuseSecretKey string `hcl:"langfuse_secret_key,optional"`
	// SpanSampling is the head sampling ratio for system spans (0.0–1.0).
	SpanSampling float64 `hcl:"span_sampling,optional"`
}

// APIConfig is the HTTP server config for the api role. Duration fields
// take Go duration strings ("15s", "2m"); use ParsedReadTimeout /
// ParsedWriteTimeout to consume them as time.Duration values.
type APIConfig struct {
	Addr         string `hcl:"addr,optional"`
	ReadTimeout  string `hcl:"read_timeout,optional"`
	WriteTimeout string `hcl:"write_timeout,optional"`
}

// SchedulerConfig holds the orchestrator's cron intervals. Use the
// Parsed* helpers in durations.go to read them as time.Duration.
type SchedulerConfig struct {
	TickInterval          string `hcl:"tick_interval,optional"`
	BulkReconcileInterval string `hcl:"bulk_reconcile_interval,optional"`
	SyncInterval          string `hcl:"sync_interval,optional"`
}

// WorkerConfig holds per-stage pool configuration. The DESIGN-0005
// `worker { pools "<stage>" { concurrency = N } }` HCL shape decodes
// into Pools as a slice of labelled blocks.
type WorkerConfig struct {
	Pools []PoolConfig `hcl:"pools,block"`
}

// PoolConfig is one stage pool's settings. Stage is the HCL block's
// label (e.g., `pools "extract" { concurrency = 8 }`).
type PoolConfig struct {
	Stage       string `hcl:"stage,label"`
	Concurrency int    `hcl:"concurrency,optional"`
}

// WatchConfig is the bootstrap declaration for a single Watch. The
// scheduler reads these at startup and (per IMPL-0001 Resolved Decision
// #4 and #5) seeds them into the datastore — runtime CRUD then flows
// through the API. Phase 3 just parses + validates the block; seeding
// lives in a later IMPL.
type WatchConfig struct {
	Name            string  `hcl:"name,label"`
	Query           string  `hcl:"query,optional"`
	Cadence         string  `hcl:"cadence,optional"`
	JudgeSampleRate float64 `hcl:"judge_sample_rate,optional"`

	Notify []WatchNotifyConfig `hcl:"notify,block"`
}

// WatchNotifyConfig is one notification target for a Watch.
type WatchNotifyConfig struct {
	Channel    string                 `hcl:"channel,optional"`
	Thresholds []WatchThresholdConfig `hcl:"threshold,block"`
}

// WatchThresholdConfig is one trigger condition under a notify block.
type WatchThresholdConfig struct {
	MaxPercentile float64 `hcl:"max_percentile,optional"`
}

// FlagOverrides carries the subset of values the CLI layer may inject
// after the HCL parse completes; an empty string means do not override.
type FlagOverrides struct {
	LogFormat string
	LogLevel  string
	AdminAddr string

	EbayAppID  string
	EbayCertID string

	PostgresDSN string
	ValkeyAddr  string
	MeiliURL    string
}

// Defaults returns a Config with the documented zero-value defaults
// applied. Load calls this before merging file/env/flag values so
// optional attributes have sensible fallbacks.
func Defaults() Config {
	return Config{
		Log: LogConfig{
			Format: "auto",
			Level:  "info",
		},
		Admin: AdminConfig{
			Addr: ":9090",
		},
		Ebay: EbayConfig{
			Marketplace: "EBAY_US",
		},
		API: APIConfig{
			Addr:         ":8080",
			ReadTimeout:  "15s",
			WriteTimeout: "15s",
		},
		Scheduler: SchedulerConfig{
			TickInterval:          "5s",
			BulkReconcileInterval: "12h",
			SyncInterval:          "5m",
		},
		Obs: ObsConfig{
			SpanSampling: 1.0,
		},
	}
}
