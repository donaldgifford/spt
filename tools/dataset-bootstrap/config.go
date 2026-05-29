package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// StratificationConfig drives Sampler. Bucket keys are the canonical
// names from DESIGN-0002's Confidence contract:
//   - "<0.5"     — needs-review band
//   - "0.5-0.8"  — mid-confidence
//   - "0.8-1.0"  — high-confidence
type StratificationConfig struct {
	SinceDuration       time.Duration
	PerKind             int
	PerConfidenceBucket map[string]int
	TotalCap            int
	Seed                int64
	OutputPath          string
}

// String returns a short one-line summary used by error messages.
func (c StratificationConfig) String() string {
	return fmt.Sprintf("since=%s per_kind=%d total_cap=%d seed=%d out=%s buckets=%v",
		c.SinceDuration, c.PerKind, c.TotalCap, c.Seed, c.OutputPath, c.PerConfidenceBucket)
}

// ConfidenceBucketFor returns the canonical bucket name for a 0.0-1.0
// confidence value.
func ConfidenceBucketFor(conf float64) string {
	switch {
	case conf < 0.5:
		return "<0.5"
	case conf < 0.8:
		return "0.5-0.8"
	default:
		return "0.8-1.0"
	}
}

// parseBuckets accepts the --per-confidence-bucket flag value and
// returns a map keyed by bucket name. Empty input is an error so the
// operator sees the mistake early.
func parseBuckets(spec string) (map[string]int, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, errors.New("--per-confidence-bucket must be non-empty")
	}
	out := make(map[string]int)
	for pair := range strings.SplitSeq(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		idx := strings.LastIndex(pair, ":")
		if idx <= 0 {
			return nil, fmt.Errorf("invalid bucket spec %q (want \"name:N\")", pair)
		}
		name := strings.TrimSpace(pair[:idx])
		nStr := strings.TrimSpace(pair[idx+1:])
		n, err := strconv.Atoi(nStr)
		if err != nil {
			return nil, fmt.Errorf("invalid bucket count %q: %w", nStr, err)
		}
		if n < 0 {
			return nil, fmt.Errorf("bucket %q count must be non-negative", name)
		}
		out[name] = n
	}
	return out, nil
}
