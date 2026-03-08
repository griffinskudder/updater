// Package observability provides OpenTelemetry-based metrics and tracing
// for the updater service. It sets up TracerProvider and MeterProvider with
// configurable exporters (stdout, OTLP for tracing; Prometheus for metrics).
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"updater/internal/models"
	"updater/internal/version"

	clientprom "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
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
	promGatherer   clientprom.Gatherer
}

// PrometheusExporter returns the Prometheus exporter for serving metrics.
func (p *Provider) PrometheusExporter() *prometheus.Exporter {
	return p.promExporter
}

// MeterProvider returns the underlying SDK MeterProvider, or nil when metrics are disabled.
func (p *Provider) MeterProvider() *sdkmetric.MeterProvider {
	return p.meterProvider
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

// Option configures Setup behavior.
type Option func(*setupOptions)

type setupOptions struct {
	prometheusRegisterer clientprom.Registerer
}

// WithPrometheusRegisterer overrides the Prometheus registerer used for the metrics
// exporter. Default is prometheus.DefaultRegisterer. Pass a custom registry in tests
// to avoid polluting the global default registry.
func WithPrometheusRegisterer(reg clientprom.Registerer) Option {
	return func(o *setupOptions) {
		o.prometheusRegisterer = reg
	}
}

// Setup initializes OpenTelemetry tracing and metrics providers based on configuration.
// It returns a Provider that must be shut down on application exit.
func Setup(metrics models.MetricsConfig, obs models.ObservabilityConfig, ver version.Info, opts ...Option) (*Provider, error) {
	o := &setupOptions{prometheusRegisterer: clientprom.DefaultRegisterer}
	for _, opt := range opts {
		opt(o)
	}

	p := &Provider{}

	env := getEnvironment()

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(obs.ServiceName),
			semconv.ServiceVersion(ver.Version),
			attribute.String("service.instance.id", ver.InstanceID),
			attribute.String("host.name", ver.Hostname),
			attribute.String("git.commit", ver.GitCommit),
			attribute.String("build.date", ver.BuildDate),
			attribute.String("deployment.environment", env),
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
		promExporter, err := prometheus.New(prometheus.WithRegisterer(o.prometheusRegisterer))
		if err != nil {
			return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
		}
		p.promExporter = promExporter
		// Derive the gatherer from the registerer. *prometheus.Registry implements both
		// Registerer and Gatherer; fall back to DefaultGatherer for any other type.
		if g, ok := o.prometheusRegisterer.(clientprom.Gatherer); ok {
			p.promGatherer = g
		} else {
			p.promGatherer = clientprom.DefaultGatherer
		}

		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(promExporter),
		)
		p.meterProvider = mp
		otel.SetMeterProvider(mp)

		// registerBuildInfo failure is non-fatal: a missing build_info gauge should
		// not prevent the service from starting, unlike resource or exporter failures
		// which would leave the entire observability stack broken.
		if err := p.registerBuildInfo(ver, env); err != nil {
			slog.Warn("failed to register build_info metric", "error", err)
		}
	}

	return p, nil
}

// registerBuildInfo registers an updater_build_info gauge that always returns 1
// with version metadata as labels. This follows the standard Prometheus build_info
// pattern, making version data queryable and enabling version-change alerting.
func (p *Provider) registerBuildInfo(ver version.Info, env string) error {
	meter := p.meterProvider.Meter("updater.build")

	gauge, err := meter.Int64ObservableGauge(
		"updater_build_info",
		metric.WithDescription("Build and version information (always 1)."),
		metric.WithUnit("{info}"),
	)
	if err != nil {
		return fmt.Errorf("register build_info gauge: %w", err)
	}

	attrs := metric.WithAttributes(
		attribute.String("version", ver.Version),
		attribute.String("git_commit", ver.GitCommit),
		attribute.String("build_date", ver.BuildDate),
		// environment is intentionally repeated as a direct label so it is queryable
		// on the metric itself — resource attributes appear in target_info, not on
		// individual metrics, in standard Prometheus scraping.
		attribute.String("environment", env),
	)

	// The Registration is discarded because this is a process-lifetime gauge
	// registered exactly once at startup. MeterProvider.Shutdown() stops all
	// callbacks, so there is no leak.
	_, err = meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			o.ObserveInt64(gauge, 1, attrs)
			return nil
		},
		gauge,
	)
	if err != nil {
		return fmt.Errorf("register build_info callback: %w", err)
	}

	return nil
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

// getEnvironment returns the deployment environment from environment variables,
// falling back to "development" if not set.
func getEnvironment() string {
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("DEPLOYMENT_ENV"); env != "" {
		return env
	}
	return "development"
}
