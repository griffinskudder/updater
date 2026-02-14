# Version Models Documentation

## Overview

The version models provide semantic versioning support with flexible parsing and comparison capabilities. This is the foundation for update availability determination and compatibility checking throughout the updater service.

## Design Philosophy

### Core Principles
- **Semantic Versioning Compliance**: Follows Semantic Versioning 2.0.0 specification for industry compatibility
- **Flexible Parsing**: Supports both strict semver and custom versioning schemes
- **Preservation of Original**: Maintains original version string for exact representation
- **Performance Focused**: Efficient comparison operations for high-volume update checks

### Architecture Decisions
- **Build Metadata Handling**: Preserved but doesn't affect version precedence (per semver spec)
- **Pre-release Support**: Full support for alpha, beta, release candidate workflows
- **Constraint System**: Flexible version requirements for dependency management
- **Error Handling**: Clear, actionable error messages for invalid version strings

## Core Components

### Version Struct

```go
type Version struct {
    Major int    // Breaking changes
    Minor int    // Backward compatible features
    Patch int    // Backward compatible bug fixes
    Pre   string // Pre-release identifier (alpha, beta, rc.1)
    Build string // Build metadata (commit hash, build number)
    Raw   string // Original version string
}
```

#### Field Descriptions

**Major Version**
- Incremented for breaking changes
- Indicates incompatible API changes
- Used for major feature releases

**Minor Version**
- Incremented for backward compatible features
- New functionality that doesn't break existing usage
- Used for feature releases

**Patch Version**
- Incremented for backward compatible bug fixes
- No new features, only fixes
- Used for maintenance releases

**Pre-release Identifier**
- Optional string for pre-release versions
- Examples: "alpha", "beta", "rc.1", "dev.123"
- Pre-release versions have lower precedence than normal versions

**Build Metadata**
- Optional string for build information
- Examples: commit hash, build number, compilation date
- Does not affect version precedence in comparisons

**Raw String**
- Preserves the original version string exactly as input
- Used for exact representation and debugging
- Enables round-trip parsing without data loss

### Parsing and Creation

#### ParseVersion Function

```go
func ParseVersion(v string) (*Version, error)
```

**Supported Formats:**
- `"1.2.3"` - Standard semantic version
- `"1.2.3-alpha"` - Pre-release version
- `"1.2.3+build.123"` - Version with build metadata
- `"1.2.3-beta.1+build.456"` - Complete semantic version
- `"1.0"` or `"1"` - Partial versions (missing components default to 0)

**Parsing Logic:**
1. Split on `+` to extract build metadata
2. Split remaining on `-` to extract pre-release identifier
3. Parse major.minor.patch components numerically
4. Validate all numeric components are non-negative integers
5. Preserve original string in Raw field

**Example Usage:**
```go
// Basic version parsing
version, err := ParseVersion("1.2.3")
if err != nil {
    log.Fatalf("Invalid version: %v", err)
}

// Pre-release version
prerelease, _ := ParseVersion("2.0.0-beta.1")

// Version with build metadata
withBuild, _ := ParseVersion("1.0.0+build.123")

// Complete semantic version
complete, _ := ParseVersion("1.0.0-rc.1+build.456")
```

### Version Comparison

#### Core Comparison Method

```go
func (v *Version) Compare(other *Version) int
```

**Return Values:**
- `-1` if `v < other`
- `0` if `v == other`
- `+1` if `v > other`

**Comparison Logic:**
1. Compare major versions numerically
2. If equal, compare minor versions numerically
3. If equal, compare patch versions numerically
4. If all equal, compare pre-release identifiers:
   - Normal version > pre-release version
   - Pre-release versions compared lexically

**Examples:**
```go
v1, _ := ParseVersion("1.0.0")
v2, _ := ParseVersion("0.9.9")
fmt.Println(v1.Compare(v2)) // Output: 1 (v1 > v2)

v3, _ := ParseVersion("1.0.0")
v4, _ := ParseVersion("1.0.0-alpha")
fmt.Println(v3.Compare(v4)) // Output: 1 (normal > prerelease)

v5, _ := ParseVersion("1.0.0-beta")
v6, _ := ParseVersion("1.0.0-alpha")
fmt.Println(v5.Compare(v6)) // Output: 1 (beta > alpha lexically)
```

#### Convenience Comparison Methods

```go
func (v *Version) Equal(other *Version) bool
func (v *Version) GreaterThan(other *Version) bool
func (v *Version) LessThan(other *Version) bool
func (v *Version) GreaterThanOrEqual(other *Version) bool
func (v *Version) LessThanOrEqual(other *Version) bool
```

**Usage Examples:**
```go
current, _ := ParseVersion("1.0.0")
latest, _ := ParseVersion("1.1.0")

if latest.GreaterThan(current) {
    fmt.Println("Update available!")
}

minimum, _ := ParseVersion("0.9.0")
if current.GreaterThanOrEqual(minimum) {
    fmt.Println("Version requirements met")
}
```

### Version Constraints

#### VersionConstraint Struct

```go
type VersionConstraint struct {
    Operator string // Comparison operator
    Version  string // Version string to compare against
}
```

**Supported Operators:**
- `"="`, `"=="`, `""` - Exact match
- `"!="` - Not equal
- `">"` - Greater than
- `">="` - Greater than or equal
- `"<"` - Less than
- `"<="` - Less than or equal

#### Constraint Checking

```go
func (vc *VersionConstraint) Check(version *Version) (bool, error)
```

**Example Usage:**
```go
// Minimum version constraint
constraint := &VersionConstraint{
    Operator: ">=",
    Version:  "1.0.0",
}

userVersion, _ := ParseVersion("1.2.0")
meets, err := constraint.Check(userVersion)
if err != nil {
    log.Printf("Constraint check failed: %v", err)
}
if meets {
    fmt.Println("Version meets minimum requirements")
}

// Exact version constraint
exact := &VersionConstraint{
    Operator: "=",
    Version:  "2.0.0",
}

// Range constraint (can be combined)
constraints := []*VersionConstraint{
    {Operator: ">=", Version: "1.0.0"},
    {Operator: "<", Version: "2.0.0"},
}
```

## Common Usage Patterns

### Update Availability Check

```go
func CheckForUpdate(currentVersionStr string, releases []Release) (*Release, error) {
    currentVersion, err := ParseVersion(currentVersionStr)
    if err != nil {
        return nil, fmt.Errorf("invalid current version: %w", err)
    }

    var latestRelease *Release
    var latestVersion *Version

    for _, release := range releases {
        releaseVersion, err := ParseVersion(release.Version)
        if err != nil {
            continue // Skip invalid versions
        }

        if latestVersion == nil || releaseVersion.GreaterThan(latestVersion) {
            latestVersion = releaseVersion
            latestRelease = &release
        }
    }

    if latestVersion != nil && latestVersion.GreaterThan(currentVersion) {
        return latestRelease, nil
    }

    return nil, nil // No update available
}
```

### Compatibility Checking

```go
func IsCompatible(appVersion, minimumRequired string) (bool, error) {
    app, err := ParseVersion(appVersion)
    if err != nil {
        return false, fmt.Errorf("invalid app version: %w", err)
    }

    min, err := ParseVersion(minimumRequired)
    if err != nil {
        return false, fmt.Errorf("invalid minimum version: %w", err)
    }

    return app.GreaterThanOrEqual(min), nil
}
```

### Version Filtering

```go
func FilterVersions(versions []string, allowPrerelease bool) ([]string, error) {
    var filtered []string

    for _, versionStr := range versions {
        version, err := ParseVersion(versionStr)
        if err != nil {
            continue // Skip invalid versions
        }

        // Skip pre-release versions if not allowed
        if !allowPrerelease && version.Pre != "" {
            continue
        }

        filtered = append(filtered, versionStr)
    }

    return filtered, nil
}
```

## Error Handling

### Common Errors

**Empty Version String**
```go
_, err := ParseVersion("")
// Returns: "version string cannot be empty"
```

**Invalid Format**
```go
_, err := ParseVersion("not.a.version")
// Returns: "invalid major version: not"
```

**Too Many Components**
```go
_, err := ParseVersion("1.2.3.4.5")
// Returns: "invalid version format: 1.2.3.4.5"
```

### Best Practices

**Always Validate Input**
```go
func ProcessVersion(versionStr string) error {
    version, err := ParseVersion(versionStr)
    if err != nil {
        return fmt.Errorf("invalid version '%s': %w", versionStr, err)
    }

    // Process valid version...
    return nil
}
```

**Handle Constraints Gracefully**
```go
func CheckConstraint(constraint *VersionConstraint, version *Version) bool {
    meets, err := constraint.Check(version)
    if err != nil {
        log.Printf("Constraint check failed: %v", err)
        return false // Fail safe
    }
    return meets
}
```

## Performance Considerations

### Parsing Optimization
- Parse versions once and cache `Version` objects
- Avoid repeated parsing of the same version string
- Consider version string interning for frequently used versions

### Comparison Optimization
- Major/minor/patch comparisons are O(1)
- Pre-release comparison is O(n) for string length
- Cache comparison results for frequently compared version pairs

### Memory Usage
- `Version` struct is designed for minimal memory footprint
- String fields use Go's string interning where possible
- Consider using pointers for optional fields in large datasets

## Testing Strategies

### Unit Test Categories

**Parsing Tests**
```go
func TestParseVersion(t *testing.T) {
    tests := []struct {
        input    string
        expected Version
        hasError bool
    }{
        {"1.2.3", Version{Major: 1, Minor: 2, Patch: 3}, false},
        {"1.0.0-alpha", Version{Major: 1, Pre: "alpha"}, false},
        {"invalid", Version{}, true},
    }

    for _, tt := range tests {
        result, err := ParseVersion(tt.input)
        // Validate result matches expected...
    }
}
```

**Comparison Tests**
```go
func TestVersionComparison(t *testing.T) {
    tests := []struct {
        v1, v2   string
        expected int
    }{
        {"1.0.0", "0.9.9", 1},
        {"1.0.0", "1.0.0", 0},
        {"1.0.0-alpha", "1.0.0", -1},
    }

    for _, tt := range tests {
        version1, _ := ParseVersion(tt.v1)
        version2, _ := ParseVersion(tt.v2)
        result := version1.Compare(version2)
        // Validate result matches expected...
    }
}
```

### Property-Based Testing
- Generate random valid version strings
- Verify parsing and re-serialization consistency
- Test comparison transitivity and reflexivity

## Integration Examples

### With Release Models

```go
type Release struct {
    Version string
    // ... other fields
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
```

### With API Models

```go
type UpdateCheckRequest struct {
    CurrentVersion string
}

func (r *UpdateCheckRequest) Validate() error {
    if r.CurrentVersion == "" {
        return errors.New("current_version is required")
    }

    if _, err := ParseVersion(r.CurrentVersion); err != nil {
        return fmt.Errorf("invalid current_version format: %w", err)
    }

    return nil
}
```

## Migration and Evolution

### Backward Compatibility
- New operators can be added to `VersionConstraint` without breaking existing code
- Version parsing is additive-only (new formats don't break existing ones)
- Comparison logic follows semantic versioning spec strictly

### Future Enhancements
- Support for version ranges (e.g., "1.0.0 - 2.0.0")
- Custom comparison algorithms for non-semver schemes
- Performance optimizations for bulk version operations
- Integration with external versioning systems

This version model system provides a robust foundation for all version-related operations in the updater service, ensuring consistent behavior and reliable update logic.