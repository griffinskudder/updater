package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"updater/internal/api"
	"updater/internal/config"
	"updater/internal/models"
	"updater/internal/storage"
	"updater/internal/update"
)

var (
	configFile = flag.String("config", "", "Path to configuration file")
	version    = "1.0.0" // This would typically be set by build flags
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize storage
	storageInstance, err := initializeStorage(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
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
		log.Printf("Starting server on %s", server.Addr)

		var err error
		if cfg.Server.TLSEnabled {
			if cfg.Server.TLSCertFile == "" || cfg.Server.TLSKeyFile == "" {
				log.Fatal("TLS is enabled but cert file or key file is not specified")
			}
			log.Printf("Starting HTTPS server with TLS")
			err = server.ListenAndServeTLS(cfg.Server.TLSCertFile, cfg.Server.TLSKeyFile)
		} else {
			log.Printf("Starting HTTP server")
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create a deadline to wait for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server shutdown complete")
}

// initializeStorage creates and returns a storage instance based on configuration
func initializeStorage(cfg *models.Config) (storage.Storage, error) {
	storageConfig := storage.Config{
		Type:     cfg.Storage.Type,
		Path:     cfg.Storage.Path,
		CacheTTL: "5m", // Default cache TTL
	}

	switch cfg.Storage.Type {
	case "json":
		return storage.NewJSONStorage(storageConfig)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Storage.Type)
	}
}

// printVersion prints version information
func printVersion() {
	fmt.Printf("Updater Service v%s\n", version)
}