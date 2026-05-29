package obs

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestCategoryFilterForwardsMatching(t *testing.T) {
	langfuse := &recordingExporter{}
	system := &recordingExporter{}

	// Real wiring: system path receives every span via a syncer;
	// Langfuse path receives only agent-tagged spans via a filtered
	// syncer wrapping the same exporter shape.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(system),
		sdktrace.WithSpanProcessor(newCategoryFilter(
			sdktrace.NewSimpleSpanProcessor(langfuse),
			SpanCategoryAgent,
		)),
	)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	tracer := tp.Tracer("test")

	_, sys := tracer.Start(context.Background(), "system-span")
	sys.End()

	_, agent := tracer.Start(context.Background(), "agent-span")
	SetCategory(agent, SpanCategoryAgent)
	agent.End()

	systemSpans := system.snapshot()
	langfuseSpans := langfuse.snapshot()

	if len(systemSpans) != 2 {
		t.Errorf("system exporter: got %d spans, want 2 (every span)", len(systemSpans))
	}
	if len(langfuseSpans) != 1 {
		t.Errorf("langfuse exporter: got %d spans, want 1 (agent only)", len(langfuseSpans))
	}
	if len(langfuseSpans) == 1 && langfuseSpans[0].Name() != "agent-span" {
		t.Errorf("langfuse got wrong span: %q want agent-span", langfuseSpans[0].Name())
	}
}

func TestCategoryFilterDropsUntagged(t *testing.T) {
	langfuse := &recordingExporter{}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(newCategoryFilter(
			sdktrace.NewSimpleSpanProcessor(langfuse),
			SpanCategoryAgent,
		)),
	)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	_, span := tp.Tracer("test").Start(context.Background(), "untagged")
	span.End()

	if got := len(langfuse.snapshot()); got != 0 {
		t.Errorf("langfuse exporter: got %d spans, want 0 (untagged dropped)", got)
	}
}
