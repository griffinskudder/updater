package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
	"updater/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJSONStorage(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.json")

	config := Config{
		Type:     "json",
		Path:     filePath,
		CacheTTL: "1m",
	}

	storage, err := NewJSONStorage(config)
	require.NoError(t, err)
	require.NotNil(t, storage)
	defer storage.Close()

	// Check that file was created
	assert.FileExists(t, filePath)

	// Check that cache TTL was set correctly
	assert.Equal(t, time.Minute, storage.cacheTTL)
}

func TestNewJSONStorage_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not enforced on Windows")
	}
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "subdir", "test.json")

	storage, err := NewJSONStorage(Config{Type: "json", Path: filePath})
	require.NoError(t, err)
	defer storage.Close()

	// Directory must be traversable by owner only.
	dirInfo, err := os.Stat(filepath.Dir(filePath))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm(),
		"directory should be 0700 (owner rwx only)")

	// Data file must be readable/writable by owner only.
	fileInfo, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fileInfo.Mode().Perm(),
		"data file should be 0600 (owner rw only)")
}

func TestNewJSONStorage_InvalidPath(t *testing.T) {
	// Use a path that can't be created (root directory on most systems)
	config := Config{
		Type: "json",
		Path: "/",
	}

	_, err := NewJSONStorage(config)
	assert.Error(t, err)
}

func TestJSONStorage_Applications(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Initially should be empty
	apps, err := storage.Applications(ctx)
	require.NoError(t, err)
	assert.Empty(t, apps)

	// Add an application
	app := &models.Application{
		ID:        "test-app",
		Name:      "Test App",
		Platforms: []string{"windows", "linux"},
		Config:    models.ApplicationConfig{UpdateInterval: 3600},
	}

	err = storage.SaveApplication(ctx, app)
	require.NoError(t, err)

	// Should now have one application
	apps, err = storage.Applications(ctx)
	require.NoError(t, err)
	assert.Len(t, apps, 1)
	assert.Equal(t, "test-app", apps[0].ID)
	assert.Equal(t, "Test App", apps[0].Name)
}

func TestJSONStorage_GetApplication(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Test getting non-existent application
	_, err := storage.GetApplication(ctx, "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Add an application
	app := &models.Application{
		ID:        "test-app",
		Name:      "Test App",
		Platforms: []string{"windows", "linux"},
		Config:    models.ApplicationConfig{UpdateInterval: 3600},
	}

	err = storage.SaveApplication(ctx, app)
	require.NoError(t, err)

	// Should be able to retrieve it
	retrievedApp, err := storage.GetApplication(ctx, "test-app")
	require.NoError(t, err)
	assert.Equal(t, "test-app", retrievedApp.ID)
	assert.Equal(t, "Test App", retrievedApp.Name)
	assert.Equal(t, []string{"windows", "linux"}, retrievedApp.Platforms)
}

func TestJSONStorage_SaveApplication(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	app := &models.Application{
		ID:        "test-app",
		Name:      "Test App",
		Platforms: []string{"windows"},
		Config:    models.ApplicationConfig{UpdateInterval: 3600},
	}

	// Save new application
	err := storage.SaveApplication(ctx, app)
	require.NoError(t, err)

	// Update existing application
	app.Name = "Updated Test App"
	app.Platforms = []string{"windows", "linux"}
	err = storage.SaveApplication(ctx, app)
	require.NoError(t, err)

	// Verify update
	retrievedApp, err := storage.GetApplication(ctx, "test-app")
	require.NoError(t, err)
	assert.Equal(t, "Updated Test App", retrievedApp.Name)
	assert.Equal(t, []string{"windows", "linux"}, retrievedApp.Platforms)
}

func TestJSONStorage_Releases(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Initially should be empty
	releases, err := storage.Releases(ctx, "test-app")
	require.NoError(t, err)
	assert.Empty(t, releases)

	// Add some releases
	release1 := createTestRelease("test-app", "1.0.0", "windows", "amd64")
	release2 := createTestRelease("test-app", "1.1.0", "windows", "amd64")
	release3 := createTestRelease("other-app", "1.0.0", "windows", "amd64")

	err = storage.SaveRelease(ctx, release1)
	require.NoError(t, err)
	err = storage.SaveRelease(ctx, release2)
	require.NoError(t, err)
	err = storage.SaveRelease(ctx, release3)
	require.NoError(t, err)

	// Should get only releases for test-app
	releases, err = storage.Releases(ctx, "test-app")
	require.NoError(t, err)
	assert.Len(t, releases, 2)

	// Should be sorted by release date (latest first)
	assert.Equal(t, "1.1.0", releases[0].Version)
	assert.Equal(t, "1.0.0", releases[1].Version)
}

func TestJSONStorage_GetRelease(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Test getting non-existent release
	_, err := storage.GetRelease(ctx, "test-app", "1.0.0", "windows", "amd64")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Add a release
	release := createTestRelease("test-app", "1.0.0", "windows", "amd64")
	err = storage.SaveRelease(ctx, release)
	require.NoError(t, err)

	// Should be able to retrieve it
	retrieved, err := storage.GetRelease(ctx, "test-app", "1.0.0", "windows", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", retrieved.Version)
	assert.Equal(t, "windows", retrieved.Platform)
	assert.Equal(t, "amd64", retrieved.Architecture)
}

func TestJSONStorage_SaveRelease(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	release := createTestRelease("test-app", "1.0.0", "windows", "amd64")

	// Save new release
	err := storage.SaveRelease(ctx, release)
	require.NoError(t, err)

	// Update existing release
	release.ReleaseNotes = "Updated release notes"
	release.FileSize = 9999999
	err = storage.SaveRelease(ctx, release)
	require.NoError(t, err)

	// Verify update
	retrieved, err := storage.GetRelease(ctx, "test-app", "1.0.0", "windows", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "Updated release notes", retrieved.ReleaseNotes)
	assert.Equal(t, int64(9999999), retrieved.FileSize)
}

func TestJSONStorage_DeleteRelease(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Test deleting non-existent release
	err := storage.DeleteRelease(ctx, "test-app", "1.0.0", "windows", "amd64")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Add a release
	release := createTestRelease("test-app", "1.0.0", "windows", "amd64")
	err = storage.SaveRelease(ctx, release)
	require.NoError(t, err)

	// Delete the release
	err = storage.DeleteRelease(ctx, "test-app", "1.0.0", "windows", "amd64")
	require.NoError(t, err)

	// Should no longer exist
	_, err = storage.GetRelease(ctx, "test-app", "1.0.0", "windows", "amd64")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestJSONStorage_DeleteApplication(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(s *JSONStorage)
		appID     string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "delete existing application",
			setup: func(s *JSONStorage) {
				s.SaveApplication(ctx, &models.Application{
					ID:        "app-to-delete",
					Name:      "Delete Me",
					Platforms: []string{"windows"},
					Config:    models.ApplicationConfig{},
				})
			},
			appID:   "app-to-delete",
			wantErr: false,
		},
		{
			name:      "delete non-existent application",
			setup:     func(s *JSONStorage) {},
			appID:     "non-existent",
			wantErr:   true,
			errSubstr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := setupTestStorage(t)
			defer storage.Close()

			tt.setup(storage)

			err := storage.DeleteApplication(ctx, tt.appID)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}

			require.NoError(t, err)

			// Verify application is gone
			_, err = storage.GetApplication(ctx, tt.appID)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not found")
		})
	}
}

func TestJSONStorage_GetLatestRelease(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Test getting latest from empty storage
	_, err := storage.GetLatestRelease(ctx, "test-app", "windows", "amd64")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no releases found")

	// Add multiple releases
	releases := []*models.Release{
		createTestRelease("test-app", "1.0.0", "windows", "amd64"),
		createTestRelease("test-app", "1.2.0", "windows", "amd64"),
		createTestRelease("test-app", "1.1.0", "windows", "amd64"),
		createTestRelease("test-app", "1.0.0", "linux", "amd64"),    // Different platform
		createTestRelease("other-app", "2.0.0", "windows", "amd64"), // Different app
	}

	for _, release := range releases {
		err := storage.SaveRelease(ctx, release)
		require.NoError(t, err)
	}

	// Should get the latest version for the specific platform
	latest, err := storage.GetLatestRelease(ctx, "test-app", "windows", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "1.2.0", latest.Version)
	assert.Equal(t, "windows", latest.Platform)
	assert.Equal(t, "amd64", latest.Architecture)
}

func TestJSONStorage_GetReleasesAfterVersion(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Add multiple releases
	releases := []*models.Release{
		createTestRelease("test-app", "1.0.0", "windows", "amd64"),
		createTestRelease("test-app", "1.1.0", "windows", "amd64"),
		createTestRelease("test-app", "1.2.0", "windows", "amd64"),
		createTestRelease("test-app", "2.0.0", "windows", "amd64"),
		createTestRelease("test-app", "1.1.0", "linux", "amd64"), // Different platform
	}

	for _, release := range releases {
		err := storage.SaveRelease(ctx, release)
		require.NoError(t, err)
	}

	// Get releases after 1.0.0
	newerReleases, err := storage.GetReleasesAfterVersion(ctx, "test-app", "1.0.0", "windows", "amd64")
	require.NoError(t, err)
	assert.Len(t, newerReleases, 3)

	// Should be sorted by version (latest first)
	assert.Equal(t, "2.0.0", newerReleases[0].Version)
	assert.Equal(t, "1.2.0", newerReleases[1].Version)
	assert.Equal(t, "1.1.0", newerReleases[2].Version)

	// Get releases after 1.5.0 (should only get 2.0.0)
	newerReleases, err = storage.GetReleasesAfterVersion(ctx, "test-app", "1.5.0", "windows", "amd64")
	require.NoError(t, err)
	assert.Len(t, newerReleases, 1)
	assert.Equal(t, "2.0.0", newerReleases[0].Version)

	// Test invalid current version
	_, err = storage.GetReleasesAfterVersion(ctx, "test-app", "invalid-version", "windows", "amd64")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid current version")
}

func TestJSONStorage_Caching(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "cache_test.json")

	config := Config{
		Type:     "json",
		Path:     filePath,
		CacheTTL: "100ms", // Very short TTL for testing
	}

	storage, err := NewJSONStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	// Add an application
	app := &models.Application{
		ID:        "test-app",
		Name:      "Test App",
		Platforms: []string{"windows"},
		Config:    models.ApplicationConfig{},
	}

	err = storage.SaveApplication(ctx, app)
	require.NoError(t, err)

	// Verify it's cached
	apps, err := storage.Applications(ctx)
	require.NoError(t, err)
	assert.Len(t, apps, 1)

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Should reload from disk
	apps, err = storage.Applications(ctx)
	require.NoError(t, err)
	assert.Len(t, apps, 1)
}

func TestJSONStorage_ConcurrentAccess(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Test concurrent reads and writes
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	// Start multiple goroutines doing operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Create an application
			app := &models.Application{
				ID:        fmt.Sprintf("app-%d", id),
				Name:      fmt.Sprintf("App %d", id),
				Platforms: []string{"windows"},
				Config:    models.ApplicationConfig{},
			}

			err := storage.SaveApplication(ctx, app)
			assert.NoError(t, err)

			// Read it back
			_, err = storage.GetApplication(ctx, app.ID)
			assert.NoError(t, err)

			// List all applications
			_, err = storage.Applications(ctx)
			assert.NoError(t, err)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all applications were created
	apps, err := storage.Applications(ctx)
	require.NoError(t, err)
	assert.Len(t, apps, numGoroutines)
}

func TestJSONStorage_ConcurrentLoad(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	// Expire the cache so all goroutines hit the slow path.
	storage.mu.Lock()
	storage.cacheExpiry = time.Time{}
	storage.mu.Unlock()

	const n = 20
	errs := make(chan error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			errs <- storage.loadData()
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
	storage.mu.RLock()
	assert.NotNil(t, storage.data)
	storage.mu.RUnlock()
}

// Helper functions

func setupTestStorage(t *testing.T) *JSONStorage {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.json")

	config := Config{
		Type: "json",
		Path: filePath,
	}

	storage, err := NewJSONStorage(config)
	require.NoError(t, err)
	return storage
}

func createTestRelease(appID, version, platform, arch string) *models.Release {
	release := models.NewRelease(appID, version, platform, arch, "https://example.com/download")
	release.Checksum = "abc123"
	release.ChecksumType = "sha256"
	release.FileSize = 1234567
	release.ReleaseNotes = "Test release"

	// Set release date based on version for consistent sorting
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	switch version {
	case "1.0.0":
		release.ReleaseDate = baseTime
	case "1.1.0":
		release.ReleaseDate = baseTime.Add(24 * time.Hour)
	case "1.2.0":
		release.ReleaseDate = baseTime.Add(48 * time.Hour)
	case "2.0.0":
		release.ReleaseDate = baseTime.Add(72 * time.Hour)
	default:
		release.ReleaseDate = baseTime.Add(time.Hour)
	}

	return release
}

func TestJSONStorage_APIKeyCRUD(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSONStorage(Config{Path: filepath.Join(dir, "data.json")})
	require.NoError(t, err)
	ctx := context.Background()

	raw, err := models.GenerateAPIKey()
	require.NoError(t, err)
	key := models.NewAPIKey(models.NewKeyID(), "ci", raw, []string{"write"})

	require.NoError(t, s.CreateAPIKey(ctx, key))

	got, err := s.GetAPIKeyByHash(ctx, key.KeyHash)
	require.NoError(t, err)
	assert.Equal(t, "ci", got.Name)
	assert.Equal(t, []string{"write"}, got.Permissions)

	keys, err := s.ListAPIKeys(ctx)
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	key.Name = "ci-updated"
	require.NoError(t, s.UpdateAPIKey(ctx, key))
	got, err = s.GetAPIKeyByHash(ctx, key.KeyHash)
	require.NoError(t, err)
	assert.Equal(t, "ci-updated", got.Name)

	require.NoError(t, s.DeleteAPIKey(ctx, key.ID))
	_, err = s.GetAPIKeyByHash(ctx, key.KeyHash)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestJSONStorage_GetAPIKeyByHash_NotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSONStorage(Config{Path: filepath.Join(dir, "data.json")})
	require.NoError(t, err)
	_, err = s.GetAPIKeyByHash(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}
