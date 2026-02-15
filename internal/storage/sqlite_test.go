package storage

import (
	"context"
	"sync"
	"testing"
	"time"
	"updater/internal/models"
)

func newSQLiteTestStorage(t *testing.T) Storage {
	t.Helper()
	s, err := NewSQLiteStorage(Config{ConnectionString: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteStorageConnectionError(t *testing.T) {
	_, err := NewSQLiteStorage(Config{ConnectionString: ""})
	if err == nil {
		t.Error("expected error for empty connection string")
	}
}

func TestSQLiteStorageSchemaCreation(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	// Verify tables exist by performing operations
	apps, err := s.Applications(ctx)
	if err != nil {
		t.Fatalf("Applications failed: %v", err)
	}
	if apps == nil {
		t.Error("expected non-nil slice")
	}

	releases, err := s.Releases(ctx, "test-app")
	if err != nil {
		t.Fatalf("Releases failed: %v", err)
	}
	if releases == nil {
		t.Error("expected non-nil slice")
	}
}

func TestSQLiteStorageApplicationCRUD(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	// Create application
	app := models.NewApplication("test-app", "Test Application", []string{"windows", "linux"})
	app.Description = "A test application"

	err := s.SaveApplication(ctx, app)
	if err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	// Get application
	got, err := s.GetApplication(ctx, "test-app")
	if err != nil {
		t.Fatalf("GetApplication failed: %v", err)
	}
	if got.Name != "Test Application" {
		t.Errorf("expected name 'Test Application', got %q", got.Name)
	}
	if got.Description != "A test application" {
		t.Errorf("expected description 'A test application', got %q", got.Description)
	}
	if len(got.Platforms) != 2 {
		t.Errorf("expected 2 platforms, got %d", len(got.Platforms))
	}

	// Update application
	app.Name = "Updated Application"
	app.Description = "Updated description"
	err = s.SaveApplication(ctx, app)
	if err != nil {
		t.Fatalf("SaveApplication (update) failed: %v", err)
	}

	got, err = s.GetApplication(ctx, "test-app")
	if err != nil {
		t.Fatalf("GetApplication after update failed: %v", err)
	}
	if got.Name != "Updated Application" {
		t.Errorf("expected updated name, got %q", got.Name)
	}
	if got.Description != "Updated description" {
		t.Errorf("expected updated description, got %q", got.Description)
	}

	// List applications
	apps, err := s.Applications(ctx)
	if err != nil {
		t.Fatalf("Applications failed: %v", err)
	}
	if len(apps) != 1 {
		t.Errorf("expected 1 application, got %d", len(apps))
	}

	// Create second application
	app2 := models.NewApplication("test-app-2", "Second App", []string{"darwin"})
	if err := s.SaveApplication(ctx, app2); err != nil {
		t.Fatalf("SaveApplication (second) failed: %v", err)
	}

	apps, err = s.Applications(ctx)
	if err != nil {
		t.Fatalf("Applications failed: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 applications, got %d", len(apps))
	}

	// Get non-existent application
	_, err = s.GetApplication(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent application")
	}
}

func TestSQLiteStorageReleaseCRUD(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	// Create application first
	app := models.NewApplication("rel-app", "Release App", []string{"windows"})
	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	// Create release
	release := models.NewRelease("rel-app", "1.0.0", "windows", "amd64", "https://example.com/v1.0.0")
	release.Checksum = "abc123"
	release.FileSize = 1024
	release.ReleaseNotes = "Initial release"
	release.Metadata = map[string]string{"build": "123"}

	err := s.SaveRelease(ctx, release)
	if err != nil {
		t.Fatalf("SaveRelease failed: %v", err)
	}

	// Get release
	got, err := s.GetRelease(ctx, "rel-app", "1.0.0", "windows", "amd64")
	if err != nil {
		t.Fatalf("GetRelease failed: %v", err)
	}
	if got.DownloadURL != "https://example.com/v1.0.0" {
		t.Errorf("expected download URL, got %q", got.DownloadURL)
	}
	if got.Checksum != "abc123" {
		t.Errorf("expected checksum 'abc123', got %q", got.Checksum)
	}
	if got.FileSize != 1024 {
		t.Errorf("expected file size 1024, got %d", got.FileSize)
	}
	if got.ReleaseNotes != "Initial release" {
		t.Errorf("expected release notes, got %q", got.ReleaseNotes)
	}
	if got.Metadata["build"] != "123" {
		t.Errorf("expected metadata build=123, got %v", got.Metadata)
	}

	// Update release
	release.DownloadURL = "https://example.com/v1.0.0-updated"
	release.ReleaseNotes = "Updated release"
	err = s.SaveRelease(ctx, release)
	if err != nil {
		t.Fatalf("SaveRelease (update) failed: %v", err)
	}

	got, err = s.GetRelease(ctx, "rel-app", "1.0.0", "windows", "amd64")
	if err != nil {
		t.Fatalf("GetRelease after update failed: %v", err)
	}
	if got.DownloadURL != "https://example.com/v1.0.0-updated" {
		t.Errorf("expected updated URL, got %q", got.DownloadURL)
	}

	// List releases
	releases, err := s.Releases(ctx, "rel-app")
	if err != nil {
		t.Fatalf("Releases failed: %v", err)
	}
	if len(releases) != 1 {
		t.Errorf("expected 1 release, got %d", len(releases))
	}

	// Get non-existent release
	_, err = s.GetRelease(ctx, "rel-app", "9.9.9", "windows", "amd64")
	if err == nil {
		t.Error("expected error for non-existent release")
	}

	// Delete release
	err = s.DeleteRelease(ctx, "rel-app", "1.0.0", "windows", "amd64")
	if err != nil {
		t.Fatalf("DeleteRelease failed: %v", err)
	}

	_, err = s.GetRelease(ctx, "rel-app", "1.0.0", "windows", "amd64")
	if err == nil {
		t.Error("expected error after deletion")
	}

	// Delete non-existent release
	err = s.DeleteRelease(ctx, "rel-app", "1.0.0", "windows", "amd64")
	if err == nil {
		t.Error("expected error for deleting non-existent release")
	}

	// Empty releases list
	releases, err = s.Releases(ctx, "rel-app")
	if err != nil {
		t.Fatalf("Releases failed: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("expected 0 releases after deletion, got %d", len(releases))
	}
}

func TestSQLiteStorageLatestRelease(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	app := models.NewApplication("latest-app", "Latest App", []string{"linux"})
	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	// Create multiple releases out of order
	versions := []string{"1.0.0", "2.0.0", "1.5.0", "3.0.0-beta.1"}
	for _, v := range versions {
		r := models.NewRelease("latest-app", v, "linux", "amd64", "https://example.com/"+v)
		r.Checksum = "checksum-" + v
		r.FileSize = 1024
		r.ReleaseDate = time.Now()
		if err := s.SaveRelease(ctx, r); err != nil {
			t.Fatalf("SaveRelease %s failed: %v", v, err)
		}
	}

	latest, err := s.GetLatestRelease(ctx, "latest-app", "linux", "amd64")
	if err != nil {
		t.Fatalf("GetLatestRelease failed: %v", err)
	}
	// 3.0.0-beta.1 is a pre-release and is less than 2.0.0 in semver
	// But 3.0.0-beta.1 has major version 3, so semver considers pre-releases
	// Pre-releases have lower precedence than release: 3.0.0-beta.1 < 3.0.0 but 3.0.0-beta.1 > 2.0.0
	if latest.Version != "3.0.0-beta.1" {
		t.Errorf("expected latest version 3.0.0-beta.1, got %s", latest.Version)
	}

	// No releases for different platform
	_, err = s.GetLatestRelease(ctx, "latest-app", "windows", "amd64")
	if err == nil {
		t.Error("expected error for platform with no releases")
	}

	// No releases for non-existent app
	_, err = s.GetLatestRelease(ctx, "nonexistent", "linux", "amd64")
	if err == nil {
		t.Error("expected error for non-existent app")
	}
}

func TestSQLiteStorageReleasesAfterVersion(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	app := models.NewApplication("after-app", "After App", []string{"darwin"})
	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	versions := []string{"1.0.0", "1.5.0", "2.0.0", "3.0.0"}
	for _, v := range versions {
		r := models.NewRelease("after-app", v, "darwin", "arm64", "https://example.com/"+v)
		r.Checksum = "checksum-" + v
		r.FileSize = 1024
		r.ReleaseDate = time.Now()
		if err := s.SaveRelease(ctx, r); err != nil {
			t.Fatalf("SaveRelease %s failed: %v", v, err)
		}
	}

	// Get releases after 1.5.0
	newer, err := s.GetReleasesAfterVersion(ctx, "after-app", "1.5.0", "darwin", "arm64")
	if err != nil {
		t.Fatalf("GetReleasesAfterVersion failed: %v", err)
	}
	if len(newer) != 2 {
		t.Errorf("expected 2 newer releases, got %d", len(newer))
	}
	if len(newer) > 0 && newer[0].Version != "3.0.0" {
		t.Errorf("expected first result to be 3.0.0 (sorted latest first), got %s", newer[0].Version)
	}

	// Get releases after 3.0.0 (none expected)
	newer, err = s.GetReleasesAfterVersion(ctx, "after-app", "3.0.0", "darwin", "arm64")
	if err != nil {
		t.Fatalf("GetReleasesAfterVersion (none) failed: %v", err)
	}
	if len(newer) != 0 {
		t.Errorf("expected 0 newer releases, got %d", len(newer))
	}

	// Invalid version
	_, err = s.GetReleasesAfterVersion(ctx, "after-app", "invalid", "darwin", "arm64")
	if err == nil {
		t.Error("expected error for invalid version")
	}

	// Non-existent app returns empty
	newer, err = s.GetReleasesAfterVersion(ctx, "nonexistent", "1.0.0", "darwin", "arm64")
	if err != nil {
		t.Fatalf("GetReleasesAfterVersion (nonexistent) failed: %v", err)
	}
	if len(newer) != 0 {
		t.Errorf("expected 0 releases for nonexistent app, got %d", len(newer))
	}
}

func TestSQLiteStorageConcurrency(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	app := models.NewApplication("concurrent-app", "Concurrent App", []string{"windows"})
	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	// Concurrent reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, err := s.Applications(ctx)
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				app := models.NewApplication("concurrent-app", "Updated "+time.Now().String(), []string{"windows"})
				if err := s.SaveApplication(ctx, app); err != nil {
					errs <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent operation error: %v", err)
	}
}

func TestSQLiteStorageApplicationConfig(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	app := models.NewApplication("config-app", "Config App", []string{"windows"})
	app.Config.AutoUpdate = true
	app.Config.UpdateInterval = 7200
	app.Config.MinVersion = "1.0.0"
	app.Config.CustomFields = map[string]string{"env": "production"}

	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	got, err := s.GetApplication(ctx, "config-app")
	if err != nil {
		t.Fatalf("GetApplication failed: %v", err)
	}

	if !got.Config.AutoUpdate {
		t.Error("expected AutoUpdate to be true")
	}
	if got.Config.UpdateInterval != 7200 {
		t.Errorf("expected UpdateInterval 7200, got %d", got.Config.UpdateInterval)
	}
	if got.Config.MinVersion != "1.0.0" {
		t.Errorf("expected MinVersion '1.0.0', got %q", got.Config.MinVersion)
	}
	if got.Config.CustomFields["env"] != "production" {
		t.Errorf("expected custom field env=production, got %v", got.Config.CustomFields)
	}
}

func TestSQLiteStorage_DeleteApplication(t *testing.T) {
	s := newSQLiteTestStorage(t)
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
				app := models.NewApplication("del-app", "Delete App", []string{"windows"})
				if err := s.SaveApplication(ctx, app); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			},
			appID:   "del-app",
			wantErr: false,
		},
		{
			name:    "delete non-existent application",
			setup:   func() {},
			appID:   "non-existent",
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

func TestSQLiteStorageClose(t *testing.T) {
	s, err := NewSQLiteStorage(Config{ConnectionString: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	err = s.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
