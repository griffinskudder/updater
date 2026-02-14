package update

import (
	"context"
	"fmt"
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
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	req.Normalize()

	// Get application to verify it exists and supports the platform
	app, err := s.storage.GetApplication(ctx, req.ApplicationID)
	if err != nil {
		return nil, fmt.Errorf("application not found: %w", err)
	}

	// Check if application supports the requested platform
	if !app.SupportsPlatform(req.Platform) {
		return nil, fmt.Errorf("application %s does not support platform %s", req.ApplicationID, req.Platform)
	}

	// Get the latest available release for this platform/architecture
	latestRelease, err := s.storage.GetLatestRelease(ctx, req.ApplicationID, req.Platform, req.Architecture)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}

	// Parse current and latest versions for comparison
	currentVersion, err := semver.NewVersion(req.CurrentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}

	latestVersion, err := semver.NewVersion(latestRelease.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid latest version: %w", err)
	}

	response := &models.UpdateCheckResponse{
		CurrentVersion: req.CurrentVersion,
	}

	// Check if an update is available
	if latestVersion.GreaterThan(currentVersion) {
		// Check pre-release handling
		if !req.AllowPrerelease && latestVersion.Prerelease() != "" {
			// Look for the latest non-prerelease version
			releases, err := s.storage.Releases(ctx, req.ApplicationID)
			if err != nil {
				return nil, fmt.Errorf("failed to get releases: %w", err)
			}

			var latestStable *models.Release
			for _, release := range releases {
				if release.Platform != req.Platform || release.Architecture != req.Architecture {
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

				// Skip if not newer than current
				if !releaseVer.GreaterThan(currentVersion) {
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
				return nil, fmt.Errorf("failed to check minimum version: %w", err)
			}
			if !meets {
				return nil, fmt.Errorf("current version %s does not meet minimum required version %s for update to %s",
					req.CurrentVersion, latestRelease.MinimumVersion, latestRelease.Version)
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
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	req.Normalize()

	// Get application to verify it exists and supports the platform
	app, err := s.storage.GetApplication(ctx, req.ApplicationID)
	if err != nil {
		return nil, fmt.Errorf("application not found: %w", err)
	}

	// Check if application supports the requested platform
	if !app.SupportsPlatform(req.Platform) {
		return nil, fmt.Errorf("application %s does not support platform %s", req.ApplicationID, req.Platform)
	}

	// Get the latest available release for this platform/architecture
	latestRelease, err := s.storage.GetLatestRelease(ctx, req.ApplicationID, req.Platform, req.Architecture)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}

	// Handle pre-release filtering
	if !req.AllowPrerelease {
		latestVersion, err := semver.NewVersion(latestRelease.Version)
		if err != nil {
			return nil, fmt.Errorf("invalid latest version: %w", err)
		}

		if latestVersion.Prerelease() != "" {
			// Look for the latest non-prerelease version
			releases, err := s.storage.Releases(ctx, req.ApplicationID)
			if err != nil {
				return nil, fmt.Errorf("failed to get releases: %w", err)
			}

			var latestStable *models.Release
			for _, release := range releases {
				if release.Platform != req.Platform || release.Architecture != req.Architecture {
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

			if latestStable != nil {
				latestRelease = latestStable
			} else {
				return nil, fmt.Errorf("no stable releases found for %s on %s-%s", req.ApplicationID, req.Platform, req.Architecture)
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
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	req.Normalize()

	// Get all releases for the application
	allReleases, err := s.storage.Releases(ctx, req.ApplicationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get releases: %w", err)
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
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	req.Normalize()

	// Verify application exists
	app, err := s.storage.GetApplication(ctx, req.ApplicationID)
	if err != nil {
		return nil, fmt.Errorf("application not found: %w", err)
	}

	// Check if application supports the platform
	if !app.SupportsPlatform(req.Platform) {
		return nil, fmt.Errorf("application %s does not support platform %s", req.ApplicationID, req.Platform)
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
		return nil, fmt.Errorf("invalid release: %w", err)
	}

	// Save the release
	if err := s.storage.SaveRelease(ctx, release); err != nil {
		return nil, fmt.Errorf("failed to save release: %w", err)
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
			return !originalLess(i, j)
		}
	}

	// Sort the releases
	for i := 0; i < len(releases)-1; i++ {
		for j := 0; j < len(releases)-i-1; j++ {
			if less(j+1, j) {
				releases[j], releases[j+1] = releases[j+1], releases[j]
			}
		}
	}
}
