package observability

import (
	"context"
	"fmt"
	"testing"
	"time"
	"updater/internal/models"
	"updater/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestProvider(t *testing.T) *Provider {
	t.Helper()
	metrics := models.MetricsConfig{Enabled: true, Path: "/metrics", Port: 9090}
	obs := models.ObservabilityConfig{
		ServiceName:    "test",
		ServiceVersion: "1.0.0",
		Tracing: models.TracingConfig{
			Enabled:    true,
			Exporter:   "stdout",
			SampleRate: 1.0,
		},
	}
	provider, err := Setup(metrics, obs)
	require.NoError(t, err)
	t.Cleanup(func() { provider.Shutdown(context.Background()) })
	return provider
}

func setupMemoryStorage(t *testing.T) storage.Storage {
	t.Helper()
	s, err := storage.NewMemoryStorage(storage.Config{Type: "memory"})
	require.NoError(t, err)
	return s
}

func TestNewInstrumentedStorage(t *testing.T) {
	_ = setupTestProvider(t)
	inner := setupMemoryStorage(t)

	instrumented, err := NewInstrumentedStorage(inner)
	require.NoError(t, err)
	assert.NotNil(t, instrumented)
}

func TestInstrumentedStorage_Ping(t *testing.T) {
	_ = setupTestProvider(t)
	inner := setupMemoryStorage(t)

	instrumented, err := NewInstrumentedStorage(inner)
	require.NoError(t, err)

	err = instrumented.Ping(context.Background())
	assert.NoError(t, err)
}

func TestInstrumentedStorage_ApplicationOperations(t *testing.T) {
	_ = setupTestProvider(t)
	inner := setupMemoryStorage(t)

	instrumented, err := NewInstrumentedStorage(inner)
	require.NoError(t, err)

	ctx := context.Background()

	// SaveApplication
	app := &models.Application{
		ID:   "test-app",
		Name: "Test App",
	}
	err = instrumented.SaveApplication(ctx, app)
	assert.NoError(t, err)

	// GetApplication
	result, err := instrumented.GetApplication(ctx, "test-app")
	assert.NoError(t, err)
	assert.Equal(t, "test-app", result.ID)

	// Applications
	apps, err := instrumented.Applications(ctx)
	assert.NoError(t, err)
	assert.Len(t, apps, 1)
}

func TestInstrumentedStorage_DeleteApplication(t *testing.T) {
	_ = setupTestProvider(t)
	inner := setupMemoryStorage(t)

	instrumented, err := NewInstrumentedStorage(inner)
	require.NoError(t, err)

	ctx := context.Background()

	// Save an application first
	app := &models.Application{
		ID:   "del-app",
		Name: "Delete App",
	}
	err = instrumented.SaveApplication(ctx, app)
	require.NoError(t, err)

	// Delete it
	err = instrumented.DeleteApplication(ctx, "del-app")
	assert.NoError(t, err)

	// Verify it's gone
	_, err = instrumented.GetApplication(ctx, "del-app")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Delete non-existent should error
	err = instrumented.DeleteApplication(ctx, "non-existent")
	assert.Error(t, err)
}

func TestInstrumentedStorage_ReleaseOperations(t *testing.T) {
	_ = setupTestProvider(t)
	inner := setupMemoryStorage(t)

	instrumented, err := NewInstrumentedStorage(inner)
	require.NoError(t, err)

	ctx := context.Background()

	// SaveRelease
	release := &models.Release{
		ID:            "test-release",
		ApplicationID: "test-app",
		Version:       "1.0.0",
		Platform:      "windows",
		Architecture:  "amd64",
		DownloadURL:   "https://example.com/v1.0.0/app.exe",
		ReleaseDate:   time.Now(),
	}
	err = instrumented.SaveRelease(ctx, release)
	assert.NoError(t, err)

	// GetRelease
	result, err := instrumented.GetRelease(ctx, "test-app", "1.0.0", "windows", "amd64")
	assert.NoError(t, err)
	assert.Equal(t, "1.0.0", result.Version)

	// Releases
	releases, err := instrumented.Releases(ctx, "test-app")
	assert.NoError(t, err)
	assert.Len(t, releases, 1)

	// GetLatestRelease
	latest, err := instrumented.GetLatestRelease(ctx, "test-app", "windows", "amd64")
	assert.NoError(t, err)
	assert.Equal(t, "1.0.0", latest.Version)

	// Save a newer release
	release2 := &models.Release{
		ID:            "test-release-2",
		ApplicationID: "test-app",
		Version:       "2.0.0",
		Platform:      "windows",
		Architecture:  "amd64",
		DownloadURL:   "https://example.com/v2.0.0/app.exe",
		ReleaseDate:   time.Now(),
	}
	err = instrumented.SaveRelease(ctx, release2)
	assert.NoError(t, err)

	// GetReleasesAfterVersion
	newer, err := instrumented.GetReleasesAfterVersion(ctx, "test-app", "1.0.0", "windows", "amd64")
	assert.NoError(t, err)
	assert.Len(t, newer, 1)
	assert.Equal(t, "2.0.0", newer[0].Version)

	// DeleteRelease
	err = instrumented.DeleteRelease(ctx, "test-app", "2.0.0", "windows", "amd64")
	assert.NoError(t, err)
}

func TestInstrumentedStorage_ErrorRecording(t *testing.T) {
	_ = setupTestProvider(t)
	inner := setupMemoryStorage(t)

	instrumented, err := NewInstrumentedStorage(inner)
	require.NoError(t, err)

	ctx := context.Background()

	// GetApplication for non-existent app should record error span
	_, err = instrumented.GetApplication(ctx, "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInstrumentedStorage_Close(t *testing.T) {
	_ = setupTestProvider(t)
	inner := setupMemoryStorage(t)

	instrumented, err := NewInstrumentedStorage(inner)
	require.NoError(t, err)

	err = instrumented.Close()
	assert.NoError(t, err)
}

func TestInstrumentedStorage_ImplementsInterface(t *testing.T) {
	_ = setupTestProvider(t)
	inner := setupMemoryStorage(t)

	instrumented, err := NewInstrumentedStorage(inner)
	require.NoError(t, err)

	// Verify it implements storage.Storage
	var _ storage.Storage = instrumented
	_ = fmt.Sprintf("%T", instrumented) // avoid unused variable
}

func TestInstrumentedStorage_APIKeyMethods(t *testing.T) {
	_ = setupTestProvider(t)
	inner, err := storage.NewMemoryStorage(storage.Config{})
	require.NoError(t, err)
	s, err := NewInstrumentedStorage(inner)
	require.NoError(t, err)
	ctx := context.Background()

	raw, err := models.GenerateAPIKey()
	require.NoError(t, err)
	key := models.NewAPIKey(models.NewKeyID(), "test", raw, []string{"read"})

	assert.NoError(t, s.CreateAPIKey(ctx, key))
	_, err = s.GetAPIKeyByHash(ctx, key.KeyHash)
	assert.NoError(t, err)
	_, err = s.ListAPIKeys(ctx)
	assert.NoError(t, err)
	key.Name = "test2"
	assert.NoError(t, s.UpdateAPIKey(ctx, key))
	assert.NoError(t, s.DeleteAPIKey(ctx, key.ID))
}
