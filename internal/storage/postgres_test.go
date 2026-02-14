package storage

import (
	"context"
	"os"
	"testing"
	"updater/internal/models"
)

func TestPostgresStorage(t *testing.T) {
	// Skip if no PostgreSQL test database is available
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set, skipping PostgreSQL tests")
	}

	config := Config{
		Type:             "postgres",
		ConnectionString: dsn,
	}

	// Create storage instance
	storage, err := NewPostgresStorage(config)
	if err != nil {
		t.Fatalf("Failed to create PostgreSQL storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	t.Run("Basic Connection", func(t *testing.T) {
		// Test basic connection works
		if storage == nil {
			t.Error("Storage should not be nil")
		}
	})

	t.Run("Placeholder Methods", func(t *testing.T) {
		// Test that placeholder methods return appropriate responses
		apps, err := storage.Applications(ctx)
		if err != nil {
			t.Errorf("Applications() should not error: %v", err)
		}
		if apps == nil {
			t.Error("Applications() should return empty slice, not nil")
		}

		releases, err := storage.Releases(ctx, "test-app")
		if err != nil {
			t.Errorf("Releases() should not error: %v", err)
		}
		if releases == nil {
			t.Error("Releases() should return empty slice, not nil")
		}

		// These methods should return errors indicating they're not implemented
		_, err = storage.GetApplication(ctx, "test")
		if err == nil {
			t.Error("GetApplication() should return error for unimplemented method")
		}

		err = storage.SaveApplication(ctx, &models.Application{})
		if err == nil {
			t.Error("SaveApplication() should return error for unimplemented method")
		}
	})
}

func TestPostgresStorageErrors(t *testing.T) {
	t.Run("Invalid Connection String", func(t *testing.T) {
		config := Config{
			Type:             "postgres",
			ConnectionString: "",
		}

		_, err := NewPostgresStorage(config)
		if err == nil {
			t.Error("Expected error with empty connection string")
		}
	})

	t.Run("Invalid Database Connection", func(t *testing.T) {
		config := Config{
			Type:             "postgres",
			ConnectionString: "postgres://invalid:invalid@localhost:5432/invalid",
		}

		_, err := NewPostgresStorage(config)
		if err == nil {
			t.Error("Expected error with invalid connection string")
		}
	})
}
