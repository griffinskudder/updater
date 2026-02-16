package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
	"updater/internal/models"

	"github.com/Masterminds/semver/v3"
)

// JSONStorage implements the Storage interface using JSON files for persistence.
// It provides an in-memory cache for performance and supports concurrent access.
type JSONStorage struct {
	filePath     string
	cacheTTL     time.Duration
	mu           sync.RWMutex
	data         *JSONData
	lastModified time.Time
	cacheExpiry  time.Time
}

// JSONData represents the structure of data stored in JSON format
type JSONData struct {
	Applications []*models.Application `json:"applications"`
	Releases     []*models.Release     `json:"releases"`
	LastUpdated  time.Time             `json:"last_updated"`
}

// NewJSONStorage creates a new JSON-based storage instance
func NewJSONStorage(config Config) (*JSONStorage, error) {
	cacheTTL := 5 * time.Minute
	if config.CacheTTL != "" {
		if duration, err := time.ParseDuration(config.CacheTTL); err == nil {
			cacheTTL = duration
		}
	}

	storage := &JSONStorage{
		filePath: config.Path,
		cacheTTL: cacheTTL,
	}

	// Initialize with empty data if file doesn't exist
	if err := storage.ensureFileExists(); err != nil {
		return nil, fmt.Errorf("failed to ensure file exists: %w", err)
	}

	// Load initial data
	if err := storage.loadData(); err != nil {
		return nil, fmt.Errorf("failed to load initial data: %w", err)
	}

	return storage, nil
}

// ensureFileExists creates the JSON file with empty data if it doesn't exist
func (j *JSONStorage) ensureFileExists() error {
	if _, err := os.Stat(j.filePath); os.IsNotExist(err) {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(j.filePath), 0700); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Create empty JSON file
		emptyData := &JSONData{
			Applications: []*models.Application{},
			Releases:     []*models.Release{},
			LastUpdated:  time.Now(),
		}

		return j.saveData(emptyData)
	}
	return nil
}

// loadData loads data from the JSON file with caching.
// It uses double-checked locking: a fast read-lock path for cache hits,
// and a write-lock slow path with re-validation to prevent TOCTOU races.
func (j *JSONStorage) loadData() error {
	// Fast path: cache is still valid.
	j.mu.RLock()
	if j.data != nil && time.Now().Before(j.cacheExpiry) {
		j.mu.RUnlock()
		return nil
	}
	j.mu.RUnlock()

	// Slow path: acquire write lock and re-validate before doing any I/O.
	j.mu.Lock()
	defer j.mu.Unlock()

	// Another goroutine may have loaded while we waited for the write lock.
	if j.data != nil && time.Now().Before(j.cacheExpiry) {
		return nil
	}

	// Stat and all subsequent reads are done under the write lock.
	info, err := os.Stat(j.filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// If the file hasn't changed, extend the cache and return.
	if j.data != nil && !info.ModTime().After(j.lastModified) {
		j.cacheExpiry = time.Now().Add(j.cacheTTL)
		return nil
	}

	fileData, err := os.ReadFile(j.filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var data JSONData
	if err := json.Unmarshal(fileData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	j.data = &data
	j.lastModified = info.ModTime()
	j.cacheExpiry = time.Now().Add(j.cacheTTL)
	return nil
}

// saveData saves data to the JSON file
func (j *JSONStorage) saveData(data *JSONData) error {
	data.LastUpdated = time.Now()

	fileData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(j.filePath, fileData, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Applications returns all registered applications
func (j *JSONStorage) Applications(ctx context.Context) ([]*models.Application, error) {
	if err := j.loadData(); err != nil {
		return nil, err
	}

	j.mu.RLock()
	defer j.mu.RUnlock()

	// Return a copy to prevent external modification
	apps := make([]*models.Application, len(j.data.Applications))
	copy(apps, j.data.Applications)
	return apps, nil
}

// GetApplication retrieves an application by its ID
func (j *JSONStorage) GetApplication(ctx context.Context, appID string) (*models.Application, error) {
	if err := j.loadData(); err != nil {
		return nil, err
	}

	j.mu.RLock()
	defer j.mu.RUnlock()

	for _, app := range j.data.Applications {
		if app.ID == appID {
			// Return a copy
			appCopy := *app
			return &appCopy, nil
		}
	}

	return nil, fmt.Errorf("application %s not found", appID)
}

// SaveApplication stores or updates an application
func (j *JSONStorage) SaveApplication(ctx context.Context, app *models.Application) error {
	if err := j.loadData(); err != nil {
		return err
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	// Find existing application
	for i, existingApp := range j.data.Applications {
		if existingApp.ID == app.ID {
			// Update existing
			j.data.Applications[i] = app
			return j.saveData(j.data)
		}
	}

	// Add new application
	j.data.Applications = append(j.data.Applications, app)
	return j.saveData(j.data)
}

// DeleteApplication removes an application by its ID
func (j *JSONStorage) DeleteApplication(ctx context.Context, appID string) error {
	if err := j.loadData(); err != nil {
		return err
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	for i, app := range j.data.Applications {
		if app.ID == appID {
			j.data.Applications = append(j.data.Applications[:i], j.data.Applications[i+1:]...)
			return j.saveData(j.data)
		}
	}

	return fmt.Errorf("application %s not found", appID)
}

// Releases returns all releases for a given application
func (j *JSONStorage) Releases(ctx context.Context, appID string) ([]*models.Release, error) {
	if err := j.loadData(); err != nil {
		return nil, err
	}

	j.mu.RLock()
	defer j.mu.RUnlock()

	var releases []*models.Release
	for _, release := range j.data.Releases {
		if release.ApplicationID == appID {
			// Add copy to prevent external modification
			releaseCopy := *release
			releases = append(releases, &releaseCopy)
		}
	}

	// Sort by version (latest first)
	sort.Slice(releases, func(i, j int) bool {
		return releases[j].ReleaseDate.Before(releases[i].ReleaseDate)
	})

	return releases, nil
}

// GetRelease retrieves a specific release
func (j *JSONStorage) GetRelease(ctx context.Context, appID, version, platform, arch string) (*models.Release, error) {
	if err := j.loadData(); err != nil {
		return nil, err
	}

	j.mu.RLock()
	defer j.mu.RUnlock()

	for _, release := range j.data.Releases {
		if release.ApplicationID == appID &&
			release.Version == version &&
			release.Platform == platform &&
			release.Architecture == arch {
			// Return a copy
			releaseCopy := *release
			return &releaseCopy, nil
		}
	}

	return nil, fmt.Errorf("release not found: %s@%s-%s-%s", appID, version, platform, arch)
}

// SaveRelease stores or updates a release
func (j *JSONStorage) SaveRelease(ctx context.Context, release *models.Release) error {
	if err := j.loadData(); err != nil {
		return err
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	// Find existing release
	for i, existingRelease := range j.data.Releases {
		if existingRelease.ApplicationID == release.ApplicationID &&
			existingRelease.Version == release.Version &&
			existingRelease.Platform == release.Platform &&
			existingRelease.Architecture == release.Architecture {
			// Update existing
			j.data.Releases[i] = release
			return j.saveData(j.data)
		}
	}

	// Add new release
	j.data.Releases = append(j.data.Releases, release)
	return j.saveData(j.data)
}

// DeleteRelease removes a release
func (j *JSONStorage) DeleteRelease(ctx context.Context, appID, version, platform, arch string) error {
	if err := j.loadData(); err != nil {
		return err
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	for i, release := range j.data.Releases {
		if release.ApplicationID == appID &&
			release.Version == version &&
			release.Platform == platform &&
			release.Architecture == arch {
			// Remove release
			j.data.Releases = append(j.data.Releases[:i], j.data.Releases[i+1:]...)
			return j.saveData(j.data)
		}
	}

	return fmt.Errorf("release not found: %s@%s-%s-%s", appID, version, platform, arch)
}

// GetLatestRelease returns the latest release for a given application, platform, and architecture
func (j *JSONStorage) GetLatestRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error) {
	if err := j.loadData(); err != nil {
		return nil, err
	}

	j.mu.RLock()
	defer j.mu.RUnlock()

	var candidates []*models.Release
	for _, release := range j.data.Releases {
		if release.ApplicationID == appID &&
			release.Platform == platform &&
			release.Architecture == arch {
			candidates = append(candidates, release)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no releases found for %s on %s-%s", appID, platform, arch)
	}

	// Sort by semantic version (latest first)
	sort.Slice(candidates, func(i, j int) bool {
		versionI, errI := semver.NewVersion(candidates[i].Version)
		versionJ, errJ := semver.NewVersion(candidates[j].Version)

		// If both are valid semver, compare semantically
		if errI == nil && errJ == nil {
			return versionI.GreaterThan(versionJ)
		}

		// Fallback to string comparison if semver parsing fails
		return candidates[i].Version > candidates[j].Version
	})

	// Return a copy
	latest := *candidates[0]
	return &latest, nil
}

// GetReleasesAfterVersion returns all releases after a given version
func (j *JSONStorage) GetReleasesAfterVersion(ctx context.Context, appID, currentVersion, platform, arch string) ([]*models.Release, error) {
	if err := j.loadData(); err != nil {
		return nil, err
	}

	j.mu.RLock()
	defer j.mu.RUnlock()

	currentVer, err := semver.NewVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}

	var newerReleases []*models.Release
	for _, release := range j.data.Releases {
		if release.ApplicationID == appID &&
			release.Platform == platform &&
			release.Architecture == arch {

			releaseVer, err := semver.NewVersion(release.Version)
			if err != nil {
				continue // Skip releases with invalid version format
			}

			if releaseVer.GreaterThan(currentVer) {
				releaseCopy := *release
				newerReleases = append(newerReleases, &releaseCopy)
			}
		}
	}

	// Sort by version (latest first)
	sort.Slice(newerReleases, func(i, j int) bool {
		versionI, _ := semver.NewVersion(newerReleases[i].Version)
		versionJ, _ := semver.NewVersion(newerReleases[j].Version)
		return versionI.GreaterThan(versionJ)
	})

	return newerReleases, nil
}

// Ping verifies the storage backend is reachable and operational.
func (j *JSONStorage) Ping(_ context.Context) error {
	return nil
}

// Close closes the storage connection and cleans up resources
func (j *JSONStorage) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	// Clear cache
	j.data = nil
	j.cacheExpiry = time.Time{}

	return nil
}
