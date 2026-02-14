package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"updater/internal/models"
	"updater/internal/update"

	"github.com/gorilla/mux"
)

// Handlers contains HTTP handlers for the updater API
type Handlers struct {
	updateService update.ServiceInterface
}

// NewHandlers creates a new handlers instance
func NewHandlers(updateService update.ServiceInterface) *Handlers {
	return &Handlers{
		updateService: updateService,
	}
}

// CheckForUpdates handles update check requests
// GET /api/v1/updates/{app_id}/check
func (h *Handlers) CheckForUpdates(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]

	// Parse query parameters
	req := &models.UpdateCheckRequest{
		ApplicationID:   appID,
		CurrentVersion:  r.URL.Query().Get("current_version"),
		Platform:        r.URL.Query().Get("platform"),
		Architecture:    r.URL.Query().Get("architecture"),
		AllowPrerelease: r.URL.Query().Get("allow_prerelease") == "true",
		IncludeMetadata: r.URL.Query().Get("include_metadata") == "true",
		UserAgent:       r.Header.Get("User-Agent"),
		ClientID:        r.URL.Query().Get("client_id"),
	}

	// Check for updates
	response, err := h.updateService.CheckForUpdate(r.Context(), req)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeBadRequest, err.Error())
		return
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// GetLatestVersion handles latest version requests
// GET /api/v1/updates/{app_id}/latest
func (h *Handlers) GetLatestVersion(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]

	// Parse query parameters
	req := &models.LatestVersionRequest{
		ApplicationID:   appID,
		Platform:        r.URL.Query().Get("platform"),
		Architecture:    r.URL.Query().Get("architecture"),
		AllowPrerelease: r.URL.Query().Get("allow_prerelease") == "true",
		IncludeMetadata: r.URL.Query().Get("include_metadata") == "true",
	}

	// Get latest version
	response, err := h.updateService.GetLatestVersion(r.Context(), req)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeBadRequest, err.Error())
		return
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// ListReleases handles release list requests
// GET /api/v1/updates/{app_id}/releases
func (h *Handlers) ListReleases(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]

	// Parse query parameters
	req := &models.ListReleasesRequest{
		ApplicationID: appID,
		Platform:      r.URL.Query().Get("platform"),
		Architecture:  r.URL.Query().Get("architecture"),
		Version:       r.URL.Query().Get("version"),
		SortBy:        r.URL.Query().Get("sort_by"),
		SortOrder:     r.URL.Query().Get("sort_order"),
	}

	// Parse required filter
	if requiredParam := r.URL.Query().Get("required"); requiredParam != "" {
		if required, err := strconv.ParseBool(requiredParam); err == nil {
			req.Required = &required
		}
	}

	// Parse limit and offset
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		if limit, err := strconv.Atoi(limitParam); err == nil {
			req.Limit = limit
		}
	}

	if offsetParam := r.URL.Query().Get("offset"); offsetParam != "" {
		if offset, err := strconv.Atoi(offsetParam); err == nil {
			req.Offset = offset
		}
	}

	// Parse platforms array
	if platforms := r.URL.Query().Get("platforms"); platforms != "" {
		// Simple comma-separated parsing
		req.Platforms = splitAndTrim(platforms, ",")
	}

	// List releases
	response, err := h.updateService.ListReleases(r.Context(), req)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeBadRequest, err.Error())
		return
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// RegisterRelease handles release registration requests
// POST /api/v1/updates/{app_id}/register
// Requires authentication and 'write' permission
func (h *Handlers) RegisterRelease(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]

	// Get security context for audit logging
	securityContext := GetSecurityContext(r)

	// Log the admin operation attempt
	fmt.Printf("SECURITY: Release registration attempt for app %s by API key: %s, IP: %s\n",
		appID,
		getAPIKeyName(securityContext),
		getClientIP(r))

	// Parse request body
	var req models.RegisterReleaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("SECURITY: Invalid JSON in release registration for app %s by %s\n",
			appID, getAPIKeyName(securityContext))
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeBadRequest, "Invalid JSON body")
		return
	}

	// Set application ID from URL
	req.ApplicationID = appID

	// Register release
	response, err := h.updateService.RegisterRelease(r.Context(), &req)
	if err != nil {
		fmt.Printf("SECURITY: Release registration failed for app %s, version %s by %s: %s\n",
			appID, req.Version, getAPIKeyName(securityContext), err.Error())
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeBadRequest, err.Error())
		return
	}

	// Log successful registration
	fmt.Printf("SECURITY: Release registration successful for app %s, version %s by %s\n",
		appID, req.Version, getAPIKeyName(securityContext))

	h.writeJSONResponse(w, http.StatusCreated, response)
}

// HealthCheck handles health check requests
// GET /health
// Provides basic health info publicly, enhanced details with authentication
func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := models.NewHealthCheckResponse(models.StatusHealthy)
	response.Version = "1.0.0" // This could be injected from build info

	// Add basic health metrics
	response.AddComponent("storage", models.StatusHealthy, "Storage is operational")
	response.AddComponent("api", models.StatusHealthy, "API is operational")

	// Get security context to check if user is authenticated
	securityContext := GetSecurityContext(r)

	// If authenticated, add enhanced details
	if securityContext != nil && securityContext.HasPermission(PermissionRead) {
		// Add enhanced metrics for authenticated users
		if response.Metrics == nil {
			response.Metrics = make(map[string]interface{})
		}

		response.Metrics["authentication_enabled"] = true
		response.Metrics["api_key_name"] = getAPIKeyName(securityContext)
		response.Metrics["permissions"] = securityContext.APIKey.Permissions
		response.AddComponent("auth", models.StatusHealthy, "Authentication system operational")
	} else {
		// Add limited info for public access
		if response.Metrics == nil {
			response.Metrics = make(map[string]interface{})
		}
		response.Metrics["authentication_enabled"] = false
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// writeJSONResponse writes a JSON response
func (h *Handlers) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If we can't encode the response, log it but don't try to send another response
		// as headers have already been written
		fmt.Printf("Error encoding JSON response: %v\n", err)
	}
}

// writeErrorResponse writes an error response
func (h *Handlers) writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message string) {
	errorResp := models.NewErrorResponse(message, errorCode)

	// Request ID could be added here if middleware provides it
	// errorResp.RequestID = "some-request-id"

	h.writeJSONResponse(w, statusCode, errorResp)
}

// splitAndTrim splits a string by delimiter and trims whitespace
func splitAndTrim(s, delim string) []string {
	if s == "" {
		return nil
	}

	parts := make([]string, 0)
	for _, part := range strings.Split(s, delim) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// getAPIKeyName safely extracts the API key name for logging
func getAPIKeyName(securityContext *SecurityContext) string {
	if securityContext == nil || securityContext.APIKey == nil {
		return "anonymous"
	}
	if securityContext.APIKey.Name != "" {
		return securityContext.APIKey.Name
	}
	return "unnamed-key"
}