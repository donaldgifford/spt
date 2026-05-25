package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// Load reads HCL config from disk, layers env-var overrides (via the
// env() HCL function), applies CLI flag overrides, and validates the
// result. The flow is:
//
//  1. Start from Defaults().
//  2. Discover files: every *.hcl in cfg.ConfigDir (lexical order),
//     then each path in cfg.ConfigFiles (declaration order). Later
//     files override earlier ones.
//  3. Parse + merge + decode through an EvalContext that exposes
//     `env("VAR")` for inline env-var interpolation.
//  4. Apply FlagOverrides (CLI wins over everything else).
//  5. Validate; aggregated errors come back as *ValidationError.
//
// env is the environment passed to the `env()` HCL function. Production
// callers pass envSliceToMap(os.Environ()); tests pass synthetic maps.
//
// Discovery is explicit-only: no XDG / /etc/spt / $SPT_CONFIG lookups
// (per IMPL-0001 Resolved Decision #3). If both ConfigDir and
// ConfigFiles are empty, the loader returns Defaults + flag overrides
// without touching the filesystem.
func Load(
	paths []string, configDir string, env map[string]string, flags *FlagOverrides,
) (Config, error) {
	cfg := Defaults()
	cfg.ConfigFiles = paths
	cfg.ConfigDir = configDir

	files, err := discoverFiles(configDir, paths)
	if err != nil {
		return Config{}, err
	}

	if len(files) > 0 {
		if err := decodeFiles(files, env, &cfg); err != nil {
			return Config{}, err
		}
	}

	if flags != nil {
		applyFlagOverrides(&cfg, flags)
	}
	applyDefaultsAfterMerge(&cfg)

	if err := Validate(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// discoverFiles resolves the ordered list of HCL file paths to read:
// every *.hcl in dir (sorted alphabetically) followed by each explicit
// path. The dir need not exist when no --config-dir was supplied; an
// explicit dir that doesn't exist is a hard error.
func discoverFiles(dir string, paths []string) ([]string, error) {
	var out []string

	if dir != "" {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrReadFile, dir, err)
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !strings.HasSuffix(e.Name(), ".hcl") {
				continue
			}
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, n := range names {
			out = append(out, filepath.Join(dir, n))
		}
	}

	out = append(out, paths...)
	return out, nil
}

// parseSchema is the HCL-facing mirror of Config. Every top-level
// block is a pointer so the user can omit any of them, and gohcl
// reports unknown blocks/attrs (the only required-block path remains
// the labelled blocks which legitimately accept 0..N).
type parseSchema struct {
	Log         *LogConfig         `hcl:"log,block"`
	Admin       *AdminConfig       `hcl:"admin,block"`
	Ebay        *EbayConfig        `hcl:"ebay,block"`
	Postgres    *PostgresConfig    `hcl:"postgres,block"`
	Valkey      *ValkeyConfig      `hcl:"valkey,block"`
	Meilisearch *MeilisearchConfig `hcl:"meilisearch,block"`
	Obs         *ObsConfig         `hcl:"obs,block"`
	API         *APIConfig         `hcl:"api,block"`
	Scheduler   *SchedulerConfig   `hcl:"scheduler,block"`
	Worker      *WorkerConfig      `hcl:"worker,block"`

	Watches []WatchConfig `hcl:"watch,block"`
}

// decodeFiles parses and decodes each file separately, projecting the
// non-nil sections onto cfg in order. Sequential projection is the
// merge semantic: later files override earlier files block-by-block,
// while sections only one file declared are kept verbatim. env
// populates the env() HCL function so expressions like
// `app_id = env("EBAY_APP_ID")` resolve at decode time.
//
// Per-file decode (rather than hcl.MergeFiles + single decode) is
// necessary because MergeFiles rejects duplicate single-instance blocks
// across files, which would forbid the override pattern this loader
// exists to support.
func decodeFiles(paths []string, env map[string]string, cfg *Config) error {
	ctx := &hcl.EvalContext{
		Functions: map[string]function.Function{
			"env": envFunction(env),
		},
	}

	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrReadFile, p, err)
		}
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL(src, p)
		if diags.HasErrors() {
			return fmt.Errorf("%w: %s: %w", ErrParse, p, diags)
		}

		var schema parseSchema
		if diags := gohcl.DecodeBody(file.Body, ctx, &schema); diags.HasErrors() {
			return fmt.Errorf("%w: %w", ErrDecode, diags)
		}
		projectSchema(&schema, cfg)
	}
	return nil
}

// projectSchema copies every non-nil section from schema onto cfg.
// Sections the user omitted retain whatever cfg already has (which the
// caller seeded from Defaults), so partial config files only override
// what they actually declare.
func projectSchema(s *parseSchema, cfg *Config) {
	if s.Log != nil {
		cfg.Log = *s.Log
	}
	if s.Admin != nil {
		cfg.Admin = *s.Admin
	}
	if s.Ebay != nil {
		cfg.Ebay = *s.Ebay
	}
	if s.Postgres != nil {
		cfg.Postgres = *s.Postgres
	}
	if s.Valkey != nil {
		cfg.Valkey = *s.Valkey
	}
	if s.Meilisearch != nil {
		cfg.Meilisearch = *s.Meilisearch
	}
	if s.Obs != nil {
		cfg.Obs = *s.Obs
	}
	if s.API != nil {
		cfg.API = *s.API
	}
	if s.Scheduler != nil {
		cfg.Scheduler = *s.Scheduler
	}
	if s.Worker != nil {
		cfg.Worker = *s.Worker
	}
	cfg.Watches = append(cfg.Watches, s.Watches...)
}

// envFunction returns an HCL function that looks up environment variables
// from env. Calling `env("MISSING")` yields the empty string — matches
// the Unix shell default and lets callers fall back to an HCL literal
// via the standard `coalesce()` pattern.
func envFunction(env map[string]string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{Name: "name", Type: cty.String},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			return cty.StringVal(env[args[0].AsString()]), nil
		},
	})
}

// applyFlagOverrides copies non-empty fields from flags onto cfg. CLI
// values always win over file / env.
func applyFlagOverrides(cfg *Config, flags *FlagOverrides) {
	if flags.LogFormat != "" {
		cfg.Log.Format = flags.LogFormat
	}
	if flags.LogLevel != "" {
		cfg.Log.Level = flags.LogLevel
	}
	if flags.AdminAddr != "" {
		cfg.Admin.Addr = flags.AdminAddr
	}
	if flags.EbayAppID != "" {
		cfg.Ebay.AppID = flags.EbayAppID
	}
	if flags.EbayCertID != "" {
		cfg.Ebay.CertID = flags.EbayCertID
	}
	if flags.PostgresDSN != "" {
		cfg.Postgres.DSN = flags.PostgresDSN
	}
	if flags.ValkeyAddr != "" {
		cfg.Valkey.Addr = flags.ValkeyAddr
	}
	if flags.MeiliURL != "" {
		cfg.Meilisearch.URL = flags.MeiliURL
	}
}

// applyDefaultsAfterMerge fills in any fields the HCL decode left at
// their zero value with the documented defaults. We can't rely on
// Defaults() being the starting point because gohcl decode overwrites
// blocks wholesale when the user declares them, even partially.
func applyDefaultsAfterMerge(cfg *Config) {
	d := Defaults()
	if cfg.Log.Format == "" {
		cfg.Log.Format = d.Log.Format
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = d.Log.Level
	}
	if cfg.Admin.Addr == "" {
		cfg.Admin.Addr = d.Admin.Addr
	}
	if cfg.Ebay.Marketplace == "" {
		cfg.Ebay.Marketplace = d.Ebay.Marketplace
	}
	if cfg.API.Addr == "" {
		cfg.API.Addr = d.API.Addr
	}
	if cfg.API.ReadTimeout == "" {
		cfg.API.ReadTimeout = d.API.ReadTimeout
	}
	if cfg.API.WriteTimeout == "" {
		cfg.API.WriteTimeout = d.API.WriteTimeout
	}
	if cfg.Scheduler.TickInterval == "" {
		cfg.Scheduler.TickInterval = d.Scheduler.TickInterval
	}
	if cfg.Scheduler.BulkReconcileInterval == "" {
		cfg.Scheduler.BulkReconcileInterval = d.Scheduler.BulkReconcileInterval
	}
	if cfg.Scheduler.SyncInterval == "" {
		cfg.Scheduler.SyncInterval = d.Scheduler.SyncInterval
	}
	if cfg.Obs.SpanSampling == 0 {
		cfg.Obs.SpanSampling = d.Obs.SpanSampling
	}
}

// EnvSliceToMap converts os.Environ()-style "KEY=value" entries into a
// map suitable for Load. Exported because the CLI layer calls it
// before passing the env to Load.
func EnvSliceToMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		out[k] = v
	}
	return out
}
