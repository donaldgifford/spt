package obs

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	l, err := NewLogger(&buf, "json", "info")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Info("hello", "k", "v")
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Errorf("expected JSON line, got: %s", buf.String())
	}
}

func TestNewLoggerText(t *testing.T) {
	var buf bytes.Buffer
	l, err := NewLogger(&buf, "text", "info")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Info("hello", "k", "v")
	if !strings.Contains(buf.String(), "msg=hello") {
		t.Errorf("expected text line, got: %s", buf.String())
	}
}

func TestNewLoggerAutoOnBuffer(t *testing.T) {
	var buf bytes.Buffer
	l, err := NewLogger(&buf, "auto", "info")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Info("hello")
	// Non-file writer → JSON.
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Errorf(`"auto" on non-tty writer should produce JSON, got: %s`, buf.String())
	}
}

func TestNewLoggerUnknownFormat(t *testing.T) {
	_, err := NewLogger(&bytes.Buffer{}, "yaml", "info")
	if !errors.Is(err, ErrUnknownLogFormat) {
		t.Errorf("got %v, want ErrUnknownLogFormat", err)
	}
}

func TestNewLoggerUnknownLevel(t *testing.T) {
	_, err := NewLogger(&bytes.Buffer{}, "json", "trace")
	if !errors.Is(err, ErrUnknownLogLevel) {
		t.Errorf("got %v, want ErrUnknownLogLevel", err)
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"debug":   slog.LevelDebug,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"INFO":    slog.LevelInfo,
	}
	for in, want := range cases {
		got, err := ParseLevel(in)
		if err != nil {
			t.Errorf("ParseLevel(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}
