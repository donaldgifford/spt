package config

import (
	"fmt"
	"strings"
)

// Validate inspects cfg for required-but-missing fields, out-of-range
// numerics, and malformed duration strings. All problems are aggregated
// into a single *ValidationError so the user sees every issue at once.
//
// Required fields are NOT enforced in Phase 3 for production-only
// dependencies (Postgres DSN, eBay credentials, Valkey, Meilisearch)
// because the role stubs from Phase 2 don't yet open those connections.
// Phase 4+ wires the real handlers and the role-aware required-field
// matrix in [internal/config/README.md]. Phase 3 catches only:
//
//   - Malformed log_format / log_level values.
//   - Malformed Go duration strings anywhere they appear.
//   - obs.span_sampling outside [0, 1].
//   - watch blocks with no query.
//   - worker pools with non-positive concurrency.
//   - judge_sample_rate outside [0, 1].
//
// Each problem is recorded with its dotted field path so the user can
// jump straight to it in their HCL.
func Validate(cfg *Config) error {
	v := &ValidationError{}

	validateLog(v, cfg.Log)
	validateObs(v, cfg.Obs)
	validateAPI(v, cfg.API)
	validateScheduler(v, cfg.Scheduler)
	validateWorker(v, cfg.Worker)
	validateWatches(v, cfg.Watches)

	return v.asError()
}

// validLogFormats and validLogLevels are the accepted values for the
// log block. Listed as constants so the case-statement, default
// message, and types.go doc comment all read from one source.
var (
	validLogFormats = []string{"auto", "text", "json"}
	validLogLevels  = []string{"debug", "info", "warn", "warning", "error"}
)

func validateLog(v *ValidationError, log LogConfig) {
	if log.Format != "" && !containsFold(validLogFormats, log.Format) {
		v.add("log.format", fmt.Errorf("%w: %q (want one of %s)",
			ErrOutOfRange, log.Format, strings.Join(validLogFormats, ", ")))
	}
	if log.Level != "" && !containsFold(validLogLevels, log.Level) {
		v.add("log.level", fmt.Errorf("%w: %q (want one of %s)",
			ErrOutOfRange, log.Level, strings.Join(validLogLevels, ", ")))
	}
}

func containsFold(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

func validateObs(v *ValidationError, obs ObsConfig) {
	if obs.SpanSampling < 0 || obs.SpanSampling > 1 {
		v.add("obs.span_sampling", fmt.Errorf("%w: %v (want [0, 1])", ErrOutOfRange, obs.SpanSampling))
	}
}

func validateAPI(v *ValidationError, api APIConfig) {
	if _, err := api.ParsedReadTimeout(); err != nil {
		v.add("api.read_timeout", err)
	}
	if _, err := api.ParsedWriteTimeout(); err != nil {
		v.add("api.write_timeout", err)
	}
}

func validateScheduler(v *ValidationError, sch SchedulerConfig) {
	if _, err := sch.ParsedTickInterval(); err != nil {
		v.add("scheduler.tick_interval", err)
	}
	if _, err := sch.ParsedBulkReconcileInterval(); err != nil {
		v.add("scheduler.bulk_reconcile_interval", err)
	}
	if _, err := sch.ParsedSyncInterval(); err != nil {
		v.add("scheduler.sync_interval", err)
	}
}

func validateWorker(v *ValidationError, w WorkerConfig) {
	for _, p := range w.Pools {
		if p.Concurrency <= 0 {
			v.add(
				fmt.Sprintf("worker.pools[%q].concurrency", p.Stage),
				fmt.Errorf("%w: %d (want > 0)", ErrOutOfRange, p.Concurrency),
			)
		}
	}
}

func validateWatches(v *ValidationError, watches []WatchConfig) {
	for _, w := range watches {
		path := fmt.Sprintf("watch[%q]", w.Name)
		v.addRequired(path+".query", w.Query)
		if w.JudgeSampleRate < 0 || w.JudgeSampleRate > 1 {
			v.add(path+".judge_sample_rate", fmt.Errorf("%w: %v (want [0, 1])", ErrOutOfRange, w.JudgeSampleRate))
		}
		if _, err := w.ParsedCadence(); err != nil {
			v.add(path+".cadence", err)
		}
	}
}
