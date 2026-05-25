package obs

import (
	"context"
	"sync"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// recordingExporter is an in-process sdktrace.SpanExporter that
// captures every span passed to ExportSpans so the split tests can
// assert exactly which spans reach the Langfuse pipeline.
type recordingExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (r *recordingExporter) ExportSpans(_ context.Context, ss []sdktrace.ReadOnlySpan) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spans = append(r.spans, ss...)
	return nil
}

func (*recordingExporter) Shutdown(_ context.Context) error { return nil }

func (r *recordingExporter) snapshot() []sdktrace.ReadOnlySpan {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]sdktrace.ReadOnlySpan, len(r.spans))
	copy(out, r.spans)
	return out
}

func TestSetCategoryAndCategoryOf(t *testing.T) {
	system := &recordingExporter{}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(system),
	)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "tagged")
	SetCategory(span, SpanCategoryAgent)
	span.End()

	spans := system.snapshot()
	if len(spans) != 1 {
		t.Fatalf("recorded spans: got %d want 1", len(spans))
	}
	if got := CategoryOf(spans[0]); got != SpanCategoryAgent {
		t.Errorf("CategoryOf: got %q want %q", got, SpanCategoryAgent)
	}
}

func TestSetCategoryNilSpan(_ *testing.T) {
	// Just verifies no panic on nil span.
	SetCategory(nil, SpanCategoryAgent)
}

func TestCategoryOfDefaultsToSystem(t *testing.T) {
	system := &recordingExporter{}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(system))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	_, span := tp.Tracer("test").Start(context.Background(), "no-category")
	span.End()

	spans := system.snapshot()
	if got := CategoryOf(spans[0]); got != SpanCategorySystem {
		t.Errorf("untagged span: got %q want %q", got, SpanCategorySystem)
	}
}
