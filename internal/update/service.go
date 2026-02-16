package update

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
			latestStable, err := s.findLatestStableRelease(ctx, req.ApplicationID, req.Platform, req.Architecture, req.CurrentVersion)
			if err != nil {
				return nil, NewInternalError("failed to find stable release", err)
			}

			if latestStable != nil {
				latestRelease = latestStable
			} else {
				// No stable update available
				response.SetNoUpdateAvailable(req.CurrentVersion)
				return response, nil
			}
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
			latestStable, err := s.findLatestStableRelease(ctx, req.ApplicationID, req.Platform, req.Architecture, "")
			if err != nil {
				return nil, NewInternalError("failed to find stable release", err)
			}

			if latestStable != nil {
				latestRelease = latestStable
			} else {
				return nil, NewApplicationNotFoundError(fmt.Sprintf("%s on %s-%s (no stable releases)", req.ApplicationID, req.Platform, req.Architecture))
			}
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

	// Get all releases for the application
	allReleases, err := s.storage.Releases(ctx, req.ApplicationID)
	if err != nil {
		return nil, NewInternalError("failed to get releases", err)
	}

	// Apply filters
	var filteredReleases []*models.Release
	for _, release := range allReleases {
		// Platform filter
		if req.Platform != "" && release.Platform != req.Platform {
			continue
		}

		// Architecture filter
		if req.Architecture != "" && release.Architecture != req.Architecture {
			continue
		}

		// Version filter
		if req.Version != "" && release.Version != req.Version {
			continue
		}

		// Required filter
		if req.Required != nil && release.Required != *req.Required {
			continue
		}

		// Platforms filter (multiple platforms)
		if len(req.Platforms) > 0 {
			found := false
			for _, platform := range req.Platforms {
				if release.Platform == platform {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filteredReleases = append(filteredReleases, release)
	}

	// Sort releases
	s.sortReleases(filteredReleases, req.SortBy, req.SortOrder)

	// Apply pagination
	totalCount := len(filteredReleases)
	start := req.Offset
	end := start + req.Limit

	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}

	paginatedReleases := filteredReleases[start:end]

	// Convert to response format
	releaseInfos := make([]models.ReleaseInfo, len(paginatedReleases))
	for i, release := range paginatedReleases {
		releaseInfos[i].FromRelease(release)
	}

	return &models.ListReleasesResponse{
		Releases:   releaseInfos,
		TotalCount: totalCount,
		Page:       (req.Offset / req.Limit) + 1,
		PageSize:   req.Limit,
		HasMore:    end < totalCount,
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

// sortReleases sorts releases based on the specified field and order
func (s *Service) sortReleases(releases []*models.Release, sortBy, sortOrder string) {
	if len(releases) <= 1 {
		return
	}

	// Default ascending comparison function
	less := func(i, j int) bool {
		switch sortBy {
		case "version":
			versionI, errI := semver.NewVersion(releases[i].Version)
			versionJ, errJ := semver.NewVersion(releases[j].Version)
			if errI == nil && errJ == nil {
				return versionI.LessThan(versionJ)
			}
			return releases[i].Version < releases[j].Version
		case "release_date":
			return releases[i].ReleaseDate.Before(releases[j].ReleaseDate)
		case "platform":
			return releases[i].Platform < releases[j].Platform
		case "architecture":
			return releases[i].Architecture < releases[j].Architecture
		case "created_at":
			return releases[i].CreatedAt.Before(releases[j].CreatedAt)
		default:
			return releases[i].ReleaseDate.Before(releases[j].ReleaseDate)
		}
	}

	// Reverse comparison for descending order
	if sortOrder == "desc" {
		originalLess := less
		less = func(i, j int) bool {
			return originalLess(j, i)
		}
	}

	// Sort the releases
	sort.Slice(releases, less)
}

// findLatestStableRelease finds the latest non-prerelease version that's newer than the current version
// for the specified platform and architecture. Returns nil if no suitable stable release is found.
func (s *Service) findLatestStableRelease(ctx context.Context, appID, platform, arch, currentVersion string) (*models.Release, error) {
	releases, err := s.storage.Releases(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get releases: %w", err)
	}

	var currentVer *semver.Version
	if currentVersion != "" {
		currentVer, err = semver.NewVersion(currentVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid current version: %w", err)
		}
	}

	var latestStable *models.Release
	for _, release := range releases {
		if release.Platform != platform || release.Architecture != arch {
			continue
		}

		releaseVer, err := semver.NewVersion(release.Version)
		if err != nil {
			continue
		}

		// Skip pre-release versions
		if releaseVer.Prerelease() != "" {
			continue
		}

		// Skip if not newer than current version
		if currentVer != nil && !releaseVer.GreaterThan(currentVer) {
			continue
		}

		// Check if this is the latest stable version we've seen
		if latestStable == nil {
			latestStable = release
			continue
		}

		stableVer, err := semver.NewVersion(latestStable.Version)
		if err != nil {
			latestStable = release
			continue
		}

		if releaseVer.GreaterThan(stableVer) {
			latestStable = release
		}
	}

	return latestStable, nil
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

	// Fetch releases to compute stats
	releases, err := s.storage.Releases(ctx, appID)
	if err != nil {
		return nil, NewInternalError("failed to get releases", err)
	}

	stats := computeApplicationStats(releases)

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
func (s *Service) ListApplications(ctx context.Context, limit, offset int) (*models.ListApplicationsResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	allApps, err := s.storage.Applications(ctx)
	if err != nil {
		return nil, NewInternalError("failed to list applications", err)
	}

	totalCount := len(allApps)

	// Apply pagination
	start := offset
	end := start + limit
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}

	paginatedApps := allApps[start:end]

	summaries := make([]models.ApplicationSummary, len(paginatedApps))
	for i, app := range paginatedApps {
		summaries[i].FromApplication(app)
		summaries[i].CreatedAt, _ = time.Parse(time.RFC3339, app.CreatedAt)
		summaries[i].UpdatedAt, _ = time.Parse(time.RFC3339, app.UpdatedAt)
	}

	return &models.ListApplicationsResponse{
		Applications: summaries,
		TotalCount:   totalCount,
		Page:         (offset / limit) + 1,
		PageSize:     limit,
		HasMore:      end < totalCount,
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

	// Check for existing releases
	releases, err := s.storage.Releases(ctx, appID)
	if err != nil {
		return NewInternalError("failed to check releases", err)
	}
	if len(releases) > 0 {
		return NewConflictError(fmt.Sprintf("cannot delete application '%s': has %d existing releases", appID, len(releases)))
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

// computeApplicationStats computes statistics from a list of releases.
func computeApplicationStats(releases []*models.Release) models.ApplicationStats {
	stats := models.ApplicationStats{
		TotalReleases: len(releases),
	}

	if len(releases) == 0 {
		return stats
	}

	platforms := make(map[string]struct{})
	var latestVersion *semver.Version
	var latestReleaseDate *time.Time

	for _, release := range releases {
		platforms[release.Platform] = struct{}{}

		if release.Required {
			stats.RequiredReleases++
		}

		// Track latest version
		ver, err := semver.NewVersion(release.Version)
		if err == nil {
			if latestVersion == nil || ver.GreaterThan(latestVersion) {
				latestVersion = ver
				stats.LatestVersion = release.Version
			}
		}

		// Track latest release date
		if latestReleaseDate == nil || release.ReleaseDate.After(*latestReleaseDate) {
			rd := release.ReleaseDate
			latestReleaseDate = &rd
		}
	}

	stats.PlatformCount = len(platforms)
	stats.LatestReleaseDate = latestReleaseDate

	return stats
}
