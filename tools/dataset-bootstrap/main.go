package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "spt-dataset-bootstrap: %v\n", err)
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "spt-dataset-bootstrap",
		Short:         "Pull a stratified regression dataset from Postgres",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newSampleCmd())
	return root
}

type sampleFlags struct {
	since     time.Duration
	perKind   int
	perBucket string
	totalCap  int
	seed      int64
	out       string
}

func newSampleCmd() *cobra.Command {
	f := &sampleFlags{}
	cmd := &cobra.Command{
		Use:   "sample",
		Short: "Sample a stratified set of recent listings into a regression JSON file",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := buildConfig(f)
			if err != nil {
				return err
			}
			return errors.New("dataset-bootstrap: sample run requires a configured Datastore — " +
				"the Postgres impl is in the datastore IMPL. " +
				"Until then, exercise Sampler via the unit tests against a fake Datastore. " +
				"Config parsed OK: " + cfg.String())
		},
	}
	pf := cmd.Flags()
	pf.DurationVar(&f.since, "since", 30*24*time.Hour, "Lookback window for candidate listings.")
	pf.IntVar(&f.perKind, "per-kind", 10, "Sample N listings per ComponentKind.")
	pf.StringVar(&f.perBucket, "per-confidence-bucket", "<0.5:5,0.5-0.8:10,0.8-1.0:10",
		"Comma-separated bucket:N pairs.")
	pf.IntVar(&f.totalCap, "total-cap", 200, "Hard cap on the total sample size.")
	pf.Int64Var(&f.seed, "seed", 42, "RNG seed for deterministic reproducibility.")
	pf.StringVar(&f.out, "out", "",
		"Output filename; defaults to regression-<UTC-timestamp>.json.")
	return cmd
}

func buildConfig(f *sampleFlags) (StratificationConfig, error) {
	buckets, err := parseBuckets(f.perBucket)
	if err != nil {
		return StratificationConfig{}, fmt.Errorf("dataset-bootstrap: %w", err)
	}
	out := f.out
	if out == "" {
		out = fmt.Sprintf("regression-%s.json", time.Now().UTC().Format("20060102T150405Z"))
	}
	return StratificationConfig{
		SinceDuration:       f.since,
		PerKind:             f.perKind,
		PerConfidenceBucket: buckets,
		TotalCap:            f.totalCap,
		Seed:                f.seed,
		OutputPath:          out,
	}, nil
}
