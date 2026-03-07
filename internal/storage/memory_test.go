package storage

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
	"updater/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStorage(t *testing.T) {
	storage, err := NewMemoryStorage()
	if err != nil {
		t.Fatalf("Failed to create memory storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Test application operations
	t.Run("Application Operations", func(t *testing.T) {
		// Test empty applications list
		apps, _, err := storage.ListApplicationsPaged(ctx, 50, 0)
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
		apps, _, err = storage.ListApplicationsPaged(ctx, 50, 0)
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
		releases, _, err := storage.ListReleasesPaged(ctx, "test-app", models.ReleaseFilters{}, "release_date", "desc", 50, 0)
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
		releases, _, err = storage.ListReleasesPaged(ctx, "test-app", models.ReleaseFilters{}, "release_date", "desc", 50, 0)
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
		releases, _, err = storage.ListReleasesPaged(ctx, "test-app", models.ReleaseFilters{}, "release_date", "desc", 50, 0)
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

func TestMemoryStorage_DeleteApplication(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(s *MemoryStorage)
		appID     string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "delete existing application",
			setup: func(s *MemoryStorage) {
				s.SaveApplication(ctx, &models.Application{
					ID:   "app-to-delete",
					Name: "Delete Me",
				})
			},
			appID:   "app-to-delete",
			wantErr: false,
		},
		{
			name:      "delete non-existent application",
			setup:     func(s *MemoryStorage) {},
			appID:     "non-existent",
			wantErr:   true,
			errSubstr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := NewMemoryStorage()
			if err != nil {
				t.Fatalf("Failed to create memory storage: %v", err)
			}
			defer storage.Close()

			tt.setup(storage)

			err = storage.DeleteApplication(ctx, tt.appID)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify application is gone
			_, err = storage.GetApplication(ctx, tt.appID)
			if err == nil {
				t.Error("expected error when getting deleted application")
			}
		})
	}
}

func TestMemoryStorageConcurrency(t *testing.T) {
	storage, err := NewMemoryStorage()
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

func TestMemoryStorage_APIKeyCRUD(t *testing.T) {
	s, err := NewMemoryStorage()
	require.NoError(t, err)
	ctx := context.Background()

	raw, err := models.GenerateAPIKey()
	require.NoError(t, err)
	key := models.NewAPIKey(models.NewKeyID(), "test", raw, []string{"read"})

	// Create
	require.NoError(t, s.CreateAPIKey(ctx, key))

	// GetByHash
	got, err := s.GetAPIKeyByHash(ctx, key.KeyHash)
	require.NoError(t, err)
	assert.Equal(t, key.ID, got.ID)
	assert.Equal(t, key.Name, got.Name)

	// List
	list, err := s.ListAPIKeys(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Update
	key.Name = "updated"
	require.NoError(t, s.UpdateAPIKey(ctx, key))
	got, err = s.GetAPIKeyByHash(ctx, key.KeyHash)
	require.NoError(t, err)
	assert.Equal(t, "updated", got.Name)

	// Delete
	require.NoError(t, s.DeleteAPIKey(ctx, key.ID))
	_, err = s.GetAPIKeyByHash(ctx, key.KeyHash)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestMemoryStorage_GetAPIKeyByHash_NotFound(t *testing.T) {
	s, err := NewMemoryStorage()
	require.NoError(t, err)
	_, err = s.GetAPIKeyByHash(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestMemoryStorage_UpdateAPIKey_NotFound(t *testing.T) {
	s, err := NewMemoryStorage()
	require.NoError(t, err)
	key := &models.APIKey{ID: "missing"}
	err = s.UpdateAPIKey(context.Background(), key)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestMemoryStorage_DeleteAPIKey_NotFound(t *testing.T) {
	s, err := NewMemoryStorage()
	require.NoError(t, err)
	err = s.DeleteAPIKey(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

// seedApp is a helper that saves an application with the given id/name.
func seedApp(t *testing.T, s *MemoryStorage, id, name string) {
	t.Helper()
	require.NoError(t, s.SaveApplication(context.Background(), &models.Application{
		ID:   id,
		Name: name,
	}))
}

// seedRelease is a helper that saves a release with the given fields.
func seedRelease(t *testing.T, s *MemoryStorage, appID, version, platform, arch string, required bool, releaseDate time.Time) {
	t.Helper()
	require.NoError(t, s.SaveRelease(context.Background(), &models.Release{
		ID:            fmt.Sprintf("%s-%s-%s-%s", appID, version, platform, arch),
		ApplicationID: appID,
		Version:       version,
		Platform:      platform,
		Architecture:  arch,
		DownloadURL:   "https://example.com/download",
		Checksum:      "abc123",
		ChecksumType:  "sha256",
		ReleaseDate:   releaseDate,
		Required:      required,
		CreatedAt:     releaseDate,
	}))
}

func TestMemoryStorage_ListApplicationsPaged(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		setup      func(s *MemoryStorage)
		limit      int
		offset     int
		wantCount  int
		wantTotal  int
		wantEmpty  bool
	}{
		{
			name: "page 2 of size 2 from 5 apps",
			setup: func(s *MemoryStorage) {
				for i := 1; i <= 5; i++ {
					seedApp(t, s, fmt.Sprintf("app-%d", i), fmt.Sprintf("App %d", i))
				}
			},
			limit:     2,
			offset:    2,
			wantCount: 2,
			wantTotal: 5,
		},
		{
			name: "offset beyond end returns empty",
			setup: func(s *MemoryStorage) {
				for i := 1; i <= 5; i++ {
					seedApp(t, s, fmt.Sprintf("app-%d", i), fmt.Sprintf("App %d", i))
				}
			},
			limit:     2,
			offset:    10,
			wantCount: 0,
			wantTotal: 5,
			wantEmpty: true,
		},
		{
			name: "limit 0 returns empty",
			setup: func(s *MemoryStorage) {
				seedApp(t, s, "app-1", "App 1")
			},
			limit:     0,
			offset:    0,
			wantCount: 0,
			wantTotal: 1,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewMemoryStorage()
			require.NoError(t, err)
			defer s.Close()

			tt.setup(s)

			apps, total, err := s.ListApplicationsPaged(ctx, tt.limit, tt.offset)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, total)
			assert.Len(t, apps, tt.wantCount)
			if tt.wantEmpty {
				assert.Empty(t, apps)
			}
		})
	}
}

func TestMemoryStorage_ListReleasesPaged(t *testing.T) {
	ctx := context.Background()
	appID := "test-app"
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		setup      func(s *MemoryStorage)
		filters    models.ReleaseFilters
		sortBy     string
		sortOrder  string
		limit      int
		offset     int
		wantCount  int
		wantTotal  int
		wantEmpty  bool
		// wantVersionOrder, if set, asserts the versions in the returned slice in order.
		wantVersionOrder []string
	}{
		{
			name: "filter by platform linux",
			setup: func(s *MemoryStorage) {
				for i := 0; i < 3; i++ {
					seedRelease(t, s, appID, fmt.Sprintf("1.%d.0", i), "linux", "amd64", false, base.Add(time.Duration(i)*time.Hour))
				}
				for i := 0; i < 2; i++ {
					seedRelease(t, s, appID, fmt.Sprintf("2.%d.0", i), "windows", "amd64", false, base.Add(time.Duration(i)*time.Hour))
				}
			},
			filters:   models.ReleaseFilters{Platforms: []string{"linux"}},
			sortBy:    "release_date",
			sortOrder: "asc",
			limit:     10,
			offset:    0,
			wantCount: 3,
			wantTotal: 3,
		},
		{
			name: "filter by architecture amd64",
			setup: func(s *MemoryStorage) {
				seedRelease(t, s, appID, "1.0.0", "linux", "amd64", false, base)
				seedRelease(t, s, appID, "1.0.0", "linux", "arm64", false, base)
				seedRelease(t, s, appID, "2.0.0", "linux", "amd64", false, base.Add(time.Hour))
			},
			filters:   models.ReleaseFilters{Architecture: "amd64"},
			sortBy:    "release_date",
			sortOrder: "asc",
			limit:     10,
			offset:    0,
			wantCount: 2,
			wantTotal: 2,
		},
		{
			name: "sort by version desc",
			setup: func(s *MemoryStorage) {
				seedRelease(t, s, appID, "1.0.0", "linux", "amd64", false, base)
				seedRelease(t, s, appID, "1.5.0", "linux", "amd64", false, base.Add(time.Hour))
				seedRelease(t, s, appID, "2.0.0", "linux", "amd64", false, base.Add(2*time.Hour))
			},
			filters:          models.ReleaseFilters{},
			sortBy:           "version",
			sortOrder:        "desc",
			limit:            10,
			offset:           0,
			wantCount:        3,
			wantTotal:        3,
			wantVersionOrder: []string{"2.0.0", "1.5.0", "1.0.0"},
		},
		{
			name: "pagination limit 2 offset 0",
			setup: func(s *MemoryStorage) {
				for i := 1; i <= 4; i++ {
					seedRelease(t, s, appID, fmt.Sprintf("%d.0.0", i), "linux", "amd64", false, base.Add(time.Duration(i)*time.Hour))
				}
			},
			filters:   models.ReleaseFilters{},
			sortBy:    "release_date",
			sortOrder: "asc",
			limit:     2,
			offset:    0,
			wantCount: 2,
			wantTotal: 4,
		},
		{
			name: "empty result for non-existent version filter",
			setup: func(s *MemoryStorage) {
				seedRelease(t, s, appID, "1.0.0", "linux", "amd64", false, base)
			},
			filters:   models.ReleaseFilters{Version: "9.9.9"},
			sortBy:    "release_date",
			sortOrder: "asc",
			limit:     10,
			offset:    0,
			wantCount: 0,
			wantTotal: 0,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewMemoryStorage()
			require.NoError(t, err)
			defer s.Close()

			tt.setup(s)

			releases, total, err := s.ListReleasesPaged(ctx, appID, tt.filters, tt.sortBy, tt.sortOrder, tt.limit, tt.offset)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, total)
			assert.Len(t, releases, tt.wantCount)
			if tt.wantEmpty {
				assert.Empty(t, releases)
			}
			if len(tt.wantVersionOrder) > 0 {
				require.Len(t, releases, len(tt.wantVersionOrder))
				for i, v := range tt.wantVersionOrder {
					assert.Equal(t, v, releases[i].Version, "index %d", i)
				}
			}
		})
	}
}

func TestMemoryStorage_GetLatestStableRelease(t *testing.T) {
	ctx := context.Background()
	appID := "test-app"
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		setup       func(s *MemoryStorage)
		platform    string
		arch        string
		wantVersion string
		wantErr     error
	}{
		{
			name: "returns highest stable version, ignoring pre-releases",
			setup: func(s *MemoryStorage) {
				seedRelease(t, s, appID, "1.0.0", "linux", "amd64", false, base)
				seedRelease(t, s, appID, "2.0.0-beta", "linux", "amd64", false, base.Add(time.Hour))
				seedRelease(t, s, appID, "1.5.0", "linux", "amd64", false, base.Add(2*time.Hour))
			},
			platform:    "linux",
			arch:        "amd64",
			wantVersion: "1.5.0",
		},
		{
			name: "only pre-releases returns ErrNotFound",
			setup: func(s *MemoryStorage) {
				seedRelease(t, s, appID, "1.0.0-beta", "linux", "amd64", false, base)
			},
			platform: "linux",
			arch:     "amd64",
			wantErr:  ErrNotFound,
		},
		{
			name: "wrong platform returns ErrNotFound",
			setup: func(s *MemoryStorage) {
				seedRelease(t, s, appID, "1.0.0", "linux", "amd64", false, base)
			},
			platform: "windows",
			arch:     "amd64",
			wantErr:  ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewMemoryStorage()
			require.NoError(t, err)
			defer s.Close()

			tt.setup(s)

			release, err := s.GetLatestStableRelease(ctx, appID, tt.platform, tt.arch)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, release)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantVersion, release.Version)
		})
	}
}

func TestMemoryStorage_GetApplicationStats(t *testing.T) {
	ctx := context.Background()
	appID := "test-app"
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		setup             func(s *MemoryStorage)
		wantTotalReleases int
		wantRequired      int
		wantPlatformCount int
		wantLatestVersion string
	}{
		{
			name: "app with multiple releases across platforms",
			setup: func(s *MemoryStorage) {
				// 1 required release on linux
				seedRelease(t, s, appID, "1.0.0", "linux", "amd64", true, base)
				// 1 non-required on linux
				seedRelease(t, s, appID, "1.5.0", "linux", "amd64", false, base.Add(time.Hour))
				// 1 non-required on windows (2.0.0-beta is highest version overall)
				seedRelease(t, s, appID, "2.0.0-beta", "windows", "amd64", false, base.Add(2*time.Hour))
			},
			wantTotalReleases: 3,
			wantRequired:      1,
			wantPlatformCount: 2,
			wantLatestVersion: "2.0.0-beta",
		},
		{
			name:              "app with no releases returns zero stats",
			setup:             func(s *MemoryStorage) {},
			wantTotalReleases: 0,
			wantRequired:      0,
			wantPlatformCount: 0,
			wantLatestVersion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewMemoryStorage()
			require.NoError(t, err)
			defer s.Close()

			tt.setup(s)

			stats, err := s.GetApplicationStats(ctx, appID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotalReleases, stats.TotalReleases)
			assert.Equal(t, tt.wantRequired, stats.RequiredReleases)
			assert.Equal(t, tt.wantPlatformCount, stats.PlatformCount)
			assert.Equal(t, tt.wantLatestVersion, stats.LatestVersion)
		})
	}
}
