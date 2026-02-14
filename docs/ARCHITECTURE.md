# Update Service Architecture

## Overview

The updater service is designed to be queried by desktop applications to check for and download updates. The service acts as a metadata provider, referencing externally hosted download files rather than hosting the files directly. This design allows for efficient distribution via CDNs while maintaining a lightweight, scalable update service.

## Core Design Principles

1. **Stateless Design**: The service doesn't store user-specific data, only release metadata
2. **External Storage**: Download files are hosted separately (CDN/object storage)
3. **Flexible Storage**: Support multiple storage backends for metadata
4. **Version Agnostic**: Support semantic versioning and custom version schemes
5. **Platform Aware**: Handle different OS/architecture combinations
6. **Caching Ready**: Designed for CDN caching and local performance caching
7. **Extensible**: Plugin-like architecture for different storage backends

## Architecture Components

### 1. API Layer (`internal/api/`)
Handles HTTP requests and responses for update checks.

**Responsibilities:**
- REST/HTTP handlers for update checks
- Version comparison endpoints
- Health checks and monitoring
- Request validation and authentication
- Rate limiting and security

### 2. Update Management (`internal/update/`)
Core business logic for version comparison and update determination.

**Responsibilities:**
- Version comparison logic (semantic versioning)
- Update availability determination
- Release metadata management
- Platform/architecture filtering
- Update requirement rules (critical vs optional updates)

### 3. Storage Layer (`internal/storage/`)
Abstraction for release metadata persistence and retrieval.

**Responsibilities:**
- Release metadata storage interface
- Configuration management
- Caching for performance
- Support for multiple backends (JSON, database, external APIs)

### 4. Models (`internal/models/`)
Data structures and domain objects.

**Responsibilities:**
- Application metadata structures
- Version information schemas
- Update response formats
- Configuration schemas
- Validation rules

### 5. External Integration (`internal/external/`)
Integration with external services and validation of external resources.

**Responsibilities:**
- Download URL validation
- External file metadata retrieval
- CDN integration helpers
- Checksum verification

## API Design

### Core Endpoints

#### Check for Updates
```
GET /api/v1/updates/{app_id}/check?current_version=1.2.3&platform=windows&arch=amd64
```

#### Get Latest Version Info
```
GET /api/v1/updates/{app_id}/latest?platform=windows&arch=amd64
```

#### List All Releases
```
GET /api/v1/updates/{app_id}/releases
```

#### Register New Release (Admin)
```
POST /api/v1/updates/{app_id}/register
```

### Request/Response Format

#### Update Check Request
Parameters can be provided via query params or headers:
- `current_version`: Current application version
- `platform`: Target platform (windows, linux, darwin)
- `arch`: Architecture (amd64, arm64, 386)

#### Update Check Response
```json
{
  "update_available": true,
  "latest_version": "1.3.0",
  "download_url": "https://releases.example.com/app/1.3.0/app-windows-amd64.exe",
  "checksum": "sha256:abc123...",
  "checksum_type": "sha256",
  "file_size": 15728640,
  "release_notes": "Bug fixes and improvements",
  "release_date": "2024-02-14T10:00:00Z",
  "required": false,
  "minimum_version": "1.0.0"
}
```

## Directory Structure

```
.
├── cmd/
│   └── updater/               # Main application entry point
│       ├── main.go
│       ├── server.go
│       └── config.go
├── internal/
│   ├── api/                   # HTTP handlers and routing
│   │   ├── handlers.go
│   │   ├── middleware.go
│   │   ├── routes.go
│   │   └── validators.go
│   ├── update/                # Core update logic
│   │   ├── service.go
│   │   ├── version.go
│   │   └── comparator.go
│   ├── storage/               # Data persistence
│   │   ├── interface.go
│   │   ├── json.go
│   │   ├── database.go
│   │   └── cache.go
│   ├── models/                # Data structures
│   │   ├── release.go
│   │   ├── application.go
│   │   ├── version.go
│   │   └── config.go
│   ├── external/              # External service integration
│   │   ├── validator.go
│   │   └── metadata.go
│   ├── config/                # Configuration management
│   │   ├── config.go
│   │   └── loader.go
│   └── middleware/            # HTTP middleware
│       ├── auth.go
│       ├── logging.go
│       └── ratelimit.go
├── pkg/
│   ├── version/               # Version comparison utilities
│   │   ├── semver.go
│   │   └── custom.go
│   └── client/                # Go client library for apps
│       ├── client.go
│       └── types.go
├── docs/
│   ├── api.md                 # API documentation
│   ├── ARCHITECTURE.md        # This file
│   └── deployment.md          # Deployment guide
├── scripts/
│   ├── build.sh               # Build scripts
│   └── deploy.sh              # Deployment scripts
├── configs/
│   ├── config.yaml            # Default configuration
│   └── releases.json          # Sample release metadata
└── examples/
    ├── client/                # Example client implementations
    └── config/                # Example configurations
```

## Data Models

### Application
```go
type Application struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Platforms   []string          `json:"platforms"`
    Config      ApplicationConfig `json:"config"`
}
```

### Release
```go
type Release struct {
    ID           string            `json:"id"`
    ApplicationID string           `json:"application_id"`
    Version      string            `json:"version"`
    Platform     string            `json:"platform"`
    Architecture string            `json:"architecture"`
    DownloadURL  string            `json:"download_url"`
    Checksum     string            `json:"checksum"`
    ChecksumType string            `json:"checksum_type"`
    FileSize     int64             `json:"file_size"`
    ReleaseNotes string            `json:"release_notes"`
    ReleaseDate  time.Time         `json:"release_date"`
    Required     bool              `json:"required"`
    MinimumVersion string          `json:"minimum_version,omitempty"`
    Metadata     map[string]string `json:"metadata,omitempty"`
}
```

## Security Considerations

### Authentication & Authorization
- API key authentication for administrative endpoints
- Rate limiting to prevent abuse
- Request validation and sanitization

### Data Integrity
- Checksum validation for download files
- HTTPS enforcement for all communications
- Signed release metadata (future enhancement)

### Privacy
- Minimal data collection
- No user tracking or analytics storage
- Optional usage statistics aggregation

## Configuration Management

### Environment Variables
- `UPDATER_PORT`: Server port (default: 8080)
- `UPDATER_CONFIG_PATH`: Path to configuration file
- `UPDATER_STORAGE_TYPE`: Storage backend type (json, database)
- `UPDATER_LOG_LEVEL`: Logging level (debug, info, warn, error)

### Configuration File Structure
```yaml
server:
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

storage:
  type: json
  path: ./data/releases.json
  cache_ttl: 300s

security:
  api_keys:
    - key: "admin-key-123"
      permissions: ["read", "write"]
  rate_limit:
    requests_per_minute: 60

logging:
  level: info
  format: json
```

## Performance Considerations

### Caching Strategy
- In-memory caching of frequently requested releases
- HTTP cache headers for CDN optimization
- Configurable cache TTL per endpoint

### Scalability
- Stateless design enables horizontal scaling
- Database connection pooling
- Efficient indexing on version and platform fields

## Deployment Options

### Standalone Binary
- Single executable with embedded configuration
- Suitable for small deployments
- File-based storage backend

### Containerized Deployment
- Docker container with external configuration
- Kubernetes deployment with ConfigMaps
- External database backend

### Serverless
- AWS Lambda or similar for API handlers
- DynamoDB or similar for metadata storage
- CloudFront for caching

## Future Enhancements

1. **Metrics & Monitoring**
   - Prometheus metrics endpoint
   - Health check endpoints
   - Request tracing

2. **Advanced Features**
   - Delta updates support
   - Rollback functionality
   - A/B testing for releases

3. **Security Enhancements**
   - Release signing and verification
   - OAuth2/JWT authentication
   - Audit logging

4. **Storage Backends**
   - PostgreSQL support
   - Redis caching
   - S3-compatible object storage

## Testing Strategy

### Unit Tests
- Core update logic validation
- Version comparison algorithms
- Configuration parsing

### Integration Tests
- API endpoint functionality
- Storage backend integration
- External service validation

### End-to-End Tests
- Complete update check workflows
- Client library integration
- Performance benchmarking