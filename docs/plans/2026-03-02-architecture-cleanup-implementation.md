# Architecture Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove dead code, consolidate duplicated patterns, and add application-level metrics as specified in the [architecture cleanup design](2026-03-02-architecture-cleanup-design.md).

**Architecture:** Bottom-up execution -- smallest safe changes first, escalating to structural removals. Each task is independently committable and testable. TDD where applicable.

**Tech Stack:** Go 1.25, sqlc, PostgreSQL, SQLite, Prometheus/OTel, gorilla/mux

---

### Task 1: Delete Unused Model Types

**Files:**
- Modify: `internal/models/release.go` (delete lines 72-105 and 297-316)
- Modify: `internal/models/response.go` (delete lines 183-202 and 247-252)
- Modify: `internal/models/request.go` (delete lines 115-118)
- Modify: `internal/models/release_test.go` (delete tests at lines 628-682 and 705-719)
- Modify: `internal/models/response_test.go` (delete tests at lines 150-160, 360-386, 388-402)

**Step 1: Delete ReleaseFilter, ReleaseMetadata, ReleaseStats from release.go**

Remove the following from `internal/models/release.go`:
- Lines 72-91: `ReleaseFilter` struct and its doc comment
- Lines 93-105: `ReleaseMetadata` struct and its doc comment
- Lines 297-303: `ReleaseStats` struct
- Lines 305-316: `ReleaseFilter.Validate()` method

**Step 2: Delete their tests from release_test.go**

Remove from `internal/models/release_test.go`:
- Lines 628-682: `TestReleaseFilter_Validate` function
- Lines 705-719: `TestReleaseStats` function

**Step 3: Delete StatsResponse, ActivityItem, ValidationErrorResponse from response.go**

Remove from `internal/models/response.go`:
- Lines 183-197: `StatsResponse` and `ActivityItem` structs
- Lines 199-202: `ValidationErrorResponse` struct
- Lines 247-252: `NewValidationErrorResponse` function

**Step 4: Delete their tests from response_test.go**

Remove from `internal/models/response_test.go`:
- Lines 150-160: `TestNewValidationErrorResponse` function
- Lines 360-386: `TestStatsResponse_Structure` function
- Lines 388-402: `TestActivityItem_Structure` function

**Step 5: Delete HealthCheckRequest from request.go**

Remove from `internal/models/request.go`:
- Lines 115-118: `HealthCheckRequest` struct

**Step 6: Run tests**

Run: `make test`
Expected: All tests pass. No compilation errors.

**Step 7: Run auto-generated docs**

Run: `make docs-generate`

This regenerates `docs/models/auto/models.md` from Go doc comments. The deleted types will automatically disappear.

**Step 8: Commit**

```
refactor: delete 7 unused model types

Remove ReleaseFilter, ReleaseMetadata, ReleaseStats, StatsResponse,
ActivityItem, ValidationErrorResponse, and HealthCheckRequest.
None were instantiated or returned by any handler.
```

---

### Task 2: Fix Permission Duplication

**Files:**
- Modify: `internal/api/middleware.go` (remove SecurityContext, rewrite RequirePermission and GetSecurityContext)
- Modify: `internal/api/handlers.go` (update getAPIKeyName signature and callers)
- Modify: `internal/api/handlers_applications.go` (update SecurityContext references)
- Modify: `internal/api/handlers_admin.go` (no changes expected -- uses cookie auth, not SecurityContext)
- Test: existing tests in `internal/api/security_test.go`, `internal/api/handlers_test.go`

**Step 1: Write a helper function to extract APIKey from context**

Replace `GetSecurityContext` in `internal/api/middleware.go` (lines 59-68) with:

```go
// GetAPIKey extracts the authenticated API key from request context.
// Returns nil if no key is present (unauthenticated request).
func GetAPIKey(r *http.Request) *models.APIKey {
	if apiKey, ok := r.Context().Value("api_key").(*models.APIKey); ok {
		return apiKey
	}
	return nil
}
```

**Step 2: Rewrite RequirePermission to use APIKey.HasPermission directly**

Replace `RequirePermission` in `internal/api/middleware.go` (lines 70-92) with:

```go
func RequirePermission(required Permission) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := GetAPIKey(r)
			if apiKey == nil || !apiKey.HasPermission(string(required)) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				errorResp := models.NewErrorResponse(
					"Insufficient permissions for this operation",
					models.ErrorCodeForbidden,
				)
				json.NewEncoder(w).Encode(errorResp)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

**Step 3: Delete SecurityContext struct and HasPermission method**

Remove from `internal/api/middleware.go`:
- Lines 23-27: `SecurityContext` struct
- Lines 29-57: `SecurityContext.HasPermission` method

**Step 4: Update getAPIKeyName to accept *models.APIKey**

Change the signature in `internal/api/handlers.go` (line 368):

```go
func getAPIKeyName(apiKey *models.APIKey) string {
	if apiKey == nil {
		return "anonymous"
	}
	if apiKey.Name != "" {
		return apiKey.Name
	}
	return "unnamed-key"
}
```

**Step 5: Update all handler call sites**

In each handler that calls `GetSecurityContext(r)`, replace with `GetAPIKey(r)`:

`internal/api/handlers.go`:
- Line 212: `securityContext := GetSecurityContext(r)` -> `apiKey := GetAPIKey(r)`
- Line 218: `getAPIKeyName(securityContext)` -> `getAPIKeyName(apiKey)`
- Line 227: same pattern
- Line 283: `securityContext := GetSecurityContext(r)` -> `apiKey := GetAPIKey(r)`
- Line 286: `securityContext != nil && securityContext.HasPermission(PermissionRead)` -> `apiKey != nil && apiKey.HasPermission(string(PermissionRead))`
- Line 294: `securityContext.APIKey.Permissions` -> `apiKey.Permissions`

`internal/api/handlers_applications.go`:
- Lines 19, 118, 174, 215: Same pattern -- replace `GetSecurityContext(r)` with `GetAPIKey(r)` and update `getAPIKeyName` calls.

**Step 6: Run tests**

Run: `make test`
Expected: All tests pass, including security_test.go permission hierarchy tests.

**Step 7: Commit**

```
refactor: remove SecurityContext, use APIKey.HasPermission directly

Eliminates duplicated permission hierarchy logic. Middleware and
handlers now call models.APIKey.HasPermission instead of going
through a SecurityContext wrapper.
```

---

### Task 3: Remove Unused Config Fields

**Files:**
- Modify: `internal/models/config.go` (remove CacheConfig, RedisConfig, MemoryConfig, log rotation fields, Cache from Config struct)
- Modify: `internal/models/config_test.go` (remove cache/rotation tests)
- Modify: `internal/models/application.go` (simplify ApplicationConfig)
- Modify: `internal/models/application_test.go` (update ApplicationConfig tests)
- Modify: `internal/config/config.go` (remove cache/rotation env var loading)
- Modify: `internal/config/config_test.go` (remove cache/rotation test assertions)
- Modify: `internal/integration/integration_test.go` (update config blocks)
- Modify: `internal/storage/dbconvert.go` (update marshalConfig/unmarshalConfig if ApplicationConfig changes)
- Modify: `internal/api/openapi/openapi.yaml` (remove ApplicationConfig unused fields, remove cache config)
- Modify: `examples/config.yaml` (remove cache section, rotation fields)
- Modify: `docs/ARCHITECTURE.md` (remove cache env vars)

**Step 1: Remove CacheConfig, RedisConfig, MemoryConfig structs**

In `internal/models/config.go`:
- Delete lines 101-119 (CacheConfig, RedisConfig, MemoryConfig structs)
- Remove `Cache CacheConfig` field from Config struct (line 47)
- Delete `NewDefaultConfig` cache section (lines 192-200)
- Delete `CacheConfig.Validate()` method (lines 363-389)
- Remove `c.Cache.Validate()` call from `Config.Validate()` (lines 234-236)

**Step 2: Remove log rotation fields from LoggingConfig**

In `internal/models/config.go`:
- Remove fields `MaxSize`, `MaxBackups`, `MaxAge`, `Compress` from `LoggingConfig` (lines 95-98)
- Remove their defaults from `NewDefaultConfig` (lines 187-190)

**Step 3: Simplify ApplicationConfig**

In `internal/models/application.go`:
- Remove from `ApplicationConfig` struct (lines 86-95): `UpdateCheckURL`, `AutoUpdate`, `UpdateInterval`, `RequiredUpdate`, `MinVersion`, `MaxVersion`, `AllowPrerelease`, `NotificationURL`, `AnalyticsEnabled`
- Keep only `CustomFields map[string]string`
- Simplify `NewApplication` -- remove the config fields from defaults (lines 112-118), keep only `CustomFields: make(map[string]string)`
- Simplify or remove `ApplicationConfig.Validate()` (lines 169-195) since the only remaining field is CustomFields which needs no validation

**Step 4: Remove cache env var loading from config/config.go**

In `internal/config/config.go`:
- Remove all `UPDATER_CACHE_*` env var loading (lines ~215-262)
- Remove `UPDATER_REDIS_*` env var loading
- Remove `UPDATER_MEMORY_CACHE_*` env var loading
- Remove `UPDATER_LOG_MAX_SIZE`, `UPDATER_LOG_MAX_BACKUPS`, `UPDATER_LOG_MAX_AGE`, `UPDATER_LOG_COMPRESS` env var loading (lines ~193-213)

**Step 5: Update tests**

In `internal/models/config_test.go`:
- Remove test assertions for cache defaults and validation
- Remove test assertions for log rotation defaults

In `internal/models/application_test.go`:
- Remove tests for `UpdateInterval`, `MinVersion`, `MaxVersion` validation
- Update any tests that construct `ApplicationConfig` with the removed fields

In `internal/config/config_test.go`:
- Remove env var loading tests for cache and rotation fields

In `internal/integration/integration_test.go`:
- Remove any `AllowPrerelease` or cache-related config from test setup
- The pre-release test (lines 230-349) uses `AllowPrerelease` on `ApplicationConfig` -- this test logic needs updating since that field is being removed. The pre-release filtering still works (it is in the update service), but is no longer per-application configurable.

**Step 6: Update OpenAPI spec**

In `internal/api/openapi/openapi.yaml`:
- Remove unused fields from the `ApplicationConfig` schema
- Keep only `custom_fields`

**Step 7: Update config examples**

In `examples/config.yaml`:
- Remove entire `cache:` section
- Remove `max_size`, `max_backups`, `max_age`, `compress` from `logging:` section

**Step 8: Run tests**

Run: `make test`
Expected: All tests pass.

**Step 9: Validate OpenAPI spec**

Run: `make openapi-validate`
Expected: Spec validates.

**Step 10: Regenerate docs**

Run: `make docs-generate`

**Step 11: Commit**

```
refactor: remove unused config fields

Remove CacheConfig, RedisConfig, MemoryConfig (no caching layer
exists). Remove log rotation fields (no rotation library imported).
Simplify ApplicationConfig to only CustomFields (other fields were
stored but never consumed by any handler or service).
```

---

### Task 4: Remove Factory Pattern

**Files:**
- Delete: `internal/storage/factory.go`
- Delete: `internal/storage/factory_test.go`
- Modify: `internal/storage/interface.go` (remove `storage.Config` type)
- Modify: `internal/storage/json.go` (update constructor to not use `storage.Config` -- will be deleted in Task 5, but needs to compile here)
- Modify: `internal/storage/memory.go` (update constructor if it uses `storage.Config`)
- Modify: `internal/storage/postgres.go` (update constructor if it uses `storage.Config`)
- Modify: `internal/storage/sqlite.go` (update constructor if it uses `storage.Config`)
- Modify: `cmd/updater/updater.go` (update `initializeStorage` to not use `storage.Config`)
- Modify: `internal/integration/integration_test.go` (update storage initialization)

**Step 1: Identify all uses of storage.Config**

`storage.Config` is used in:
- `factory.go` (being deleted)
- `cmd/updater/updater.go:initializeStorage` -- constructs a `storage.Config`, then passes to constructors
- `internal/integration/integration_test.go` -- constructs `storage.Config` for JSON storage
- Each storage constructor (`NewJSONStorage`, `NewMemoryStorage`, `NewPostgresStorage`, `NewSQLiteStorage`)

**Step 2: Update storage constructors to accept specific parameters**

Instead of the opaque `storage.Config`, each constructor receives only what it needs:

- `NewMemoryStorage()` -- takes no arguments (it has no config)
- `NewJSONStorage(filePath string)` -- takes just the file path
- `NewPostgresStorage(dsn string)` -- takes just the connection string
- `NewSQLiteStorage(dsn string)` -- takes just the connection string

Review each constructor to confirm what fields of `storage.Config` they actually read, then update their signatures.

**Step 3: Update cmd/updater/updater.go initializeStorage**

```go
func initializeStorage(cfg *models.Config) (storage.Storage, error) {
	switch cfg.Storage.Type {
	case "json":
		return storage.NewJSONStorage(cfg.Storage.Path)
	case "memory":
		return storage.NewMemoryStorage()
	case "postgres":
		return storage.NewPostgresStorage(cfg.Storage.Database.DSN)
	case "sqlite":
		return storage.NewSQLiteStorage(cfg.Storage.Database.DSN)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Storage.Type)
	}
}
```

**Step 4: Update integration_test.go storage initialization**

Replace `storage.Config{...}` construction with direct constructor calls.

**Step 5: Delete factory.go, factory_test.go, and storage.Config**

- Delete `internal/storage/factory.go`
- Delete `internal/storage/factory_test.go`
- Remove `Config` struct from `internal/storage/interface.go` (lines 65-81)

**Step 6: Run tests**

Run: `make test`
Expected: All tests pass.

**Step 7: Commit**

```
refactor: remove storage factory pattern

The factory was only used in its own tests. Main uses direct
constructor calls. Simplified storage constructors to accept
only the parameters they need.
```

---

### Task 5: Drop JSON Storage Provider

**Files:**
- Delete: `internal/storage/json.go`
- Delete: `internal/storage/json_test.go`
- Modify: `internal/models/config.go` (remove StorageTypeJSON constant, update validation)
- Modify: `cmd/updater/updater.go` (remove "json" case from switch)
- Modify: `internal/integration/integration_test.go` (switch from JSON to memory or SQLite storage)
- Modify: `examples/config.yaml` (change default storage type)
- Modify: `docs/ARCHITECTURE.md` (remove JSON provider references)
- Modify: `docs/storage.md` (remove JSON provider documentation if it exists)

**Step 1: Delete JSON storage files**

- Delete `internal/storage/json.go`
- Delete `internal/storage/json_test.go`

**Step 2: Remove StorageTypeJSON constant and validation**

In `internal/models/config.go`:
- Remove `StorageTypeJSON = "json"` constant (line 21)
- In `StorageConfig.Validate()` (line 283): remove `StorageTypeJSON` from `validTypes` slice and remove the `stc.Type == StorageTypeJSON` path check (line 295-297)
- In `NewDefaultConfig()`: change `Type: "json"` to `Type: "sqlite"` and update `Path` to a SQLite database path like `"./data/updater.db"`

**Step 3: Remove JSON case from cmd/updater/updater.go**

In `initializeStorage`, remove the `case "json":` branch.

**Step 4: Update integration tests**

In `internal/integration/integration_test.go`:
- Switch storage initialization from `NewJSONStorage` to `NewMemoryStorage()` or `NewSQLiteStorage` with a temp file

**Step 5: Update config examples**

In `examples/config.yaml`:
- Change `type: json` to `type: sqlite`
- Update `path:` to point to a SQLite database file

**Step 6: Run tests**

Run: `make test`
Expected: All tests pass.

**Step 7: Update documentation**

- Update `docs/storage.md` if it exists (remove JSON provider section)
- Update `docs/ARCHITECTURE.md` -- remove references to JSON storage provider

**Step 8: Commit**

```
refactor: remove JSON storage provider

SQLite serves the same niche (file-based, single-node) with better
concurrency support and atomic writes. Default storage type is now
sqlite. Memory provider remains as test double, PostgreSQL for
production at scale.
```

---

### Task 6: Replace SELECT + INSERT/UPDATE with Upserts

**Files:**
- Modify: `internal/storage/sqlc/queries/postgres/applications.sql` (add upsert query)
- Modify: `internal/storage/sqlc/queries/postgres/releases.sql` (add upsert query)
- Modify: `internal/storage/sqlc/queries/sqlite/applications.sql` (add upsert query)
- Modify: `internal/storage/sqlc/queries/sqlite/releases.sql` (add upsert query)
- Regenerate: `internal/storage/sqlc/postgres/*.go` and `internal/storage/sqlc/sqlite/*.go`
- Modify: `internal/storage/postgres.go` (simplify SaveApplication, SaveRelease)
- Modify: `internal/storage/sqlite.go` (simplify SaveApplication, SaveRelease)

**Step 1: Add upsert query for applications (PostgreSQL)**

In `internal/storage/sqlc/queries/postgres/applications.sql`, add:

```sql
-- name: UpsertApplication :exec
INSERT INTO applications (id, name, description, platforms, config, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    platforms = EXCLUDED.platforms,
    config = EXCLUDED.config,
    updated_at = EXCLUDED.updated_at;
```

**Step 2: Add upsert query for releases (PostgreSQL)**

In `internal/storage/sqlc/queries/postgres/releases.sql`, add:

```sql
-- name: UpsertRelease :exec
INSERT INTO releases (
    id, application_id, version, platform, architecture, download_url,
    checksum, checksum_type, file_size, release_notes, release_date,
    required, minimum_version, metadata, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (application_id, version, platform, architecture) DO UPDATE SET
    id = EXCLUDED.id,
    download_url = EXCLUDED.download_url,
    checksum = EXCLUDED.checksum,
    checksum_type = EXCLUDED.checksum_type,
    file_size = EXCLUDED.file_size,
    release_notes = EXCLUDED.release_notes,
    release_date = EXCLUDED.release_date,
    required = EXCLUDED.required,
    minimum_version = EXCLUDED.minimum_version,
    metadata = EXCLUDED.metadata;
```

**Step 3: Add upsert queries for SQLite**

Same logic but with `?` placeholders instead of `$N`.

In `internal/storage/sqlc/queries/sqlite/applications.sql`:

```sql
-- name: UpsertApplication :exec
INSERT INTO applications (id, name, description, platforms, config, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name,
    description = excluded.description,
    platforms = excluded.platforms,
    config = excluded.config,
    updated_at = excluded.updated_at;
```

In `internal/storage/sqlc/queries/sqlite/releases.sql`:

```sql
-- name: UpsertRelease :exec
INSERT INTO releases (
    id, application_id, version, platform, architecture, download_url,
    checksum, checksum_type, file_size, release_notes, release_date,
    required, minimum_version, metadata, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (application_id, version, platform, architecture) DO UPDATE SET
    id = excluded.id,
    download_url = excluded.download_url,
    checksum = excluded.checksum,
    checksum_type = excluded.checksum_type,
    file_size = excluded.file_size,
    release_notes = excluded.release_notes,
    release_date = excluded.release_date,
    required = excluded.required,
    minimum_version = excluded.minimum_version,
    metadata = excluded.metadata;
```

**Step 4: Regenerate sqlc code**

Run: `make sqlc-generate`

**Step 5: Remove old Create/Update queries if no longer needed**

Check if `CreateApplication`, `UpdateApplication`, `CreateRelease`, `UpdateRelease` are used anywhere other than `SaveApplication`/`SaveRelease`. If they are only used by the upsert pattern, remove them from the SQL files and regenerate. If they are used elsewhere (e.g., specific create-only paths), keep them.

**Step 6: Simplify PostgresStorage.SaveApplication**

Replace the read-then-write pattern in `internal/storage/postgres.go` (lines 79-106):

```go
func (ps *PostgresStorage) SaveApplication(ctx context.Context, app *models.Application) error {
	params, err := modelToPgUpsertApp(app)
	if err != nil {
		return fmt.Errorf("failed to convert application for upsert: %w", err)
	}
	if err := ps.queries.UpsertApplication(ctx, params); err != nil {
		return fmt.Errorf("failed to upsert application: %w", err)
	}
	return nil
}
```

Write `modelToPgUpsertApp` in `dbconvert.go` (or reuse `modelToPgCreateApp` since the upsert uses the same column set as create).

**Step 7: Simplify PostgresStorage.SaveRelease**

Same pattern -- replace lines 155-187 with a single `UpsertRelease` call.

**Step 8: Simplify SQLiteStorage.SaveApplication and SaveRelease**

Same pattern for SQLite (lines 107-134 and 183-215 in `sqlite.go`).

**Step 9: Remove unused conversion functions**

If `modelToPgCreateApp`, `modelToPgUpdateApp`, `modelToPgCreateRelease`, `modelToPgUpdateRelease` (and SQLite equivalents) are no longer called, delete them.

**Step 10: Run tests**

Run: `make test`
Expected: All tests pass. The upsert behavior is identical to the previous read-then-write.

**Step 11: Commit**

```
refactor: replace SELECT + INSERT/UPDATE with SQL upserts

Use INSERT ... ON CONFLICT DO UPDATE for SaveApplication and
SaveRelease in both PostgreSQL and SQLite. Eliminates TOCTOU
race condition and simplifies provider code.
```

---

### Task 7: Remove Admin UI

**Files:**
- Delete: `internal/api/handlers_admin.go`
- Delete: `internal/api/handlers_admin_test.go`
- Delete: `internal/api/middleware_admin.go`
- Delete: `internal/api/middleware_admin_test.go`
- Delete: `internal/api/admin/` directory (templates)
- Modify: `internal/api/routes.go` (remove admin UI route block, lines 74-98)
- Modify: `internal/api/handlers.go` (remove template-related code if any)
- Modify: `internal/api/openapi/openapi.yaml` (remove admin UI endpoints if documented)

**Step 1: Delete admin UI files**

- Delete `internal/api/handlers_admin.go`
- Delete `internal/api/handlers_admin_test.go`
- Delete `internal/api/middleware_admin.go`
- Delete `internal/api/middleware_admin_test.go`
- Delete `internal/api/admin/` directory (entire templates directory)

**Step 2: Remove admin routes from routes.go**

In `internal/api/routes.go`, remove lines 74-98 (the entire admin router block starting with `// Admin UI` through the last `adminRouter.HandleFunc` call).

**Step 3: Check for compilation errors**

The `Handlers` struct may reference admin handler methods that no longer exist. Since the handlers were defined in `handlers_admin.go`, deleting that file removes both the methods and their definitions. The route registrations in `routes.go` are the only call sites. Removing both should compile cleanly.

Check if `handlers_admin.go` exported any functions or types used elsewhere (e.g., `ParseAdminTemplates`). If `ParseAdminTemplates` is called in `cmd/updater/updater.go` or elsewhere, remove that call site too.

**Step 4: Remove allPlatforms/allArchitectures if admin-only**

Check if `allPlatforms` and `allArchitectures` (defined in `handlers_admin.go` lines 123-124) are referenced outside the admin handlers. If not, they are deleted along with the file.

**Step 5: Run tests**

Run: `make test`
Expected: All tests pass. Admin-specific tests are deleted along with the admin files.

**Step 6: Update documentation**

- Update `docs/ARCHITECTURE.md`: remove any admin UI references
- Update CLAUDE.md: add note about admin UI removal

**Step 7: Commit**

```
refactor: remove admin UI

Delete the HTML/cookie-authenticated admin interface. The REST
API admin endpoints (/api/v1/admin/keys) remain available. All
admin operations can be performed via Swagger UI at /api/v1/docs
or any HTTP client.
```

---

### Task 8: Add Application Metrics

**Files:**
- Create: `internal/observability/httpmiddleware.go` (HTTP metrics middleware)
- Create: `internal/observability/httpmiddleware_test.go`
- Modify: `internal/observability/metrics.go` (register application metrics)
- Modify: `internal/api/routes.go` (add metrics middleware to chain)
- Modify: `internal/api/handlers.go` (emit update-check and register metrics)
- Modify: `docs/observability.md` (document new metrics)

**Step 1: Write failing test for HTTP metrics middleware**

Create `internal/observability/httpmiddleware_test.go`:

```go
package observability_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"updater/internal/observability"

	"github.com/stretchr/testify/assert"
)

func TestMetricsMiddleware_RecordsRequestCount(t *testing.T) {
	// Create a provider and middleware
	provider, err := observability.NewProvider(observability.ProviderConfig{
		ServiceName: "test",
	})
	assert.NoError(t, err)
	defer provider.Shutdown(t.Context())

	middleware := observability.NewMetricsMiddleware(provider)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/updates/myapp/check", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Verify middleware doesn't break request flow
}
```

**Step 2: Run test to verify it fails**

Run: `make test`
Expected: FAIL -- `NewMetricsMiddleware` not defined.

**Step 3: Implement HTTP metrics middleware**

Create `internal/observability/httpmiddleware.go`:

```go
package observability

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsMiddleware records HTTP request count and latency.
type MetricsMiddleware struct {
	requestsTotal metric.Int64Counter
	requestDuration metric.Float64Histogram
}

// NewMetricsMiddleware creates middleware that records HTTP metrics.
func NewMetricsMiddleware(provider *Provider) func(http.Handler) http.Handler {
	meter := provider.MeterProvider().Meter("updater.http")

	requestsTotal, _ := meter.Int64Counter("updater_http_requests_total",
		metric.WithDescription("Total HTTP requests"),
		metric.WithUnit("{request}"),
	)

	requestDuration, _ := meter.Float64Histogram("updater_http_request_duration_seconds",
		metric.WithDescription("HTTP request latency in seconds"),
		metric.WithUnit("s"),
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sw, r)

			elapsed := time.Since(start).Seconds()
			attrs := []attribute.KeyValue{
				attribute.String("method", r.Method),
				attribute.String("path", r.URL.Path),
				attribute.String("status", strconv.Itoa(sw.status)),
			}
			requestsTotal.Add(r.Context(), 1, metric.WithAttributes(attrs...))
			requestDuration.Record(r.Context(), elapsed, metric.WithAttributes(attrs...))
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
	written bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.written {
		sw.status = code
		sw.written = true
	}
	sw.ResponseWriter.WriteHeader(code)
}
```

**Step 4: Run tests**

Run: `make test`
Expected: All tests pass.

**Step 5: Add business metrics (update checks, registrations)**

Add counters to the `Handlers` struct or emit from handler functions. The simplest approach is to add counters to the observability package and call them from handlers.

Add to `internal/observability/httpmiddleware.go` (or a separate `appmetrics.go` file):

```go
// AppMetrics holds application-level business metrics.
type AppMetrics struct {
	UpdateChecks       metric.Int64Counter
	ReleasesRegistered metric.Int64Counter
}

// NewAppMetrics creates application-level business metric instruments.
func NewAppMetrics(provider *Provider) *AppMetrics {
	meter := provider.MeterProvider().Meter("updater.app")

	updateChecks, _ := meter.Int64Counter("updater_update_checks_total",
		metric.WithDescription("Total update check requests"),
		metric.WithUnit("{check}"),
	)

	releasesRegistered, _ := meter.Int64Counter("updater_releases_registered_total",
		metric.WithDescription("Total releases registered"),
		metric.WithUnit("{release}"),
	)

	return &AppMetrics{
		UpdateChecks:       updateChecks,
		ReleasesRegistered: releasesRegistered,
	}
}
```

**Step 6: Wire metrics middleware into routes**

In `internal/api/routes.go`, add a `RouteOption` similar to `WithOTelMiddleware`:

```go
func WithMetricsMiddleware(provider *observability.Provider) RouteOption {
	return func(r *mux.Router) {
		r.Use(observability.NewMetricsMiddleware(provider))
	}
}
```

Wire it in `cmd/updater/updater.go` when setting up routes.

**Step 7: Wire business metrics into handlers**

Add `*observability.AppMetrics` to the `Handlers` struct. In `CheckForUpdates` and `RegisterRelease`, emit counter increments with `app_id` and result labels.

**Step 8: Run all tests**

Run: `make test`
Expected: All tests pass.

**Step 9: Update documentation**

Update `docs/observability.md` with the new metrics table:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `updater_http_requests_total` | Counter | `method`, `path`, `status` | Total HTTP requests |
| `updater_http_request_duration_seconds` | Histogram | `method`, `path` | Request latency |
| `updater_update_checks_total` | Counter | `app_id`, `result` | Update check outcomes |
| `updater_releases_registered_total` | Counter | `app_id` | New releases registered |

**Step 10: Commit**

```
feat: add application-level Prometheus metrics

Add HTTP request count/latency middleware and business metrics
for update checks and release registrations. The metrics endpoint
now reports meaningful application behavior alongside the existing
storage operation metrics.
```

---

### Task 9: Update CLAUDE.md and Final Cleanup

**Files:**
- Modify: `CLAUDE.md` (fix stale references)
- Modify: `docs/ARCHITECTURE.md` (align with new state)
- Modify: `docs/plans/2026-02-27-architecture-review.md` (mark items as resolved)

**Step 1: Fix CLAUDE.md**

- Remove mention of `internal/ratelimit/` (already gone)
- Remove CacheConfig references
- Update storage provider list to: Memory, SQLite, PostgreSQL (no JSON)
- Remove factory pattern documentation
- Update Key Patterns to reflect changes (no SecurityContext, upserts, etc.)

**Step 2: Update ARCHITECTURE.md**

- Remove JSON storage provider section
- Remove CacheConfig environment variables
- Note admin UI removal
- Update storage provider table

**Step 3: Run final validation**

Run: `make check` (format + vet + test)
Expected: All pass.

**Step 4: Commit**

```
docs: update CLAUDE.md and ARCHITECTURE.md for cleanup changes

Align documentation with post-cleanup codebase state: no JSON
provider, no factory pattern, no admin UI, no cache config,
simplified ApplicationConfig.
```