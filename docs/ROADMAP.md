# Roadmap

This document captures the planned direction for the updater service. Items are grouped into phases reflecting agreed priorities: developer experience comes first, then storage extensions, then ops hardening. Work items within a phase are roughly sequenced but may shift based on feedback.

Items marked **TBD** require an explicit design decision before implementation begins.

---

## Phase 1: Developer Experience

The most pressing gap is that every consumer of the service has to write their own HTTP integration and manage API keys by editing YAML. This phase removes both of those friction points.

### 1.1 Admin REST API

Add CRUD endpoints for managing API keys without touching config files or restarting the service.

**Scope:**

- `GET /api/v1/admin/keys` — list API keys (metadata only; never return the raw key after creation)
- `POST /api/v1/admin/keys` — create a key, returning it once in the response
- `PATCH /api/v1/admin/keys/{id}` — update name, permissions, or enabled state
- `DELETE /api/v1/admin/keys/{id}` — revoke a key

**Constraints:**

- Requires a new `admin` permission scope distinct from the existing `admin` release-management permission (or a super-admin concept)
- Keys stored in whichever configured storage backend is active; no separate keystore
- Audit log entry on every mutation

### 1.2 Admin Web UI

A server-rendered HTMX + Go templates UI embedded directly in the binary at `/admin`. No separate frontend build step; no external CDN at runtime.

Design document: `docs/plans/2026-02-16-admin-frontend.md`

**Covers:**

- API key login with HttpOnly session cookie
- Application CRUD
- Release listing and deletion
- Health dashboard (storage ping, uptime, active key count)

**Depends on:** 1.1 Admin REST API (or direct service layer calls — per the plan, handlers call the service directly)

### 1.3 Go Client SDK

A Go module (`updater/client` or a separate repository) that wraps the HTTP API and handles retries, version parsing, and platform detection.

**Scope:**

- `CheckForUpdate(appID, currentVersion, platform, arch string) (*UpdateResult, error)`
- `GetLatestVersion(appID, platform, arch string) (*Release, error)`
- `RegisterRelease(appID string, r ReleaseRequest) error` (authenticated)
- Configurable HTTP client (timeout, base URL, API key)
- Automatic platform and architecture detection from `runtime.GOOS` / `runtime.GOARCH`

### 1.4 Python and Java Client SDKs

Generate clients from the OpenAPI 3.0.3 spec (`internal/api/openapi/openapi.yaml`) using the official OpenAPI Generator, then layer hand-written convenience wrappers on top.

**Scope:**

- Python package (`updater-client`) — targets Python 3.10+, published to PyPI
- Java library — targets Java 17+, published to Maven Central
- Both expose the same conceptual surface as the Go SDK: `check_for_update`, `get_latest_version`, `register_release`

**Prerequisite:** The OpenAPI spec must be complete and validated before client generation. Run `make openapi-validate` before each release.

### 1.5 Admin CLI

A `updater-ctl` binary for scripting key management and release publishing without the web UI.

```
updater-ctl keys list
updater-ctl keys create --name "CI Publisher" --permissions write
updater-ctl keys revoke <id>
updater-ctl releases publish --app myapp --version 2.1.0 --platform linux --arch amd64 --url https://...
```

**Depends on:** 1.1 Admin REST API

---

## Phase 2: Storage Extensions

### 2.1 Presigned Redirect URL Support

Clients are redirected (or given a short-lived signed URL) to private object storage rather than receiving a static public URL. This enables storing binaries in private S3/GCS/Azure Blob buckets.

**Providers:**

| Provider | Implementation |
|----------|---------------|
| AWS S3 (+ S3-compatible: MinIO, Backblaze B2) | `storage/signing/s3.go` |
| Google Cloud Storage | `storage/signing/gcs.go` |
| Azure Blob Storage | `storage/signing/azure.go` |

**Design:**

- New `URLSigner` interface: `Sign(rawURL string, ttl time.Duration) (string, error)`
- Factory selects provider from config; falls back to passthrough (no signing) if unconfigured
- URL TTL configurable per-request and globally via config
- The `download_url` field in update check responses returns the signed URL

**Config:**

```yaml
signing:
  provider: s3       # s3 | gcs | azure | none
  ttl: 15m
  s3:
    bucket: my-releases
    region: us-east-1
    # credentials from environment (AWS_ACCESS_KEY_ID etc.) or IAM role
  gcs:
    bucket: my-releases
    credentials_file: /etc/updater/gcs-key.json
  azure:
    account: myaccount
    container: releases
```

### 2.2 Multi-Tenancy — TBD

**Decision required:** Should tenant isolation be enforced at the data layer (each tenant gets their own schema or database) or at the API key layer (keys are scoped to specific applications)?

Until a decision is reached, the service operates as single-tenant. The flag to introduce multi-tenancy early is whether a SaaS deployment model becomes a near-term requirement.

**Options under consideration:**

- **Schema-per-tenant (PostgreSQL):** Strong isolation; higher operational complexity
- **Application-namespace isolation:** Each API key is bound to one or more `app_id` values; enforced in middleware. Low complexity, weaker isolation.
- **Separate deployments:** Each tenant runs their own instance. No code changes needed.

---

## Phase 3: Ops & Reliability

### 3.1 Built-in Migration Runner

Embed the SQL migration files and apply any outstanding migrations automatically at startup. No separate migration tool or manual SQL execution required.

**Design:**

- Embed `internal/storage/sqlc/schema/` using `go:embed`
- Apply migrations in filename order (`001_initial.sql`, `002_add_indexes.sql`, …)
- Track applied migrations in a `schema_migrations` table
- Skip already-applied migrations; fail fast on hash mismatch
- Dry-run mode: `--migrate=dry-run` prints pending migrations without applying

**Supported backends:** PostgreSQL and SQLite only (JSON and memory providers do not need migrations).

### 3.2 High Availability

Document and validate the HA deployment model: multiple replicas, shared PostgreSQL, load balancer.

**Scope:**

- Verify all in-flight state is either in the database or stateless (rate limiter state is currently in-memory — requires a distributed backend for HA)
- Distributed rate limiter backend (Redis recommended)
- Documentation: `docs/deployment-ha.md`

### 3.3 Production Helm Chart

A production-grade Helm chart for Kubernetes deployment, replacing the current bare `deployments/kubernetes/deployment.yaml`.

**Scope:**

- Configurable replicas, resource limits, and pod disruption budgets
- Secret management via Kubernetes Secrets or external-secrets
- Optional ingress with TLS
- Readiness and liveness probes wired to `/health`
- Optional PostgreSQL dependency (Bitnami chart)

---

## Future / Under Consideration

These items have no committed phase. They will be re-evaluated as the above phases complete.

| Item | Notes |
|------|-------|
| Release signing and verification | Cosign or minisign signatures on binaries; `signature_url` field in responses |
| Delta updates | Binary patch format (bsdiff or similar); reduces download size for incremental updates |
| Staged rollouts / A/B releases | Serve different versions to percentage-based client cohorts |
| Redis caching layer | Reduce storage reads for high-volume update check endpoints |
| Webhook notifications | Notify downstream systems on new release registration |
| CLI tool auto-update | The `updater-ctl` CLI uses the service to update itself |

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-02-16 | Remain a pure metadata service; add presigned redirect support rather than hosting binaries | Keeps the service lightweight and offloads bandwidth to CDN/object storage |
| 2026-02-16 | Developer experience before ops hardening | Admin API, web UI, and SDKs remove the most immediate integration friction |
| 2026-02-16 | Multi-tenancy deferred pending design decision | Namespace-based vs schema-based isolation has significant architectural implications; premature commitment risks expensive rework |