package storage

import (
	"context"
	"updater/internal/models"
)

// Storage defines the interface for release metadata persistence and retrieval.
// It provides a clean abstraction that can be implemented by different backends
// such as JSON files, databases, or external APIs.
type Storage interface {
	// Applications returns all registered applications
	Applications(ctx context.Context) ([]*models.Application, error)

	// GetApplication retrieves an application by its ID
	GetApplication(ctx context.Context, appID string) (*models.Application, error)

	// SaveApplication stores or updates an application
	SaveApplication(ctx context.Context, app *models.Application) error

	// Releases returns all releases for a given application
	Releases(ctx context.Context, appID string) ([]*models.Release, error)

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

	// Close closes the storage connection and cleans up resources
	Close() error
}

// Config holds configuration for storage backends
type Config struct {
	// Type specifies the storage backend type (json, database, etc.)
	Type string `json:"type" yaml:"type"`

	// Path is used for file-based storage backends
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// ConnectionString is used for database backends
	ConnectionString string `json:"connection_string,omitempty" yaml:"connection_string,omitempty"`

	// CacheTTL specifies how long to cache data in memory
	CacheTTL string `json:"cache_ttl,omitempty" yaml:"cache_ttl,omitempty"`

	// Additional options for specific backends
	Options map[string]interface{} `json:"options,omitempty" yaml:"options,omitempty"`
}