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

	// Test CORS defaults
	assert.True(t, config.Server.CORS.Enabled)
	assert.Contains(t, config.Server.CORS.AllowedOrigins, "*")
	assert.Contains(t, config.Server.CORS.AllowedMethods, "GET")
	assert.Contains(t, config.Server.CORS.AllowedMethods, "POST")
	assert.Contains(t, config.Server.CORS.AllowedHeaders, "*")
	assert.Equal(t, 86400, config.Server.CORS.MaxAge)

	// Test storage defaults
	assert.Equal(t, "json", config.Storage.Type)
	assert.Equal(t, "./data/releases.json", config.Storage.Path)
	assert.Equal(t, "sqlite3", config.Storage.Database.Driver)
	assert.Equal(t, 25, config.Storage.Database.MaxOpenConns)
	assert.Equal(t, 5, config.Storage.Database.MaxIdleConns)
	assert.NotNil(t, config.Storage.Options)

	// Test security defaults
	assert.Empty(t, config.Security.APIKeys)
	assert.True(t, config.Security.RateLimit.Enabled)
	assert.Equal(t, 60, config.Security.RateLimit.RequestsPerMinute)
	assert.Equal(t, 1000, config.Security.RateLimit.RequestsPerHour)
	assert.Equal(t, 10, config.Security.RateLimit.BurstSize)
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
				Security: SecurityConfig{
					RateLimit: RateLimitConfig{Enabled: false},
				},
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
				Security: SecurityConfig{
					RateLimit: RateLimitConfig{Enabled: false},
				},
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
			name: "valid config with rate limiting",
			config: SecurityConfig{
				RateLimit: RateLimitConfig{
					Enabled:           true,
					RequestsPerMinute: 60,
					RequestsPerHour:   1000,
					BurstSize:         10,
				},
			},
			expectError: false,
		},
		{
			name: "valid config with API keys",
			config: SecurityConfig{
				APIKeys: []APIKey{
					{
						Key:         "test-key",
						Name:        "Test Key",
						Permissions: []string{"read", "write"},
						Enabled:     true,
					},
				},
				RateLimit: RateLimitConfig{Enabled: false},
			},
			expectError: false,
		},
		{
			name: "negative requests per minute",
			config: SecurityConfig{
				RateLimit: RateLimitConfig{
					Enabled:           true,
					RequestsPerMinute: -1,
				},
			},
			expectError: true,
			errorMsg:    "requests per minute cannot be negative",
		},
		{
			name: "negative requests per hour",
			config: SecurityConfig{
				RateLimit: RateLimitConfig{
					Enabled:         true,
					RequestsPerHour: -1,
				},
			},
			expectError: true,
			errorMsg:    "requests per hour cannot be negative",
		},
		{
			name: "negative burst size",
			config: SecurityConfig{
				RateLimit: RateLimitConfig{
					Enabled:   true,
					BurstSize: -1,
				},
			},
			expectError: true,
			errorMsg:    "burst size cannot be negative",
		},
		{
			name: "API key without key",
			config: SecurityConfig{
				APIKeys: []APIKey{
					{
						Name:    "Test Key",
						Enabled: true,
					},
				},
				RateLimit: RateLimitConfig{Enabled: false},
			},
			expectError: true,
			errorMsg:    "API key cannot be empty",
		},
		{
			name: "API key without name",
			config: SecurityConfig{
				APIKeys: []APIKey{
					{
						Key:     "test-key",
						Enabled: true,
					},
				},
				RateLimit: RateLimitConfig{Enabled: false},
			},
			expectError: true,
			errorMsg:    "API key name cannot be empty",
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
				Key:         "test-key",
				Permissions: []string{"read", "write"},
				Enabled:     true,
			},
			permission: "read",
			expected:   true,
		},
		{
			name: "does not have permission",
			apiKey: APIKey{
				Key:         "test-key",
				Permissions: []string{"read"},
				Enabled:     true,
			},
			permission: "write",
			expected:   false,
		},
		{
			name: "has wildcard permission",
			apiKey: APIKey{
				Key:         "admin-key",
				Permissions: []string{"*"},
				Enabled:     true,
			},
			permission: "admin",
			expected:   true,
		},
		{
			name: "key disabled",
			apiKey: APIKey{
				Key:         "disabled-key",
				Permissions: []string{"read", "write"},
				Enabled:     false,
			},
			permission: "read",
			expected:   false,
		},
		{
			name: "empty permissions",
			apiKey: APIKey{
				Key:         "empty-key",
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

	// Verify nested structures
	assert.NotNil(t, config.Server.CORS)
	assert.NotNil(t, config.Storage.Database)
	assert.NotNil(t, config.Security.RateLimit)
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

func TestRateLimitConfig_Structure(t *testing.T) {
	rateLimitConfig := RateLimitConfig{
		Enabled:           true,
		RequestsPerMinute: 120,
		RequestsPerHour:   5000,
		BurstSize:         20,
		CleanupInterval:   10 * time.Minute,
	}

	assert.True(t, rateLimitConfig.Enabled)
	assert.Equal(t, 120, rateLimitConfig.RequestsPerMinute)
	assert.Equal(t, 5000, rateLimitConfig.RequestsPerHour)
	assert.Equal(t, 20, rateLimitConfig.BurstSize)
	assert.Equal(t, 10*time.Minute, rateLimitConfig.CleanupInterval)
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
