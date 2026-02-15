// Package models - Application and platform management.
// This file defines application metadata, platform support, and configuration structures.
//
// Design Decisions:
// - Platform-agnostic design supporting multiple operating systems and architectures
// - Flexible configuration system allowing per-application customization
// - Strong validation to ensure data integrity and security
// - Normalized platform/architecture identifiers for consistency
package models

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Platform and Architecture Constants
//
// Design Rationale:
// - Uses Go's GOOS naming convention for consistency with build system
// - Covers major desktop and mobile platforms for broad compatibility
// - Architecture names follow Go's GOARCH convention
// - Lowercase naming ensures consistent URL and file naming
const (
	// Supported Operating Systems
	PlatformWindows = "windows" // Microsoft Windows
	PlatformLinux   = "linux"   // Linux distributions
	PlatformDarwin  = "darwin"  // macOS (Apple's Darwin kernel)
	PlatformAndroid = "android" // Android mobile OS
	PlatformIOS     = "ios"     // Apple iOS

	// Supported CPU Architectures
	ArchAMD64 = "amd64" // 64-bit x86 (Intel/AMD)
	ArchARM64 = "arm64" // 64-bit ARM (Apple Silicon, ARM servers)
	Arch386   = "386"   // 32-bit x86 (legacy support)
	ArchARM   = "arm"   // 32-bit ARM (Raspberry Pi, older mobile)
)

var (
	SupportedPlatforms = []string{
		PlatformWindows,
		PlatformLinux,
		PlatformDarwin,
		PlatformAndroid,
		PlatformIOS,
	}

	SupportedArchitectures = []string{
		ArchAMD64,
		ArchARM64,
		Arch386,
		ArchARM,
	}
)

// Application represents a software application that can receive updates.
//
// Design Principles:
// - ID serves as unique identifier and is used in API URLs (must be URL-safe)
// - Platforms array supports multi-platform applications
// - Configuration is embedded for easy access and serialization
// - Timestamps as strings for flexibility with different storage backends
// - Validation tags provide input validation constraints
type Application struct {
	ID          string            `json:"id" validate:"required"`              // Unique application identifier (URL-safe)
	Name        string            `json:"name" validate:"required"`            // Human-readable application name
	Description string            `json:"description"`                         // Optional application description
	Platforms   []string          `json:"platforms" validate:"required,min=1"` // Supported platforms (windows, linux, etc.)
	Config      ApplicationConfig `json:"config"`                              // Application-specific configuration
	CreatedAt   string            `json:"created_at,omitempty"`                // Creation timestamp (RFC3339 format)
	UpdatedAt   string            `json:"updated_at,omitempty"`                // Last modification timestamp
}

// ApplicationConfig contains application-specific settings for update behavior.
//
// Design Considerations:
// - Flexible update policies (auto-update, intervals, required updates)
// - Version constraints for compatibility management
// - Webhook support for integration with external systems
// - Privacy-conscious analytics (disabled by default)
// - Extensible via CustomFields for application-specific metadata
type ApplicationConfig struct {
	UpdateCheckURL   string            `json:"update_check_url,omitempty"` // Custom update check endpoint override
	AutoUpdate       bool              `json:"auto_update"`                // Enable automatic updates
	UpdateInterval   int               `json:"update_interval"`            // Update check interval in seconds
	RequiredUpdate   bool              `json:"required_update"`            // Force updates (security patches)
	MinVersion       string            `json:"min_version,omitempty"`      // Minimum supported version
	MaxVersion       string            `json:"max_version,omitempty"`      // Maximum supported version
	AllowPrerelease  bool              `json:"allow_prerelease"`           // Include pre-release versions
	CustomFields     map[string]string `json:"custom_fields,omitempty"`    // Application-specific metadata
	NotificationURL  string            `json:"notification_url,omitempty"` // Webhook for update notifications
	AnalyticsEnabled bool              `json:"analytics_enabled"`          // Privacy-conscious usage analytics
}

// NewApplication creates a new Application with sensible defaults.
//
// Default Configuration:
// - Auto-update disabled for safety
// - 1-hour update check interval
// - No required updates (user choice)
// - Pre-release versions disabled
// - Analytics disabled (privacy first)
// - Empty custom fields map initialized
func NewApplication(id, name string, platforms []string) *Application {
	return &Application{
		ID:        id,
		Name:      name,
		Platforms: platforms,
		Config: ApplicationConfig{
			AutoUpdate:       false,
			UpdateInterval:   3600, // 1 hour default
			RequiredUpdate:   false,
			AllowPrerelease:  false,
			AnalyticsEnabled: false,
			CustomFields:     make(map[string]string),
		},
	}
}

func (a *Application) Validate() error {
	if a.ID == "" {
		return errors.New("application ID cannot be empty")
	}

	if !isValidID(a.ID) {
		return errors.New("application ID must contain only alphanumeric characters, hyphens, and underscores")
	}

	if a.Name == "" {
		return errors.New("application name cannot be empty")
	}

	if len(a.Platforms) == 0 {
		return errors.New("at least one platform must be specified")
	}

	for _, platform := range a.Platforms {
		if !isValidPlatform(platform) {
			return fmt.Errorf("invalid platform: %s", platform)
		}
	}

	if err := a.Config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	return nil
}

func (a *Application) SupportsPlatform(platform string) bool {
	for _, p := range a.Platforms {
		if p == platform {
			return true
		}
	}
	return false
}

func (a *Application) SupportsArchitecture(platform, arch string) bool {
	if !a.SupportsPlatform(platform) {
		return false
	}
	return isValidArchitecture(arch)
}

func (ac *ApplicationConfig) Validate() error {
	if ac.UpdateInterval < 0 {
		return errors.New("update interval cannot be negative")
	}

	if ac.MinVersion != "" {
		if _, err := semver.NewVersion(ac.MinVersion); err != nil {
			return fmt.Errorf("invalid min version: %w", err)
		}
	}

	if ac.MaxVersion != "" {
		if _, err := semver.NewVersion(ac.MaxVersion); err != nil {
			return fmt.Errorf("invalid max version: %w", err)
		}
	}

	if ac.MinVersion != "" && ac.MaxVersion != "" {
		minVer, _ := semver.NewVersion(ac.MinVersion)
		maxVer, _ := semver.NewVersion(ac.MaxVersion)
		if minVer.GreaterThan(maxVer) {
			return errors.New("min version cannot be greater than max version")
		}
	}

	return nil
}

func isValidID(id string) bool {
	// Allow alphanumeric characters, hyphens, and underscores
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", id)
	return matched && len(id) > 0 && len(id) <= 100
}

func isValidPlatform(platform string) bool {
	platform = strings.ToLower(platform)
	for _, p := range SupportedPlatforms {
		if p == platform {
			return true
		}
	}
	return false
}

func isValidArchitecture(arch string) bool {
	arch = strings.ToLower(arch)
	for _, a := range SupportedArchitectures {
		if a == arch {
			return true
		}
	}
	return false
}

func NormalizePlatform(platform string) string {
	return strings.ToLower(platform)
}

func NormalizeArchitecture(arch string) string {
	return strings.ToLower(arch)
}

// PlatformInfo represents a platform/architecture combination.
//
// Usage:
// - Used for filtering releases by target platform
// - Validates platform/architecture compatibility
// - Provides string representation for file naming and URLs
//
// Example: PlatformInfo{"windows", "amd64"} -> "windows-amd64"
type PlatformInfo struct {
	Platform     string `json:"platform"`     // Operating system (windows, linux, darwin, etc.)
	Architecture string `json:"architecture"` // CPU architecture (amd64, arm64, 386, arm)
}

func (pi *PlatformInfo) Validate() error {
	if !isValidPlatform(pi.Platform) {
		return fmt.Errorf("invalid platform: %s", pi.Platform)
	}
	if !isValidArchitecture(pi.Architecture) {
		return fmt.Errorf("invalid architecture: %s", pi.Architecture)
	}
	return nil
}

func (pi *PlatformInfo) String() string {
	return fmt.Sprintf("%s-%s", pi.Platform, pi.Architecture)
}
