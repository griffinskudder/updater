// Package observability provides OpenTelemetry-based metrics and tracing
// for the updater service. It sets up TracerProvider and MeterProvider with
// configurable exporters (stdout, OTLP for tracing; Prometheus for metrics).
package observability

import (
	"context"
	"fmt"
	"updater/internal/models"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Provider holds the OpenTelemetry providers for graceful shutdown.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	promExporter   *prometheus.Exporter
}

// PrometheusExporter returns the Prometheus exporter for serving metrics.
func (p *Provider) PrometheusExporter() *prometheus.Exporter {
	return p.promExporter
}

// Shutdown gracefully shuts down all OpenTelemetry providers.
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error

	if p.tracerProvider != nil {
		if err := p.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer provider shutdown: %w", err))
		}
	}

	if p.meterProvider != nil {
		if err := p.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("observability shutdown errors: %v", errs)
	}
	return nil
}

// Setup initializes OpenTelemetry tracing and metrics providers based on configuration.
// It returns a Provider that must be shut down on application exit.
func Setup(metrics models.MetricsConfig, obs models.ObservabilityConfig) (*Provider, error) {
	p := &Provider{}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(obs.ServiceName),
			semconv.ServiceVersion(obs.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Setup tracing
	if obs.Tracing.Enabled {
		tp, err := setupTracing(res, obs.Tracing)
		if err != nil {
			return nil, fmt.Errorf("failed to setup tracing: %w", err)
		}
		p.tracerProvider = tp
		otel.SetTracerProvider(tp)
	}

	// Setup metrics with Prometheus exporter
	if metrics.Enabled {
		promExporter, err := prometheus.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
		}
		p.promExporter = promExporter

		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(promExporter),
		)
		p.meterProvider = mp
		otel.SetMeterProvider(mp)
	}

	return p, nil
}

func setupTracing(res *resource.Resource, cfg models.TracingConfig) (*sdktrace.TracerProvider, error) {
	var exporter sdktrace.SpanExporter
	var err error

	switch cfg.Exporter {
	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	case "otlp":
		exporter, err = otlptracegrpc.New(context.Background(),
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlptracegrpc.WithInsecure(),
		)
	default:
		return nil, fmt.Errorf("unsupported trace exporter: %s", cfg.Exporter)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s exporter: %w", cfg.Exporter, err)
	}

	var sampler sdktrace.Sampler
	switch {
	case cfg.SampleRate >= 1.0:
		sampler = sdktrace.AlwaysSample()
	case cfg.SampleRate <= 0:
		sampler = sdktrace.NeverSample()
	default:
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sampler),
	)

	return tp, nil
}
