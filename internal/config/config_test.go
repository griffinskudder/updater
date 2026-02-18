package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"updater/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_WithValidConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
server:
  port: 8080
  host: "localhost"
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s
  tls_enabled: false
storage:
  type: "json"
  path: "./data/test.json"

security:
  enable_auth: true
  bootstrap_key: "upd_test-bootstrap-key-here"

logging:
  level: "debug"
  format: "json"
  output: "stdout"
  max_size: 50
  max_backups: 5
  max_age: 30
  compress: true

cache:
  enabled: true
  type: "memory"
  ttl: 600s
  memory:
    max_size: 500
    cleanup_interval: 300s

metrics:
  enabled: true
  path: "/metrics"
  port: 9090
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	// Verify server config
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "localhost", config.Server.Host)
	assert.Equal(t, 30*time.Second, config.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, config.Server.WriteTimeout)
	assert.Equal(t, 60*time.Second, config.Server.IdleTimeout)
	assert.False(t, config.Server.TLSEnabled)

	// Verify storage config
	assert.Equal(t, "json", config.Storage.Type)
	assert.Equal(t, "./data/test.json", config.Storage.Path)

	// Verify security config
	assert.True(t, config.Security.EnableAuth)
	assert.Equal(t, "upd_test-bootstrap-key-here", config.Security.BootstrapKey)

	// Verify logging config
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "stdout", config.Logging.Output)
	assert.Equal(t, 50, config.Logging.MaxSize)
	assert.Equal(t, 5, config.Logging.MaxBackups)
	assert.Equal(t, 30, config.Logging.MaxAge)
	assert.True(t, config.Logging.Compress)

	// Verify cache config
	assert.True(t, config.Cache.Enabled)
	assert.Equal(t, "memory", config.Cache.Type)
	assert.Equal(t, 600*time.Second, config.Cache.TTL)
	assert.Equal(t, 500, config.Cache.Memory.MaxSize)
	assert.Equal(t, 300*time.Second, config.Cache.Memory.CleanupInterval)

	// Verify metrics config
	assert.True(t, config.Metrics.Enabled)
	assert.Equal(t, "/metrics", config.Metrics.Path)
	assert.Equal(t, 9090, config.Metrics.Port)
}

func TestLoad_WithDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "minimal_config.yaml")

	// Minimal config file
	configContent := `
server:
  port: 3000

storage:
  type: "json"
  path: "./test.json"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	// Verify defaults are applied
	assert.Equal(t, 3000, config.Server.Port)
	assert.Equal(t, "0.0.0.0", config.Server.Host)              // Default
	assert.Equal(t, 30*time.Second, config.Server.ReadTimeout)  // Default
	assert.Equal(t, 30*time.Second, config.Server.WriteTimeout) // Default
	assert.Equal(t, 60*time.Second, config.Server.IdleTimeout)  // Default
	assert.False(t, config.Server.TLSEnabled)                   // Default

	// Storage config should be as specified
	assert.Equal(t, "json", config.Storage.Type)
	assert.Equal(t, "./test.json", config.Storage.Path)

	// Security defaults
	assert.False(t, config.Security.EnableAuth) // Default
	assert.Empty(t, config.Security.BootstrapKey)

	// Logging defaults
	assert.Equal(t, "info", config.Logging.Level)    // Default
	assert.Equal(t, "json", config.Logging.Format)   // Default
	assert.Equal(t, "stdout", config.Logging.Output) // Default

	// Cache defaults
	assert.True(t, config.Cache.Enabled)               // Default
	assert.Equal(t, "memory", config.Cache.Type)       // Default
	assert.Equal(t, 300*time.Second, config.Cache.TTL) // Default

	// Metrics defaults
	assert.True(t, config.Metrics.Enabled)           // Default
	assert.Equal(t, "/metrics", config.Metrics.Path) // Default
	assert.Equal(t, 9090, config.Metrics.Port)       // Default
}

func TestLoad_WithEnvironmentVariables(t *testing.T) {
	// Set environment variables
	originalEnv := map[string]string{
		"UPDATER_PORT":          os.Getenv("UPDATER_PORT"),
		"UPDATER_HOST":          os.Getenv("UPDATER_HOST"),
		"UPDATER_STORAGE_TYPE":  os.Getenv("UPDATER_STORAGE_TYPE"),
		"UPDATER_STORAGE_PATH":  os.Getenv("UPDATER_STORAGE_PATH"),
		"UPDATER_ENABLE_AUTH":   os.Getenv("UPDATER_ENABLE_AUTH"),
		"UPDATER_BOOTSTRAP_KEY": os.Getenv("UPDATER_BOOTSTRAP_KEY"),
		"UPDATER_LOG_LEVEL":     os.Getenv("UPDATER_LOG_LEVEL"),
	}

	// Clean up after test
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Set test environment variables
	os.Setenv("UPDATER_PORT", "9999")
	os.Setenv("UPDATER_HOST", "127.0.0.1")
	os.Setenv("UPDATER_STORAGE_TYPE", "memory")
	os.Setenv("UPDATER_STORAGE_PATH", "/tmp/test.json")
	os.Setenv("UPDATER_ENABLE_AUTH", "true")
	os.Setenv("UPDATER_BOOTSTRAP_KEY", "upd_test-env-bootstrap-key")
	os.Setenv("UPDATER_LOG_LEVEL", "warn")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "env_config.yaml")

	// Config file with different values (should be overridden by env vars)
	configContent := `
server:
  port: 8080
  host: "localhost"

storage:
  type: "json"
  path: "./data.json"

security:
  enable_auth: false

logging:
  level: "info"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	// Environment variables should override config file values
	assert.Equal(t, 9999, config.Server.Port)
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, "memory", config.Storage.Type)
	assert.Equal(t, "/tmp/test.json", config.Storage.Path)
	assert.True(t, config.Security.EnableAuth)
	assert.Equal(t, "warn", config.Logging.Level)
}

func TestLoad_NonExistentFile(t *testing.T) {
	_, err := Load("/non/existent/path.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestLoad_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid.yaml")

	// Invalid YAML content
	invalidContent := `
server:
  port: 8080
  invalid: [unclosed array
`

	err := os.WriteFile(configFile, []byte(invalidContent), 0644)
	require.NoError(t, err)

	_, err = Load(configFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML config")
}

func TestLoad_EmptyConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "empty.yaml")

	err := os.WriteFile(configFile, []byte(""), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	// Should have all defaults applied
	assert.Equal(t, 8080, config.Server.Port)                // Default
	assert.Equal(t, "0.0.0.0", config.Server.Host)           // Default
	assert.Equal(t, "json", config.Storage.Type)             // Default
	assert.Contains(t, config.Storage.Path, "releases.json") // Default
}

func TestLoad_WithTLSConfig(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "tls_config.yaml")

	configContent := `
server:
  port: 8443
  tls_enabled: true
  tls_cert_file: "/path/to/cert.pem"
  tls_key_file: "/path/to/key.pem"

storage:
  type: "json"
  path: "./data.json"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	assert.Equal(t, 8443, config.Server.Port)
	assert.True(t, config.Server.TLSEnabled)
	assert.Equal(t, "/path/to/cert.pem", config.Server.TLSCertFile)
	assert.Equal(t, "/path/to/key.pem", config.Server.TLSKeyFile)
}

func TestLoad_WithDatabaseConfig(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "db_config.yaml")

	configContent := `
server:
  port: 8080

storage:
  type: "postgres"
  path: ""
  database:
    driver: "postgres"
    dsn: "postgres://user:pass@localhost/updater"
    max_open_conns: 50
    max_idle_conns: 10
    conn_max_lifetime: 600s
    conn_max_idle_time: 120s
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	assert.Equal(t, "postgres", config.Storage.Type)
	assert.Equal(t, "postgres", config.Storage.Database.Driver)
	assert.Equal(t, "postgres://user:pass@localhost/updater", config.Storage.Database.DSN)
	assert.Equal(t, 50, config.Storage.Database.MaxOpenConns)
	assert.Equal(t, 10, config.Storage.Database.MaxIdleConns)
	assert.Equal(t, 600*time.Second, config.Storage.Database.ConnMaxLifetime)
	assert.Equal(t, 120*time.Second, config.Storage.Database.ConnMaxIdleTime)
}

func TestLoad_WithRedisCache(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "redis_config.yaml")

	configContent := `
server:
  port: 8080

storage:
  type: "json"
  path: "./data.json"

cache:
  enabled: true
  type: "redis"
  ttl: 1200s
  redis:
    addr: "localhost:6379"
    password: "secret"
    db: 1
    pool_size: 20
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	assert.True(t, config.Cache.Enabled)
	assert.Equal(t, "redis", config.Cache.Type)
	assert.Equal(t, 1200*time.Second, config.Cache.TTL)
	assert.Equal(t, "localhost:6379", config.Cache.Redis.Addr)
	assert.Equal(t, "secret", config.Cache.Redis.Password)
	assert.Equal(t, 1, config.Cache.Redis.DB)
	assert.Equal(t, 20, config.Cache.Redis.PoolSize)
}

func TestLoad_WithBootstrapKey(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "bootstrap_key_config.yaml")

	configContent := `
server:
  port: 8080

storage:
  type: "json"
  path: "./data.json"

security:
  enable_auth: true
  bootstrap_key: "upd_my-bootstrap-key-abc123"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	assert.True(t, config.Security.EnableAuth)
	assert.Equal(t, "upd_my-bootstrap-key-abc123", config.Security.BootstrapKey)
}

func TestLoad_WithFileLogging(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "file_logging_config.yaml")

	configContent := `
server:
  port: 8080

storage:
  type: "json"
  path: "./data.json"

logging:
  level: "error"
  format: "text"
  output: "file"
  file_path: "/var/log/updater.log"
  max_size: 200
  max_backups: 10
  max_age: 60
  compress: false
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	assert.Equal(t, "error", config.Logging.Level)
	assert.Equal(t, "text", config.Logging.Format)
	assert.Equal(t, "file", config.Logging.Output)
	assert.Equal(t, "/var/log/updater.log", config.Logging.FilePath)
	assert.Equal(t, 200, config.Logging.MaxSize)
	assert.Equal(t, 10, config.Logging.MaxBackups)
	assert.Equal(t, 60, config.Logging.MaxAge)
	assert.False(t, config.Logging.Compress)
}

func TestValidate_ValidConfig(t *testing.T) {
	config := &models.Config{
		Server: models.ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Storage: models.StorageConfig{
			Type: "json",
			Path: "./test.json",
		},
		Logging: models.LoggingConfig{
			Level:  "error",
			Format: "text",
			Output: "stdout",
		},
	}

	err := config.Validate()
	assert.NoError(t, err)
}

func TestValidate_InvalidPort(t *testing.T) {
	config := &models.Config{
		Server: models.ServerConfig{
			Port: 0, // Invalid port
			Host: "localhost",
		},
		Storage: models.StorageConfig{
			Type: "json",
			Path: "./test.json",
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port must be between 1 and 65535")
}

func TestValidate_EmptyStorageType(t *testing.T) {
	config := &models.Config{
		Server: models.ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Storage: models.StorageConfig{
			Type: "", // Empty storage type
			Path: "./test.json",
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid storage type")
}

func TestValidate_TLSEnabledWithoutCerts(t *testing.T) {
	config := &models.Config{
		Server: models.ServerConfig{
			Port:       8443,
			Host:       "localhost",
			TLSEnabled: true,
			// Missing TLSCertFile and TLSKeyFile
		},
		Storage: models.StorageConfig{
			Type: "json",
			Path: "./test.json",
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TLS cert file is required when TLS is enabled")
}

func TestWarnDeprecatedKeys(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantWarn []string
	}{
		{
			name:     "no deprecated keys",
			yaml:     "server:\n  port: 8080\n",
			wantWarn: nil,
		},
		{
			name:     "cors key warns",
			yaml:     "server:\n  cors:\n    enabled: true\n",
			wantWarn: []string{"server.cors"},
		},
		{
			name:     "rate_limit key warns",
			yaml:     "security:\n  rate_limit:\n    enabled: true\n",
			wantWarn: []string{"security.rate_limit"},
		},
		{
			name:     "jwt_secret key warns",
			yaml:     "security:\n  jwt_secret: \"abc\"\n",
			wantWarn: []string{"security.jwt_secret"},
		},
		{
			name:     "trusted_proxies key warns",
			yaml:     "security:\n  trusted_proxies:\n    - \"10.0.0.1\"\n",
			wantWarn: []string{"security.trusted_proxies"},
		},
		{
			name: "all deprecated keys warn",
			yaml: "server:\n  cors:\n    enabled: true\nsecurity:\n  jwt_secret: \"x\"\n  rate_limit:\n    enabled: true\n  trusted_proxies: []\n",
			wantWarn: []string{"server.cors", "security.jwt_secret", "security.rate_limit", "security.trusted_proxies"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var warnings []string
			h := &capturingSlogHandler{warned: &warnings}
			orig := slog.Default()
			slog.SetDefault(slog.New(h))
			defer slog.SetDefault(orig)

			warnDeprecatedKeys([]byte(tt.yaml))

			if len(tt.wantWarn) == 0 && len(warnings) > 0 {
				t.Errorf("expected no warnings, got %v", warnings)
			}
			for _, want := range tt.wantWarn {
				found := false
				for _, w := range warnings {
					if strings.Contains(w, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected warning containing %q, got %v", want, warnings)
				}
			}
		})
	}
}

// capturingSlogHandler captures Warn-level log messages for testing.
type capturingSlogHandler struct {
	warned *[]string
}

func (h *capturingSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

func (h *capturingSlogHandler) Handle(_ context.Context, r slog.Record) error {
	*h.warned = append(*h.warned, r.Message)
	return nil
}

func (h *capturingSlogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *capturingSlogHandler) WithGroup(_ string) slog.Handler      { return h }
