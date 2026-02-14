# Configuration Models Documentation

## Overview

The configuration models provide comprehensive service configuration and operational settings. This system supports multiple deployment scenarios with production-ready defaults, hierarchical configuration management, and extensive validation to prevent misconfigurations.

## Design Philosophy

### Configuration Principles
- **Environment Friendly**: Defaults that work out of the box for development and production
- **Security First**: Safe defaults with authentication ready but requiring explicit enablement
- **Hierarchical Structure**: Logical grouping of related settings for maintainability
- **Validation Comprehensive**: Catch misconfigurations early with detailed error messages
- **Deployment Flexible**: Support for development, staging, and production environments

### Architecture Decisions
- **Component Separation**: Clear boundaries between server, storage, security, logging, cache, and metrics
- **Validation at Load Time**: Fail fast during service startup rather than runtime
- **Override Friendly**: Easy to customize specific settings without replacing entire configuration
- **Backward Compatibility**: New configuration options don't break existing deployments

## Root Configuration Structure

### Config Struct
```go
type Config struct {
    Server   ServerConfig   // HTTP server configuration
    Storage  StorageConfig  // Data persistence settings
    Security SecurityConfig // Authentication and authorization
    Logging  LoggingConfig  // Logging and output configuration
    Cache    CacheConfig    // Performance caching
    Metrics  MetricsConfig  // Monitoring and metrics
}
```

**Benefits:**
- Single source of truth for all configuration
- Clear separation of concerns by component
- Easy to serialize/deserialize from YAML/JSON
- Comprehensive validation across all components

### Default Configuration Factory

```go
func NewDefaultConfig() *Config
```

**Default Philosophy:**
- **Port 8080**: Standard non-privileged HTTP port
- **30-second timeouts**: Balance between user experience and resource protection
- **JSON storage**: Simple setup without external dependencies
- **Rate limiting enabled**: Prevent abuse from the start
- **Structured logging**: Better for log aggregation and analysis
- **Memory caching**: Good performance without external dependencies

## Server Configuration

### ServerConfig Structure
```go
type ServerConfig struct {
    Port         int           // Server port (1-65535)
    Host         string        // Bind address ("0.0.0.0" for all interfaces)
    ReadTimeout  time.Duration // Request read timeout
    WriteTimeout time.Duration // Response write timeout
    IdleTimeout  time.Duration // Connection idle timeout
    TLSEnabled   bool          // Enable HTTPS
    TLSCertFile  string        // TLS certificate file path
    TLSKeyFile   string        // TLS private key file path
    CORS         CORSConfig    // Cross-origin request settings
}
```

#### CORS Configuration
```go
type CORSConfig struct {
    Enabled        bool     // Enable CORS support
    AllowedOrigins []string // Allowed origin domains
    AllowedMethods []string // Allowed HTTP methods
    AllowedHeaders []string // Allowed request headers
    MaxAge         int      // Preflight cache duration (seconds)
}
```

**Production CORS Example:**
```go
CORS: CORSConfig{
    Enabled:        true,
    AllowedOrigins: []string{"https://myapp.com", "https://admin.myapp.com"},
    AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders: []string{"Authorization", "Content-Type"},
    MaxAge:         3600,
}
```

**Development CORS Example:**
```go
CORS: CORSConfig{
    Enabled:        true,
    AllowedOrigins: []string{"*"}, // Permissive for development
    AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders: []string{"*"},
    MaxAge:         86400,
}
```

#### Server Validation
```go
func (sc *ServerConfig) Validate() error
```

**Validation Rules:**
- Port must be between 1 and 65535
- Host cannot be empty
- Timeouts cannot be negative
- If TLS is enabled, both cert and key files must be specified
- CORS configuration must be valid if enabled

## Storage Configuration

### StorageConfig Structure
```go
type StorageConfig struct {
    Type     string            // Storage backend type
    Path     string            // File path (for JSON storage)
    Database DatabaseConfig    // Database connection settings
    Options  map[string]string // Backend-specific options
}
```

#### Supported Storage Types
- **json**: File-based JSON storage (development, small deployments)
- **memory**: In-memory storage (development, testing)

#### Database Configuration
```go
type DatabaseConfig struct {
    Driver          string        // Database driver name
    DSN             string        // Data source name (connection string)
    MaxOpenConns    int           // Maximum open connections
    MaxIdleConns    int           // Maximum idle connections
    ConnMaxLifetime time.Duration // Connection maximum lifetime
    ConnMaxIdleTime time.Duration // Connection maximum idle time
}
```

**PostgreSQL Example:**
```go
Database: DatabaseConfig{
    Driver:          "postgres",
    DSN:             "postgres://user:password@localhost/updater?sslmode=require",
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 5 * time.Minute,
    ConnMaxIdleTime: 5 * time.Minute,
}
```

**SQLite Example:**
```go
Database: DatabaseConfig{
    Driver:          "sqlite3",
    DSN:             "./data/updater.db",
    MaxOpenConns:    1,  // SQLite limitation
    MaxIdleConns:    1,
    ConnMaxLifetime: time.Hour,
    ConnMaxIdleTime: time.Hour,
}
```

## Security Configuration

### SecurityConfig Structure
```go
type SecurityConfig struct {
    APIKeys         []APIKey      // Authentication keys
    RateLimit       RateLimitConfig // Rate limiting settings
    JWTSecret       string        // JWT signing secret
    EnableAuth      bool          // Enable authentication
    TrustedProxies  []string      // Trusted proxy IP addresses
}
```

#### API Key Management
```go
type APIKey struct {
    Key         string   // The actual API key
    Name        string   // Human-readable name
    Permissions []string // Allowed operations
    Enabled     bool     // Active status
}
```

**API Key Usage Examples:**
```go
APIKeys: []APIKey{
    {
        Key:         "admin-key-secure-random-string",
        Name:        "Admin Key",
        Permissions: []string{"read", "write", "admin"},
        Enabled:     true,
    },
    {
        Key:         "readonly-key-another-random-string",
        Name:        "Read Only Key",
        Permissions: []string{"read"},
        Enabled:     true,
    },
    {
        Key:         "app-integration-key",
        Name:        "Application Integration",
        Permissions: []string{"read", "write"},
        Enabled:     true,
    },
}
```

#### Rate Limiting Configuration
```go
type RateLimitConfig struct {
    Enabled            bool          // Enable rate limiting
    RequestsPerMinute  int           // Requests allowed per minute
    RequestsPerHour    int           // Requests allowed per hour
    BurstSize          int           // Burst capacity
    CleanupInterval    time.Duration // Cleanup frequency for rate limit data
}
```

**Production Rate Limiting:**
```go
RateLimit: RateLimitConfig{
    Enabled:            true,
    RequestsPerMinute:  120,  // Higher for production
    RequestsPerHour:    5000, // Daily limit consideration
    BurstSize:          20,   // Handle traffic spikes
    CleanupInterval:    5 * time.Minute,
}
```

**Development Rate Limiting:**
```go
RateLimit: RateLimitConfig{
    Enabled:            true,
    RequestsPerMinute:  300,  // More permissive for development
    RequestsPerHour:    10000,
    BurstSize:          50,
    CleanupInterval:    10 * time.Minute,
}
```

## Logging Configuration

### LoggingConfig Structure
```go
type LoggingConfig struct {
    Level      string // Log level (debug, info, warn, error)
    Format     string // Log format (json, text)
    Output     string // Output destination (stdout, stderr, file)
    FilePath   string // File path (when output is file)
    MaxSize    int    // Maximum file size in MB
    MaxBackups int    // Maximum number of backup files
    MaxAge     int    // Maximum age in days
    Compress   bool   // Compress rotated files
}
```

#### Log Level Hierarchy
- **debug**: Detailed information for diagnosing problems
- **info**: General information about service operation
- **warn**: Warning messages that don't halt operation
- **error**: Error messages for problems that need attention

#### Format Options
- **json**: Structured JSON logging (recommended for production)
- **text**: Human-readable text format (good for development)

**Production Logging Example:**
```go
Logging: LoggingConfig{
    Level:      "info",
    Format:     "json",
    Output:     "file",
    FilePath:   "/var/log/updater/service.log",
    MaxSize:    100,  // 100MB files
    MaxBackups: 10,   // Keep 10 backup files
    MaxAge:     30,   // Delete logs older than 30 days
    Compress:   true, // Compress old logs
}
```

**Development Logging Example:**
```go
Logging: LoggingConfig{
    Level:      "debug",
    Format:     "text",
    Output:     "stdout",
    MaxSize:    10,   // Smaller files for development
    MaxBackups: 3,
    MaxAge:     7,    // Shorter retention for development
    Compress:   false,
}
```

## Caching Configuration

### CacheConfig Structure
```go
type CacheConfig struct {
    Enabled bool          // Enable caching
    Type    string        // Cache backend type
    TTL     time.Duration // Time to live for cached items
    Redis   RedisConfig   // Redis connection settings
    Memory  MemoryConfig  // In-memory cache settings
}
```

#### Supported Cache Types
- **memory**: In-memory caching (single instance, development)
- **redis**: Redis caching (distributed, production)

#### Redis Configuration
```go
type RedisConfig struct {
    Addr     string // Redis server address
    Password string // Redis password
    DB       int    // Database number
    PoolSize int    // Connection pool size
}
```

#### Memory Configuration
```go
type MemoryConfig struct {
    MaxSize         int           // Maximum number of cached items
    CleanupInterval time.Duration // Cleanup frequency for expired items
}
```

**Production Redis Caching:**
```go
Cache: CacheConfig{
    Enabled: true,
    Type:    "redis",
    TTL:     15 * time.Minute,
    Redis: RedisConfig{
        Addr:     "localhost:6379",
        Password: "secure-redis-password",
        DB:       0,
        PoolSize: 10,
    },
}
```

**Development Memory Caching:**
```go
Cache: CacheConfig{
    Enabled: true,
    Type:    "memory",
    TTL:     5 * time.Minute,
    Memory: MemoryConfig{
        MaxSize:         1000,
        CleanupInterval: 2 * time.Minute,
    },
}
```

## Metrics Configuration

### MetricsConfig Structure
```go
type MetricsConfig struct {
    Enabled bool   // Enable metrics collection
    Path    string // Metrics endpoint path
    Port    int    // Metrics server port
}
```

**Prometheus Integration Example:**
```go
Metrics: MetricsConfig{
    Enabled: true,
    Path:    "/metrics",
    Port:    9090, // Standard Prometheus port
}
```

**Disabled Metrics:**
```go
Metrics: MetricsConfig{
    Enabled: false,
}
```

## Configuration Loading and Validation

### Configuration Loading Patterns

#### From File
```go
func LoadConfigFromFile(path string) (*Config, error) {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }

    config := NewDefaultConfig()
    if err := yaml.Unmarshal(data, config); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }

    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }

    return config, nil
}
```

#### From Environment Variables
```go
func LoadConfigFromEnv() (*Config, error) {
    config := NewDefaultConfig()

    if port := os.Getenv("UPDATER_PORT"); port != "" {
        if p, err := strconv.Atoi(port); err == nil {
            config.Server.Port = p
        }
    }

    if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
        config.Storage.Type = "postgres"
        config.Storage.Database.DSN = dbURL
    }

    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }

    return config, nil
}
```

#### Combined Loading Strategy
```go
func LoadConfig() (*Config, error) {
    // Start with defaults
    config := NewDefaultConfig()

    // Override with file if exists
    if configPath := os.Getenv("UPDATER_CONFIG_PATH"); configPath != "" {
        if fileConfig, err := LoadConfigFromFile(configPath); err == nil {
            config = fileConfig
        }
    }

    // Override with environment variables
    applyEnvironmentOverrides(config)

    // Validate final configuration
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("configuration validation failed: %w", err)
    }

    return config, nil
}
```

### Configuration Validation

#### Root Validation
```go
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
```

## Deployment Scenarios

### Development Configuration
```yaml
server:
  port: 8080
  host: "127.0.0.1"
  tls_enabled: false
  cors:
    enabled: true
    allowed_origins: ["*"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]

storage:
  type: "json"
  path: "./data/releases.json"

security:
  enable_auth: false
  rate_limit:
    enabled: true
    requests_per_minute: 300

logging:
  level: "debug"
  format: "text"
  output: "stdout"

cache:
  enabled: true
  type: "memory"
  ttl: "5m"

metrics:
  enabled: true
  port: 9090
```

### Production Configuration
```yaml
server:
  port: 443
  host: "0.0.0.0"
  tls_enabled: true
  tls_cert_file: "/etc/ssl/certs/updater.crt"
  tls_key_file: "/etc/ssl/private/updater.key"
  cors:
    enabled: true
    allowed_origins: ["https://myapp.com"]
    allowed_methods: ["GET", "POST"]

storage:
  type: "postgres"
  database:
    dsn: "postgres://updater:password@db:5432/updater?sslmode=require"
    max_open_conns: 25
    max_idle_conns: 5

security:
  enable_auth: true
  api_keys:
    - key: "${ADMIN_API_KEY}"
      name: "Production Admin"
      permissions: ["read", "write", "admin"]
  rate_limit:
    enabled: true
    requests_per_minute: 120
    requests_per_hour: 5000

logging:
  level: "info"
  format: "json"
  output: "file"
  file_path: "/var/log/updater/service.log"
  max_size: 100
  max_backups: 10
  compress: true

cache:
  enabled: true
  type: "redis"
  ttl: "15m"
  redis:
    addr: "redis:6379"
    password: "${REDIS_PASSWORD}"
    pool_size: 10

metrics:
  enabled: true
  port: 9090
```

### Container Configuration
```dockerfile
# Environment-based configuration for containers
ENV UPDATER_PORT=8080
ENV UPDATER_HOST=0.0.0.0
ENV DATABASE_URL=postgres://user:pass@postgres:5432/updater
ENV REDIS_URL=redis://redis:6379/0
ENV LOG_LEVEL=info
ENV ENABLE_AUTH=true
ENV ADMIN_API_KEY=secure-random-key
```

## Configuration Security

### Sensitive Data Management
- **Environment Variables**: Use for secrets and credentials
- **File Permissions**: Restrict config file access (600 or 640)
- **Secret Rotation**: Support for runtime secret updates
- **Validation**: Ensure no secrets are logged or exposed

### Security Validation
```go
func (sec *SecurityConfig) Validate() error {
    // Validate API keys are not empty
    for _, apiKey := range sec.APIKeys {
        if apiKey.Key == "" {
            return errors.New("API key cannot be empty")
        }
        if len(apiKey.Key) < 32 {
            return errors.New("API key too short, minimum 32 characters")
        }
    }

    // Validate JWT secret strength
    if sec.EnableAuth && len(sec.JWTSecret) < 32 {
        return errors.New("JWT secret too short, minimum 32 characters")
    }

    return nil
}
```

## Performance Tuning

### High-Traffic Configuration
```go
// Optimized for high-traffic scenarios
config := &Config{
    Server: ServerConfig{
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  120 * time.Second,
    },
    Storage: StorageConfig{
        Database: DatabaseConfig{
            MaxOpenConns:    100, // Higher connection pool
            MaxIdleConns:    20,
            ConnMaxLifetime: 30 * time.Minute,
        },
    },
    Cache: CacheConfig{
        TTL: 30 * time.Minute, // Longer cache duration
        Redis: RedisConfig{
            PoolSize: 50, // Larger Redis pool
        },
    },
}
```

### Resource-Constrained Configuration
```go
// Optimized for limited resources
config := &Config{
    Server: ServerConfig{
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
    },
    Storage: StorageConfig{
        Type: "sqlite", // Lower resource usage
        Database: DatabaseConfig{
            MaxOpenConns: 5,  // Conservative limits
            MaxIdleConns: 2,
        },
    },
    Cache: CacheConfig{
        Type: "memory",
        Memory: MemoryConfig{
            MaxSize: 500, // Smaller cache
        },
    },
}
```

## Testing Configuration

### Test Configuration Factory
```go
func NewTestConfig() *Config {
    config := NewDefaultConfig()

    // Override for testing
    config.Server.Port = 0 // Random available port
    config.Storage.Type = "memory"
    config.Logging.Level = "error" // Reduce test noise
    config.Cache.Enabled = false
    config.Metrics.Enabled = false
    config.Security.EnableAuth = false

    return config
}
```

### Configuration Testing
```go
func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name     string
        config   Config
        hasError bool
    }{
        {
            name:     "default config",
            config:   *NewDefaultConfig(),
            hasError: false,
        },
        {
            name: "invalid port",
            config: Config{
                Server: ServerConfig{Port: 70000}, // Invalid port
            },
            hasError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if (err != nil) != tt.hasError {
                t.Errorf("Validate() error = %v, hasError = %v", err, tt.hasError)
            }
        })
    }
}
```

This comprehensive configuration system provides flexible, secure, and maintainable service configuration supporting various deployment scenarios while maintaining security and performance best practices.