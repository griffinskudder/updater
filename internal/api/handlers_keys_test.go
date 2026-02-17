package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"updater/internal/models"
	"updater/internal/storage"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newKeyTestHandlers creates a Handlers instance with a MemoryStorage pre-seeded with one admin key.
// It returns the handlers and the raw admin key string for use in Authorization headers.
func newKeyTestHandlers(t *testing.T) (*Handlers, string) {
	t.Helper()
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)

	adminRaw := "upd_test-admin-key-for-handlers"
	adminKey := models.NewAPIKey(models.NewKeyID(), "admin", adminRaw, []string{"admin"})
	require.NoError(t, store.CreateAPIKey(context.Background(), adminKey))

	h := NewHandlers(&MockUpdateService{}, WithStorage(store))
	return h, adminRaw
}

// adminCtxRequest creates a request with the admin APIKey already in the context
// (simulating what authMiddleware would do).
func adminCtxRequest(method, path string, body []byte, store storage.Storage, rawKey string) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	hash := models.HashAPIKey(rawKey)
	ak, _ := store.GetAPIKeyByHash(context.Background(), hash)
	ctx := context.WithValue(req.Context(), "api_key", ak)
	return req.WithContext(ctx)
}

func TestListAPIKeys_ReturnsEmptyList(t *testing.T) {
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	h := NewHandlers(&MockUpdateService{}, WithStorage(store))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/keys", nil)
	rr := httptest.NewRecorder()
	h.ListAPIKeys(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp []apiKeyResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp)
}

func TestListAPIKeys_ReturnsKeys(t *testing.T) {
	h, adminRaw := newKeyTestHandlers(t)

	req := adminCtxRequest(http.MethodGet, "/api/v1/admin/keys", nil, h.storage, adminRaw)
	rr := httptest.NewRecorder()
	h.ListAPIKeys(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp []apiKeyResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, "admin", resp[0].Name)
	assert.Empty(t, resp[0].ID == "", "ID must be set")
}

func TestCreateAPIKey_ValidRequest_Returns201(t *testing.T) {
	h, adminRaw := newKeyTestHandlers(t)

	body, _ := json.Marshal(createAPIKeyRequest{Name: "CI Publisher", Permissions: []string{"write"}})
	req := adminCtxRequest(http.MethodPost, "/api/v1/admin/keys", body, h.storage, adminRaw)
	rr := httptest.NewRecorder()
	h.CreateAPIKey(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp createAPIKeyResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "CI Publisher", resp.Name)
	assert.NotEmpty(t, resp.Key, "raw key must be present in creation response")
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, []string{"write"}, resp.Permissions)
}

func TestCreateAPIKey_MissingName_Returns400(t *testing.T) {
	h, adminRaw := newKeyTestHandlers(t)

	body, _ := json.Marshal(createAPIKeyRequest{Permissions: []string{"read"}})
	req := adminCtxRequest(http.MethodPost, "/api/v1/admin/keys", body, h.storage, adminRaw)
	rr := httptest.NewRecorder()
	h.CreateAPIKey(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateAPIKey_MissingPermissions_Returns400(t *testing.T) {
	h, adminRaw := newKeyTestHandlers(t)

	body, _ := json.Marshal(createAPIKeyRequest{Name: "Test"})
	req := adminCtxRequest(http.MethodPost, "/api/v1/admin/keys", body, h.storage, adminRaw)
	rr := httptest.NewRecorder()
	h.CreateAPIKey(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUpdateAPIKey_ValidRequest_Returns200(t *testing.T) {
	h, adminRaw := newKeyTestHandlers(t)

	// List to get the admin key's ID
	keys, err := h.storage.ListAPIKeys(context.Background())
	require.NoError(t, err)
	require.Len(t, keys, 1)
	id := keys[0].ID

	newName := "renamed-admin"
	body, _ := json.Marshal(updateAPIKeyRequest{Name: &newName})
	req := adminCtxRequest(http.MethodPatch, "/api/v1/admin/keys/"+id, body, h.storage, adminRaw)
	req = mux.SetURLVars(req, map[string]string{"id": id})
	rr := httptest.NewRecorder()
	h.UpdateAPIKey(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp apiKeyResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "renamed-admin", resp.Name)
}

func TestUpdateAPIKey_NotFound_Returns404(t *testing.T) {
	h, adminRaw := newKeyTestHandlers(t)

	newName := "x"
	body, _ := json.Marshal(updateAPIKeyRequest{Name: &newName})
	req := adminCtxRequest(http.MethodPatch, "/api/v1/admin/keys/nonexistent", body, h.storage, adminRaw)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})
	rr := httptest.NewRecorder()
	h.UpdateAPIKey(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDeleteAPIKey_ValidID_Returns204(t *testing.T) {
	h, adminRaw := newKeyTestHandlers(t)

	// Create a second key to delete
	raw2 := "upd_second-key-to-delete"
	k2 := models.NewAPIKey(models.NewKeyID(), "to-delete", raw2, []string{"read"})
	require.NoError(t, h.storage.CreateAPIKey(context.Background(), k2))

	req := adminCtxRequest(http.MethodDelete, "/api/v1/admin/keys/"+k2.ID, nil, h.storage, adminRaw)
	req = mux.SetURLVars(req, map[string]string{"id": k2.ID})
	rr := httptest.NewRecorder()
	h.DeleteAPIKey(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Confirm gone
	_, err := h.storage.GetAPIKeyByHash(context.Background(), k2.KeyHash)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestDeleteAPIKey_NotFound_Returns404(t *testing.T) {
	h, adminRaw := newKeyTestHandlers(t)

	req := adminCtxRequest(http.MethodDelete, "/api/v1/admin/keys/nonexistent", nil, h.storage, adminRaw)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})
	rr := httptest.NewRecorder()
	h.DeleteAPIKey(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
