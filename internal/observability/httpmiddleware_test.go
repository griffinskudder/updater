package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"updater/internal/models"
	"updater/internal/version"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	metrics := models.MetricsConfig{Enabled: true, Path: "/metrics", Port: 9090}
	obs := models.ObservabilityConfig{ServiceName: "test"}
	p, err := Setup(metrics, obs, version.Info{})
	require.NoError(t, err)
	t.Cleanup(func() { p.Shutdown(context.Background()) })
	return p
}

func TestMetricsMiddleware_RecordsRequestCount(t *testing.T) {
	provider := newTestProvider(t)
	middleware := NewMetricsMiddleware(provider)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/updates/myapp/check", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMetricsMiddleware_CapturesStatusCode(t *testing.T) {
	provider := newTestProvider(t)
	middleware := NewMetricsMiddleware(provider)

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

func TestNewAppMetrics(t *testing.T) {
	provider := newTestProvider(t)
	m := NewAppMetrics(provider)
	assert.NotNil(t, m.UpdateChecks)
	assert.NotNil(t, m.ReleasesRegistered)
}
