//go:build integration

package datastore

import (
	"io"
	"log/slog"
)

// nopLogger returns a slog.Logger that discards every record.
// Used to keep integration-test output uncluttered.
func nopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
