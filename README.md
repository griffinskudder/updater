# updater

A lightweight HTTP service that lets desktop applications check for and download software updates. The service stores release metadata and returns download URLs pointing to externally hosted files (CDN, object storage), keeping the service itself stateless and easy to scale.

## Features

- **Update checking**: Clients supply their current version; the service returns the latest release for their platform and architecture
- **Semantic versioning**: Full semver support, pre-release filtering, minimum version enforcement, required-update flagging
- **Multiple storage backends**: JSON file, in-memory, PostgreSQL, SQLite — switched via config, no code changes
- **API key authentication**: Role-based permissions (`read` / `write` / `admin`) with permission inheritance
- **Rate limiting**: Per-IP token bucket, configurable anonymous and authenticated tiers
- **Observability**: Prometheus metrics, OpenTelemetry tracing (OTLP/gRPC + Jaeger), structured JSON logging
- **Containerized**: Distroless Docker image, multi-stage build, read-only filesystem, non-root user

## Quick Start

Only Docker is required.

```bash
make docker-dev     # Start with Docker Compose (JSON storage, no auth)
```

The service starts on `http://localhost:8080`.

### Check for an Update

```http
GET /api/v1/updates/{app_id}/check?current_version=1.2.3&platform=linux&architecture=amd64
```

```json
{
  "update_available": true,
  "latest_version": "2.0.0",
  "download_url": "https://releases.example.com/app/2.0.0/app-linux-amd64.tar.gz",
  "checksum": "abc123...",
  "checksum_type": "sha256",
  "release_notes": "New features and bug fixes",
  "required": false
}
```

## API

| Method | Endpoint | Permission | Description |
|--------|----------|------------|-------------|
| GET | `/api/v1/updates/{app_id}/check` | public | Check for update |
| POST | `/api/v1/check` | public | Check for update via JSON body |
| GET | `/api/v1/updates/{app_id}/latest` | public | Get latest version |
| GET | `/api/v1/updates/{app_id}/releases` | read | List releases |
| POST | `/api/v1/updates/{app_id}/register` | write | Register a release |
| DELETE | `/api/v1/updates/{app_id}/releases/{ver}/{plat}/{arch}` | admin | Delete a release |
| GET | `/api/v1/applications` | read | List applications |
| GET | `/api/v1/applications/{app_id}` | read | Get application details |
| POST | `/api/v1/applications` | write | Create application |
| PUT | `/api/v1/applications/{app_id}` | admin | Update application |
| DELETE | `/api/v1/applications/{app_id}` | admin | Delete application |
| GET | `/health` | public | Health check |

Authenticated requests use `Authorization: Bearer <api-key>`.

The full OpenAPI 3.0.3 specification is at `internal/api/openapi/openapi.yaml`.

## Configuration

Configuration is loaded from a YAML file passed via the `-config` flag. Environment variables prefixed with `UPDATER_` override file values.

```bash
./updater -config configs/dev.yaml
```

Key settings:

```yaml
server:
  port: 8080

storage:
  type: json          # json | memory | postgres | sqlite
  path: ./data/releases.json

security:
  enable_auth: false
  api_keys:
    - key: "my-admin-key"
      name: "Admin"
      permissions: ["admin"]
      enabled: true
  rate_limit:
    enabled: true
    requests_per_minute: 60

metrics:
  enabled: false
  port: 9090
```

See `examples/config.yaml` for a full reference with all available fields.

## Development

All targets run inside Docker containers. No local Go installation required.

```bash
make build            # Build binary to bin/updater
make test             # Run unit tests
make integration-test # Run integration tests
make check            # Format check + vet + unit tests
make security         # Run gosec security scanner
make help             # Show all available targets
```

Run the full observability stack locally (Prometheus + Grafana + Jaeger):

```bash
make docker-obs-up
```

## Documentation

Full documentation is available via MkDocs (Docker-based, no Python required):

```bash
make docs-serve       # Start docs server at http://localhost:8000
```

Key docs:

- `docs/ARCHITECTURE.md` — Design overview, data flow, and architectural decisions
- `docs/SECURITY.md` — Security model, permission matrix, and threat mitigations
- `docs/storage.md` — Storage backend configuration and trade-offs
- `docs/observability.md` — Metrics, tracing, and local observability stack setup