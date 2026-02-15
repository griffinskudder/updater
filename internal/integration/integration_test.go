package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"updater/internal/api"
	"updater/internal/config"
	"updater/internal/models"
	"updater/internal/storage"
	"updater/internal/update"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests that test the entire system end-to-end

func TestIntegration_FullUpdateFlow(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	storageFile := filepath.Join(tempDir, "test_releases.json")

	// Initialize storage
	storageConfig := storage.Config{
		Type:     "json",
		Path:     storageFile,
		CacheTTL: "1m",
	}

	store, err := storage.NewJSONStorage(storageConfig)
	require.NoError(t, err)
	defer store.Close()

	// Initialize services
	updateService := update.NewService(store)
	handlers := api.NewHandlers(updateService)

	// Create test configuration
	cfg := &models.Config{
		Server: models.ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Storage: models.StorageConfig{
			Type: "json",
			Path: storageFile,
		},
	}

	// Setup routes
	router := api.SetupRoutes(handlers, cfg)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx := context.Background()

	// Step 1: Create a test application
	app := &models.Application{
		ID:        "integration-test-app",
		Name:      "Integration Test App",
		Platforms: []string{"windows", "linux", "darwin"},
		Config: models.ApplicationConfig{
			AutoUpdate:       true,
			UpdateInterval:   3600,
			RequiredUpdate:   false,
			AllowPrerelease:  false,
			AnalyticsEnabled: true,
		},
	}

	err = store.SaveApplication(ctx, app)
	require.NoError(t, err)

	// Step 2: Register initial release (v1.0.0)
	registerRequest := models.RegisterReleaseRequest{
		ApplicationID: "integration-test-app",
		Version:       "1.0.0",
		Platform:      "windows",
		Architecture:  "amd64",
		DownloadURL:   "https://releases.example.com/v1.0.0/app-windows-amd64.exe",
		Checksum:      "abc123def456",
		ChecksumType:  "sha256",
		FileSize:      10485760, // 10MB
		ReleaseNotes:  "Initial release with core functionality",
		Required:      false,
	}

	body, err := json.Marshal(registerRequest)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/api/v1/updates/integration-test-app/register", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var registerResponse models.RegisterReleaseResponse
	err = json.NewDecoder(resp.Body).Decode(&registerResponse)
	require.NoError(t, err)
	assert.NotEmpty(t, registerResponse.ID)
	assert.Contains(t, registerResponse.Message, "registered successfully")

	// Step 3: Register newer release (v1.1.0)
	newerRegisterRequest := models.RegisterReleaseRequest{
		ApplicationID: "integration-test-app",
		Version:       "1.1.0",
		Platform:      "windows",
		Architecture:  "amd64",
		DownloadURL:   "https://releases.example.com/v1.1.0/app-windows-amd64.exe",
		Checksum:      "def456abc789",
		ChecksumType:  "sha256",
		FileSize:      11534336, // 11MB
		ReleaseNotes:  "Bug fixes and performance improvements",
		Required:      false,
	}

	body, err = json.Marshal(newerRegisterRequest)
	require.NoError(t, err)

	resp, err = http.Post(server.URL+"/api/v1/updates/integration-test-app/register", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Step 4: Check for update (should find v1.1.0 when current is v1.0.0)
	updateCheckRequest := models.UpdateCheckRequest{
		ApplicationID:   "integration-test-app",
		CurrentVersion:  "1.0.0",
		Platform:        "windows",
		Architecture:    "amd64",
		AllowPrerelease: false,
	}

	body, err = json.Marshal(updateCheckRequest)
	require.NoError(t, err)

	resp, err = http.Post(server.URL+"/api/v1/check", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var updateCheckResponse models.UpdateCheckResponse
	err = json.NewDecoder(resp.Body).Decode(&updateCheckResponse)
	require.NoError(t, err)

	assert.True(t, updateCheckResponse.UpdateAvailable)
	assert.Equal(t, "1.1.0", updateCheckResponse.LatestVersion)
	assert.Equal(t, "https://releases.example.com/v1.1.0/app-windows-amd64.exe", updateCheckResponse.DownloadURL)
	assert.Equal(t, "Bug fixes and performance improvements", updateCheckResponse.ReleaseNotes)
	assert.Equal(t, int64(11534336), updateCheckResponse.FileSize)
	assert.Equal(t, "def456abc789", updateCheckResponse.Checksum)
	assert.Equal(t, "sha256", updateCheckResponse.ChecksumType)
	assert.False(t, updateCheckResponse.Required)

	// Step 5: Check for update when already on latest (should report no update)
	updateCheckRequest.CurrentVersion = "1.1.0"
	body, err = json.Marshal(updateCheckRequest)
	require.NoError(t, err)

	resp, err = http.Post(server.URL+"/api/v1/check", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&updateCheckResponse)
	require.NoError(t, err)

	assert.False(t, updateCheckResponse.UpdateAvailable)

	// Step 6: Get latest version directly
	resp, err = http.Get(server.URL + "/api/v1/latest?app_id=integration-test-app&platform=windows&architecture=amd64")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var latestVersionResponse models.LatestVersionResponse
	err = json.NewDecoder(resp.Body).Decode(&latestVersionResponse)
	require.NoError(t, err)

	assert.Equal(t, "1.1.0", latestVersionResponse.Version)
	assert.Equal(t, "https://releases.example.com/v1.1.0/app-windows-amd64.exe", latestVersionResponse.DownloadURL)

	// Step 7: List all releases
	resp, err = http.Get(server.URL + "/api/v1/updates/integration-test-app/releases?platform=windows&architecture=amd64")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listReleasesResponse models.ListReleasesResponse
	err = json.NewDecoder(resp.Body).Decode(&listReleasesResponse)
	require.NoError(t, err)

	assert.Len(t, listReleasesResponse.Releases, 2)
	assert.Equal(t, 2, listReleasesResponse.TotalCount)

	// Releases should be sorted by release date (newest first)
	assert.Equal(t, "1.1.0", listReleasesResponse.Releases[0].Version)
	assert.Equal(t, "1.0.0", listReleasesResponse.Releases[1].Version)

	// Step 8: Health check
	resp, err = http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var healthResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&healthResponse)
	require.NoError(t, err)

	assert.Equal(t, "healthy", healthResponse["status"])
	assert.NotEmpty(t, healthResponse["timestamp"])
}

func TestIntegration_PreReleaseHandling(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	storageFile := filepath.Join(tempDir, "prerelease_test.json")

	storageConfig := storage.Config{
		Type:     "json",
		Path:     storageFile,
		CacheTTL: "1m",
	}

	store, err := storage.NewJSONStorage(storageConfig)
	require.NoError(t, err)
	defer store.Close()

	updateService := update.NewService(store)
	handlers := api.NewHandlers(updateService)

	cfg := &models.Config{
		Server: models.ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Storage: models.StorageConfig{
			Type: "json",
			Path: storageFile,
		},
	}

	router := api.SetupRoutes(handlers, cfg)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx := context.Background()

	// Create test application
	app := &models.Application{
		ID:        "prerelease-test-app",
		Name:      "Prerelease Test App",
		Platforms: []string{"windows"},
		Config: models.ApplicationConfig{
			AllowPrerelease: true,
		},
	}

	err = store.SaveApplication(ctx, app)
	require.NoError(t, err)

	// Register stable release (v1.0.0)
	registerStable := models.RegisterReleaseRequest{
		ApplicationID: "prerelease-test-app",
		Version:       "1.0.0",
		Platform:      "windows",
		Architecture:  "amd64",
		DownloadURL:   "https://example.com/v1.0.0/app.exe",
		Checksum:      "stable123",
		ChecksumType:  "sha256",
		FileSize:      10485760,
		ReleaseNotes:  "Stable release",
		Required:      false,
	}

	body, err := json.Marshal(registerStable)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/api/v1/updates/prerelease-test-app/register", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Register pre-release (v1.1.0-beta.1)
	registerPrerelease := models.RegisterReleaseRequest{
		ApplicationID: "prerelease-test-app",
		Version:       "1.1.0-beta.1",
		Platform:      "windows",
		Architecture:  "amd64",
		DownloadURL:   "https://example.com/v1.1.0-beta.1/app.exe",
		Checksum:      "beta123",
		ChecksumType:  "sha256",
		FileSize:      11534336,
		ReleaseNotes:  "Beta release with new features",
		Required:      false,
	}

	body, err = json.Marshal(registerPrerelease)
	require.NoError(t, err)

	resp, err = http.Post(server.URL+"/api/v1/updates/prerelease-test-app/register", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Test 1: Check for update with prerelease disabled (should get stable)
	updateCheck := models.UpdateCheckRequest{
		ApplicationID:   "prerelease-test-app",
		CurrentVersion:  "0.9.0",
		Platform:        "windows",
		Architecture:    "amd64",
		AllowPrerelease: false,
	}

	body, err = json.Marshal(updateCheck)
	require.NoError(t, err)

	resp, err = http.Post(server.URL+"/api/v1/check", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var updateResponse models.UpdateCheckResponse
	err = json.NewDecoder(resp.Body).Decode(&updateResponse)
	require.NoError(t, err)

	assert.True(t, updateResponse.UpdateAvailable)
	assert.Equal(t, "1.0.0", updateResponse.LatestVersion)

	// Test 2: Check for update with prerelease enabled (should get beta)
	updateCheck.AllowPrerelease = true
	body, err = json.Marshal(updateCheck)
	require.NoError(t, err)

	resp, err = http.Post(server.URL+"/api/v1/check", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&updateResponse)
	require.NoError(t, err)

	assert.True(t, updateResponse.UpdateAvailable)
	assert.Equal(t, "1.1.0-beta.1", updateResponse.LatestVersion)
}

func TestIntegration_ErrorHandling(t *testing.T) {
	// Setup minimal test environment
	tempDir := t.TempDir()
	storageFile := filepath.Join(tempDir, "error_test.json")

	storageConfig := storage.Config{
		Type:     "json",
		Path:     storageFile,
		CacheTTL: "1m",
	}

	store, err := storage.NewJSONStorage(storageConfig)
	require.NoError(t, err)
	defer store.Close()

	updateService := update.NewService(store)
	handlers := api.NewHandlers(updateService)

	cfg := &models.Config{
		Server: models.ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Storage: models.StorageConfig{
			Type: "json",
			Path: storageFile,
		},
	}

	router := api.SetupRoutes(handlers, cfg)
	server := httptest.NewServer(router)
	defer server.Close()

	// Test 1: Check for update with non-existent application
	updateCheckRequest := models.UpdateCheckRequest{
		ApplicationID:  "non-existent-app",
		CurrentVersion: "1.0.0",
		Platform:       "windows",
		Architecture:   "amd64",
	}

	body, err := json.Marshal(updateCheckRequest)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/api/v1/check", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var errorResponse models.ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "APPLICATION_NOT_FOUND", errorResponse.Code)
	assert.Contains(t, errorResponse.Message, "not found")

	// Test 2: Invalid request format
	resp, err = http.Post(server.URL+"/api/v1/check", "application/json", bytes.NewReader([]byte("invalid json")))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_REQUEST", errorResponse.Code)

	// Test 3: Missing required query parameters
	resp, err = http.Get(server.URL + "/api/v1/latest")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Test 4: Wrong content type
	resp, err = http.Post(server.URL+"/api/v1/check", "text/plain", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)

	// Test 5: Method not allowed
	req, err := http.NewRequest("DELETE", server.URL+"/api/v1/check", nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	storageFile := filepath.Join(tempDir, "concurrent_test.json")

	storageConfig := storage.Config{
		Type:     "json",
		Path:     storageFile,
		CacheTTL: "1m",
	}

	store, err := storage.NewJSONStorage(storageConfig)
	require.NoError(t, err)
	defer store.Close()

	updateService := update.NewService(store)
	handlers := api.NewHandlers(updateService)

	cfg := &models.Config{
		Server: models.ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Storage: models.StorageConfig{
			Type: "json",
			Path: storageFile,
		},
	}

	router := api.SetupRoutes(handlers, cfg)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx := context.Background()

	// Create test application
	app := &models.Application{
		ID:        "concurrent-test-app",
		Name:      "Concurrent Test App",
		Platforms: []string{"windows"},
		Config:    models.ApplicationConfig{},
	}

	err = store.SaveApplication(ctx, app)
	require.NoError(t, err)

	// Register a release
	registerRequest := models.RegisterReleaseRequest{
		ApplicationID: "concurrent-test-app",
		Version:       "1.0.0",
		Platform:      "windows",
		Architecture:  "amd64",
		DownloadURL:   "https://example.com/v1.0.0/app.exe",
		Checksum:      "concurrent123",
		ChecksumType:  "sha256",
		FileSize:      10485760,
		ReleaseNotes:  "Concurrent test release",
		Required:      false,
	}

	body, err := json.Marshal(registerRequest)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/api/v1/updates/concurrent-test-app/register", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	resp.Body.Close()

	// Perform concurrent update checks
	const numRequests = 10
	results := make(chan error, numRequests)

	updateCheckRequest := models.UpdateCheckRequest{
		ApplicationID:  "concurrent-test-app",
		CurrentVersion: "0.9.0",
		Platform:       "windows",
		Architecture:   "amd64",
	}

	requestBody, err := json.Marshal(updateCheckRequest)
	require.NoError(t, err)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			resp, err := http.Post(server.URL+"/api/v1/check", "application/json", bytes.NewReader(requestBody))
			if err != nil {
				results <- fmt.Errorf("request %d failed: %v", id, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("request %d got status %d", id, resp.StatusCode)
				return
			}

			var updateResponse models.UpdateCheckResponse
			if err := json.NewDecoder(resp.Body).Decode(&updateResponse); err != nil {
				results <- fmt.Errorf("request %d decode error: %v", id, err)
				return
			}

			if !updateResponse.UpdateAvailable || updateResponse.LatestVersion != "1.0.0" {
				results <- fmt.Errorf("request %d got unexpected response", id)
				return
			}

			results <- nil
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		err := <-results
		assert.NoError(t, err, "Concurrent request failed")
	}
}

func TestIntegration_ConfigLoading(t *testing.T) {
	// Test configuration loading and validation
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "integration_config.yaml")

	configContent := `
server:
  port: 8081
  host: "127.0.0.1"
  read_timeout: 45s
  write_timeout: 45s
  idle_timeout: 90s

storage:
  type: "json"
  path: "./integration_test.json"

security:
  enable_auth: false
  rate_limit:
    enabled: true
    requests_per_minute: 120

logging:
  level: "debug"
  format: "text"

cache:
  enabled: true
  ttl: 600s

metrics:
  enabled: true
  port: 9091
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load configuration
	cfg, err := config.Load(configFile)
	require.NoError(t, err)

	// Validate loaded configuration
	assert.Equal(t, 8081, cfg.Server.Port)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 45*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 45*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 90*time.Second, cfg.Server.IdleTimeout)

	assert.Equal(t, "json", cfg.Storage.Type)
	assert.Equal(t, "./integration_test.json", cfg.Storage.Path)

	assert.False(t, cfg.Security.EnableAuth)
	assert.True(t, cfg.Security.RateLimit.Enabled)
	assert.Equal(t, 120, cfg.Security.RateLimit.RequestsPerMinute)

	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)

	assert.True(t, cfg.Cache.Enabled)
	assert.Equal(t, 600*time.Second, cfg.Cache.TTL)

	assert.True(t, cfg.Metrics.Enabled)
	assert.Equal(t, 9091, cfg.Metrics.Port)

	// Test configuration validation
	err = cfg.Validate()
	assert.NoError(t, err)
}

func TestIntegration_PaginationAndFiltering(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	storageFile := filepath.Join(tempDir, "pagination_test.json")

	storageConfig := storage.Config{
		Type:     "json",
		Path:     storageFile,
		CacheTTL: "1m",
	}

	store, err := storage.NewJSONStorage(storageConfig)
	require.NoError(t, err)
	defer store.Close()

	updateService := update.NewService(store)
	handlers := api.NewHandlers(updateService)

	cfg := &models.Config{
		Server: models.ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Storage: models.StorageConfig{
			Type: "json",
			Path: storageFile,
		},
	}

	router := api.SetupRoutes(handlers, cfg)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx := context.Background()

	// Create test application
	app := &models.Application{
		ID:        "pagination-test-app",
		Name:      "Pagination Test App",
		Platforms: []string{"windows", "linux"},
		Config:    models.ApplicationConfig{},
	}

	err = store.SaveApplication(ctx, app)
	require.NoError(t, err)

	// Register multiple releases
	versions := []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0", "2.1.0"}
	platforms := []string{"windows", "linux"}

	for _, version := range versions {
		for _, platform := range platforms {
			registerRequest := models.RegisterReleaseRequest{
				ApplicationID: "pagination-test-app",
				Version:       version,
				Platform:      platform,
				Architecture:  "amd64",
				DownloadURL:   fmt.Sprintf("https://example.com/%s/app-%s-amd64.exe", version, platform),
				Checksum:      fmt.Sprintf("%s-%s-checksum", version, platform),
				ChecksumType:  "sha256",
				FileSize:      10485760,
				ReleaseNotes:  fmt.Sprintf("Release %s for %s", version, platform),
				Required:      false,
			}

			body, err := json.Marshal(registerRequest)
			require.NoError(t, err)

			resp, err := http.Post(server.URL+"/api/v1/updates/pagination-test-app/register", "application/json", bytes.NewReader(body))
			require.NoError(t, err)
			resp.Body.Close()
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
		}
	}

	// Test 1: List all releases (default pagination)
	resp, err := http.Get(server.URL + "/api/v1/updates/pagination-test-app/releases")
	require.NoError(t, err)
	defer resp.Body.Close()

	var listResponse models.ListReleasesResponse
	err = json.NewDecoder(resp.Body).Decode(&listResponse)
	require.NoError(t, err)

	assert.Len(t, listResponse.Releases, 10) // 5 versions × 2 platforms
	assert.Equal(t, 10, listResponse.TotalCount)

	// Test 2: Pagination with limit and offset
	resp, err = http.Get(server.URL + "/api/v1/updates/pagination-test-app/releases?limit=3&offset=2")
	require.NoError(t, err)
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&listResponse)
	require.NoError(t, err)

	assert.Len(t, listResponse.Releases, 3)
	assert.Equal(t, 10, listResponse.TotalCount)
	assert.Equal(t, 3, listResponse.PageSize)
	assert.Equal(t, 1, listResponse.Page) // Page is calculated as (offset/limit) + 1 = (2/3) + 1 = 0 + 1 = 1 (integer division)

	// Test 3: Platform filtering
	resp, err = http.Get(server.URL + "/api/v1/updates/pagination-test-app/releases?platform=windows")
	require.NoError(t, err)
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&listResponse)
	require.NoError(t, err)

	assert.Len(t, listResponse.Releases, 5) // 5 versions for windows
	for _, release := range listResponse.Releases {
		assert.Equal(t, "windows", release.Platform)
	}

	// Test 4: Combined filtering and pagination
	resp, err = http.Get(server.URL + "/api/v1/updates/pagination-test-app/releases?platform=linux&limit=2&offset=1")
	require.NoError(t, err)
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&listResponse)
	require.NoError(t, err)

	assert.Len(t, listResponse.Releases, 2)
	assert.Equal(t, 2, listResponse.PageSize)
	assert.Equal(t, 1, listResponse.Page) // Page is calculated as (offset/limit) + 1 = (1/2) + 1 = 0 + 1 = 1 (integer division)
	for _, release := range listResponse.Releases {
		assert.Equal(t, "linux", release.Platform)
	}
}

// setupTestServer creates a test server with the given auth configuration.
// When enableAuth is true, it configures API keys with read, write, and admin permissions.
func setupTestServer(t *testing.T, enableAuth bool) (*httptest.Server, storage.Storage) {
	t.Helper()

	tempDir := t.TempDir()
	storageFile := filepath.Join(tempDir, "test_releases.json")

	storageConfig := storage.Config{
		Type:     "json",
		Path:     storageFile,
		CacheTTL: "1m",
	}

	store, err := storage.NewJSONStorage(storageConfig)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	updateService := update.NewService(store)
	handlers := api.NewHandlers(updateService)

	cfg := &models.Config{
		Server: models.ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Storage: models.StorageConfig{
			Type: "json",
			Path: storageFile,
		},
	}

	if enableAuth {
		cfg.Security = models.SecurityConfig{
			EnableAuth: true,
			APIKeys: []models.APIKey{
				{
					Key:         "test-read-key",
					Name:        "read-key",
					Permissions: []string{"read"},
					Enabled:     true,
				},
				{
					Key:         "test-write-key",
					Name:        "write-key",
					Permissions: []string{"write"},
					Enabled:     true,
				},
				{
					Key:         "test-admin-key",
					Name:        "admin-key",
					Permissions: []string{"admin"},
					Enabled:     true,
				},
			},
		}
	}

	router := api.SetupRoutes(handlers, cfg)
	server := httptest.NewServer(router)
	t.Cleanup(func() { server.Close() })

	return server, store
}

// doRequest is a helper that creates and executes an HTTP request with optional auth and body.
func doRequest(t *testing.T, method, url, authToken string, body interface{}) *http.Response {
	t.Helper()

	var reqBody *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(data)
	} else {
		reqBody = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, url, reqBody)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func TestApplicationLifecycle(t *testing.T) {
	server, _ := setupTestServer(t, true)

	// Step 1: Create application via POST with write auth
	createReq := models.CreateApplicationRequest{
		ID:          "lifecycle-test-app",
		Name:        "Lifecycle Test App",
		Description: "An application for lifecycle testing",
		Platforms:   []string{"windows", "linux", "darwin"},
		Config: models.ApplicationConfig{
			AutoUpdate:      true,
			UpdateInterval:  3600,
			AllowPrerelease: false,
		},
	}

	resp := doRequest(t, "POST", server.URL+"/api/v1/applications", "test-write-key", createReq)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResp models.CreateApplicationResponse
	err := json.NewDecoder(resp.Body).Decode(&createResp)
	require.NoError(t, err)
	assert.Equal(t, "lifecycle-test-app", createResp.ID)
	assert.NotEmpty(t, createResp.Message)

	// Step 2: Get application via GET with read auth
	resp = doRequest(t, "GET", server.URL+"/api/v1/applications/lifecycle-test-app", "test-read-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var appInfo models.ApplicationInfoResponse
	err = json.NewDecoder(resp.Body).Decode(&appInfo)
	require.NoError(t, err)
	assert.Equal(t, "lifecycle-test-app", appInfo.ID)
	assert.Equal(t, "Lifecycle Test App", appInfo.Name)
	assert.Equal(t, "An application for lifecycle testing", appInfo.Description)
	assert.ElementsMatch(t, []string{"windows", "linux", "darwin"}, appInfo.Platforms)
	assert.True(t, appInfo.Config.AutoUpdate)
	assert.Equal(t, 3600, appInfo.Config.UpdateInterval)
	assert.False(t, appInfo.Config.AllowPrerelease)

	// Step 3: List applications via GET with read auth
	resp = doRequest(t, "GET", server.URL+"/api/v1/applications", "test-read-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp models.ListApplicationsResponse
	err = json.NewDecoder(resp.Body).Decode(&listResp)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, listResp.TotalCount, 1)

	found := false
	for _, app := range listResp.Applications {
		if app.ID == "lifecycle-test-app" {
			found = true
			assert.Equal(t, "Lifecycle Test App", app.Name)
			break
		}
	}
	assert.True(t, found, "lifecycle-test-app should be in the application list")

	// Step 4: Update application via PUT with admin auth
	updatedName := "Lifecycle Test App Updated"
	updatedDescription := "Updated description"
	updateReq := models.UpdateApplicationRequest{
		Name:        &updatedName,
		Description: &updatedDescription,
		Platforms:   []string{"windows", "linux"},
	}

	resp = doRequest(t, "PUT", server.URL+"/api/v1/applications/lifecycle-test-app", "test-admin-key", updateReq)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var updateResp models.UpdateApplicationResponse
	err = json.NewDecoder(resp.Body).Decode(&updateResp)
	require.NoError(t, err)
	assert.Equal(t, "lifecycle-test-app", updateResp.ID)
	assert.NotEmpty(t, updateResp.Message)

	// Step 5: Get updated application and verify updated fields
	resp = doRequest(t, "GET", server.URL+"/api/v1/applications/lifecycle-test-app", "test-read-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&appInfo)
	require.NoError(t, err)
	assert.Equal(t, "Lifecycle Test App Updated", appInfo.Name)
	assert.Equal(t, "Updated description", appInfo.Description)
	assert.ElementsMatch(t, []string{"windows", "linux"}, appInfo.Platforms)

	// Step 6: Delete application via DELETE with admin auth
	resp = doRequest(t, "DELETE", server.URL+"/api/v1/applications/lifecycle-test-app", "test-admin-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Step 7: Get deleted application -- expect 404
	resp = doRequest(t, "GET", server.URL+"/api/v1/applications/lifecycle-test-app", "test-read-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteApplicationWithReleases(t *testing.T) {
	server, _ := setupTestServer(t, true)

	// Step 1: Create application
	createReq := models.CreateApplicationRequest{
		ID:        "delete-with-releases-app",
		Name:      "Delete With Releases App",
		Platforms: []string{"windows"},
		Config:    models.ApplicationConfig{},
	}

	resp := doRequest(t, "POST", server.URL+"/api/v1/applications", "test-write-key", createReq)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Step 2: Register a release for the application
	registerReq := models.RegisterReleaseRequest{
		ApplicationID: "delete-with-releases-app",
		Version:       "1.0.0",
		Platform:      "windows",
		Architecture:  "amd64",
		DownloadURL:   "https://releases.example.com/v1.0.0/app-windows-amd64.exe",
		Checksum:      "abc123def456",
		ChecksumType:  "sha256",
		FileSize:      10485760,
		ReleaseNotes:  "Initial release",
		Required:      false,
	}

	resp = doRequest(t, "POST", server.URL+"/api/v1/updates/delete-with-releases-app/register", "test-write-key", registerReq)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Step 3: Try to delete the application -- expect 409 Conflict
	resp = doRequest(t, "DELETE", server.URL+"/api/v1/applications/delete-with-releases-app", "test-admin-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var errorResp models.ErrorResponse
	err := json.NewDecoder(resp.Body).Decode(&errorResp)
	require.NoError(t, err)
	assert.Equal(t, "CONFLICT", errorResp.Code)

	// Step 4: Delete the release
	resp = doRequest(t, "DELETE", server.URL+"/api/v1/updates/delete-with-releases-app/releases/1.0.0/windows/amd64", "test-admin-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Step 5: Delete the application now that releases are gone -- expect 204
	resp = doRequest(t, "DELETE", server.URL+"/api/v1/applications/delete-with-releases-app", "test-admin-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Verify the application is gone
	resp = doRequest(t, "GET", server.URL+"/api/v1/applications/delete-with-releases-app", "test-read-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestApplicationPermissions(t *testing.T) {
	server, _ := setupTestServer(t, true)

	// First, create an application with admin key for permission testing
	createReq := models.CreateApplicationRequest{
		ID:        "permissions-test-app",
		Name:      "Permissions Test App",
		Platforms: []string{"windows"},
		Config:    models.ApplicationConfig{},
	}

	resp := doRequest(t, "POST", server.URL+"/api/v1/applications", "test-admin-key", createReq)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Test 1: Read-only key cannot create applications (expect 403)
	newAppReq := models.CreateApplicationRequest{
		ID:        "should-not-exist",
		Name:      "Should Not Exist",
		Platforms: []string{"windows"},
		Config:    models.ApplicationConfig{},
	}

	resp = doRequest(t, "POST", server.URL+"/api/v1/applications", "test-read-key", newAppReq)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Verify the application was not created
	resp = doRequest(t, "GET", server.URL+"/api/v1/applications/should-not-exist", "test-admin-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Test 2: Write key can create but cannot delete (expect 403 on delete)
	writeCreateReq := models.CreateApplicationRequest{
		ID:        "write-created-app",
		Name:      "Write Created App",
		Platforms: []string{"linux"},
		Config:    models.ApplicationConfig{},
	}

	resp = doRequest(t, "POST", server.URL+"/api/v1/applications", "test-write-key", writeCreateReq)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Write key cannot delete
	resp = doRequest(t, "DELETE", server.URL+"/api/v1/applications/write-created-app", "test-write-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Write key cannot update (requires admin)
	updatedName := "Updated by Write"
	updateReq := models.UpdateApplicationRequest{
		Name: &updatedName,
	}
	resp = doRequest(t, "PUT", server.URL+"/api/v1/applications/write-created-app", "test-write-key", updateReq)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Test 3: Admin key can do everything
	// Admin can read
	resp = doRequest(t, "GET", server.URL+"/api/v1/applications/permissions-test-app", "test-admin-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Admin can list
	resp = doRequest(t, "GET", server.URL+"/api/v1/applications", "test-admin-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Admin can update
	adminUpdateName := "Updated by Admin"
	adminUpdateReq := models.UpdateApplicationRequest{
		Name: &adminUpdateName,
	}
	resp = doRequest(t, "PUT", server.URL+"/api/v1/applications/permissions-test-app", "test-admin-key", adminUpdateReq)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Admin can delete
	resp = doRequest(t, "DELETE", server.URL+"/api/v1/applications/permissions-test-app", "test-admin-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Clean up write-created-app with admin key
	resp = doRequest(t, "DELETE", server.URL+"/api/v1/applications/write-created-app", "test-admin-key", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Test 4: No auth header at all returns 401
	resp = doRequest(t, "GET", server.URL+"/api/v1/applications", "", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
