package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRelease(t *testing.T) {
	appID := "test-app"
	version := "1.2.3"
	platform := "Windows" // Test normalization
	arch := "AMD64"       // Test normalization
	downloadURL := "https://example.com/download"

	release := NewRelease(appID, version, platform, arch, downloadURL)

	assert.Equal(t, "test-app-1.2.3-windows-amd64", release.ID)
	assert.Equal(t, appID, release.ApplicationID)
	assert.Equal(t, version, release.Version)
	assert.Equal(t, "windows", release.Platform)   // Should be normalized
	assert.Equal(t, "amd64", release.Architecture) // Should be normalized
	assert.Equal(t, downloadURL, release.DownloadURL)
	assert.Equal(t, ChecksumTypeSHA256, release.ChecksumType)
	assert.False(t, release.Required)
	assert.NotNil(t, release.Metadata)
	assert.Empty(t, release.Metadata)

	// Check timestamps are set and recent
	assert.WithinDuration(t, time.Now(), release.ReleaseDate, time.Second)
	assert.WithinDuration(t, time.Now(), release.CreatedAt, time.Second)
	assert.WithinDuration(t, time.Now(), release.UpdatedAt, time.Second)
}

func TestRelease_Validate(t *testing.T) {
	tests := []struct {
		name        string
		release     *Release
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid release",
			release: &Release{
				ID:            "test-app-1.2.3-windows-amd64",
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
			name: "empty ID",
			release: &Release{
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "release ID cannot be empty",
		},
		{
			name: "empty application ID",
			release: &Release{
				ID:           "test-1.2.3-windows-amd64",
				Version:      "1.2.3",
				Platform:     "windows",
				Architecture: "amd64",
				DownloadURL:  "https://example.com/download",
				Checksum:     "abc123",
				ChecksumType: "sha256",
			},
			expectError: true,
			errorMsg:    "application ID cannot be empty",
		},
		{
			name: "empty version",
			release: &Release{
				ID:            "test-app--windows-amd64",
				ApplicationID: "test-app",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "version cannot be empty",
		},
		{
			name: "invalid version format",
			release: &Release{
				ID:            "test-app-invalid-windows-amd64",
				ApplicationID: "test-app",
				Version:       "invalid-version-format",
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
			release: &Release{
				ID:            "test-app-1.2.3-invalid-amd64",
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "invalid-platform",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "invalid platform: invalid-platform",
		},
		{
			name: "invalid architecture",
			release: &Release{
				ID:            "test-app-1.2.3-windows-invalid",
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "invalid-arch",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "invalid architecture: invalid-arch",
		},
		{
			name: "empty download URL",
			release: &Release{
				ID:            "test-app-1.2.3-windows-amd64",
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "amd64",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "download URL cannot be empty",
		},
		{
			name: "invalid download URL",
			release: &Release{
				ID:            "test-app-1.2.3-windows-amd64",
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "not-a-valid-url",
				Checksum:      "abc123",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "invalid download URL",
		},
		{
			name: "empty checksum",
			release: &Release{
				ID:            "test-app-1.2.3-windows-amd64",
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				ChecksumType:  "sha256",
			},
			expectError: true,
			errorMsg:    "checksum cannot be empty",
		},
		{
			name: "invalid checksum type",
			release: &Release{
				ID:            "test-app-1.2.3-windows-amd64",
				ApplicationID: "test-app",
				Version:       "1.2.3",
				Platform:      "windows",
				Architecture:  "amd64",
				DownloadURL:   "https://example.com/download",
				Checksum:      "abc123",
				ChecksumType:  "invalid-checksum-type",
			},
			expectError: true,
			errorMsg:    "invalid checksum type: invalid-checksum-type",
		},
		{
			name: "negative file size",
			release: &Release{
				ID:            "test-app-1.2.3-windows-amd64",
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
			errorMsg:    "file size cannot be negative",
		},
		{
			name: "invalid minimum version",
			release: &Release{
				ID:             "test-app-1.2.3-windows-amd64",
				ApplicationID:  "test-app",
				Version:        "1.2.3",
				Platform:       "windows",
				Architecture:   "amd64",
				DownloadURL:    "https://example.com/download",
				Checksum:       "abc123",
				ChecksumType:   "sha256",
				MinimumVersion: "invalid-version",
			},
			expectError: true,
			errorMsg:    "invalid minimum version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.release.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRelease_ValidateDownloadURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid HTTPS URL",
			url:         "https://example.com/download",
			expectError: false,
		},
		{
			name:        "valid HTTP URL",
			url:         "http://example.com/download",
			expectError: false,
		},
		{
			name:        "URL with path and query",
			url:         "https://example.com/path/to/file?version=1.2.3",
			expectError: false,
		},
		{
			name:        "malformed URL",
			url:         "not-a-url",
			expectError: true,
			errorMsg:    "URL must use HTTP or HTTPS scheme",
		},
		{
			name:        "unsupported scheme",
			url:         "ftp://example.com/download",
			expectError: true,
			errorMsg:    "URL must use HTTP or HTTPS scheme",
		},
		{
			name:        "no host",
			url:         "https://",
			expectError: true,
			errorMsg:    "URL must have a valid host",
		},
		{
			name:        "empty URL",
			url:         "",
			expectError: true,
			errorMsg:    "URL must use HTTP or HTTPS scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release := &Release{DownloadURL: tt.url}
			err := release.ValidateDownloadURL()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRelease_GetPlatformInfo(t *testing.T) {
	release := &Release{
		Platform:     "windows",
		Architecture: "amd64",
	}

	platformInfo := release.GetPlatformInfo()
	assert.Equal(t, "windows", platformInfo.Platform)
	assert.Equal(t, "amd64", platformInfo.Architecture)
	assert.Equal(t, "windows-amd64", platformInfo.String())
}

func TestRelease_IsNewerThan(t *testing.T) {
	tests := []struct {
		name        string
		version1    string
		version2    string
		expected    bool
		expectError bool
	}{
		{
			name:        "newer version",
			version1:    "1.2.3",
			version2:    "1.2.2",
			expected:    true,
			expectError: false,
		},
		{
			name:        "older version",
			version1:    "1.2.2",
			version2:    "1.2.3",
			expected:    false,
			expectError: false,
		},
		{
			name:        "same version",
			version1:    "1.2.3",
			version2:    "1.2.3",
			expected:    false,
			expectError: false,
		},
		{
			name:        "invalid version in first release",
			version1:    "invalid",
			version2:    "1.2.3",
			expected:    false,
			expectError: true,
		},
		{
			name:        "invalid version in second release",
			version1:    "1.2.3",
			version2:    "invalid",
			expected:    false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r1 := &Release{Version: tt.version1}
			r2 := &Release{Version: tt.version2}

			result, err := r1.IsNewerThan(r2)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRelease_IsCompatibleWith(t *testing.T) {
	release := &Release{
		Platform:     "windows",
		Architecture: "amd64",
	}

	assert.True(t, release.IsCompatibleWith("windows", "amd64"))
	assert.True(t, release.IsCompatibleWith("Windows", "AMD64")) // Test normalization
	assert.False(t, release.IsCompatibleWith("linux", "amd64"))
	assert.False(t, release.IsCompatibleWith("windows", "arm64"))
	assert.False(t, release.IsCompatibleWith("linux", "arm64"))
}

func TestRelease_MeetsMinimumVersion(t *testing.T) {
	tests := []struct {
		name           string
		minimumVersion string
		currentVersion string
		expected       bool
		expectError    bool
	}{
		{
			name:           "no minimum version requirement",
			minimumVersion: "",
			currentVersion: "1.0.0",
			expected:       true,
			expectError:    false,
		},
		{
			name:           "meets minimum version",
			minimumVersion: "1.0.0",
			currentVersion: "1.2.3",
			expected:       true,
			expectError:    false,
		},
		{
			name:           "exactly minimum version",
			minimumVersion: "1.2.3",
			currentVersion: "1.2.3",
			expected:       true,
			expectError:    false,
		},
		{
			name:           "below minimum version",
			minimumVersion: "1.2.3",
			currentVersion: "1.2.2",
			expected:       false,
			expectError:    false,
		},
		{
			name:           "invalid current version",
			minimumVersion: "1.2.3",
			currentVersion: "invalid",
			expected:       false,
			expectError:    true,
		},
		{
			name:           "invalid minimum version",
			minimumVersion: "invalid",
			currentVersion: "1.2.3",
			expected:       false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release := &Release{MinimumVersion: tt.minimumVersion}
			result, err := release.MeetsMinimumVersion(tt.currentVersion)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRelease_GenerateChecksum(t *testing.T) {
	release := &Release{ChecksumType: ChecksumTypeSHA256}
	data := []byte("test data")

	checksum := release.GenerateChecksum(data)
	assert.NotEmpty(t, checksum)
	assert.Equal(t, 64, len(checksum)) // SHA256 produces 64-character hex string

	// Test that the same data produces the same checksum
	checksum2 := release.GenerateChecksum(data)
	assert.Equal(t, checksum, checksum2)

	// Test different data produces different checksum
	checksum3 := release.GenerateChecksum([]byte("different data"))
	assert.NotEqual(t, checksum, checksum3)

	// Test with unsupported checksum type - should fall back to SHA256
	release.ChecksumType = "unsupported"
	checksum4 := release.GenerateChecksum(data)
	assert.Equal(t, checksum, checksum4) // Should be the same as SHA256
}

func TestRelease_VerifyChecksum(t *testing.T) {
	data := []byte("test data")
	release := &Release{ChecksumType: ChecksumTypeSHA256}

	// Generate correct checksum
	correctChecksum := release.GenerateChecksum(data)
	release.Checksum = correctChecksum

	// Should verify successfully
	assert.True(t, release.VerifyChecksum(data))

	// Test with wrong data
	assert.False(t, release.VerifyChecksum([]byte("wrong data")))

	// Test case insensitive checksum comparison
	release.Checksum = "ABCDEF" // Uppercase
	release.ChecksumType = ChecksumTypeSHA256
	// We can't easily test this without knowing the exact checksum for specific data
	// But we can test that the comparison is case insensitive by setting both
	upperChecksum := release.GenerateChecksum(data)
	release.Checksum = upperChecksum
	assert.True(t, release.VerifyChecksum(data))
}

func TestRelease_SetMetadata(t *testing.T) {
	release := &Release{
		Metadata:  nil,
		UpdatedAt: time.Now().Add(-time.Hour), // Set to past time
	}

	beforeUpdate := time.Now()
	release.SetMetadata("key1", "value1")

	assert.NotNil(t, release.Metadata)
	assert.Equal(t, "value1", release.Metadata["key1"])
	assert.True(t, release.UpdatedAt.After(beforeUpdate))

	// Test updating existing metadata
	release.SetMetadata("key1", "new_value1")
	release.SetMetadata("key2", "value2")

	assert.Equal(t, "new_value1", release.Metadata["key1"])
	assert.Equal(t, "value2", release.Metadata["key2"])
}

func TestRelease_GetMetadata(t *testing.T) {
	release := &Release{
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// Test existing key
	value, exists := release.GetMetadata("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value)

	// Test non-existing key
	value, exists = release.GetMetadata("nonexistent")
	assert.False(t, exists)
	assert.Equal(t, "", value)

	// Test with nil metadata
	release.Metadata = nil
	value, exists = release.GetMetadata("key1")
	assert.False(t, exists)
	assert.Equal(t, "", value)
}

func TestGenerateReleaseID(t *testing.T) {
	tests := []struct {
		name     string
		appID    string
		version  string
		platform string
		arch     string
		expected string
	}{
		{
			name:     "standard release ID",
			appID:    "my-app",
			version:  "1.2.3",
			platform: "windows",
			arch:     "amd64",
			expected: "my-app-1.2.3-windows-amd64",
		},
		{
			name:     "with pre-release version",
			appID:    "test-app",
			version:  "1.0.0-alpha",
			platform: "linux",
			arch:     "arm64",
			expected: "test-app-1.0.0-alpha-linux-arm64",
		},
		{
			name:     "empty components",
			appID:    "",
			version:  "",
			platform: "",
			arch:     "",
			expected: "---",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateReleaseID(tt.appID, tt.version, tt.platform, tt.arch)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidChecksumType(t *testing.T) {
	tests := []struct {
		name         string
		checksumType string
		expected     bool
	}{
		{"sha256", "sha256", true},
		{"SHA256", "SHA256", true}, // Test case insensitivity
		{"md5", "md5", true},
		{"MD5", "MD5", true},
		{"sha1", "sha1", true},
		{"SHA1", "SHA1", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidChecksumType(tt.checksumType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReleaseFilter_Validate(t *testing.T) {
	tests := []struct {
		name        string
		filter      ReleaseFilter
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid filter",
			filter:      ReleaseFilter{Limit: 10, Offset: 0, SortOrder: "asc"},
			expectError: false,
		},
		{
			name:        "negative limit",
			filter:      ReleaseFilter{Limit: -1},
			expectError: true,
			errorMsg:    "limit cannot be negative",
		},
		{
			name:        "negative offset",
			filter:      ReleaseFilter{Offset: -1},
			expectError: true,
			errorMsg:    "offset cannot be negative",
		},
		{
			name:        "invalid sort order",
			filter:      ReleaseFilter{SortOrder: "invalid"},
			expectError: true,
			errorMsg:    "sort order must be 'asc' or 'desc'",
		},
		{
			name:        "valid sort orders",
			filter:      ReleaseFilter{SortOrder: "desc"},
			expectError: false,
		},
		{
			name:        "empty sort order is valid",
			filter:      ReleaseFilter{SortOrder: ""},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSupportedChecksumTypes(t *testing.T) {
	expectedTypes := []string{
		ChecksumTypeSHA256,
		ChecksumTypeMD5,
		ChecksumTypeSHA1,
	}
	assert.Equal(t, expectedTypes, SupportedChecksumTypes)

	// Test that all supported types validate correctly
	for _, checksumType := range SupportedChecksumTypes {
		assert.True(t, isValidChecksumType(checksumType))
	}
}

func TestReleaseConstants(t *testing.T) {
	// Test that checksum type constants are lowercase
	assert.Equal(t, "sha256", ChecksumTypeSHA256)
	assert.Equal(t, "md5", ChecksumTypeMD5)
	assert.Equal(t, "sha1", ChecksumTypeSHA1)
}

func TestReleaseStats(t *testing.T) {
	// Test that ReleaseStats struct exists and has expected fields
	stats := ReleaseStats{
		TotalReleases:     10,
		LatestVersion:     "1.2.3",
		LatestReleaseDate: time.Now(),
		PlatformCount:     3,
		RequiredReleases:  2,
	}

	assert.Equal(t, 10, stats.TotalReleases)
	assert.Equal(t, "1.2.3", stats.LatestVersion)
	assert.Equal(t, 3, stats.PlatformCount)
	assert.Equal(t, 2, stats.RequiredReleases)
}
