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

	// Update endpoints
	api.HandleFunc("/updates/{app_id}/check", handlers.CheckForUpdates).Methods("GET")
	api.HandleFunc("/updates/{app_id}/latest", handlers.GetLatestVersion).Methods("GET")
	api.HandleFunc("/updates/{app_id}/releases", handlers.ListReleases).Methods("GET")
	api.HandleFunc("/updates/{app_id}/register", handlers.RegisterRelease).Methods("POST")

	// Health check endpoint
	router.HandleFunc("/health", handlers.HealthCheck).Methods("GET")
	router.HandleFunc("/api/v1/health", handlers.HealthCheck).Methods("GET")

	// Add middleware
	if config.Server.CORS.Enabled {
		router.Use(corsMiddleware(config.Server.CORS))
	}

	router.Use(loggingMiddleware)
	router.Use(recoveryMiddleware)

	// Add authentication middleware if enabled
	if config.Security.EnableAuth {
		api.Use(authMiddleware(config.Security))
	}

	// Add rate limiting if enabled
	if config.Security.RateLimit.Enabled {
		router.Use(rateLimitMiddleware(config.Security.RateLimit))
	}

	return router
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

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fallback to RemoteAddr
	return r.RemoteAddr
}