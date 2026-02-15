package storage

import (
	"context"
	"os"
	"testing"
	"time"
	"updater/internal/models"
)

func getPostgresDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set, skipping PostgreSQL tests")
	}
	return dsn
}

func newPostgresTestStorage(t *testing.T) Storage {
	t.Helper()
	dsn := getPostgresDSN(t)
	s, err := NewPostgresStorage(Config{ConnectionString: dsn})
	if err != nil {
		t.Fatalf("failed to create postgres storage: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPostgresStorageConnectionError(t *testing.T) {
	_, err := NewPostgresStorage(Config{ConnectionString: ""})
	if err == nil {
		t.Error("expected error for empty connection string")
	}
}

func TestPostgresStorageInvalidDSN(t *testing.T) {
	_, err := NewPostgresStorage(Config{ConnectionString: "postgres://invalid:5432/nonexistent"})
	if err == nil {
		t.Error("expected error for invalid DSN")
	}
}

func TestPostgresStorageApplicationCRUD(t *testing.T) {
	s := newPostgresTestStorage(t)
	ctx := context.Background()

	// Create application
	app := models.NewApplication("pg-test-app", "PostgreSQL Test App", []string{"windows", "linux"})
	app.Description = "A test application for PostgreSQL"

	err := s.SaveApplication(ctx, app)
	if err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	// Get application
	got, err := s.GetApplication(ctx, "pg-test-app")
	if err != nil {
		t.Fatalf("GetApplication failed: %v", err)
	}
	if got.Name != "PostgreSQL Test App" {
		t.Errorf("expected name 'PostgreSQL Test App', got %q", got.Name)
	}
	if got.Description != "A test application for PostgreSQL" {
		t.Errorf("expected description, got %q", got.Description)
	}

	// Update application
	app.Name = "Updated PG App"
	err = s.SaveApplication(ctx, app)
	if err != nil {
		t.Fatalf("SaveApplication (update) failed: %v", err)
	}

	got, err = s.GetApplication(ctx, "pg-test-app")
	if err != nil {
		t.Fatalf("GetApplication after update failed: %v", err)
	}
	if got.Name != "Updated PG App" {
		t.Errorf("expected updated name, got %q", got.Name)
	}

	// List applications
	apps, err := s.Applications(ctx)
	if err != nil {
		t.Fatalf("Applications failed: %v", err)
	}
	found := false
	for _, a := range apps {
		if a.ID == "pg-test-app" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find pg-test-app in applications list")
	}

	// Get non-existent application
	_, err = s.GetApplication(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent application")
	}
}

func TestPostgresStorageReleaseCRUD(t *testing.T) {
	s := newPostgresTestStorage(t)
	ctx := context.Background()

	// Create application first
	app := models.NewApplication("pg-rel-app", "PG Release App", []string{"windows"})
	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	// Create release
	release := models.NewRelease("pg-rel-app", "1.0.0", "windows", "amd64", "https://example.com/v1.0.0")
	release.Checksum = "abc123"
	release.FileSize = 1024
	release.ReleaseNotes = "Initial release"
	release.Metadata = map[string]string{"build": "123"}

	err := s.SaveRelease(ctx, release)
	if err != nil {
		t.Fatalf("SaveRelease failed: %v", err)
	}

	// Get release
	got, err := s.GetRelease(ctx, "pg-rel-app", "1.0.0", "windows", "amd64")
	if err != nil {
		t.Fatalf("GetRelease failed: %v", err)
	}
	if got.DownloadURL != "https://example.com/v1.0.0" {
		t.Errorf("expected download URL, got %q", got.DownloadURL)
	}
	if got.ReleaseNotes != "Initial release" {
		t.Errorf("expected release notes, got %q", got.ReleaseNotes)
	}

	// Update release
	release.DownloadURL = "https://example.com/v1.0.0-updated"
	release.ReleaseNotes = "Updated release"
	err = s.SaveRelease(ctx, release)
	if err != nil {
		t.Fatalf("SaveRelease (update) failed: %v", err)
	}

	got, err = s.GetRelease(ctx, "pg-rel-app", "1.0.0", "windows", "amd64")
	if err != nil {
		t.Fatalf("GetRelease after update failed: %v", err)
	}
	if got.DownloadURL != "https://example.com/v1.0.0-updated" {
		t.Errorf("expected updated URL, got %q", got.DownloadURL)
	}

	// List releases
	releases, err := s.Releases(ctx, "pg-rel-app")
	if err != nil {
		t.Fatalf("Releases failed: %v", err)
	}
	if len(releases) == 0 {
		t.Error("expected at least one release")
	}

	// Delete release
	err = s.DeleteRelease(ctx, "pg-rel-app", "1.0.0", "windows", "amd64")
	if err != nil {
		t.Fatalf("DeleteRelease failed: %v", err)
	}

	_, err = s.GetRelease(ctx, "pg-rel-app", "1.0.0", "windows", "amd64")
	if err == nil {
		t.Error("expected error after deletion")
	}

	// Delete non-existent release
	err = s.DeleteRelease(ctx, "pg-rel-app", "9.9.9", "windows", "amd64")
	if err == nil {
		t.Error("expected error for deleting non-existent release")
	}
}

func TestPostgresStorageLatestRelease(t *testing.T) {
	s := newPostgresTestStorage(t)
	ctx := context.Background()

	app := models.NewApplication("pg-latest-app", "PG Latest App", []string{"linux"})
	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	// Create multiple releases
	versions := []string{"1.0.0", "2.0.0", "1.5.0"}
	for _, v := range versions {
		r := models.NewRelease("pg-latest-app", v, "linux", "amd64", "https://example.com/"+v)
		r.Checksum = "checksum-" + v
		r.FileSize = 1024
		r.ReleaseDate = time.Now()
		if err := s.SaveRelease(ctx, r); err != nil {
			t.Fatalf("SaveRelease %s failed: %v", v, err)
		}
	}

	latest, err := s.GetLatestRelease(ctx, "pg-latest-app", "linux", "amd64")
	if err != nil {
		t.Fatalf("GetLatestRelease failed: %v", err)
	}
	if latest.Version != "2.0.0" {
		t.Errorf("expected latest version 2.0.0, got %s", latest.Version)
	}

	// No releases for different platform
	_, err = s.GetLatestRelease(ctx, "pg-latest-app", "windows", "amd64")
	if err == nil {
		t.Error("expected error for platform with no releases")
	}
}

func TestPostgresStorageReleasesAfterVersion(t *testing.T) {
	s := newPostgresTestStorage(t)
	ctx := context.Background()

	app := models.NewApplication("pg-after-app", "PG After App", []string{"darwin"})
	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	versions := []string{"1.0.0", "1.5.0", "2.0.0", "3.0.0"}
	for _, v := range versions {
		r := models.NewRelease("pg-after-app", v, "darwin", "arm64", "https://example.com/"+v)
		r.Checksum = "checksum-" + v
		r.FileSize = 1024
		r.ReleaseDate = time.Now()
		if err := s.SaveRelease(ctx, r); err != nil {
			t.Fatalf("SaveRelease %s failed: %v", v, err)
		}
	}

	newer, err := s.GetReleasesAfterVersion(ctx, "pg-after-app", "1.5.0", "darwin", "arm64")
	if err != nil {
		t.Fatalf("GetReleasesAfterVersion failed: %v", err)
	}
	if len(newer) != 2 {
		t.Errorf("expected 2 newer releases, got %d", len(newer))
	}
	if len(newer) > 0 && newer[0].Version != "3.0.0" {
		t.Errorf("expected first result to be 3.0.0, got %s", newer[0].Version)
	}

	// Invalid version
	_, err = s.GetReleasesAfterVersion(ctx, "pg-after-app", "invalid", "darwin", "arm64")
	if err == nil {
		t.Error("expected error for invalid version")
	}
}

func TestPostgresStorage_DeleteApplication(t *testing.T) {
	s := newPostgresTestStorage(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func()
		appID   string
		wantErr bool
	}{
		{
			name: "delete existing application",
			setup: func() {
				app := models.NewApplication("pg-del-app", "PG Delete App", []string{"windows"})
				if err := s.SaveApplication(ctx, app); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			},
			appID:   "pg-del-app",
			wantErr: false,
		},
		{
			name:    "delete non-existent application",
			setup:   func() {},
			appID:   "pg-non-existent",
			wantErr: false, // SQL DELETE with no matching rows does not return an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			err := s.DeleteApplication(ctx, tt.appID)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify application is gone
			_, err = s.GetApplication(ctx, tt.appID)
			if err == nil {
				t.Error("expected error when getting deleted application")
			}
		})
	}
}
