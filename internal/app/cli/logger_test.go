package cli

import (
	"errors"
	"log/slog"
	"os"
	"testing"
)

func TestResolveLogFormatExplicit(t *testing.T) {
	cases := map[string]string{
		"text": logFormatText,
		"json": logFormatJSON,
		"TEXT": logFormatText,
		"JSON": logFormatJSON,
		"Text": logFormatText,
		"jSoN": logFormatJSON,
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			got, err := resolveLogFormat(input, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != want {
				t.Errorf("resolveLogFormat(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestResolveLogFormatAutoOnNonTTY(t *testing.T) {
	// Create a regular (non-TTY) temp file to stand in for stderr.
	f, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer func() { _ = f.Close() }()

	got, err := resolveLogFormat("auto", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != logFormatJSON {
		t.Errorf("auto on non-tty: got %q want %q", got, logFormatJSON)
	}
}

func TestResolveLogFormatEmptyDefaultsToAuto(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer func() { _ = f.Close() }()

	got, err := resolveLogFormat("", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != logFormatJSON {
		t.Errorf(`resolveLogFormat("") = %q, want %q (non-tty default)`, got, logFormatJSON)
	}
}

func TestResolveLogFormatUnknown(t *testing.T) {
	_, err := resolveLogFormat("yaml", nil)
	if !errors.Is(err, ErrUnknownLogFormat) {
		t.Errorf("got %v, want ErrUnknownLogFormat", err)
	}
}

func TestParseLogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"INFO":    slog.LevelInfo,
		"Debug":   slog.LevelDebug,
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			got, err := parseLogLevel(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", input, got, want)
			}
		})
	}
}

func TestParseLogLevelUnknown(t *testing.T) {
	_, err := parseLogLevel("trace")
	if !errors.Is(err, ErrUnknownLogLevel) {
		t.Errorf("got %v, want ErrUnknownLogLevel", err)
	}
}

func TestInstallSlogValid(t *testing.T) {
	if err := installSlog("json", "info"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallSlogInvalidFormat(t *testing.T) {
	err := installSlog("yaml", "info")
	if !errors.Is(err, ErrUnknownLogFormat) {
		t.Errorf("got %v, want ErrUnknownLogFormat", err)
	}
}

func TestInstallSlogInvalidLevel(t *testing.T) {
	err := installSlog("json", "trace")
	if !errors.Is(err, ErrUnknownLogLevel) {
		t.Errorf("got %v, want ErrUnknownLogLevel", err)
	}
}

func TestIsTerminalOnRegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "regular")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer func() { _ = f.Close() }()

	if isTerminal(f) {
		t.Error("isTerminal on regular file: got true, want false")
	}
}

func TestIsTerminalNilFile(t *testing.T) {
	if isTerminal(nil) {
		t.Error("isTerminal(nil): got true, want false")
	}
}
