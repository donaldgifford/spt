package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/term"
)

// ErrUnknownLogFormat is returned when --log-format receives a value other
// than "auto", "text", or "json".
var ErrUnknownLogFormat = errors.New("cli: unknown log format")

// ErrUnknownLogLevel is returned when --log-level receives a value other
// than "debug", "info", "warn", or "error".
var ErrUnknownLogLevel = errors.New("cli: unknown log level")

// installSlog configures the package-level slog logger for the running
// role based on the --log-format and --log-level flag values.
//
// Format "auto" (the default) resolves to "text" when stderr is a TTY and
// "json" otherwise. This is a Phase 2 inline implementation; Phase 4
// (IMPL-0001) extracts it to internal/obs alongside OTel + Prometheus.
func installSlog(format, level string) error {
	resolved, err := resolveLogFormat(format, os.Stderr)
	if err != nil {
		return err
	}

	lvl, err := parseLogLevel(level)
	if err != nil {
		return err
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	switch resolved {
	case logFormatJSON:
		handler = slog.NewJSONHandler(os.Stderr, opts)
	case logFormatText:
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
	return nil
}

const (
	logFormatAuto = "auto"
	logFormatText = "text"
	logFormatJSON = "json"
)

// resolveLogFormat resolves a user-supplied format value into the concrete
// format that handlers consume. The TTY-detection branch is the reason
// "auto" exists — see IMPL-0001 Resolved Decision #2.
func resolveLogFormat(format string, stderr *os.File) (string, error) {
	switch strings.ToLower(format) {
	case "", logFormatAuto:
		if isTerminal(stderr) {
			return logFormatText, nil
		}
		return logFormatJSON, nil
	case logFormatText:
		return logFormatText, nil
	case logFormatJSON:
		return logFormatJSON, nil
	default:
		return "", fmt.Errorf("%w: %q (want one of auto, text, json)", ErrUnknownLogFormat, format)
	}
}

// isTerminal checks whether the given file descriptor is a terminal.
// Extracted so tests can substitute a non-tty file.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	// File descriptors are small non-negative integers on every supported
	// platform, so the uintptr→int conversion cannot overflow.
	return term.IsTerminal(int(f.Fd())) //nolint:gosec // fd fits in int
}

func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%w: %q (want one of debug, info, warn, error)", ErrUnknownLogLevel, level)
	}
}
