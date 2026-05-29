package obs

import (
	"errors"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/donaldgifford/spt/internal/config"
)

func TestNewTracerProviderNoExporters(t *testing.T) {
	// With both OTLPEndpoint and LangfuseExporter empty, NewTracerProvider
	// returns a TracerProvider with just the resource + sampler — useful
	// for unit tests of code under test that need a working trace tree
	// but no real exporter.
	tp, shutdown, err := NewTracerProvider(t.Context(), TracerOptions{
		ServiceName: "spt-test",
		Sampling:    1.0,
	})
	if err != nil {
		t.Fatalf("NewTracerProvider: %v", err)
	}
	if tp == nil {
		t.Fatal("TracerProvider is nil")
	}

	// Use it to start + end a span; should not panic.
	_, span := tp.Tracer("test").Start(t.Context(), "smoke")
	span.End()

	if err := shutdown(t.Context()); err != nil {
		t.Errorf("shutdown: %v", err)
	}
}

func TestNewTracerProviderWithLangfuseExporter(t *testing.T) {
	langfuse := &recordingExporter{}

	tp, shutdown, err := NewTracerProvider(t.Context(), TracerOptions{
		ServiceName:      "spt-test",
		Sampling:         1.0,
		LangfuseExporter: langfuse,
	})
	if err != nil {
		t.Fatalf("NewTracerProvider: %v", err)
	}

	tracer := tp.Tracer("test")
	_, agentSpan := tracer.Start(t.Context(), "agent-call")
	SetCategory(agentSpan, SpanCategoryAgent)
	agentSpan.End()

	_, sysSpan := tracer.Start(t.Context(), "system-call")
	sysSpan.End()

	// shutdown flushes the batch processor; afterwards the Langfuse
	// exporter should hold exactly one span (the agent one).
	if err := shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	spans := langfuse.snapshot()
	if len(spans) != 1 {
		t.Fatalf("langfuse exporter: got %d spans, want 1 (agent only)", len(spans))
	}
	if spans[0].Name() != "agent-call" {
		t.Errorf("langfuse got wrong span: %q", spans[0].Name())
	}
}

func TestNewTracerProviderInvalidOTLPEndpoint(t *testing.T) {
	// Non-resolvable hostname with a bogus scheme should at least
	// surface through the wrapped error chain when the exporter
	// rejects it. The OTLP HTTP exporter is fairly forgiving and only
	// errors on truly malformed endpoints; we test the wrap path
	// rather than depending on every invalid value being rejected.
	_, _, err := NewTracerProvider(t.Context(), TracerOptions{
		ServiceName:  "spt-test",
		OTLPEndpoint: "https://example.invalid:99999",
		Sampling:     1.0,
	})
	// Either the exporter rejected the endpoint (ErrInvalidOTLPEndpoint)
	// or it accepted it as future-batch DNS (nil). Both are valid for
	// this test; we just want to exercise the OTLP branch.
	if err != nil && !errors.Is(err, ErrInvalidOTLPEndpoint) {
		t.Errorf("unexpected non-wrapped error: %v", err)
	}
}

func TestSetupNoTracingNoFlushError(t *testing.T) {
	cfg := config.Defaults()
	o, shutdown, err := Setup(t.Context(), &cfg, "spt-test")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if o.Logger == nil || o.TracerProvider == nil || o.Registry == nil {
		t.Errorf("Obs has nil field: %+v", o)
	}
	if err := shutdown(t.Context()); err != nil {
		t.Errorf("shutdown: %v", err)
	}
}

func TestSetupInvalidLogFormat(t *testing.T) {
	cfg := config.Defaults()
	cfg.Log.Format = "yaml"
	_, _, err := Setup(t.Context(), &cfg, "spt-test")
	if !errors.Is(err, ErrUnknownLogFormat) {
		t.Errorf("got %v, want ErrUnknownLogFormat", err)
	}
}

func TestCategoryFilterForceFlush(t *testing.T) {
	langfuse := &recordingExporter{}
	bsp := newCategoryFilter(
		sdktrace.NewSimpleSpanProcessor(langfuse), SpanCategoryAgent,
	)
	if err := bsp.ForceFlush(t.Context()); err != nil {
		t.Errorf("ForceFlush: %v", err)
	}
	if err := bsp.Shutdown(t.Context()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}
