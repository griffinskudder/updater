# Application Management API Design

**Date:** 2026-02-15
**Status:** Approved

## Problem

The updater service has no HTTP API for managing applications. Models (`CreateApplicationRequest`, `UpdateApplicationRequest`, etc.) and storage methods (`SaveApplication`, `GetApplication`, `Applications`) exist, but there are zero HTTP handlers or routes. Operators cannot create, list, update, or delete applications through the API -- they can only write directly to the storage backend.

Additionally, the `DeleteRelease` storage method exists in all backends but has no HTTP endpoint.

## Goals

1. Add full CRUD endpoints for application management
2. Add a delete endpoint for releases
3. Follow existing architectural patterns (service layer, typed errors, audit logging)
4. Enforce referential integrity when deleting applications that have releases

## Design

### API Endpoints

Six new endpoints under `/api/v1`:

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/applications` | write | Create application |
| `GET` | `/api/v1/applications` | read | List applications (paginated) |
| `GET` | `/api/v1/applications/{app_id}` | read | Get application with stats |
| `PUT` | `/api/v1/applications/{app_id}` | admin | Update application |
| `DELETE` | `/api/v1/applications/{app_id}` | admin | Delete application |
| `DELETE` | `/api/v1/updates/{app_id}/releases/{version}/{platform}/{arch}` | admin | Delete release |

Permission levels follow the existing hierarchy: `read < write < admin`. Creating an application requires `write` (same as registering a release). Updating and deleting require `admin` since these are destructive or high-impact operations.

The delete release path uses `{version}/{platform}/{arch}` because that is the compound key in the storage interface.

### Service Interface Extensions

Five new methods added to `ServiceInterface`, one for delete release:

```go
type ServiceInterface interface {
    // Existing methods
    CheckForUpdate(ctx context.Context, req *models.UpdateCheckRequest) (*models.UpdateCheckResponse, error)
    GetLatestVersion(ctx context.Context, req *models.LatestVersionRequest) (*models.LatestVersionResponse, error)
    ListReleases(ctx context.Context, req *models.ListReleasesRequest) (*models.ListReleasesResponse, error)
    RegisterRelease(ctx context.Context, req *models.RegisterReleaseRequest) (*models.RegisterReleaseResponse, error)

    // New methods
    CreateApplication(ctx context.Context, req *models.CreateApplicationRequest) (*models.CreateApplicationResponse, error)
    GetApplication(ctx context.Context, appID string) (*models.ApplicationInfoResponse, error)
    ListApplications(ctx context.Context, limit, offset int) (*models.ListApplicationsResponse, error)
    UpdateApplication(ctx context.Context, appID string, req *models.UpdateApplicationRequest) (*models.UpdateApplicationResponse, error)
    DeleteApplication(ctx context.Context, appID string) error
    DeleteRelease(ctx context.Context, req *models.DeleteReleaseRequest) (*models.DeleteReleaseResponse, error)
}
```

**Business rules:**

- `CreateApplication` -- validate request, check for duplicate ID (return `409 Conflict`), populate `CreatedAt`/`UpdatedAt` timestamps, save
- `GetApplication` -- fetch application and compute `ApplicationStats` (total releases, latest version, platform count) by querying storage
- `ListApplications` -- fetch all applications, apply pagination, populate summaries
- `UpdateApplication` -- verify application exists (return `404`), merge partial update fields, update `UpdatedAt` timestamp
- `DeleteApplication` -- verify application exists, enforce referential integrity (see below), delete
- `DeleteRelease` -- verify release exists (return `404`), delete, audit log

### Storage Layer Changes

**New method on the `Storage` interface:**

```go
DeleteApplication(ctx context.Context, appID string) error
```

**Two-tier referential integrity enforcement:**

| Backend | Referential integrity | Mechanism |
|---------|----------------------|-----------|
| PostgreSQL | `ON DELETE RESTRICT` foreign key on `releases.application_id` | Database rejects deletion, returns error |
| SQLite | `ON DELETE RESTRICT` foreign key on `releases.application_id` | Database rejects deletion, returns error |
| Memory | Service layer checks `Releases(ctx, appID)` before delete | Application code rejects deletion |
| JSON | Service layer checks `Releases(ctx, appID)` before delete | Application code rejects deletion |

**Database migrations:**

New migration files (`002_add_release_fk.sql`) in each engine's schema directory to add the FK constraint.

PostgreSQL:

```sql
ALTER TABLE releases
  ADD CONSTRAINT fk_releases_application
  FOREIGN KEY (application_id) REFERENCES applications(id)
  ON DELETE RESTRICT;
```

SQLite requires recreating the table since `ALTER TABLE ADD CONSTRAINT` is not supported. The FK is added inline in the new table definition.

Database backends detect FK constraint violations from the driver error and return a `storage.ErrHasDependencies` sentinel error. The service layer checks for this with `errors.Is()` and maps it to a `409 Conflict` response.

For memory/JSON backends, the service layer calls `Releases(ctx, appID)` before `DeleteApplication`. If releases exist, it returns the same typed error, keeping behavior consistent across all backends.

**New sqlc queries:**

- `DeleteApplication :exec` in both PostgreSQL and SQLite query directories
- sqlc regeneration after schema/query changes

### Handler Implementation

A new file `internal/api/handlers_applications.go` to keep application handlers separate.

**Handler methods on the existing `Handlers` struct:**

- `CreateApplication(w, r)` -- parse JSON body, call service, return `201 Created`
- `GetApplication(w, r)` -- extract `{app_id}` from path, call service, return `200 OK`
- `ListApplications(w, r)` -- parse `limit`/`offset` query params with defaults, call service, return `200 OK`
- `UpdateApplication(w, r)` -- extract `{app_id}`, parse JSON body, call service, return `200 OK`
- `DeleteApplication(w, r)` -- extract `{app_id}`, call service, return `204 No Content`
- `DeleteRelease(w, r)` -- extract `{app_id}/{version}/{platform}/{arch}`, call service, return `200 OK`

**Patterns carried forward:**

- Security audit logging on all write/admin operations (`slog.Warn` with `"event", "security_audit"` tag)
- `Content-Type` validation on POST/PUT bodies
- `writeServiceErrorResponse` for error mapping
- `GetSecurityContext(r)` for audit log enrichment
- `mux.Vars(r)` for path parameter extraction

**Route registration in `routes.go`:**

```go
// Application management endpoints
appAPI := api.PathPrefix("/applications").Subrouter()
appAPI.Use(authMiddleware(config))

appAPI.HandleFunc("", RequirePermission(PermissionWrite, handlers.CreateApplication)).Methods("POST")
appAPI.HandleFunc("", RequirePermission(PermissionRead, handlers.ListApplications)).Methods("GET")
appAPI.HandleFunc("/{app_id}", RequirePermission(PermissionRead, handlers.GetApplication)).Methods("GET")
appAPI.HandleFunc("/{app_id}", RequirePermission(PermissionAdmin, handlers.UpdateApplication)).Methods("PUT")
appAPI.HandleFunc("/{app_id}", RequirePermission(PermissionAdmin, handlers.DeleteApplication)).Methods("DELETE")

// Release deletion (alongside existing release endpoints)
protectedAPI.HandleFunc("/updates/{app_id}/releases/{version}/{platform}/{arch}",
    RequirePermission(PermissionAdmin, handlers.DeleteRelease)).Methods("DELETE")
```

### Error Handling

New conflict error constructor in `internal/update/errors.go`:

```go
func NewConflictError(message string) *ServiceError {
    return &ServiceError{
        Code:       models.ErrorCodeConflict,
        Message:    message,
        StatusCode: http.StatusConflict,
    }
}
```

Error mapping:

| Scenario | Error | HTTP Status |
|----------|-------|-------------|
| Create app with duplicate ID | `NewConflictError("application already exists")` | `409` |
| Get/update/delete non-existent app | `NewApplicationNotFoundError(appID)` | `404` |
| Delete app with existing releases | `NewConflictError("cannot delete application with existing releases")` | `409` |
| Delete non-existent release | `NewNotFoundError("release not found")` | `404` |
| Invalid request body | `NewValidationError(...)` | `400` |
| Database FK violation | Storage maps to `NewConflictError` | `409` |

### Testing

**Unit tests (table-driven, co-located):**

| File | Coverage |
|------|----------|
| `internal/update/service_test.go` | All six new service methods: create (success, duplicate, validation), get (success, not found), list (empty, pagination), update (success, not found, partial), delete app (success, not found, has releases), delete release (success, not found) |
| `internal/api/handlers_applications_test.go` | Handler tests via `httptest`: status codes, JSON responses, auth/permission enforcement, content-type validation, audit logging |
| `internal/models/request_test.go` | Validation for `CreateApplicationRequest` and `UpdateApplicationRequest` |
| `internal/storage/memory_test.go` | `DeleteApplication`: success, not found |
| `internal/storage/json_test.go` | Same as memory |
| `internal/storage/postgres_test.go` | `DeleteApplication`: success, FK violation returns typed error |
| `internal/storage/sqlite_test.go` | Same as postgres |

**Integration tests** (extend `internal/integration/integration_test.go`):

- Full application lifecycle: create, get, list, update, delete
- Referential integrity: create app, register release, attempt delete app (expect `409`), delete release, delete app (expect success)
- Permission enforcement: read-only key cannot create/update/delete, write key can create but not delete

### Documentation

- **`docs/api.md`** (new) -- Full API reference for all endpoints including existing update endpoints and new application management endpoints. Request/response examples, authentication, error codes.
- **`docs/ARCHITECTURE.md`** -- Update endpoint table and architecture overview.
- **`mkdocs.yml`** -- Add `- API: api.md` to nav.