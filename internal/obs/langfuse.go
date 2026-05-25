package obs

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// categoryFilterProcessor wraps another SpanProcessor and only forwards
// spans whose AttrSpanCategory attribute matches `want`. Spans that
// lack the attribute (treated as SpanCategorySystem) are dropped — the
// system OTLP pipeline already has every span via its own processor.
//
// This is what realizes the agent/system split documented in
// DESIGN-0005 § "OTel Span Categories": Langfuse subscribes to agent
// spans only; the system collector receives everything.
type categoryFilterProcessor struct {
	inner sdktrace.SpanProcessor
	want  string
}

// newCategoryFilter returns a SpanProcessor that delegates OnEnd to
// inner only when a span's category equals want. OnStart, Shutdown,
// and ForceFlush are forwarded unconditionally so inner's lifecycle
// behaves identically to using it directly.
func newCategoryFilter(inner sdktrace.SpanProcessor, want string) sdktrace.SpanProcessor {
	return &categoryFilterProcessor{inner: inner, want: want}
}

func (p *categoryFilterProcessor) OnStart(ctx context.Context, s sdktrace.ReadWriteSpan) {
	p.inner.OnStart(ctx, s)
}

func (p *categoryFilterProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	if CategoryOf(s) != p.want {
		return
	}
	p.inner.OnEnd(s)
}

func (p *categoryFilterProcessor) Shutdown(ctx context.Context) error {
	return p.inner.Shutdown(ctx)
}

func (p *categoryFilterProcessor) ForceFlush(ctx context.Context) error {
	return p.inner.ForceFlush(ctx)
}
