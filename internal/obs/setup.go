package obs

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/donaldgifford/spt/internal/config"
)

// Obs is the bundle of observability primitives a role uses for its
// lifetime. Setup wires every field; the role plumbs Logger into
// request scopes (via WithLogger) and registers role-specific metrics
// against Registry.
type Obs struct {
	Logger         *slog.Logger
	TracerProvider *sdktrace.TracerProvider
	Registry       *prometheus.Registry
}

// Setup wires logging, tracing, and metrics from a typed Config in one
// call. The returned shutdown fn flushes traces and shuts down OTel
// exporters; call it on the role's Run defer. ServiceName is the
// resource label OTel attaches to every span (e.g., "spt-api").
//
// Setup also installs the constructed *slog.Logger as the default
// (slog.SetDefault) so packages that haven't been refactored to use
// LoggerFromContext yet still emit through the configured handler.
func Setup(
	ctx context.Context, cfg *config.Config, serviceName string,
) (*Obs, func(context.Context) error, error) {
	logger, err := NewLogger(os.Stderr, cfg.Log.Format, cfg.Log.Level)
	if err != nil {
		return nil, nil, fmt.Errorf("obs: configure logger: %w", err)
	}
	slog.SetDefault(logger)

	tp, shutdown, err := NewTracerProvider(ctx, TracerOptions{
		ServiceName:  serviceName,
		OTLPEndpoint: cfg.Obs.OTLPEndpoint,
		Sampling:     cfg.Obs.SpanSampling,
		// LangfuseExporter is left nil until the Langfuse client lands
		// in a later IMPL. The agent/system split plumbing is already
		// wired — when an exporter shows up, drop it here and tagged
		// spans route automatically.
	})
	if err != nil {
		return nil, nil, fmt.Errorf("obs: configure tracer: %w", err)
	}
	otel.SetTracerProvider(tp)

	return &Obs{
		Logger:         logger,
		TracerProvider: tp,
		Registry:       NewRegistry(),
	}, shutdown, nil
}
