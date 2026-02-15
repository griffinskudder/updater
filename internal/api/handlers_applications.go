package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"updater/internal/models"

	"github.com/gorilla/mux"
)

// CreateApplication handles application creation requests
// POST /api/v1/applications
// Requires authentication and 'write' permission
func (h *Handlers) CreateApplication(w http.ResponseWriter, r *http.Request) {
	// Get security context for audit logging
	securityContext := GetSecurityContext(r)

	// Log the admin operation attempt
	slog.Warn("Application creation attempt",
		"event", "security_audit",
		"api_key", getAPIKeyName(securityContext),
		"client_ip", getClientIP(r))

	// Validate content-type
	contentType := r.Header.Get("Content-Type")
	if contentType == "" || !strings.HasPrefix(contentType, "application/json") {
		h.writeErrorResponse(w, http.StatusUnsupportedMediaType, models.ErrorCodeBadRequest, "Content-Type must be application/json")
		return
	}

	// Parse request body
	var req models.CreateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("Invalid JSON in application creation",
			"event", "security_audit",
			"api_key", getAPIKeyName(securityContext))
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeInvalidRequest, "Invalid JSON body")
		return
	}

	// Create application
	response, err := h.updateService.CreateApplication(r.Context(), &req)
	if err != nil {
		slog.Warn("Application creation failed",
			"event", "security_audit",
			"app_id", req.ID,
			"api_key", getAPIKeyName(securityContext),
			"error", err.Error())
		h.writeServiceErrorResponse(w, err)
		return
	}

	// Log successful creation
	slog.Info("Application created successfully",
		"event", "security_audit",
		"app_id", req.ID,
		"api_key", getAPIKeyName(securityContext))

	h.writeJSONResponse(w, http.StatusCreated, response)
}

// GetApplication handles application retrieval requests
// GET /api/v1/applications/{app_id}
func (h *Handlers) GetApplication(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]

	// Get application
	response, err := h.updateService.GetApplication(r.Context(), appID)
	if err != nil {
		h.writeServiceErrorResponse(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// ListApplications handles application listing requests
// GET /api/v1/applications
func (h *Handlers) ListApplications(w http.ResponseWriter, r *http.Request) {
	// Parse limit and offset with defaults
	limit := 50
	offset := 0

	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		if parsed, err := strconv.Atoi(limitParam); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if offsetParam := r.URL.Query().Get("offset"); offsetParam != "" {
		if parsed, err := strconv.Atoi(offsetParam); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// List applications
	response, err := h.updateService.ListApplications(r.Context(), limit, offset)
	if err != nil {
		h.writeServiceErrorResponse(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// UpdateApplication handles application update requests
// PUT /api/v1/applications/{app_id}
// Requires authentication and 'admin' permission
func (h *Handlers) UpdateApplication(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]

	// Get security context for audit logging
	securityContext := GetSecurityContext(r)

	// Log the admin operation attempt
	slog.Warn("Application update attempt",
		"event", "security_audit",
		"app_id", appID,
		"api_key", getAPIKeyName(securityContext),
		"client_ip", getClientIP(r))

	// Validate content-type
	contentType := r.Header.Get("Content-Type")
	if contentType == "" || !strings.HasPrefix(contentType, "application/json") {
		h.writeErrorResponse(w, http.StatusUnsupportedMediaType, models.ErrorCodeBadRequest, "Content-Type must be application/json")
		return
	}

	// Parse request body
	var req models.UpdateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("Invalid JSON in application update",
			"event", "security_audit",
			"app_id", appID,
			"api_key", getAPIKeyName(securityContext))
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeInvalidRequest, "Invalid JSON body")
		return
	}

	// Update application
	response, err := h.updateService.UpdateApplication(r.Context(), appID, &req)
	if err != nil {
		slog.Warn("Application update failed",
			"event", "security_audit",
			"app_id", appID,
			"api_key", getAPIKeyName(securityContext),
			"error", err.Error())
		h.writeServiceErrorResponse(w, err)
		return
	}

	// Log successful update
	slog.Info("Application updated successfully",
		"event", "security_audit",
		"app_id", appID,
		"api_key", getAPIKeyName(securityContext))

	h.writeJSONResponse(w, http.StatusOK, response)
}

// DeleteApplication handles application deletion requests
// DELETE /api/v1/applications/{app_id}
// Requires authentication and 'admin' permission
func (h *Handlers) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]

	// Get security context for audit logging
	securityContext := GetSecurityContext(r)

	// Log the admin operation attempt
	slog.Warn("Application deletion attempt",
		"event", "security_audit",
		"app_id", appID,
		"api_key", getAPIKeyName(securityContext),
		"client_ip", getClientIP(r))

	// Delete application
	err := h.updateService.DeleteApplication(r.Context(), appID)
	if err != nil {
		slog.Warn("Application deletion failed",
			"event", "security_audit",
			"app_id", appID,
			"api_key", getAPIKeyName(securityContext),
			"error", err.Error())
		h.writeServiceErrorResponse(w, err)
		return
	}

	// Log successful deletion
	slog.Info("Application deleted successfully",
		"event", "security_audit",
		"app_id", appID,
		"api_key", getAPIKeyName(securityContext))

	w.WriteHeader(http.StatusNoContent)
}

// DeleteRelease handles release deletion requests
// DELETE /api/v1/updates/{app_id}/releases/{version}/{platform}/{arch}
// Requires authentication and 'admin' permission
func (h *Handlers) DeleteRelease(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]
	version := vars["version"]
	platform := vars["platform"]
	arch := vars["arch"]

	// Get security context for audit logging
	securityContext := GetSecurityContext(r)

	// Log the admin operation attempt
	slog.Warn("Release deletion attempt",
		"event", "security_audit",
		"app_id", appID,
		"version", version,
		"platform", platform,
		"arch", arch,
		"api_key", getAPIKeyName(securityContext),
		"client_ip", getClientIP(r))

	// Delete release
	response, err := h.updateService.DeleteRelease(r.Context(), appID, version, platform, arch)
	if err != nil {
		slog.Warn("Release deletion failed",
			"event", "security_audit",
			"app_id", appID,
			"version", version,
			"platform", platform,
			"arch", arch,
			"api_key", getAPIKeyName(securityContext),
			"error", err.Error())
		h.writeServiceErrorResponse(w, err)
		return
	}

	// Log successful deletion
	slog.Info("Release deleted successfully",
		"event", "security_audit",
		"app_id", appID,
		"version", version,
		"platform", platform,
		"arch", arch,
		"api_key", getAPIKeyName(securityContext))

	h.writeJSONResponse(w, http.StatusOK, response)
}
