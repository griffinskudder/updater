# Future Enhancements

This document tracks potential enhancements and improvements that could be made
to the updater service. These are not currently planned for implementation but
represent opportunities for future development.

## 1. Config Cleanup

**Status:** Not Implemented
**Priority:** Medium
**Effort:** Small (1-2 hours)

Several fields in the configuration structs are defined and validated but never
consumed at runtime. They add surface area without providing any behaviour.

### Dead fields

| Field | Struct | Issue |
|-------|--------|-------|
| `StorageConfig.Path` | `models/config.go` | Never read; SQLite uses `Database.DSN` |
| `DatabaseConfig.Driver` | `models/config.go` | Never read; storage type dispatch uses `Storage.Type` |
| `DatabaseConfig.MaxOpenConns` | `models/config.go` | Defined but not applied to DB connections |
| `DatabaseConfig.MaxIdleConns` | `models/config.go` | Defined but not applied to DB connections |
| `DatabaseConfig.ConnMaxLifetime` | `models/config.go` | Defined but not applied to DB connections |
| `DatabaseConfig.ConnMaxIdleTime` | `models/config.go` | Defined but not applied to DB connections |

### Options

**Option A — Remove:** Delete `Path`, `Driver`, and the four connection pool
fields. Simplest; reduces config surface area.

**Option B — Implement connection pooling:** Keep the four `DatabaseConfig`
pool fields and apply them via `sql.DB.SetMaxOpenConns` etc. in the Postgres
and SQLite constructors. This makes the config fields functional and gives
operators tuning control. `Path` and `Driver` can still be removed as they
are truly redundant.

Option B is the better choice — connection pool tuning is useful in
production and the fields already exist in the config schema.

**Files to modify:**
- `internal/models/config.go` — remove `Path` and `Driver`
- `internal/models/config_test.go` — update tests
- `internal/storage/postgres.go` — apply pool settings after `sql.Open`
- `internal/storage/sqlite.go` — apply pool settings after `sql.Open`
- `internal/api/openapi/openapi.yaml` — update config schema if documented
- `examples/config.yaml` — remove `path` and `driver` fields
- `docs/storage.md` — update config reference

---

## 2. Observability Quick Wins

**Status:** Not Implemented
**Priority:** Medium
**Effort:** Small (1-2 hours)

### 2a. Build info metric

Add a standard Prometheus `build_info` gauge with version metadata as labels.
This is a widely-used pattern (Prometheus itself, Kubernetes, etc.) that makes
version information queryable and enables version-change alerting in Grafana.

**Implementation:** Register an `Int64ObservableGauge` named `build_info` in
`internal/observability/observability.go` that always returns 1, with labels
`version`, `git_commit`, `build_date`, and `environment`. Wire it up in
`Setup()` after the `MeterProvider` is created.

**Files to modify:**
- `internal/observability/observability.go`
- `internal/observability/observability_test.go`
- `docs/observability.md`

### 2b. Prune stale `future-enhancements.md`

The "Admin UI Version Display" item referenced the admin UI removed in PR #26.
Removed in this revision.

---

## 3. Storage Pagination Pushdown

**Status:** Not Implemented
**Priority:** Medium
**Effort:** Medium (half-day)

`ListApplications` and `ListReleases` currently call `storage.Applications()`
and `storage.Releases(appID)` respectively, which load all records into memory,
then paginate in the service layer. This works for small datasets but will not
scale.

**Approach:** Add `limit` and `offset` parameters to the storage interface
methods (or add new `ListApplications`/`ListReleases` interface methods), update
SQL queries to use `LIMIT`/`OFFSET`, and remove in-process slicing from the
service.

**Files to modify:**
- `internal/storage/interface.go` — new interface methods
- `internal/storage/memory.go` — in-memory implementation
- `internal/storage/postgres.go` + sqlc queries
- `internal/storage/sqlite.go` + sqlc queries
- `internal/update/service.go` — remove in-process pagination
- All provider tests

---

## 4. Error Response Metadata

**Status:** Not Implemented
**Priority:** Low
**Effort:** Small (1-2 hours)

Include build metadata in error responses to help with debugging and support.

**Current error response:**
```json
{
  "error": "application not found",
  "details": "application 'myapp' does not exist",
  "status": 404
}
```

**Enhanced error response:**
```json
{
  "error": "application not found",
  "details": "application 'myapp' does not exist",
  "status": 404,
  "metadata": {
    "version": "v1.0.0",
    "instance_id": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": "2026-03-07T10:30:00Z"
  }
}
```

**Files to modify:**
- `internal/models/response.go` — add optional `metadata` field to error type
- `internal/api/handlers.go` — inject version info into error responses
- `internal/api/openapi/openapi.yaml` — update error response schema
- `docs/` — update API reference

---

## 5. Version Comparison Endpoint

**Status:** Not Implemented
**Priority:** Low
**Effort:** Medium (3-4 hours)

Add a `GET /api/v1/version/compare?version=v1.0.0` endpoint that compares the
service version against a provided semver string. Useful for client
compatibility checks in rolling deployments.

**Files to modify:**
- `internal/version/version.go` — add `Compare()` function
- `internal/api/handlers.go` — add `CompareVersion` handler
- `internal/api/routes.go` — register route
- `internal/api/openapi/openapi.yaml` — document endpoint

---

## 6. Deployment History Tracking

**Status:** Not Implemented
**Priority:** Low
**Effort:** Large (2-3 days)

Track deployment history by persisting version information to storage on
startup, creating an audit trail of deployments.

**New endpoints:**
- `GET /api/v1/admin/deployments` — list deployment history
- `GET /api/v1/admin/deployments/current` — show currently running instances

**Data model:**
```go
type DeploymentRecord struct {
    ID          string     `json:"id"`
    Version     string     `json:"version"`
    GitCommit   string     `json:"git_commit"`
    BuildDate   string     `json:"build_date"`
    InstanceID  string     `json:"instance_id"`
    Hostname    string     `json:"hostname"`
    Environment string     `json:"environment"`
    StartTime   time.Time  `json:"start_time"`
    EndTime     *time.Time `json:"end_time,omitempty"`
}
```

**Implementation considerations:**
- New `deployment_history` table in both Postgres and SQLite schemas
- Record deployment on service startup, mark ended on graceful shutdown
- Handle multiple concurrent instances (Kubernetes)
- Add retention policy (e.g. last 100 deployments or 90 days)
- Requires admin permission

**Files to modify:**
- `internal/storage/sqlc/schemas/*/002_deployment_history.sql`
- `internal/storage/sqlc/queries/*/deployments.sql`
- `internal/models/deployment.go`
- `cmd/updater/updater.go`
- `internal/api/handlers.go` + routes + OpenAPI spec

---

## Implementation Priority

| # | Enhancement | Effort | Value |
|---|-------------|--------|-------|
| 1 | Config cleanup (implement pool settings) | Small | Medium |
| 2 | Build info metric | Small | High |
| 3 | Storage pagination pushdown | Medium | Medium |
| 4 | Error response metadata | Small | Low |
| 5 | Version comparison endpoint | Medium | Low |
| 6 | Deployment history tracking | Large | Low |