package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"updater/internal/models"
	"updater/internal/storage"

	"github.com/gorilla/mux"
)

// Permission represents the different permission levels
type Permission string

const (
	PermissionRead  Permission = "read"
	PermissionWrite Permission = "write"
	PermissionAdmin Permission = "admin"
)

// contextKey is an unexported type for context keys in this package.
// Using a named type prevents collisions with context keys from other packages.
type contextKey struct{}

// apiKeyContextKey is the context key used to store and retrieve the authenticated API key.
var apiKeyContextKey = contextKey{}

// GetAPIKey extracts the authenticated API key from request context.
// Returns nil if no key is present (unauthenticated request).
func GetAPIKey(r *http.Request) *models.APIKey {
	if apiKey, ok := r.Context().Value(apiKeyContextKey).(*models.APIKey); ok {
		return apiKey
	}
	return nil
}

// RequirePermission creates middleware that enforces a specific permission
func RequirePermission(required Permission) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := GetAPIKey(r)
			if apiKey == nil || !apiKey.HasPermission(string(required)) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				errorResp := models.NewErrorResponse(
					"Insufficient permissions for this operation",
					models.ErrorCodeForbidden,
				)
				json.NewEncoder(w).Encode(errorResp)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// OptionalAuth creates middleware that allows optional authentication.
// Used for endpoints that provide different data based on auth status.
// On any error, the request continues without authentication.
func OptionalAuth(store storage.Storage) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for API key in Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// No auth provided, continue without security context
				next.ServeHTTP(w, r)
				return
			}

			// Try to authenticate, but don't fail if invalid
			const prefix = "Bearer "
			if !strings.HasPrefix(authHeader, prefix) {
				// Invalid format, continue without auth
				next.ServeHTTP(w, r)
				return
			}

			token := authHeader[len(prefix):]

			hash := models.HashAPIKey(token)
			validKey, err := store.GetAPIKeyByHash(r.Context(), hash)
			if err != nil || !validKey.Enabled {
				// Invalid or disabled key, continue without auth
				next.ServeHTTP(w, r)
				return
			}

			// Add API key info to context for handlers to use
			ctx := context.WithValue(r.Context(), apiKeyContextKey, validKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
