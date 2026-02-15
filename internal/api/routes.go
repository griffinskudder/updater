package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"updater/internal/models"

	"github.com/gorilla/mux"
)

// SetupRoutes configures the HTTP routes for the API
func SetupRoutes(handlers *Handlers, config *models.Config) *mux.Router {
	router := mux.NewRouter()

	// API prefix
	api := router.PathPrefix("/api/v1").Subrouter()

	// Public update endpoints (no authentication required)
	publicAPI := api.PathPrefix("").Subrouter()
	publicAPI.HandleFunc("/updates/{app_id}/check", handlers.CheckForUpdates).Methods("GET")
	publicAPI.HandleFunc("/updates/{app_id}/latest", handlers.GetLatestVersion).Methods("GET")
	publicAPI.HandleFunc("/check", handlers.CheckForUpdates).Methods("POST")  // POST version with JSON body
	publicAPI.HandleFunc("/check", methodNotAllowedHandler).Methods("GET", "PUT", "DELETE", "PATCH") // Explicitly handle other methods
	publicAPI.HandleFunc("/latest", handlers.GetLatestVersion).Methods("GET")  // GET version for compatibility

	// Health check endpoint (public with optional enhanced details for authenticated users)
	router.HandleFunc("/health", handlers.HealthCheck).Methods("GET")
	router.HandleFunc("/api/v1/health", handlers.HealthCheck).Methods("GET")

	// Add OPTIONS handler for all API routes
	api.PathPrefix("").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}).Methods("OPTIONS")

	// Add middleware
	if config.Server.CORS.Enabled {
		router.Use(corsMiddleware(config.Server.CORS))
	}

	router.Use(loggingMiddleware)
	router.Use(recoveryMiddleware)

	// Add rate limiting if enabled
	if config.Security.RateLimit.Enabled {
		router.Use(rateLimitMiddleware(config.Security.RateLimit))
	}

	// Add method not allowed handler
	router.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		errorResp := models.NewErrorResponse("Method not allowed", models.ErrorCodeInvalidRequest)
		json.NewEncoder(w).Encode(errorResp)
	})

	// Apply authentication and permission middleware
	if config.Security.EnableAuth {
		// Protected endpoints with read permission
		readAPI := api.PathPrefix("").Subrouter()
		readAPI.Use(authMiddleware(config.Security))
		readAPI.Use(RequirePermission(PermissionRead))
		readAPI.HandleFunc("/updates/{app_id}/releases", handlers.ListReleases).Methods("GET")

		// Protected endpoints with write permission
		writeAPI := api.PathPrefix("").Subrouter()
		writeAPI.Use(authMiddleware(config.Security))
		writeAPI.Use(RequirePermission(PermissionWrite))
		writeAPI.HandleFunc("/updates/{app_id}/register", handlers.RegisterRelease).Methods("POST")

		// Health endpoint uses optional auth for enhanced details
		router.Use(OptionalAuth(config.Security))
	} else {
		// If auth is disabled, add endpoints without protection (for development)
		api.HandleFunc("/updates/{app_id}/releases", handlers.ListReleases).Methods("GET")
		api.HandleFunc("/updates/{app_id}/register", handlers.RegisterRelease).Methods("POST")
	}

	return router
}

// methodNotAllowedHandler handles requests with invalid HTTP methods
func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	errorResp := models.NewErrorResponse("Method not allowed", models.ErrorCodeInvalidRequest)
	json.NewEncoder(w).Encode(errorResp)
}

// corsMiddleware handles Cross-Origin Resource Sharing
func corsMiddleware(corsConfig models.CORSConfig) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			if len(corsConfig.AllowedOrigins) > 0 {
				origin := r.Header.Get("Origin")
				if origin != "" && (contains(corsConfig.AllowedOrigins, "*") || contains(corsConfig.AllowedOrigins, origin)) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}

			if len(corsConfig.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", joinStrings(corsConfig.AllowedMethods, ", "))
			}

			if len(corsConfig.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", joinStrings(corsConfig.AllowedHeaders, ", "))
			}

			if corsConfig.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", corsConfig.MaxAge))
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple request logging - in production this would use proper logging
		fmt.Printf("%s %s %s\n", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware handles panics
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("Panic recovered: %v\n", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)

				errorResp := models.NewErrorResponse("Internal server error", models.ErrorCodeInternalError)
				json.NewEncoder(w).Encode(errorResp)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// authMiddleware handles API key authentication
func authMiddleware(securityConfig models.SecurityConfig) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for health checks
			if r.URL.Path == "/health" || r.URL.Path == "/api/v1/health" {
				next.ServeHTTP(w, r)
				return
			}

			// Check for API key in Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				errorResp := models.NewErrorResponse("Authorization required", models.ErrorCodeUnauthorized)
				json.NewEncoder(w).Encode(errorResp)
				return
			}

			// Extract token (expect "Bearer <token>" format)
			const prefix = "Bearer "
			if !strings.HasPrefix(authHeader, prefix) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				errorResp := models.NewErrorResponse("Invalid authorization format", models.ErrorCodeUnauthorized)
				json.NewEncoder(w).Encode(errorResp)
				return
			}

			token := authHeader[len(prefix):]

			// Check if API key is valid
			var validKey *models.APIKey
			for _, apiKey := range securityConfig.APIKeys {
				if apiKey.Key == token && apiKey.Enabled {
					validKey = &apiKey
					break
				}
			}

			if validKey == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				errorResp := models.NewErrorResponse("Invalid API key", models.ErrorCodeUnauthorized)
				json.NewEncoder(w).Encode(errorResp)
				return
			}

			// Add API key info to context for handlers to use
			ctx := context.WithValue(r.Context(), "api_key", validKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// rateLimitMiddleware implements simple in-memory rate limiting
func rateLimitMiddleware(rateLimitConfig models.RateLimitConfig) mux.MiddlewareFunc {
	// Simple in-memory rate limiter - in production use Redis or similar
	limiter := make(map[string]time.Time)
	var mu sync.Mutex

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use IP as identifier
			clientIP := getClientIP(r)

			mu.Lock()
			lastRequest, exists := limiter[clientIP]
			now := time.Now()

			if exists && now.Sub(lastRequest) < time.Minute/time.Duration(rateLimitConfig.RequestsPerMinute) {
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				errorResp := models.NewErrorResponse("Rate limit exceeded", "RATE_LIMIT_EXCEEDED")
				json.NewEncoder(w).Encode(errorResp)
				return
			}

			limiter[clientIP] = now
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func joinStrings(slice []string, separator string) string {
	return strings.Join(slice, separator)
}

