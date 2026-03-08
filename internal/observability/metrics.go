package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsServer serves Prometheus metrics on a separate port.
type MetricsServer struct {
	server *http.Server
}

// newMetricsHandler builds the HTTP handler that serves Prometheus metrics for the given
// provider at the given path. It uses the provider's configured gatherer via
// promhttp.HandlerFor so that only metrics from the configured registry are served,
// not the global default Prometheus registry.
func newMetricsHandler(path string, provider *Provider) http.Handler {
	mux := http.NewServeMux()
	if provider != nil && provider.promExporter != nil && provider.promGatherer != nil {
		mux.Handle(path, promhttp.HandlerFor(provider.promGatherer, promhttp.HandlerOpts{}))
	}
	return mux
}

// NewMetricsServer creates a metrics HTTP server serving the Prometheus handler
// at the given path on the given port.
func NewMetricsServer(port int, path string, provider *Provider) *MetricsServer {
	return &MetricsServer{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: newMetricsHandler(path, provider),
		},
	}
}

// Start begins serving metrics in a blocking call.
// Returns http.ErrServerClosed on graceful shutdown.
func (ms *MetricsServer) Start() error {
	slog.Info("Starting metrics server", "addr", ms.server.Addr)
	return ms.server.ListenAndServe()
}

// Shutdown gracefully stops the metrics server.
func (ms *MetricsServer) Shutdown(ctx context.Context) error {
	return ms.server.Shutdown(ctx)
}
