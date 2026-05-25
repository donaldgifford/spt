// Package main is the entry point for the spt CLI.
//
// The binary embeds every role (api, scheduler, worker, migrate) and
// dispatches via cobra subcommands. Build-time identity is injected
// through -ldflags -X main.version / .commit / .date; see the
// `build-core` recipe in justfile.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/donaldgifford/spt/internal/app/cli"
)

// Build-time identity, populated via -ldflags -X. The defaults make
// `go run ./cmd/spt version` produce useful (if generic) output.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := cli.NewRootCmd(cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	})

	if err := root.ExecuteContext(ctx); err != nil {
		// A cancelled context is the normal shutdown path (SIGINT/SIGTERM
		// during a role's Run). Treat it as success — non-zero exit is
		// reserved for actual configuration or runtime failures.
		if errors.Is(err, context.Canceled) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "spt:", err)
		return 1
	}
	return 0
}
