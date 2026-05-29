package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/term"
)

const (
	logFormatText = "text"
	logFormatJSON = "json"
	logFormatAuto = "auto"
)

// errUnknownLogFormat is returned when --log-format is not text/json/auto.
var errUnknownLogFormat = errors.New("mock-server: unknown log format")

// newLogger mirrors internal/obs/slog.go behavior so the tool's log
// output looks like the main binary's: TTY auto-detects to text, pipes
// default to JSON.
func newLogger(w io.Writer, format, level string) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	hopts := &slog.HandlerOptions{Level: lvl}

	switch resolveFormat(w, format) {
	case logFormatText:
		return slog.New(slog.NewTextHandler(w, hopts)), nil
	case logFormatJSON:
		return slog.New(slog.NewJSONHandler(w, hopts)), nil
	default:
		return nil, fmt.Errorf("%w: %q", errUnknownLogFormat, format)
	}
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("mock-server: unknown log level %q", s)
	}
}

func resolveFormat(w io.Writer, format string) string {
	switch strings.ToLower(format) {
	case logFormatText:
		return logFormatText
	case logFormatJSON:
		return logFormatJSON
	case "", logFormatAuto:
		if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) { //nolint:gosec // fd fits in int
			return logFormatText
		}
		return logFormatJSON
	default:
		return format
	}
}
