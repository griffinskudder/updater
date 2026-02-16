package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"updater/internal/models"

	"github.com/Masterminds/semver/v3"
)

// MemoryStorage implements the Storage interface using in-memory data structures.
// This provider is ideal for development, testing, and scenarios where data
// persistence is not required. It provides fast access but data is lost on restart.
type MemoryStorage struct {
	mu           sync.RWMutex
	applications map[string]*models.Application
	releases     map[string][]*models.Release // key: applicationID
	apiKeys      map[string]*models.APIKey    // keyed by ID
	apiKeyHashes map[string]string            // hash -> ID
}

// NewMemoryStorage creates a new memory-based storage instance
func NewMemoryStorage(config Config) (*MemoryStorage, error) {
	return &MemoryStorage{
		applications: make(map[string]*models.Application),
		releases:     make(map[string][]*models.Release),
		apiKeys:      make(map[string]*models.APIKey),
		apiKeyHashes: make(map[string]string),
	}, nil
}

// Applications returns all registered applications
func (m *MemoryStorage) Applications(ctx context.Context) ([]*models.Application, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]*models.Application, 0, len(m.applications))
	for _, app := range m.applications {
		// Return a copy to prevent external modification
		appCopy := *app
		apps = append(apps, &appCopy)
	}

	return apps, nil
}

// GetApplication retrieves an application by its ID
func (m *MemoryStorage) GetApplication(ctx context.Context, appID string) (*models.Application, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, exists := m.applications[appID]
	if !exists {
		return nil, fmt.Errorf("application %s not found", appID)
	}

	// Return a copy
	appCopy := *app
	return &appCopy, nil
}

// SaveApplication stores or updates an application
func (m *MemoryStorage) SaveApplication(ctx context.Context, app *models.Application) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Store a copy to prevent external modification
	appCopy := *app
	m.applications[app.ID] = &appCopy

	return nil
}

// DeleteApplication removes an application by its ID
func (m *MemoryStorage) DeleteApplication(ctx context.Context, appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.applications[appID]; !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	delete(m.applications, appID)
	return nil
}

// Releases returns all releases for a given application
func (m *MemoryStorage) Releases(ctx context.Context, appID string) ([]*models.Release, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	releases, exists := m.releases[appID]
	if !exists {
		return []*models.Release{}, nil
	}

	// Return copies to prevent external modification
	result := make([]*models.Release, len(releases))
	for i, release := range releases {
		releaseCopy := *release
		result[i] = &releaseCopy
	}

	// Sort by release date (latest first)
	sort.Slice(result, func(i, j int) bool {
		return result[j].ReleaseDate.Before(result[i].ReleaseDate)
	})

	return result, nil
}

// GetRelease retrieves a specific release by application ID, version, platform, and architecture
func (m *MemoryStorage) GetRelease(ctx context.Context, appID, version, platform, arch string) (*models.Release, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	releases, exists := m.releases[appID]
	if !exists {
		return nil, fmt.Errorf("release not found: %s@%s-%s-%s", appID, version, platform, arch)
	}

	for _, release := range releases {
		if release.Version == version &&
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
func (m *MemoryStorage) SaveRelease(ctx context.Context, release *models.Release) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get existing releases for the application
	releases := m.releases[release.ApplicationID]

	// Find existing release to update
	for i, existingRelease := range releases {
		if existingRelease.Version == release.Version &&
			existingRelease.Platform == release.Platform &&
			existingRelease.Architecture == release.Architecture {
			// Update existing release
			releaseCopy := *release
			releases[i] = &releaseCopy
			return nil
		}
	}

	// Add new release
	releaseCopy := *release
	m.releases[release.ApplicationID] = append(releases, &releaseCopy)

	return nil
}

// DeleteRelease removes a release
func (m *MemoryStorage) DeleteRelease(ctx context.Context, appID, version, platform, arch string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	releases, exists := m.releases[appID]
	if !exists {
		return fmt.Errorf("release not found: %s@%s-%s-%s", appID, version, platform, arch)
	}

	for i, release := range releases {
		if release.Version == version &&
			release.Platform == platform &&
			release.Architecture == arch {
			// Remove release
			m.releases[appID] = append(releases[:i], releases[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("release not found: %s@%s-%s-%s", appID, version, platform, arch)
}

// GetLatestRelease returns the latest release for a given application, platform, and architecture
func (m *MemoryStorage) GetLatestRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	releases, exists := m.releases[appID]
	if !exists {
		return nil, fmt.Errorf("no releases found for %s on %s-%s", appID, platform, arch)
	}

	var candidates []*models.Release
	for _, release := range releases {
		if release.Platform == platform && release.Architecture == arch {
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

// GetReleasesAfterVersion returns all releases after a given version for a specific platform/arch
func (m *MemoryStorage) GetReleasesAfterVersion(ctx context.Context, appID, currentVersion, platform, arch string) ([]*models.Release, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	currentVer, err := semver.NewVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}

	releases, exists := m.releases[appID]
	if !exists {
		return []*models.Release{}, nil
	}

	var newerReleases []*models.Release
	for _, release := range releases {
		if release.Platform == platform && release.Architecture == arch {
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
func (m *MemoryStorage) Ping(_ context.Context) error {
	return nil
}

// Close closes the storage connection and cleans up resources
func (m *MemoryStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear all data
	m.applications = make(map[string]*models.Application)
	m.releases = make(map[string][]*models.Release)
	m.apiKeys = make(map[string]*models.APIKey)
	m.apiKeyHashes = make(map[string]string)

	return nil
}

// CreateAPIKey stores a new API key in memory.
func (m *MemoryStorage) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := *key
	m.apiKeys[key.ID] = &c
	m.apiKeyHashes[key.KeyHash] = key.ID
	return nil
}

// GetAPIKeyByHash retrieves an API key by its SHA-256 hash.
// Returns ErrNotFound if no matching key exists.
func (m *MemoryStorage) GetAPIKeyByHash(ctx context.Context, hash string) (*models.APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.apiKeyHashes[hash]
	if !ok {
		return nil, ErrNotFound
	}
	c := *m.apiKeys[id]
	return &c, nil
}

// ListAPIKeys returns all API keys (both enabled and disabled).
func (m *MemoryStorage) ListAPIKeys(ctx context.Context) ([]*models.APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*models.APIKey, 0, len(m.apiKeys))
	for _, k := range m.apiKeys {
		c := *k
		out = append(out, &c)
	}
	return out, nil
}

// UpdateAPIKey replaces the mutable fields of an existing API key.
// Returns ErrNotFound if the key does not exist.
func (m *MemoryStorage) UpdateAPIKey(ctx context.Context, key *models.APIKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.apiKeys[key.ID]
	if !ok {
		return ErrNotFound
	}
	if existing.KeyHash != key.KeyHash {
		delete(m.apiKeyHashes, existing.KeyHash)
		m.apiKeyHashes[key.KeyHash] = key.ID
	}
	c := *key
	m.apiKeys[key.ID] = &c
	return nil
}

// DeleteAPIKey permanently removes an API key by ID.
// Returns ErrNotFound if the key does not exist.
func (m *MemoryStorage) DeleteAPIKey(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.apiKeys[id]
	if !ok {
		return ErrNotFound
	}
	delete(m.apiKeyHashes, k.KeyHash)
	delete(m.apiKeys, id)
	return nil
}
