package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
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
		fmt.Fprintf(os.Stderr, "spt-regression-runner: %v\n", err)
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "spt-regression-runner",
		Short:         "Run a regression dataset against one or more model backends",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newRunCmd())
	return root
}

type runFlags struct {
	backends string
	dataset  string
	format   string
	out      string
	langfuse bool
}

func newRunCmd() *cobra.Command {
	f := &runFlags{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the regression suite",
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := LoadDataset(f.dataset)
			if err != nil {
				return err
			}
			backends, err := resolveBackends(f.backends)
			if err != nil {
				return err
			}
			report := executeAll(cmd.Context(), f.dataset, entries, backends)

			sink := cmd.OutOrStdout()
			if f.out != "" {
				file, err := os.Create(f.out)
				if err != nil {
					return fmt.Errorf("regression-runner: create %s: %w", f.out, err)
				}
				defer func() { _ = file.Close() }() //nolint:errcheck // best-effort close
				sink = file
			}
			return WriteReport(sink, report, Format(f.format))
		},
	}
	pf := cmd.Flags()
	pf.StringVar(&f.backends, "backend", "ollama",
		"Comma-separated list (ollama,anthropic,openai).")
	pf.StringVar(&f.dataset, "dataset", "tools/regression-runner/testdata/baseline",
		"Local directory of *.json entries or langfuse://<id>.")
	pf.StringVar(&f.format, "format", string(FormatText),
		`Report format ("text" or "json").`)
	pf.StringVar(&f.out, "out", "", "Output path (stdout if empty).")
	pf.BoolVar(&f.langfuse, "langfuse", false,
		"Also log per-Result traces to Langfuse (TODO: wire to dataset-upload.Client).")
	return cmd
}

func resolveBackends(spec string) ([]Backend, error) {
	out := make([]Backend, 0)
	for name := range strings.SplitSeq(spec, ",") {
		name = strings.TrimSpace(name)
		switch name {
		case "":
			continue
		case "ollama":
			out = append(out, OllamaBackend{})
		case "anthropic":
			out = append(out, AnthropicBackend{})
		case "openai":
			out = append(out, OpenAIBackend{})
		default:
			return nil, fmt.Errorf("regression-runner: unknown backend %q", name)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("regression-runner: --backend list resolved to zero backends")
	}
	return out, nil
}

func executeAll(
	ctx context.Context, datasetName string, entries []DatasetEntry, backends []Backend,
) Report {
	report := Report{
		GeneratedAt: time.Now().UTC(),
		Dataset:     datasetName,
		Backends:    make([]BackendReport, 0, len(backends)),
	}
	for _, b := range backends {
		results := executeBackend(ctx, b, entries)
		report.Backends = append(report.Backends, Aggregate(b.Name(), results))
	}
	return report
}

func executeBackend(ctx context.Context, b Backend, entries []DatasetEntry) []Result {
	out := make([]Result, 0, len(entries))
	for _, entry := range entries {
		start := time.Now()
		got, err := b.Extract(ctx, entry.Listing)
		latency := time.Since(start)
		if err != nil {
			// Backend errors land as NoMatch in the report — the
			// operator sees an accuracy drop and can investigate.
			out = append(out, Result{
				Backend: b.Name(), ListingID: entry.Listing.ID,
				Outcome: NoMatch, Latency: latency, Expected: entry.Expected,
			})
			continue
		}
		out = append(out, Result{
			Backend: b.Name(), ListingID: entry.Listing.ID,
			Outcome: MatchComponents(got, entry.Expected),
			Latency: latency, Got: got, Expected: entry.Expected,
		})
	}
	return out
}
