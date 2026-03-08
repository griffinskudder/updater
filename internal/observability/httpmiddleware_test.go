package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"updater/internal/models"
	"updater/internal/version"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	metrics := models.MetricsConfig{Enabled: true, Path: "/metrics", Port: 9090}
	obs := models.ObservabilityConfig{ServiceName: "test"}
	// Use an isolated registry per test to avoid duplicate-registration panics when
	// multiple tests call Setup in the same process.
	p, err := Setup(metrics, obs, version.Info{}, WithPrometheusRegisterer(prometheus.NewRegistry()))
	require.NoError(t, err)
	t.Cleanup(func() { p.Shutdown(context.Background()) })
	return p
}

func TestMetricsMiddleware_RecordsRequestCount(t *testing.T) {
	provider := newTestProvider(t)
	middleware, err := NewMetricsMiddleware(provider)
	require.NoError(t, err)

	router := mux.NewRouter()
	router.Handle("/api/v1/updates/{app_id}/check", middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))).Methods("GET")

	req := httptest.NewRequest("GET", "/api/v1/updates/myapp/check", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMetricsMiddleware_CapturesStatusCode(t *testing.T) {
	provider := newTestProvider(t)
	middleware, err := NewMetricsMiddleware(provider)
	require.NoError(t, err)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("GET", "/api/v1/updates/unknown/check", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStatusWriter_DefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	// Write body without calling WriteHeader; status should remain 200.
	sw.Write([]byte("ok"))
	assert.Equal(t, http.StatusOK, sw.status)
}

func TestStatusWriter_WriteHeaderOnlyOnce(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	sw.WriteHeader(http.StatusCreated)
	sw.WriteHeader(http.StatusInternalServerError) // should be ignored
	assert.Equal(t, http.StatusCreated, sw.status)
}

func TestStatusWriter_WriteSetsWrittenFlag(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	assert.False(t, sw.written, "written should be false before Write")

	n, err := sw.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.True(t, sw.written, "written should be true after Write")

	// A subsequent WriteHeader should not change the status.
	sw.WriteHeader(http.StatusInternalServerError)
	assert.Equal(t, http.StatusOK, sw.status, "WriteHeader after Write should not change status")
}

func TestNewAppMetrics(t *testing.T) {
	provider := newTestProvider(t)
	m, err := NewAppMetrics(provider)
	require.NoError(t, err)
	assert.NotNil(t, m.UpdateChecks)
	assert.NotNil(t, m.ReleasesRegistered)
}
