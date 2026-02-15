package ratelimit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"updater/internal/models"
)

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestMiddleware_AllowedRequest(t *testing.T) {
	limiter := NewMemoryLimiter(60, 10, 5*time.Minute)
	defer limiter.Close()

	handler := Middleware(limiter, limiter)(http.HandlerFunc(okHandler))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Reset"))
}

func TestMiddleware_DeniedRequest(t *testing.T) {
	limiter := NewMemoryLimiter(60, 2, 5*time.Minute)
	defer limiter.Close()

	handler := Middleware(limiter, limiter)(http.HandlerFunc(okHandler))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// Third request should be denied
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Limit"))

	// Verify JSON error body
	var errResp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Rate limit exceeded", errResp["message"])
}

func TestMiddleware_AuthenticatedRequest(t *testing.T) {
	anonLimiter := NewMemoryLimiter(60, 2, 5*time.Minute)
	defer anonLimiter.Close()
	authLimiter := NewMemoryLimiter(120, 5, 5*time.Minute)
	defer authLimiter.Close()

	handler := Middleware(anonLimiter, authLimiter)(http.HandlerFunc(okHandler))

	apiKey := &models.APIKey{
		Key:         "test-key",
		Name:        "Test Key",
		Permissions: []string{"read"},
		Enabled:     true,
	}

	// Anonymous requests exhaust burst of 2
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// Anonymous is now denied
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)

	// Authenticated request from same IP should still be allowed (different limiter)
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	ctx := context.WithValue(req.Context(), "api_key", apiKey)
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify authenticated limit header is 120
	limit, err := strconv.Atoi(rr.Header().Get("X-RateLimit-Limit"))
	require.NoError(t, err)
	assert.Equal(t, 120, limit)
}

func TestMiddleware_XForwardedFor(t *testing.T) {
	limiter := NewMemoryLimiter(60, 10, 5*time.Minute)
	defer limiter.Close()

	handler := Middleware(limiter, limiter)(http.HandlerFunc(okHandler))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMiddleware_XRealIP(t *testing.T) {
	limiter := NewMemoryLimiter(60, 10, 5*time.Minute)
	defer limiter.Close()

	handler := Middleware(limiter, limiter)(http.HandlerFunc(okHandler))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Real-IP", "203.0.113.50")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}
