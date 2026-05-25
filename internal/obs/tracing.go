package obs

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

// ErrInvalidOTLPEndpoint signals NewTracerProvider was called with a
// non-empty endpoint that the OTLP HTTP exporter rejected.
var ErrInvalidOTLPEndpoint = errors.New("obs: invalid OTLP endpoint")

// TracerOptions configures NewTracerProvider. ServiceName labels the
// emitted resource (Tempo/Jaeger key off this). OTLPEndpoint is the
// system-spans destination — empty string disables the OTLP exporter
// (useful for tests). LangfuseExporter is the optional agent-only
// destination; if nil, no Langfuse-side processor is registered.
// Sampling is the head sampling ratio for both exporters in [0, 1].
type TracerOptions struct {
	ServiceName      string
	OTLPEndpoint     string
	Sampling         float64
	LangfuseExporter sdktrace.SpanExporter
}

// NewTracerProvider constructs an sdktrace.TracerProvider wired with
// the system OTLP exporter (when configured) and the agent-only
// Langfuse processor (when an exporter is provided). The returned
// shutdown function flushes both pipelines and tears down the
// providers; call it from the role's defer in Run.
func NewTracerProvider(
	ctx context.Context, opts TracerOptions,
) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(opts.ServiceName),
	))
	if err != nil {
		return nil, nil, fmt.Errorf("obs: build resource: %w", err)
	}

	sampling := opts.Sampling
	if sampling <= 0 {
		sampling = 1.0
	}

	tpOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(sampling)),
	}

	var shutdownFns []func(context.Context) error

	if opts.OTLPEndpoint != "" {
		exp, err := otlptrace.New(ctx, otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(opts.OTLPEndpoint),
			otlptracehttp.WithInsecure(),
		))
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %s: %w", ErrInvalidOTLPEndpoint, opts.OTLPEndpoint, err)
		}
		tpOpts = append(tpOpts, sdktrace.WithBatcher(exp))
		shutdownFns = append(shutdownFns, exp.Shutdown)
	}

	if opts.LangfuseExporter != nil {
		bsp := sdktrace.NewBatchSpanProcessor(opts.LangfuseExporter)
		tpOpts = append(tpOpts, sdktrace.WithSpanProcessor(newCategoryFilter(bsp, SpanCategoryAgent)))
		shutdownFns = append(shutdownFns, opts.LangfuseExporter.Shutdown)
	}

	tp := sdktrace.NewTracerProvider(tpOpts...)
	shutdownFns = append(shutdownFns, tp.Shutdown)

	return tp, composeShutdown(shutdownFns), nil
}

// composeShutdown calls each shutdown in order, returning the first
// non-nil error but always running every fn so partial-flush failures
// don't strand a downstream pipeline.
func composeShutdown(fns []func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		var firstErr error
		for _, fn := range fns {
			if err := fn(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
}
