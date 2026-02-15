// Package models - API response types and error handling.
// This file defines all outgoing API response structures with consistent formatting.
//
// Response Design Principles:
// - Consistent JSON structure across all endpoints
// - Optional fields use omitempty to reduce response size
// - Rich error information with codes and details for debugging
// - Standardized pagination with metadata
// - Helper methods for easy response construction
// - RFC3339 timestamps for international compatibility
package models

import (
	"time"
)

// UpdateCheckResponse provides complete information about available updates.
//
// Response Strategy:
// - UpdateAvailable is the primary decision field for clients
// - All download information is provided when updates are available
// - CurrentVersion echoes the request for client verification
// - Required flag indicates critical security updates
// - Metadata is optional to control response size
// - UpgradeInstructions support complex update workflows
//
// Client Usage:
// - Check UpdateAvailable first
// - Use Required flag to determine update urgency
// - Verify checksums before installation
// - Display ReleaseNotes to users for informed decisions
type UpdateCheckResponse struct {
	UpdateAvailable     bool              `json:"update_available"`               // Primary decision flag
	LatestVersion       string            `json:"latest_version,omitempty"`       // Available version (if update exists)
	CurrentVersion      string            `json:"current_version"`                // Client's current version (echoed)
	DownloadURL         string            `json:"download_url,omitempty"`         // Download location (if update exists)
	Checksum            string            `json:"checksum,omitempty"`             // File integrity hash
	ChecksumType        string            `json:"checksum_type,omitempty"`        // Hash algorithm
	FileSize            int64             `json:"file_size,omitempty"`            // File size for progress tracking
	ReleaseNotes        string            `json:"release_notes,omitempty"`        // Human-readable changes
	ReleaseDate         *time.Time        `json:"release_date,omitempty"`         // Release timestamp
	Required            bool              `json:"required"`                       // Critical update flag
	MinimumVersion      string            `json:"minimum_version,omitempty"`      // Required current version
	Metadata            map[string]string `json:"metadata,omitempty"`             // Extended metadata (optional)
	UpgradeInstructions string            `json:"upgrade_instructions,omitempty"` // Custom upgrade steps
}

type LatestVersionResponse struct {
	Version      string            `json:"version"`
	DownloadURL  string            `json:"download_url"`
	Checksum     string            `json:"checksum"`
	ChecksumType string            `json:"checksum_type"`
	FileSize     int64             `json:"file_size"`
	ReleaseNotes string            `json:"release_notes"`
	ReleaseDate  time.Time         `json:"release_date"`
	Required     bool              `json:"required"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type ListReleasesResponse struct {
	Releases   []ReleaseInfo `json:"releases"`
	TotalCount int           `json:"total_count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	HasMore    bool          `json:"has_more"`
}

type ReleaseInfo struct {
	ID             string            `json:"id"`
	Version        string            `json:"version"`
	Platform       string            `json:"platform"`
	Architecture   string            `json:"architecture"`
	DownloadURL    string            `json:"download_url"`
	Checksum       string            `json:"checksum"`
	ChecksumType   string            `json:"checksum_type"`
	FileSize       int64             `json:"file_size"`
	ReleaseNotes   string            `json:"release_notes"`
	ReleaseDate    time.Time         `json:"release_date"`
	Required       bool              `json:"required"`
	MinimumVersion string            `json:"minimum_version,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type RegisterReleaseResponse struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateApplicationResponse struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type UpdateApplicationResponse struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DeleteReleaseResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// ErrorResponse provides structured error information with debugging context.
//
// Error Handling Design:
// - Consistent error structure across all endpoints
// - Machine-readable error codes for programmatic handling
// - Human-readable messages for user interfaces
// - Details map for field-specific validation errors
// - Request ID for distributed tracing and support
// - Timestamps for debugging and audit trails
//
// Error Categories:
// - Validation errors: Input format/constraint violations
// - Not found errors: Resource doesn't exist
// - Authorization errors: Authentication/permission failures
// - Internal errors: Server-side issues
type ErrorResponse struct {
	Error     string            `json:"error"`                // Error type (always "error")
	Message   string            `json:"message"`              // Human-readable error description
	Code      string            `json:"code,omitempty"`       // Machine-readable error code
	Details   map[string]string `json:"details,omitempty"`    // Field-specific error details
	Timestamp time.Time         `json:"timestamp"`            // Error occurrence time
	RequestID string            `json:"request_id,omitempty"` // Unique request identifier
}

type HealthCheckResponse struct {
	Status     string                     `json:"status"`
	Timestamp  time.Time                  `json:"timestamp"`
	Version    string                     `json:"version,omitempty"`
	Uptime     string                     `json:"uptime,omitempty"`
	Components map[string]ComponentHealth `json:"components,omitempty"`
	Metrics    map[string]interface{}     `json:"metrics,omitempty"`
}

type ComponentHealth struct {
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

type ApplicationInfoResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Platforms   []string          `json:"platforms"`
	Config      ApplicationConfig `json:"config"`
	Stats       ApplicationStats  `json:"stats"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type ApplicationStats struct {
	TotalReleases     int        `json:"total_releases"`
	LatestVersion     string     `json:"latest_version,omitempty"`
	LatestReleaseDate *time.Time `json:"latest_release_date,omitempty"`
	PlatformCount     int        `json:"platform_count"`
	RequiredReleases  int        `json:"required_releases"`
}

type ListApplicationsResponse struct {
	Applications []ApplicationSummary `json:"applications"`
	TotalCount   int                  `json:"total_count"`
	Page         int                  `json:"page"`
	PageSize     int                  `json:"page_size"`
	HasMore      bool                 `json:"has_more"`
}

type ApplicationSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Platforms   []string  `json:"platforms"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type StatsResponse struct {
	TotalApplications int                    `json:"total_applications"`
	TotalReleases     int                    `json:"total_releases"`
	PlatformStats     map[string]int         `json:"platform_stats"`
	VersionStats      map[string]int         `json:"version_stats"`
	RecentActivity    []ActivityItem         `json:"recent_activity"`
	SystemInfo        map[string]interface{} `json:"system_info"`
}

type ActivityItem struct {
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Timestamp   time.Time         `json:"timestamp"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type ValidationErrorResponse struct {
	Error  string            `json:"error"`
	Errors map[string]string `json:"errors"`
}

// Health Status Constants
//
// Health Monitoring:
// - Healthy: All systems operational
// - Degraded: Partial functionality (some features may be slow/limited)
// - Unhealthy: Major issues affecting core functionality
// - Unknown: Health status cannot be determined
const (
	StatusHealthy   = "healthy"   // All systems operational
	StatusUnhealthy = "unhealthy" // Major system issues
	StatusDegraded  = "degraded"  // Partial functionality
	StatusUnknown   = "unknown"   // Status indeterminate
)

// Standard HTTP Error Codes
//
// Error Code Strategy:
// - Upper-case with underscores for consistency
// - Maps to standard HTTP status codes
// - Machine-readable for client error handling
// - Extensible for service-specific errors
const (
	ErrorCodeNotFound            = "NOT_FOUND"            // 404: Resource doesn't exist
	ErrorCodeApplicationNotFound = "APPLICATION_NOT_FOUND" // 404: Application doesn't exist
	ErrorCodeBadRequest          = "BAD_REQUEST"          // 400: Invalid request format
	ErrorCodeInvalidRequest      = "INVALID_REQUEST"      // 400: Invalid request data
	ErrorCodeValidation          = "VALIDATION_ERROR"     // 422: Input validation failed
	ErrorCodeInternalError       = "INTERNAL_ERROR"       // 500: Server-side error
	ErrorCodeUnauthorized        = "UNAUTHORIZED"         // 401: Authentication required
	ErrorCodeForbidden           = "FORBIDDEN"            // 403: Permission denied
	ErrorCodeConflict            = "CONFLICT"             // 409: Resource conflict
	ErrorCodeServiceUnavailable  = "SERVICE_UNAVAILABLE"  // 503: Service temporarily down
)

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

func (r *UpdateCheckResponse) SetUpdateAvailable(release *Release) {
	r.UpdateAvailable = true
	r.LatestVersion = release.Version
	r.DownloadURL = release.DownloadURL
	r.Checksum = release.Checksum
	r.ChecksumType = release.ChecksumType
	r.FileSize = release.FileSize
	r.ReleaseNotes = release.ReleaseNotes
	r.ReleaseDate = &release.ReleaseDate
	r.Required = release.Required
	r.MinimumVersion = release.MinimumVersion
	r.Metadata = release.Metadata
}

func (r *UpdateCheckResponse) SetNoUpdateAvailable(currentVersion string) {
	r.UpdateAvailable = false
	r.CurrentVersion = currentVersion
}

func (r *LatestVersionResponse) FromRelease(release *Release) {
	r.Version = release.Version
	r.DownloadURL = release.DownloadURL
	r.Checksum = release.Checksum
	r.ChecksumType = release.ChecksumType
	r.FileSize = release.FileSize
	r.ReleaseNotes = release.ReleaseNotes
	r.ReleaseDate = release.ReleaseDate
	r.Required = release.Required
	r.Metadata = release.Metadata
}

func (ri *ReleaseInfo) FromRelease(release *Release) {
	ri.ID = release.ID
	ri.Version = release.Version
	ri.Platform = release.Platform
	ri.Architecture = release.Architecture
	ri.DownloadURL = release.DownloadURL
	ri.Checksum = release.Checksum
	ri.ChecksumType = release.ChecksumType
	ri.FileSize = release.FileSize
	ri.ReleaseNotes = release.ReleaseNotes
	ri.ReleaseDate = release.ReleaseDate
	ri.Required = release.Required
	ri.MinimumVersion = release.MinimumVersion
	ri.Metadata = release.Metadata
}

func (as *ApplicationSummary) FromApplication(app *Application) {
	as.ID = app.ID
	as.Name = app.Name
	as.Description = app.Description
	as.Platforms = app.Platforms
}

func NewHealthCheckResponse(status string) *HealthCheckResponse {
	return &HealthCheckResponse{
		Status:     status,
		Timestamp:  time.Now(),
		Components: make(map[string]ComponentHealth),
		Metrics:    make(map[string]interface{}),
	}
}

func (h *HealthCheckResponse) AddComponent(name, status, message string) {
	h.Components[name] = ComponentHealth{
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}
}

func (h *HealthCheckResponse) AddMetric(name string, value interface{}) {
	h.Metrics[name] = value
}
