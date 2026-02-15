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

	// Initialize storage
	storageInstance, err := initializeStorage(cfg)
	if err != nil {
		slog.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer storageInstance.Close()

	// Initialize update service
	updateService := update.NewService(storageInstance)

	// Initialize HTTP handlers
	handlers := api.NewHandlers(updateService)

	// Setup routes with middleware
	router := api.SetupRoutes(handlers, cfg)

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
