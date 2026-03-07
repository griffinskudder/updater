package update

import (
	"context"
	"errors"
	"fmt"
	"time"
	"updater/internal/models"
	"updater/internal/storage"

	"github.com/Masterminds/semver/v3"
)

// Service handles update checking and version comparison business logic
type Service struct {
	storage storage.Storage
}

// NewService creates a new update service with the given storage backend
func NewService(storage storage.Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// CheckForUpdate determines if there's an update available for the given request
func (s *Service) CheckForUpdate(ctx context.Context, req *models.UpdateCheckRequest) (*models.UpdateCheckResponse, error) {
	// Validate and normalize request
	if err := req.Validate(); err != nil {
		return nil, NewValidationError("invalid request", err)
	}
	req.Normalize()

	// Get application to verify it exists and supports the platform
	app, err := s.storage.GetApplication(ctx, req.ApplicationID)
	if err != nil {
		return nil, NewApplicationNotFoundError(req.ApplicationID)
	}

	// Check if application supports the requested platform
	if !app.SupportsPlatform(req.Platform) {
		return nil, NewInvalidRequestError(
			fmt.Sprintf("application %s does not support platform %s", req.ApplicationID, req.Platform),
			nil,
		)
	}

	// Get the latest available release for this platform/architecture
	latestRelease, err := s.storage.GetLatestRelease(ctx, req.ApplicationID, req.Platform, req.Architecture)
	if err != nil {
		return nil, NewInternalError("failed to get latest release", err)
	}

	// Parse current and latest versions for comparison
	currentVersion, err := semver.NewVersion(req.CurrentVersion)
	if err != nil {
		return nil, NewValidationError("invalid current version format", err)
	}

	latestVersion, err := semver.NewVersion(latestRelease.Version)
	if err != nil {
		return nil, NewInternalError("invalid latest version format", err)
	}

	response := &models.UpdateCheckResponse{
		CurrentVersion: req.CurrentVersion,
	}

	// Check if an update is available
	if latestVersion.GreaterThan(currentVersion) {
		// Check pre-release handling
		if !req.AllowPrerelease && latestVersion.Prerelease() != "" {
			stableRelease, err := s.storage.GetLatestStableRelease(ctx, req.ApplicationID, req.Platform, req.Architecture)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					response.SetNoUpdateAvailable(req.CurrentVersion)
					return response, nil
				}
				return nil, NewInternalError("failed to find stable release", err)
			}
			stableVer, err := semver.NewVersion(stableRelease.Version)
			if err != nil {
				return nil, NewInternalError("invalid stable release version", err)
			}
			if !stableVer.GreaterThan(currentVersion) {
				response.SetNoUpdateAvailable(req.CurrentVersion)
				return response, nil
			}
			latestRelease = stableRelease
		}

		// Check minimum version requirement
		if latestRelease.MinimumVersion != "" {
			meets, err := latestRelease.MeetsMinimumVersion(req.CurrentVersion)
			if err != nil {
				return nil, NewInternalError("failed to check minimum version", err)
			}
			if !meets {
				return nil, NewInvalidRequestError(fmt.Sprintf("current version %s does not meet minimum required version %s for update to %s",
					req.CurrentVersion, latestRelease.MinimumVersion, latestRelease.Version), nil)
			}
		}

		// Update is available
		response.SetUpdateAvailable(latestRelease)

		// Include metadata if requested
		if !req.IncludeMetadata {
			response.Metadata = nil
		}
	} else {
		// No update available
		response.SetNoUpdateAvailable(req.CurrentVersion)
	}

	return response, nil
}

// GetLatestVersion returns the latest version information for the given request
func (s *Service) GetLatestVersion(ctx context.Context, req *models.LatestVersionRequest) (*models.LatestVersionResponse, error) {
	// Validate and normalize request
	if err := req.Validate(); err != nil {
		return nil, NewValidationError("invalid request", err)
	}
	req.Normalize()

	// Get application to verify it exists and supports the platform
	app, err := s.storage.GetApplication(ctx, req.ApplicationID)
	if err != nil {
		return nil, NewApplicationNotFoundError(req.ApplicationID)
	}

	// Check if application supports the requested platform
	if !app.SupportsPlatform(req.Platform) {
		return nil, NewInvalidRequestError(
			fmt.Sprintf("application %s does not support platform %s", req.ApplicationID, req.Platform),
			nil,
		)
	}

	// Get the latest available release for this platform/architecture
	latestRelease, err := s.storage.GetLatestRelease(ctx, req.ApplicationID, req.Platform, req.Architecture)
	if err != nil {
		return nil, NewInternalError("failed to get latest release", err)
	}

	// Handle pre-release filtering
	if !req.AllowPrerelease {
		latestVersion, err := semver.NewVersion(latestRelease.Version)
		if err != nil {
			return nil, NewInternalError("invalid latest version", err)
		}

		if latestVersion.Prerelease() != "" {
			stableRelease, err := s.storage.GetLatestStableRelease(ctx, req.ApplicationID, req.Platform, req.Architecture)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					return nil, NewApplicationNotFoundError(fmt.Sprintf("%s on %s-%s (no stable releases)", req.ApplicationID, req.Platform, req.Architecture))
				}
				return nil, NewInternalError("failed to find stable release", err)
			}
			latestRelease = stableRelease
		}
	}

	response := &models.LatestVersionResponse{}
	response.FromRelease(latestRelease)

	// Include metadata if requested
	if !req.IncludeMetadata {
		response.Metadata = nil
	}

	return response, nil
}

// ListReleases returns a filtered list of releases for the given request
func (s *Service) ListReleases(ctx context.Context, req *models.ListReleasesRequest) (*models.ListReleasesResponse, error) {
	// Validate and normalize request
	if err := req.Validate(); err != nil {
		return nil, NewValidationError("invalid request", err)
	}
	req.Normalize()

	var cursor *models.ReleaseCursor
	if req.After != "" {
		var err error
		cursor, err = models.DecodeReleaseCursor(req.After)
		if err != nil {
			return nil, NewValidationError("invalid cursor", err)
		}
		if cursor.SortBy != req.SortBy || cursor.SortOrder != req.SortOrder {
			return nil, NewValidationError("cursor sort_by/sort_order mismatch", nil)
		}
	}

	filters := models.ReleaseFilters{
		Architecture: req.Architecture,
		Version:      req.Version,
		Required:     req.Required,
	}
	if req.Platform != "" {
		filters.Platforms = []string{req.Platform}
	} else {
		filters.Platforms = req.Platforms
	}

	releases, totalCount, err := s.storage.ListReleasesPaged(ctx, req.ApplicationID, filters, req.SortBy, req.SortOrder, req.Limit, cursor)
	if err != nil {
		return nil, NewInternalError("failed to get releases", err)
	}

	releaseInfos := make([]models.ReleaseInfo, len(releases))
	for i, release := range releases {
		releaseInfos[i].FromRelease(release)
	}

	var nextCursor string
	if len(releases) > 0 && len(releases) == req.Limit {
		last := releases[len(releases)-1]
		c := &models.ReleaseCursor{
			SortBy:       req.SortBy,
			SortOrder:    req.SortOrder,
			ID:           last.ID,
			ReleaseDate:  last.ReleaseDate,
			Platform:     last.Platform,
			Architecture: last.Architecture,
			CreatedAt:    last.CreatedAt,
		}
		generateCursor := true
		if req.SortBy == "version" {
			sv, err := semver.NewVersion(last.Version)
			if err != nil {
				// Cannot generate a keyset cursor for version sort when the last item has a
				// non-semver version string. Pagination appears complete. This should not
				// occur in normal operation since the API validates versions on write.
				generateCursor = false
			} else {
				c.VersionMajor = int64(sv.Major()) //#nosec G115 -- components validated at API layer, within int64 range
				c.VersionMinor = int64(sv.Minor()) //#nosec G115 -- components validated at API layer, within int64 range
				c.VersionPatch = int64(sv.Patch()) //#nosec G115 -- components validated at API layer, within int64 range
				c.VersionIsStable = sv.Prerelease() == ""
				c.VersionPreRelease = sv.Prerelease()
			}
		}
		if generateCursor {
			encoded, err := c.Encode()
			if err != nil {
				return nil, NewInternalError("failed to encode pagination cursor", err)
			}
			nextCursor = encoded
		}
	}

	return &models.ListReleasesResponse{
		Releases:   releaseInfos,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil
}

// RegisterRelease creates a new release from the given request
func (s *Service) RegisterRelease(ctx context.Context, req *models.RegisterReleaseRequest) (*models.RegisterReleaseResponse, error) {
	// Validate and normalize request
	if err := req.Validate(); err != nil {
		return nil, NewValidationError("invalid request", err)
	}
	req.Normalize()

	// Verify application exists
	app, err := s.storage.GetApplication(ctx, req.ApplicationID)
	if err != nil {
		return nil, NewApplicationNotFoundError(req.ApplicationID)
	}

	// Check if application supports the platform
	if !app.SupportsPlatform(req.Platform) {
		return nil, NewInvalidRequestError(fmt.Sprintf("application %s does not support platform %s", req.ApplicationID, req.Platform), nil)
	}

	// Create release from request
	release := models.NewRelease(req.ApplicationID, req.Version, req.Platform, req.Architecture, req.DownloadURL)
	release.Checksum = req.Checksum
	release.ChecksumType = req.ChecksumType
	release.FileSize = req.FileSize
	release.ReleaseNotes = req.ReleaseNotes
	release.Required = req.Required
	release.MinimumVersion = req.MinimumVersion

	// Copy metadata
	if req.Metadata != nil {
		release.Metadata = make(map[string]string)
		for k, v := range req.Metadata {
			release.Metadata[k] = v
		}
	}

	// Validate the created release
	if err := release.Validate(); err != nil {
		return nil, NewValidationError("invalid release", err)
	}

	// Save the release
	if err := s.storage.SaveRelease(ctx, release); err != nil {
		return nil, NewInternalError("failed to save release", err)
	}

	return &models.RegisterReleaseResponse{
		ID:        release.ID,
		Message:   fmt.Sprintf("Release %s registered successfully", release.Version),
		CreatedAt: release.CreatedAt,
	}, nil
}

// CreateApplication creates a new application after validating and normalizing the request.
func (s *Service) CreateApplication(ctx context.Context, req *models.CreateApplicationRequest) (*models.CreateApplicationResponse, error) {
	// Validate and normalize request
	if err := req.Validate(); err != nil {
		return nil, NewValidationError("invalid request", err)
	}
	req.Normalize()

	// Check for duplicate ID
	if _, err := s.storage.GetApplication(ctx, req.ID); err == nil {
		return nil, NewConflictError(fmt.Sprintf("application '%s' already exists", req.ID))
	}

	// Create application with defaults
	app := models.NewApplication(req.ID, req.Name, req.Platforms)
	app.Description = req.Description
	app.Config = req.Config
	now := time.Now().Format(time.RFC3339)
	app.CreatedAt = now
	app.UpdatedAt = now

	// Save application
	if err := s.storage.SaveApplication(ctx, app); err != nil {
		return nil, NewInternalError("failed to save application", err)
	}

	createdAt, _ := time.Parse(time.RFC3339, app.CreatedAt)
	return &models.CreateApplicationResponse{
		ID:        app.ID,
		Message:   fmt.Sprintf("Application '%s' created successfully", app.ID),
		CreatedAt: createdAt,
	}, nil
}

// GetApplication retrieves an application by ID with computed statistics.
func (s *Service) GetApplication(ctx context.Context, appID string) (*models.ApplicationInfoResponse, error) {
	app, err := s.storage.GetApplication(ctx, appID)
	if err != nil {
		return nil, NewApplicationNotFoundError(appID)
	}

	stats, err := s.storage.GetApplicationStats(ctx, appID)
	if err != nil {
		return nil, NewInternalError("failed to get application stats", err)
	}

	createdAt, _ := time.Parse(time.RFC3339, app.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, app.UpdatedAt)

	return &models.ApplicationInfoResponse{
		ID:          app.ID,
		Name:        app.Name,
		Description: app.Description,
		Platforms:   app.Platforms,
		Config:      app.Config,
		Stats:       stats,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// ListApplications returns a paginated list of applications.
func (s *Service) ListApplications(ctx context.Context, req *models.ListApplicationsRequest) (*models.ListApplicationsResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, NewValidationError("invalid request", err)
	}
	req.Normalize()

	var cursor *models.ApplicationCursor
	if req.After != "" {
		var err error
		cursor, err = models.DecodeApplicationCursor(req.After)
		if err != nil {
			return nil, NewValidationError("invalid cursor", err)
		}
	}

	apps, totalCount, err := s.storage.ListApplicationsPaged(ctx, req.Limit, cursor)
	if err != nil {
		return nil, NewInternalError("failed to list applications", err)
	}

	summaries := make([]models.ApplicationSummary, len(apps))
	for i, app := range apps {
		summaries[i].FromApplication(app)
		summaries[i].CreatedAt, _ = time.Parse(time.RFC3339, app.CreatedAt)
		summaries[i].UpdatedAt, _ = time.Parse(time.RFC3339, app.UpdatedAt)
	}

	var nextCursor string
	if len(apps) > 0 && len(apps) == req.Limit {
		last := apps[len(apps)-1]
		createdAt, _ := time.Parse(time.RFC3339, last.CreatedAt)
		c := &models.ApplicationCursor{
			CreatedAt: createdAt,
			ID:        last.ID,
		}
		encoded, err := c.Encode()
		if err != nil {
			return nil, NewInternalError("failed to encode pagination cursor", err)
		}
		nextCursor = encoded
	}

	return &models.ListApplicationsResponse{
		Applications: summaries,
		TotalCount:   totalCount,
		NextCursor:   nextCursor,
	}, nil
}

// UpdateApplication applies partial updates to an existing application.
func (s *Service) UpdateApplication(ctx context.Context, appID string, req *models.UpdateApplicationRequest) (*models.UpdateApplicationResponse, error) {
	// Validate and normalize request
	if err := req.Validate(); err != nil {
		return nil, NewValidationError("invalid request", err)
	}
	req.Normalize()

	// Fetch existing application
	app, err := s.storage.GetApplication(ctx, appID)
	if err != nil {
		return nil, NewApplicationNotFoundError(appID)
	}

	// Apply partial updates
	if req.Name != nil {
		app.Name = *req.Name
	}
	if req.Description != nil {
		app.Description = *req.Description
	}
	if req.Platforms != nil {
		app.Platforms = req.Platforms
	}
	if req.Config != nil {
		app.Config = *req.Config
	}

	// Update timestamp
	now := time.Now()
	app.UpdatedAt = now.Format(time.RFC3339)

	// Save updated application
	if err := s.storage.SaveApplication(ctx, app); err != nil {
		return nil, NewInternalError("failed to save application", err)
	}

	return &models.UpdateApplicationResponse{
		ID:        app.ID,
		Message:   fmt.Sprintf("Application '%s' updated successfully", app.ID),
		UpdatedAt: now,
	}, nil
}

// DeleteApplication removes an application that has no existing releases.
func (s *Service) DeleteApplication(ctx context.Context, appID string) error {
	// Verify application exists
	if _, err := s.storage.GetApplication(ctx, appID); err != nil {
		return NewApplicationNotFoundError(appID)
	}

	// Delete application
	if err := s.storage.DeleteApplication(ctx, appID); err != nil {
		if errors.Is(err, storage.ErrHasDependencies) {
			return NewConflictError(fmt.Sprintf("cannot delete application '%s': has existing releases", appID))
		}
		return NewInternalError("failed to delete application", err)
	}

	return nil
}

// DeleteRelease removes a specific release.
func (s *Service) DeleteRelease(ctx context.Context, appID, version, platform, arch string) (*models.DeleteReleaseResponse, error) {
	// Verify release exists
	release, err := s.storage.GetRelease(ctx, appID, version, platform, arch)
	if err != nil {
		return nil, NewNotFoundError(fmt.Sprintf("release '%s-%s-%s-%s' not found", appID, version, platform, arch))
	}

	// Delete release
	if err := s.storage.DeleteRelease(ctx, appID, version, platform, arch); err != nil {
		return nil, NewInternalError("failed to delete release", err)
	}

	return &models.DeleteReleaseResponse{
		ID:      release.ID,
		Message: fmt.Sprintf("Release '%s' deleted successfully", release.ID),
	}, nil
}
