package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"updater/internal/api"
	"updater/internal/config"
	"updater/internal/logger"
	"updater/internal/models"
	"updater/internal/observability"
	"updater/internal/ratelimit"
	"updater/internal/storage"
	"updater/internal/update"
)

var configFile = flag.String("config", "", "Path to configuration file")

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize structured logging
	log, closer, err := logger.Setup(cfg.Logging)
	if err != nil {
		slog.Error("Failed to initialize logger", "error", err)
		os.Exit(1)
	}
	if closer != nil {
		defer closer.Close()
	}
	slog.SetDefault(log)

	// Initialize observability (OpenTelemetry)
	otelProvider, err := observability.Setup(cfg.Metrics, cfg.Observability)
	if err != nil {
		slog.Error("Failed to initialize observability", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := otelProvider.Shutdown(shutdownCtx); err != nil {
			slog.Error("Failed to shutdown observability", "error", err)
		}
	}()

	// Initialize storage
	storageInstance, err := initializeStorage(cfg)
	if err != nil {
		slog.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer storageInstance.Close()

	// Wrap storage with instrumentation if metrics are enabled
	var activeStorage storage.Storage = storageInstance
	if cfg.Metrics.Enabled {
		instrumented, err := observability.NewInstrumentedStorage(storageInstance)
		if err != nil {
			slog.Error("Failed to create instrumented storage", "error", err)
			os.Exit(1)
		}
		activeStorage = instrumented
	}

	if err := seedBootstrapKey(context.Background(), activeStorage, cfg); err != nil {
		slog.Error("Failed to seed bootstrap key", "error", err)
		os.Exit(1)
	}

	// Initialize update service
	updateService := update.NewService(activeStorage)

	// Parse admin UI templates from embedded FS.
	adminTmpl, err := api.ParseAdminTemplates()
	if err != nil {
		slog.Error("Failed to parse admin templates", "error", err)
		os.Exit(1)
	}

	// Initialize HTTP handlers with storage for health checks
	handlers := api.NewHandlers(updateService,
		api.WithStorage(activeStorage),
		api.WithAdminTemplates(adminTmpl),
		api.WithSecurityConfig(cfg.Security),
	)

	// Setup routes with middleware
	routeOpts := []api.RouteOption{}
	if cfg.Observability.Tracing.Enabled {
		routeOpts = append(routeOpts, api.WithOTelMiddleware(cfg.Observability.ServiceName))
	}

	// Initialize rate limiter if enabled
	if cfg.Security.RateLimit.Enabled {
		rlCfg := cfg.Security.RateLimit

		// Default authenticated values to 2x anonymous if not set
		authRPM := rlCfg.AuthenticatedRequestsPerMinute
		if authRPM == 0 {
			authRPM = rlCfg.RequestsPerMinute * 2
		}
		authBurst := rlCfg.AuthenticatedBurstSize
		if authBurst == 0 {
			authBurst = rlCfg.BurstSize * 2
		}

		anonLimiter := ratelimit.NewMemoryLimiter(rlCfg.RequestsPerMinute, rlCfg.BurstSize, rlCfg.CleanupInterval)
		authLimiter := ratelimit.NewMemoryLimiter(authRPM, authBurst, rlCfg.CleanupInterval)
		defer anonLimiter.Close()
		defer authLimiter.Close()

		routeOpts = append(routeOpts, api.WithRateLimiter(ratelimit.Middleware(anonLimiter, authLimiter)))
	}

	router := api.SetupRoutes(handlers, cfg, routeOpts...)

	// Start metrics server if enabled
	var metricsServer *observability.MetricsServer
	if cfg.Metrics.Enabled {
		metricsServer = observability.NewMetricsServer(cfg.Metrics.Port, cfg.Metrics.Path, otelProvider)
		go func() {
			if err := metricsServer.Start(); err != nil && err != http.ErrServerClosed {
				slog.Error("Metrics server failed", "error", err)
			}
		}()
	}

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("Starting server", "addr", server.Addr)

		var err error
		if cfg.Server.TLSEnabled {
			if cfg.Server.TLSCertFile == "" || cfg.Server.TLSKeyFile == "" {
				slog.Error("TLS is enabled but cert file or key file is not specified")
				os.Exit(1)
			}
			slog.Info("Starting HTTPS server with TLS")
			err = server.ListenAndServeTLS(cfg.Server.TLSCertFile, cfg.Server.TLSKeyFile)
		} else {
			slog.Info("Starting HTTP server")
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server")

	// Create a deadline to wait for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown metrics server
	if metricsServer != nil {
		if err := metricsServer.Shutdown(ctx); err != nil {
			slog.Error("Metrics server forced to shutdown", "error", err)
		}
	}

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server shutdown complete")
}

// initializeStorage creates and returns a storage instance based on configuration
func initializeStorage(cfg *models.Config) (storage.Storage, error) {
	storageConfig := storage.Config{
		Type:             cfg.Storage.Type,
		Path:             cfg.Storage.Path,
		ConnectionString: cfg.Storage.Database.DSN,
		CacheTTL:         cfg.Cache.TTL.String(),
	}

	switch cfg.Storage.Type {
	case "json":
		return storage.NewJSONStorage(storageConfig)
	case "memory":
		return storage.NewMemoryStorage(storageConfig)
	case "postgres":
		return storage.NewPostgresStorage(storageConfig)
	case "sqlite":
		return storage.NewSQLiteStorage(storageConfig)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Storage.Type)
	}
}

// seedBootstrapKey inserts the configured bootstrap key into storage if it
// does not already exist. It is a no-op when BootstrapKey is empty.
func seedBootstrapKey(ctx context.Context, store storage.Storage, cfg *models.Config) error {
	raw := cfg.Security.BootstrapKey
	if raw == "" {
		return nil
	}
	hash := models.HashAPIKey(raw)
	if _, err := store.GetAPIKeyByHash(ctx, hash); err == nil {
		// Already seeded - idempotent.
		return nil
	}
	key := models.NewAPIKey(models.NewKeyID(), "bootstrap", raw, []string{"admin"})
	if err := store.CreateAPIKey(ctx, key); err != nil {
		return fmt.Errorf("seed bootstrap key: %w", err)
	}
	slog.Info("bootstrap API key seeded", "id", key.ID, "prefix", key.Prefix)
	return nil
}
