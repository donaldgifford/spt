package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
		fmt.Fprintf(os.Stderr, "spt-mock-server: %v\n", err)
		return 1
	}
	return 0
}

type serveFlags struct {
	port         int
	scenario     string
	logFormat    string
	logLevel     string
	fixturesDir  string
	tokenExpires time.Duration
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "spt-mock-server",
		Short:         "In-memory eBay-shaped HTTP mock for tests and local dev",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newServeCmd())
	return root
}

func newServeCmd() *cobra.Command {
	f := &serveFlags{}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the mock eBay server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger, err := newLogger(os.Stderr, f.logFormat, f.logLevel)
			if err != nil {
				return err
			}

			opts := ServerOptions{
				Logger:       logger,
				Scenario:     f.scenario,
				FixturesDir:  f.fixturesDir,
				TokenExpires: f.tokenExpires,
			}
			srv, err := NewServer(opts)
			if err != nil {
				return err
			}

			addr := fmt.Sprintf(":%d", f.port)
			httpSrv := &http.Server{
				Addr:              addr,
				Handler:           srv.Routes(),
				ReadHeaderTimeout: 5 * time.Second,
			}

			errCh := make(chan error, 1)
			go func() {
				logger.Info("mock-server listening",
					"addr", addr, "scenario", srv.ActiveScenario())
				errCh <- httpSrv.ListenAndServe()
			}()

			select {
			case <-cmd.Context().Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := httpSrv.Shutdown(shutdownCtx); err != nil {
					return fmt.Errorf("shutdown: %w", err)
				}
				return nil
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return fmt.Errorf("listen: %w", err)
			}
		},
	}

	pf := cmd.Flags()
	pf.IntVar(&f.port, "port", 8080, "TCP port to bind on.")
	pf.StringVar(&f.scenario, "scenario", "default", "Active scenario name (subdirectory of fixtures/).")
	pf.StringVar(&f.logFormat, "log-format", "auto", `Log output format ("text", "json", or "auto").`)
	pf.StringVar(&f.logLevel, "log-level", "info", `Log level ("debug", "info", "warn", "error").`)
	pf.StringVar(&f.fixturesDir, "fixtures-dir", "",
		"Override the embedded fixtures with a filesystem directory (for local iteration).")
	pf.DurationVar(&f.tokenExpires, "token-expires", 2*time.Hour, "OAuth token TTL reported to clients.")

	return cmd
}
