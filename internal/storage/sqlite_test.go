package storage

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
	"updater/internal/models"
	sqlcite "updater/internal/storage/sqlc/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSQLiteTestStorage(t *testing.T) Storage {
	t.Helper()
	s, err := NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteStorageConnectionError(t *testing.T) {
	_, err := NewSQLiteStorage("")
	if err == nil {
		t.Error("expected error for empty connection string")
	}
}

func TestSQLiteStorageSchemaCreation(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	// Verify tables exist by performing operations
	apps, _, err := s.ListApplicationsPaged(ctx, 50, nil)
	if err != nil {
		t.Fatalf("ListApplicationsPaged failed: %v", err)
	}
	if apps == nil {
		t.Error("expected non-nil slice")
	}

	releases, _, err := s.ListReleasesPaged(ctx, "test-app", models.ReleaseFilters{}, "release_date", "desc", 50, nil)
	if err != nil {
		t.Fatalf("ListReleasesPaged failed: %v", err)
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
	apps, _, err := s.ListApplicationsPaged(ctx, 50, nil)
	if err != nil {
		t.Fatalf("ListApplicationsPaged failed: %v", err)
	}
	if len(apps) != 1 {
		t.Errorf("expected 1 application, got %d", len(apps))
	}

	// Create second application
	app2 := models.NewApplication("test-app-2", "Second App", []string{"darwin"})
	if err := s.SaveApplication(ctx, app2); err != nil {
		t.Fatalf("SaveApplication (second) failed: %v", err)
	}

	apps, _, err = s.ListApplicationsPaged(ctx, 50, nil)
	if err != nil {
		t.Fatalf("ListApplicationsPaged failed: %v", err)
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
	releases, _, err := s.ListReleasesPaged(ctx, "rel-app", models.ReleaseFilters{}, "release_date", "desc", 50, nil)
	if err != nil {
		t.Fatalf("ListReleasesPaged failed: %v", err)
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
	releases, _, err = s.ListReleasesPaged(ctx, "rel-app", models.ReleaseFilters{}, "release_date", "desc", 50, nil)
	if err != nil {
		t.Fatalf("ListReleasesPaged failed: %v", err)
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
				_, _, err := s.ListApplicationsPaged(ctx, 50, nil)
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
	app.Config.CustomFields = map[string]string{"env": "production"}

	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	got, err := s.GetApplication(ctx, "config-app")
	if err != nil {
		t.Fatalf("GetApplication failed: %v", err)
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
	s, err := NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	err = s.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestSQLiteStorage_APIKeyCRUD(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	raw, err := models.GenerateAPIKey()
	require.NoError(t, err)
	key := models.NewAPIKey(models.NewKeyID(), "deploy", raw, []string{"write"})

	require.NoError(t, s.CreateAPIKey(ctx, key))

	got, err := s.GetAPIKeyByHash(ctx, key.KeyHash)
	require.NoError(t, err)
	assert.Equal(t, key.ID, got.ID)
	assert.Equal(t, []string{"write"}, got.Permissions)

	keys, err := s.ListAPIKeys(ctx)
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	key.Name = "deploy-v2"
	key.Permissions = []string{"write", "read"}
	require.NoError(t, s.UpdateAPIKey(ctx, key))

	got, err = s.GetAPIKeyByHash(ctx, key.KeyHash)
	require.NoError(t, err)
	assert.Equal(t, "deploy-v2", got.Name)
	assert.Equal(t, []string{"write", "read"}, got.Permissions)

	require.NoError(t, s.DeleteAPIKey(ctx, key.ID))
	_, err = s.GetAPIKeyByHash(ctx, key.KeyHash)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSQLiteStorage_GetAPIKeyByHash_NotFound(t *testing.T) {
	s := newSQLiteTestStorage(t)
	_, err := s.GetAPIKeyByHash(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSQLiteStorage_UpdateAPIKey_NotFound(t *testing.T) {
	s := newSQLiteTestStorage(t)
	key := &models.APIKey{ID: "missing", Name: "x", Permissions: []string{"read"}}
	err := s.UpdateAPIKey(context.Background(), key)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSQLiteStorage_DeleteAPIKey_NotFound(t *testing.T) {
	s := newSQLiteTestStorage(t)
	err := s.DeleteAPIKey(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSQLiteStorage_ListApplicationsPaged(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	// Seed 3 apps
	appIDs := []string{"sqlite-paged-app-a", "sqlite-paged-app-b", "sqlite-paged-app-c"}
	for _, id := range appIDs {
		app := models.NewApplication(id, "Name "+id, []string{"linux"})
		if err := s.SaveApplication(ctx, app); err != nil {
			t.Fatalf("SaveApplication %s failed: %v", id, err)
		}
	}

	t.Run("first page returns 2 apps total=3", func(t *testing.T) {
		apps, total, err := s.ListApplicationsPaged(ctx, 2, nil)
		if err != nil {
			t.Fatalf("ListApplicationsPaged failed: %v", err)
		}
		if total != 3 {
			t.Errorf("expected total=3, got %d", total)
		}
		if len(apps) != 2 {
			t.Errorf("expected 2 apps, got %d", len(apps))
		}
	})

	t.Run("all apps returned with large limit", func(t *testing.T) {
		apps, total, err := s.ListApplicationsPaged(ctx, 1000, nil)
		if err != nil {
			t.Fatalf("ListApplicationsPaged large limit failed: %v", err)
		}
		if total != 3 {
			t.Errorf("expected total=3, got %d", total)
		}
		if len(apps) != 3 {
			t.Errorf("expected 3 apps, got %d", len(apps))
		}
	})
}

func TestSQLiteStorage_ListReleasesPaged(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	appID := "sqlite-paged-rel-app"
	app := models.NewApplication(appID, "SQLite Paged Release App", []string{"linux", "windows"})
	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	releases := []struct {
		version  string
		platform string
		arch     string
		offset   time.Duration
	}{
		{"1.0.0", "linux", "amd64", 0},
		{"1.5.0", "linux", "amd64", time.Hour},
		{"2.0.0", "windows", "amd64", 2 * time.Hour},
	}
	for _, r := range releases {
		rel := models.NewRelease(appID, r.version, r.platform, r.arch, "https://example.com/"+r.version)
		rel.Checksum = "chk-" + r.version
		rel.FileSize = 512
		rel.ReleaseDate = base.Add(r.offset)
		if err := s.SaveRelease(ctx, rel); err != nil {
			t.Fatalf("SaveRelease %s failed: %v", r.version, err)
		}
	}

	t.Run("filter by linux platform returns 2 releases total=2", func(t *testing.T) {
		rels, total, err := s.ListReleasesPaged(ctx, appID, models.ReleaseFilters{Platforms: []string{"linux"}}, "release_date", "asc", 10, nil)
		if err != nil {
			t.Fatalf("ListReleasesPaged failed: %v", err)
		}
		if total != 2 {
			t.Errorf("expected total=2, got %d", total)
		}
		if len(rels) != 2 {
			t.Errorf("expected 2 releases, got %d", len(rels))
		}
		for _, r := range rels {
			if r.Platform != "linux" {
				t.Errorf("expected linux platform, got %s", r.Platform)
			}
		}
	})

	t.Run("sort by version desc returns correct order", func(t *testing.T) {
		rels, total, err := s.ListReleasesPaged(ctx, appID, models.ReleaseFilters{Platforms: []string{"linux"}}, "version", "desc", 10, nil)
		if err != nil {
			t.Fatalf("ListReleasesPaged sort by version failed: %v", err)
		}
		if total != 2 {
			t.Errorf("expected total=2, got %d", total)
		}
		if len(rels) != 2 {
			t.Fatalf("expected 2 releases, got %d", len(rels))
		}
		if rels[0].Version != "1.5.0" {
			t.Errorf("expected first release 1.5.0, got %s", rels[0].Version)
		}
		if rels[1].Version != "1.0.0" {
			t.Errorf("expected second release 1.0.0, got %s", rels[1].Version)
		}
	})

	t.Run("pagination limit=1 no cursor returns one release total=3", func(t *testing.T) {
		rels, total, err := s.ListReleasesPaged(ctx, appID, models.ReleaseFilters{}, "release_date", "asc", 1, nil)
		if err != nil {
			t.Fatalf("ListReleasesPaged pagination failed: %v", err)
		}
		if total != 3 {
			t.Errorf("expected total=3, got %d", total)
		}
		if len(rels) != 1 {
			t.Errorf("expected 1 release, got %d", len(rels))
		}
	})
}

func TestSQLiteStorage_GetLatestStableRelease(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	appID := "sqlite-stable-rel-app"
	app := models.NewApplication(appID, "SQLite Stable Release App", []string{"linux"})
	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	seeds := []struct {
		version  string
		platform string
		arch     string
	}{
		{"1.0.0", "linux", "amd64"},
		{"2.0.0-beta", "linux", "amd64"},
		{"1.5.0", "linux", "amd64"},
	}
	for i, r := range seeds {
		rel := models.NewRelease(appID, r.version, r.platform, r.arch, "https://example.com/"+r.version)
		rel.Checksum = "chk-" + r.version
		rel.FileSize = 512
		rel.ReleaseDate = base.Add(time.Duration(i) * time.Hour)
		if err := s.SaveRelease(ctx, rel); err != nil {
			t.Fatalf("SaveRelease %s failed: %v", r.version, err)
		}
	}

	t.Run("returns highest stable version ignoring pre-releases", func(t *testing.T) {
		release, err := s.GetLatestStableRelease(ctx, appID, "linux", "amd64")
		if err != nil {
			t.Fatalf("GetLatestStableRelease failed: %v", err)
		}
		if release.Version != "1.5.0" {
			t.Errorf("expected 1.5.0, got %s", release.Version)
		}
	})

	t.Run("only pre-releases returns ErrNotFound", func(t *testing.T) {
		appID2 := "sqlite-stable-pre-only-app"
		app2 := models.NewApplication(appID2, "SQLite Stable Pre-only App", []string{"linux"})
		if err := s.SaveApplication(ctx, app2); err != nil {
			t.Fatalf("SaveApplication failed: %v", err)
		}
		rel := models.NewRelease(appID2, "1.0.0-beta", "linux", "amd64", "https://example.com/1.0.0-beta")
		rel.Checksum = "chk-beta"
		rel.FileSize = 512
		if err := s.SaveRelease(ctx, rel); err != nil {
			t.Fatalf("SaveRelease failed: %v", err)
		}
		_, err := s.GetLatestStableRelease(ctx, appID2, "linux", "amd64")
		if err == nil {
			t.Error("expected error for pre-release only app, got nil")
		}
	})

	t.Run("no releases returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetLatestStableRelease(ctx, appID, "windows", "amd64")
		if err == nil {
			t.Error("expected error for platform with no releases, got nil")
		}
	})
}

func TestSQLiteStorage_GetApplicationStats(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ctx := context.Background()

	appID := "sqlite-stats-app"
	app := models.NewApplication(appID, "SQLite Stats App", []string{"linux", "windows"})
	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Seed: 1 required linux release, 1 non-required linux release, 1 non-required windows pre-release
	seeds := []struct {
		version  string
		platform string
		arch     string
		required bool
		offset   time.Duration
	}{
		{"1.0.0", "linux", "amd64", true, 0},
		{"1.5.0", "linux", "amd64", false, time.Hour},
		{"2.0.0-beta", "windows", "amd64", false, 2 * time.Hour},
	}
	for _, r := range seeds {
		rel := models.NewRelease(appID, r.version, r.platform, r.arch, "https://example.com/"+r.version)
		rel.Checksum = "chk-" + r.version
		rel.FileSize = 512
		rel.Required = r.required
		rel.ReleaseDate = base.Add(r.offset)
		if err := s.SaveRelease(ctx, rel); err != nil {
			t.Fatalf("SaveRelease %s failed: %v", r.version, err)
		}
	}

	stats, err := s.GetApplicationStats(ctx, appID)
	if err != nil {
		t.Fatalf("GetApplicationStats failed: %v", err)
	}
	if stats.TotalReleases != 3 {
		t.Errorf("expected TotalReleases=3, got %d", stats.TotalReleases)
	}
	if stats.RequiredReleases != 1 {
		t.Errorf("expected RequiredReleases=1, got %d", stats.RequiredReleases)
	}
	if stats.PlatformCount != 2 {
		t.Errorf("expected PlatformCount=2, got %d", stats.PlatformCount)
	}
	if stats.LatestVersion != "2.0.0-beta" {
		t.Errorf("expected LatestVersion=2.0.0-beta, got %q", stats.LatestVersion)
	}
	if stats.LatestReleaseDate == nil {
		t.Error("expected LatestReleaseDate to be non-nil")
	}
}

func TestSQLiteStorage_ListApplicationsPaged_TotalCountStable(t *testing.T) {
	store, err := NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer store.Close()
	ctx := context.Background()

	now := time.Now().UTC()
	for i := range 5 {
		app := &models.Application{
			ID:        fmt.Sprintf("app-%d", i),
			Name:      fmt.Sprintf("App %d", i),
			Platforms: []string{"windows"},
			Config:    models.ApplicationConfig{},
			CreatedAt: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			UpdatedAt: now.Format(time.RFC3339),
		}
		require.NoError(t, store.SaveApplication(ctx, app))
	}

	// Page 1: limit=2, no cursor
	page1, total1, err := store.ListApplicationsPaged(ctx, 2, nil)
	require.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.Equal(t, 5, total1, "total_count on page 1 should be 5")

	// Page 2: using cursor from last item on page 1
	createdAt1, err := time.Parse(time.RFC3339, page1[len(page1)-1].CreatedAt)
	require.NoError(t, err)
	cursor := &models.ApplicationCursor{CreatedAt: createdAt1, ID: page1[len(page1)-1].ID}
	page2, total2, err := store.ListApplicationsPaged(ctx, 2, cursor)
	require.NoError(t, err)
	assert.Len(t, page2, 2)
	assert.Equal(t, 5, total2, "total_count on page 2 must equal total_count on page 1")
}

func TestSQLiteStorage_ListReleasesPaged_TotalCountStable(t *testing.T) {
	store, err := NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer store.Close()
	ctx := context.Background()

	app := &models.Application{
		ID:        "app1",
		Name:      "App1",
		Platforms: []string{"windows"},
		Config:    models.ApplicationConfig{},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, store.SaveApplication(ctx, app))

	now := time.Now().UTC()
	for i := range 5 {
		r := &models.Release{
			ID:            fmt.Sprintf("r%d", i),
			ApplicationID: "app1",
			Version:       fmt.Sprintf("1.0.%d", i),
			Platform:      "windows",
			Architecture:  "amd64",
			DownloadURL:   "http://example.com",
			Checksum:      "abc",
			ChecksumType:  "sha256",
			ReleaseDate:   now.Add(time.Duration(i) * time.Second),
			CreatedAt:     now.Add(time.Duration(i) * time.Second),
		}
		require.NoError(t, store.SaveRelease(ctx, r))
	}

	// Page 1
	page1, total1, err := store.ListReleasesPaged(ctx, "app1", models.ReleaseFilters{}, "release_date", "desc", 2, nil)
	require.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.Equal(t, 5, total1)

	// Page 2 using cursor from last item on page 1
	cursor := &models.ReleaseCursor{
		SortBy:      "release_date",
		SortOrder:   "desc",
		ID:          page1[len(page1)-1].ID,
		ReleaseDate: page1[len(page1)-1].ReleaseDate,
	}
	page2, total2, err := store.ListReleasesPaged(ctx, "app1", models.ReleaseFilters{}, "release_date", "desc", 2, cursor)
	require.NoError(t, err)
	assert.Len(t, page2, 2)
	assert.Equal(t, 5, total2, "total_count on page 2 must equal total_count on page 1")
}

func TestSQLiteStorageSaveRelease_VersionSortColumns(t *testing.T) {
	s := newSQLiteTestStorage(t)
	ss := s.(*SQLiteStorage)
	ctx := context.Background()

	app := models.NewApplication("ver-sort-app", "Version Sort App", []string{"linux"})
	require.NoError(t, s.SaveApplication(ctx, app))

	tests := []struct {
		name         string
		version      string
		wantMajor    int64
		wantMinor    int64
		wantPatch    int64
		wantPreValid bool
		wantPreStr   string
	}{
		{
			name:         "pre-release version",
			version:      "2.3.4-beta.1",
			wantMajor:    2,
			wantMinor:    3,
			wantPatch:    4,
			wantPreValid: true,
			wantPreStr:   "beta.1",
		},
		{
			name:         "stable version",
			version:      "1.5.0",
			wantMajor:    1,
			wantMinor:    5,
			wantPatch:    0,
			wantPreValid: false,
			wantPreStr:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release := models.NewRelease("ver-sort-app", tt.version, "linux", "amd64", "https://example.com/"+tt.version)
			release.Checksum = "chk-" + tt.version
			release.FileSize = 512
			require.NoError(t, s.SaveRelease(ctx, release))

			row, err := ss.queries.GetRelease(ctx, sqlcite.GetReleaseParams{
				ApplicationID: "ver-sort-app",
				Version:       tt.version,
				Platform:      "linux",
				Architecture:  "amd64",
			})
			require.NoError(t, err)

			assert.Equal(t, tt.wantMajor, row.VersionMajor, "VersionMajor")
			assert.Equal(t, tt.wantMinor, row.VersionMinor, "VersionMinor")
			assert.Equal(t, tt.wantPatch, row.VersionPatch, "VersionPatch")
			assert.Equal(t, tt.wantPreValid, row.VersionPreRelease.Valid, "VersionPreRelease.Valid")
			if tt.wantPreValid {
				assert.Equal(t, tt.wantPreStr, row.VersionPreRelease.String, "VersionPreRelease.String")
			}
		})
	}
}
