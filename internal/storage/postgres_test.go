package storage

import (
	"context"
	"os"
	"testing"
	"time"
	"updater/internal/models"
	sqlcpg "updater/internal/storage/sqlc/postgres"
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
	s, err := NewPostgresStorage(dsn)
	if err != nil {
		t.Fatalf("failed to create postgres storage: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPostgresStorageConnectionError(t *testing.T) {
	_, err := NewPostgresStorage("")
	if err == nil {
		t.Error("expected error for empty connection string")
	}
}

func TestPostgresStorageInvalidDSN(t *testing.T) {
	_, err := NewPostgresStorage("postgres://invalid:5432/nonexistent")
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

func TestPostgresStorage_APIKeyCRUD(t *testing.T) {
	s := newPostgresTestStorage(t)
	ctx := context.Background()

	raw, err := models.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}
	key := models.NewAPIKey(models.NewKeyID(), "ci", raw, []string{"write"})

	if err := s.CreateAPIKey(ctx, key); err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	got, err := s.GetAPIKeyByHash(ctx, key.KeyHash)
	if err != nil {
		t.Fatalf("GetAPIKeyByHash failed: %v", err)
	}
	if got.ID != key.ID {
		t.Errorf("expected ID %q, got %q", key.ID, got.ID)
	}
	if len(got.Permissions) != 1 || got.Permissions[0] != "write" {
		t.Errorf("expected permissions [write], got %v", got.Permissions)
	}

	keys, err := s.ListAPIKeys(ctx)
	if err != nil {
		t.Fatalf("ListAPIKeys failed: %v", err)
	}
	found := false
	for _, k := range keys {
		if k.ID == key.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find key in ListAPIKeys result")
	}

	key.Name = "ci-v2"
	if err := s.UpdateAPIKey(ctx, key); err != nil {
		t.Fatalf("UpdateAPIKey failed: %v", err)
	}

	if err := s.DeleteAPIKey(ctx, key.ID); err != nil {
		t.Fatalf("DeleteAPIKey failed: %v", err)
	}
	_, err = s.GetAPIKeyByHash(ctx, key.KeyHash)
	if err == nil {
		t.Error("expected ErrNotFound after deletion, got nil")
	}
}

func TestPostgresStorage_GetAPIKeyByHash_NotFound(t *testing.T) {
	s := newPostgresTestStorage(t)
	_, err := s.GetAPIKeyByHash(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected ErrNotFound, got nil")
	}
}

func TestPostgresStorage_UpdateAPIKey_NotFound(t *testing.T) {
	s := newPostgresTestStorage(t)
	key := &models.APIKey{ID: "missing", Name: "x", Permissions: []string{"read"}}
	err := s.UpdateAPIKey(context.Background(), key)
	if err == nil {
		t.Error("expected ErrNotFound, got nil")
	}
}

func TestPostgresStorage_DeleteAPIKey_NotFound(t *testing.T) {
	s := newPostgresTestStorage(t)
	err := s.DeleteAPIKey(context.Background(), "missing")
	if err == nil {
		t.Error("expected ErrNotFound, got nil")
	}
}

func TestPostgresStorage_ListApplicationsPaged(t *testing.T) {
	s := newPostgresTestStorage(t)
	ctx := context.Background()

	// Seed 3 apps
	appIDs := []string{"pg-paged-app-a", "pg-paged-app-b", "pg-paged-app-c"}
	for _, id := range appIDs {
		app := models.NewApplication(id, "Name "+id, []string{"linux"})
		if err := s.SaveApplication(ctx, app); err != nil {
			t.Fatalf("SaveApplication %s failed: %v", id, err)
		}
	}

	t.Run("first page returns 2 apps total=3", func(t *testing.T) {
		apps, total, err := s.ListApplicationsPaged(ctx, 2, 0)
		if err != nil {
			t.Fatalf("ListApplicationsPaged failed: %v", err)
		}
		if total < 3 {
			t.Errorf("expected total >= 3, got %d", total)
		}
		if len(apps) != 2 {
			t.Errorf("expected 2 apps, got %d", len(apps))
		}
	})

	t.Run("second page offset 2 returns remaining apps", func(t *testing.T) {
		_, total, _ := s.ListApplicationsPaged(ctx, 2, 0)
		apps, total2, err := s.ListApplicationsPaged(ctx, 2, 2)
		if err != nil {
			t.Fatalf("ListApplicationsPaged offset=2 failed: %v", err)
		}
		if total2 != total {
			t.Errorf("expected total %d, got %d", total, total2)
		}
		if total >= 3 && len(apps) < 1 {
			t.Errorf("expected at least 1 app on second page, got %d", len(apps))
		}
	})

	t.Run("offset beyond total returns empty slice", func(t *testing.T) {
		apps, _, err := s.ListApplicationsPaged(ctx, 2, 10000)
		if err != nil {
			t.Fatalf("ListApplicationsPaged offset beyond total failed: %v", err)
		}
		if len(apps) != 0 {
			t.Errorf("expected empty apps, got %d", len(apps))
		}
	})
}

func TestPostgresStorage_ListReleasesPaged(t *testing.T) {
	s := newPostgresTestStorage(t)
	ctx := context.Background()

	appID := "pg-paged-rel-app"
	app := models.NewApplication(appID, "PG Paged Release App", []string{"linux", "windows"})
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
		rels, total, err := s.ListReleasesPaged(ctx, appID, models.ReleaseFilters{Platforms: []string{"linux"}}, "release_date", "asc", 10, 0)
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
		rels, total, err := s.ListReleasesPaged(ctx, appID, models.ReleaseFilters{Platforms: []string{"linux"}}, "version", "desc", 10, 0)
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

	t.Run("pagination limit=1 offset=0 returns one release total=3", func(t *testing.T) {
		rels, total, err := s.ListReleasesPaged(ctx, appID, models.ReleaseFilters{}, "release_date", "asc", 1, 0)
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

	t.Run("offset beyond total returns empty", func(t *testing.T) {
		rels, _, err := s.ListReleasesPaged(ctx, appID, models.ReleaseFilters{}, "release_date", "asc", 10, 10000)
		if err != nil {
			t.Fatalf("ListReleasesPaged offset beyond total failed: %v", err)
		}
		if len(rels) != 0 {
			t.Errorf("expected empty, got %d", len(rels))
		}
	})
}

func TestPostgresStorage_GetLatestStableRelease(t *testing.T) {
	s := newPostgresTestStorage(t)
	ctx := context.Background()

	appID := "pg-stable-rel-app"
	app := models.NewApplication(appID, "PG Stable Release App", []string{"linux"})
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
		appID2 := "pg-stable-pre-only-app"
		app2 := models.NewApplication(appID2, "PG Stable Pre-only App", []string{"linux"})
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

func TestPostgresStorage_GetApplicationStats(t *testing.T) {
	s := newPostgresTestStorage(t)
	ctx := context.Background()

	appID := "pg-stats-app"
	app := models.NewApplication(appID, "PG Stats App", []string{"linux", "windows"})
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

func TestPostgresStorageSaveRelease_VersionSortColumns(t *testing.T) {
	s := newPostgresTestStorage(t)
	ps := s.(*PostgresStorage)
	ctx := context.Background()

	app := models.NewApplication("pg-ver-sort-app", "PG Version Sort App", []string{"linux"})
	if err := s.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication failed: %v", err)
	}

	tests := []struct {
		name         string
		version      string
		wantMajor    int32
		wantMinor    int32
		wantPatch    int32
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
			release := models.NewRelease("pg-ver-sort-app", tt.version, "linux", "amd64", "https://example.com/"+tt.version)
			release.Checksum = "chk-" + tt.version
			release.FileSize = 512
			if err := s.SaveRelease(ctx, release); err != nil {
				t.Fatalf("SaveRelease failed: %v", err)
			}

			row, err := ps.queries.GetRelease(ctx, sqlcpg.GetReleaseParams{
				ApplicationID: "pg-ver-sort-app",
				Version:       tt.version,
				Platform:      "linux",
				Architecture:  "amd64",
			})
			if err != nil {
				t.Fatalf("GetRelease failed: %v", err)
			}

			if row.VersionMajor != tt.wantMajor {
				t.Errorf("VersionMajor: want %d, got %d", tt.wantMajor, row.VersionMajor)
			}
			if row.VersionMinor != tt.wantMinor {
				t.Errorf("VersionMinor: want %d, got %d", tt.wantMinor, row.VersionMinor)
			}
			if row.VersionPatch != tt.wantPatch {
				t.Errorf("VersionPatch: want %d, got %d", tt.wantPatch, row.VersionPatch)
			}
			if row.VersionPreRelease.Valid != tt.wantPreValid {
				t.Errorf("VersionPreRelease.Valid: want %v, got %v", tt.wantPreValid, row.VersionPreRelease.Valid)
			}
			if tt.wantPreValid && row.VersionPreRelease.String != tt.wantPreStr {
				t.Errorf("VersionPreRelease.String: want %q, got %q", tt.wantPreStr, row.VersionPreRelease.String)
			}
		})
	}
}
