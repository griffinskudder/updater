package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"updater/internal/models"
	"updater/internal/storage"
	"updater/internal/update"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestSecurityContext tests the security context functionality
func TestSecurityContext(t *testing.T) {
	tests := []struct {
		name         string
		apiKey       *models.APIKey
		required     Permission
		expectAccess bool
	}{
		{
			name: "admin has all permissions",
			apiKey: &models.APIKey{
				Name:        "Admin Key",
				Permissions: []string{"admin"},
				Enabled:     true,
			},
			required:     PermissionRead,
			expectAccess: true,
		},
		{
			name: "admin has write permission",
			apiKey: &models.APIKey{
				Name:        "Admin Key",
				Permissions: []string{"admin"},
				Enabled:     true,
			},
			required:     PermissionWrite,
			expectAccess: true,
		},
		{
			name: "write has read permission",
			apiKey: &models.APIKey{
				Name:        "Write Key",
				Permissions: []string{"write"},
				Enabled:     true,
			},
			required:     PermissionRead,
			expectAccess: true,
		},
		{
			name: "read does not have write permission",
			apiKey: &models.APIKey{
				Name:        "Read Key",
				Permissions: []string{"read"},
				Enabled:     true,
			},
			required:     PermissionWrite,
			expectAccess: false,
		},
		{
			name: "read does not have admin permission",
			apiKey: &models.APIKey{
				Name:        "Read Key",
				Permissions: []string{"read"},
				Enabled:     true,
			},
			required:     PermissionAdmin,
			expectAccess: false,
		},
		{
			name:         "nil context has no permissions",
			apiKey:       nil,
			required:     PermissionRead,
			expectAccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var securityContext *SecurityContext
			if tt.apiKey != nil {
				securityContext = &SecurityContext{
					APIKey:      tt.apiKey,
					Permissions: tt.apiKey.Permissions,
				}
			}

			hasPermission := securityContext.HasPermission(tt.required)
			assert.Equal(t, tt.expectAccess, hasPermission)
		})
	}
}

// TestAuthMiddleware tests the authentication middleware
func TestAuthMiddleware(t *testing.T) {
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)

	validRawKey := "valid-key-123"
	disabledRawKey := "disabled-key"

	// Create a valid enabled key
	vk := models.NewAPIKey(models.NewKeyID(), "Test Key", validRawKey, []string{"read"})
	err = store.CreateAPIKey(context.Background(), vk)
	require.NoError(t, err)

	// Create a disabled key
	dk := models.NewAPIKey(models.NewKeyID(), "Disabled Key", disabledRawKey, []string{"admin"})
	dk.Enabled = false
	err = store.CreateAPIKey(context.Background(), dk)
	require.NoError(t, err)

	middleware := authMiddleware(store)

	tests := []struct {
		name              string
		authHeader        string
		path              string
		expectedStatus    int
		expectedError     string
		expectAPIKeyInCtx bool
	}{
		{
			name:              "valid API key",
			authHeader:        "Bearer valid-key-123",
			path:              "/api/v1/test",
			expectedStatus:    http.StatusOK,
			expectAPIKeyInCtx: true,
		},
		{
			name:              "missing authorization header",
			authHeader:        "",
			path:              "/api/v1/test",
			expectedStatus:    http.StatusUnauthorized,
			expectedError:     "Authorization required",
			expectAPIKeyInCtx: false,
		},
		{
			name:              "invalid authorization format",
			authHeader:        "InvalidFormat",
			path:              "/api/v1/test",
			expectedStatus:    http.StatusUnauthorized,
			expectedError:     "Invalid authorization format",
			expectAPIKeyInCtx: false,
		},
		{
			name:              "invalid API key",
			authHeader:        "Bearer invalid-key",
			path:              "/api/v1/test",
			expectedStatus:    http.StatusUnauthorized,
			expectedError:     "Invalid API key",
			expectAPIKeyInCtx: false,
		},
		{
			name:              "disabled API key",
			authHeader:        "Bearer disabled-key",
			path:              "/api/v1/test",
			expectedStatus:    http.StatusUnauthorized,
			expectedError:     "Invalid API key",
			expectAPIKeyInCtx: false,
		},
		{
			name:              "health check skips auth",
			authHeader:        "",
			path:              "/health",
			expectedStatus:    http.StatusOK,
			expectAPIKeyInCtx: false,
		},
		{
			name:              "api health check skips auth",
			authHeader:        "",
			path:              "/api/v1/health",
			expectedStatus:    http.StatusOK,
			expectAPIKeyInCtx: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check if API key is in context
				apiKey := r.Context().Value("api_key")
				if tt.expectAPIKeyInCtx {
					assert.NotNil(t, apiKey, "Expected API key in context")
					if actualKey, ok := apiKey.(*models.APIKey); ok {
						assert.Equal(t, "Test Key", actualKey.Name)
					}
				} else {
					assert.Nil(t, apiKey, "Expected no API key in context")
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			})

			// Create request
			req := httptest.NewRequest("GET", tt.path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Apply middleware
			middleware(handler).ServeHTTP(rr, req)

			// Check response
			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" {
				var errorResp models.ErrorResponse
				err := json.Unmarshal(rr.Body.Bytes(), &errorResp)
				require.NoError(t, err)
				assert.Contains(t, errorResp.Message, tt.expectedError)
			}
		})
	}
}

// TestRequirePermissionMiddleware tests the permission enforcement middleware
func TestRequirePermissionMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		apiKey         *models.APIKey
		required       Permission
		expectedStatus int
		expectedError  string
	}{
		{
			name: "read permission for read endpoint",
			apiKey: &models.APIKey{
				Name:        "Read Key",
				Permissions: []string{"read"},
				Enabled:     true,
			},
			required:       PermissionRead,
			expectedStatus: http.StatusOK,
		},
		{
			name: "write permission for write endpoint",
			apiKey: &models.APIKey{
				Name:        "Write Key",
				Permissions: []string{"write"},
				Enabled:     true,
			},
			required:       PermissionWrite,
			expectedStatus: http.StatusOK,
		},
		{
			name: "admin permission for any endpoint",
			apiKey: &models.APIKey{
				Name:        "Admin Key",
				Permissions: []string{"admin"},
				Enabled:     true,
			},
			required:       PermissionWrite,
			expectedStatus: http.StatusOK,
		},
		{
			name: "insufficient permission",
			apiKey: &models.APIKey{
				Name:        "Read Key",
				Permissions: []string{"read"},
				Enabled:     true,
			},
			required:       PermissionWrite,
			expectedStatus: http.StatusForbidden,
			expectedError:  "Insufficient permissions",
		},
		{
			name:           "no API key in context",
			apiKey:         nil,
			required:       PermissionRead,
			expectedStatus: http.StatusForbidden,
			expectedError:  "Insufficient permissions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			})

			// Create request with API key in context
			req := httptest.NewRequest("GET", "/test", nil)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Apply permission middleware
			middleware := RequirePermission(tt.required)

			// We need to simulate the auth middleware setting the context
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.apiKey != nil {
					// Add API key to context (simulating auth middleware)
					ctx := context.WithValue(r.Context(), "api_key", tt.apiKey)
					r = r.WithContext(ctx)
				}
				middleware(handler).ServeHTTP(w, r)
			})

			testHandler.ServeHTTP(rr, req)

			// Check response
			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" {
				var errorResp models.ErrorResponse
				err := json.Unmarshal(rr.Body.Bytes(), &errorResp)
				require.NoError(t, err)
				assert.Contains(t, errorResp.Message, tt.expectedError)
			}
		})
	}
}

// TestEndpointSecurity tests that endpoints properly enforce security
func TestEndpointSecurity(t *testing.T) {
	// Set up storage with test API keys
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	for _, spec := range []struct {
		raw   string
		name  string
		perms []string
	}{
		{"read-key-123", "Read Only Key", []string{"read"}},
		{"write-key-456", "Write Key", []string{"write"}},
		{"admin-key-789", "Admin Key", []string{"admin"}},
	} {
		ak := models.NewAPIKey(models.NewKeyID(), spec.name, spec.raw, spec.perms)
		require.NoError(t, store.CreateAPIKey(context.Background(), ak))
	}

	// Create test configuration
	config := &models.Config{
		Security: models.SecurityConfig{
			EnableAuth:   true,
			BootstrapKey: "upd_test-bootstrap",
		},
		Server: models.ServerConfig{},
	}

	// Mock handlers with update service
	mockUpdateService := &MockUpdateService{}

	// Set up mock expectations to return application not found for all requests
	mockUpdateService.On("CheckForUpdate", mock.Anything, mock.Anything).
		Return((*models.UpdateCheckResponse)(nil), update.NewApplicationNotFoundError("test-app"))
	mockUpdateService.On("GetLatestVersion", mock.Anything, mock.Anything).
		Return((*models.LatestVersionResponse)(nil), update.NewApplicationNotFoundError("test-app"))
	mockUpdateService.On("ListReleases", mock.Anything, mock.Anything).
		Return((*models.ListReleasesResponse)(nil), update.NewApplicationNotFoundError("test-app"))
	mockUpdateService.On("RegisterRelease", mock.Anything, mock.MatchedBy(func(req *models.RegisterReleaseRequest) bool {
		return req.Version == ""
	})).Return((*models.RegisterReleaseResponse)(nil), update.NewInvalidRequestError("invalid request: missing required fields", nil))
	mockUpdateService.On("RegisterRelease", mock.Anything, mock.MatchedBy(func(req *models.RegisterReleaseRequest) bool {
		return req.Version != ""
	})).Return(&models.RegisterReleaseResponse{
		ID:      "test-id",
		Message: "Release registered successfully",
	}, nil)

	mockHandlers := NewHandlers(mockUpdateService, WithStorage(store))

	// Setup routes
	router := SetupRoutes(mockHandlers, config)

	tests := []struct {
		name           string
		method         string
		path           string
		authHeader     string
		expectedStatus int
		description    string
	}{
		{
			name:           "public update check",
			method:         "GET",
			path:           "/api/v1/updates/test-app/check",
			authHeader:     "",
			expectedStatus: http.StatusNotFound, // Handler not implemented, but should pass auth
			description:    "Update check should be publicly accessible",
		},
		{
			name:           "public latest version",
			method:         "GET",
			path:           "/api/v1/updates/test-app/latest",
			authHeader:     "",
			expectedStatus: http.StatusNotFound, // Handler not implemented, but should pass auth
			description:    "Latest version should be publicly accessible",
		},
		{
			name:           "protected releases list without auth",
			method:         "GET",
			path:           "/api/v1/updates/test-app/releases",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			description:    "Releases list should require authentication",
		},
		{
			name:           "protected releases list with read permission",
			method:         "GET",
			path:           "/api/v1/updates/test-app/releases",
			authHeader:     "Bearer read-key-123",
			expectedStatus: http.StatusNotFound, // Handler not implemented, but should pass auth
			description:    "Releases list should accept read permission",
		},
		{
			name:           "protected register without auth",
			method:         "POST",
			path:           "/api/v1/updates/test-app/register",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			description:    "Release registration should require authentication",
		},
		{
			name:           "protected register with insufficient permission",
			method:         "POST",
			path:           "/api/v1/updates/test-app/register",
			authHeader:     "Bearer read-key-123",
			expectedStatus: http.StatusForbidden,
			description:    "Release registration should require write permission",
		},
		{
			name:           "protected register with write permission",
			method:         "POST",
			path:           "/api/v1/updates/test-app/register",
			authHeader:     "Bearer write-key-456",
			expectedStatus: http.StatusCreated, // Mock returns successful response
			description:    "Release registration should accept write permission",
		},
		{
			name:           "protected register with admin permission",
			method:         "POST",
			path:           "/api/v1/updates/test-app/register",
			authHeader:     "Bearer admin-key-789",
			expectedStatus: http.StatusCreated, // Mock returns successful response
			description:    "Release registration should accept admin permission",
		},
		{
			name:           "health check public access",
			method:         "GET",
			path:           "/health",
			authHeader:     "",
			expectedStatus: http.StatusOK, // Health check should return 200 OK
			description:    "Health check should be publicly accessible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with proper payload for POST requests
			var body []byte
			if tt.method == "POST" {
				// Send a valid registration request
				body = []byte(`{"version": "1.0.0", "platform": "windows", "architecture": "amd64", "download_url": "https://example.com/download.exe"}`)
			} else {
				body = []byte(`{"test": "data"}`)
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			rr := httptest.NewRecorder()

			// Execute request
			router.ServeHTTP(rr, req)

			// Check response
			assert.Equal(t, tt.expectedStatus, rr.Code, fmt.Sprintf("Test: %s - %s", tt.name, tt.description))
		})
	}
}

// TestSecurityVulnerabilities tests for common security vulnerabilities
func TestSecurityVulnerabilities(t *testing.T) {
	vulnStore, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	vulnAdminKey := "admin-key-123"
	vak := models.NewAPIKey(models.NewKeyID(), "Admin Key", vulnAdminKey, []string{"admin"})
	require.NoError(t, vulnStore.CreateAPIKey(context.Background(), vak))

	config := &models.Config{
		Security: models.SecurityConfig{
			EnableAuth:   true,
			BootstrapKey: "upd_test-bootstrap",
		},
		Server: models.ServerConfig{},
	}

	mockUpdateService := &MockUpdateService{}

	// Set up mock expectations for vulnerability tests
	mockUpdateService.On("CheckForUpdate", mock.Anything, mock.Anything).
		Return((*models.UpdateCheckResponse)(nil), update.NewApplicationNotFoundError("test-app")).Maybe()
	mockUpdateService.On("GetLatestVersion", mock.Anything, mock.Anything).
		Return((*models.LatestVersionResponse)(nil), update.NewApplicationNotFoundError("test-app")).Maybe()
	mockUpdateService.On("ListReleases", mock.Anything, mock.Anything).
		Return((*models.ListReleasesResponse)(nil), update.NewApplicationNotFoundError("test-app")).Maybe()
	mockUpdateService.On("RegisterRelease", mock.Anything, mock.Anything).
		Return(&models.RegisterReleaseResponse{
			ID:      "test-id",
			Message: "Release registered successfully",
		}, nil).Maybe()

	mockHandlers := NewHandlers(mockUpdateService, WithStorage(vulnStore))
	router := SetupRoutes(mockHandlers, config)

	t.Run("SQL Injection Protection", func(t *testing.T) {
		maliciousInputs := []string{
			"'; DROP TABLE releases; --",
			"test' OR '1'='1",
			"test'; DELETE FROM applications; --",
			"test' UNION SELECT * FROM users --",
		}

		for _, maliciousInput := range maliciousInputs {
			// Properly URL-encode the malicious input to prevent NewRequest parsing issues
			encodedInput := url.QueryEscape(maliciousInput)
			path := fmt.Sprintf("/api/v1/updates/%s/check", encodedInput)
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			// Should not return 500 (internal server error) - should handle gracefully
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code,
				"SQL injection attempt should not cause internal server error: %s", maliciousInput)

			// Should typically return 400 or 404 for malformed input
			assert.True(t, rr.Code == http.StatusBadRequest || rr.Code == http.StatusNotFound,
				"Expected 400 or 404 for malicious input: %s, got: %d", maliciousInput, rr.Code)
		}
	})

	t.Run("Path Traversal Protection", func(t *testing.T) {
		pathTraversalAttempts := []string{
			"../../../etc/passwd",
			"....//....//etc//passwd",
			"..\\..\\..\\windows\\system32\\config\\sam",
			"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		}

		for _, traversalAttempt := range pathTraversalAttempts {
			path := fmt.Sprintf("/api/v1/updates/%s/check", traversalAttempt)
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			// Should not return successful response for path traversal
			assert.NotEqual(t, http.StatusOK, rr.Code,
				"Path traversal attempt should not succeed: %s", traversalAttempt)

			// Should not return 500 (should handle gracefully)
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code,
				"Path traversal should not cause internal server error: %s", traversalAttempt)
		}
	})

	t.Run("Header Injection Protection", func(t *testing.T) {
		maliciousHeaders := map[string]string{
			"Authorization": "Bearer test\r\nX-Injected-Header: evil",
			"User-Agent":    "Mozilla/5.0\r\nX-Injected: malicious",
			"Content-Type":  "application/json\r\nHost: evil.com",
		}

		for headerName, maliciousValue := range maliciousHeaders {
			req := httptest.NewRequest("GET", "/api/v1/updates/test/releases", nil)
			req.Header.Set(headerName, maliciousValue)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			// Should not return 500 (should handle malformed headers gracefully)
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code,
				"Header injection should not cause internal server error: %s", headerName)

			// Should reject malformed authorization headers
			if headerName == "Authorization" {
				assert.Equal(t, http.StatusUnauthorized, rr.Code,
					"Malformed Authorization header should be rejected")
			}
		}
	})

	t.Run("Large Payload Protection", func(t *testing.T) {
		// Create a large JSON payload (>1MB)
		largeData := make(map[string]interface{})
		largeString := strings.Repeat("x", 1024*1024) // 1MB string
		largeData["large_field"] = largeString
		largeData["version"] = "1.0.0"
		largeData["platform"] = "windows"

		jsonData, err := json.Marshal(largeData)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/v1/updates/test/register", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer admin-key-123")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// Should handle large payloads gracefully (either reject or process)
		assert.True(t, rr.Code == http.StatusRequestEntityTooLarge ||
			rr.Code == http.StatusBadRequest ||
			rr.Code == http.StatusCreated || // Mock might accept the payload
			rr.Code == http.StatusNotFound, // Handler not implemented
			"Large payload should be handled gracefully, got: %d", rr.Code)
	})

	t.Run("Invalid JSON Protection", func(t *testing.T) {
		invalidJSONPayloads := []string{
			`{invalid json}`,
			`{"unclosed": "quote}`,
			`{"number": 123abc}`,
			`{{"nested": "malformed"}}`,
		}

		for _, invalidJSON := range invalidJSONPayloads {
			req := httptest.NewRequest("POST", "/api/v1/updates/test/register",
				strings.NewReader(invalidJSON))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer admin-key-123")
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			// Should return 400 Bad Request for invalid JSON
			assert.Equal(t, http.StatusBadRequest, rr.Code,
				"Invalid JSON should return 400 Bad Request: %s", invalidJSON)

			var errorResp models.ErrorResponse
			err := json.Unmarshal(rr.Body.Bytes(), &errorResp)
			require.NoError(t, err)
			assert.Contains(t, errorResp.Message, "Invalid JSON")
		}
	})
}

// TestSecurityHeaders tests that appropriate security headers are set
func TestSecurityHeaders(t *testing.T) {
	config := &models.Config{
		Security: models.SecurityConfig{
			EnableAuth: false,
		},
		Server: models.ServerConfig{},
	}

	mockUpdateService := &MockUpdateService{}

	// Set up mock expectations for header tests
	mockUpdateService.On("CheckForUpdate", mock.Anything, mock.Anything).
		Return((*models.UpdateCheckResponse)(nil), update.NewApplicationNotFoundError("test")).Maybe()

	mockHandlers := NewHandlers(mockUpdateService)
	router := SetupRoutes(mockHandlers, config)

	t.Run("Content-Type Header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// Should set proper Content-Type for JSON responses
		if rr.Code == http.StatusOK {
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		}
	})
}

// BenchmarkAuthMiddleware benchmarks authentication middleware performance
func BenchmarkAuthMiddleware(b *testing.B) {
	benchStore, _ := storage.NewMemoryStorage(storage.Config{})
	benchRawKey := "benchmark-key-123"
	bak := models.NewAPIKey(models.NewKeyID(), "Benchmark Key", benchRawKey, []string{"read"})
	_ = benchStore.CreateAPIKey(context.Background(), bak)

	middleware := authMiddleware(benchStore)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer benchmark-key-123")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)
		}
	})
}

// BenchmarkPermissionCheck benchmarks permission checking performance
func BenchmarkPermissionCheck(b *testing.B) {
	securityContext := &SecurityContext{
		APIKey: &models.APIKey{
			Name:        "Test Key",
			Permissions: []string{"read", "write"},
			Enabled:     true,
		},
		Permissions: []string{"read", "write"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = securityContext.HasPermission(PermissionRead)
	}
}

// TestOptionalAuthMiddleware tests optional authentication functionality
func TestOptionalAuthMiddleware(t *testing.T) {
	optStore, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	optRawKey := "valid-key-123"
	ok := models.NewAPIKey(models.NewKeyID(), "Test Key", optRawKey, []string{"read"})
	err = optStore.CreateAPIKey(context.Background(), ok)
	require.NoError(t, err)

	middleware := OptionalAuth(optStore)

	tests := []struct {
		name              string
		authHeader        string
		expectAPIKeyInCtx bool
	}{
		{
			name:              "valid API key sets context",
			authHeader:        "Bearer valid-key-123",
			expectAPIKeyInCtx: true,
		},
		{
			name:              "no auth header continues without context",
			authHeader:        "",
			expectAPIKeyInCtx: false,
		},
		{
			name:              "invalid format continues without context",
			authHeader:        "InvalidFormat",
			expectAPIKeyInCtx: false,
		},
		{
			name:              "invalid key continues without context",
			authHeader:        "Bearer invalid-key",
			expectAPIKeyInCtx: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiKey := r.Context().Value("api_key")
				if tt.expectAPIKeyInCtx {
					assert.NotNil(t, apiKey, "Expected API key in context")
				} else {
					assert.Nil(t, apiKey, "Expected no API key in context")
				}

				w.WriteHeader(http.StatusOK)
			})

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Apply middleware
			middleware(handler).ServeHTTP(rr, req)

			// All requests should succeed with optional auth
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}
