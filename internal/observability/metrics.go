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

// NewMetricsServer creates a metrics HTTP server serving the Prometheus handler
// at the given path on the given port.
func NewMetricsServer(port int, path string, provider *Provider) *MetricsServer {
	mux := http.NewServeMux()

	if provider != nil && provider.promExporter != nil {
		mux.Handle(path, promhttp.Handler())
	}

	return &MetricsServer{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
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
