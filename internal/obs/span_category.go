package obs

import (
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// AttrSpanCategory is the OTel attribute key used to distinguish system
// spans (default) from agent spans (LLM-touching stage handlers).
// Langfuse's ingestion path keys off this attribute via the SpanProcessor
// in tracing.go. The exact key name is provisional pending the
// "Langfuse-compatible attribute" question raised in DESIGN-0001;
// callers should always reference the constant rather than the literal.
const AttrSpanCategory = "spt.span_category"

// Span category values. SpanCategorySystem is implicit for any span
// that does not call SetCategory.
const (
	SpanCategorySystem = "system"
	SpanCategoryAgent  = "agent"
)

// SetCategory tags span with the given category. Pass SpanCategoryAgent
// from any handler that hits an LLM; the Langfuse processor will fork
// those spans to the Langfuse ingestion endpoint while leaving the
// system OTLP path unchanged.
func SetCategory(span trace.Span, cat string) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.String(AttrSpanCategory, cat))
}

// CategoryOf returns the span_category attribute value previously set
// on span, or SpanCategorySystem when nothing is set. This is the
// helper the Langfuse SpanProcessor uses to decide whether to forward.
func CategoryOf(span sdktrace.ReadOnlySpan) string {
	if span == nil {
		return SpanCategorySystem
	}
	for _, a := range span.Attributes() {
		if string(a.Key) == AttrSpanCategory {
			return a.Value.AsString()
		}
	}
	return SpanCategorySystem
}
