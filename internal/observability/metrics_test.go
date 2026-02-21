package observability

import (
	"updater/internal/version"
	"context"
	"net/http"
	"testing"
	"time"
	"updater/internal/models"

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
		ServiceName:    "test",
		Tracing:        models.TracingConfig{Enabled: false},
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
		ServiceName:    "test",
		Tracing:        models.TracingConfig{Enabled: false},
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
