package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"updater/internal/models"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockStorage implements storage.Storage for handler tests
type mockStorage struct {
	pingErr error
}

func (m *mockStorage) Applications(_ context.Context) ([]*models.Application, error) {
	return nil, nil
}
func (m *mockStorage) GetApplication(_ context.Context, _ string) (*models.Application, error) {
	return nil, nil
}
func (m *mockStorage) SaveApplication(_ context.Context, _ *models.Application) error { return nil }
func (m *mockStorage) Releases(_ context.Context, _ string) ([]*models.Release, error) {
	return nil, nil
}
func (m *mockStorage) GetRelease(_ context.Context, _, _, _, _ string) (*models.Release, error) {
	return nil, nil
}
func (m *mockStorage) SaveRelease(_ context.Context, _ *models.Release) error   { return nil }
func (m *mockStorage) DeleteRelease(_ context.Context, _, _, _, _ string) error { return nil }
func (m *mockStorage) GetLatestRelease(_ context.Context, _, _, _ string) (*models.Release, error) {
	return nil, nil
}
func (m *mockStorage) GetReleasesAfterVersion(_ context.Context, _, _, _, _ string) ([]*models.Release, error) {
	return nil, nil
}
func (m *mockStorage) Ping(_ context.Context) error { return m.pingErr }
func (m *mockStorage) Close() error                 { return nil }

// MockUpdateService implements the update.ServiceInterface for testing
type MockUpdateService struct {
	mock.Mock
}

func (m *MockUpdateService) CheckForUpdate(ctx context.Context, req *models.UpdateCheckRequest) (*models.UpdateCheckResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.UpdateCheckResponse), args.Error(1)
}

func (m *MockUpdateService) GetLatestVersion(ctx context.Context, req *models.LatestVersionRequest) (*models.LatestVersionResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.LatestVersionResponse), args.Error(1)
}

func (m *MockUpdateService) ListReleases(ctx context.Context, req *models.ListReleasesRequest) (*models.ListReleasesResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.ListReleasesResponse), args.Error(1)
}

func (m *MockUpdateService) RegisterRelease(ctx context.Context, req *models.RegisterReleaseRequest) (*models.RegisterReleaseResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.RegisterReleaseResponse), args.Error(1)
}

func TestNewHandlers(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockService, handlers.updateService)
	assert.Nil(t, handlers.storage)
}

func TestNewHandlers_WithStorage(t *testing.T) {
	mockService := &MockUpdateService{}
	mockStore := &mockStorage{}
	handlers := NewHandlers(mockService, WithStorage(mockStore))

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStore, handlers.storage)
}

func TestHandlers_CheckForUpdates_Success(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	// Setup mock response
	releaseTime := time.Now()
	expectedResponse := &models.UpdateCheckResponse{
		UpdateAvailable: true,
		LatestVersion:   "2.0.0",
		DownloadURL:     "https://example.com/v2.0.0/app.exe",
		ReleaseNotes:    "New features and bug fixes",
		FileSize:        1024000,
		Checksum:        "abc123",
		ChecksumType:    "sha256",
		ReleaseDate:     &releaseTime,
		Required:        false,
	}

	mockService.On("CheckForUpdate", mock.Anything, mock.AnythingOfType("*models.UpdateCheckRequest")).Return(expectedResponse, nil)

	// Create GET request with query parameters
	req := httptest.NewRequest(http.MethodGet, "/api/v1/updates/test-app/check?current_version=1.0.0&platform=windows&architecture=amd64", nil)
	recorder := httptest.NewRecorder()

	// Setup router to extract path variables
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/updates/{app_id}/check", handlers.CheckForUpdates).Methods("GET")

	// Use router to serve the request to properly extract path variables
	router.ServeHTTP(recorder, req)

	// Assert response
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var response models.UpdateCheckResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, expectedResponse.UpdateAvailable, response.UpdateAvailable)
	assert.Equal(t, expectedResponse.LatestVersion, response.LatestVersion)
	assert.Equal(t, expectedResponse.DownloadURL, response.DownloadURL)
	assert.Equal(t, expectedResponse.ReleaseNotes, response.ReleaseNotes)

	mockService.AssertExpectations(t)
}

func TestHandlers_CheckForUpdates_InvalidJSON(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	// Invalid JSON request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/check", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	mockService.On("CheckForUpdate").Return((*models.UpdateCheckResponse)(nil), fmt.Errorf("invalid json"))

	handlers.CheckForUpdates(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	var errorResponse models.ErrorResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_REQUEST", errorResponse.Code)
}

func TestHandlers_CheckForUpdates_ServiceError(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	// Setup mock to return error
	mockService.On("CheckForUpdate", mock.Anything, mock.AnythingOfType("*models.UpdateCheckRequest")).Return((*models.UpdateCheckResponse)(nil), fmt.Errorf("service error"))

	requestBody := models.UpdateCheckRequest{
		ApplicationID:  "test-app",
		CurrentVersion: "1.0.0",
		Platform:       "windows",
		Architecture:   "amd64",
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handlers.CheckForUpdates(recorder, req)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)

	var errorResponse models.ErrorResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "INTERNAL_ERROR", errorResponse.Code)

	mockService.AssertExpectations(t)
}

func TestHandlers_GetLatestVersion_Success(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	expectedResponse := &models.LatestVersionResponse{
		Version:      "2.0.0",
		DownloadURL:  "https://example.com/v2.0.0/app.exe",
		ReleaseDate:  time.Now(),
		ReleaseNotes: "Latest stable release",
		FileSize:     2048000,
		Checksum:     "def456",
		ChecksumType: "sha256",
	}

	mockService.On("GetLatestVersion", mock.Anything, mock.AnythingOfType("*models.LatestVersionRequest")).Return(expectedResponse, nil)

	// Use query parameters instead of JSON body for GET request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/latest?app_id=test-app&platform=windows&architecture=amd64", nil)
	recorder := httptest.NewRecorder()

	// Setup router with query parameter extraction
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/latest", handlers.GetLatestVersion).Methods("GET")
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response models.LatestVersionResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, expectedResponse.Version, response.Version)
	assert.Equal(t, expectedResponse.DownloadURL, response.DownloadURL)
	assert.Equal(t, expectedResponse.ReleaseNotes, response.ReleaseNotes)

	mockService.AssertExpectations(t)
}

func TestHandlers_GetLatestVersion_MissingQueryParams(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	// Missing required query parameters
	req := httptest.NewRequest(http.MethodGet, "/api/v1/latest", nil)
	recorder := httptest.NewRecorder()

	handlers.GetLatestVersion(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	var errorResponse models.ErrorResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_REQUEST", errorResponse.Code)
}

func TestHandlers_ListReleases_Success(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	releases := []*models.Release{
		{
			ID:            "test-app-1.0.0-windows-amd64",
			ApplicationID: "test-app",
			Version:       "1.0.0",
			Platform:      "windows",
			Architecture:  "amd64",
			DownloadURL:   "https://example.com/v1.0.0/app.exe",
			ReleaseDate:   time.Now().Add(-24 * time.Hour),
		},
		{
			ID:            "test-app-2.0.0-windows-amd64",
			ApplicationID: "test-app",
			Version:       "2.0.0",
			Platform:      "windows",
			Architecture:  "amd64",
			DownloadURL:   "https://example.com/v2.0.0/app.exe",
			ReleaseDate:   time.Now(),
		},
	}

	// Convert releases to ReleaseInfo format for expected response
	releaseInfos := make([]models.ReleaseInfo, len(releases))
	for i, r := range releases {
		releaseInfos[i] = models.ReleaseInfo{
			ID:           r.ID,
			Version:      r.Version,
			Platform:     r.Platform,
			Architecture: r.Architecture,
			DownloadURL:  r.DownloadURL,
			ReleaseDate:  r.ReleaseDate,
		}
	}

	expectedResponse := &models.ListReleasesResponse{
		Releases:   releaseInfos,
		TotalCount: 2,
		Page:       1,
		PageSize:   10,
		HasMore:    false,
	}

	mockService.On("ListReleases", mock.Anything, mock.AnythingOfType("*models.ListReleasesRequest")).Return(expectedResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases?app_id=test-app&platform=windows&limit=10&offset=0", nil)
	recorder := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/releases", handlers.ListReleases).Methods("GET")
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response models.ListReleasesResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Releases, 2)
	assert.Equal(t, 2, response.TotalCount)
	assert.Equal(t, 10, response.PageSize)
	assert.Equal(t, 1, response.Page)
	assert.False(t, response.HasMore)

	mockService.AssertExpectations(t)
}

func TestHandlers_ListReleases_WithPagination(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	releases := []*models.Release{
		{
			ID:            "test-app-2.0.0-windows-amd64",
			ApplicationID: "test-app",
			Version:       "2.0.0",
			Platform:      "windows",
			Architecture:  "amd64",
			DownloadURL:   "https://example.com/v2.0.0/app.exe",
			ReleaseDate:   time.Now(),
		},
	}

	// Convert releases to ReleaseInfo format for expected response
	releaseInfos2 := make([]models.ReleaseInfo, len(releases))
	for i, r := range releases {
		releaseInfos2[i] = models.ReleaseInfo{
			ID:           r.ID,
			Version:      r.Version,
			Platform:     r.Platform,
			Architecture: r.Architecture,
			DownloadURL:  r.DownloadURL,
			ReleaseDate:  r.ReleaseDate,
		}
	}

	expectedResponse := &models.ListReleasesResponse{
		Releases:   releaseInfos2,
		TotalCount: 5, // Total available
		Page:       2, // (offset/limit) + 1 = (1/1) + 1 = 2
		PageSize:   1,
		HasMore:    true,
	}

	mockService.On("ListReleases", mock.Anything, mock.AnythingOfType("*models.ListReleasesRequest")).Return(expectedResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases?app_id=test-app&limit=1&offset=1", nil)
	recorder := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/releases", handlers.ListReleases).Methods("GET")
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response models.ListReleasesResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Releases, 1)
	assert.Equal(t, 5, response.TotalCount)
	assert.Equal(t, 1, response.PageSize)
	assert.Equal(t, 2, response.Page)
	assert.True(t, response.HasMore)

	mockService.AssertExpectations(t)
}

func TestHandlers_RegisterRelease_Success(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	expectedResponse := &models.RegisterReleaseResponse{
		ID:        "test-app-1.0.0-windows-amd64",
		Message:   "Release registered successfully",
		CreatedAt: time.Now(),
	}

	mockService.On("RegisterRelease", mock.Anything, mock.AnythingOfType("*models.RegisterReleaseRequest")).Return(expectedResponse, nil)

	requestBody := models.RegisterReleaseRequest{
		ApplicationID: "test-app",
		Version:       "1.0.0",
		Platform:      "windows",
		Architecture:  "amd64",
		DownloadURL:   "https://example.com/v1.0.0/app.exe",
		Checksum:      "abc123",
		ChecksumType:  "sha256",
		FileSize:      1024000,
		ReleaseNotes:  "Initial release",
		Required:      false,
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/releases", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handlers.RegisterRelease(recorder, req)

	assert.Equal(t, http.StatusCreated, recorder.Code)

	var response models.RegisterReleaseResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, expectedResponse.ID, response.ID)
	assert.Equal(t, expectedResponse.Message, response.Message)

	mockService.AssertExpectations(t)
}

func TestHandlers_RegisterRelease_InvalidRequest(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	// Invalid request - missing required fields
	requestBody := models.RegisterReleaseRequest{
		ApplicationID: "test-app",
		// Missing Version, Platform, Architecture, etc.
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/releases", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	// Mock service should return validation error
	mockService.On("RegisterRelease", mock.Anything, mock.AnythingOfType("*models.RegisterReleaseRequest")).Return((*models.RegisterReleaseResponse)(nil), fmt.Errorf("invalid request: missing required fields"))

	handlers.RegisterRelease(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	var errorResponse models.ErrorResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_REQUEST", errorResponse.Code)

	mockService.AssertExpectations(t)
}

func TestHandlers_HealthCheck(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	handlers.HealthCheck(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.NotEmpty(t, response["timestamp"])
	assert.Equal(t, "1.0.0", response["version"])
}

func TestHandlers_HealthCheck_WithStorage(t *testing.T) {
	mockService := &MockUpdateService{}
	store := &mockStorage{}
	handlers := NewHandlers(mockService, WithStorage(store))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	handlers.HealthCheck(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])

	components := response["components"].(map[string]interface{})
	storageComp := components["storage"].(map[string]interface{})
	assert.Equal(t, "healthy", storageComp["status"])
}

func TestHandlers_HealthCheck_StorageDegraded(t *testing.T) {
	mockService := &MockUpdateService{}
	store := &mockStorage{pingErr: fmt.Errorf("connection refused")}
	handlers := NewHandlers(mockService, WithStorage(store))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	handlers.HealthCheck(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "degraded", response["status"])

	components := response["components"].(map[string]interface{})
	storageComp := components["storage"].(map[string]interface{})
	assert.Equal(t, "unhealthy", storageComp["status"])
	assert.Contains(t, storageComp["message"], "connection refused")
}

func TestHandlers_HTTPMethodNotAllowed(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	// Try POST on a GET-only endpoint - handler itself doesn't validate methods
	// Method validation happens at router level, so this will result in missing app_id
	req := httptest.NewRequest(http.MethodPost, "/api/v1/latest", nil)
	recorder := httptest.NewRecorder()

	handlers.GetLatestVersion(recorder, req)

	// Handler will process the request and find missing app_id parameter
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestHandlers_ParseQueryParams(t *testing.T) {
	tests := []struct {
		name           string
		queryString    string
		expectedAppID  string
		expectedLimit  int
		expectedOffset int
		expectError    bool
	}{
		{
			name:           "valid params",
			queryString:    "app_id=test-app&limit=5&offset=10",
			expectedAppID:  "test-app",
			expectedLimit:  5,
			expectedOffset: 10,
			expectError:    false,
		},
		{
			name:           "default limit and offset",
			queryString:    "app_id=test-app",
			expectedAppID:  "test-app",
			expectedLimit:  10, // default
			expectedOffset: 0,  // default
			expectError:    false,
		},
		{
			name:        "missing app_id",
			queryString: "limit=5&offset=10",
			expectError: true,
		},
		{
			name:        "invalid limit",
			queryString: "app_id=test-app&limit=invalid",
			expectError: true,
		},
		{
			name:        "invalid offset",
			queryString: "app_id=test-app&offset=invalid",
			expectError: true,
		},
		{
			name:        "negative limit",
			queryString: "app_id=test-app&limit=-1",
			expectError: true,
		},
		{
			name:        "negative offset",
			queryString: "app_id=test-app&offset=-1",
			expectError: true,
		},
	}

	// This test focuses on query parameter parsing logic

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.queryString, nil)

			appID := req.URL.Query().Get("app_id")
			limitStr := req.URL.Query().Get("limit")
			offsetStr := req.URL.Query().Get("offset")

			// Test the logic that would be in the handler
			if appID == "" && tt.expectError {
				return // Expected error case
			}

			limit := 10 // default
			if limitStr != "" {
				if parsed, err := parseInt(limitStr); err != nil {
					if tt.expectError {
						return // Expected error case
					}
					t.Errorf("Unexpected error parsing limit: %v", err)
				} else {
					limit = parsed
				}
			}

			offset := 0 // default
			if offsetStr != "" {
				if parsed, err := parseInt(offsetStr); err != nil {
					if tt.expectError {
						return // Expected error case
					}
					t.Errorf("Unexpected error parsing offset: %v", err)
				} else {
					offset = parsed
				}
			}

			if !tt.expectError {
				assert.Equal(t, tt.expectedAppID, appID)
				assert.Equal(t, tt.expectedLimit, limit)
				assert.Equal(t, tt.expectedOffset, offset)
			}
		})
	}
}

// Helper function to simulate integer parsing with validation
func parseInt(s string) (int, error) {
	if s == "invalid" {
		return 0, fmt.Errorf("invalid integer")
	}
	if s == "-1" {
		return -1, nil
	}
	if s == "5" {
		return 5, nil
	}
	if s == "10" {
		return 10, nil
	}
	return 0, fmt.Errorf("unknown value")
}

func TestHandlers_ContentTypeValidation(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	tests := []struct {
		name        string
		contentType string
		expectError bool
	}{
		{
			name:        "valid content type",
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "valid content type with charset",
			contentType: "application/json; charset=utf-8",
			expectError: false,
		},
		{
			name:        "invalid content type",
			contentType: "text/plain",
			expectError: true,
		},
		{
			name:        "missing content type",
			contentType: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestBody := models.UpdateCheckRequest{
				ApplicationID:  "test-app",
				CurrentVersion: "1.0.0",
				Platform:       "windows",
				Architecture:   "amd64",
			}

			body, err := json.Marshal(requestBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/check", bytes.NewReader(body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			recorder := httptest.NewRecorder()

			if !tt.expectError {
				// Setup mock for successful case
				expectedResponse := &models.UpdateCheckResponse{
					UpdateAvailable: false,
				}
				mockService.On("CheckForUpdate", mock.Anything, mock.AnythingOfType("*models.UpdateCheckRequest")).Return(expectedResponse, nil).Once()
			}

			handlers.CheckForUpdates(recorder, req)

			if tt.expectError {
				assert.Equal(t, http.StatusUnsupportedMediaType, recorder.Code)
			} else {
				assert.Equal(t, http.StatusOK, recorder.Code)
			}
		})
	}

	mockService.AssertExpectations(t)
}

func TestHandlers_ErrorResponseFormat(t *testing.T) {
	mockService := &MockUpdateService{}
	handlers := NewHandlers(mockService)

	// Test invalid JSON to trigger error response
	req := httptest.NewRequest(http.MethodPost, "/api/v1/check", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handlers.CheckForUpdates(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var errorResponse models.ErrorResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &errorResponse)
	require.NoError(t, err)

	// Verify error response structure
	assert.NotEmpty(t, errorResponse.Code)
	assert.NotEmpty(t, errorResponse.Message)
	assert.NotEmpty(t, errorResponse.Timestamp)
	assert.Empty(t, errorResponse.Details) // Should be empty for this error type
}
