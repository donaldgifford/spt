package obs

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type loggerKey struct{}

// WithLogger stores l on ctx so downstream LoggerFromContext calls
// receive the same logger. Callers typically pass a request-scoped
// logger with handler-specific fields already attached.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

// LoggerFromContext returns the logger stashed by WithLogger, falling
// back to slog.Default(). When a span is active on ctx, the returned
// logger has trace_id and span_id attributes attached so log lines and
// spans correlate without manual plumbing.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	l, ok := ctx.Value(loggerKey{}).(*slog.Logger)
	if !ok || l == nil {
		l = slog.Default()
	}

	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return l
	}
	return l.With(
		slog.String("trace_id", sc.TraceID().String()),
		slog.String("span_id", sc.SpanID().String()),
	)
}
