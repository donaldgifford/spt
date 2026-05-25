package obs

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/term"
)

// ErrUnknownLogFormat is returned when format is not "auto", "text", or "json".
var ErrUnknownLogFormat = errors.New("obs: unknown log format")

// ErrUnknownLogLevel is returned when level is not one of the documented levels.
var ErrUnknownLogLevel = errors.New("obs: unknown log level")

const (
	logFormatAuto = "auto"
	logFormatText = "text"
	logFormatJSON = "json"
)

// NewLogger returns a *slog.Logger configured for the given format and
// level, writing to w. format "auto" picks JSON when w is not a TTY, text
// otherwise — but only when w is an *os.File (the TTY check needs an fd).
// For non-file writers, "auto" resolves to JSON.
func NewLogger(w io.Writer, format, level string) (*slog.Logger, error) {
	resolved, err := resolveFormat(format, w)
	if err != nil {
		return nil, err
	}
	lvl, err := ParseLevel(level)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{Level: lvl}
	switch resolved {
	case logFormatJSON:
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	default:
		return slog.New(slog.NewTextHandler(w, opts)), nil
	}
}

// ParseLevel maps the documented level strings onto slog.Level. An
// empty string resolves to slog.LevelInfo.
func ParseLevel(level string) (slog.Level, error) {
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

// resolveFormat translates a user-supplied format ("", "auto", "text",
// "json") into the concrete handler choice. For non-file writers,
// "auto" resolves to JSON since there is no TTY to detect.
func resolveFormat(format string, w io.Writer) (string, error) {
	switch strings.ToLower(format) {
	case "", logFormatAuto:
		if f, ok := w.(*os.File); ok && isTerminal(f) {
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

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	// File descriptors are small non-negative integers on every supported
	// platform, so the uintptr→int conversion cannot overflow.
	return term.IsTerminal(int(f.Fd())) //nolint:gosec // fd fits in int
}
