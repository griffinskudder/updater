package api

import (
	"net/http"
	"strings"
	"updater/internal/models"

	"github.com/gorilla/mux"
)

// adminSessionMiddleware validates the HttpOnly admin_session cookie.
// Requests to /admin/login and /admin/logout are always passed through.
func adminSessionMiddleware(cfg models.SecurityConfig) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Login and logout pages are exempt.
			if strings.HasSuffix(r.URL.Path, "/login") || strings.HasSuffix(r.URL.Path, "/logout") {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("admin_session")
			if err != nil || !isValidAdminKey(cookie.Value, cfg) {
				// Clear any stale cookie.
				http.SetCookie(w, &http.Cookie{
					Name:   "admin_session",
					Value:  "",
					Path:   "/admin",
					MaxAge: -1,
				})
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isValidAdminKey returns true when key is authorised to access the admin UI.
// If no API keys are configured the service is in dev mode and any non-empty
// string is accepted.
func isValidAdminKey(key string, cfg models.SecurityConfig) bool {
	if key == "" {
		return false
	}
	if len(cfg.APIKeys) == 0 {
		return true // dev mode
	}
	for _, ak := range cfg.APIKeys {
		if ak.Key != key || !ak.Enabled {
			continue
		}
		for _, p := range ak.Permissions {
			if p == "admin" || p == "*" {
				return true
			}
		}
	}
	return false
}
