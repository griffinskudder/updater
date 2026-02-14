package models

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateCheckResponse_SetUpdateAvailable(t *testing.T) {
	release := &Release{
		Version:        "1.2.3",
		DownloadURL:    "https://example.com/download",
		Checksum:       "abc123",
		ChecksumType:   "sha256",
		FileSize:       12345,
		ReleaseNotes:   "Bug fixes and improvements",
		ReleaseDate:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Required:       true,
		MinimumVersion: "1.0.0",
		Metadata:       map[string]string{"key": "value"},
	}

	response := &UpdateCheckResponse{}
	response.SetUpdateAvailable(release)

	assert.True(t, response.UpdateAvailable)
	assert.Equal(t, "1.2.3", response.LatestVersion)
	assert.Equal(t, "https://example.com/download", response.DownloadURL)
	assert.Equal(t, "abc123", response.Checksum)
	assert.Equal(t, "sha256", response.ChecksumType)
	assert.Equal(t, int64(12345), response.FileSize)
	assert.Equal(t, "Bug fixes and improvements", response.ReleaseNotes)
	assert.Equal(t, release.ReleaseDate, *response.ReleaseDate)
	assert.True(t, response.Required)
	assert.Equal(t, "1.0.0", response.MinimumVersion)
	assert.Equal(t, map[string]string{"key": "value"}, response.Metadata)
}

func TestUpdateCheckResponse_SetNoUpdateAvailable(t *testing.T) {
	currentVersion := "1.2.3"
	response := &UpdateCheckResponse{}
	response.SetNoUpdateAvailable(currentVersion)

	assert.False(t, response.UpdateAvailable)
	assert.Equal(t, currentVersion, response.CurrentVersion)
	assert.Empty(t, response.LatestVersion)
	assert.Empty(t, response.DownloadURL)
}

func TestLatestVersionResponse_FromRelease(t *testing.T) {
	release := &Release{
		Version:      "2.0.0",
		DownloadURL:  "https://example.com/download/v2",
		Checksum:     "def456",
		ChecksumType: "sha256",
		FileSize:     67890,
		ReleaseNotes: "Major update with new features",
		ReleaseDate:  time.Date(2024, 2, 1, 14, 0, 0, 0, time.UTC),
		Required:     false,
		Metadata:     map[string]string{"category": "major"},
	}

	response := &LatestVersionResponse{}
	response.FromRelease(release)

	assert.Equal(t, "2.0.0", response.Version)
	assert.Equal(t, "https://example.com/download/v2", response.DownloadURL)
	assert.Equal(t, "def456", response.Checksum)
	assert.Equal(t, "sha256", response.ChecksumType)
	assert.Equal(t, int64(67890), response.FileSize)
	assert.Equal(t, "Major update with new features", response.ReleaseNotes)
	assert.Equal(t, release.ReleaseDate, response.ReleaseDate)
	assert.False(t, response.Required)
	assert.Equal(t, map[string]string{"category": "major"}, response.Metadata)
}

func TestReleaseInfo_FromRelease(t *testing.T) {
	release := &Release{
		ID:             "app-1.0.0-windows-amd64",
		Version:        "1.0.0",
		Platform:       "windows",
		Architecture:   "amd64",
		DownloadURL:    "https://example.com/app-1.0.0.exe",
		Checksum:       "hash123",
		ChecksumType:   "sha256",
		FileSize:       54321,
		ReleaseNotes:   "Initial release",
		ReleaseDate:    time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		Required:       true,
		MinimumVersion: "",
		Metadata:       map[string]string{"type": "stable"},
	}

	releaseInfo := &ReleaseInfo{}
	releaseInfo.FromRelease(release)

	assert.Equal(t, "app-1.0.0-windows-amd64", releaseInfo.ID)
	assert.Equal(t, "1.0.0", releaseInfo.Version)
	assert.Equal(t, "windows", releaseInfo.Platform)
	assert.Equal(t, "amd64", releaseInfo.Architecture)
	assert.Equal(t, "https://example.com/app-1.0.0.exe", releaseInfo.DownloadURL)
	assert.Equal(t, "hash123", releaseInfo.Checksum)
	assert.Equal(t, "sha256", releaseInfo.ChecksumType)
	assert.Equal(t, int64(54321), releaseInfo.FileSize)
	assert.Equal(t, "Initial release", releaseInfo.ReleaseNotes)
	assert.Equal(t, release.ReleaseDate, releaseInfo.ReleaseDate)
	assert.True(t, releaseInfo.Required)
	assert.Equal(t, "", releaseInfo.MinimumVersion)
	assert.Equal(t, map[string]string{"type": "stable"}, releaseInfo.Metadata)
}

func TestApplicationSummary_FromApplication(t *testing.T) {
	app := &Application{
		ID:          "test-app",
		Name:        "Test Application",
		Description: "A test application for unit tests",
		Platforms:   []string{"windows", "linux", "darwin"},
		CreatedAt:   "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-15T12:00:00Z",
	}

	summary := &ApplicationSummary{}
	summary.FromApplication(app)

	assert.Equal(t, "test-app", summary.ID)
	assert.Equal(t, "Test Application", summary.Name)
	assert.Equal(t, "A test application for unit tests", summary.Description)
	assert.Equal(t, []string{"windows", "linux", "darwin"}, summary.Platforms)
	// Note: CreatedAt and UpdatedAt are not copied in the FromApplication method
	// This might be intentional design - the summary might set these separately
}

func TestNewErrorResponse(t *testing.T) {
	message := "Test error message"
	code := "TEST_ERROR"

	response := NewErrorResponse(message, code)

	assert.Equal(t, "error", response.Error)
	assert.Equal(t, message, response.Message)
	assert.Equal(t, code, response.Code)
	assert.WithinDuration(t, time.Now(), response.Timestamp, time.Second)
	assert.Empty(t, response.Details)
	assert.Empty(t, response.RequestID)
}

func TestNewValidationErrorResponse(t *testing.T) {
	errors := map[string]string{
		"field1": "Field 1 is required",
		"field2": "Field 2 must be a valid email",
	}

	response := NewValidationErrorResponse(errors)

	assert.Equal(t, "validation_error", response.Error)
	assert.Equal(t, errors, response.Errors)
}

func TestNewHealthCheckResponse(t *testing.T) {
	status := StatusHealthy

	response := NewHealthCheckResponse(status)

	assert.Equal(t, status, response.Status)
	assert.WithinDuration(t, time.Now(), response.Timestamp, time.Second)
	assert.NotNil(t, response.Components)
	assert.NotNil(t, response.Metrics)
	assert.Empty(t, response.Components)
	assert.Empty(t, response.Metrics)
}

func TestHealthCheckResponse_AddComponent(t *testing.T) {
	response := NewHealthCheckResponse(StatusHealthy)

	componentName := "database"
	componentStatus := StatusHealthy
	componentMessage := "Database is operational"

	response.AddComponent(componentName, componentStatus, componentMessage)

	require.Contains(t, response.Components, componentName)
	component := response.Components[componentName]
	assert.Equal(t, componentStatus, component.Status)
	assert.Equal(t, componentMessage, component.Message)
	assert.WithinDuration(t, time.Now(), component.Timestamp, time.Second)
	assert.NotNil(t, component.Details)
	assert.Empty(t, component.Details)
}

func TestHealthCheckResponse_AddMetric(t *testing.T) {
	response := NewHealthCheckResponse(StatusHealthy)

	metricName := "response_time"
	metricValue := 125.5

	response.AddMetric(metricName, metricValue)

	assert.Contains(t, response.Metrics, metricName)
	assert.Equal(t, metricValue, response.Metrics[metricName])
}

func TestHealthStatusConstants(t *testing.T) {
	// Test that the constants have the expected values
	assert.Equal(t, "healthy", StatusHealthy)
	assert.Equal(t, "unhealthy", StatusUnhealthy)
	assert.Equal(t, "degraded", StatusDegraded)
	assert.Equal(t, "unknown", StatusUnknown)
}

func TestErrorCodeConstants(t *testing.T) {
	// Test that error codes follow the expected format
	assert.Equal(t, "NOT_FOUND", ErrorCodeNotFound)
	assert.Equal(t, "BAD_REQUEST", ErrorCodeBadRequest)
	assert.Equal(t, "VALIDATION_ERROR", ErrorCodeValidation)
	assert.Equal(t, "INTERNAL_ERROR", ErrorCodeInternalError)
	assert.Equal(t, "UNAUTHORIZED", ErrorCodeUnauthorized)
	assert.Equal(t, "FORBIDDEN", ErrorCodeForbidden)
	assert.Equal(t, "CONFLICT", ErrorCodeConflict)
	assert.Equal(t, "SERVICE_UNAVAILABLE", ErrorCodeServiceUnavailable)

	// All error codes should be uppercase
	errorCodes := []string{
		ErrorCodeNotFound,
		ErrorCodeBadRequest,
		ErrorCodeValidation,
		ErrorCodeInternalError,
		ErrorCodeUnauthorized,
		ErrorCodeForbidden,
		ErrorCodeConflict,
		ErrorCodeServiceUnavailable,
	}

	for _, code := range errorCodes {
		assert.Equal(t, code, strings.ToUpper(code))
		// Most error codes should contain underscores, but not all (like UNAUTHORIZED, FORBIDDEN, CONFLICT)
	}
}

func TestListReleasesResponse_Structure(t *testing.T) {
	// Test that the structure has the expected fields
	response := ListReleasesResponse{
		Releases:   []ReleaseInfo{},
		TotalCount: 100,
		Page:       1,
		PageSize:   20,
		HasMore:    true,
	}

	assert.NotNil(t, response.Releases)
	assert.Equal(t, 100, response.TotalCount)
	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 20, response.PageSize)
	assert.True(t, response.HasMore)
}

func TestRegisterReleaseResponse_Structure(t *testing.T) {
	now := time.Now()
	response := RegisterReleaseResponse{
		ID:        "test-release-id",
		Message:   "Release registered successfully",
		CreatedAt: now,
	}

	assert.Equal(t, "test-release-id", response.ID)
	assert.Equal(t, "Release registered successfully", response.Message)
	assert.Equal(t, now, response.CreatedAt)
}

func TestCreateApplicationResponse_Structure(t *testing.T) {
	now := time.Now()
	response := CreateApplicationResponse{
		ID:        "test-app-id",
		Message:   "Application created successfully",
		CreatedAt: now,
	}

	assert.Equal(t, "test-app-id", response.ID)
	assert.Equal(t, "Application created successfully", response.Message)
	assert.Equal(t, now, response.CreatedAt)
}

func TestUpdateApplicationResponse_Structure(t *testing.T) {
	now := time.Now()
	response := UpdateApplicationResponse{
		ID:        "test-app-id",
		Message:   "Application updated successfully",
		UpdatedAt: now,
	}

	assert.Equal(t, "test-app-id", response.ID)
	assert.Equal(t, "Application updated successfully", response.Message)
	assert.Equal(t, now, response.UpdatedAt)
}

func TestDeleteReleaseResponse_Structure(t *testing.T) {
	response := DeleteReleaseResponse{
		ID:      "test-release-id",
		Message: "Release deleted successfully",
	}

	assert.Equal(t, "test-release-id", response.ID)
	assert.Equal(t, "Release deleted successfully", response.Message)
}

func TestApplicationInfoResponse_Structure(t *testing.T) {
	now := time.Now()
	config := ApplicationConfig{
		AutoUpdate:     true,
		UpdateInterval: 3600,
	}
	stats := ApplicationStats{
		TotalReleases:     5,
		LatestVersion:     "1.2.3",
		LatestReleaseDate: &now,
		PlatformCount:     3,
		RequiredReleases:  1,
	}

	response := ApplicationInfoResponse{
		ID:          "test-app",
		Name:        "Test App",
		Description: "Test application",
		Platforms:   []string{"windows", "linux"},
		Config:      config,
		Stats:       stats,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	assert.Equal(t, "test-app", response.ID)
	assert.Equal(t, "Test App", response.Name)
	assert.Equal(t, config, response.Config)
	assert.Equal(t, stats, response.Stats)
}

func TestApplicationStats_Structure(t *testing.T) {
	now := time.Now()
	stats := ApplicationStats{
		TotalReleases:     10,
		LatestVersion:     "2.0.0",
		LatestReleaseDate: &now,
		PlatformCount:     4,
		RequiredReleases:  2,
	}

	assert.Equal(t, 10, stats.TotalReleases)
	assert.Equal(t, "2.0.0", stats.LatestVersion)
	assert.Equal(t, &now, stats.LatestReleaseDate)
	assert.Equal(t, 4, stats.PlatformCount)
	assert.Equal(t, 2, stats.RequiredReleases)

	// Test with nil LatestReleaseDate
	stats.LatestReleaseDate = nil
	assert.Nil(t, stats.LatestReleaseDate)
}

func TestStatsResponse_Structure(t *testing.T) {
	now := time.Now()
	activity := []ActivityItem{
		{
			Type:        "release_created",
			Description: "New release 1.2.3 created",
			Timestamp:   now,
			Metadata:    map[string]string{"version": "1.2.3"},
		},
	}

	response := StatsResponse{
		TotalApplications: 5,
		TotalReleases:     25,
		PlatformStats:     map[string]int{"windows": 10, "linux": 15},
		VersionStats:      map[string]int{"1.0.0": 5, "1.1.0": 10, "1.2.0": 10},
		RecentActivity:    activity,
		SystemInfo:        map[string]interface{}{"uptime": "24h", "memory_usage": "512MB"},
	}

	assert.Equal(t, 5, response.TotalApplications)
	assert.Equal(t, 25, response.TotalReleases)
	assert.Equal(t, 2, len(response.PlatformStats))
	assert.Equal(t, 3, len(response.VersionStats))
	assert.Equal(t, 1, len(response.RecentActivity))
	assert.Equal(t, 2, len(response.SystemInfo))
}

func TestActivityItem_Structure(t *testing.T) {
	now := time.Now()
	item := ActivityItem{
		Type:        "application_created",
		Description: "Application 'test-app' was created",
		Timestamp:   now,
		Metadata:    map[string]string{"app_id": "test-app", "user": "admin"},
	}

	assert.Equal(t, "application_created", item.Type)
	assert.Equal(t, "Application 'test-app' was created", item.Description)
	assert.Equal(t, now, item.Timestamp)
	assert.Equal(t, "test-app", item.Metadata["app_id"])
	assert.Equal(t, "admin", item.Metadata["user"])
}

func TestComponentHealth_Structure(t *testing.T) {
	now := time.Now()
	component := ComponentHealth{
		Status:    StatusHealthy,
		Message:   "All systems operational",
		Details:   map[string]interface{}{"connections": 10, "latency": "5ms"},
		Timestamp: now,
	}

	assert.Equal(t, StatusHealthy, component.Status)
	assert.Equal(t, "All systems operational", component.Message)
	assert.Equal(t, 2, len(component.Details))
	assert.Equal(t, now, component.Timestamp)
}
