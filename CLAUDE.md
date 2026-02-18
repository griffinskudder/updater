# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

Go-based software update service ("updater") queried by desktop applications to check for and download updates, with downloads hosted externally.

**Go version**: 1.25.0

## Project Structure

```
cmd/updater/          - Main application entry point (server initialization)
internal/
  api/                - HTTP handlers, middleware, routing
    openapi/          - OpenAPI 3.0.3 specification (openapi.yaml)
  config/             - Configuration loading and validation
  integration/        - Integration tests
  logger/             - Structured logging (log/slog)
  models/             - Data models: application, release, request, response, config, api_key
  ratelimit/          - Rate limiting (token bucket, HTTP middleware, in-memory backend)
  storage/            - Storage providers (JSON, memory, PostgreSQL, SQLite)
    sqlc/             - Generated type-safe database code (postgres/, sqlite/)
  update/             - Business logic: version comparison, release management, errors
  observability/      - Metrics (Prometheus) and distributed tracing (OpenTelemetry)
make/                 - Split Makefile targets (go.mk, docs.mk, docker.mk, db.mk)
configs/              - Configuration files
data/                 - Data directory (releases.json)
deployments/          - Kubernetes deployment manifests
docker/               - Nginx, Prometheus, Grafana configuration
docs/                 - MkDocs documentation site
examples/             - Example configuration and release data
pkg/                  - Public packages (currently empty placeholder)
scripts/              - Build scripts (docker-build.sh)
```

## Development Commands

All targets run inside Docker containers. Only Docker is required locally. Targets are split across `make/*.mk` files and auto-discovered by the root Makefile. Run `make help` to see all available targets.

```bash
make build            # Build to bin/updater (Docker)
make run              # Run the application (Docker)
make test             # Run tests (Docker)
make integration-test # Run integration tests with -tags integration (Docker)
make security         # Run gosec security scanner (high severity, medium confidence)
make secrets          # Scan git history for committed secrets with gitleaks
make fmt              # Format code (Docker)
make vet              # Vet code (Docker)
make clean            # Remove build artifacts (bin/)
make tidy             # Tidy go.mod dependencies (Docker)
make check            # Format + vet + test (Docker)
make docs-serve       # MkDocs dev server (http://localhost:8000)
make docs-build       # Build docs site (openapi-validate + docs-generate + docs-db + MkDocs)
make docs-generate    # Auto-generate model reference from Go doc comments (gomarkdoc -> docs/models/auto/models.md)
make docs-db          # Auto-generate database schema docs (tbls + ephemeral PostgreSQL -> docs/db/)
make docs-clean       # Clean docs artifacts
make openapi-validate # Validate OpenAPI spec with Redocly CLI (Docker)
make sqlc-generate    # Generate Go code from SQL schemas (Docker)
make sqlc-vet         # Validate SQL schemas and queries (Docker)
make help             # Show all commands
```

Docker and observability:

```bash
make docker-build     # Build secure Docker image
make docker-scan      # Scan Docker image for vulnerabilities
make docker-run       # Run container with security defaults (read-only, no caps)
make docker-dev       # Start dev environment with Docker Compose
make docker-prod      # Run container with production config (local testing)
make docker-clean     # Prune Docker artifacts
make docker-push      # Build and push image to registry
make docker-obs-up    # Start full observability stack (Jaeger, Prometheus, Grafana)
make docker-obs-down  # Stop observability stack
```

## Documentation

MkDocs with Material theme, Docker-based (no Python/pip needed). See `mkdocs.yml` for nav structure.

Key docs: `docs/ARCHITECTURE.md` (design), `docs/models/index.md` (model layer), `docs/storage.md` (storage providers), `docs/logging.md` (structured logging), `docs/SECURITY.md` (security overview), `docs/observability.md` (metrics & tracing).

`docs/models/auto/models.md` and `docs/db/` are auto-generated — edit Go doc comments and SQL schemas, not those files.

## Architecture

Layered architecture, all layers complete:

| Layer | Location |
|-------|----------|
| API (HTTP handlers, middleware) | `internal/api/` |
| Business Logic (version comparison) | `internal/update/` |
| Models (data structures, validation) | `internal/models/` |
| Storage (multi-provider persistence) | `internal/storage/` |
| Configuration | `internal/config/`, `internal/models/config.go` |
| Logging | `internal/logger/` |
| Observability (metrics, tracing) | `internal/observability/` |
| Containerization | `Dockerfile`, `docker-compose.yml` |

### Key Patterns

- **Storage**: Factory pattern (`storage/factory.go`), interface with `context.Context` support, copy-on-return
- **Database**: sqlc-generated queries, engine-specific schemas (`postgres/`, `sqlite/`), migration-friendly naming (`001_initial.sql`)
- **API**: Middleware chain (CORS -> Auth -> Permissions -> Handler), API key auth with role-based permissions; keys stored in DB, auth middleware calls `GetAPIKeyByHash` on every request
- **API Key Management**: Storage-backed (`internal/models/api_key.go`); bootstrap key seeds first admin key; CRUD via `GET/POST /api/v1/admin/keys` and `PATCH/DELETE /api/v1/admin/keys/{id}`
- **Rate Limiting**: Token bucket algorithm (`internal/ratelimit/`), two-tier limits (anonymous vs authenticated), middleware sets standard `X-RateLimit-*` headers
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
- NEVER: Use CGO. CGO IS NOT GO. EXCEPTION: Race detector requires CGO, but it is only used in tests and CI.
- ALWAYS: Ensure all tests are passing before finalising the request. This doesn't include docs changes.
- ALWAYS: Use context7 before using library code.
- ALWAYS: Update the openapi file when updating the API. This is a manual process, but it is important to keep the openapi file up to date.
- ALWAYS: Edit files directly, rather than using scripts to make changes. This is to ensure that the changes are intentional and to avoid accidentally making changes that are not intended.
- ALWAYS: Use the Makefile targets to run commands, rather than running commands directly. This is to ensure that the commands are run in a consistent environment and to avoid accidentally running commands that are not intended.
- ALWAYS: Use the GitHub MCP instead of the GitHub CLI.

## Gotchas

- **Docker is the only local requirement**: All Make targets run inside Docker containers. No local Go, sqlc, or Python needed.
- **Makefile requires POSIX shell**: On Windows, GNU Make + Git for Windows (which provides `sh`) are needed. All Makefile commands use POSIX syntax.
- **Config loading**: Use `-config path/to/config.yaml` CLI flag. Environment variables override file values.
- **`docs-build` has three pre-build steps**: `openapi-validate` (Redocly), `docs-generate` (gomarkdoc), and `docs-db` (tbls + ephemeral PostgreSQL). Any failure aborts the build.
- **Windows `$(CURDIR)` in Make loops**: Shell loops using `ls $(CURDIR)/...` fail on Windows (path isn't POSIX). Mount the directory into the container and use the container-internal path instead.
- **`pg_isready` race**: `pg_isready` can pass while psql connections still fail during PostgreSQL startup. Use `psql -c "SELECT 1"` as the readiness check.
