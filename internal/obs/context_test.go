package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestLoggerFromContextWithoutSpan(t *testing.T) {
	var buf bytes.Buffer
	l, err := NewLogger(&buf, "json", "info")
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	ctx := WithLogger(context.Background(), l)
	LoggerFromContext(ctx).Info("hello")

	if strings.Contains(buf.String(), "trace_id") {
		t.Errorf("trace_id should be absent without a span; got: %s", buf.String())
	}
}

func TestLoggerFromContextWithSpan(t *testing.T) {
	var buf bytes.Buffer
	l, err := NewLogger(&buf, "json", "info")
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	ctx = WithLogger(ctx, l)
	LoggerFromContext(ctx).Info("hello")

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("decode log: %v\nraw: %s", err, buf.String())
	}

	if _, ok := entry["trace_id"]; !ok {
		t.Errorf("trace_id missing from log entry: %v", entry)
	}
	if _, ok := entry["span_id"]; !ok {
		t.Errorf("span_id missing from log entry: %v", entry)
	}
}

func TestLoggerFromContextDefaultWhenEmpty(t *testing.T) {
	l := LoggerFromContext(context.Background())
	if l == nil {
		t.Fatal("LoggerFromContext on empty ctx should fall back to slog.Default(), got nil")
	}
}
