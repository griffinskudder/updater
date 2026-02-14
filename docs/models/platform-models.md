# Platform & Application Models Documentation

## Overview

The platform and application models manage multi-platform support, application metadata, and configuration settings. This system enables the updater service to support diverse operating systems and hardware architectures while providing flexible application-specific configuration.

## Design Philosophy

### Core Principles
- **Platform Agnostic**: Support for multiple operating systems and architectures
- **Go Convention Alignment**: Uses GOOS/GOARCH naming for consistency with Go build system
- **Configuration Flexibility**: Allows per-application customization of update behavior
- **Security Consciousness**: Privacy-first defaults with security-aware configuration
- **Extensibility**: Support for future platforms and configuration options

### Architecture Decisions
- **Normalized Identifiers**: Consistent lowercase naming for platforms and architectures
- **Validation at Boundaries**: Input validation with clear error messages
- **Immutable Configuration**: Application configuration changes require explicit updates
- **Default Safety**: Conservative defaults that prioritize user choice and security

## Platform Support System

### Platform Constants

#### Supported Operating Systems
```go
const (
    PlatformWindows = "windows" // Microsoft Windows
    PlatformLinux   = "linux"   // Linux distributions
    PlatformDarwin  = "darwin"  // macOS (Apple's Darwin kernel)
    PlatformAndroid = "android" // Android mobile OS
    PlatformIOS     = "ios"     // Apple iOS
)
```

**Design Rationale:**
- **Consistent Naming**: Uses Go's GOOS convention for build system compatibility
- **Broad Coverage**: Supports major desktop and mobile platforms
- **Lowercase Format**: Ensures consistent URL and file naming across the system
- **Extensible**: New platforms can be added without breaking existing functionality

#### Supported CPU Architectures
```go
const (
    ArchAMD64 = "amd64" // 64-bit x86 (Intel/AMD)
    ArchARM64 = "arm64" // 64-bit ARM (Apple Silicon, ARM servers)
    Arch386   = "386"   // 32-bit x86 (legacy support)
    ArchARM   = "arm"   // 32-bit ARM (Raspberry Pi, older mobile)
)
```

**Coverage Strategy:**
- **Modern Focus**: Prioritizes 64-bit architectures (amd64, arm64)
- **Legacy Support**: Maintains 32-bit support for compatibility
- **Future Ready**: ARM64 support for modern ARM processors
- **Embedded Support**: ARM support for IoT and embedded systems

### Platform Information

#### PlatformInfo Struct
```go
type PlatformInfo struct {
    Platform     string // Operating system
    Architecture string // CPU architecture
}
```

**Usage Examples:**
```go
// Create platform info
platform := PlatformInfo{
    Platform:     PlatformWindows,
    Architecture: ArchAMD64,
}

// Validate compatibility
err := platform.Validate()
if err != nil {
    log.Printf("Invalid platform: %v", err)
}

// String representation for file naming
filename := fmt.Sprintf("app-%s.exe", platform.String())
// Results in: "app-windows-amd64.exe"
```

#### Validation and Normalization

**Platform Validation:**
```go
func isValidPlatform(platform string) bool
func isValidArchitecture(arch string) bool
```

**Normalization Functions:**
```go
func NormalizePlatform(platform string) string
func NormalizeArchitecture(arch string) string
```

**Example Usage:**
```go
// Normalize user input
userPlatform := "Windows"  // User input
normalized := NormalizePlatform(userPlatform)  // "windows"

// Validation
if !isValidPlatform(normalized) {
    return fmt.Errorf("unsupported platform: %s", userPlatform)
}
```

## Application Management

### Application Struct

```go
type Application struct {
    ID          string            // Unique application identifier
    Name        string            // Human-readable application name
    Description string            // Optional application description
    Platforms   []string          // Supported platforms
    Config      ApplicationConfig // Application-specific configuration
    CreatedAt   string            // Creation timestamp (RFC3339)
    UpdatedAt   string            // Last modification timestamp
}
```

#### Field Descriptions

**Application ID**
- Must be URL-safe for use in API endpoints
- Unique across all applications in the system
- Validated with regex pattern: `^[a-zA-Z0-9_-]+$`
- Length limited to 100 characters for practical use

**Name and Description**
- Name is required and serves as display text
- Description is optional and provides additional context
- Both fields support Unicode for international applications

**Platforms Array**
- List of supported platforms for the application
- Must contain at least one valid platform
- Validated against supported platform constants
- Used for filtering releases by target platform

**Timestamps**
- Stored as RFC3339 strings for flexibility
- Compatible with various storage backends
- Include timezone information for global deployments

### Application Configuration

```go
type ApplicationConfig struct {
    UpdateCheckURL    string            // Custom update check endpoint override
    AutoUpdate        bool              // Enable automatic updates
    UpdateInterval    int               // Update check interval in seconds
    RequiredUpdate    bool              // Force updates (security patches)
    MinVersion        string            // Minimum supported version
    MaxVersion        string            // Maximum supported version
    AllowPrerelease   bool              // Include pre-release versions
    CustomFields      map[string]string // Application-specific metadata
    NotificationURL   string            // Webhook for update notifications
    AnalyticsEnabled  bool              // Privacy-conscious usage analytics
}
```

#### Configuration Options

**Update Behavior**
- **AutoUpdate**: Disabled by default for user safety
- **UpdateInterval**: Default 3600 seconds (1 hour) for reasonable checking frequency
- **RequiredUpdate**: Forces critical security updates when enabled

**Version Constraints**
- **MinVersion**: Enforces minimum client version requirements
- **MaxVersion**: Sets maximum supported version for compatibility
- **AllowPrerelease**: Enables beta testing workflows

**Integration Features**
- **UpdateCheckURL**: Allows custom update endpoints for specialized deployments
- **NotificationURL**: Webhook integration for external systems
- **CustomFields**: Extensible metadata for application-specific needs

**Privacy and Analytics**
- **AnalyticsEnabled**: Disabled by default, respects user privacy
- Minimal data collection philosophy
- Focus on update functionality, not user behavior

### Application Creation and Management

#### Factory Method
```go
func NewApplication(id, name string, platforms []string) *Application
```

**Default Configuration Values:**
```go
Config: ApplicationConfig{
    AutoUpdate:       false,        // Safety first
    UpdateInterval:   3600,         // 1 hour
    RequiredUpdate:   false,        // User choice
    AllowPrerelease:  false,        // Stable releases only
    AnalyticsEnabled: false,        // Privacy first
    CustomFields:     make(map[string]string),
}
```

**Usage Example:**
```go
// Create new application
app := NewApplication(
    "myapp",
    "My Application",
    []string{PlatformWindows, PlatformLinux, PlatformDarwin},
)

// Customize configuration
app.Config.AutoUpdate = true
app.Config.UpdateInterval = 1800  // 30 minutes
app.Config.AllowPrerelease = true

// Validate before storage
if err := app.Validate(); err != nil {
    log.Fatalf("Invalid application: %v", err)
}
```

### Validation System

#### Application Validation
```go
func (a *Application) Validate() error
```

**Validation Rules:**
1. **ID Requirements**: Non-empty, URL-safe characters only, length ≤ 100
2. **Name Requirements**: Non-empty string
3. **Platform Requirements**: At least one valid platform
4. **Configuration Validation**: All config fields must be valid

**Example Validation Errors:**
```go
// Invalid ID
app := &Application{ID: "invalid@id"}
err := app.Validate()
// Returns: "application ID must contain only alphanumeric characters, hyphens, and underscores"

// No platforms
app := &Application{ID: "valid", Name: "Test", Platforms: []string{}}
err := app.Validate()
// Returns: "at least one platform must be specified"
```

#### Configuration Validation
```go
func (ac *ApplicationConfig) Validate() error
```

**Validation Checks:**
- Update interval cannot be negative
- Version strings must be valid semantic versions
- MinVersion must be ≤ MaxVersion if both specified
- URL fields must be valid HTTP/HTTPS URLs

## Common Usage Patterns

### Multi-Platform Application Setup

```go
func CreateMultiPlatformApp() (*Application, error) {
    app := NewApplication(
        "cross-platform-app",
        "Cross Platform Application",
        []string{
            PlatformWindows,
            PlatformLinux,
            PlatformDarwin,
        },
    )

    // Configure for broad compatibility
    app.Config.MinVersion = "1.0.0"
    app.Config.AllowPrerelease = false
    app.Config.UpdateInterval = 7200  // 2 hours

    // Add custom metadata
    app.Config.CustomFields["category"] = "productivity"
    app.Config.CustomFields["license"] = "MIT"

    return app, app.Validate()
}
```

### Platform Filtering

```go
func GetCompatibleReleases(app *Application, userPlatform, userArch string) []Release {
    if !app.SupportsPlatform(userPlatform) {
        return nil  // Platform not supported
    }

    if !app.SupportsArchitecture(userPlatform, userArch) {
        return nil  // Architecture not supported
    }

    // Filter releases for this platform/arch combination
    var compatible []Release
    for _, release := range allReleases {
        if release.IsCompatibleWith(userPlatform, userArch) {
            compatible = append(compatible, release)
        }
    }

    return compatible
}
```

### Configuration Management

```go
func UpdateApplicationConfig(appID string, updates map[string]interface{}) error {
    app, err := getApplication(appID)
    if err != nil {
        return err
    }

    // Apply configuration updates
    if interval, ok := updates["update_interval"].(int); ok {
        app.Config.UpdateInterval = interval
    }

    if autoUpdate, ok := updates["auto_update"].(bool); ok {
        app.Config.AutoUpdate = autoUpdate
    }

    if minVersion, ok := updates["min_version"].(string); ok {
        app.Config.MinVersion = minVersion
    }

    // Validate updated configuration
    if err := app.Config.Validate(); err != nil {
        return fmt.Errorf("invalid configuration update: %w", err)
    }

    app.UpdatedAt = time.Now().Format(time.RFC3339)
    return saveApplication(app)
}
```

## Integration Examples

### With Release Models

```go
func FilterReleasesByPlatform(releases []Release, platform, arch string) []Release {
    platformInfo := PlatformInfo{
        Platform:     NormalizePlatform(platform),
        Architecture: NormalizeArchitecture(arch),
    }

    if err := platformInfo.Validate(); err != nil {
        return nil  // Invalid platform/arch combination
    }

    var filtered []Release
    for _, release := range releases {
        if release.IsCompatibleWith(platform, arch) {
            filtered = append(filtered, release)
        }
    }

    return filtered
}
```

### With API Models

```go
func ValidateUpdateRequest(req *UpdateCheckRequest) error {
    // Validate platform
    if !isValidPlatform(req.Platform) {
        return fmt.Errorf("unsupported platform: %s", req.Platform)
    }

    // Validate architecture
    if !isValidArchitecture(req.Architecture) {
        return fmt.Errorf("unsupported architecture: %s", req.Architecture)
    }

    // Normalize for consistent processing
    req.Platform = NormalizePlatform(req.Platform)
    req.Architecture = NormalizeArchitecture(req.Architecture)

    return nil
}
```

### With Configuration Models

```go
func LoadApplicationConfiguration(configPath string, appID string) (*Application, error) {
    // Load base configuration
    config, err := LoadConfig(configPath)
    if err != nil {
        return nil, err
    }

    // Create application with defaults
    app := NewApplication(appID, "Default App", []string{PlatformLinux})

    // Override with configuration values
    if config.Applications[appID] != nil {
        app.Config = *config.Applications[appID]
    }

    return app, app.Validate()
}
```

## Performance Considerations

### Validation Optimization
- Platform/architecture validation uses pre-computed lookup tables
- Regex compilation is done once at startup for ID validation
- String normalization uses efficient lowercase conversion

### Memory Efficiency
- Platform constants use string interning
- CustomFields map is initialized only when needed
- Struct field ordering optimized for memory alignment

### Caching Strategies
- Platform validation results can be cached
- Application configurations are typically cached in memory
- Platform compatibility matrices can be pre-computed

## Security Considerations

### Input Validation
- All user input is validated against known good values
- Platform/architecture strings are normalized to prevent variations
- Application IDs are restricted to URL-safe characters

### Configuration Security
- Default configurations prioritize user privacy and safety
- Auto-updates are disabled by default to prevent unauthorized changes
- Version constraints prevent downgrade attacks when properly configured

### Access Control
- Application creation/modification should require authentication
- Configuration changes should be logged for audit trails
- Platform filtering prevents access to incompatible releases

## Testing Strategies

### Unit Testing

**Platform Validation Tests**
```go
func TestPlatformValidation(t *testing.T) {
    validPlatforms := []string{"windows", "linux", "darwin"}
    invalidPlatforms := []string{"", "invalid", "WINDOWS"}

    for _, platform := range validPlatforms {
        if !isValidPlatform(platform) {
            t.Errorf("Expected %s to be valid", platform)
        }
    }

    for _, platform := range invalidPlatforms {
        if isValidPlatform(platform) {
            t.Errorf("Expected %s to be invalid", platform)
        }
    }
}
```

**Application Configuration Tests**
```go
func TestApplicationConfig(t *testing.T) {
    tests := []struct {
        config   ApplicationConfig
        hasError bool
    }{
        {ApplicationConfig{UpdateInterval: 3600}, false},
        {ApplicationConfig{UpdateInterval: -1}, true},
        {ApplicationConfig{MinVersion: "1.0.0", MaxVersion: "0.9.0"}, true},
    }

    for _, tt := range tests {
        err := tt.config.Validate()
        hasError := err != nil
        if hasError != tt.hasError {
            t.Errorf("Config %+v: expected error=%v, got=%v",
                tt.config, tt.hasError, hasError)
        }
    }
}
```

### Integration Testing
- Test platform filtering with real release data
- Validate configuration loading from various sources
- Test application creation/update workflows

## Migration and Evolution

### Backward Compatibility
- New platforms can be added without breaking existing applications
- Configuration fields use optional/default patterns for safe evolution
- Platform validation is additive-only

### Future Enhancements
- Support for custom architecture definitions
- Enhanced configuration validation with business rules
- Integration with external platform detection services
- Advanced platform capability detection (OS version, features, etc.)

This platform and application model system provides comprehensive multi-platform support while maintaining security, performance, and ease of use for the updater service.