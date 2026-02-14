package update

import (
	"context"
	"updater/internal/models"
)

// ServiceInterface defines the interface for update service operations
type ServiceInterface interface {
	// CheckForUpdate determines if an update is available for the given request
	CheckForUpdate(ctx context.Context, req *models.UpdateCheckRequest) (*models.UpdateCheckResponse, error)

	// GetLatestVersion returns the latest version information for the given request
	GetLatestVersion(ctx context.Context, req *models.LatestVersionRequest) (*models.LatestVersionResponse, error)

	// ListReleases returns a paginated list of releases for the given request
	ListReleases(ctx context.Context, req *models.ListReleasesRequest) (*models.ListReleasesResponse, error)

	// RegisterRelease creates a new release from the given request
	RegisterRelease(ctx context.Context, req *models.RegisterReleaseRequest) (*models.RegisterReleaseResponse, error)
}

// Ensure Service implements ServiceInterface
var _ ServiceInterface = (*Service)(nil)
