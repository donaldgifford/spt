package main

import (
	"context"
	"encoding/json"
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
		fmt.Fprintf(os.Stderr, "spt-judge-bootstrap: %v\n", err)
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "spt-judge-bootstrap",
		Short:         "Surface and apply few-shot examples for the LLM-as-judge layer",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newListCmd(), newApplyCmd())
	return root
}

type listFlags struct {
	since      time.Duration
	candidates int
	strategy   string
	out        string
}

func newListCmd() *cobra.Command {
	f := &listFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Surface candidate few-shot examples to a JSON file for operator review",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := StrategyByName(f.strategy)
			if err != nil {
				return err
			}
			return errors.New("judge-bootstrap: list requires a configured Datastore " +
				"(Postgres impl lands in the datastore IMPL); " +
				"until then, exercise the strategy via the unit tests against a fake Reader")
		},
	}
	pf := cmd.Flags()
	pf.DurationVar(&f.since, "since", 30*24*time.Hour, "Lookback window.")
	pf.IntVar(&f.candidates, "candidates", 50, "Maximum candidates surfaced.")
	pf.StringVar(&f.strategy, "strategy", "ambiguous",
		"Surface strategy (ambiguous, low-confidence, high-stakes, disagreement).")
	pf.StringVar(&f.out, "out", "candidates.json", "Output JSON path.")
	return cmd
}

type applyFlags struct {
	input  string
	output string
}

func newApplyCmd() *cobra.Command {
	f := &applyFlags{}
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Write accepted candidates to internal/agent/judge/examples.json",
		RunE: func(_ *cobra.Command, _ []string) error {
			raw, err := os.ReadFile(f.input)
			if err != nil {
				return fmt.Errorf("judge-bootstrap: read input: %w", err)
			}
			var candidates []Candidate
			if err := json.Unmarshal(raw, &candidates); err != nil {
				return fmt.Errorf("judge-bootstrap: parse input: %w", err)
			}
			accepted, err := FilterAccepted(candidates)
			if err != nil {
				return err
			}
			out, err := json.MarshalIndent(accepted, "", "  ")
			if err != nil {
				return fmt.Errorf("judge-bootstrap: marshal output: %w", err)
			}
			if err := os.WriteFile(f.output, out, 0o600); err != nil {
				return fmt.Errorf("judge-bootstrap: write output: %w", err)
			}
			return nil
		},
	}
	pf := cmd.Flags()
	pf.StringVar(&f.input, "input", "",
		"Path to operator-reviewed candidates JSON (required).")
	pf.StringVar(&f.output, "output", "internal/agent/judge/examples.json",
		"Path to write the few-shots file.")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

// FilterAccepted returns only Candidates marked Accepted:true.
// Validates each accepted candidate has non-empty Notes; if any are
// missing, returns an error listing the offending ScoreIDs.
func FilterAccepted(candidates []Candidate) ([]Candidate, error) {
	var missing []string
	out := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if !c.Accepted {
			continue
		}
		if c.Notes == "" {
			missing = append(missing, string(c.ScoreID))
			continue
		}
		out = append(out, c)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"judge-bootstrap: accepted candidates missing Notes: %v", missing)
	}
	return out, nil
}
