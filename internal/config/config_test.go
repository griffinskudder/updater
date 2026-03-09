package config

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
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
storage:
  type: "sqlite"
  database:
    dsn: ":memory:"

security:
  enable_auth: true
  bootstrap_key: "upd_test-bootstrap-key-here"

logging:
  level: "debug"
  format: "json"
  output: "stdout"

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
	assert.Equal(t, "sqlite", config.Storage.Type)
	assert.Equal(t, ":memory:", config.Storage.Database.DSN)

	// Verify security config
	assert.True(t, config.Security.EnableAuth)
	assert.Equal(t, "upd_test-bootstrap-key-here", config.Security.BootstrapKey)

	// Verify logging config
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "stdout", config.Logging.Output)

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
  type: "memory"
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
	assert.Equal(t, "memory", config.Storage.Type)

	// Security defaults
	assert.False(t, config.Security.EnableAuth) // Default
	assert.Empty(t, config.Security.BootstrapKey)

	// Logging defaults
	assert.Equal(t, "info", config.Logging.Level)    // Default
	assert.Equal(t, "json", config.Logging.Format)   // Default
	assert.Equal(t, "stdout", config.Logging.Output) // Default

	// Metrics defaults
	assert.True(t, config.Metrics.Enabled)           // Default
	assert.Equal(t, "/metrics", config.Metrics.Path) // Default
	assert.Equal(t, 9090, config.Metrics.Port)       // Default
}

func TestLoad_WithEnvironmentVariables(t *testing.T) {
	// Set environment variables
	originalEnv := map[string]string{
		"UPDATER_PORT":             os.Getenv("UPDATER_PORT"),
		"UPDATER_HOST":             os.Getenv("UPDATER_HOST"),
		"UPDATER_STORAGE_TYPE":     os.Getenv("UPDATER_STORAGE_TYPE"),
		"UPDATER_STORAGE_PATH":     os.Getenv("UPDATER_STORAGE_PATH"),
		"UPDATER_ENABLE_AUTH":      os.Getenv("UPDATER_ENABLE_AUTH"),
		"UPDATER_BOOTSTRAP_KEY":    os.Getenv("UPDATER_BOOTSTRAP_KEY"),
		"UPDATER_LOG_LEVEL":        os.Getenv("UPDATER_LOG_LEVEL"),
		"UPDATER_SHUTDOWN_TIMEOUT": os.Getenv("UPDATER_SHUTDOWN_TIMEOUT"),
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
	os.Setenv("UPDATER_SHUTDOWN_TIMEOUT", "45s")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "env_config.yaml")

	// Config file with different values (should be overridden by env vars)
	configContent := `
server:
  port: 8080
  host: "localhost"

storage:
  type: "memory"

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
	assert.Equal(t, 45*time.Second, config.Server.ShutdownTimeout)
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
	assert.Equal(t, 8080, config.Server.Port)             // Default
	assert.Equal(t, "0.0.0.0", config.Server.Host)        // Default
	assert.Equal(t, "sqlite", config.Storage.Type)        // Default
	assert.Contains(t, config.Storage.Path, "updater.db") // Default
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
  type: "memory"
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

func TestLoad_WithBootstrapKey(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "bootstrap_key_config.yaml")

	configContent := `
server:
  port: 8080

storage:
  type: "memory"

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
  type: "memory"

logging:
  level: "error"
  format: "text"
  output: "file"
  file_path: "/var/log/updater.log"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	assert.Equal(t, "error", config.Logging.Level)
	assert.Equal(t, "text", config.Logging.Format)
	assert.Equal(t, "file", config.Logging.Output)
	assert.Equal(t, "/var/log/updater.log", config.Logging.FilePath)
}

func TestValidate_ValidConfig(t *testing.T) {
	config := &models.Config{
		Server: models.ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Storage: models.StorageConfig{
			Type: "memory",
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
			Type: "memory",
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
			Type: "memory",
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
			name:     "cache key warns",
			yaml:     "cache:\n  enabled: true\n",
			wantWarn: []string{"cache"},
		},
		{
			name:     "all deprecated keys warn",
			yaml:     "server:\n  cors:\n    enabled: true\nsecurity:\n  jwt_secret: \"x\"\n  rate_limit:\n    enabled: true\n  trusted_proxies: []\ncache:\n  enabled: true\n",
			wantWarn: []string{"server.cors", "security.jwt_secret", "security.rate_limit", "security.trusted_proxies", "cache"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var records []capturedLog
			h := &capturingSlogHandler{records: &records}
			orig := slog.Default()
			slog.SetDefault(slog.New(h))
			defer slog.SetDefault(orig)

			warnDeprecatedKeys([]byte(tt.yaml))

			if len(tt.wantWarn) == 0 && len(records) > 0 {
				t.Errorf("expected no warnings, got %v", records)
			}
			for _, want := range tt.wantWarn {
				found := false
				for _, rec := range records {
					if rec.configKey == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected warning with config_key=%q, got %v", want, records)
				}
			}
		})
	}
}

// capturedLog holds a single captured Warn-level log record for testing.
type capturedLog struct {
	msg       string
	configKey string
}

// capturingSlogHandler captures Warn-level log records (message + config_key attribute) for testing.
type capturingSlogHandler struct {
	records *[]capturedLog
}

func (h *capturingSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

func (h *capturingSlogHandler) Handle(_ context.Context, r slog.Record) error {
	log := capturedLog{msg: r.Message}
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "config_key" {
			log.configKey = a.Value.String()
		}
		return true
	})
	*h.records = append(*h.records, log)
	return nil
}

func (h *capturingSlogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *capturingSlogHandler) WithGroup(_ string) slog.Handler      { return h }

// testGenerateSelfSignedCert creates a temporary self-signed ECDSA cert/key pair
// and writes them to files in a temp directory. Returns the file paths.
func testGenerateSelfSignedCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	var certBuf bytes.Buffer
	require.NoError(t, pem.Encode(&certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}))

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	var keyBuf bytes.Buffer
	require.NoError(t, pem.Encode(&keyBuf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))

	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, certBuf.Bytes(), 0600))
	require.NoError(t, os.WriteFile(keyPath, keyBuf.Bytes(), 0600))

	return certPath, keyPath
}

func TestValidateRuntime_TLSDisabled(t *testing.T) {
	cfg := models.NewDefaultConfig()
	cfg.Storage.Type = "memory"
	err := ValidateRuntime(cfg)
	assert.NoError(t, err)
}

func TestValidateRuntime_TLS_ValidPair(t *testing.T) {
	certPath, keyPath := testGenerateSelfSignedCert(t)

	cfg := models.NewDefaultConfig()
	cfg.Storage.Type = "memory"
	cfg.Server.TLSEnabled = true
	cfg.Server.TLSCertFile = certPath
	cfg.Server.TLSKeyFile = keyPath

	err := ValidateRuntime(cfg)
	assert.NoError(t, err)
}

func TestValidateRuntime_TLS_CertFileMissing(t *testing.T) {
	_, keyPath := testGenerateSelfSignedCert(t)

	cfg := models.NewDefaultConfig()
	cfg.Storage.Type = "memory"
	cfg.Server.TLSEnabled = true
	cfg.Server.TLSCertFile = "/nonexistent/cert.pem"
	cfg.Server.TLSKeyFile = keyPath

	err := ValidateRuntime(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read TLS cert file")
}

func TestValidateRuntime_TLS_KeyFileMissing(t *testing.T) {
	certPath, _ := testGenerateSelfSignedCert(t)

	cfg := models.NewDefaultConfig()
	cfg.Storage.Type = "memory"
	cfg.Server.TLSEnabled = true
	cfg.Server.TLSCertFile = certPath
	cfg.Server.TLSKeyFile = "/nonexistent/key.pem"

	err := ValidateRuntime(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read TLS key file")
}

func TestValidateRuntime_TLS_BothFilesMissing_BothReported(t *testing.T) {
	cfg := models.NewDefaultConfig()
	cfg.Storage.Type = "memory"
	cfg.Server.TLSEnabled = true
	cfg.Server.TLSCertFile = "/nonexistent/cert.pem"
	cfg.Server.TLSKeyFile = "/nonexistent/key.pem"

	err := ValidateRuntime(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read TLS cert file")
	assert.Contains(t, err.Error(), "cannot read TLS key file")
}

func TestValidateRuntime_TLS_MismatchedPair(t *testing.T) {
	certPath1, _ := testGenerateSelfSignedCert(t)
	_, keyPath2 := testGenerateSelfSignedCert(t)

	cfg := models.NewDefaultConfig()
	cfg.Storage.Type = "memory"
	cfg.Server.TLSEnabled = true
	cfg.Server.TLSCertFile = certPath1
	cfg.Server.TLSKeyFile = keyPath2

	err := ValidateRuntime(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid TLS cert/key pair")
}

func TestValidateRuntime_LogDir_NotFile(t *testing.T) {
	cfg := models.NewDefaultConfig()
	cfg.Storage.Type = "memory"
	// Default output is stdout, not file — no log-dir check.
	err := ValidateRuntime(cfg)
	assert.NoError(t, err)
}

func TestValidateRuntime_LogDir_WritableDir(t *testing.T) {
	dir := t.TempDir()

	cfg := models.NewDefaultConfig()
	cfg.Storage.Type = "memory"
	cfg.Logging.Output = "file"
	cfg.Logging.FilePath = filepath.Join(dir, "updater.log")

	err := ValidateRuntime(cfg)
	assert.NoError(t, err)
}

func TestValidateRuntime_LogDir_NonExistentDir(t *testing.T) {
	cfg := models.NewDefaultConfig()
	cfg.Storage.Type = "memory"
	cfg.Logging.Output = "file"
	// Use a path under t.TempDir() with a subdirectory that is never created.
	cfg.Logging.FilePath = filepath.Join(t.TempDir(), "doesnotexist", "updater.log")

	err := ValidateRuntime(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "log directory does not exist or is not accessible")
}

func TestValidateConfig_AllPass(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	require.NoError(t, os.WriteFile(configFile, []byte(`
storage:
  type: "memory"
`), 0644))

	results := ValidateConfig(configFile)
	for _, r := range results {
		assert.True(t, r.OK, "expected check %q to pass, got: %s", r.Name, r.Message)
	}
}

func TestValidateConfig_EmptyPath_UsesDefaults(t *testing.T) {
	results := ValidateConfig("")
	require.NotEmpty(t, results)
	for _, r := range results {
		assert.True(t, r.OK, "expected check %q to pass, got: %s", r.Name, r.Message)
	}
	names := make(map[string]bool, len(results))
	for _, r := range results {
		names[r.Name] = true
	}
	for _, want := range []string{
		"config.server", "config.storage", "config.security",
		"config.logging", "config.metrics", "config.observability",
		"config.cross-field", "runtime.tls", "runtime.log-dir",
	} {
		assert.True(t, names[want], "expected check %q to be present", want)
	}
}

func TestValidateConfig_FileNotFound(t *testing.T) {
	results := ValidateConfig("/nonexistent/config.yaml")
	require.Len(t, results, 1)
	assert.Equal(t, "config.load", results[0].Name)
	assert.False(t, results[0].OK)
	assert.Contains(t, results[0].Message, "config file not found")
}

func TestValidateConfig_PortConflict(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	require.NoError(t, os.WriteFile(configFile, []byte(`
storage:
  type: "memory"
server:
  port: 9090
metrics:
  enabled: true
  port: 9090
  path: "/metrics"
`), 0644))

	results := ValidateConfig(configFile)

	var crossFieldResult *CheckResult
	for i := range results {
		if results[i].Name == "config.cross-field" {
			crossFieldResult = &results[i]
			break
		}
	}
	require.NotNil(t, crossFieldResult)
	assert.False(t, crossFieldResult.OK)
	assert.Contains(t, crossFieldResult.Message, "must not be the same")
}

func TestValidateConfig_InvalidOTLPEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	require.NoError(t, os.WriteFile(configFile, []byte(`
storage:
  type: "memory"
observability:
  tracing:
    enabled: true
    exporter: "otlp"
    sample_rate: 1.0
    otlp_endpoint: "not-a-host-port"
`), 0644))

	results := ValidateConfig(configFile)

	var obsResult *CheckResult
	for i := range results {
		if results[i].Name == "config.observability" {
			obsResult = &results[i]
			break
		}
	}
	require.NotNil(t, obsResult)
	assert.False(t, obsResult.OK)
	assert.Contains(t, obsResult.Message, "invalid OTLP endpoint")
}

func TestValidateConfig_TLSFilesMissing(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	require.NoError(t, os.WriteFile(configFile, []byte(`
storage:
  type: "memory"
server:
  port: 8443
  tls_enabled: true
  tls_cert_file: "/nonexistent/cert.pem"
  tls_key_file: "/nonexistent/key.pem"
`), 0644))

	results := ValidateConfig(configFile)

	var tlsResult *CheckResult
	for i := range results {
		if results[i].Name == "runtime.tls" {
			tlsResult = &results[i]
			break
		}
	}
	require.NotNil(t, tlsResult)
	assert.False(t, tlsResult.OK)
	assert.Contains(t, tlsResult.Message, "cannot read TLS cert file")
	assert.Contains(t, tlsResult.Message, "cannot read TLS key file")
}

func TestLoad_ExampleConfigFile(t *testing.T) {
	// Verify that the shipped examples/config.yaml loads successfully and
	// passes validation so it never drifts out of sync with models.Config.
	examplePath := filepath.Join("..", "..", "examples", "config.yaml")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Skip("examples/config.yaml not found; skipping")
	}

	config, err := Load(examplePath)
	require.NoError(t, err, "examples/config.yaml must load without error")
	require.NoError(t, config.Validate(), "examples/config.yaml must pass validation")

	// Verify key values match what the example documents.
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "0.0.0.0", config.Server.Host)
	assert.Equal(t, 30*time.Second, config.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, config.Server.WriteTimeout)
	assert.Equal(t, 60*time.Second, config.Server.IdleTimeout)
	assert.Equal(t, 30*time.Second, config.Server.ShutdownTimeout)
	assert.False(t, config.Server.TLSEnabled)

	assert.Equal(t, "sqlite", config.Storage.Type)
	assert.Equal(t, "./data/updater.db", config.Storage.Database.DSN)
	assert.Equal(t, 25, config.Storage.Database.MaxOpenConns)
	assert.Equal(t, 5, config.Storage.Database.MaxIdleConns)

	assert.False(t, config.Security.EnableAuth)

	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "stdout", config.Logging.Output)

	assert.True(t, config.Metrics.Enabled)
	assert.Equal(t, "/metrics", config.Metrics.Path)
	assert.Equal(t, 9090, config.Metrics.Port)

	assert.Equal(t, "updater", config.Observability.ServiceName)
	assert.False(t, config.Observability.Tracing.Enabled)
	assert.Equal(t, "stdout", config.Observability.Tracing.Exporter)
	assert.Equal(t, 1.0, config.Observability.Tracing.SampleRate)
}

func TestSaveExample_FilePermissions(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "example.yaml")

	err := SaveExample(filePath)
	require.NoError(t, err)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
