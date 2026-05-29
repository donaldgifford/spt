package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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
		fmt.Fprintf(os.Stderr, "spt-dataset-upload: %v\n", err)
		return 1
	}
	return 0
}

type uploadFlags struct {
	datasetID string
	input     string
	dryRun    bool
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "spt-dataset-upload",
		Short:         "Upload regression JSON to Langfuse as a DatasetItem set",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newUploadCmd())
	return root
}

func newUploadCmd() *cobra.Command {
	f := &uploadFlags{}
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload the regression dataset to Langfuse",
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

			rawJSON, err := os.ReadFile(f.input)
			if err != nil {
				return fmt.Errorf("dataset-upload: read input: %w", err)
			}
			items, err := ParseItems(rawJSON)
			if err != nil {
				return err
			}

			var client Client
			if f.dryRun {
				client = dryRunSink{}
			} else {
				client, err = NewHTTPClient(ClientOptions{
					Host:      os.Getenv("LANGFUSE_HOST"),
					PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
					SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
				})
				if err != nil {
					return err
				}
			}

			u := &Uploader{
				Client:    client,
				DatasetID: f.datasetID,
				DryRun:    f.dryRun,
				Logger:    logger,
			}
			return u.Upsert(cmd.Context(), items)
		},
	}

	pf := cmd.Flags()
	pf.StringVar(&f.datasetID, "dataset-id", "", "Langfuse dataset ID to upload into (required).")
	pf.StringVar(&f.input, "input", "", "Path to regression JSON from spt-dataset-bootstrap (required).")
	pf.BoolVar(&f.dryRun, "dry-run", false, "Print planned actions, perform no HTTP calls.")

	_ = cmd.MarkFlagRequired("dataset-id")
	_ = cmd.MarkFlagRequired("input")

	return cmd
}

// dryRunSink satisfies Client without making any network calls. The
// Uploader.DryRun path is the real "no I/O" guard; dryRunSink is
// belt-and-suspenders.
type dryRunSink struct{}

func (dryRunSink) UpsertDatasetItem(_ context.Context, _, _ string, _ []byte) error {
	return nil
}

// discardLogger is shared by tests that don't want stderr noise.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
