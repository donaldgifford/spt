package config

import (
	"fmt"
	"time"
)

// ParsedTickInterval returns SchedulerConfig.TickInterval as a parsed
// time.Duration. An empty string yields a zero duration with no error so
// callers can apply their own fallback.
func (s SchedulerConfig) ParsedTickInterval() (time.Duration, error) {
	return parseOptionalDuration("scheduler.tick_interval", s.TickInterval)
}

// ParsedBulkReconcileInterval mirrors ParsedTickInterval for the bulk
// reconcile cron.
func (s SchedulerConfig) ParsedBulkReconcileInterval() (time.Duration, error) {
	return parseOptionalDuration("scheduler.bulk_reconcile_interval", s.BulkReconcileInterval)
}

// ParsedSyncInterval mirrors ParsedTickInterval for the eBay sync cron.
func (s SchedulerConfig) ParsedSyncInterval() (time.Duration, error) {
	return parseOptionalDuration("scheduler.sync_interval", s.SyncInterval)
}

// ParsedReadTimeout returns APIConfig.ReadTimeout as a parsed duration.
func (a APIConfig) ParsedReadTimeout() (time.Duration, error) {
	return parseOptionalDuration("api.read_timeout", a.ReadTimeout)
}

// ParsedWriteTimeout returns APIConfig.WriteTimeout as a parsed duration.
func (a APIConfig) ParsedWriteTimeout() (time.Duration, error) {
	return parseOptionalDuration("api.write_timeout", a.WriteTimeout)
}

// ParsedCadence returns WatchConfig.Cadence as a parsed duration.
func (w *WatchConfig) ParsedCadence() (time.Duration, error) {
	return parseOptionalDuration(fmt.Sprintf("watch[%q].cadence", w.Name), w.Cadence)
}

// parseOptionalDuration parses raw as a time.Duration. An empty string
// returns (0, nil) so the caller can apply a default; a non-empty
// invalid value returns a wrapped ErrInvalidDuration that includes the
// field path for clear diagnostics.
func parseOptionalDuration(field, raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %s = %q: %w", ErrInvalidDuration, field, raw, err)
	}
	return d, nil
}
