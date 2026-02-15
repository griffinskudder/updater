// Package models - Service configuration and operational settings.
// This file defines comprehensive configuration structures for all service components.
//
// Configuration Philosophy:
// - Hierarchical configuration with logical grouping (server, storage, security, etc.)
// - Environment-friendly defaults that work out of the box
// - Comprehensive validation to catch misconfigurations early
// - Support for multiple deployment scenarios (development, production, cloud)
// - Security-first approach with safe defaults
// - Extensible design for future enhancements
package models

import (
	"errors"
	"fmt"
	"time"
)

// Storage type constants
const (
	StorageTypeJSON     = "json"
	StorageTypeMemory   = "memory"
	StorageTypePostgres = "postgres"
	StorageTypeSQLite   = "sqlite"
)

// Config is the root configuration structure containing all service settings.
//
// Configuration Structure:
// - Server: HTTP server and network settings
// - Storage: Database and file storage configuration
// - Security: Authentication, authorization, and rate limiting
// - Logging: Structured logging and output configuration
// - Cache: Performance caching settings
// - Metrics: Monitoring and observability
//
// Design Benefits:
// - Single source of truth for all configuration
// - Clear separation of concerns by component
// - Easy to serialize/deserialize from YAML/JSON
// - Comprehensive validation across all components
type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server"`     // HTTP server configuration
	Storage  StorageConfig  `yaml:"storage" json:"storage"`   // Data persistence settings
	Security SecurityConfig `yaml:"security" json:"security"` // Authentication and authorization
	Logging  LoggingConfig  `yaml:"logging" json:"logging"`   // Logging and output configuration
	Cache    CacheConfig    `yaml:"cache" json:"cache"`       // Performance caching
	Metrics  MetricsConfig  `yaml:"metrics" json:"metrics"`   // Monitoring and metrics
}

type ServerConfig struct {
	Port         int           `yaml:"port" json:"port"`
	Host         string        `yaml:"host" json:"host"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	TLSEnabled   bool          `yaml:"tls_enabled" json:"tls_enabled"`
	TLSCertFile  string        `yaml:"tls_cert_file" json:"tls_cert_file"`
	TLSKeyFile   string        `yaml:"tls_key_file" json:"tls_key_file"`
	CORS         CORSConfig    `yaml:"cors" json:"cors"`
}

type CORSConfig struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods" json:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers" json:"allowed_headers"`
	MaxAge         int      `yaml:"max_age" json:"max_age"`
}

type StorageConfig struct {
	Type     string            `yaml:"type" json:"type"`
	Path     string            `yaml:"path" json:"path"`
	Database DatabaseConfig    `yaml:"database" json:"database"`
	Options  map[string]string `yaml:"options" json:"options"`
}

type DatabaseConfig struct {
	Driver          string        `yaml:"driver" json:"driver"`
	DSN             string        `yaml:"dsn" json:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
}

type SecurityConfig struct {
	APIKeys        []APIKey        `yaml:"api_keys" json:"api_keys"`
	RateLimit      RateLimitConfig `yaml:"rate_limit" json:"rate_limit"`
	JWTSecret      string          `yaml:"jwt_secret" json:"jwt_secret"`
	EnableAuth     bool            `yaml:"enable_auth" json:"enable_auth"`
	TrustedProxies []string        `yaml:"trusted_proxies" json:"trusted_proxies"`
}

type APIKey struct {
	Key         string   `yaml:"key" json:"key"`
	Name        string   `yaml:"name" json:"name"`
	Permissions []string `yaml:"permissions" json:"permissions"`
	Enabled     bool     `yaml:"enabled" json:"enabled"`
}

type RateLimitConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	RequestsPerMinute int           `yaml:"requests_per_minute" json:"requests_per_minute"`
	RequestsPerHour   int           `yaml:"requests_per_hour" json:"requests_per_hour"`
	BurstSize         int           `yaml:"burst_size" json:"burst_size"`
	CleanupInterval   time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
}

type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"`
	Format     string `yaml:"format" json:"format"`
	Output     string `yaml:"output" json:"output"`
	FilePath   string `yaml:"file_path" json:"file_path"`
	MaxSize    int    `yaml:"max_size" json:"max_size"`
	MaxBackups int    `yaml:"max_backups" json:"max_backups"`
	MaxAge     int    `yaml:"max_age" json:"max_age"`
	Compress   bool   `yaml:"compress" json:"compress"`
}

type CacheConfig struct {
	Enabled bool          `yaml:"enabled" json:"enabled"`
	Type    string        `yaml:"type" json:"type"`
	TTL     time.Duration `yaml:"ttl" json:"ttl"`
	Redis   RedisConfig   `yaml:"redis" json:"redis"`
	Memory  MemoryConfig  `yaml:"memory" json:"memory"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr" json:"addr"`
	Password string `yaml:"password" json:"password"`
	DB       int    `yaml:"db" json:"db"`
	PoolSize int    `yaml:"pool_size" json:"pool_size"`
}

type MemoryConfig struct {
	MaxSize         int           `yaml:"max_size" json:"max_size"`
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Path    string `yaml:"path" json:"path"`
	Port    int    `yaml:"port" json:"port"`
}

// NewDefaultConfig creates a configuration with production-ready defaults.
//
// Default Configuration Principles:
// - Security-first: Authentication disabled but ready, HTTPS preferred
// - Performance: Reasonable timeouts and connection limits
// - Reliability: Conservative rate limits, structured logging
// - Observability: Metrics enabled by default for monitoring
// - Development-friendly: JSON file storage, permissive CORS for testing
// - Production-ready: Easy to override for deployment-specific needs
//
// Default Values Rationale:
// - Port 8080: Standard non-privileged HTTP port
// - 30-second timeouts: Balance between user experience and resource protection
// - JSON storage: Simple setup without external dependencies
// - Rate limiting enabled: Prevent abuse from the start
// - Structured logging: Better for log aggregation and analysis
// - Memory caching: Good performance without external dependencies
func NewDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			Host:         "0.0.0.0",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
			TLSEnabled:   false,
			CORS: CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders: []string{"*"},
				MaxAge:         86400,
			},
		},
		Storage: StorageConfig{
			Type: "json",
			Path: "./data/releases.json",
			Database: DatabaseConfig{
				Driver:          "sqlite3",
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
				ConnMaxIdleTime: 5 * time.Minute,
			},
			Options: make(map[string]string),
		},
		Security: SecurityConfig{
			APIKeys: []APIKey{},
			RateLimit: RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 60,
				RequestsPerHour:   1000,
				BurstSize:         10,
				CleanupInterval:   5 * time.Minute,
			},
			EnableAuth:     false,
			TrustedProxies: []string{},
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
		Cache: CacheConfig{
			Enabled: true,
			Type:    "memory",
			TTL:     5 * time.Minute,
			Memory: MemoryConfig{
				MaxSize:         1000,
				CleanupInterval: 10 * time.Minute,
			},
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
			Port:    9090,
		},
	}
}

func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("invalid server config: %w", err)
	}

	if err := c.Storage.Validate(); err != nil {
		return fmt.Errorf("invalid storage config: %w", err)
	}

	if err := c.Security.Validate(); err != nil {
		return fmt.Errorf("invalid security config: %w", err)
	}

	if err := c.Logging.Validate(); err != nil {
		return fmt.Errorf("invalid logging config: %w", err)
	}

	if err := c.Cache.Validate(); err != nil {
		return fmt.Errorf("invalid cache config: %w", err)
	}

	if err := c.Metrics.Validate(); err != nil {
		return fmt.Errorf("invalid metrics config: %w", err)
	}

	return nil
}

func (sc *ServerConfig) Validate() error {
	if sc.Port <= 0 || sc.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	if sc.Host == "" {
		return errors.New("host cannot be empty")
	}

	if sc.ReadTimeout < 0 {
		return errors.New("read timeout cannot be negative")
	}

	if sc.WriteTimeout < 0 {
		return errors.New("write timeout cannot be negative")
	}

	if sc.IdleTimeout < 0 {
		return errors.New("idle timeout cannot be negative")
	}

	if sc.TLSEnabled {
		if sc.TLSCertFile == "" {
			return errors.New("TLS cert file is required when TLS is enabled")
		}
		if sc.TLSKeyFile == "" {
			return errors.New("TLS key file is required when TLS is enabled")
		}
	}

	return nil
}

func (stc *StorageConfig) Validate() error {
	validTypes := []string{StorageTypeJSON, StorageTypeMemory, StorageTypePostgres, StorageTypeSQLite}
	found := false
	for _, vt := range validTypes {
		if stc.Type == vt {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid storage type: %s", stc.Type)
	}

	if stc.Type == StorageTypeJSON && stc.Path == "" {
		return errors.New("path is required for JSON storage")
	}

	if stc.Type == StorageTypeMemory {
		// Memory storage requires no additional configuration
		return nil
	}

	if (stc.Type == StorageTypePostgres || stc.Type == StorageTypeSQLite) && stc.Database.DSN == "" {
		return errors.New("database DSN is required for database storage")
	}

	return nil
}

func (sec *SecurityConfig) Validate() error {
	if sec.RateLimit.Enabled {
		if sec.RateLimit.RequestsPerMinute < 0 {
			return errors.New("requests per minute cannot be negative")
		}
		if sec.RateLimit.RequestsPerHour < 0 {
			return errors.New("requests per hour cannot be negative")
		}
		if sec.RateLimit.BurstSize < 0 {
			return errors.New("burst size cannot be negative")
		}
	}

	for _, apiKey := range sec.APIKeys {
		if apiKey.Key == "" {
			return errors.New("API key cannot be empty")
		}
		if apiKey.Name == "" {
			return errors.New("API key name cannot be empty")
		}
	}

	return nil
}

func (lc *LoggingConfig) Validate() error {
	validLevels := []string{"debug", "info", "warn", "error"}
	found := false
	for _, vl := range validLevels {
		if lc.Level == vl {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid log level: %s", lc.Level)
	}

	validFormats := []string{"json", "text"}
	found = false
	for _, vf := range validFormats {
		if lc.Format == vf {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid log format: %s", lc.Format)
	}

	validOutputs := []string{"stdout", "stderr", "file"}
	found = false
	for _, vo := range validOutputs {
		if lc.Output == vo {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid log output: %s", lc.Output)
	}

	if lc.Output == "file" && lc.FilePath == "" {
		return errors.New("file path is required when output is file")
	}

	return nil
}

func (cc *CacheConfig) Validate() error {
	if !cc.Enabled {
		return nil
	}

	validTypes := []string{"memory", "redis"}
	found := false
	for _, vt := range validTypes {
		if cc.Type == vt {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid cache type: %s", cc.Type)
	}

	if cc.TTL < 0 {
		return errors.New("cache TTL cannot be negative")
	}

	if cc.Type == "redis" && cc.Redis.Addr == "" {
		return errors.New("Redis address is required when cache type is redis")
	}

	return nil
}

func (mc *MetricsConfig) Validate() error {
	if !mc.Enabled {
		return nil
	}

	if mc.Path == "" {
		return errors.New("metrics path cannot be empty")
	}

	if mc.Port <= 0 || mc.Port > 65535 {
		return errors.New("metrics port must be between 1 and 65535")
	}

	return nil
}

func (ak *APIKey) HasPermission(permission string) bool {
	if !ak.Enabled {
		return false
	}
	for _, p := range ak.Permissions {
		if p == permission || p == "*" {
			return true
		}
	}
	return false
}
