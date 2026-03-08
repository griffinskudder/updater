package config

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"updater/internal/models"

	"gopkg.in/yaml.v3"
)

// Load loads configuration from file and environment variables
func Load(configPath string) (*models.Config, error) {
	// Start with default configuration
	config := models.NewDefaultConfig()

	// Load from file if provided and exists
	if configPath != "" {
		if err := loadFromFile(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	// Override with environment variables
	loadFromEnvironment(config)

	// Validate the final configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// deprecatedConfig mirrors removed config fields for detecting stale operator configs.
type deprecatedConfig struct {
	Server struct {
		CORS interface{} `yaml:"cors"`
	} `yaml:"server"`
	Security struct {
		JWTSecret      string      `yaml:"jwt_secret"`
		TrustedProxies interface{} `yaml:"trusted_proxies"`
		RateLimit      interface{} `yaml:"rate_limit"`
	} `yaml:"security"`
	Observability struct {
		ServiceVersion string `yaml:"service_version"`
	} `yaml:"observability"`
	Cache interface{} `yaml:"cache"`
}

// warnDeprecatedKeys logs a warning for each removed config key found in the YAML data.
// The service continues to start normally - these keys are silently ignored by the main decoder.
func warnDeprecatedKeys(data []byte) {
	var dep deprecatedConfig
	if err := yaml.Unmarshal(data, &dep); err != nil {
		return
	}
	if dep.Server.CORS != nil {
		slog.Warn("Config key is no longer supported; configure CORS at your reverse proxy. See docs/reverse-proxy.md.", "config_key", "server.cors")
	}
	if dep.Security.JWTSecret != "" {
		slog.Warn("Config key is no longer used and can be removed from your config file.", "config_key", "security.jwt_secret")
	}
	if dep.Security.TrustedProxies != nil {
		slog.Warn("Config key is no longer supported; configure proxy trust at your reverse proxy. See docs/reverse-proxy.md.", "config_key", "security.trusted_proxies")
	}
	if dep.Security.RateLimit != nil {
		slog.Warn("Config key is no longer supported; configure rate limiting at your reverse proxy. See docs/reverse-proxy.md.", "config_key", "security.rate_limit")
	}
	if dep.Observability.ServiceVersion != "" {
		slog.Warn("Config key is no longer supported; version is now set at build time via ldflags. See docs/ARCHITECTURE.md.", "config_key", "observability.service_version")
	}
	if dep.Cache != nil {
		slog.Warn("Config key is no longer supported; caching was never implemented and the placeholder has been removed.", "config_key", "cache")
	}
}

// loadFromFile loads configuration from a YAML file
func loadFromFile(config *models.Config, filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", filePath)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	warnDeprecatedKeys(data)
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse YAML config: %w", err)
	}
	return nil
}

// loadFromEnvironment loads configuration from environment variables
func loadFromEnvironment(config *models.Config) {
	// Server configuration
	if port := os.Getenv("UPDATER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}

	if host := os.Getenv("UPDATER_HOST"); host != "" {
		config.Server.Host = host
	}

	if timeout := os.Getenv("UPDATER_READ_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			config.Server.ReadTimeout = d
		}
	}

	if timeout := os.Getenv("UPDATER_WRITE_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			config.Server.WriteTimeout = d
		}
	}

	if timeout := os.Getenv("UPDATER_IDLE_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			config.Server.IdleTimeout = d
		}
	}

	if timeout := os.Getenv("UPDATER_SHUTDOWN_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			config.Server.ShutdownTimeout = d
		}
	}

	if tls := os.Getenv("UPDATER_TLS_ENABLED"); tls != "" {
		config.Server.TLSEnabled = strings.ToLower(tls) == "true"
	}

	if certFile := os.Getenv("UPDATER_TLS_CERT_FILE"); certFile != "" {
		config.Server.TLSCertFile = certFile
	}

	if keyFile := os.Getenv("UPDATER_TLS_KEY_FILE"); keyFile != "" {
		config.Server.TLSKeyFile = keyFile
	}

	// Storage configuration
	if storageType := os.Getenv("UPDATER_STORAGE_TYPE"); storageType != "" {
		config.Storage.Type = storageType
	}

	if storagePath := os.Getenv("UPDATER_STORAGE_PATH"); storagePath != "" {
		config.Storage.Path = storagePath
	}

	if dsn := os.Getenv("UPDATER_DATABASE_DSN"); dsn != "" {
		config.Storage.Database.DSN = dsn
	}

	if driver := os.Getenv("UPDATER_DATABASE_DRIVER"); driver != "" {
		config.Storage.Database.Driver = driver
	}

	if maxOpen := os.Getenv("UPDATER_DATABASE_MAX_OPEN_CONNS"); maxOpen != "" {
		if conns, err := strconv.Atoi(maxOpen); err == nil {
			config.Storage.Database.MaxOpenConns = conns
		}
	}

	if maxIdle := os.Getenv("UPDATER_DATABASE_MAX_IDLE_CONNS"); maxIdle != "" {
		if conns, err := strconv.Atoi(maxIdle); err == nil {
			config.Storage.Database.MaxIdleConns = conns
		}
	}

	// Security configuration
	if auth := os.Getenv("UPDATER_ENABLE_AUTH"); auth != "" {
		config.Security.EnableAuth = strings.ToLower(auth) == "true"
	}

	// Bootstrap key from environment
	if bk := os.Getenv("UPDATER_BOOTSTRAP_KEY"); bk != "" {
		config.Security.BootstrapKey = bk
	}

	// Logging configuration
	if level := os.Getenv("UPDATER_LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}

	if format := os.Getenv("UPDATER_LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}

	if output := os.Getenv("UPDATER_LOG_OUTPUT"); output != "" {
		config.Logging.Output = output
	}

	if filePath := os.Getenv("UPDATER_LOG_FILE_PATH"); filePath != "" {
		config.Logging.FilePath = filePath
	}

	// Metrics configuration
	if metrics := os.Getenv("UPDATER_METRICS_ENABLED"); metrics != "" {
		config.Metrics.Enabled = strings.ToLower(metrics) == "true"
	}

	if path := os.Getenv("UPDATER_METRICS_PATH"); path != "" {
		config.Metrics.Path = path
	}

	if port := os.Getenv("UPDATER_METRICS_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Metrics.Port = p
		}
	}
}

// CheckResult holds the outcome of a single named validation check.
type CheckResult struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// ValidateConfig loads configuration from configPath (or defaults if empty) and runs
// all validation checks, returning a structured list of per-section results. Unlike
// Load, it does not fail fast — every check runs regardless of prior failures.
// Intended for use with the --validate CLI flag.
func ValidateConfig(configPath string) []CheckResult {
	var results []CheckResult

	add := func(name string, err error) {
		r := CheckResult{Name: name, OK: err == nil}
		if err != nil {
			r.Message = err.Error()
		}
		results = append(results, r)
	}

	cfg := models.NewDefaultConfig()
	if configPath != "" {
		if err := loadFromFile(cfg, configPath); err != nil {
			add("config.load", err)
			return results // Cannot validate without a parseable config.
		}
	}
	loadFromEnvironment(cfg)

	add("config.server", cfg.Server.Validate())
	add("config.storage", cfg.Storage.Validate())
	add("config.security", cfg.Security.Validate())
	add("config.logging", cfg.Logging.Validate())
	add("config.metrics", cfg.Metrics.Validate())
	add("config.observability", cfg.Observability.Validate())

	// Cross-field: server and metrics ports must not conflict.
	var crossErrs []error
	if cfg.Metrics.Enabled && cfg.Server.Port > 0 && cfg.Metrics.Port > 0 && cfg.Server.Port == cfg.Metrics.Port {
		crossErrs = append(crossErrs, fmt.Errorf(
			"server port and metrics port must not be the same (both are %d)", cfg.Server.Port,
		))
	}
	add("config.cross-field", errors.Join(crossErrs...))

	add("runtime.tls", validateTLS(cfg))
	add("runtime.log-dir", validateLogDir(cfg))

	return results
}

// ValidateRuntime performs I/O-bound checks on a fully loaded and validated
// configuration. It verifies that TLS files are readable and form a valid key
// pair, and that the log file directory exists and is writable. All checks run
// before any error is returned. Call this after Load and before starting any
// subsystem.
func ValidateRuntime(cfg *models.Config) error {
	return errors.Join(validateTLS(cfg), validateLogDir(cfg))
}

// validateTLS checks that the TLS cert and key files exist and form a valid
// key pair. It is a no-op when TLS is disabled.
func validateTLS(cfg *models.Config) error {
	if !cfg.Server.TLSEnabled {
		return nil
	}

	var errs []error

	certPEM, err := os.ReadFile(cfg.Server.TLSCertFile)
	if err != nil {
		errs = append(errs, fmt.Errorf("cannot read TLS cert file %q: %w", cfg.Server.TLSCertFile, err))
	}

	keyPEM, err := os.ReadFile(cfg.Server.TLSKeyFile)
	if err != nil {
		errs = append(errs, fmt.Errorf("cannot read TLS key file %q: %w", cfg.Server.TLSKeyFile, err))
	}

	// Only attempt pair validation when both reads succeeded.
	if certPEM != nil && keyPEM != nil {
		if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
			errs = append(errs, fmt.Errorf("invalid TLS cert/key pair: %w", err))
		}
	}

	return errors.Join(errs...)
}

// validateLogDir checks that the directory for the configured log file exists
// and is writable. It is a no-op when log output is not "file".
func validateLogDir(cfg *models.Config) error {
	if cfg.Logging.Output != "file" {
		return nil
	}

	dir := filepath.Dir(cfg.Logging.FilePath)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("log directory does not exist or is not accessible %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".updater-writable-check-*")
	if err != nil {
		return fmt.Errorf("log directory is not writable %q: %w", dir, err)
	}
	tmp.Close()
	os.Remove(tmp.Name())

	return nil
}

// SaveExample saves an example configuration file
func SaveExample(filePath string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Get default config with some example values
	config := models.NewDefaultConfig()

	// Set example bootstrap key
	config.Security.BootstrapKey = "upd_your-bootstrap-key-here"

	// Enable authentication for example
	config.Security.EnableAuth = true

	// Example TLS configuration
	config.Server.TLSEnabled = false
	config.Server.TLSCertFile = "/path/to/cert.pem"
	config.Server.TLSKeyFile = "/path/to/key.pem"

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
