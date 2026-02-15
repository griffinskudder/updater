# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based software update service called "updater". The service is designed to be queried by desktop applications to check for and download updates, with downloads hosted externally. The project now has a complete models layer implemented with comprehensive documentation.

## Project Structure

The project follows standard Go project layout conventions:

- `cmd/` - Main application entry points (currently contains `updater.go` with empty main function)
- `internal/` - Private application and library code
  - `models/` - Complete data model layer with version management, platform support, API contracts, and configuration
- `pkg/` - Library code that can be used by external applications (currently empty)
- `scripts/` - Build and development scripts (currently empty)
- `docs/` - Documentation (configured with MkDocs)
  - `index.md` - Main documentation homepage
  - `ARCHITECTURE.md` - Complete architectural design and API specification
  - `models/` - Comprehensive models documentation split by component
    - `index.md` - Overview and navigation guide for all model documentation
    - `version-models.md` - Version management and semantic versioning
    - `platform-models.md` - Platform support and application configuration
    - `release-models.md` - Release management and security validation
    - `api-models.md` - HTTP request/response contracts and validation
    - `config-models.md` - Service configuration and operational settings
- `mkdocs.yml` - MkDocs configuration file

## Development Commands

The Go module is already initialized. Use the Makefile for common development tasks:

Using the Makefile (recommended):
```bash
# Build the application to bin/updater
make build

# Run the application
make run

# Run tests
make test

# Format and vet code
make fmt
make vet

# Run all checks (format, vet, test)
make check

# Documentation commands (Docker-based)
make docs-serve  # Start MkDocs development server with Docker
make docs-build  # Build documentation site with Docker
make docs-clean  # Clean documentation artifacts

# Database commands
make sqlc-generate  # Generate Go code from SQL schemas using sqlc
make sqlc-vet      # Validate SQL schemas and queries

# Show all available commands
make help
```

Direct Go commands:
```bash
# Build the application
go build ./cmd/updater

# Run the application
go run ./cmd/updater

# Test the code
go test ./...

# Format the code
go fmt ./...

# Vet the code for issues
go vet ./...
```

## Documentation

The project uses MkDocs with the Material theme for documentation. The documentation is well-organized and includes:

### Setup
The documentation system uses Docker, so no Python/pip installation is required. You only need Docker installed on your system.

### Development
Use the Makefile commands for documentation development:
- `make docs-serve` - Start local development server with Docker (http://localhost:8000)
- `make docs-build` - Build static documentation site with Docker
- `make docs-clean` - Clean build artifacts

The Docker-based approach provides:
- No local Python/pip dependencies required
- Consistent environment across all platforms
- Uses the official `squidfunk/mkdocs-material` Docker image
- Automatic hot reloading during development

### Structure
- Professional Material theme with dark/light mode toggle
- Full-text search across all documentation
- Mobile responsive design
- Code syntax highlighting for Go examples
- Navigation organized by component (Architecture, Models)

### Requirements
- Docker (for documentation commands)
- No Python/pip installation needed

## Architecture Notes

The project has a comprehensive models layer implemented and documented. See `docs/ARCHITECTURE.md` for complete architectural design and `docs/MODELS.md` for detailed model documentation.

### Current Implementation Status

**✅ Completed:**
- Complete models layer (`internal/models/`)
  - Version management with semantic versioning support
  - Platform/architecture constants and application configuration
  - Release management with security and integrity verification
  - API request/response types with validation
  - Comprehensive service configuration system
- Storage layer (`internal/storage/`) with multiple providers:
  - JSON file storage with caching and concurrent access
  - In-memory storage for development/testing
  - Database foundation using sqlc-generated type-safe queries
  - Factory pattern for provider instantiation
  - Comprehensive test coverage with concurrency testing
- Database schema management:
  - Migration-friendly folder structure for PostgreSQL and SQLite
  - sqlc integration for type-safe database operations
  - Automated code generation from SQL schemas
- Complete API layer (`internal/api/`) with production-ready features:
  - HTTP handlers for all update operations (check, latest, list, register)
  - Comprehensive middleware stack (authentication, authorization, CORS, rate limiting)
  - Security-first design with API key authentication and permission system
  - Robust error handling with structured ServiceError types
  - Complete test coverage including security vulnerability testing
  - Request validation, logging, and panic recovery
- Core update service (`internal/update/`) with business logic:
  - Version comparison and update determination logic
  - Release management and filtering
  - Semantic versioning support with pre-release handling
  - Platform and architecture awareness
  - Structured error handling with HTTP status code mapping
- Comprehensive documentation and design rationales
- Build system (Makefile) with development commands
- Docker containerization with security-first approach:
  - Distroless base images for minimal attack surface
  - Multi-stage builds for optimized image sizes
  - Security hardening (non-root users, read-only filesystems, dropped capabilities)
  - Environment-based configuration management

**🚧 Next Steps:**
- Complete database storage provider implementations
- HTTP server setup and routing integration
- Configuration loading and management
- Production deployment configuration

### Architecture Overview

The service follows a layered architecture:
- **API Layer**: HTTP handlers and routing ✅ **COMPLETE**
- **Business Logic**: Update determination and version comparison ✅ **COMPLETE**
- **Models Layer**: Data structures and validation ✅ **COMPLETE**
- **Storage Layer**: Data persistence abstraction ✅ **COMPLETE**
- **Configuration**: Service configuration and settings ✅ **COMPLETE**

## Implementation Patterns & Best Practices

This section documents key patterns and approaches used in the codebase to maintain consistency and guide future development.

### Storage Layer Pattern

**Factory Pattern Implementation**
- **Location**: `internal/storage/factory.go`
- **Purpose**: Centralized provider instantiation with validation
- **Pattern**: Abstract Factory with provider registration
- **Usage**: `factory.Create(config)` returns interface-compliant storage
- **Benefits**: Easy provider switching, consistent validation, extensibility

**Provider Interface Design**
- **Location**: `internal/storage/interface.go`
- **Pattern**: Interface segregation with context support
- **Key Features**:
  - All methods accept `context.Context` for cancellation/timeouts
  - Consistent error handling patterns
  - Copy-on-return to prevent external mutation
  - Resource cleanup via `Close()` method

**Concrete Implementations**
- **JSON Provider**: File-based with caching, concurrent-safe, configurable TTL
- **Memory Provider**: Fast in-memory for development/testing, concurrent-safe
- **Database Foundation**: sqlc-generated type-safe queries (PostgreSQL/SQLite)

### Database Schema Management

**Migration-Friendly Structure**
- **Schema Organization**: Engine-specific folders (`postgres/`, `sqlite/`)
- **Naming Convention**: `{number}_{description}.sql` (001_initial.sql)
- **Sequential Numbering**: 3-digit zero-padded for proper ordering
- **Engine Optimization**: PostgreSQL uses JSONB, SQLite uses TEXT

**sqlc Integration Pattern**
- **Configuration**: `sqlc.yaml` points to schema folders, not individual files
- **Code Generation**: Type-safe Go structs and query methods
- **Query Organization**: Separate files per entity (applications.sql, releases.sql)
- **Makefile Integration**: `make sqlc-generate` and `make sqlc-vet` commands

### Docker Containerization Pattern

**Security-First Approach**
- **Base Image**: Distroless for minimal attack surface (3.9MB final image)
- **Multi-Stage Build**: Separate build and runtime stages for optimization
- **User Security**: Non-root user (65532:65532) with proper permissions
- **Filesystem**: Read-only root filesystem with tmpfs for temporary data
- **Capabilities**: All Linux capabilities dropped for minimal privileges

**Environment-Based Configuration**
- **Single Dockerfile**: Use environment variables instead of multiple Dockerfiles
- **Configuration**: `UPDATER_CONFIG_SECTION` for development/production modes
- **Secrets**: External environment files (`.env.example`) for sensitive data
- **Resource Limits**: Memory and CPU limits for production deployments

### Testing Patterns

**Comprehensive Coverage Approach**
- **Unit Tests**: All providers tested independently
- **Integration Tests**: Factory pattern with real provider creation
- **Concurrency Tests**: Multi-goroutine access validation
- **Mock/Fake**: Memory provider serves as fast fake for other layer testing
- **Error Scenarios**: Negative cases and edge conditions covered

**Test Organization**
- **Co-located**: `*_test.go` files alongside implementation
- **Table-Driven**: Multiple scenarios in single test functions
- **Helper Functions**: Reusable test utilities and setup
- **Cleanup**: Proper resource cleanup in test teardown

### Configuration Management Pattern

**Hierarchical Structure**
- **Organization**: Logical grouping (server, storage, security)
- **Validation**: Built-in validation methods per configuration section
- **Defaults**: Secure defaults with override capability
- **Environment Support**: Multiple environments (development, production)
- **Extensibility**: Easy addition of new configuration sections

### Error Handling Patterns

**ServiceError Pattern (API Layer)**
- **Location**: `internal/update/errors.go`
- **Purpose**: Structured errors with HTTP status code mapping
- **Pattern**: Type-safe error constructors for common scenarios
- **Usage**: `NewApplicationNotFoundError(appID)` returns proper HTTP 404
- **Benefits**: Consistent error responses, proper HTTP status codes, structured logging

**Consistent Error Wrapping**
- **Pattern**: `fmt.Errorf("operation failed: %w", err)` for context
- **Storage Errors**: Specific error types for different failure modes
- **Validation**: Structured validation errors with field-specific messages
- **Context Preservation**: Original error context maintained through layers
- **API Integration**: ServiceError types provide HTTP status code mapping

### API Layer Patterns

**Middleware Chain Pattern**
- **Location**: `internal/api/routes.go` and `internal/api/middleware.go`
- **Purpose**: Composable request processing pipeline
- **Implementation**: Sequential middleware application (CORS → Auth → Permissions → Business Logic)
- **Features**: Authentication, authorization, rate limiting, CORS, logging, panic recovery
- **Benefits**: Separation of concerns, reusable components, testable in isolation

**Security Middleware Pattern**
- **Authentication**: API key-based with Bearer token format
- **Authorization**: Role-based permissions (read/write/admin) with hierarchy
- **Optional Auth**: Endpoints that enhance data based on authentication status
- **Context Propagation**: Security context passed through request lifecycle
- **Audit Logging**: Security events logged with client identification

**Handler Pattern (API Layer)**
- **Location**: `internal/api/handlers.go`
- **Purpose**: Clean separation between HTTP concerns and business logic
- **Pattern**: Dependency injection of service interfaces
- **Error Handling**: ServiceError integration for proper HTTP status codes
- **Validation**: Request validation with structured error responses
- **Security**: Client IP detection and security context integration

**Testing Patterns (API Layer)**
- **Mock Services**: `MockUpdateService` with testify/mock for behavior verification
- **Security Testing**: Comprehensive vulnerability testing (SQL injection, path traversal, etc.)
- **Integration Testing**: End-to-end API testing with authentication flows
- **Mock Expectations**: Proper setup of service method expectations for different test scenarios
- **Test Organization**: Separate test files for different concerns (handlers vs security)

### Development Workflow Patterns

**Makefile-Driven Development**
- **Consistency**: Standardized commands across environments
- **Docker Integration**: Both local and containerized development
- **Code Generation**: Automated sqlc and documentation generation
- **Testing**: Multiple test scenarios (unit, integration, security)
- **Documentation**: Automated docs building and serving

**Documentation-Driven Development**
- **Architecture First**: Design documented before implementation
- **Model Documentation**: Comprehensive model layer documentation
- **Migration Guides**: Clear schema evolution documentation
- **Pattern Documentation**: This section for consistency

# IMPORTANT

- ALWAYS: Create a todo list.
- ALWAYS: Consider security when designing and implementing.
- ALWAYS: Write unit tests for the code.
- ALWAYS: Write docs to go with the code.
- ALWAYS: Use mermaid for diagrams in docs, except for directory structures.
- ALWAYS: Add docs to the nav config for the mkdocs site.
- NEVER: Use emojis.
- NEVER: Link to files outside the docs directory in documentation inside the docs directory.
- ALWAYS: Generate code after modifying sql files.
- NEVER: Use CGO. CGO IS NOT GO.
- ALWAYS: Ensure all tests are passing before finalising the request.