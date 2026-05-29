package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Mode selects whether main writes files or validates them.
type Mode int

const (
	// ModeWrite overwrites on-disk dashboards + rules.
	ModeWrite Mode = iota
	// ModeValidate regenerates to memory and diffs against on-disk
	// files, returning ErrDriftDetected on any mismatch.
	ModeValidate
)

// ErrDriftDetected indicates -validate found at least one file whose
// on-disk contents differ from the regenerated form.
var ErrDriftDetected = errors.New("dashgen: drift detected (run dashgen without -validate to regenerate)")

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
		fmt.Fprintf(os.Stderr, "spt-dashgen: %v\n", err)
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	var validate bool
	cmd := &cobra.Command{
		Use:   "spt-dashgen <out-dir>",
		Short: "Generate or validate Grafana dashboards + Prometheus rules",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := ModeWrite
			if validate {
				mode = ModeValidate
			}
			return Generate(cmd.OutOrStdout(), args[0], mode)
		},
	}
	cmd.Flags().BoolVar(&validate, "validate", false,
		"Diff regenerated output against on-disk files; exit non-zero on drift.")
	return cmd
}

// Generate walks DashboardSpecs() + RuleFiles() and either writes
// each file atomically or validates it against on-disk contents.
func Generate(w io.Writer, outDir string, mode Mode) error {
	var drifted []string

	for _, spec := range DashboardSpecs() {
		want, err := marshalDashboard(spec.Build())
		if err != nil {
			return err
		}
		target := filepath.Join(outDir, spec.File)
		if err := writeOrCompare(target, want, mode, &drifted); err != nil {
			return err
		}
	}

	for _, rf := range RuleFiles() {
		want, err := marshalRules(rf.Groups)
		if err != nil {
			return err
		}
		target := filepath.Join(outDir, rf.File)
		if err := writeOrCompare(target, want, mode, &drifted); err != nil {
			return err
		}
	}

	if mode == ModeValidate && len(drifted) > 0 {
		_, _ = fmt.Fprintln(w, "drift in:") //nolint:errcheck // best-effort report
		for _, p := range drifted {
			_, _ = fmt.Fprintln(w, "  - "+p) //nolint:errcheck // best-effort report
		}
		return ErrDriftDetected
	}
	return nil
}

func writeOrCompare(target string, want []byte, mode Mode, drifted *[]string) error {
	switch mode {
	case ModeWrite:
		return writeAtomic(target, want)
	case ModeValidate:
		got, err := os.ReadFile(target)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				*drifted = append(*drifted, target+" (missing)")
				return nil
			}
			return fmt.Errorf("dashgen: read %s: %w", target, err)
		}
		if !bytes.Equal(got, want) {
			*drifted = append(*drifted, target)
		}
		return nil
	default:
		return fmt.Errorf("dashgen: unknown mode %d", mode)
	}
}

func marshalDashboard(d any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return nil, fmt.Errorf("dashgen: marshal dashboard: %w", err)
	}
	return buf.Bytes(), nil
}

func marshalRules(groups []RuleGroupSpec) ([]byte, error) {
	wrapper := struct {
		Groups []RuleGroupSpec `yaml:"groups"`
	}{Groups: groups}
	out, err := yaml.Marshal(wrapper)
	if err != nil {
		return nil, fmt.Errorf("dashgen: marshal rules: %w", err)
	}
	return out, nil
}

// writeAtomic writes data to target via a .tmp + os.Rename for
// atomicity. mkdir parents as needed.
func writeAtomic(target string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("dashgen: mkdir %s: %w", filepath.Dir(target), err)
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("dashgen: write tmp: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("dashgen: rename %s -> %s: %w", tmp, target, err)
	}
	return nil
}
