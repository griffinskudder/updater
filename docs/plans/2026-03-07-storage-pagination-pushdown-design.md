# Storage Pagination Pushdown Design

**Date:** 2026-03-07
**Issue:** [#35 Storage Pagination Pushdown for ListApplications and ListReleases](https://github.com/griffinskudder/updater/issues/35)
**Status:** Approved

## Problem

`ListApplications` and `ListReleases` load all records into memory before filtering,
sorting, and paginating in Go. This works for small datasets but does not scale: a
table with thousands of releases causes a full load on every list request.

Additionally, internal callers of `Releases()` (stats computation, stable-release
search, delete guard) are using a full table load for operations that are better
served by targeted SQL queries.

## Approach

Move filtering, sorting, and pagination into SQL. Replace the two full-load storage
methods (`Applications`, `Releases`) with purpose-built methods. Remove the
in-memory helpers in the service layer that become redundant.

## Design

### Storage Interface

**Remove:**

| Method | Reason |
|--------|--------|
| `Applications(ctx) ([]*Application, error)` | Single caller; replaced by paged version |
| `Releases(ctx, appID) ([]*Release, error)` | All callers replaced by targeted methods |

**Add:**

```go
// ListApplicationsPaged returns a page of applications and the total count.
ListApplicationsPaged(ctx context.Context, limit, offset int) ([]*models.Application, int, error)

// ListReleasesPaged returns a filtered, sorted page of releases and the total count.
ListReleasesPaged(ctx context.Context, appID string, filters models.ReleaseFilters, sortBy, sortOrder string, limit, offset int) ([]*models.Release, int, error)

// GetLatestStableRelease returns the highest non-prerelease version for the given
// application, platform, and architecture. Returns storage.ErrNotFound if none exists.
GetLatestStableRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error)

// GetApplicationStats returns aggregate statistics for an application.
GetApplicationStats(ctx context.Context, appID string) (models.ApplicationStats, error)
```

### models.ReleaseFilters

New struct in `internal/models/`:

```go
type ReleaseFilters struct {
    Platform     string
    Architecture string
    Version      string
    Required     *bool
    Platforms    []string // OR filter across multiple platforms
}
```

### Schema Migration

New migration `005_version_sort_fields.sql` for both PostgreSQL and SQLite adds four
columns to `releases`:

```sql
ALTER TABLE releases ADD COLUMN version_major     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_minor     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_patch     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_pre_release TEXT;  -- NULL for stable releases
```

Backfill existing rows from the `version` TEXT column using a Go migration helper
(the semver library is already available).

Composite index for sorted list queries:

```sql
CREATE INDEX idx_releases_version_sort
    ON releases(application_id, version_major DESC, version_minor DESC, version_patch DESC);
```

The existing `version` TEXT column is unchanged. It remains the business key and is
returned in all API responses. The new columns are sort-only.

#### Version Sort Ordering

```sql
ORDER BY version_major DESC,
         version_minor DESC,
         version_patch DESC,
         (version_pre_release IS NULL) DESC,  -- stable > pre-release
         version_pre_release ASC              -- approximate pre-release ordering
```

Pre-release string ordering is approximate: `alpha < beta < rc` sorts correctly, but
numeric identifiers within pre-release tags (e.g. `beta.2` vs `beta.11`) are not
guaranteed to sort correctly. This is acceptable for a software update service where
pre-releases are uncommon.

### SQL Queries

#### ListApplicationsPaged

Single query using `COUNT(*) OVER()` window function (supported in both PostgreSQL
and SQLite 3.25+):

```sql
SELECT *, COUNT(*) OVER() AS total_count
FROM applications
ORDER BY name
LIMIT $1 OFFSET $2;
```

#### ListReleasesPaged

The dynamic `ORDER BY` (five sort columns x two directions) cannot be expressed in a
single static sqlc query. This query is written as raw SQL in the storage providers,
built at call time with an allowlist-validated `ORDER BY` clause. Filters use nullable
parameters. Total count via `COUNT(*) OVER()`.

Supported sort columns: `release_date` (default), `version`, `platform`,
`architecture`, `created_at`.

#### GetLatestStableRelease

Static sqlc query:

```sql
SELECT * FROM releases
WHERE application_id = $1 AND platform = $2 AND architecture = $3
  AND version_pre_release IS NULL
ORDER BY version_major DESC, version_minor DESC, version_patch DESC
LIMIT 1;
```

#### GetApplicationStats

Single sqlc query using a correlated subquery for `latest_version`:

```sql
SELECT
    COUNT(*)                         AS total_releases,
    COUNT(*) FILTER (WHERE required) AS required_releases,
    COUNT(DISTINCT platform)         AS platform_count,
    MAX(release_date)                AS latest_release_date,
    (
        SELECT version FROM releases r2
        WHERE r2.application_id = $1
        ORDER BY r2.version_major DESC, r2.version_minor DESC, r2.version_patch DESC,
                 (r2.version_pre_release IS NULL) DESC,
                 r2.version_pre_release ASC
        LIMIT 1
    ) AS latest_version
FROM releases WHERE application_id = $1;
```

Note: SQLite does not support `COUNT(*) FILTER (WHERE ...)`. The SQLite variant uses
`SUM(CASE WHEN required THEN 1 ELSE 0 END)` instead.

### Service Layer

| Removed | Replacement |
|---------|-------------|
| `sortReleases()` | Deleted; no callers remain |
| `computeApplicationStats()` | Deleted; no callers remain |
| `findLatestStableRelease()` | Deleted; replaced by `storage.GetLatestStableRelease` |

**`ListApplications`** — replace `storage.Applications()` + in-memory slice with
`storage.ListApplicationsPaged(ctx, limit, offset)`.

**`ListReleases`** — replace `storage.Releases()` + in-memory filter/sort/slice with
`storage.ListReleasesPaged(ctx, appID, filters, sortBy, sortOrder, limit, offset)`.
Construct `models.ReleaseFilters` from the request fields.

**`GetApplication`** — replace `storage.Releases()` + `computeApplicationStats()`
with `storage.GetApplicationStats(ctx, appID)`.

**`CheckForUpdate` / `GetLatestVersion`** — replace `findLatestStableRelease()` calls
with `storage.GetLatestStableRelease(ctx, appID, platform, arch)`.

**`DeleteApplication`** — remove the `storage.Releases()` pre-check. The FK
constraint in `003_fk_restrict.sql` already enforces this and returns
`storage.ErrHasDependencies`, which the existing error handler already covers.

### Testing

**Storage providers** (memory, postgres, sqlite) — new table-driven test cases for
each new method:

- `ListApplicationsPaged`: correct items, total count, boundary behaviour (offset
  beyond end, limit=1)
- `ListReleasesPaged`: each filter field, each sort column and direction, pagination
  boundaries, empty result
- `GetLatestStableRelease`: stable-over-prerelease preference, platform/arch
  isolation, not-found case
- `GetApplicationStats`: counts, latest version with mixed stable/prerelease, empty
  application

**Service layer** — existing tests use the memory provider as a fake. Tests for
`ListReleases`, `GetApplication`, `CheckForUpdate`, and `GetLatestVersion` are updated
to remove assertions on the deleted helpers and verify behaviour through the updated
storage calls.

**Integration tests** — existing pagination assertions for
`GET /api/v1/admin/applications` and `GET /api/v1/admin/applications/{id}/releases`
remain. Verify `total_count`, `page`, and `has_more` remain correct after the
pushdown.

## Files to Modify

| File | Change |
|------|--------|
| `internal/storage/interface.go` | Remove `Applications`, `Releases`; add four new methods |
| `internal/models/request.go` | Add `ReleaseFilters` struct |
| `internal/storage/memory.go` | Implement new methods |
| `internal/storage/memory_test.go` | Add tests for new methods |
| `internal/storage/postgres.go` | Implement new methods |
| `internal/storage/postgres_test.go` | Add tests for new methods |
| `internal/storage/sqlite.go` | Implement new methods |
| `internal/storage/sqlite_test.go` | Add tests for new methods |
| `internal/storage/sqlc/schema/postgres/005_version_sort_fields.sql` | New migration |
| `internal/storage/sqlc/schema/sqlite/005_version_sort_fields.sql` | New migration |
| `internal/storage/sqlc/queries/postgres/applications.sql` | Add paged query |
| `internal/storage/sqlc/queries/postgres/releases.sql` | Add stable/stats queries |
| `internal/storage/sqlc/queries/sqlite/applications.sql` | Add paged query |
| `internal/storage/sqlc/queries/sqlite/releases.sql` | Add stable/stats queries |
| `internal/update/service.go` | Update callers; remove three helper functions |
| `internal/update/service_test.go` | Update tests |
| `internal/integration/integration_test.go` | Verify pagination still correct |
| `docs/storage.md` | Document new interface methods |