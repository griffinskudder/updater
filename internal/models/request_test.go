package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateCheckRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     UpdateCheckRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid request",
			request: UpdateCheckRequest{
				ApplicationID:  "test-app",
				CurrentVersion: "1.2.3",
				Platform:       "windows",
				Architecture:   "amd64",
			},
			expectError: false,
		},
		{
			name: "empty application ID",
			request: UpdateCheckRequest{
				CurrentVersion: "1.2.3",
				Platform:       "windows",
				Architecture:   "amd64",
			},
			expectError: true,
			errorMsg:    "application_id is required",
		},
		{
			name: "empty current version",
			request: UpdateCheckRequest{
				ApplicationID: "test-app",
				Platform:      "windows",
				Architecture:  "amd64",
			},
			expectError: true,
			errorMsg:    "version is required",
		},
		{
			name: "invalid current version format",
			request: UpdateCheckRequest{
				ApplicationID:  "test-app",
				CurrentVersion: "invalid-version",
				Platform:       "windows",
				Architecture:   "amd64",
			},
			expectError: true,
			errorMsg:    "invalid version format",
		},
		{
			name: "empty platform",
			request: UpdateCheckRequest{
				ApplicationID:  "test-app",
				CurrentVersion: "1.2.3",
				Architecture:   "amd64",
			},
			expectError: true,
			errorMsg:    "platform is required",
		},
		{
			name: "invalid platform",
			request: UpdateCheckRequest{
				ApplicationID:  "test-app",
				CurrentVersion: "1.2.3",
				Platform:       "invalid-platform",
				Architecture:   "amd64",
			},
			expectError: true,
			errorMsg:    "invalid platform: invalid-platform",
		},
		{
			name: "empty architecture",
			request: UpdateCheckRequest{
				ApplicationID:  "test-app",
				CurrentVersion: "1.2.3",
				Platform:       "windows",
			},
			expectError: true,
			errorMsg:    "architecture is required",
		},
		{
			name: "invalid architecture",
			request: UpdateCheckRequest{
				ApplicationID:  "test-app",
				CurrentVersion: "1.2.3",
				Platform:       "windows",
				Architecture:   "invalid-arch",
			},
			expectError: true,
			errorMsg:    "invalid architecture: invalid-arch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateCheckRequest_Normalize(t *testing.T) {
	request := UpdateCheckRequest{
		ApplicationID:  "  test-app  ",
		CurrentVersion: "  1.2.3  ",
		Platform:       "Windows",
		Architecture:   "AMD64",
	}

	request.Normalize()

	assert.Equal(t, "test-app", request.ApplicationID)
	assert.Equal(t, "1.2.3", request.CurrentVersion)
	assert.Equal(t, "windows", request.Platform)
	assert.Equal(t, "amd64", request.Architecture)
}

func TestLatestVersionRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     LatestVersionRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid request",
			request: LatestVersionRequest{
				ApplicationID: "test-app",
				Platform:      "linux",
				Architecture:  "arm64",
			},
			expectError: false,
		},
		{
			name: "empty application ID",
			request: LatestVersionRequest{
				Platform:     "linux",
				Architecture: "arm64",
			},
			expectError: true,
			errorMsg:    "application_id is required",
		},
		{
			name: "invalid platform",
			request: LatestVersionRequest{
				ApplicationID: "test-app",
				Platform:      "invalid",
				Architecture:  "arm64",
			},
			expectError: true,
			errorMsg:    "invalid platform: invalid",
		},
		{
			name: "invalid architecture",
			request: LatestVersionRequest{
				ApplicationID: "test-app",
				Platform:      "linux",
				Architecture:  "invalid",
			},
			expectError: true,
			errorMsg:    "invalid architecture: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLatestVersionRequest_Normalize(t *testing.T) {
	request := LatestVersionRequest{
		ApplicationID: "  test-app  ",
		Platform:      "LINUX",
		Architecture:  "ARM64",
	}

	request.Normalize()

	assert.Equal(t, "test-app", request.ApplicationID)
	assert.Equal(t, "linux", request.Platform)
	assert.Equal(t, "arm64", request.Architecture)
}

func TestListReleasesRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     ListReleasesRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid request",
			request: ListReleasesRequest{
				ApplicationID: "test-app",
				Platform:      "windows",
				Architecture:  "amd64",
				Limit:         10,
				Offset:        0,
				SortBy:        "version",
				SortOrder:     "desc",
			},
			expectError: false,
		},
		{
			name: "minimal valid request",
			request: ListReleasesRequest{
				ApplicationID: "test-app",
			},
			expectError: false,
		},
		{
			name: "empty application ID",
			request: ListReleasesRequest{
				Platform: "windows",
			},
			expectError: true,
			errorMsg:    "application_id is required",
		},
		{
			name: "invalid platform",
			request: ListReleasesRequest{
				ApplicationID: "test-app",
				Platform:      "invalid",
			},
			expectError: true,
			errorMsg:    "invalid platform: invalid",
		},
		{
			name: "invalid architecture",
			request: ListReleasesRequest{
				ApplicationID: "test-app",
				Architecture:  "invalid",
			},
			expectError: true,
			errorMsg:    "invalid architecture: invalid",
		},
		{
			name: "invalid version format",
			request: ListReleasesRequest{
				ApplicationID: "test-app",
				Version:       "invalid-version",
			},
			expectError: true,
			errorMsg:    "invalid version format",
		},
		{
			name: "negative limit",
			request: ListReleasesRequest{
				ApplicationID: "test-app",
				Limit:         -1,
			},
			expectError: true,
			errorMsg:    "limit cannot be negative",
		},
		{
			name: "negative offset",
			request: ListReleasesRequest{
				ApplicationID: "test-app",
				Offset:        -1,
			},
			expectError: true,
			errorMsg:    "offset cannot be negative",
		},
		{
			name: "invalid sort order",
			request: ListReleasesRequest{
				ApplicationID: "test-app",
				SortOrder:     "invalid",
			},
			expectError: true,
			errorMsg:    "sort_order must be 'asc' or 'desc'",
		},
		{
			name: "invalid sort field",
			request: ListReleasesRequest{
				ApplicationID: "test-app",
				SortBy:        "invalid_field",
			},
			expectError: true,
			errorMsg:    "invalid sort_by field: invalid_field",
		},
		{
			name: "invalid platform in platforms list",
			request: ListReleasesRequest{
				ApplicationID: "test-app",
				Platforms:     []string{"windows", "invalid"},
			},
			expectError: true,
			errorMsg:    "invalid platform in platforms list: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListReleasesRequest_Normalize(t *testing.T) {
	request := ListReleasesRequest{
		ApplicationID: "  test-app  ",
		Platform:      "Windows",
		Architecture:  "AMD64",
		Platforms:     []string{"LINUX", "Darwin"},
		Limit:         0,  // Should be set to default
		SortBy:        "", // Should be set to default
		SortOrder:     "", // Should be set to default
	}

	request.Normalize()

	assert.Equal(t, "test-app", request.ApplicationID)
	assert.Equal(t, "windows", request.Platform)
	assert.Equal(t, "amd64", request.Architecture)
	assert.Equal(t, []string{"linux", "darwin"}, request.Platforms)
	assert.Equal(t, 50, request.Limit)              // Default limit
	assert.Equal(t, "release_date", request.SortBy) // Default sort field
	assert.Equal(t, "desc", request.SortOrder)      // Default sort order
}

func TestRegisterReleaseRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     RegisterReleaseRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid request",
			request: RegisterReleaseRequest{
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
				FileSize:      12345,
			},
			expectError: false,
		},
		{
			name: "empty application ID",
			request: RegisterReleaseRequest{
				Version:      "1.2.3",
				Platform:     "windows",
				Architecture: "amd64",
				DownloadURL:  "https://example.com/download",
				Checksum:     "abc123",
				ChecksumType: "sha256",
			},
			expectError: true,
			errorMsg:    "application_id is required",
		},
		{
			name: "empty version",
			request: RegisterReleaseRequest{
				ApplicationID: "test-app",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "version is required",
		},
		{
			name: "invalid version format",
			request: RegisterReleaseRequest{
				ApplicationID: "test-app",
				Version:       "invalid-version",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "invalid version format",
		},
		{
			name: "invalid platform",
			request: RegisterReleaseRequest{
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "invalid",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "invalid platform: invalid",
		},
		{
			name: "invalid architecture",
			request: RegisterReleaseRequest{
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "invalid",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "invalid architecture: invalid",
		},
		{
			name: "empty download URL",
			request: RegisterReleaseRequest{
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "amd64",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "download_url is required",
		},
		{
			name: "empty checksum",
			request: RegisterReleaseRequest{
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "checksum is required",
		},
		{
			name: "empty checksum type",
			request: RegisterReleaseRequest{
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
			},
			expectError: true,
			errorMsg:    "checksum_type is required",
		},
		{
			name: "invalid checksum type",
			request: RegisterReleaseRequest{
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "invalid",
			},
			expectError: true,
			errorMsg:    "invalid checksum_type: invalid",
		},
		{
			name: "negative file size",
			request: RegisterReleaseRequest{
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
				FileSize:      -1,
			},
			expectError: true,
			errorMsg:    "file_size cannot be negative",
		},
		{
			name: "invalid minimum version",
			request: RegisterReleaseRequest{
				ApplicationID:  "test-app",
				Version:        "1.2.3",
				Platform:       "windows",
				Architecture:   "amd64",
				DownloadURL:    "https://example.com/download",
				Checksum:       "abc123",
				ChecksumType:   "sha256",
				MinimumVersion: "invalid",
			},
			expectError: true,
			errorMsg:    "invalid minimum_version format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegisterReleaseRequest_Normalize(t *testing.T) {
	request := RegisterReleaseRequest{
		ApplicationID: "  test-app  ",
		Version:       "  1.2.3  ",
		Platform:      "Windows",
		Architecture:  "AMD64",
		DownloadURL:   "  https://example.com/download  ",
		Checksum:      "  ABC123  ",
		ChecksumType:  "SHA256",
	}

	request.Normalize()

	assert.Equal(t, "test-app", request.ApplicationID)
	assert.Equal(t, "1.2.3", request.Version)
	assert.Equal(t, "windows", request.Platform)
	assert.Equal(t, "amd64", request.Architecture)
	assert.Equal(t, "https://example.com/download", request.DownloadURL)
	assert.Equal(t, "abc123", request.Checksum)
	assert.Equal(t, "sha256", request.ChecksumType)
}

func TestCreateApplicationRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     CreateApplicationRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid request",
			request: CreateApplicationRequest{
				ID:          "test-app",
				Name:        "Test App",
				Description: "A test application",
				Platforms:   []string{"windows", "linux"},
				Config:      ApplicationConfig{UpdateInterval: 3600},
			},
			expectError: false,
		},
		{
			name: "empty ID",
			request: CreateApplicationRequest{
				Name:      "Test App",
				Platforms: []string{"windows"},
				Config:    ApplicationConfig{},
			},
			expectError: true,
			errorMsg:    "id is required",
		},
		{
			name: "invalid ID",
			request: CreateApplicationRequest{
				ID:        "invalid@id",
				Name:      "Test App",
				Platforms: []string{"windows"},
				Config:    ApplicationConfig{},
			},
			expectError: true,
			errorMsg:    "id must contain only alphanumeric characters, hyphens, and underscores",
		},
		{
			name: "empty name",
			request: CreateApplicationRequest{
				ID:        "test-app",
				Platforms: []string{"windows"},
				Config:    ApplicationConfig{},
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "no platforms",
			request: CreateApplicationRequest{
				ID:        "test-app",
				Name:      "Test App",
				Platforms: []string{},
				Config:    ApplicationConfig{},
			},
			expectError: true,
			errorMsg:    "at least one platform must be specified",
		},
		{
			name: "invalid platform",
			request: CreateApplicationRequest{
				ID:        "test-app",
				Name:      "Test App",
				Platforms: []string{"invalid"},
				Config:    ApplicationConfig{},
			},
			expectError: true,
			errorMsg:    "invalid platform: invalid",
		},
		{
			name: "invalid config",
			request: CreateApplicationRequest{
				ID:        "test-app",
				Name:      "Test App",
				Platforms: []string{"windows"},
				Config:    ApplicationConfig{UpdateInterval: -1},
			},
			expectError: true,
			errorMsg:    "invalid config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateApplicationRequest_Normalize(t *testing.T) {
	request := CreateApplicationRequest{
		ID:          "  test-app  ",
		Name:        "  Test App  ",
		Description: "  A test application  ",
		Platforms:   []string{"Windows", "LINUX"},
	}

	request.Normalize()

	assert.Equal(t, "test-app", request.ID)
	assert.Equal(t, "Test App", request.Name)
	assert.Equal(t, "A test application", request.Description)
	assert.Equal(t, []string{"windows", "linux"}, request.Platforms)
}

func TestUpdateApplicationRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     UpdateApplicationRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid request",
			request: UpdateApplicationRequest{
				Platforms: []string{"windows", "linux"},
				Config:    &ApplicationConfig{UpdateInterval: 3600},
			},
			expectError: false,
		},
		{
			name: "empty platforms array",
			request: UpdateApplicationRequest{
				Platforms: []string{},
			},
			expectError: true,
			errorMsg:    "at least one platform must be specified",
		},
		{
			name: "invalid platform",
			request: UpdateApplicationRequest{
				Platforms: []string{"invalid"},
			},
			expectError: true,
			errorMsg:    "invalid platform: invalid",
		},
		{
			name: "invalid config",
			request: UpdateApplicationRequest{
				Config: &ApplicationConfig{UpdateInterval: -1},
			},
			expectError: true,
			errorMsg:    "invalid config",
		},
		{
			name: "nil platforms and config is valid",
			request: UpdateApplicationRequest{
				Platforms: nil,
				Config:    nil,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateApplicationRequest_Normalize(t *testing.T) {
	name := "  Test App  "
	description := "  Updated description  "

	request := UpdateApplicationRequest{
		Name:        &name,
		Description: &description,
		Platforms:   []string{"Windows", "LINUX"},
	}

	request.Normalize()

	require.NotNil(t, request.Name)
	require.NotNil(t, request.Description)
	assert.Equal(t, "Test App", *request.Name)
	assert.Equal(t, "Updated description", *request.Description)
	assert.Equal(t, []string{"windows", "linux"}, request.Platforms)

	// Test with nil pointers
	request2 := UpdateApplicationRequest{
		Name:        nil,
		Description: nil,
		Platforms:   nil,
	}

	request2.Normalize()

	assert.Nil(t, request2.Name)
	assert.Nil(t, request2.Description)
	assert.Nil(t, request2.Platforms)
}
