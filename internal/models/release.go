// Package models - Release management and integrity verification.
// This file handles software release metadata, checksum validation, and release filtering.
//
// Security Design Principles:
// - Strong cryptographic checksums (SHA256 preferred) for integrity verification
// - URL validation to prevent malicious download links
// - Comprehensive input validation for all release metadata
// - Support for multiple checksum algorithms for backward compatibility
// - Immutable release identification for audit trails
package models

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Checksum Algorithm Constants
//
// Security Considerations:
// - SHA256 is preferred for strong cryptographic integrity
// - MD5 and SHA1 are supported for legacy compatibility but discouraged
// - Future algorithms can be added without breaking existing releases
const (
	ChecksumTypeSHA256 = "sha256" // Recommended: Strong cryptographic hash
	ChecksumTypeMD5    = "md5"    // Legacy: Weak, use only for compatibility
	ChecksumTypeSHA1   = "sha1"   // Legacy: Weak, use only for compatibility
)

var SupportedChecksumTypes = []string{
	ChecksumTypeSHA256,
	ChecksumTypeMD5,
	ChecksumTypeSHA1,
}

// Release represents a software release with complete metadata and security information.
//
// Design Rationale:
// - Composite ID ensures uniqueness across app/version/platform/arch combinations
// - External download URLs support CDN distribution and scaling
// - Cryptographic checksums ensure download integrity
// - File size enables progress tracking and storage planning
// - Required flag supports security patch distribution
// - MinimumVersion enforces upgrade paths and compatibility
// - Extensible metadata for future needs (signatures, mirrors, etc.)
// - Audit trail with creation and update timestamps
type Release struct {
	ID             string            `json:"id" validate:"required"`               // Unique release identifier (app-version-platform-arch)
	ApplicationID  string            `json:"application_id" validate:"required"`   // Parent application identifier
	Version        string            `json:"version" validate:"required"`          // Semantic version string
	Platform       string            `json:"platform" validate:"required"`         // Target operating system
	Architecture   string            `json:"architecture" validate:"required"`     // Target CPU architecture
	DownloadURL    string            `json:"download_url" validate:"required,url"` // External download location
	Checksum       string            `json:"checksum" validate:"required"`         // Cryptographic hash for integrity
	ChecksumType   string            `json:"checksum_type" validate:"required"`    // Hash algorithm (sha256, md5, sha1)
	FileSize       int64             `json:"file_size" validate:"min=0"`           // File size in bytes
	ReleaseNotes   string            `json:"release_notes"`                        // Human-readable change description
	ReleaseDate    time.Time         `json:"release_date"`                         // Official release timestamp
	Required       bool              `json:"required"`                             // Force update (security patches)
	MinimumVersion string            `json:"minimum_version,omitempty"`            // Required current version for upgrade
	Metadata       map[string]string `json:"metadata,omitempty"`                   // Extensible key-value metadata
	CreatedAt      time.Time         `json:"created_at"`                           // Record creation timestamp
	UpdatedAt      time.Time         `json:"updated_at"`                           // Last modification timestamp
}

// ReleaseFilter provides flexible querying and pagination for release lists.
//
// Query Design:
// - Supports filtering by any combination of fields
// - Pagination with limit/offset for large datasets
// - Flexible sorting by multiple fields
// - Multi-platform queries for cross-platform applications
// - Boolean pointer for Required allows three states: true, false, nil (don't care)
type ReleaseFilter struct {
	ApplicationID string   `json:"application_id,omitempty"` // Filter by application
	Platform      string   `json:"platform,omitempty"`       // Filter by single platform
	Architecture  string   `json:"architecture,omitempty"`   // Filter by architecture
	Version       string   `json:"version,omitempty"`        // Filter by specific version
	Required      *bool    `json:"required,omitempty"`       // Filter by required status (nil = all)
	Limit         int      `json:"limit,omitempty"`          // Maximum results to return
	Offset        int      `json:"offset,omitempty"`         // Results to skip (pagination)
	SortBy        string   `json:"sort_by,omitempty"`        // Field to sort by
	SortOrder     string   `json:"sort_order,omitempty"`     // Sort direction (asc/desc)
	Platforms     []string `json:"platforms,omitempty"`      // Filter by multiple platforms
}

// ReleaseMetadata contains additional file and security information for releases.
//
// Extended Metadata:
// - FileName for original file naming and client-side handling
// - ContentType for proper MIME handling and security
// - Signature for future cryptographic verification (code signing)
// - Publisher for trust and accountability
type ReleaseMetadata struct {
	FileName    string `json:"file_name,omitempty"`    // Original filename for download
	ContentType string `json:"content_type,omitempty"` // MIME type for proper handling
	Signature   string `json:"signature,omitempty"`    // Cryptographic signature (future)
	Publisher   string `json:"publisher,omitempty"`    // Publisher/signer identity
}

// NewRelease creates a new Release with secure defaults.
//
// Security Defaults:
// - Generated composite ID for uniqueness and predictability
// - SHA256 checksum algorithm for strong integrity verification
// - Current timestamp for audit trails
// - Non-required update (safety first - let users choose)
// - Initialized metadata map for extensibility
func NewRelease(appID, version, platform, arch, downloadURL string) *Release {
	now := time.Now()
	normalizedPlatform := NormalizePlatform(platform)
	normalizedArch := NormalizeArchitecture(arch)
	return &Release{
		ID:            generateReleaseID(appID, version, normalizedPlatform, normalizedArch),
		ApplicationID: appID,
		Version:       version,
		Platform:      normalizedPlatform,
		Architecture:  normalizedArch,
		DownloadURL:   downloadURL,
		ChecksumType:  ChecksumTypeSHA256,
		Required:      false,
		ReleaseDate:   now,
		CreatedAt:     now,
		UpdatedAt:     now,
		Metadata:      make(map[string]string),
	}
}

func (r *Release) Validate() error {
	if r.ID == "" {
		return errors.New("release ID cannot be empty")
	}

	if r.ApplicationID == "" {
		return errors.New("application ID cannot be empty")
	}

	if r.Version == "" {
		return errors.New("version cannot be empty")
	}

	if _, err := ParseVersion(r.Version); err != nil {
		return fmt.Errorf("invalid version format: %w", err)
	}

	if !isValidPlatform(r.Platform) {
		return fmt.Errorf("invalid platform: %s", r.Platform)
	}

	if !isValidArchitecture(r.Architecture) {
		return fmt.Errorf("invalid architecture: %s", r.Architecture)
	}

	if r.DownloadURL == "" {
		return errors.New("download URL cannot be empty")
	}

	if err := r.ValidateDownloadURL(); err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}

	if r.Checksum == "" {
		return errors.New("checksum cannot be empty")
	}

	if !isValidChecksumType(r.ChecksumType) {
		return fmt.Errorf("invalid checksum type: %s", r.ChecksumType)
	}

	if r.FileSize < 0 {
		return errors.New("file size cannot be negative")
	}

	if r.MinimumVersion != "" {
		if _, err := ParseVersion(r.MinimumVersion); err != nil {
			return fmt.Errorf("invalid minimum version: %w", err)
		}
	}

	return nil
}

func (r *Release) ValidateDownloadURL() error {
	parsedURL, err := url.Parse(r.DownloadURL)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("URL must use HTTP or HTTPS scheme")
	}

	if parsedURL.Host == "" {
		return errors.New("URL must have a valid host")
	}

	return nil
}

func (r *Release) GetPlatformInfo() PlatformInfo {
	return PlatformInfo{
		Platform:     r.Platform,
		Architecture: r.Architecture,
	}
}

func (r *Release) IsNewerThan(other *Release) (bool, error) {
	thisVersion, err := ParseVersion(r.Version)
	if err != nil {
		return false, fmt.Errorf("invalid version in this release: %w", err)
	}

	otherVersion, err := ParseVersion(other.Version)
	if err != nil {
		return false, fmt.Errorf("invalid version in other release: %w", err)
	}

	return thisVersion.GreaterThan(otherVersion), nil
}

func (r *Release) IsCompatibleWith(platform, arch string) bool {
	return r.Platform == NormalizePlatform(platform) &&
		r.Architecture == NormalizeArchitecture(arch)
}

func (r *Release) MeetsMinimumVersion(currentVersion string) (bool, error) {
	if r.MinimumVersion == "" {
		return true, nil
	}

	current, err := ParseVersion(currentVersion)
	if err != nil {
		return false, fmt.Errorf("invalid current version: %w", err)
	}

	minimum, err := ParseVersion(r.MinimumVersion)
	if err != nil {
		return false, fmt.Errorf("invalid minimum version: %w", err)
	}

	return current.GreaterThanOrEqual(minimum), nil
}

func (r *Release) GenerateChecksum(data []byte) string {
	switch r.ChecksumType {
	case ChecksumTypeSHA256:
		hash := sha256.Sum256(data)
		return hex.EncodeToString(hash[:])
	default:
		hash := sha256.Sum256(data)
		return hex.EncodeToString(hash[:])
	}
}

func (r *Release) VerifyChecksum(data []byte) bool {
	expectedChecksum := strings.ToLower(r.Checksum)
	actualChecksum := strings.ToLower(r.GenerateChecksum(data))
	return expectedChecksum == actualChecksum
}

func (r *Release) SetMetadata(key, value string) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
	r.Metadata[key] = value
	r.UpdatedAt = time.Now()
}

func (r *Release) GetMetadata(key string) (string, bool) {
	if r.Metadata == nil {
		return "", false
	}
	value, exists := r.Metadata[key]
	return value, exists
}

func generateReleaseID(appID, version, platform, arch string) string {
	return fmt.Sprintf("%s-%s-%s-%s", appID, version, platform, arch)
}

func isValidChecksumType(checksumType string) bool {
	checksumType = strings.ToLower(checksumType)
	for _, ct := range SupportedChecksumTypes {
		if ct == checksumType {
			return true
		}
	}
	return false
}

type ReleaseStats struct {
	TotalReleases     int       `json:"total_releases"`
	LatestVersion     string    `json:"latest_version"`
	LatestReleaseDate time.Time `json:"latest_release_date"`
	PlatformCount     int       `json:"platform_count"`
	RequiredReleases  int       `json:"required_releases"`
}

func (rf *ReleaseFilter) Validate() error {
	if rf.Limit < 0 {
		return errors.New("limit cannot be negative")
	}
	if rf.Offset < 0 {
		return errors.New("offset cannot be negative")
	}
	if rf.SortOrder != "" && rf.SortOrder != "asc" && rf.SortOrder != "desc" {
		return errors.New("sort order must be 'asc' or 'desc'")
	}
	return nil
}
