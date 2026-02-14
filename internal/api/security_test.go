package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"updater/internal/models"

	"github.com/stretchr/testify/assert"
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
				Key:         "admin-key",
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
				Key:         "admin-key",
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
				Key:         "write-key",
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
				Key:         "read-key",
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
				Key:         "read-key",
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
	securityConfig := models.SecurityConfig{
		EnableAuth: true,
		APIKeys: []models.APIKey{
			{
				Key:         "valid-key-123",
				Name:        "Test Key",
				Permissions: []string{"read"},
				Enabled:     true,
			},
			{
				Key:         "disabled-key",
				Name:        "Disabled Key",
				Permissions: []string{"admin"},
				Enabled:     false,
			},
		},
	}

	middleware := authMiddleware(securityConfig)

	tests := []struct {
		name               string
		authHeader         string
		path               string
		expectedStatus     int
		expectedError      string
		expectAPIKeyInCtx  bool
	}{
		{
			name:               "valid API key",
			authHeader:         "Bearer valid-key-123",
			path:               "/api/v1/test",
			expectedStatus:     http.StatusOK,
			expectAPIKeyInCtx:  true,
		},
		{
			name:               "missing authorization header",
			authHeader:         "",
			path:               "/api/v1/test",
			expectedStatus:     http.StatusUnauthorized,
			expectedError:      "Authorization required",
			expectAPIKeyInCtx:  false,
		},
		{
			name:               "invalid authorization format",
			authHeader:         "InvalidFormat",
			path:               "/api/v1/test",
			expectedStatus:     http.StatusUnauthorized,
			expectedError:      "Invalid authorization format",
			expectAPIKeyInCtx:  false,
		},
		{
			name:               "invalid API key",
			authHeader:         "Bearer invalid-key",
			path:               "/api/v1/test",
			expectedStatus:     http.StatusUnauthorized,
			expectedError:      "Invalid API key",
			expectAPIKeyInCtx:  false,
		},
		{
			name:               "disabled API key",
			authHeader:         "Bearer disabled-key",
			path:               "/api/v1/test",
			expectedStatus:     http.StatusUnauthorized,
			expectedError:      "Invalid API key",
			expectAPIKeyInCtx:  false,
		},
		{
			name:               "health check skips auth",
			authHeader:         "",
			path:               "/health",
			expectedStatus:     http.StatusOK,
			expectAPIKeyInCtx:  false,
		},
		{
			name:               "api health check skips auth",
			authHeader:         "",
			path:               "/api/v1/health",
			expectedStatus:     http.StatusOK,
			expectAPIKeyInCtx:  false,
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
						assert.Equal(t, "valid-key-123", actualKey.Key)
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
				Key:         "read-key",
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
				Key:         "write-key",
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
				Key:         "admin-key",
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
				Key:         "read-key",
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
	// Create test configuration
	config := &models.Config{
		Security: models.SecurityConfig{
			EnableAuth: true,
			APIKeys: []models.APIKey{
				{
					Key:         "read-key-123",
					Name:        "Read Only Key",
					Permissions: []string{"read"},
					Enabled:     true,
				},
				{
					Key:         "write-key-456",
					Name:        "Write Key",
					Permissions: []string{"write"},
					Enabled:     true,
				},
				{
					Key:         "admin-key-789",
					Name:        "Admin Key",
					Permissions: []string{"admin"},
					Enabled:     true,
				},
			},
			RateLimit: models.RateLimitConfig{
				Enabled: false, // Disable for tests
			},
		},
		Server: models.ServerConfig{
			CORS: models.CORSConfig{
				Enabled: false, // Disable for tests
			},
		},
	}

	// Mock handlers
	mockHandlers := &Handlers{}

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
			expectedStatus: http.StatusNotFound, // Handler not implemented, but should pass auth
			description:    "Release registration should accept write permission",
		},
		{
			name:           "protected register with admin permission",
			method:         "POST",
			path:           "/api/v1/updates/test-app/register",
			authHeader:     "Bearer admin-key-789",
			expectedStatus: http.StatusNotFound, // Handler not implemented, but should pass auth
			description:    "Release registration should accept admin permission",
		},
		{
			name:           "health check public access",
			method:         "GET",
			path:           "/health",
			authHeader:     "",
			expectedStatus: http.StatusNotFound, // Handler not implemented, but should pass auth
			description:    "Health check should be publicly accessible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte(`{"test": "data"}`)))
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
	config := &models.Config{
		Security: models.SecurityConfig{
			EnableAuth: true,
			APIKeys: []models.APIKey{
				{
					Key:         "admin-key-123",
					Name:        "Admin Key",
					Permissions: []string{"admin"},
					Enabled:     true,
				},
			},
			RateLimit: models.RateLimitConfig{
				Enabled: false, // Disable for tests
			},
		},
		Server: models.ServerConfig{
			CORS: models.CORSConfig{
				Enabled: false, // Disable for tests
			},
		},
	}

	mockHandlers := &Handlers{}
	router := SetupRoutes(mockHandlers, config)

	t.Run("SQL Injection Protection", func(t *testing.T) {
		maliciousInputs := []string{
			"'; DROP TABLE releases; --",
			"test' OR '1'='1",
			"test'; DELETE FROM applications; --",
			"test' UNION SELECT * FROM users --",
		}

		for _, maliciousInput := range maliciousInputs {
			path := fmt.Sprintf("/api/v1/updates/%s/check", maliciousInput)
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

// TestRateLimiting tests the rate limiting functionality
func TestRateLimiting(t *testing.T) {
	config := &models.Config{
		Security: models.SecurityConfig{
			EnableAuth: false, // Disable auth for rate limit testing
			RateLimit: models.RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 5, // Very low limit for testing
			},
		},
		Server: models.ServerConfig{
			CORS: models.CORSConfig{
				Enabled: false,
			},
		},
	}

	mockHandlers := &Handlers{}
	router := SetupRoutes(mockHandlers, config)

	// Simulate multiple requests from the same IP
	clientIP := "192.168.1.100"

	var rateLimitHit bool
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/api/v1/updates/test/check", nil)
		req.RemoteAddr = clientIP + ":12345"
		req.Header.Set("X-Forwarded-For", clientIP)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code == http.StatusTooManyRequests {
			rateLimitHit = true
			break
		}
	}

	assert.True(t, rateLimitHit, "Rate limiting should trigger after exceeding limit")
}

// TestSecurityHeaders tests that appropriate security headers are set
func TestSecurityHeaders(t *testing.T) {
	config := &models.Config{
		Security: models.SecurityConfig{
			EnableAuth: false,
			RateLimit: models.RateLimitConfig{
				Enabled: false,
			},
		},
		Server: models.ServerConfig{
			CORS: models.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Authorization", "Content-Type"},
				MaxAge:         86400,
			},
		},
	}

	mockHandlers := &Handlers{}
	router := SetupRoutes(mockHandlers, config)

	t.Run("CORS Headers", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/api/v1/updates/test/check", nil)
		req.Header.Set("Origin", "https://example.com")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Equal(t, "https://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), "GET")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Authorization")
		assert.Equal(t, "86400", rr.Header().Get("Access-Control-Max-Age"))
	})

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
	securityConfig := models.SecurityConfig{
		EnableAuth: true,
		APIKeys: []models.APIKey{
			{
				Key:         "benchmark-key-123",
				Name:        "Benchmark Key",
				Permissions: []string{"read"},
				Enabled:     true,
			},
		},
	}

	middleware := authMiddleware(securityConfig)
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
			Key:         "test-key",
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
	securityConfig := models.SecurityConfig{
		EnableAuth: true,
		APIKeys: []models.APIKey{
			{
				Key:         "valid-key-123",
				Name:        "Test Key",
				Permissions: []string{"read"},
				Enabled:     true,
			},
		},
	}

	middleware := OptionalAuth(securityConfig)

	tests := []struct {
		name               string
		authHeader         string
		expectAPIKeyInCtx  bool
	}{
		{
			name:               "valid API key sets context",
			authHeader:         "Bearer valid-key-123",
			expectAPIKeyInCtx:  true,
		},
		{
			name:               "no auth header continues without context",
			authHeader:         "",
			expectAPIKeyInCtx:  false,
		},
		{
			name:               "invalid format continues without context",
			authHeader:         "InvalidFormat",
			expectAPIKeyInCtx:  false,
		},
		{
			name:               "invalid key continues without context",
			authHeader:         "Bearer invalid-key",
			expectAPIKeyInCtx:  false,
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