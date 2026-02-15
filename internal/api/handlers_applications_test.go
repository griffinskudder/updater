package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"updater/internal/models"
	"updater/internal/storage"
	"updater/internal/update"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestHandlers creates a Handlers instance backed by real memory storage and service.
func newTestHandlers(t *testing.T) *Handlers {
	t.Helper()
	store, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	svc := update.NewService(store)
	return NewHandlers(svc, WithStorage(store))
}

// createTestApplication creates an application through the service for test setup.
func createTestApplication(t *testing.T, h *Handlers, id, name string) {
	t.Helper()
	body, _ := json.Marshal(models.CreateApplicationRequest{
		ID:        id,
		Name:      name,
		Platforms: []string{"windows", "linux"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CreateApplication(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, "setup: failed to create test application")
}

// createTestRelease registers a release through the service for test setup.
func createTestRelease(t *testing.T, h *Handlers, appID, version, platform, arch string) {
	t.Helper()
	body, _ := json.Marshal(models.RegisterReleaseRequest{
		ApplicationID: appID,
		Version:       version,
		Platform:      platform,
		Architecture:  arch,
		DownloadURL:   "https://example.com/download",
		Checksum:      "abc123def456",
		ChecksumType:  "sha256",
		FileSize:      1024,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/updates/"+appID+"/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"app_id": appID})
	rr := httptest.NewRecorder()
	h.RegisterRelease(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, "setup: failed to create test release")
}

func TestHandlers_CreateApplication(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		body           interface{}
		expectedStatus int
	}{
		{
			name:        "valid create",
			contentType: "application/json",
			body: models.CreateApplicationRequest{
				ID:        "test-app",
				Name:      "Test Application",
				Platforms: []string{"windows", "linux"},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "invalid JSON",
			contentType:    "application/json",
			body:           "not-json{{{",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "missing content-type",
			contentType: "",
			body: models.CreateApplicationRequest{
				ID:        "test-app",
				Name:      "Test Application",
				Platforms: []string{"windows"},
			},
			expectedStatus: http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandlers(t)

			var bodyBytes []byte
			switch v := tt.body.(type) {
			case string:
				bodyBytes = []byte(v)
			default:
				bodyBytes, _ = json.Marshal(v)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(bodyBytes))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rr := httptest.NewRecorder()

			h.CreateApplication(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusCreated {
				var resp models.CreateApplicationResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, "test-app", resp.ID)
				assert.Contains(t, resp.Message, "created successfully")
			}
		})
	}
}

func TestHandlers_GetApplication(t *testing.T) {
	tests := []struct {
		name           string
		appID          string
		setupApp       bool
		expectedStatus int
	}{
		{
			name:           "success",
			appID:          "test-app",
			setupApp:       true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "not found",
			appID:          "nonexistent",
			setupApp:       false,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandlers(t)

			if tt.setupApp {
				createTestApplication(t, h, tt.appID, "Test Application")
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/"+tt.appID, nil)
			req = mux.SetURLVars(req, map[string]string{"app_id": tt.appID})
			rr := httptest.NewRecorder()

			h.GetApplication(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp models.ApplicationInfoResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.appID, resp.ID)
			}
		})
	}
}

func TestHandlers_ListApplications(t *testing.T) {
	tests := []struct {
		name           string
		setupCount     int
		query          string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "empty list",
			setupCount:     0,
			query:          "",
			expectedStatus: http.StatusOK,
			expectedCount:  0,
		},
		{
			name:           "with apps",
			setupCount:     3,
			query:          "",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandlers(t)

			for i := 0; i < tt.setupCount; i++ {
				createTestApplication(t, h, "app-"+string(rune('a'+i)), "App "+string(rune('A'+i)))
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/applications"+tt.query, nil)
			rr := httptest.NewRecorder()

			h.ListApplications(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			var resp models.ListApplicationsResponse
			err := json.NewDecoder(rr.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(resp.Applications))
		})
	}
}

func TestHandlers_UpdateApplication(t *testing.T) {
	newName := "Updated Name"

	tests := []struct {
		name           string
		appID          string
		setupApp       bool
		body           interface{}
		expectedStatus int
	}{
		{
			name:     "success",
			appID:    "test-app",
			setupApp: true,
			body: models.UpdateApplicationRequest{
				Name: &newName,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:     "not found",
			appID:    "nonexistent",
			setupApp: false,
			body: models.UpdateApplicationRequest{
				Name: &newName,
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandlers(t)

			if tt.setupApp {
				createTestApplication(t, h, tt.appID, "Original Name")
			}

			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/applications/"+tt.appID, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req = mux.SetURLVars(req, map[string]string{"app_id": tt.appID})
			rr := httptest.NewRecorder()

			h.UpdateApplication(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp models.UpdateApplicationResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.appID, resp.ID)
			}
		})
	}
}

func TestHandlers_DeleteApplication(t *testing.T) {
	tests := []struct {
		name           string
		appID          string
		setupApp       bool
		setupRelease   bool
		expectedStatus int
	}{
		{
			name:           "success",
			appID:          "test-app",
			setupApp:       true,
			setupRelease:   false,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "not found",
			appID:          "nonexistent",
			setupApp:       false,
			setupRelease:   false,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "has releases",
			appID:          "test-app",
			setupApp:       true,
			setupRelease:   true,
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandlers(t)

			if tt.setupApp {
				createTestApplication(t, h, tt.appID, "Test Application")
			}

			if tt.setupRelease {
				createTestRelease(t, h, tt.appID, "1.0.0", "windows", "amd64")
			}

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/applications/"+tt.appID, nil)
			req = mux.SetURLVars(req, map[string]string{"app_id": tt.appID})
			rr := httptest.NewRecorder()

			h.DeleteApplication(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			// 204 should have no body
			if tt.expectedStatus == http.StatusNoContent {
				assert.Empty(t, rr.Body.String())
			}
		})
	}
}

func TestHandlers_DeleteRelease(t *testing.T) {
	tests := []struct {
		name           string
		appID          string
		version        string
		platform       string
		arch           string
		setupApp       bool
		setupRelease   bool
		expectedStatus int
	}{
		{
			name:           "success",
			appID:          "test-app",
			version:        "1.0.0",
			platform:       "windows",
			arch:           "amd64",
			setupApp:       true,
			setupRelease:   true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "not found",
			appID:          "test-app",
			version:        "9.9.9",
			platform:       "windows",
			arch:           "amd64",
			setupApp:       true,
			setupRelease:   false,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandlers(t)

			if tt.setupApp {
				createTestApplication(t, h, tt.appID, "Test Application")
			}

			if tt.setupRelease {
				createTestRelease(t, h, tt.appID, tt.version, tt.platform, tt.arch)
			}

			req := httptest.NewRequest(http.MethodDelete,
				"/api/v1/updates/"+tt.appID+"/releases/"+tt.version+"/"+tt.platform+"/"+tt.arch, nil)
			req = mux.SetURLVars(req, map[string]string{
				"app_id":   tt.appID,
				"version":  tt.version,
				"platform": tt.platform,
				"arch":     tt.arch,
			})
			rr := httptest.NewRecorder()

			h.DeleteRelease(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp models.DeleteReleaseResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.NotEmpty(t, resp.Message)
			}
		})
	}
}
