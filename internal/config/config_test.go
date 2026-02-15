package config

import (
	"os"
	"path/filepath"
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
  cors:
    enabled: true
    allowed_origins: ["*"]
    allowed_methods: ["GET", "POST"]
    allowed_headers: ["Content-Type"]
    max_age: 3600

storage:
  type: "json"
  path: "./data/test.json"

security:
  enable_auth: true
  jwt_secret: "test-secret"
  api_keys:
    - key: "test-key"
      name: "Test Key"
      permissions: ["read", "write"]
      enabled: true
  rate_limit:
    enabled: true
    requests_per_minute: 100
    burst_size: 10
    cleanup_interval: 300s

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

	// Verify CORS config
	assert.True(t, config.Server.CORS.Enabled)
	assert.Equal(t, []string{"*"}, config.Server.CORS.AllowedOrigins)
	assert.Equal(t, []string{"GET", "POST"}, config.Server.CORS.AllowedMethods)
	assert.Equal(t, []string{"Content-Type"}, config.Server.CORS.AllowedHeaders)
	assert.Equal(t, 3600, config.Server.CORS.MaxAge)

	// Verify storage config
	assert.Equal(t, "json", config.Storage.Type)
	assert.Equal(t, "./data/test.json", config.Storage.Path)

	// Verify security config
	assert.True(t, config.Security.EnableAuth)
	assert.Equal(t, "test-secret", config.Security.JWTSecret)
	require.Len(t, config.Security.APIKeys, 1)
	assert.Equal(t, "test-key", config.Security.APIKeys[0].Key)
	assert.Equal(t, "Test Key", config.Security.APIKeys[0].Name)
	assert.Equal(t, []string{"read", "write"}, config.Security.APIKeys[0].Permissions)
	assert.True(t, config.Security.APIKeys[0].Enabled)

	// Verify rate limiting config
	assert.True(t, config.Security.RateLimit.Enabled)
	assert.Equal(t, 100, config.Security.RateLimit.RequestsPerMinute)
	assert.Equal(t, 10, config.Security.RateLimit.BurstSize)
	assert.Equal(t, 300*time.Second, config.Security.RateLimit.CleanupInterval)

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
	assert.Empty(t, config.Security.JWTSecret)
	assert.Empty(t, config.Security.APIKeys)

	// Rate limiting defaults
	assert.True(t, config.Security.RateLimit.Enabled)                // Default
	assert.Equal(t, 60, config.Security.RateLimit.RequestsPerMinute) // Default

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
		"UPDATER_PORT":         os.Getenv("UPDATER_PORT"),
		"UPDATER_HOST":         os.Getenv("UPDATER_HOST"),
		"UPDATER_STORAGE_TYPE": os.Getenv("UPDATER_STORAGE_TYPE"),
		"UPDATER_STORAGE_PATH": os.Getenv("UPDATER_STORAGE_PATH"),
		"UPDATER_ENABLE_AUTH":  os.Getenv("UPDATER_ENABLE_AUTH"),
		"UPDATER_LOG_LEVEL":    os.Getenv("UPDATER_LOG_LEVEL"),
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

func TestLoad_WithComplexAPIKeys(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "api_keys_config.yaml")

	configContent := `
server:
  port: 8080

storage:
  type: "json"
  path: "./data.json"

security:
  enable_auth: true
  jwt_secret: "complex-jwt-secret-123"
  api_keys:
    - key: "admin-key-12345"
      name: "Admin Key"
      permissions: ["read", "write", "delete"]
      enabled: true
    - key: "read-only-key-67890"
      name: "Read Only Key"
      permissions: ["read"]
      enabled: true
    - key: "disabled-key-abcdef"
      name: "Disabled Key"
      permissions: ["read", "write"]
      enabled: false
  trusted_proxies:
    - "10.0.0.0/8"
    - "172.16.0.0/12"
    - "192.168.0.0/16"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	assert.True(t, config.Security.EnableAuth)
	assert.Equal(t, "complex-jwt-secret-123", config.Security.JWTSecret)
	require.Len(t, config.Security.APIKeys, 3)

	// Check first API key (admin)
	assert.Equal(t, "admin-key-12345", config.Security.APIKeys[0].Key)
	assert.Equal(t, "Admin Key", config.Security.APIKeys[0].Name)
	assert.Equal(t, []string{"read", "write", "delete"}, config.Security.APIKeys[0].Permissions)
	assert.True(t, config.Security.APIKeys[0].Enabled)

	// Check second API key (read-only)
	assert.Equal(t, "read-only-key-67890", config.Security.APIKeys[1].Key)
	assert.Equal(t, "Read Only Key", config.Security.APIKeys[1].Name)
	assert.Equal(t, []string{"read"}, config.Security.APIKeys[1].Permissions)
	assert.True(t, config.Security.APIKeys[1].Enabled)

	// Check third API key (disabled)
	assert.Equal(t, "disabled-key-abcdef", config.Security.APIKeys[2].Key)
	assert.Equal(t, "Disabled Key", config.Security.APIKeys[2].Name)
	assert.Equal(t, []string{"read", "write"}, config.Security.APIKeys[2].Permissions)
	assert.False(t, config.Security.APIKeys[2].Enabled)

	// Check trusted proxies
	assert.Equal(t, []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}, config.Security.TrustedProxies)
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
