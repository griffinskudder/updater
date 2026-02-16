package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"updater/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeOpenAPISpec(t *testing.T) {
	tests := []struct {
		name            string
		wantStatus      int
		wantContentType string
		wantBodyPrefix  string
		wantBodyContain string
		wantCacheCtrl   string
	}{
		{
			name:            "returns 200 with yaml content type",
			wantStatus:      http.StatusOK,
			wantContentType: "application/yaml",
			wantBodyPrefix:  "openapi:",
			wantBodyContain: "3.0.3",
			wantCacheCtrl:   "public, max-age=3600",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := NewHandlers(&MockUpdateService{})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil)
			rec := httptest.NewRecorder()

			handlers.ServeOpenAPISpec(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantContentType, rec.Header().Get("Content-Type"))
			assert.Equal(t, tt.wantCacheCtrl, rec.Header().Get("Cache-Control"))

			body := rec.Body.String()
			require.NotEmpty(t, body)
			assert.True(t, strings.HasPrefix(strings.TrimSpace(body), tt.wantBodyPrefix),
				"body should start with %q, got: %s", tt.wantBodyPrefix, body[:min(50, len(body))])
			assert.Contains(t, body, tt.wantBodyContain)
		})
	}
}

func TestServeSwaggerUI(t *testing.T) {
	tests := []struct {
		name            string
		wantStatus      int
		wantContentType string
		wantContains    []string
		wantCacheCtrl   string
	}{
		{
			name:            "returns 200 with html content type",
			wantStatus:      http.StatusOK,
			wantContentType: "text/html; charset=utf-8",
			wantContains:    []string{"swagger-ui", "/api/v1/openapi.yaml"},
			wantCacheCtrl:   "public, max-age=3600",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := NewHandlers(&MockUpdateService{})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
			rec := httptest.NewRecorder()

			handlers.ServeSwaggerUI(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantContentType, rec.Header().Get("Content-Type"))
			assert.Equal(t, tt.wantCacheCtrl, rec.Header().Get("Cache-Control"))

			body := rec.Body.String()
			require.NotEmpty(t, body)
			for _, want := range tt.wantContains {
				assert.Contains(t, body, want, "body should contain %q", want)
			}
		})
	}
}

func TestOpenAPIRoutes_ArePublic(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		enableAuth bool
	}{
		{
			name:       "openapi spec accessible without auth",
			path:       "/api/v1/openapi.yaml",
			enableAuth: false,
		},
		{
			name:       "swagger ui accessible without auth",
			path:       "/api/v1/docs",
			enableAuth: false,
		},
		{
			name:       "openapi spec accessible without auth when auth enabled",
			path:       "/api/v1/openapi.yaml",
			enableAuth: true,
		},
		{
			name:       "swagger ui accessible without auth when auth enabled",
			path:       "/api/v1/docs",
			enableAuth: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockUpdateService{}
			handlers := NewHandlers(mockService)

			config := &models.Config{}
			config.Security.EnableAuth = tt.enableAuth
			if tt.enableAuth {
				config.Security.APIKeys = []models.APIKey{
					{Key: "test-key", Enabled: true, Permissions: []string{"read", "write", "admin"}},
				}
			}

			router := SetupRoutes(handlers, config)
			server := httptest.NewServer(router)
			defer server.Close()

			req, err := http.NewRequest(http.MethodGet, server.URL+tt.path, nil)
			require.NoError(t, err)
			// Deliberately no Authorization header

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode,
				"route %s should be public (no auth required)", tt.path)
		})
	}
}

// min returns the smaller of a and b. Used in test helpers only.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
