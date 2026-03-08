package observability

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// NewMetricsMiddleware creates middleware that records HTTP request count and latency.
// It returns an error if any metric instrument cannot be created.
func NewMetricsMiddleware(provider *Provider) (func(http.Handler) http.Handler, error) {
	meter := provider.MeterProvider().Meter("updater.http")

	requestsTotal, err := meter.Int64Counter("updater_http_requests_total",
		metric.WithDescription("Total HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("create requests_total counter: %w", err)
	}

	requestDuration, err := meter.Float64Histogram("updater_http_request_duration_seconds",
		metric.WithDescription("HTTP request latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create request_duration histogram: %w", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sw, r)

			elapsed := time.Since(start).Seconds()
			path := r.URL.Path
			if route := mux.CurrentRoute(r); route != nil {
				if tpl, err := route.GetPathTemplate(); err == nil {
					path = tpl
				}
			}
			attrs := []attribute.KeyValue{
				attribute.String("method", r.Method),
				attribute.String("path", path),
				attribute.String("status", strconv.Itoa(sw.status)),
			}
			requestsTotal.Add(r.Context(), 1, metric.WithAttributes(attrs...))
			requestDuration.Record(r.Context(), elapsed, metric.WithAttributes(attrs...))
		})
	}, nil
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.written {
		sw.written = true
	}
	return sw.ResponseWriter.Write(b)
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
// It returns an error if any metric instrument cannot be created.
func NewAppMetrics(provider *Provider) (*AppMetrics, error) {
	meter := provider.MeterProvider().Meter("updater.app")

	updateChecks, err := meter.Int64Counter("updater_update_checks_total",
		metric.WithDescription("Total update check requests"),
		metric.WithUnit("{check}"),
	)
	if err != nil {
		return nil, fmt.Errorf("create update_checks counter: %w", err)
	}

	releasesRegistered, err := meter.Int64Counter("updater_releases_registered_total",
		metric.WithDescription("Total releases registered"),
		metric.WithUnit("{release}"),
	)
	if err != nil {
		return nil, fmt.Errorf("create releases_registered counter: %w", err)
	}

	return &AppMetrics{
		UpdateChecks:       updateChecks,
		ReleasesRegistered: releasesRegistered,
	}, nil
}
