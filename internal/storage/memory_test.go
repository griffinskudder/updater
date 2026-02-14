package storage

import (
	"context"
	"fmt"
	"testing"
	"time"
	"updater/internal/models"
)

func TestMemoryStorage(t *testing.T) {
	storage, err := NewMemoryStorage(Config{})
	if err != nil {
		t.Fatalf("Failed to create memory storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Test application operations
	t.Run("Application Operations", func(t *testing.T) {
		// Test empty applications list
		apps, err := storage.Applications(ctx)
		if err != nil {
			t.Errorf("Failed to get applications: %v", err)
		}
		if len(apps) != 0 {
			t.Errorf("Expected 0 applications, got %d", len(apps))
		}

		// Test save application
		app := &models.Application{
			ID:          "test-app",
			Name:        "Test Application",
			Description: "A test application",
			Platforms:   []string{"windows"},
			CreatedAt:   time.Now().Format(time.RFC3339),
			UpdatedAt:   time.Now().Format(time.RFC3339),
		}

		err = storage.SaveApplication(ctx, app)
		if err != nil {
			t.Errorf("Failed to save application: %v", err)
		}

		// Test get application
		retrievedApp, err := storage.GetApplication(ctx, "test-app")
		if err != nil {
			t.Errorf("Failed to get application: %v", err)
		}
		if retrievedApp.ID != "test-app" {
			t.Errorf("Expected ID 'test-app', got '%s'", retrievedApp.ID)
		}
		if retrievedApp.Name != "Test Application" {
			t.Errorf("Expected name 'Test Application', got '%s'", retrievedApp.Name)
		}

		// Test applications list
		apps, err = storage.Applications(ctx)
		if err != nil {
			t.Errorf("Failed to get applications: %v", err)
		}
		if len(apps) != 1 {
			t.Errorf("Expected 1 application, got %d", len(apps))
		}

		// Test get non-existent application
		_, err = storage.GetApplication(ctx, "non-existent")
		if err == nil {
			t.Error("Expected error for non-existent application")
		}
	})

	// Test release operations
	t.Run("Release Operations", func(t *testing.T) {
		// Test empty releases list
		releases, err := storage.Releases(ctx, "test-app")
		if err != nil {
			t.Errorf("Failed to get releases: %v", err)
		}
		if len(releases) != 0 {
			t.Errorf("Expected 0 releases, got %d", len(releases))
		}

		// Test save release
		releaseDate := time.Now()
		release := &models.Release{
			ApplicationID: "test-app",
			Version:       "1.0.0",
			Platform:      "windows",
			Architecture:  "amd64",
			DownloadURL:   "https://example.com/app-1.0.0-windows-amd64.zip",
			Checksum:      "abcdef1234567890",
			ChecksumType:  "sha256",
			FileSize:      1024000,
			ReleaseNotes:  "Initial release",
			ReleaseDate:   releaseDate,
			Required:      false,
		}

		err = storage.SaveRelease(ctx, release)
		if err != nil {
			t.Errorf("Failed to save release: %v", err)
		}

		// Test get release
		retrievedRelease, err := storage.GetRelease(ctx, "test-app", "1.0.0", "windows", "amd64")
		if err != nil {
			t.Errorf("Failed to get release: %v", err)
		}
		if retrievedRelease.Version != "1.0.0" {
			t.Errorf("Expected version '1.0.0', got '%s'", retrievedRelease.Version)
		}

		// Test releases list
		releases, err = storage.Releases(ctx, "test-app")
		if err != nil {
			t.Errorf("Failed to get releases: %v", err)
		}
		if len(releases) != 1 {
			t.Errorf("Expected 1 release, got %d", len(releases))
		}

		// Test save additional releases
		release2Date := time.Now().Add(24 * time.Hour)
		release2 := &models.Release{
			ApplicationID: "test-app",
			Version:       "1.1.0",
			Platform:      "windows",
			Architecture:  "amd64",
			DownloadURL:   "https://example.com/app-1.1.0-windows-amd64.zip",
			Checksum:      "1234567890abcdef",
			ChecksumType:  "sha256",
			FileSize:      1124000,
			ReleaseNotes:  "Bug fixes and improvements",
			ReleaseDate:   release2Date,
			Required:      false,
		}

		err = storage.SaveRelease(ctx, release2)
		if err != nil {
			t.Errorf("Failed to save release: %v", err)
		}

		// Test get latest release
		latest, err := storage.GetLatestRelease(ctx, "test-app", "windows", "amd64")
		if err != nil {
			t.Errorf("Failed to get latest release: %v", err)
		}
		if latest.Version != "1.1.0" {
			t.Errorf("Expected latest version '1.1.0', got '%s'", latest.Version)
		}

		// Test get releases after version
		newer, err := storage.GetReleasesAfterVersion(ctx, "test-app", "1.0.0", "windows", "amd64")
		if err != nil {
			t.Errorf("Failed to get releases after version: %v", err)
		}
		if len(newer) != 1 {
			t.Errorf("Expected 1 newer release, got %d", len(newer))
		}
		if newer[0].Version != "1.1.0" {
			t.Errorf("Expected newer version '1.1.0', got '%s'", newer[0].Version)
		}

		// Test delete release
		err = storage.DeleteRelease(ctx, "test-app", "1.0.0", "windows", "amd64")
		if err != nil {
			t.Errorf("Failed to delete release: %v", err)
		}

		// Verify deletion
		releases, err = storage.Releases(ctx, "test-app")
		if err != nil {
			t.Errorf("Failed to get releases: %v", err)
		}
		if len(releases) != 1 {
			t.Errorf("Expected 1 release after deletion, got %d", len(releases))
		}

		// Test delete non-existent release
		err = storage.DeleteRelease(ctx, "test-app", "non-existent", "windows", "amd64")
		if err == nil {
			t.Error("Expected error for deleting non-existent release")
		}
	})
}

func TestMemoryStorageConcurrency(t *testing.T) {
	storage, err := NewMemoryStorage(Config{})
	if err != nil {
		t.Fatalf("Failed to create memory storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Test concurrent access
	app := &models.Application{
		ID:          "concurrent-test",
		Name:        "Concurrent Test",
		Description: "Testing concurrent access",
		CreatedAt:   time.Now().Format(time.RFC3339),
		UpdatedAt:   time.Now().Format(time.RFC3339),
	}

	// Save initial application
	err = storage.SaveApplication(ctx, app)
	if err != nil {
		t.Errorf("Failed to save application: %v", err)
	}

	// Concurrent reads and writes
	done := make(chan bool, 10)

	// Start multiple readers
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				_, err := storage.GetApplication(ctx, "concurrent-test")
				if err != nil {
					t.Errorf("Failed to get application in goroutine: %v", err)
					return
				}
			}
		}()
	}

	// Start multiple writers
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				updatedApp := *app
				updatedApp.Description = fmt.Sprintf("Updated by goroutine %d iteration %d", id, j)
				err := storage.SaveApplication(ctx, &updatedApp)
				if err != nil {
					t.Errorf("Failed to save application in goroutine: %v", err)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
