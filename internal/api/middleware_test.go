package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"updater/internal/models"
	"updater/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestAPIKey creates an API key in the store and returns the raw key.
func newTestAPIKey(t *testing.T, store storage.Storage, name, rawKey string, perms []string, enabled bool) string {
	t.Helper()
	ak := models.NewAPIKey(models.NewKeyID(), name, rawKey, perms)
	ak.Enabled = enabled
	err := store.CreateAPIKey(context.Background(), ak)
	require.NoError(t, err)
	return rawKey
}

// TestAuthMiddlewareWithStorage tests authMiddleware using storage-backed key lookup.
func TestAuthMiddlewareWithStorage(t *testing.T) {
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)

	validKey := newTestAPIKey(t, store, "Valid Key", "valid-raw-key", []string{"read"}, true)
	newTestAPIKey(t, store, "Disabled Key", "disabled-raw-key", []string{"admin"}, false)

	mw := authMiddleware(store)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name           string
		authHeader     string
		path           string
		expectedStatus int
	}{
		{"valid key returns 200", "Bearer " + validKey, "/api/v1/test", http.StatusOK},
		{"missing authorization header returns 401", "", "/api/v1/test", http.StatusUnauthorized},
		{"invalid key returns 401", "Bearer totally-invalid-key", "/api/v1/test", http.StatusUnauthorized},
		{"disabled key returns 401", "Bearer disabled-raw-key", "/api/v1/test", http.StatusUnauthorized},
		{"health check skips auth", "", "/health", http.StatusOK},
		{"api health check skips auth", "", "/api/v1/health", http.StatusOK},
		{"invalid bearer format returns 401", "InvalidFormat", "/api/v1/test", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()
			mw(handler).ServeHTTP(rr, req)
			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

// TestOptionalAuthWithStorage tests OptionalAuth using storage-backed key lookup.
func TestOptionalAuthWithStorage(t *testing.T) {
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)

	validKey := newTestAPIKey(t, store, "Valid Key", "opt-valid-key", []string{"read"}, true)

	mw := OptionalAuth(store)

	tests := []struct {
		name           string
		authHeader     string
		expectKeyInCtx bool
	}{
		{"valid key sets context", "Bearer " + validKey, true},
		{"no auth header continues without auth", "", false},
		{"invalid key continues without auth", "Bearer invalid-key-xyz", false},
		{"invalid format continues without auth", "InvalidFormat", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctxKey *models.APIKey
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctxKey, _ = r.Context().Value("api_key").(*models.APIKey)
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()
			mw(handler).ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code, "OptionalAuth should never block requests")
			if tt.expectKeyInCtx {
				assert.NotNil(t, ctxKey, "expected API key in context")
			} else {
				assert.Nil(t, ctxKey, "expected no API key in context")
			}
		})
	}
}

// TestIsValidAdminKeyWithStorage tests isValidAdminKey using storage-backed key lookup.
func TestIsValidAdminKeyWithStorage(t *testing.T) {
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)

	adminKey := newTestAPIKey(t, store, "Admin Key", "admin-raw-key", []string{"admin"}, true)
	readKey := newTestAPIKey(t, store, "Read Key", "read-raw-key", []string{"read"}, true)

	tests := []struct {
		name       string
		key        string
		enableAuth bool
		expected   bool
	}{
		{"valid admin key returns true", adminKey, true, true},
		{"non-admin key returns false", readKey, true, false},
		{"enableAuth=false accepts any non-empty key", "any-key-will-do", false, true},
		{"empty key always returns false", "", false, false},
		{"unknown key returns false when auth enabled", "unknown-key-xyz", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidAdminKey(context.Background(), tt.key, store, tt.enableAuth)
			assert.Equal(t, tt.expected, result)
		})
	}
}
