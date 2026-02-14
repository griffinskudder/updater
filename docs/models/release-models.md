# Release Models Documentation

## Overview

The release models handle software release metadata with comprehensive security and integrity verification. This system ensures safe and reliable distribution of application updates while supporting flexible querying and management capabilities.

## Design Philosophy

### Security First Principles
- **Cryptographic Integrity**: Strong checksums (SHA256 preferred) for file verification
- **URL Validation**: Prevent malicious download links through comprehensive validation
- **Audit Trails**: Immutable identifiers and timestamps for security monitoring
- **Input Sanitization**: Comprehensive validation of all release metadata

### Architecture Decisions
- **External Storage Model**: Download files hosted separately for CDN distribution and scaling
- **Composite Identifiers**: Deterministic ID generation for consistency and predictability
- **Metadata Extensibility**: Support for future enhancements without breaking changes
- **Performance Optimization**: Efficient querying and filtering for large release datasets

## Core Components

### Release Struct

```go
type Release struct {
    ID             string            // Unique release identifier
    ApplicationID  string            // Parent application identifier
    Version        string            // Semantic version string
    Platform       string            // Target operating system
    Architecture   string            // Target CPU architecture
    DownloadURL    string            // External download location
    Checksum       string            // Cryptographic hash for integrity
    ChecksumType   string            // Hash algorithm
    FileSize       int64             // File size in bytes
    ReleaseNotes   string            // Human-readable change description
    ReleaseDate    time.Time         // Official release timestamp
    Required       bool              // Force update flag for security patches
    MinimumVersion string            // Required current version for upgrade
    Metadata       map[string]string // Extensible key-value metadata
    CreatedAt      time.Time         // Record creation timestamp
    UpdatedAt      time.Time         // Last modification timestamp
}
```

#### Field Descriptions

**Identity Fields**
- **ID**: Composite identifier (app-version-platform-arch) for uniqueness
- **ApplicationID**: Links to parent application for relationship management
- **Version**: Semantic version string validated through version models

**Target Information**
- **Platform**: Normalized operating system identifier (windows, linux, darwin)
- **Architecture**: Normalized CPU architecture (amd64, arm64, 386, arm)

**Distribution Details**
- **DownloadURL**: External location for file retrieval, must be HTTP/HTTPS
- **FileSize**: Size in bytes for download progress and storage planning
- **ReleaseDate**: Official release timestamp for chronological ordering

**Security Information**
- **Checksum**: Cryptographic hash for file integrity verification
- **ChecksumType**: Algorithm used (sha256 preferred, md5/sha1 for legacy)
- **Required**: Flag indicating critical security updates that must be installed

**Content Description**
- **ReleaseNotes**: Markdown-formatted description of changes and improvements
- **MinimumVersion**: Enforces upgrade path requirements and compatibility
- **Metadata**: Extensible storage for additional release-specific information

### Checksum Security System

#### Supported Algorithms

```go
const (
    ChecksumTypeSHA256 = "sha256" // Recommended: Strong cryptographic hash
    ChecksumTypeMD5    = "md5"    // Legacy: Weak, use only for compatibility
    ChecksumTypeSHA1   = "sha1"   // Legacy: Weak, use only for compatibility
)
```

**Security Considerations:**
- **SHA256**: Recommended for all new releases, provides strong cryptographic integrity
- **MD5/SHA1**: Supported for legacy compatibility but discouraged for new implementations
- **Algorithm Validation**: All checksum types are validated against supported list

#### Checksum Generation and Verification

**Generation Example:**
```go
func (r *Release) GenerateChecksum(data []byte) string {
    switch r.ChecksumType {
    case ChecksumTypeSHA256:
        hash := sha256.Sum256(data)
        return hex.EncodeToString(hash[:])
    default:
        // Fallback to SHA256 for security
        hash := sha256.Sum256(data)
        return hex.EncodeToString(hash[:])
    }
}
```

**Verification Example:**
```go
func (r *Release) VerifyChecksum(data []byte) bool {
    expectedChecksum := strings.ToLower(r.Checksum)
    actualChecksum := strings.ToLower(r.GenerateChecksum(data))
    return expectedChecksum == actualChecksum
}
```

### Release Creation and Management

#### Factory Method
```go
func NewRelease(appID, version, platform, arch, downloadURL string) *Release
```

**Default Security Settings:**
- Generated composite ID for uniqueness
- SHA256 checksum algorithm for strong integrity
- Current timestamp for audit trails
- Non-required update (safety first - let users choose)
- Initialized metadata map for extensibility

**Usage Example:**
```go
// Create new release
release := NewRelease(
    "myapp",
    "1.1.0",
    "windows",
    "amd64",
    "https://releases.example.com/myapp-1.1.0-windows-amd64.exe",
)

// Set security information
release.Checksum = "sha256:abcd1234..."
release.ChecksumType = ChecksumTypeSHA256
release.FileSize = 15728640

// Add content information
release.ReleaseNotes = "Bug fixes and performance improvements"
release.Required = false // Optional update

// Validate before storage
if err := release.Validate(); err != nil {
    return fmt.Errorf("invalid release: %w", err)
}
```

### Validation System

#### Comprehensive Validation
```go
func (r *Release) Validate() error
```

**Validation Rules:**
1. **Required Fields**: ID, ApplicationID, Version, Platform, Architecture, DownloadURL, Checksum
2. **Format Validation**: Version must be valid semantic version
3. **Platform Validation**: Platform and architecture must be supported
4. **URL Validation**: Download URL must be valid HTTP/HTTPS with proper host
5. **Security Validation**: Checksum type must be supported, file size non-negative
6. **Version Constraints**: MinimumVersion must be valid semantic version if specified

**Example Validation:**
```go
release := &Release{
    ID: "myapp-1.0.0-windows-amd64",
    ApplicationID: "myapp",
    Version: "1.0.0",
    Platform: "windows",
    Architecture: "amd64",
    DownloadURL: "https://example.com/file.exe",
    Checksum: "abc123",
    ChecksumType: "sha256",
    FileSize: 1024,
}

err := release.Validate()
// Returns nil if valid, detailed error message if invalid
```

### Release Querying and Filtering

#### ReleaseFilter Struct
```go
type ReleaseFilter struct {
    ApplicationID string   // Filter by application
    Platform      string   // Filter by single platform
    Architecture  string   // Filter by architecture
    Version       string   // Filter by specific version
    Required      *bool    // Filter by required status (nil = all)
    Limit         int      // Maximum results to return
    Offset        int      // Results to skip (pagination)
    SortBy        string   // Field to sort by
    SortOrder     string   // Sort direction (asc/desc)
    Platforms     []string // Filter by multiple platforms
}
```

#### Query Examples

**Basic Filtering:**
```go
// Find latest Windows releases
filter := &ReleaseFilter{
    ApplicationID: "myapp",
    Platform:      "windows",
    Limit:         10,
    SortBy:        "release_date",
    SortOrder:     "desc",
}
```

**Multi-Platform Query:**
```go
// Find releases for desktop platforms
filter := &ReleaseFilter{
    ApplicationID: "myapp",
    Platforms:     []string{"windows", "linux", "darwin"},
    Limit:         50,
}
```

**Security-Critical Updates:**
```go
// Find required updates only
required := true
filter := &ReleaseFilter{
    ApplicationID: "myapp",
    Required:      &required,
    SortBy:        "release_date",
    SortOrder:     "desc",
}
```

### Release Comparison and Compatibility

#### Version Comparison
```go
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
```

#### Platform Compatibility
```go
func (r *Release) IsCompatibleWith(platform, arch string) bool {
    return r.Platform == NormalizePlatform(platform) &&
           r.Architecture == NormalizeArchitecture(arch)
}
```

#### Minimum Version Requirements
```go
func (r *Release) MeetsMinimumVersion(currentVersion string) (bool, error) {
    if r.MinimumVersion == "" {
        return true, nil // No minimum version requirement
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
```

## Common Usage Patterns

### Release Registration Workflow

```go
func RegisterNewRelease(req *RegisterReleaseRequest) (*Release, error) {
    // Create release from request
    release := NewRelease(
        req.ApplicationID,
        req.Version,
        req.Platform,
        req.Architecture,
        req.DownloadURL,
    )

    // Set provided metadata
    release.Checksum = req.Checksum
    release.ChecksumType = req.ChecksumType
    release.FileSize = req.FileSize
    release.ReleaseNotes = req.ReleaseNotes
    release.Required = req.Required
    release.MinimumVersion = req.MinimumVersion
    release.Metadata = req.Metadata

    // Validate before storage
    if err := release.Validate(); err != nil {
        return nil, fmt.Errorf("invalid release data: %w", err)
    }

    // Additional business logic validation
    if err := validateBusinessRules(release); err != nil {
        return nil, fmt.Errorf("business rule violation: %w", err)
    }

    return release, nil
}
```

### Update Availability Detection

```go
func FindAvailableUpdate(appID, currentVersion, platform, arch string, allowPrerelease bool) (*Release, error) {
    // Parse current version for comparison
    current, err := ParseVersion(currentVersion)
    if err != nil {
        return nil, fmt.Errorf("invalid current version: %w", err)
    }

    // Query releases for this platform
    filter := &ReleaseFilter{
        ApplicationID: appID,
        Platform:      platform,
        Architecture:  arch,
        SortBy:        "release_date",
        SortOrder:     "desc",
        Limit:         100, // Reasonable limit for processing
    }

    releases, err := queryReleases(filter)
    if err != nil {
        return nil, fmt.Errorf("failed to query releases: %w", err)
    }

    // Find newest compatible release
    for _, release := range releases {
        releaseVersion, err := ParseVersion(release.Version)
        if err != nil {
            continue // Skip invalid versions
        }

        // Skip pre-release if not allowed
        if !allowPrerelease && releaseVersion.Pre != "" {
            continue
        }

        // Check if this is newer than current version
        if releaseVersion.GreaterThan(current) {
            // Verify minimum version requirements
            meets, err := release.MeetsMinimumVersion(currentVersion)
            if err != nil || !meets {
                continue
            }

            return &release, nil
        }
    }

    return nil, nil // No update available
}
```

### Release Security Validation

```go
func ValidateReleaseIntegrity(release *Release, downloadedData []byte) error {
    // Verify file size
    if int64(len(downloadedData)) != release.FileSize {
        return fmt.Errorf("file size mismatch: expected %d, got %d",
            release.FileSize, len(downloadedData))
    }

    // Verify checksum
    if !release.VerifyChecksum(downloadedData) {
        return fmt.Errorf("checksum verification failed for release %s", release.ID)
    }

    // Additional security checks
    if err := scanForMalware(downloadedData); err != nil {
        return fmt.Errorf("security scan failed: %w", err)
    }

    return nil
}
```

### Metadata Management

```go
func UpdateReleaseMetadata(releaseID string, metadata map[string]string) error {
    release, err := getReleaseByID(releaseID)
    if err != nil {
        return err
    }

    // Update metadata fields
    for key, value := range metadata {
        release.SetMetadata(key, value)
    }

    // Validate updated release
    if err := release.Validate(); err != nil {
        return fmt.Errorf("invalid metadata update: %w", err)
    }

    return saveRelease(release)
}

// Helper method for safe metadata updates
func (r *Release) SetMetadata(key, value string) {
    if r.Metadata == nil {
        r.Metadata = make(map[string]string)
    }
    r.Metadata[key] = value
    r.UpdatedAt = time.Now()
}
```

## Extended Metadata Support

### ReleaseMetadata Struct
```go
type ReleaseMetadata struct {
    FileName    string // Original filename for download
    ContentType string // MIME type for proper handling
    Signature   string // Cryptographic signature (future)
    Publisher   string // Publisher/signer identity
}
```

**Usage Examples:**
```go
// Set extended metadata
release.SetMetadata("filename", "MyApp-1.0.0-Setup.exe")
release.SetMetadata("content_type", "application/vnd.microsoft.portable-executable")
release.SetMetadata("publisher", "My Company")
release.SetMetadata("signature", "sha256-rsa:...")
```

### ReleaseStats for Analytics
```go
type ReleaseStats struct {
    TotalReleases     int       // Total number of releases
    LatestVersion     string    // Most recent version
    LatestReleaseDate time.Time // Date of latest release
    PlatformCount     int       // Number of supported platforms
    RequiredReleases  int       // Number of critical updates
}
```

## Performance Considerations

### Query Optimization
- Use indexes on ApplicationID, Platform, Architecture for fast filtering
- Implement pagination for large result sets
- Cache frequently accessed release metadata
- Pre-compute platform compatibility matrices

### Memory Management
- Struct field ordering optimized for memory alignment
- Metadata maps initialized only when needed
- String interning for common platform/architecture values
- Efficient JSON serialization with appropriate omitempty tags

### Caching Strategies
- Release metadata caching with TTL expiration
- Version comparison result caching for frequently compared pairs
- Platform compatibility caching for hot paths
- Checksum verification result caching for repeated downloads

## Security Best Practices

### Input Validation
- Validate all URLs against allowed schemes (HTTP/HTTPS only)
- Sanitize all string inputs to prevent injection attacks
- Verify file sizes are within reasonable bounds
- Validate checksum formats match expected patterns

### Cryptographic Integrity
- Always use SHA256 or stronger for new releases
- Verify checksums before serving release information
- Consider implementing digital signatures for enhanced security
- Log all checksum verification failures for security monitoring

### Access Control
- Implement authentication for release registration endpoints
- Log all release creation/modification operations
- Validate release ownership before allowing modifications
- Rate limit release queries to prevent abuse

## Testing Strategies

### Unit Testing

**Validation Tests:**
```go
func TestReleaseValidation(t *testing.T) {
    tests := []struct {
        name     string
        release  Release
        hasError bool
    }{
        {
            name: "valid release",
            release: Release{
                ID: "app-1.0.0-windows-amd64",
                ApplicationID: "app",
                Version: "1.0.0",
                Platform: "windows",
                Architecture: "amd64",
                DownloadURL: "https://example.com/file.exe",
                Checksum: "abc123",
                ChecksumType: "sha256",
                FileSize: 1024,
            },
            hasError: false,
        },
        {
            name: "missing checksum",
            release: Release{
                ID: "app-1.0.0-windows-amd64",
                ApplicationID: "app",
                Version: "1.0.0",
                Platform: "windows",
                Architecture: "amd64",
                DownloadURL: "https://example.com/file.exe",
                ChecksumType: "sha256",
                FileSize: 1024,
            },
            hasError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.release.Validate()
            if (err != nil) != tt.hasError {
                t.Errorf("Validate() error = %v, hasError = %v", err, tt.hasError)
            }
        })
    }
}
```

**Checksum Verification Tests:**
```go
func TestChecksumVerification(t *testing.T) {
    release := &Release{
        ChecksumType: ChecksumTypeSHA256,
        Checksum:     "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    }

    // Empty data should match the SHA256 of empty string
    emptyData := []byte{}
    if !release.VerifyChecksum(emptyData) {
        t.Error("Expected checksum verification to pass for empty data")
    }

    // Invalid data should fail
    invalidData := []byte("invalid")
    if release.VerifyChecksum(invalidData) {
        t.Error("Expected checksum verification to fail for invalid data")
    }
}
```

### Integration Testing
- Test release registration with real file uploads
- Validate release querying with large datasets
- Test update detection workflows end-to-end
- Verify security validation with various file types

This release model system provides comprehensive security, performance, and reliability for software distribution while maintaining flexibility for diverse deployment scenarios.