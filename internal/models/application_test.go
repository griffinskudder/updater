package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewApplication(t *testing.T) {
	id := "test-app"
	name := "Test Application"
	platforms := []string{"windows", "linux", "darwin"}

	app := NewApplication(id, name, platforms)

	assert.Equal(t, id, app.ID)
	assert.Equal(t, name, app.Name)
	assert.Equal(t, platforms, app.Platforms)

	// Check default configuration
	assert.False(t, app.Config.AutoUpdate)
	assert.Equal(t, 3600, app.Config.UpdateInterval)
	assert.False(t, app.Config.RequiredUpdate)
	assert.False(t, app.Config.AllowPrerelease)
	assert.False(t, app.Config.AnalyticsEnabled)
	assert.NotNil(t, app.Config.CustomFields)
	assert.Empty(t, app.Config.CustomFields)
}

func TestApplication_Validate(t *testing.T) {
	tests := []struct {
		name        string
		app         *Application
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid application",
			app: &Application{
				ID:        "valid-app-id",
				Name:      "Valid App",
				Platforms: []string{"windows", "linux"},
				Config: ApplicationConfig{
					UpdateInterval: 3600,
				},
			},
			expectError: false,
		},
		{
			name: "empty ID",
			app: &Application{
				ID:        "",
				Name:      "Valid App",
				Platforms: []string{"windows"},
				Config:    ApplicationConfig{},
			},
			expectError: true,
			errorMsg:    "application ID cannot be empty",
		},
		{
			name: "invalid ID with special characters",
			app: &Application{
				ID:        "invalid@app#id",
				Name:      "Valid App",
				Platforms: []string{"windows"},
				Config:    ApplicationConfig{},
			},
			expectError: true,
			errorMsg:    "application ID must contain only alphanumeric characters, hyphens, and underscores",
		},
		{
			name: "empty name",
			app: &Application{
				ID:        "valid-app-id",
				Name:      "",
				Platforms: []string{"windows"},
				Config:    ApplicationConfig{},
			},
			expectError: true,
			errorMsg:    "application name cannot be empty",
		},
		{
			name: "no platforms",
			app: &Application{
				ID:        "valid-app-id",
				Name:      "Valid App",
				Platforms: []string{},
				Config:    ApplicationConfig{},
			},
			expectError: true,
			errorMsg:    "at least one platform must be specified",
		},
		{
			name: "invalid platform",
			app: &Application{
				ID:        "valid-app-id",
				Name:      "Valid App",
				Platforms: []string{"invalid-platform"},
				Config:    ApplicationConfig{},
			},
			expectError: true,
			errorMsg:    "invalid platform: invalid-platform",
		},
		{
			name: "invalid config",
			app: &Application{
				ID:        "valid-app-id",
				Name:      "Valid App",
				Platforms: []string{"windows"},
				Config: ApplicationConfig{
					UpdateInterval: -1, // Invalid negative interval
				},
			},
			expectError: true,
			errorMsg:    "invalid config: update interval cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.app.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApplication_SupportsPlatform(t *testing.T) {
	app := &Application{
		Platforms: []string{"windows", "linux", "darwin"},
	}

	assert.True(t, app.SupportsPlatform("windows"))
	assert.True(t, app.SupportsPlatform("linux"))
	assert.True(t, app.SupportsPlatform("darwin"))
	assert.False(t, app.SupportsPlatform("android"))
	assert.False(t, app.SupportsPlatform(""))
}

func TestApplication_SupportsArchitecture(t *testing.T) {
	app := &Application{
		Platforms: []string{"windows", "linux"},
	}

	// Should support architecture if platform is supported and arch is valid
	assert.True(t, app.SupportsArchitecture("windows", "amd64"))
	assert.True(t, app.SupportsArchitecture("linux", "arm64"))

	// Should not support if platform is not supported
	assert.False(t, app.SupportsArchitecture("android", "amd64"))

	// Should not support if architecture is invalid
	assert.False(t, app.SupportsArchitecture("windows", "invalid-arch"))
}

func TestApplicationConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      ApplicationConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: ApplicationConfig{
				UpdateInterval: 3600,
				MinVersion:     "1.0.0",
				MaxVersion:     "2.0.0",
			},
			expectError: false,
		},
		{
			name: "negative update interval",
			config: ApplicationConfig{
				UpdateInterval: -1,
			},
			expectError: true,
			errorMsg:    "update interval cannot be negative",
		},
		{
			name: "invalid min version",
			config: ApplicationConfig{
				UpdateInterval: 3600,
				MinVersion:     "invalid-version",
			},
			expectError: true,
			errorMsg:    "invalid min version",
		},
		{
			name: "invalid max version",
			config: ApplicationConfig{
				UpdateInterval: 3600,
				MaxVersion:     "invalid-version",
			},
			expectError: true,
			errorMsg:    "invalid max version",
		},
		{
			name: "min version greater than max version",
			config: ApplicationConfig{
				UpdateInterval: 3600,
				MinVersion:     "2.0.0",
				MaxVersion:     "1.0.0",
			},
			expectError: true,
			errorMsg:    "min version cannot be greater than max version",
		},
		{
			name: "empty version strings are valid",
			config: ApplicationConfig{
				UpdateInterval: 3600,
				MinVersion:     "",
				MaxVersion:     "",
			},
			expectError: false,
		},
		{
			name: "zero update interval is valid",
			config: ApplicationConfig{
				UpdateInterval: 0,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsValidID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected bool
	}{
		{"valid alphanumeric", "app123", true},
		{"valid with hyphens", "my-app", true},
		{"valid with underscores", "my_app", true},
		{"valid mixed", "my-app_123", true},
		{"empty string", "", false},
		{"with spaces", "my app", false},
		{"with special chars", "my@app", false},
		{"with dots", "my.app", false},
		{"too long", string(make([]byte, 101)), false}, // 101 characters
		{"exactly 100 chars", string(make([]byte, 100)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For the long string tests, fill with valid characters
			if tt.name == "too long" || tt.name == "exactly 100 chars" {
				id := ""
				for i := 0; i < len(tt.id); i++ {
					id += "a"
				}
				tt.id = id
			}

			result := isValidID(tt.id)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidPlatform(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		expected bool
	}{
		{"windows", "windows", true},
		{"linux", "linux", true},
		{"darwin", "darwin", true},
		{"android", "android", true},
		{"ios", "ios", true},
		{"Windows uppercase", "Windows", true}, // Should normalize to lowercase
		{"LINUX uppercase", "LINUX", true},
		{"invalid platform", "invalid", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidPlatform(tt.platform)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidArchitecture(t *testing.T) {
	tests := []struct {
		name     string
		arch     string
		expected bool
	}{
		{"amd64", "amd64", true},
		{"arm64", "arm64", true},
		{"386", "386", true},
		{"arm", "arm", true},
		{"AMD64 uppercase", "AMD64", true}, // Should normalize to lowercase
		{"ARM64 uppercase", "ARM64", true},
		{"invalid arch", "invalid", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidArchitecture(tt.arch)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizePlatform(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		expected string
	}{
		{"lowercase", "windows", "windows"},
		{"uppercase", "WINDOWS", "windows"},
		{"mixed case", "Windows", "windows"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePlatform(tt.platform)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeArchitecture(t *testing.T) {
	tests := []struct {
		name     string
		arch     string
		expected string
	}{
		{"lowercase", "amd64", "amd64"},
		{"uppercase", "AMD64", "amd64"},
		{"mixed case", "Amd64", "amd64"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeArchitecture(tt.arch)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPlatformInfo_Validate(t *testing.T) {
	tests := []struct {
		name         string
		platformInfo PlatformInfo
		expectError  bool
		errorMsg     string
	}{
		{
			name: "valid platform info",
			platformInfo: PlatformInfo{
				Platform:     "windows",
				Architecture: "amd64",
			},
			expectError: false,
		},
		{
			name: "invalid platform",
			platformInfo: PlatformInfo{
				Platform:     "invalid",
				Architecture: "amd64",
			},
			expectError: true,
			errorMsg:    "invalid platform: invalid",
		},
		{
			name: "invalid architecture",
			platformInfo: PlatformInfo{
				Platform:     "windows",
				Architecture: "invalid",
			},
			expectError: true,
			errorMsg:    "invalid architecture: invalid",
		},
		{
			name: "empty platform",
			platformInfo: PlatformInfo{
				Platform:     "",
				Architecture: "amd64",
			},
			expectError: true,
			errorMsg:    "invalid platform:",
		},
		{
			name: "empty architecture",
			platformInfo: PlatformInfo{
				Platform:     "windows",
				Architecture: "",
			},
			expectError: true,
			errorMsg:    "invalid architecture:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.platformInfo.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPlatformInfo_String(t *testing.T) {
	tests := []struct {
		name         string
		platformInfo PlatformInfo
		expected     string
	}{
		{
			name: "windows amd64",
			platformInfo: PlatformInfo{
				Platform:     "windows",
				Architecture: "amd64",
			},
			expected: "windows-amd64",
		},
		{
			name: "linux arm64",
			platformInfo: PlatformInfo{
				Platform:     "linux",
				Architecture: "arm64",
			},
			expected: "linux-arm64",
		},
		{
			name: "empty values",
			platformInfo: PlatformInfo{
				Platform:     "",
				Architecture: "",
			},
			expected: "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.platformInfo.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSupportedPlatformsAndArchitectures(t *testing.T) {
	// Test that our constants match the supported lists
	expectedPlatforms := []string{
		PlatformWindows,
		PlatformLinux,
		PlatformDarwin,
		PlatformAndroid,
		PlatformIOS,
	}
	assert.Equal(t, expectedPlatforms, SupportedPlatforms)

	expectedArchitectures := []string{
		ArchAMD64,
		ArchARM64,
		Arch386,
		ArchARM,
	}
	assert.Equal(t, expectedArchitectures, SupportedArchitectures)

	// Test that all constants are lowercase
	for _, platform := range SupportedPlatforms {
		assert.Equal(t, platform, NormalizePlatform(platform))
	}

	for _, arch := range SupportedArchitectures {
		assert.Equal(t, arch, NormalizeArchitecture(arch))
	}
}
