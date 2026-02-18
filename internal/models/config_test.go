package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDefaultConfig(t *testing.T) {
	config := NewDefaultConfig()

	// Test server defaults
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "0.0.0.0", config.Server.Host)
	assert.Equal(t, 30*time.Second, config.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, config.Server.WriteTimeout)
	assert.Equal(t, 60*time.Second, config.Server.IdleTimeout)
	assert.False(t, config.Server.TLSEnabled)

	// Test storage defaults
	assert.Equal(t, "json", config.Storage.Type)
	assert.Equal(t, "./data/releases.json", config.Storage.Path)
	assert.Equal(t, "sqlite3", config.Storage.Database.Driver)
	assert.Equal(t, 25, config.Storage.Database.MaxOpenConns)
	assert.Equal(t, 5, config.Storage.Database.MaxIdleConns)
	assert.NotNil(t, config.Storage.Options)

	// Test security defaults
	assert.Empty(t, config.Security.BootstrapKey)
	assert.False(t, config.Security.EnableAuth)

	// Test logging defaults
	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "stdout", config.Logging.Output)
	assert.Equal(t, 100, config.Logging.MaxSize)
	assert.Equal(t, 3, config.Logging.MaxBackups)
	assert.Equal(t, 28, config.Logging.MaxAge)
	assert.True(t, config.Logging.Compress)

	// Test cache defaults
	assert.True(t, config.Cache.Enabled)
	assert.Equal(t, "memory", config.Cache.Type)
	assert.Equal(t, 5*time.Minute, config.Cache.TTL)
	assert.Equal(t, 1000, config.Cache.Memory.MaxSize)
	assert.Equal(t, 10*time.Minute, config.Cache.Memory.CleanupInterval)

	// Test metrics defaults
	assert.True(t, config.Metrics.Enabled)
	assert.Equal(t, "/metrics", config.Metrics.Path)
	assert.Equal(t, 9090, config.Metrics.Port)

	// Test observability defaults
	assert.Equal(t, "updater", config.Observability.ServiceName)
	assert.Equal(t, "1.0.0", config.Observability.ServiceVersion)
	assert.False(t, config.Observability.Tracing.Enabled)
	assert.Equal(t, "stdout", config.Observability.Tracing.Exporter)
	assert.Equal(t, 1.0, config.Observability.Tracing.SampleRate)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid default config",
			config:      NewDefaultConfig(),
			expectError: false,
		},
		{
			name: "invalid server config",
			config: &Config{
				Server: ServerConfig{Port: -1}, // Invalid port
				Storage: StorageConfig{
					Type: "json",
					Path: "./data/test.json",
				},
				Security: SecurityConfig{},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
					Output: "stdout",
				},
				Cache: CacheConfig{
					Enabled: false,
				},
				Metrics: MetricsConfig{
					Enabled: false,
				},
			},
			expectError: true,
			errorMsg:    "invalid server config",
		},
		{
			name: "invalid storage config",
			config: &Config{
				Server: ServerConfig{
					Port: 8080,
					Host: "localhost",
				},
				Storage: StorageConfig{
					Type: "invalid-type",
				},
				Security: SecurityConfig{},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
					Output: "stdout",
				},
				Cache: CacheConfig{
					Enabled: false,
				},
				Metrics: MetricsConfig{
					Enabled: false,
				},
			},
			expectError: true,
			errorMsg:    "invalid storage config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServerConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      ServerConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: ServerConfig{
				Port:         8080,
				Host:         "localhost",
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				IdleTimeout:  60 * time.Second,
			},
			expectError: false,
		},
		{
			name: "invalid port - negative",
			config: ServerConfig{
				Port: -1,
				Host: "localhost",
			},
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "invalid port - too high",
			config: ServerConfig{
				Port: 70000,
				Host: "localhost",
			},
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "empty host",
			config: ServerConfig{
				Port: 8080,
				Host: "",
			},
			expectError: true,
			errorMsg:    "host cannot be empty",
		},
		{
			name: "negative read timeout",
			config: ServerConfig{
				Port:        8080,
				Host:        "localhost",
				ReadTimeout: -1 * time.Second,
			},
			expectError: true,
			errorMsg:    "read timeout cannot be negative",
		},
		{
			name: "negative write timeout",
			config: ServerConfig{
				Port:         8080,
				Host:         "localhost",
				WriteTimeout: -1 * time.Second,
			},
			expectError: true,
			errorMsg:    "write timeout cannot be negative",
		},
		{
			name: "negative idle timeout",
			config: ServerConfig{
				Port:        8080,
				Host:        "localhost",
				IdleTimeout: -1 * time.Second,
			},
			expectError: true,
			errorMsg:    "idle timeout cannot be negative",
		},
		{
			name: "TLS enabled without cert file",
			config: ServerConfig{
				Port:       8080,
				Host:       "localhost",
				TLSEnabled: true,
				TLSKeyFile: "/path/to/key.pem",
			},
			expectError: true,
			errorMsg:    "TLS cert file is required when TLS is enabled",
		},
		{
			name: "TLS enabled without key file",
			config: ServerConfig{
				Port:        8080,
				Host:        "localhost",
				TLSEnabled:  true,
				TLSCertFile: "/path/to/cert.pem",
			},
			expectError: true,
			errorMsg:    "TLS key file is required when TLS is enabled",
		},
		{
			name: "TLS enabled with both files",
			config: ServerConfig{
				Port:        8080,
				Host:        "localhost",
				TLSEnabled:  true,
				TLSCertFile: "/path/to/cert.pem",
				TLSKeyFile:  "/path/to/key.pem",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStorageConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      StorageConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid JSON storage",
			config: StorageConfig{
				Type: "json",
				Path: "./data/releases.json",
			},
			expectError: false,
		},
		{
			name: "valid database storage",
			config: StorageConfig{
				Type: "postgres",
				Database: DatabaseConfig{
					DSN: "postgres://user:pass@localhost/db",
				},
			},
			expectError: false,
		},
		{
			name: "invalid storage type",
			config: StorageConfig{
				Type: "invalid",
			},
			expectError: true,
			errorMsg:    "invalid storage type: invalid",
		},
		{
			name: "JSON storage without path",
			config: StorageConfig{
				Type: "json",
			},
			expectError: true,
			errorMsg:    "path is required for JSON storage",
		},
		{
			name: "database storage without DSN",
			config: StorageConfig{
				Type: "postgres",
				Database: DatabaseConfig{
					Driver: "postgres",
				},
			},
			expectError: true,
			errorMsg:    "database DSN is required for database storage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSecurityConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      SecurityConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid config",
			config:      SecurityConfig{},
			expectError: false,
		},
		{
			name: "auth enabled without bootstrap key",
			config: SecurityConfig{
				EnableAuth: true,
			},
			expectError: true,
			errorMsg:    "bootstrap key is required when auth is enabled",
		},
		{
			name: "auth enabled with bootstrap key",
			config: SecurityConfig{
				EnableAuth:   true,
				BootstrapKey: "upd_test-key",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoggingConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      LoggingConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
			expectError: false,
		},
		{
			name: "valid file output",
			config: LoggingConfig{
				Level:    "debug",
				Format:   "text",
				Output:   "file",
				FilePath: "/var/log/updater.log",
			},
			expectError: false,
		},
		{
			name: "invalid log level",
			config: LoggingConfig{
				Level:  "invalid",
				Format: "json",
				Output: "stdout",
			},
			expectError: true,
			errorMsg:    "invalid log level: invalid",
		},
		{
			name: "invalid log format",
			config: LoggingConfig{
				Level:  "info",
				Format: "invalid",
				Output: "stdout",
			},
			expectError: true,
			errorMsg:    "invalid log format: invalid",
		},
		{
			name: "invalid log output",
			config: LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "invalid",
			},
			expectError: true,
			errorMsg:    "invalid log output: invalid",
		},
		{
			name: "file output without path",
			config: LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "file",
			},
			expectError: true,
			errorMsg:    "file path is required when output is file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCacheConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      CacheConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "cache disabled",
			config: CacheConfig{
				Enabled: false,
			},
			expectError: false,
		},
		{
			name: "valid memory cache",
			config: CacheConfig{
				Enabled: true,
				Type:    "memory",
				TTL:     5 * time.Minute,
			},
			expectError: false,
		},
		{
			name: "valid redis cache",
			config: CacheConfig{
				Enabled: true,
				Type:    "redis",
				TTL:     10 * time.Minute,
				Redis: RedisConfig{
					Addr: "localhost:6379",
				},
			},
			expectError: false,
		},
		{
			name: "invalid cache type",
			config: CacheConfig{
				Enabled: true,
				Type:    "invalid",
			},
			expectError: true,
			errorMsg:    "invalid cache type: invalid",
		},
		{
			name: "negative TTL",
			config: CacheConfig{
				Enabled: true,
				Type:    "memory",
				TTL:     -1 * time.Second,
			},
			expectError: true,
			errorMsg:    "cache TTL cannot be negative",
		},
		{
			name: "redis cache without address",
			config: CacheConfig{
				Enabled: true,
				Type:    "redis",
				Redis:   RedisConfig{},
			},
			expectError: true,
			errorMsg:    "Redis address is required when cache type is redis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricsConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      MetricsConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "metrics disabled",
			config: MetricsConfig{
				Enabled: false,
			},
			expectError: false,
		},
		{
			name: "valid metrics config",
			config: MetricsConfig{
				Enabled: true,
				Path:    "/metrics",
				Port:    9090,
			},
			expectError: false,
		},
		{
			name: "empty metrics path",
			config: MetricsConfig{
				Enabled: true,
				Path:    "",
				Port:    9090,
			},
			expectError: true,
			errorMsg:    "metrics path cannot be empty",
		},
		{
			name: "invalid port - negative",
			config: MetricsConfig{
				Enabled: true,
				Path:    "/metrics",
				Port:    -1,
			},
			expectError: true,
			errorMsg:    "metrics port must be between 1 and 65535",
		},
		{
			name: "invalid port - too high",
			config: MetricsConfig{
				Enabled: true,
				Path:    "/metrics",
				Port:    70000,
			},
			expectError: true,
			errorMsg:    "metrics port must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestObservabilityConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      ObservabilityConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "tracing disabled",
			config: ObservabilityConfig{
				Tracing: TracingConfig{Enabled: false},
			},
			expectError: false,
		},
		{
			name: "valid stdout tracing",
			config: ObservabilityConfig{
				Tracing: TracingConfig{
					Enabled:    true,
					Exporter:   "stdout",
					SampleRate: 1.0,
				},
			},
			expectError: false,
		},
		{
			name: "valid otlp tracing",
			config: ObservabilityConfig{
				Tracing: TracingConfig{
					Enabled:      true,
					Exporter:     "otlp",
					SampleRate:   0.5,
					OTLPEndpoint: "localhost:4317",
				},
			},
			expectError: false,
		},
		{
			name: "invalid exporter",
			config: ObservabilityConfig{
				Tracing: TracingConfig{
					Enabled:    true,
					Exporter:   "invalid",
					SampleRate: 1.0,
				},
			},
			expectError: true,
			errorMsg:    "invalid tracing exporter: invalid",
		},
		{
			name: "negative sample rate",
			config: ObservabilityConfig{
				Tracing: TracingConfig{
					Enabled:    true,
					Exporter:   "stdout",
					SampleRate: -0.1,
				},
			},
			expectError: true,
			errorMsg:    "tracing sample rate must be between 0 and 1",
		},
		{
			name: "sample rate above 1",
			config: ObservabilityConfig{
				Tracing: TracingConfig{
					Enabled:    true,
					Exporter:   "stdout",
					SampleRate: 1.5,
				},
			},
			expectError: true,
			errorMsg:    "tracing sample rate must be between 0 and 1",
		},
		{
			name: "otlp without endpoint",
			config: ObservabilityConfig{
				Tracing: TracingConfig{
					Enabled:    true,
					Exporter:   "otlp",
					SampleRate: 1.0,
				},
			},
			expectError: true,
			errorMsg:    "OTLP endpoint is required when tracing exporter is otlp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPIKey_HasPermission(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     APIKey
		permission string
		expected   bool
	}{
		{
			name: "has specific permission",
			apiKey: APIKey{
				Permissions: []string{"read", "write"},
				Enabled:     true,
			},
			permission: "read",
			expected:   true,
		},
		{
			name: "does not have permission",
			apiKey: APIKey{
				Permissions: []string{"read"},
				Enabled:     true,
			},
			permission: "write",
			expected:   false,
		},
		{
			name: "has wildcard permission",
			apiKey: APIKey{
				Permissions: []string{"*"},
				Enabled:     true,
			},
			permission: "admin",
			expected:   true,
		},
		{
			name: "key disabled",
			apiKey: APIKey{
				Permissions: []string{"read", "write"},
				Enabled:     false,
			},
			permission: "read",
			expected:   false,
		},
		{
			name: "empty permissions",
			apiKey: APIKey{
				Permissions: []string{},
				Enabled:     true,
			},
			permission: "read",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.apiKey.HasPermission(tt.permission)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigStructFields(t *testing.T) {
	// Test that all config struct fields are properly initialized
	config := NewDefaultConfig()

	// Verify all major sections exist
	assert.NotNil(t, config.Server)
	assert.NotNil(t, config.Storage)
	assert.NotNil(t, config.Security)
	assert.NotNil(t, config.Logging)
	assert.NotNil(t, config.Cache)
	assert.NotNil(t, config.Metrics)
	assert.NotNil(t, config.Observability)

	// Verify nested structures
	assert.NotNil(t, config.Storage.Database)
	assert.NotNil(t, config.Cache.Memory)
	assert.NotNil(t, config.Cache.Redis)
}

func TestDatabaseConfig_Structure(t *testing.T) {
	dbConfig := DatabaseConfig{
		Driver:          "postgres",
		DSN:             "postgres://user:pass@localhost/db",
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 1 * time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}

	assert.Equal(t, "postgres", dbConfig.Driver)
	assert.Equal(t, "postgres://user:pass@localhost/db", dbConfig.DSN)
	assert.Equal(t, 50, dbConfig.MaxOpenConns)
	assert.Equal(t, 10, dbConfig.MaxIdleConns)
	assert.Equal(t, 1*time.Hour, dbConfig.ConnMaxLifetime)
	assert.Equal(t, 30*time.Minute, dbConfig.ConnMaxIdleTime)
}

func TestMemoryConfig_Structure(t *testing.T) {
	memoryConfig := MemoryConfig{
		MaxSize:         2000,
		CleanupInterval: 15 * time.Minute,
	}

	assert.Equal(t, 2000, memoryConfig.MaxSize)
	assert.Equal(t, 15*time.Minute, memoryConfig.CleanupInterval)
}

func TestRedisConfig_Structure(t *testing.T) {
	redisConfig := RedisConfig{
		Addr:     "redis.example.com:6379",
		Password: "secret",
		DB:       1,
		PoolSize: 20,
	}

	assert.Equal(t, "redis.example.com:6379", redisConfig.Addr)
	assert.Equal(t, "secret", redisConfig.Password)
	assert.Equal(t, 1, redisConfig.DB)
	assert.Equal(t, 20, redisConfig.PoolSize)
}
