# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

Go-based software update service ("updater") queried by desktop applications to check for and download updates, with downloads hosted externally.

**Go version**: 1.26.0

## Project Structure

```
cmd/updater/          - Main application entry point (server initialization)
internal/
  api/                - HTTP handlers, middleware, routing
  config/             - Configuration loading and validation
  integration/        - Integration tests
  logger/             - Structured logging (log/slog)
  models/             - Data models: application, release, request, response, config
  storage/            - Storage providers (JSON, memory, PostgreSQL, SQLite)
    sqlc/             - Generated type-safe database code (postgres/, sqlite/)
  update/             - Business logic: version comparison, release management, errors
configs/              - Configuration files
data/                 - Data directory (releases.json)
deployments/          - Kubernetes deployment manifests
docker/               - Nginx configuration
docs/                 - MkDocs documentation site
examples/             - Security configuration examples
scripts/              - Build scripts (docker-build.sh)
```

## Development Commands

Use the Makefile (recommended):

```bash
make build          # Build to bin/updater
make run            # Run the application
make test           # Run tests
make fmt            # Format code
make vet            # Vet code
make check          # Format + vet + test
make docs-serve     # MkDocs dev server via Docker (http://localhost:8000)
make docs-build     # Build docs site via Docker
make docs-clean     # Clean docs artifacts
make sqlc-generate  # Generate Go code from SQL schemas
make sqlc-vet       # Validate SQL schemas and queries
make help           # Show all commands
```

Direct Go equivalents: `go build ./cmd/updater`, `go test ./...`, `go fmt ./...`, `go vet ./...`

## Documentation

MkDocs with Material theme, Docker-based (no Python/pip needed). See `mkdocs.yml` for nav structure.

Key docs: `docs/ARCHITECTURE.md` (design), `docs/models/index.md` (model layer), `docs/storage.md` (storage providers), `docs/logging.md` (structured logging), `docs/SECURITY.md` (security overview).

## Architecture

Layered architecture, all layers complete:

| Layer | Location | Status |
|-------|----------|--------|
| API (HTTP handlers, middleware) | `internal/api/` | Complete |
| Business Logic (version comparison) | `internal/update/` | Complete |
| Models (data structures, validation) | `internal/models/` | Complete |
| Storage (multi-provider persistence) | `internal/storage/` | Complete |
| Configuration | `internal/config/`, `internal/models/config.go` | Complete |
| Logging | `internal/logger/` | Complete |
| Containerization | `Dockerfile`, `docker-compose.yml` | Complete |

### Key Patterns

- **Storage**: Factory pattern (`storage/factory.go`), interface with `context.Context` support, copy-on-return
- **Database**: sqlc-generated queries, engine-specific schemas (`postgres/`, `sqlite/`), migration-friendly naming (`001_initial.sql`)
- **API**: Middleware chain (CORS -> Auth -> Permissions -> Handler), API key auth with role-based permissions
- **Errors**: `ServiceError` type in `internal/update/errors.go` maps to HTTP status codes
- **Logging**: `log/slog` with JSON/text formats, security audit events tagged `"event", "security_audit"`
- **Testing**: Table-driven, co-located `*_test.go`, memory provider as fast fake, concurrency tests
- **Docker**: Distroless base, multi-stage build, non-root user, read-only filesystem

See `docs/ARCHITECTURE.md` for full design details and rationales.

## Rules

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
- ALWAYS: Use context7 before using library code.
