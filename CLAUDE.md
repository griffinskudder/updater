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
- Comprehensive documentation and design rationales
- Build system (Makefile) with development commands

**🚧 Next Steps:**
- API layer implementation (`internal/api/`)
- Storage layer implementation (`internal/storage/`)
- Core update logic (`internal/update/`)
- HTTP server setup and routing
- Configuration loading and management

### Architecture Overview

The service follows a layered architecture:
- **API Layer**: HTTP handlers and routing (planned)
- **Business Logic**: Update determination and version comparison (planned)
- **Models Layer**: Data structures and validation ✅ **COMPLETE**
- **Storage Layer**: Data persistence abstraction (planned)
- **Configuration**: Service configuration and settings ✅ **COMPLETE**

# IMPORTANT

- ALWAYS: Create a todo list.
- ALWAYS: Consider security when designing and implementing.
- ALWAYS: Write unit tests for the code.
- ALWAYS: Write docs to go with the code.
- ALWAYS: Use mermaid for diagrams in docs.
- NEVER: Use emojis.