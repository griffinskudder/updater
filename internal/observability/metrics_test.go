package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"updater/internal/models"
	"updater/internal/version"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricsServer(t *testing.T) {
	metrics := models.MetricsConfig{
		Enabled: true,
		Path:    "/metrics",
		Port:    9090,
	}
	obs := models.ObservabilityConfig{
		ServiceName: "test",
		Tracing:     models.TracingConfig{Enabled: false},
	}

	provider, err := Setup(metrics, obs, version.Info{})
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	ms := NewMetricsServer(metrics.Port, metrics.Path, provider)
	assert.NotNil(t, ms)
	assert.NotNil(t, ms.server)
}

func TestMetricsServer_StartAndShutdown(t *testing.T) {
	metrics := models.MetricsConfig{
		Enabled: true,
		Path:    "/metrics",
		Port:    0, // Will use a random port
	}
	obs := models.ObservabilityConfig{
		ServiceName: "test",
		Tracing:     models.TracingConfig{Enabled: false},
	}

	provider, err := Setup(metrics, obs, version.Info{})
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	ms := NewMetricsServer(0, metrics.Path, provider)

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.Start()
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = ms.Shutdown(ctx)
	assert.NoError(t, err)

	// Verify server stopped
	serverErr := <-errCh
	assert.Equal(t, http.ErrServerClosed, serverErr)
}

func TestNewMetricsServer_NilProvider(t *testing.T) {
	ms := NewMetricsServer(9090, "/metrics", nil)
	assert.NotNil(t, ms)
}

func TestMetricsHandler_ServesConfiguredRegistry(t *testing.T) {
	// Use an isolated registry: OTel metrics are registered here, not in the global default registry.
	reg := prometheus.NewRegistry()

	metrics := models.MetricsConfig{Enabled: true, Path: "/metrics", Port: 0}
	obs := models.ObservabilityConfig{ServiceName: "test-svc"}

	provider, err := Setup(metrics, obs, version.Info{}, WithPrometheusRegisterer(reg))
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	handler := newMetricsHandler("/metrics", provider)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// updater_build_info is registered by Setup into the custom registry.
	// With the bug (promhttp.Handler() using global registry), this metric would NOT appear.
	assert.Contains(t, rec.Body.String(), "updater_build_info")
}
