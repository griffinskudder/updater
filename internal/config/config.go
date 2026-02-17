package config

import (
	"fmt"
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

// LoadFromFile loads configuration from a YAML file
func loadFromFile(config *models.Config, filePath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", filePath)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
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

	if tls := os.Getenv("UPDATER_TLS_ENABLED"); tls != "" {
		config.Server.TLSEnabled = strings.ToLower(tls) == "true"
	}

	if certFile := os.Getenv("UPDATER_TLS_CERT_FILE"); certFile != "" {
		config.Server.TLSCertFile = certFile
	}

	if keyFile := os.Getenv("UPDATER_TLS_KEY_FILE"); keyFile != "" {
		config.Server.TLSKeyFile = keyFile
	}

	// CORS configuration
	if cors := os.Getenv("UPDATER_CORS_ENABLED"); cors != "" {
		config.Server.CORS.Enabled = strings.ToLower(cors) == "true"
	}

	if origins := os.Getenv("UPDATER_CORS_ALLOWED_ORIGINS"); origins != "" {
		config.Server.CORS.AllowedOrigins = strings.Split(origins, ",")
	}

	if methods := os.Getenv("UPDATER_CORS_ALLOWED_METHODS"); methods != "" {
		config.Server.CORS.AllowedMethods = strings.Split(methods, ",")
	}

	if headers := os.Getenv("UPDATER_CORS_ALLOWED_HEADERS"); headers != "" {
		config.Server.CORS.AllowedHeaders = strings.Split(headers, ",")
	}

	if maxAge := os.Getenv("UPDATER_CORS_MAX_AGE"); maxAge != "" {
		if age, err := strconv.Atoi(maxAge); err == nil {
			config.Server.CORS.MaxAge = age
		}
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

	if secret := os.Getenv("UPDATER_JWT_SECRET"); secret != "" {
		config.Security.JWTSecret = secret
	}

	if rateLimit := os.Getenv("UPDATER_RATE_LIMIT_ENABLED"); rateLimit != "" {
		config.Security.RateLimit.Enabled = strings.ToLower(rateLimit) == "true"
	}

	if rpm := os.Getenv("UPDATER_RATE_LIMIT_RPM"); rpm != "" {
		if r, err := strconv.Atoi(rpm); err == nil {
			config.Security.RateLimit.RequestsPerMinute = r
		}
	}

	if burst := os.Getenv("UPDATER_RATE_LIMIT_BURST"); burst != "" {
		if b, err := strconv.Atoi(burst); err == nil {
			config.Security.RateLimit.BurstSize = b
		}
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

	if maxSize := os.Getenv("UPDATER_LOG_MAX_SIZE"); maxSize != "" {
		if size, err := strconv.Atoi(maxSize); err == nil {
			config.Logging.MaxSize = size
		}
	}

	if maxBackups := os.Getenv("UPDATER_LOG_MAX_BACKUPS"); maxBackups != "" {
		if backups, err := strconv.Atoi(maxBackups); err == nil {
			config.Logging.MaxBackups = backups
		}
	}

	if maxAge := os.Getenv("UPDATER_LOG_MAX_AGE"); maxAge != "" {
		if age, err := strconv.Atoi(maxAge); err == nil {
			config.Logging.MaxAge = age
		}
	}

	if compress := os.Getenv("UPDATER_LOG_COMPRESS"); compress != "" {
		config.Logging.Compress = strings.ToLower(compress) == "true"
	}

	// Cache configuration
	if cache := os.Getenv("UPDATER_CACHE_ENABLED"); cache != "" {
		config.Cache.Enabled = strings.ToLower(cache) == "true"
	}

	if cacheType := os.Getenv("UPDATER_CACHE_TYPE"); cacheType != "" {
		config.Cache.Type = cacheType
	}

	if ttl := os.Getenv("UPDATER_CACHE_TTL"); ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			config.Cache.TTL = d
		}
	}

	// Redis configuration
	if addr := os.Getenv("UPDATER_REDIS_ADDR"); addr != "" {
		config.Cache.Redis.Addr = addr
	}

	if password := os.Getenv("UPDATER_REDIS_PASSWORD"); password != "" {
		config.Cache.Redis.Password = password
	}

	if db := os.Getenv("UPDATER_REDIS_DB"); db != "" {
		if dbNum, err := strconv.Atoi(db); err == nil {
			config.Cache.Redis.DB = dbNum
		}
	}

	if poolSize := os.Getenv("UPDATER_REDIS_POOL_SIZE"); poolSize != "" {
		if size, err := strconv.Atoi(poolSize); err == nil {
			config.Cache.Redis.PoolSize = size
		}
	}

	// Memory cache configuration
	if maxSize := os.Getenv("UPDATER_MEMORY_CACHE_MAX_SIZE"); maxSize != "" {
		if size, err := strconv.Atoi(maxSize); err == nil {
			config.Cache.Memory.MaxSize = size
		}
	}

	if cleanup := os.Getenv("UPDATER_MEMORY_CACHE_CLEANUP_INTERVAL"); cleanup != "" {
		if d, err := time.ParseDuration(cleanup); err == nil {
			config.Cache.Memory.CleanupInterval = d
		}
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
	config.Security.JWTSecret = "your-jwt-secret-here"

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
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
