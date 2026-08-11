// Package telemetry provides OpenTelemetry tracing and metrics initialization.
package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config holds configuration for telemetry initialization.
type Config struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string
	Environment    string
}

// Provider holds initialized telemetry providers for cleanup.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
}

// Init initializes OpenTelemetry tracing and Prometheus metrics.
// Returns a Provider that must be shut down on application exit.
func Init(ctx context.Context, cfg Config) (*Provider, error) {
	res, err := newResource(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating otel resource: %w", err)
	}

	tp, err := newTracerProvider(ctx, res, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating tracer provider: %w", err)
	}
	otel.SetTracerProvider(tp)

	mp, err := newMeterProvider(res)
	if err != nil {
		return nil, fmt.Errorf("creating meter provider: %w", err)
	}
	otel.SetMeterProvider(mp)

	return &Provider{
		tracerProvider: tp,
		meterProvider:  mp,
	}, nil
}

// Shutdown gracefully shuts down the telemetry providers.
func (p *Provider) Shutdown(ctx context.Context) error {
	var firstErr error

	if p.tracerProvider != nil {
		if err := p.tracerProvider.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("shutting down tracer provider: %w", err)
		}
	}

	if p.meterProvider != nil {
		if err := p.meterProvider.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("shutting down meter provider: %w", err)
		}
	}

	return firstErr
}

func newResource(cfg Config) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("deployment.environment.name", cfg.Environment),
		),
	)
}

func newTracerProvider(ctx context.Context, res *resource.Resource, cfg Config) (*sdktrace.TracerProvider, error) {
	if cfg.OTLPEndpoint == "" {
		// Return a no-op tracer provider if no endpoint configured.
		return sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
		), nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
	), nil
}

func newMeterProvider(res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("creating prometheus exporter: %w", err)
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exporter),
	), nil
}
