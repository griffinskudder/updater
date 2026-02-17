package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"updater/internal/models"
	"updater/internal/storage"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
)

// RouteOption configures optional route behavior.
type RouteOption func(*mux.Router)

// WithOTelMiddleware adds OpenTelemetry HTTP instrumentation middleware.
func WithOTelMiddleware(serviceName string) RouteOption {
	return func(r *mux.Router) {
		r.Use(otelmux.Middleware(serviceName,
			otelmux.WithFilter(func(r *http.Request) bool {
				return r.URL.Path != "/health" &&
					r.URL.Path != "/api/v1/health" &&
					r.URL.Path != "/metrics" &&
					r.URL.Path != "/api/v1/openapi.yaml" &&
					r.URL.Path != "/api/v1/docs"
			}),
		))
	}
}

// SetupRoutes configures the HTTP routes for the API
func SetupRoutes(handlers *Handlers, config *models.Config, opts ...RouteOption) *mux.Router {
	router := mux.NewRouter()

	for _, opt := range opts {
		opt(router)
	}

	api := router.PathPrefix("/api/v1").Subrouter()

	publicAPI := api.PathPrefix("").Subrouter()
	publicAPI.HandleFunc("/updates/{app_id}/check", handlers.CheckForUpdates).Methods("GET")
	publicAPI.HandleFunc("/updates/{app_id}/latest", handlers.GetLatestVersion).Methods("GET")
	publicAPI.HandleFunc("/check", handlers.CheckForUpdates).Methods("POST")
	publicAPI.HandleFunc("/check", methodNotAllowedHandler).Methods("GET", "PUT", "DELETE", "PATCH")
	publicAPI.HandleFunc("/latest", handlers.GetLatestVersion).Methods("GET")

	api.HandleFunc("/openapi.yaml", handlers.ServeOpenAPISpec).Methods("GET")
	api.HandleFunc("/docs", handlers.ServeSwaggerUI).Methods("GET")

	// Admin UI - cookie-authenticated; middleware skips /login and /logout.
	adminRouter := router.PathPrefix("/admin").Subrouter()
	adminRouter.Use(adminSessionMiddleware(handlers.storage, config.Security.EnableAuth))
	adminRouter.HandleFunc("/login", handlers.AdminLogin).Methods("GET", "POST")
	adminRouter.HandleFunc("/logout", handlers.AdminLogout).Methods("POST")
	adminRouter.HandleFunc("", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/applications", http.StatusSeeOther)
	}).Methods("GET")
	adminRouter.HandleFunc("/health", handlers.AdminHealth).Methods("GET")
	adminRouter.HandleFunc("/applications", handlers.AdminListApplications).Methods("GET")
	adminRouter.HandleFunc("/applications/new", handlers.AdminNewApplicationForm).Methods("GET")
	adminRouter.HandleFunc("/applications", handlers.AdminCreateApplication).Methods("POST")
	adminRouter.HandleFunc("/applications/{app_id}", handlers.AdminGetApplication).Methods("GET")
	adminRouter.HandleFunc("/applications/{app_id}/edit", handlers.AdminEditApplicationForm).Methods("GET")
	adminRouter.HandleFunc("/applications/{app_id}/edit", handlers.AdminUpdateApplication).Methods("POST")
	adminRouter.HandleFunc("/applications/{app_id}", handlers.AdminDeleteApplication).Methods("DELETE")
	adminRouter.HandleFunc("/applications/{app_id}/releases/new", handlers.AdminNewReleaseForm).Methods("GET")
	adminRouter.HandleFunc("/applications/{app_id}/releases", handlers.AdminCreateRelease).Methods("POST")
	adminRouter.HandleFunc("/applications/{app_id}/releases/{version}/{platform}/{arch}",
		handlers.AdminDeleteRelease).Methods("DELETE")

	router.HandleFunc("/health", handlers.HealthCheck).Methods("GET")
	router.HandleFunc("/api/v1/health", handlers.HealthCheck).Methods("GET")

	api.PathPrefix("").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}).Methods("OPTIONS")

	if config.Server.CORS.Enabled {
		router.Use(corsMiddleware(config.Server.CORS))
	}

	router.Use(loggingMiddleware)
	router.Use(recoveryMiddleware)

	router.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		errorResp := models.NewErrorResponse("Method not allowed", models.ErrorCodeInvalidRequest)
		json.NewEncoder(w).Encode(errorResp)
	})

	if config.Security.EnableAuth {
		readAPI := api.PathPrefix("").Subrouter()
		readAPI.Use(authMiddleware(handlers.storage))
		readAPI.Use(RequirePermission(PermissionRead))
		readAPI.HandleFunc("/updates/{app_id}/releases", handlers.ListReleases).Methods("GET")

		writeAPI := api.PathPrefix("").Subrouter()
		writeAPI.Use(authMiddleware(handlers.storage))
		writeAPI.Use(RequirePermission(PermissionWrite))
		writeAPI.HandleFunc("/updates/{app_id}/register", handlers.RegisterRelease).Methods("POST")

		appReadAPI := api.PathPrefix("/applications").Subrouter()
		appReadAPI.Use(authMiddleware(handlers.storage))
		appReadAPI.Use(RequirePermission(PermissionRead))
		appReadAPI.HandleFunc("", handlers.ListApplications).Methods("GET")
		appReadAPI.HandleFunc("/{app_id}", handlers.GetApplication).Methods("GET")

		appWriteAPI := api.PathPrefix("/applications").Subrouter()
		appWriteAPI.Use(authMiddleware(handlers.storage))
		appWriteAPI.Use(RequirePermission(PermissionWrite))
		appWriteAPI.HandleFunc("", handlers.CreateApplication).Methods("POST")

		appAdminAPI := api.PathPrefix("/applications").Subrouter()
		appAdminAPI.Use(authMiddleware(handlers.storage))
		appAdminAPI.Use(RequirePermission(PermissionAdmin))
		appAdminAPI.HandleFunc("/{app_id}", handlers.UpdateApplication).Methods("PUT")
		appAdminAPI.HandleFunc("/{app_id}", handlers.DeleteApplication).Methods("DELETE")

		adminAPI := api.PathPrefix("").Subrouter()
		adminAPI.Use(authMiddleware(handlers.storage))
		adminAPI.Use(RequirePermission(PermissionAdmin))
		adminAPI.HandleFunc("/updates/{app_id}/releases/{version}/{platform}/{arch}", handlers.DeleteRelease).Methods("DELETE")

		router.Use(OptionalAuth(handlers.storage))
	} else {
		api.HandleFunc("/updates/{app_id}/releases", handlers.ListReleases).Methods("GET")
		api.HandleFunc("/updates/{app_id}/register", handlers.RegisterRelease).Methods("POST")
		api.HandleFunc("/applications", handlers.ListApplications).Methods("GET")
		api.HandleFunc("/applications/{app_id}", handlers.GetApplication).Methods("GET")
		api.HandleFunc("/applications", handlers.CreateApplication).Methods("POST")
		api.HandleFunc("/applications/{app_id}", handlers.UpdateApplication).Methods("PUT")
		api.HandleFunc("/applications/{app_id}", handlers.DeleteApplication).Methods("DELETE")
		api.HandleFunc("/updates/{app_id}/releases/{version}/{platform}/{arch}", handlers.DeleteRelease).Methods("DELETE")
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
		slog.Info("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware handles panics
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("Panic recovered", "error", err, "path", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				errorResp := models.NewErrorResponse("Internal server error", models.ErrorCodeInternalError)
				json.NewEncoder(w).Encode(errorResp)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// authMiddleware handles API key authentication using storage-backed key lookup.
func authMiddleware(store storage.Storage) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" || r.URL.Path == "/api/v1/health" {
				next.ServeHTTP(w, r)
				return
			}
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				errorResp := models.NewErrorResponse("Authorization required", models.ErrorCodeUnauthorized)
				json.NewEncoder(w).Encode(errorResp)
				return
			}
			const prefix = "Bearer "
			if !strings.HasPrefix(authHeader, prefix) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				errorResp := models.NewErrorResponse("Invalid authorization format", models.ErrorCodeUnauthorized)
				json.NewEncoder(w).Encode(errorResp)
				return
			}
			token := authHeader[len(prefix):]
			hash := models.HashAPIKey(token)
			validKey, err := store.GetAPIKeyByHash(r.Context(), hash)
			if err != nil || !validKey.Enabled {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				errorResp := models.NewErrorResponse("Invalid API key", models.ErrorCodeUnauthorized)
				json.NewEncoder(w).Encode(errorResp)
				return
			}
			ctx := context.WithValue(r.Context(), "api_key", validKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithRateLimiter adds rate limiting middleware to the router.
func WithRateLimiter(middleware func(http.Handler) http.Handler) RouteOption {
	return func(r *mux.Router) {
		r.Use(middleware)
	}
}

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
