package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"updater/internal/models"
)

func TestSQLiteStorage(t *testing.T) {
	// Create a temporary database file
	tempDir, err := os.MkdirTemp("", "sqlite_storage_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	config := Config{
		Type:             "sqlite",
		ConnectionString: dbPath,
	}

	// Create storage instance
	storage, err := NewSQLiteStorage(config)
	if err != nil {
		t.Fatalf("Failed to create SQLite storage: %v", err)
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

func TestSQLiteStorageErrors(t *testing.T) {
	t.Run("Invalid Connection String", func(t *testing.T) {
		config := Config{
			Type:             "sqlite",
			ConnectionString: "",
		}

		_, err := NewSQLiteStorage(config)
		if err == nil {
			t.Error("Expected error with empty connection string")
		}
	})

	t.Run("Database Creation", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "sqlite_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		dbPath := filepath.Join(tempDir, "test.db")
		config := Config{
			Type:             "sqlite",
			ConnectionString: dbPath,
		}

		storage, err := NewSQLiteStorage(config)
		if err != nil {
			t.Errorf("SQLite should create database file: %v", err)
		}
		if storage != nil {
			storage.Close()
		}

		// Check if file was created
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Error("Database file should have been created")
		}
	})
}
