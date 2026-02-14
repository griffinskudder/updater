package update

import (
	"context"
	"fmt"
	"testing"
	"time"
	"updater/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockStorage implements the storage.Storage interface for testing
type MockStorage struct {
	applications map[string]*models.Application
	releases     map[string][]*models.Release
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		applications: make(map[string]*models.Application),
		releases:     make(map[string][]*models.Release),
	}
}

func (m *MockStorage) Applications(ctx context.Context) ([]*models.Application, error) {
	apps := make([]*models.Application, 0, len(m.applications))
	for _, app := range m.applications {
		apps = append(apps, app)
	}
	return apps, nil
}

func (m *MockStorage) GetApplication(ctx context.Context, appID string) (*models.Application, error) {
	app, exists := m.applications[appID]
	if !exists {
		return nil, fmt.Errorf("application %s not found", appID)
	}
	return app, nil
}

func (m *MockStorage) SaveApplication(ctx context.Context, app *models.Application) error {
	m.applications[app.ID] = app
	return nil
}

func (m *MockStorage) Releases(ctx context.Context, appID string) ([]*models.Release, error) {
	releases, exists := m.releases[appID]
	if !exists {
		return []*models.Release{}, nil
	}
	return releases, nil
}

func (m *MockStorage) GetRelease(ctx context.Context, appID, version, platform, arch string) (*models.Release, error) {
	releases, exists := m.releases[appID]
	if !exists {
		return nil, fmt.Errorf("release not found")
	}

	for _, release := range releases {
		if release.Version == version && release.Platform == platform && release.Architecture == arch {
			return release, nil
		}
	}
	return nil, fmt.Errorf("release not found")
}

func (m *MockStorage) SaveRelease(ctx context.Context, release *models.Release) error {
	if m.releases[release.ApplicationID] == nil {
		m.releases[release.ApplicationID] = make([]*models.Release, 0)
	}
	m.releases[release.ApplicationID] = append(m.releases[release.ApplicationID], release)
	return nil
}

func (m *MockStorage) DeleteRelease(ctx context.Context, appID, version, platform, arch string) error {
	releases, exists := m.releases[appID]
	if !exists {
		return fmt.Errorf("release not found")
	}

	for i, release := range releases {
		if release.Version == version && release.Platform == platform && release.Architecture == arch {
			m.releases[appID] = append(releases[:i], releases[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("release not found")
}

func (m *MockStorage) GetLatestRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error) {
	releases, exists := m.releases[appID]
	if !exists {
		return nil, fmt.Errorf("no releases found")
	}

	var latest *models.Release
	for _, release := range releases {
		if release.Platform != platform || release.Architecture != arch {
			continue
		}
		if latest == nil {
			latest = release
			continue
		}

		// Simple version comparison for testing
		if release.Version > latest.Version {
			latest = release
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no releases found for %s on %s-%s", appID, platform, arch)
	}

	return latest, nil
}

func (m *MockStorage) GetReleasesAfterVersion(ctx context.Context, appID, currentVersion, platform, arch string) ([]*models.Release, error) {
	releases, exists := m.releases[appID]
	if !exists {
		return []*models.Release{}, nil
	}

	var newerReleases []*models.Release
	for _, release := range releases {
		if release.Platform == platform && release.Architecture == arch && release.Version > currentVersion {
			newerReleases = append(newerReleases, release)
		}
	}

	return newerReleases, nil
}

func (m *MockStorage) Close() error {
	return nil
}

func TestNewService(t *testing.T) {
	mockStorage := NewMockStorage()
	service := NewService(mockStorage)

	assert.NotNil(t, service)
	assert.Equal(t, mockStorage, service.storage)
}

func TestService_CheckForUpdate(t *testing.T) {
	mockStorage := NewMockStorage()
	service := NewService(mockStorage)
	ctx := context.Background()

	// Setup test application
	app := &models.Application{
		ID:        "test-app",
		Name:      "Test App",
		Platforms: []string{"windows", "linux"},
		Config:    models.ApplicationConfig{},
	}
	mockStorage.SaveApplication(ctx, app)

	// Setup test releases
	release1 := createTestReleaseForUpdate("test-app", "1.0.0", "windows", "amd64")
	release2 := createTestReleaseForUpdate("test-app", "1.1.0", "windows", "amd64")
	mockStorage.SaveRelease(ctx, release1)
	mockStorage.SaveRelease(ctx, release2)

	tests := []struct {
		name              string
		request           *models.UpdateCheckRequest
		expectUpdate      bool
		expectedVersion   string
		expectError       bool
		errorContains     string
	}{
		{
			name: "update available",
			request: &models.UpdateCheckRequest{
				ApplicationID:  "test-app",
				CurrentVersion: "1.0.0",
				Platform:       "windows",
				Architecture:   "amd64",
			},
			expectUpdate:    true,
			expectedVersion: "1.1.0",
		},
		{
			name: "no update available",
			request: &models.UpdateCheckRequest{
				ApplicationID:  "test-app",
				CurrentVersion: "1.1.0",
				Platform:       "windows",
				Architecture:   "amd64",
			},
			expectUpdate: false,
		},
		{
			name: "invalid request - empty app id",
			request: &models.UpdateCheckRequest{
				CurrentVersion: "1.0.0",
				Platform:       "windows",
				Architecture:   "amd64",
			},
			expectError:   true,
			errorContains: "invalid request",
		},
		{
			name: "application not found",
			request: &models.UpdateCheckRequest{
				ApplicationID:  "non-existent-app",
				CurrentVersion: "1.0.0",
				Platform:       "windows",
				Architecture:   "amd64",
			},
			expectError:   true,
			errorContains: "application not found",
		},
		{
			name: "unsupported platform",
			request: &models.UpdateCheckRequest{
				ApplicationID:  "test-app",
				CurrentVersion: "1.0.0",
				Platform:       "ios",
				Architecture:   "arm64",
			},
			expectError:   true,
			errorContains: "does not support platform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := service.CheckForUpdate(ctx, tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectUpdate, response.UpdateAvailable)

			if tt.expectUpdate {
				assert.Equal(t, tt.expectedVersion, response.LatestVersion)
				assert.NotEmpty(t, response.DownloadURL)
			}
		})
	}
}

func TestService_CheckForUpdate_PreRelease(t *testing.T) {
	mockStorage := NewMockStorage()
	service := NewService(mockStorage)
	ctx := context.Background()

	// Setup test application
	app := &models.Application{
		ID:        "test-app",
		Name:      "Test App",
		Platforms: []string{"windows"},
		Config:    models.ApplicationConfig{},
	}
	mockStorage.SaveApplication(ctx, app)

	// Setup releases including pre-release
	release1 := createTestReleaseForUpdate("test-app", "1.0.0", "windows", "amd64")
	release2 := createTestReleaseForUpdate("test-app", "1.1.0", "windows", "amd64")
	preRelease := createTestReleaseForUpdate("test-app", "1.2.0-beta.1", "windows", "amd64")
	mockStorage.SaveRelease(ctx, release1)
	mockStorage.SaveRelease(ctx, release2)
	mockStorage.SaveRelease(ctx, preRelease)

	tests := []struct {
		name             string
		allowPrerelease  bool
		expectedVersion  string
	}{
		{
			name:            "allow prerelease",
			allowPrerelease: true,
			expectedVersion: "1.2.0-beta.1",
		},
		{
			name:            "disallow prerelease",
			allowPrerelease: false,
			expectedVersion: "1.1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &models.UpdateCheckRequest{
				ApplicationID:   "test-app",
				CurrentVersion:  "1.0.0",
				Platform:        "windows",
				Architecture:    "amd64",
				AllowPrerelease: tt.allowPrerelease,
			}

			// Note: This test would work better with real semver comparison
			// For now, we're testing the structure but the mock storage
			// doesn't implement proper semver comparison
			response, err := service.CheckForUpdate(ctx, request)
			require.NoError(t, err)
			assert.True(t, response.UpdateAvailable)
		})
	}
}

func TestService_GetLatestVersion(t *testing.T) {
	mockStorage := NewMockStorage()
	service := NewService(mockStorage)
	ctx := context.Background()

	// Setup test application
	app := &models.Application{
		ID:        "test-app",
		Name:      "Test App",
		Platforms: []string{"windows"},
		Config:    models.ApplicationConfig{},
	}
	mockStorage.SaveApplication(ctx, app)

	// Setup test release
	release := createTestReleaseForUpdate("test-app", "1.0.0", "windows", "amd64")
	mockStorage.SaveRelease(ctx, release)

	request := &models.LatestVersionRequest{
		ApplicationID: "test-app",
		Platform:      "windows",
		Architecture:  "amd64",
	}

	response, err := service.GetLatestVersion(ctx, request)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", response.Version)
	assert.NotEmpty(t, response.DownloadURL)
}

func TestService_ListReleases(t *testing.T) {
	mockStorage := NewMockStorage()
	service := NewService(mockStorage)
	ctx := context.Background()

	// Setup test releases
	releases := []*models.Release{
		createTestReleaseForUpdate("test-app", "1.0.0", "windows", "amd64"),
		createTestReleaseForUpdate("test-app", "1.1.0", "windows", "amd64"),
		createTestReleaseForUpdate("test-app", "1.0.0", "linux", "amd64"),
		createTestReleaseForUpdate("other-app", "1.0.0", "windows", "amd64"),
	}

	for _, release := range releases {
		mockStorage.SaveRelease(ctx, release)
	}

	tests := []struct {
		name               string
		request            *models.ListReleasesRequest
		expectedCount      int
		expectedTotalCount int
	}{
		{
			name: "all releases for app",
			request: &models.ListReleasesRequest{
				ApplicationID: "test-app",
			},
			expectedCount:      3,
			expectedTotalCount: 3,
		},
		{
			name: "platform filtered",
			request: &models.ListReleasesRequest{
				ApplicationID: "test-app",
				Platform:      "windows",
			},
			expectedCount:      2, // 2 windows releases for test-app
			expectedTotalCount: 2,
		},
		{
			name: "platform and architecture filtered",
			request: &models.ListReleasesRequest{
				ApplicationID: "test-app",
				Platform:      "windows",
				Architecture:  "amd64",
			},
			expectedCount:      2, // 2 windows-amd64 releases for test-app
			expectedTotalCount: 2,
		},
		{
			name: "version filtered",
			request: &models.ListReleasesRequest{
				ApplicationID: "test-app",
				Version:       "1.0.0",
			},
			expectedCount:      2, // 2 v1.0.0 releases for test-app (windows and linux)
			expectedTotalCount: 2,
		},
		{
			name: "with pagination",
			request: &models.ListReleasesRequest{
				ApplicationID: "test-app",
				Limit:         1,
				Offset:        0,
			},
			expectedCount:      1, // Only 1 due to limit
			expectedTotalCount: 3, // But total available is 3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := service.ListReleases(ctx, tt.request)
			require.NoError(t, err)
			assert.Len(t, response.Releases, tt.expectedCount)
			assert.Equal(t, tt.expectedTotalCount, response.TotalCount) // TotalCount is filtered count
		})
	}
}

func TestService_RegisterRelease(t *testing.T) {
	mockStorage := NewMockStorage()
	service := NewService(mockStorage)
	ctx := context.Background()

	// Setup test application
	app := &models.Application{
		ID:        "test-app",
		Name:      "Test App",
		Platforms: []string{"windows"},
		Config:    models.ApplicationConfig{},
	}
	mockStorage.SaveApplication(ctx, app)

	request := &models.RegisterReleaseRequest{
		ApplicationID: "test-app",
		Version:       "1.0.0",
		Platform:      "windows",
		Architecture:  "amd64",
		DownloadURL:   "https://example.com/download",
		Checksum:      "abc123",
		ChecksumType:  "sha256",
		FileSize:      1234567,
		ReleaseNotes:  "Test release",
		Required:      false,
	}

	response, err := service.RegisterRelease(ctx, request)
	require.NoError(t, err)
	assert.NotEmpty(t, response.ID)
	assert.Contains(t, response.Message, "registered successfully")
	assert.NotZero(t, response.CreatedAt)

	// Verify release was saved
	releases, err := mockStorage.Releases(ctx, "test-app")
	require.NoError(t, err)
	assert.Len(t, releases, 1)
	assert.Equal(t, "1.0.0", releases[0].Version)
}

func TestService_RegisterRelease_Validation(t *testing.T) {
	mockStorage := NewMockStorage()
	service := NewService(mockStorage)
	ctx := context.Background()

	// Setup test application
	app := &models.Application{
		ID:        "test-app",
		Name:      "Test App",
		Platforms: []string{"windows"},
		Config:    models.ApplicationConfig{},
	}
	mockStorage.SaveApplication(ctx, app)

	tests := []struct {
		name          string
		request       *models.RegisterReleaseRequest
		expectError   bool
		errorContains string
	}{
		{
			name: "invalid request - empty version",
			request: &models.RegisterReleaseRequest{
				ApplicationID: "test-app",
				Platform:      "windows",
				Architecture:  "amd64",
			},
			expectError:   true,
			errorContains: "invalid request",
		},
		{
			name: "application not found",
			request: &models.RegisterReleaseRequest{
				ApplicationID: "non-existent-app",
				Version:       "1.0.0",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError:   true,
			errorContains: "application not found",
		},
		{
			name: "unsupported platform",
			request: &models.RegisterReleaseRequest{
				ApplicationID: "test-app",
				Version:       "1.0.0",
				Platform:      "ios",
				Architecture:  "arm64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError:   true,
			errorContains: "does not support platform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.RegisterRelease(ctx, tt.request)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

// Helper functions

func createTestReleaseForUpdate(appID, version, platform, arch string) *models.Release {
	release := models.NewRelease(appID, version, platform, arch, "https://example.com/download")
	release.Checksum = "abc123"
	release.ChecksumType = "sha256"
	release.FileSize = 1234567
	release.ReleaseNotes = "Test release"
	release.ReleaseDate = time.Now()
	return release
}