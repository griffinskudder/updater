// Package models - API request types and input validation.
// This file defines all incoming API request structures with comprehensive validation.
//
// Validation Philosophy:
// - Fail fast with clear error messages for invalid input
// - Normalize input data for consistent processing (lowercase platforms, trimmed strings)
// - Validate business rules (version formats, platform support, constraints)
// - Provide sensible defaults where appropriate
// - Separate validation from normalization for clear error reporting
package models

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// UpdateCheckRequest represents a request to check for available updates.
//
// Core API Design:
// - Required fields ensure we have minimum information for meaningful responses
// - Platform/Architecture pair determines compatibility matching
// - AllowPrerelease enables beta testing workflows
// - IncludeMetadata controls response size (metadata can be large)
// - UserAgent and ClientID support analytics and debugging (optional)
//
// Security Notes:
// - No sensitive information should be included
// - All fields are validated before processing
// - Version format is strictly validated to prevent injection
type UpdateCheckRequest struct {
	ApplicationID   string `json:"application_id" validate:"required"`  // Target application identifier
	CurrentVersion  string `json:"current_version" validate:"required"` // Client's current version
	Platform        string `json:"platform" validate:"required"`        // Target OS (windows, linux, darwin)
	Architecture    string `json:"architecture" validate:"required"`    // Target arch (amd64, arm64, 386, arm)
	AllowPrerelease bool   `json:"allow_prerelease"`                    // Include pre-release versions
	IncludeMetadata bool   `json:"include_metadata"`                    // Include release metadata in response
	UserAgent       string `json:"user_agent,omitempty"`                // Client identification (optional)
	ClientID        string `json:"client_id,omitempty"`                 // Unique client ID (optional analytics)
}

type LatestVersionRequest struct {
	ApplicationID   string `json:"application_id" validate:"required"`
	Platform        string `json:"platform" validate:"required"`
	Architecture    string `json:"architecture" validate:"required"`
	AllowPrerelease bool   `json:"allow_prerelease"`
	IncludeMetadata bool   `json:"include_metadata"`
}

type ListReleasesRequest struct {
	ApplicationID string   `json:"application_id" validate:"required"`
	Platform      string   `json:"platform,omitempty"`
	Architecture  string   `json:"architecture,omitempty"`
	Version       string   `json:"version,omitempty"`
	Required      *bool    `json:"required,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	Offset        int      `json:"offset,omitempty"`
	SortBy        string   `json:"sort_by,omitempty"`
	SortOrder     string   `json:"sort_order,omitempty"`
	Platforms     []string `json:"platforms,omitempty"`
}

// RegisterReleaseRequest represents a request to register a new release (admin operation).
//
// Administrative API Design:
// - Complete release information required for proper distribution
// - URL validation ensures download links are accessible and safe
// - Checksum and size validation for integrity verification
// - Flexible metadata support for extensibility
// - Required flag enables critical security update workflows
// - MinimumVersion supports controlled upgrade paths
//
// Security Considerations:
// - This is an admin-only operation requiring authentication
// - All URLs are validated to prevent malicious links
// - Checksums must be provided to ensure integrity
// - File size helps detect corruption and manage storage
type RegisterReleaseRequest struct {
	ApplicationID  string            `json:"application_id" validate:"required"`   // Target application
	Version        string            `json:"version" validate:"required"`          // Release version (semantic)
	Platform       string            `json:"platform" validate:"required"`         // Target platform
	Architecture   string            `json:"architecture" validate:"required"`     // Target architecture
	DownloadURL    string            `json:"download_url" validate:"required,url"` // External download location
	Checksum       string            `json:"checksum" validate:"required"`         // File integrity hash
	ChecksumType   string            `json:"checksum_type" validate:"required"`    // Hash algorithm
	FileSize       int64             `json:"file_size" validate:"min=0"`           // File size in bytes
	ReleaseNotes   string            `json:"release_notes"`                        // Change description
	Required       bool              `json:"required"`                             // Force update flag
	MinimumVersion string            `json:"minimum_version,omitempty"`            // Required current version
	Metadata       map[string]string `json:"metadata,omitempty"`                   // Additional metadata
}

type CreateApplicationRequest struct {
	ID          string            `json:"id" validate:"required"`
	Name        string            `json:"name" validate:"required"`
	Description string            `json:"description"`
	Platforms   []string          `json:"platforms" validate:"required,min=1"`
	Config      ApplicationConfig `json:"config"`
}

type UpdateApplicationRequest struct {
	Name        *string            `json:"name,omitempty"`
	Description *string            `json:"description,omitempty"`
	Platforms   []string           `json:"platforms,omitempty"`
	Config      *ApplicationConfig `json:"config,omitempty"`
}

type DeleteReleaseRequest struct {
	ApplicationID string `json:"application_id" validate:"required"`
	ReleaseID     string `json:"release_id" validate:"required"`
}

type HealthCheckRequest struct {
	Component string `json:"component,omitempty"`
	Deep      bool   `json:"deep"`
}

func (r *UpdateCheckRequest) Validate() error {
	if err := validateRequiredFields(r.ApplicationID, r.Platform, r.Architecture); err != nil {
		return err
	}

	if err := validateVersion(r.CurrentVersion); err != nil {
		return fmt.Errorf("invalid current_version: %w", err)
	}

	return nil
}

func (r *UpdateCheckRequest) Normalize() {
	normalizeCommonFields(&r.ApplicationID, &r.Platform, &r.Architecture)
	r.CurrentVersion = strings.TrimSpace(r.CurrentVersion)
}

func (r *LatestVersionRequest) Validate() error {
	return validateRequiredFields(r.ApplicationID, r.Platform, r.Architecture)
}

func (r *LatestVersionRequest) Normalize() {
	normalizeCommonFields(&r.ApplicationID, &r.Platform, &r.Architecture)
}

func (r *ListReleasesRequest) Validate() error {
	if r.ApplicationID == "" {
		return errors.New("application_id is required")
	}

	if r.Platform != "" && !isValidPlatform(r.Platform) {
		return fmt.Errorf("invalid platform: %s", r.Platform)
	}

	if r.Architecture != "" && !isValidArchitecture(r.Architecture) {
		return fmt.Errorf("invalid architecture: %s", r.Architecture)
	}

	if r.Version != "" {
		if _, err := semver.NewVersion(r.Version); err != nil {
			return fmt.Errorf("invalid version format: %w", err)
		}
	}

	if r.Limit < 0 {
		return errors.New("limit cannot be negative")
	}

	if r.Offset < 0 {
		return errors.New("offset cannot be negative")
	}

	if r.SortOrder != "" && r.SortOrder != "asc" && r.SortOrder != "desc" {
		return errors.New("sort_order must be 'asc' or 'desc'")
	}

	validSortFields := []string{"version", "release_date", "platform", "architecture", "created_at"}
	if r.SortBy != "" {
		found := false
		for _, field := range validSortFields {
			if r.SortBy == field {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid sort_by field: %s", r.SortBy)
		}
	}

	for _, platform := range r.Platforms {
		if !isValidPlatform(platform) {
			return fmt.Errorf("invalid platform in platforms list: %s", platform)
		}
	}

	return nil
}

func (r *ListReleasesRequest) Normalize() {
	r.Platform = NormalizePlatform(r.Platform)
	r.Architecture = NormalizeArchitecture(r.Architecture)
	r.ApplicationID = strings.TrimSpace(r.ApplicationID)

	for i, platform := range r.Platforms {
		r.Platforms[i] = NormalizePlatform(platform)
	}

	if r.Limit == 0 {
		r.Limit = 50 // Default limit
	}
	if r.SortBy == "" {
		r.SortBy = "release_date"
	}
	if r.SortOrder == "" {
		r.SortOrder = "desc"
	}
}

func (r *RegisterReleaseRequest) Validate() error {
	if err := validateRequiredFields(r.ApplicationID, r.Platform, r.Architecture); err != nil {
		return err
	}

	if err := validateVersion(r.Version); err != nil {
		return fmt.Errorf("invalid version: %w", err)
	}

	if r.DownloadURL == "" {
		return errors.New("download_url is required")
	}

	if r.Checksum == "" {
		return errors.New("checksum is required")
	}

	if r.ChecksumType == "" {
		return errors.New("checksum_type is required")
	}

	if !isValidChecksumType(r.ChecksumType) {
		return fmt.Errorf("invalid checksum_type: %s", r.ChecksumType)
	}

	if r.FileSize < 0 {
		return errors.New("file_size cannot be negative")
	}

	if r.MinimumVersion != "" {
		if _, err := semver.NewVersion(r.MinimumVersion); err != nil {
			return fmt.Errorf("invalid minimum_version format: %w", err)
		}
	}

	return nil
}

func (r *RegisterReleaseRequest) Normalize() {
	normalizeCommonFields(&r.ApplicationID, &r.Platform, &r.Architecture)
	r.ChecksumType = strings.ToLower(r.ChecksumType)
	r.Version = strings.TrimSpace(r.Version)
	r.DownloadURL = strings.TrimSpace(r.DownloadURL)
	r.Checksum = strings.TrimSpace(strings.ToLower(r.Checksum))
}

func (r *CreateApplicationRequest) Validate() error {
	if r.ID == "" {
		return errors.New("id is required")
	}

	if !isValidID(r.ID) {
		return errors.New("id must contain only alphanumeric characters, hyphens, and underscores")
	}

	if r.Name == "" {
		return errors.New("name is required")
	}

	if len(r.Platforms) == 0 {
		return errors.New("at least one platform must be specified")
	}

	for _, platform := range r.Platforms {
		if !isValidPlatform(platform) {
			return fmt.Errorf("invalid platform: %s", platform)
		}
	}

	if err := r.Config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	return nil
}

func (r *CreateApplicationRequest) Normalize() {
	r.ID = strings.TrimSpace(r.ID)
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)

	for i, platform := range r.Platforms {
		r.Platforms[i] = NormalizePlatform(platform)
	}
}

func (r *UpdateApplicationRequest) Validate() error {
	if r.Platforms != nil {
		if len(r.Platforms) == 0 {
			return errors.New("at least one platform must be specified")
		}

		for _, platform := range r.Platforms {
			if !isValidPlatform(platform) {
				return fmt.Errorf("invalid platform: %s", platform)
			}
		}
	}

	if r.Config != nil {
		if err := r.Config.Validate(); err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
	}

	return nil
}

func (r *UpdateApplicationRequest) Normalize() {
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		r.Name = &name
	}

	if r.Description != nil {
		desc := strings.TrimSpace(*r.Description)
		r.Description = &desc
	}

	if r.Platforms != nil {
		for i, platform := range r.Platforms {
			r.Platforms[i] = NormalizePlatform(platform)
		}
	}
}

// validateRequiredFields validates common required fields across request types
func validateRequiredFields(appID, platform, architecture string) error {
	if appID == "" {
		return errors.New("application_id is required")
	}

	if platform == "" {
		return errors.New("platform is required")
	}

	if !isValidPlatform(platform) {
		return fmt.Errorf("invalid platform: %s", platform)
	}

	if architecture == "" {
		return errors.New("architecture is required")
	}

	if !isValidArchitecture(architecture) {
		return fmt.Errorf("invalid architecture: %s", architecture)
	}

	return nil
}

// validateVersion validates a semantic version string using semver library
func validateVersion(version string) error {
	if version == "" {
		return errors.New("version is required")
	}

	if _, err := semver.NewVersion(version); err != nil {
		return fmt.Errorf("invalid version format: %w", err)
	}

	return nil
}

// normalizeCommonFields normalizes common fields across request types
func normalizeCommonFields(appID, platform, arch *string) {
	if appID != nil {
		*appID = strings.TrimSpace(*appID)
	}
	if platform != nil {
		*platform = NormalizePlatform(*platform)
	}
	if arch != nil {
		*arch = NormalizeArchitecture(*arch)
	}
}
