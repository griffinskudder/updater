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

	// CreateApplication creates a new application
	CreateApplication(ctx context.Context, req *models.CreateApplicationRequest) (*models.CreateApplicationResponse, error)

	// GetApplication retrieves an application by ID with computed statistics
	GetApplication(ctx context.Context, appID string) (*models.ApplicationInfoResponse, error)

	// ListApplications returns a paginated list of applications
	ListApplications(ctx context.Context, limit, offset int) (*models.ListApplicationsResponse, error)

	// UpdateApplication applies partial updates to an existing application
	UpdateApplication(ctx context.Context, appID string, req *models.UpdateApplicationRequest) (*models.UpdateApplicationResponse, error)

	// DeleteApplication removes an application that has no existing releases
	DeleteApplication(ctx context.Context, appID string) error

	// DeleteRelease removes a specific release
	DeleteRelease(ctx context.Context, appID, version, platform, arch string) (*models.DeleteReleaseResponse, error)
}

// Ensure Service implements ServiceInterface
var _ ServiceInterface = (*Service)(nil)
