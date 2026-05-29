package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
	"time"
)

// Format selects the report renderer.
type Format string

// Output formats per IMPL-0002 Phase 6.
const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Aggregate folds a backend's Results into a BackendReport. Per-Kind
// accuracy is keyed by the expected Component's first Kind so the
// report surfaces "we got CPU right but missed RAM" rather than a
// single overall number.
func Aggregate(name string, results []Result) BackendReport {
	r := BackendReport{
		Name:            name,
		PerKindAccuracy: make(map[string]float64),
		Results:         results,
	}
	if len(results) == 0 {
		return r
	}

	var totalCorrect int
	perKind := make(map[string]struct{ correct, total int })
	latencies := make([]time.Duration, 0, len(results))

	for _, res := range results {
		latencies = append(latencies, res.Latency)
		kind := "<no-kind>"
		if len(res.Expected) > 0 {
			kind = res.Expected[0].Kind
		}
		k := perKind[kind]
		k.total++
		switch res.Outcome {
		case ExactMatch:
			r.Counts.ExactMatch++
			totalCorrect++
			k.correct++
		case PartialMatch:
			r.Counts.PartialMatch++
		case NoMatch:
			r.Counts.NoMatch++
		}
		perKind[kind] = k
	}

	r.Accuracy = float64(totalCorrect) / float64(len(results))
	for kind, v := range perKind {
		if v.total == 0 {
			continue
		}
		r.PerKindAccuracy[kind] = float64(v.correct) / float64(v.total)
	}
	r.LatencyP50 = percentile(latencies, 50)
	r.LatencyP95 = percentile(latencies, 95)
	return r
}

// percentile uses the nearest-rank method — sufficient for the small
// (~50-item baseline) datasets this tool runs against. No extra
// dependencies needed.
func percentile(in []time.Duration, p int) time.Duration {
	if len(in) == 0 {
		return 0
	}
	dup := make([]time.Duration, len(in))
	copy(dup, in)
	sort.Slice(dup, func(i, j int) bool { return dup[i] < dup[j] })
	idx := (p * len(dup)) / 100
	if idx >= len(dup) {
		idx = len(dup) - 1
	}
	return dup[idx]
}

// WriteReport serializes report to w in the requested Format.
func WriteReport(w io.Writer, report Report, format Format) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return fmt.Errorf("regression-runner: encode JSON: %w", err)
		}
		return nil
	case FormatText:
		return writeTextReport(w, report)
	default:
		return fmt.Errorf("regression-runner: unknown format %q", format)
	}
}

func writeTextReport(w io.Writer, report Report) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }() //nolint:errcheck // best-effort flush

	if _, err := fmt.Fprintf(tw, "Regression report — dataset=%s — generated=%s\n\n",
		report.Dataset, report.GeneratedAt.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("regression-runner: write text header: %w", err)
	}
	if _, err := fmt.Fprintln(tw,
		"BACKEND\tACCURACY\tEXACT\tPARTIAL\tNONE\tP50\tP95"); err != nil {
		return fmt.Errorf("regression-runner: write text columns: %w", err)
	}

	for _, b := range report.Backends {
		if _, err := fmt.Fprintf(tw, "%s\t%.2f\t%d\t%d\t%d\t%s\t%s\n",
			b.Name, b.Accuracy, b.Counts.ExactMatch, b.Counts.PartialMatch, b.Counts.NoMatch,
			b.LatencyP50, b.LatencyP95); err != nil {
			return fmt.Errorf("regression-runner: write text row: %w", err)
		}
	}
	return nil
}
