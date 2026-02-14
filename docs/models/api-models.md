# API Models Documentation

## Overview

The API models define standardized HTTP request/response contracts with comprehensive validation and error handling. This system ensures consistent communication between clients and the updater service while providing robust input validation and clear error reporting.

## Design Philosophy

### Request/Response Design Principles
- **Consistent Structure**: Standardized JSON format across all endpoints
- **Validation First**: Fail fast with clear error messages for invalid input
- **Performance Aware**: Optional fields and pagination to control response sizes
- **Client Friendly**: Helper methods and clear documentation for easy integration

### Error Handling Strategy
- **Machine Readable**: Structured error codes for programmatic handling
- **Human Friendly**: Clear messages for user interfaces and debugging
- **Contextual Information**: Field-specific errors and debugging metadata
- **Consistent Format**: Unified error structure across all endpoints

## Request Models

### Core Request Types

#### UpdateCheckRequest
```go
type UpdateCheckRequest struct {
    ApplicationID    string // Target application identifier
    CurrentVersion   string // Client's current version
    Platform         string // Target OS (windows, linux, darwin)
    Architecture     string // Target arch (amd64, arm64, 386, arm)
    AllowPrerelease  bool   // Include pre-release versions
    IncludeMetadata  bool   // Include release metadata in response
    UserAgent        string // Client identification (optional)
    ClientID         string // Unique client ID (optional analytics)
}
```

**Purpose**: Primary endpoint for checking update availability
**Security**: No sensitive information, all fields validated
**Usage Pattern**: High-frequency endpoint, optimized for performance

**Validation Rules:**
- ApplicationID: Required, non-empty string
- CurrentVersion: Required, valid semantic version format
- Platform: Required, must be supported platform
- Architecture: Required, must be supported architecture

**Example Usage:**
```go
request := &UpdateCheckRequest{
    ApplicationID:    "myapp",
    CurrentVersion:   "1.0.0",
    Platform:         "windows",
    Architecture:     "amd64",
    AllowPrerelease:  false,
    IncludeMetadata:  true,
}

if err := request.Validate(); err != nil {
    return NewErrorResponse(err.Error(), ErrorCodeValidation)
}
```

#### LatestVersionRequest
```go
type LatestVersionRequest struct {
    ApplicationID   string // Target application identifier
    Platform        string // Target platform
    Architecture    string // Target architecture
    AllowPrerelease bool   // Include pre-release versions
    IncludeMetadata bool   // Include release metadata in response
}
```

**Purpose**: Get latest version without comparison to current version
**Use Case**: Initial installation, clean slate updates

#### ListReleasesRequest
```go
type ListReleasesRequest struct {
    ApplicationID string   // Required: Filter by application
    Platform      string   // Optional: Filter by single platform
    Architecture  string   // Optional: Filter by architecture
    Version       string   // Optional: Filter by specific version
    Required      *bool    // Optional: Filter by required status
    Limit         int      // Optional: Maximum results (default: 50)
    Offset        int      // Optional: Pagination offset
    SortBy        string   // Optional: Field to sort by (default: release_date)
    SortOrder     string   // Optional: Sort direction (default: desc)
    Platforms     []string // Optional: Filter by multiple platforms
}
```

**Purpose**: Comprehensive release querying with pagination
**Features**: Flexible filtering, sorting, and pagination support

### Administrative Request Types

#### RegisterReleaseRequest
```go
type RegisterReleaseRequest struct {
    ApplicationID  string            // Target application
    Version        string            // Release version (semantic)
    Platform       string            // Target platform
    Architecture   string            // Target architecture
    DownloadURL    string            // External download location
    Checksum       string            // File integrity hash
    ChecksumType   string            // Hash algorithm
    FileSize       int64             // File size in bytes
    ReleaseNotes   string            // Change description
    Required       bool              // Force update flag
    MinimumVersion string            // Required current version
    Metadata       map[string]string // Additional metadata
}
```

**Security**: Admin-only operation requiring authentication
**Validation**: Comprehensive validation of all security-critical fields

#### CreateApplicationRequest
```go
type CreateApplicationRequest struct {
    ID          string            // Unique application identifier
    Name        string            // Human-readable name
    Description string            // Optional description
    Platforms   []string          // Supported platforms
    Config      ApplicationConfig // Application configuration
}
```

**Purpose**: Register new applications in the system
**Requirements**: Admin authentication, unique ID validation

### Request Validation System

#### Validation Interface
All request types implement comprehensive validation:

```go
func (r *RequestType) Validate() error
func (r *RequestType) Normalize()
```

#### Validation Examples

**Input Sanitization:**
```go
func (r *UpdateCheckRequest) Normalize() {
    r.Platform = NormalizePlatform(r.Platform)
    r.Architecture = NormalizeArchitecture(r.Architecture)
    r.ApplicationID = strings.TrimSpace(r.ApplicationID)
    r.CurrentVersion = strings.TrimSpace(r.CurrentVersion)
}
```

**Business Rule Validation:**
```go
func (r *ListReleasesRequest) Validate() error {
    if r.ApplicationID == "" {
        return errors.New("application_id is required")
    }

    if r.Limit < 0 {
        return errors.New("limit cannot be negative")
    }

    if r.SortOrder != "" && r.SortOrder != "asc" && r.SortOrder != "desc" {
        return errors.New("sort_order must be 'asc' or 'desc'")
    }

    return nil
}
```

## Response Models

### Primary Response Types

#### UpdateCheckResponse
```go
type UpdateCheckResponse struct {
    UpdateAvailable     bool              // Primary decision flag
    LatestVersion       string            // Available version (if update exists)
    CurrentVersion      string            // Client's current version (echoed)
    DownloadURL         string            // Download location (if update exists)
    Checksum            string            // File integrity hash
    ChecksumType        string            // Hash algorithm
    FileSize            int64             // File size for progress tracking
    ReleaseNotes        string            // Human-readable changes
    ReleaseDate         *time.Time        // Release timestamp
    Required            bool              // Critical update flag
    MinimumVersion      string            // Required current version
    Metadata            map[string]string // Extended metadata (optional)
    UpgradeInstructions string            // Custom upgrade steps
}
```

**Client Usage Pattern:**
1. Check `UpdateAvailable` first
2. Use `Required` flag to determine update urgency
3. Verify checksums before installation
4. Display `ReleaseNotes` to users for informed decisions

**Helper Methods:**
```go
func (r *UpdateCheckResponse) SetUpdateAvailable(release *Release)
func (r *UpdateCheckResponse) SetNoUpdateAvailable(currentVersion string)
```

#### LatestVersionResponse
```go
type LatestVersionResponse struct {
    Version      string            // Latest version available
    DownloadURL  string            // Download location
    Checksum     string            // File integrity hash
    ChecksumType string            // Hash algorithm
    FileSize     int64             // File size
    ReleaseNotes string            // Change description
    ReleaseDate  time.Time         // Release timestamp
    Required     bool              // Critical update flag
    Metadata     map[string]string // Extended metadata
}
```

#### ListReleasesResponse
```go
type ListReleasesResponse struct {
    Releases   []ReleaseInfo // Release information array
    TotalCount int           // Total available results
    Page       int           // Current page number
    PageSize   int           // Results per page
    HasMore    bool          // More results available flag
}
```

**Pagination Support:**
- Standard pagination metadata for client navigation
- Efficient handling of large release datasets
- Clear indication of additional data availability

### Error Response System

#### ErrorResponse Structure
```go
type ErrorResponse struct {
    Error     string            // Error type (always "error")
    Message   string            // Human-readable error description
    Code      string            // Machine-readable error code
    Details   map[string]string // Field-specific error details
    Timestamp time.Time         // Error occurrence time
    RequestID string            // Unique request identifier
}
```

#### Standard Error Codes
```go
const (
    ErrorCodeNotFound           = "NOT_FOUND"           // 404
    ErrorCodeBadRequest         = "BAD_REQUEST"         // 400
    ErrorCodeValidation         = "VALIDATION_ERROR"    // 422
    ErrorCodeInternalError      = "INTERNAL_ERROR"      // 500
    ErrorCodeUnauthorized       = "UNAUTHORIZED"        // 401
    ErrorCodeForbidden          = "FORBIDDEN"           // 403
    ErrorCodeConflict          = "CONFLICT"             // 409
    ErrorCodeServiceUnavailable = "SERVICE_UNAVAILABLE" // 503
)
```

#### Error Response Factory
```go
func NewErrorResponse(message string, code string) *ErrorResponse {
    return &ErrorResponse{
        Error:     "error",
        Message:   message,
        Code:      code,
        Timestamp: time.Now(),
    }
}

func NewValidationErrorResponse(errors map[string]string) *ValidationErrorResponse {
    return &ValidationErrorResponse{
        Error:  "validation_error",
        Errors: errors,
    }
}
```

### Health and Monitoring Responses

#### HealthCheckResponse
```go
type HealthCheckResponse struct {
    Status     string                       // Overall health status
    Timestamp  time.Time                    // Check timestamp
    Version    string                       // Service version
    Uptime     string                       // Service uptime
    Components map[string]ComponentHealth   // Component-specific health
    Metrics    map[string]interface{}       // Performance metrics
}
```

**Health Status Constants:**
```go
const (
    StatusHealthy   = "healthy"   // All systems operational
    StatusUnhealthy = "unhealthy" // Major system issues
    StatusDegraded  = "degraded"  // Partial functionality
    StatusUnknown   = "unknown"   // Status indeterminate
)
```

## Common Usage Patterns

### Request Processing Pipeline

```go
func ProcessUpdateCheckRequest(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    var req UpdateCheckRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeErrorResponse(w, "Invalid JSON", ErrorCodeBadRequest)
        return
    }

    // 2. Normalize input
    req.Normalize()

    // 3. Validate request
    if err := req.Validate(); err != nil {
        writeErrorResponse(w, err.Error(), ErrorCodeValidation)
        return
    }

    // 4. Process business logic
    response, err := checkForUpdate(&req)
    if err != nil {
        writeErrorResponse(w, "Update check failed", ErrorCodeInternalError)
        return
    }

    // 5. Send response
    writeJSONResponse(w, response)
}
```

### Response Construction

```go
func CheckForUpdate(req *UpdateCheckRequest) (*UpdateCheckResponse, error) {
    response := &UpdateCheckResponse{
        CurrentVersion: req.CurrentVersion,
    }

    // Find latest release
    release, err := findLatestRelease(req)
    if err != nil {
        return nil, err
    }

    if release == nil {
        response.SetNoUpdateAvailable(req.CurrentVersion)
        return response, nil
    }

    // Check version comparison
    isNewer, err := isReleaseNewer(release, req.CurrentVersion)
    if err != nil {
        return nil, err
    }

    if isNewer {
        response.SetUpdateAvailable(release)
    } else {
        response.SetNoUpdateAvailable(req.CurrentVersion)
    }

    return response, nil
}
```

### Error Handling Patterns

```go
func HandleValidationErrors(err error) *ErrorResponse {
    if validationErr, ok := err.(*ValidationError); ok {
        details := make(map[string]string)
        for field, message := range validationErr.FieldErrors {
            details[field] = message
        }

        return &ErrorResponse{
            Error:     "validation_error",
            Message:   "Invalid input data",
            Code:      ErrorCodeValidation,
            Details:   details,
            Timestamp: time.Now(),
        }
    }

    return NewErrorResponse(err.Error(), ErrorCodeInternalError)
}
```

## Client Integration Examples

### HTTP Client Usage

```go
type UpdaterClient struct {
    baseURL    string
    httpClient *http.Client
}

func (c *UpdaterClient) CheckForUpdate(req *UpdateCheckRequest) (*UpdateCheckResponse, error) {
    // Validate request locally
    if err := req.Validate(); err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }

    // Make HTTP request
    url := fmt.Sprintf("%s/api/v1/updates/%s/check", c.baseURL, req.ApplicationID)
    body, err := json.Marshal(req)
    if err != nil {
        return nil, err
    }

    resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // Handle response
    if resp.StatusCode != http.StatusOK {
        var errResp ErrorResponse
        json.NewDecoder(resp.Body).Decode(&errResp)
        return nil, fmt.Errorf("API error: %s", errResp.Message)
    }

    var updateResp UpdateCheckResponse
    if err := json.NewDecoder(resp.Body).Decode(&updateResp); err != nil {
        return nil, err
    }

    return &updateResp, nil
}
```

### JavaScript/TypeScript Integration

```typescript
interface UpdateCheckRequest {
    applicationId: string;
    currentVersion: string;
    platform: string;
    architecture: string;
    allowPrerelease?: boolean;
    includeMetadata?: boolean;
}

interface UpdateCheckResponse {
    updateAvailable: boolean;
    latestVersion?: string;
    currentVersion: string;
    downloadUrl?: string;
    checksum?: string;
    checksumType?: string;
    fileSize?: number;
    releaseNotes?: string;
    required: boolean;
}

class UpdaterClient {
    constructor(private baseUrl: string) {}

    async checkForUpdate(request: UpdateCheckRequest): Promise<UpdateCheckResponse> {
        const response = await fetch(`${this.baseUrl}/api/v1/updates/${request.applicationId}/check`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(request)
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(`Update check failed: ${error.message}`);
        }

        return response.json();
    }
}
```

## Performance Optimizations

### Response Size Management
- **Conditional Metadata**: Include metadata only when `IncludeMetadata` is true
- **Omitempty Tags**: Reduce JSON payload size for optional fields
- **Pagination**: Efficient handling of large result sets

### Request Validation Optimization
- **Early Validation**: Fail fast for obviously invalid requests
- **Cached Validation**: Cache validation results for repeated patterns
- **Batch Validation**: Validate multiple fields efficiently

### Serialization Performance
- **Efficient JSON Encoding**: Optimized struct tags and field ordering
- **String Interning**: Reuse common strings (platform names, error codes)
- **Memory Pooling**: Reuse response objects for high-traffic endpoints

## Security Considerations

### Input Validation Security
- **SQL Injection Prevention**: Parameterized queries, input sanitization
- **XSS Prevention**: Proper JSON encoding, no HTML content
- **Path Traversal Prevention**: Validate application IDs against patterns
- **Size Limits**: Prevent large payloads from causing memory exhaustion

### Response Information Disclosure
- **Error Message Sanitization**: Avoid leaking internal system details
- **Sensitive Data Filtering**: Never include authentication tokens or secrets
- **Rate Limiting**: Prevent information gathering through repeated requests

### Authentication Integration
- **API Key Validation**: Secure authentication for admin operations
- **Request Signing**: Optional cryptographic request verification
- **CORS Configuration**: Proper cross-origin request handling

## Testing Strategies

### Request Validation Testing

```go
func TestUpdateCheckRequestValidation(t *testing.T) {
    tests := []struct {
        name     string
        request  UpdateCheckRequest
        hasError bool
    }{
        {
            name: "valid request",
            request: UpdateCheckRequest{
                ApplicationID:  "myapp",
                CurrentVersion: "1.0.0",
                Platform:       "windows",
                Architecture:   "amd64",
            },
            hasError: false,
        },
        {
            name: "missing application ID",
            request: UpdateCheckRequest{
                CurrentVersion: "1.0.0",
                Platform:       "windows",
                Architecture:   "amd64",
            },
            hasError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.request.Validate()
            if (err != nil) != tt.hasError {
                t.Errorf("Validate() error = %v, hasError = %v", err, tt.hasError)
            }
        })
    }
}
```

### Response Serialization Testing

```go
func TestResponseSerialization(t *testing.T) {
    response := &UpdateCheckResponse{
        UpdateAvailable: true,
        LatestVersion:   "1.1.0",
        CurrentVersion:  "1.0.0",
        DownloadURL:     "https://example.com/file.exe",
        Required:        false,
    }

    data, err := json.Marshal(response)
    if err != nil {
        t.Fatalf("Failed to marshal response: %v", err)
    }

    var unmarshaled UpdateCheckResponse
    if err := json.Unmarshal(data, &unmarshaled); err != nil {
        t.Fatalf("Failed to unmarshal response: %v", err)
    }

    if response.UpdateAvailable != unmarshaled.UpdateAvailable {
        t.Error("UpdateAvailable field not preserved")
    }
}
```

### Integration Testing
- **End-to-End API Testing**: Complete request/response cycles
- **Error Handling Testing**: Verify proper error responses
- **Performance Testing**: Load testing with realistic request patterns
- **Security Testing**: Input fuzzing, malformed request handling

This API model system provides a robust, secure, and performant foundation for client-server communication in the updater service while maintaining consistency and ease of use.