package observability

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// NewMetricsMiddleware creates middleware that records HTTP request count and latency.
func NewMetricsMiddleware(provider *Provider) func(http.Handler) http.Handler {
	meter := provider.MeterProvider().Meter("updater.http")

	requestsTotal, _ := meter.Int64Counter("updater_http_requests_total",
		metric.WithDescription("Total HTTP requests"),
		metric.WithUnit("{request}"),
	)

	requestDuration, _ := meter.Float64Histogram("updater_http_request_duration_seconds",
		metric.WithDescription("HTTP request latency in seconds"),
		metric.WithUnit("s"),
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sw, r)

			elapsed := time.Since(start).Seconds()
			attrs := []attribute.KeyValue{
				attribute.String("method", r.Method),
				attribute.String("path", r.URL.Path),
				attribute.String("status", strconv.Itoa(sw.status)),
			}
			requestsTotal.Add(r.Context(), 1, metric.WithAttributes(attrs...))
			requestDuration.Record(r.Context(), elapsed, metric.WithAttributes(attrs...))
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.written {
		sw.status = code
		sw.written = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

// AppMetrics holds application-level business metrics.
type AppMetrics struct {
	UpdateChecks       metric.Int64Counter
	ReleasesRegistered metric.Int64Counter
}

// NewAppMetrics creates application-level business metric instruments.
func NewAppMetrics(provider *Provider) *AppMetrics {
	meter := provider.MeterProvider().Meter("updater.app")

	updateChecks, _ := meter.Int64Counter("updater_update_checks_total",
		metric.WithDescription("Total update check requests"),
		metric.WithUnit("{check}"),
	)

	releasesRegistered, _ := meter.Int64Counter("updater_releases_registered_total",
		metric.WithDescription("Total releases registered"),
		metric.WithUnit("{release}"),
	)

	return &AppMetrics{
		UpdateChecks:       updateChecks,
		ReleasesRegistered: releasesRegistered,
	}
}
