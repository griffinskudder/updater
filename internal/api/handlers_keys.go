package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
	"updater/internal/models"
	"updater/internal/storage"

	"github.com/gorilla/mux"
)

// createAPIKeyRequest is the request body for POST /api/v1/admin/keys.
type createAPIKeyRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// createAPIKeyResponse includes the raw key — returned exactly once.
type createAPIKeyResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Key         string    `json:"key"`
	Prefix      string    `json:"prefix"`
	Permissions []string  `json:"permissions"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// apiKeyResponse is the metadata-only view (no raw key, no hash).
type apiKeyResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Prefix      string    `json:"prefix"`
	Permissions []string  `json:"permissions"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// updateAPIKeyRequest is the request body for PATCH /api/v1/admin/keys/{id}.
// All fields are optional.
type updateAPIKeyRequest struct {
	Name        *string  `json:"name"`
	Permissions []string `json:"permissions"`
	Enabled     *bool    `json:"enabled"`
}

func apiKeyToResponse(k *models.APIKey) apiKeyResponse {
	return apiKeyResponse{
		ID:          k.ID,
		Name:        k.Name,
		Prefix:      k.Prefix,
		Permissions: k.Permissions,
		Enabled:     k.Enabled,
		CreatedAt:   k.CreatedAt,
		UpdatedAt:   k.UpdatedAt,
	}
}

// ListAPIKeys handles GET /api/v1/admin/keys
func (h *Handlers) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.storage.ListAPIKeys(r.Context())
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, models.ErrorCodeInternalError, "failed to list keys")
		return
	}
	resp := make([]apiKeyResponse, len(keys))
	for i, k := range keys {
		resp[i] = apiKeyToResponse(k)
	}
	h.writeJSONResponse(w, http.StatusOK, resp)
}

// CreateAPIKey handles POST /api/v1/admin/keys
func (h *Handlers) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeInvalidRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeInvalidRequest, "name is required")
		return
	}
	if len(req.Permissions) == 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeInvalidRequest, "permissions is required")
		return
	}

	rawKey, err := models.GenerateAPIKey()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, models.ErrorCodeInternalError, "failed to generate key")
		return
	}

	key := models.NewAPIKey(models.NewKeyID(), req.Name, rawKey, req.Permissions)
	if err := h.storage.CreateAPIKey(r.Context(), key); err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, models.ErrorCodeInternalError, "failed to create key")
		return
	}

	slog.Info("api key created",
		"event", "security_audit",
		"action", "create",
		"key_id", key.ID,
		"key_name", key.Name,
		"actor_key_id", actorKeyID(r),
	)

	h.writeJSONResponse(w, http.StatusCreated, createAPIKeyResponse{
		ID:          key.ID,
		Name:        key.Name,
		Key:         rawKey,
		Prefix:      key.Prefix,
		Permissions: key.Permissions,
		Enabled:     key.Enabled,
		CreatedAt:   key.CreatedAt,
	})
}

// UpdateAPIKey handles PATCH /api/v1/admin/keys/{id}
func (h *Handlers) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req updateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeInvalidRequest, "invalid request body")
		return
	}

	// Fetch existing key by scanning the list (no GetByID method).
	keys, err := h.storage.ListAPIKeys(r.Context())
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, models.ErrorCodeInternalError, "failed to fetch keys")
		return
	}
	var key *models.APIKey
	for _, k := range keys {
		if k.ID == id {
			c := *k
			key = &c
			break
		}
	}
	if key == nil {
		h.writeErrorResponse(w, http.StatusNotFound, models.ErrorCodeNotFound, "key not found")
		return
	}

	if req.Name != nil {
		key.Name = *req.Name
	}
	if req.Permissions != nil {
		key.Permissions = req.Permissions
	}
	if req.Enabled != nil {
		key.Enabled = *req.Enabled
	}
	key.UpdatedAt = time.Now().UTC()

	if err := h.storage.UpdateAPIKey(r.Context(), key); err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, models.ErrorCodeInternalError, "failed to update key")
		return
	}

	slog.Info("api key updated",
		"event", "security_audit",
		"action", "update",
		"key_id", key.ID,
		"key_name", key.Name,
		"actor_key_id", actorKeyID(r),
	)

	h.writeJSONResponse(w, http.StatusOK, apiKeyToResponse(key))
}

// DeleteAPIKey handles DELETE /api/v1/admin/keys/{id}
func (h *Handlers) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.storage.DeleteAPIKey(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			h.writeErrorResponse(w, http.StatusNotFound, models.ErrorCodeNotFound, "key not found")
		} else {
			h.writeErrorResponse(w, http.StatusInternalServerError, models.ErrorCodeInternalError, "failed to delete key")
		}
		return
	}

	slog.Info("api key deleted",
		"event", "security_audit",
		"action", "delete",
		"key_id", id,
		"actor_key_id", actorKeyID(r),
	)

	w.WriteHeader(http.StatusNoContent)
}

// actorKeyID extracts the ID of the authenticated key making this request.
func actorKeyID(r *http.Request) string {
	if k, ok := r.Context().Value("api_key").(*models.APIKey); ok {
		return k.ID
	}
	return "unknown"
}
