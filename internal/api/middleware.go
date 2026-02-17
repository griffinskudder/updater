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

// SecurityContext represents the security information for a request
type SecurityContext struct {
	APIKey      *models.APIKey
	Permissions []string
}

// HasPermission checks if the security context has the required permission
func (sc *SecurityContext) HasPermission(required Permission) bool {
	if sc == nil || sc.APIKey == nil {
		return false
	}

	requiredStr := string(required)

	// Check if the API key has the exact permission
	for _, permission := range sc.APIKey.Permissions {
		if permission == requiredStr {
			return true
		}

		// Check permission hierarchy
		switch permission {
		case string(PermissionAdmin):
			// Admin permission grants access to everything
			return true
		case string(PermissionWrite):
			// Write permission includes read
			if required == PermissionRead {
				return true
			}
		}
	}

	return false
}

// GetSecurityContext extracts security context from request context
func GetSecurityContext(r *http.Request) *SecurityContext {
	if apiKey, ok := r.Context().Value("api_key").(*models.APIKey); ok {
		return &SecurityContext{
			APIKey:      apiKey,
			Permissions: apiKey.Permissions,
		}
	}
	return nil
}

// RequirePermission creates middleware that enforces a specific permission
func RequirePermission(required Permission) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			securityContext := GetSecurityContext(r)

			// Check for nil security context or insufficient permissions
			if securityContext == nil || !securityContext.HasPermission(required) {
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
			ctx := context.WithValue(r.Context(), "api_key", validKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
