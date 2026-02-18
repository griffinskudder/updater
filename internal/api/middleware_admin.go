package api

import (
	"context"
	"net/http"
	"strings"
	"updater/internal/models"
	"updater/internal/storage"

	"github.com/gorilla/mux"
)

// adminSessionMiddleware validates the HttpOnly admin_session cookie.
// Requests to /admin/login and /admin/logout are always passed through.
func adminSessionMiddleware(store storage.Storage, enableAuth bool) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Login and logout pages are exempt.
			if strings.HasSuffix(r.URL.Path, "/login") || strings.HasSuffix(r.URL.Path, "/logout") {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("admin_session")
			if err != nil || !isValidAdminKey(r.Context(), cookie.Value, store, enableAuth) {
				// Clear any stale cookie.
				http.SetCookie(w, &http.Cookie{
					Name:     "admin_session",
					Value:    "",
					Path:     "/admin",
					MaxAge:   -1,
					HttpOnly: true,
					Secure:   true,
					SameSite: http.SameSiteStrictMode,
				})
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isValidAdminKey returns true when key is authorised to access the admin UI.
// If enableAuth is false (dev mode), any non-empty key is accepted.
// Otherwise the key is looked up in storage and must have the "admin" permission.
func isValidAdminKey(ctx context.Context, key string, store storage.Storage, enableAuth bool) bool {
	if key == "" {
		return false
	}
	if !enableAuth {
		// Dev mode: any non-empty key is accepted when auth is disabled.
		return true
	}
	hash := models.HashAPIKey(key)
	ak, err := store.GetAPIKeyByHash(ctx, hash)
	if err != nil || !ak.Enabled {
		return false
	}
	return ak.HasPermission("admin")
}
