package storage

import (
	"context"
	"updater/internal/models"
)

// Storage defines the interface for release metadata persistence and retrieval.
// It provides a clean abstraction that can be implemented by different backends
// such as JSON files, databases, or external APIs.
type Storage interface {
	// GetApplication retrieves an application by its ID
	GetApplication(ctx context.Context, appID string) (*models.Application, error)

	// SaveApplication stores or updates an application
	SaveApplication(ctx context.Context, app *models.Application) error

	// DeleteApplication removes an application by its ID
	DeleteApplication(ctx context.Context, appID string) error

	// GetRelease retrieves a specific release by application ID, version, platform, and architecture
	GetRelease(ctx context.Context, appID, version, platform, arch string) (*models.Release, error)

	// SaveRelease stores or updates a release
	SaveRelease(ctx context.Context, release *models.Release) error

	// DeleteRelease removes a release
	DeleteRelease(ctx context.Context, appID, version, platform, arch string) error

	// GetLatestRelease returns the latest release for a given application, platform, and architecture
	GetLatestRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error)

	// GetReleasesAfterVersion returns all releases after a given version for a specific platform/arch
	GetReleasesAfterVersion(ctx context.Context, appID, currentVersion, platform, arch string) ([]*models.Release, error)

	// Ping verifies the storage backend is reachable and operational
	Ping(ctx context.Context) error

	// Close closes the storage connection and cleans up resources
	Close() error

	// CreateAPIKey stores a new API key.
	CreateAPIKey(ctx context.Context, key *models.APIKey) error

	// GetAPIKeyByHash retrieves an API key by its SHA-256 hash.
	// Returns storage.ErrNotFound if no matching key exists.
	GetAPIKeyByHash(ctx context.Context, hash string) (*models.APIKey, error)

	// ListAPIKeys returns all API keys (both enabled and disabled).
	ListAPIKeys(ctx context.Context) ([]*models.APIKey, error)

	// UpdateAPIKey replaces the mutable fields of an existing API key.
	UpdateAPIKey(ctx context.Context, key *models.APIKey) error

	// DeleteAPIKey permanently removes an API key by ID.
	DeleteAPIKey(ctx context.Context, id string) error

	// ListApplicationsPaged returns a page of applications sorted by created_at DESC, id DESC,
	// and the total count of all applications.
	// cursor, when non-nil, positions the query after the given item for keyset pagination.
	ListApplicationsPaged(ctx context.Context, limit int, cursor *models.ApplicationCursor) ([]*models.Application, int, error)

	// ListReleasesPaged returns a filtered, sorted page of releases for an application,
	// and the total count of matching releases.
	// sortBy must be one of: release_date, version, platform, architecture, created_at.
	// sortOrder must be "asc" or "desc".
	// cursor, when non-nil, positions the query after the given item for keyset pagination.
	ListReleasesPaged(ctx context.Context, appID string, filters models.ReleaseFilters, sortBy, sortOrder string, limit int, cursor *models.ReleaseCursor) ([]*models.Release, int, error)

	// GetLatestStableRelease returns the highest non-prerelease version for the given
	// application, platform, and architecture.
	// Returns storage.ErrNotFound if no stable release exists.
	GetLatestStableRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error)

	// GetApplicationStats returns aggregate statistics for an application.
	GetApplicationStats(ctx context.Context, appID string) (models.ApplicationStats, error)
}
