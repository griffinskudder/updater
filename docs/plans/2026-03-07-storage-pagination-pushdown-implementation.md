# Storage Pagination Pushdown Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Push filtering, sorting, and pagination for `ListApplications` and `ListReleases` into SQL, eliminating full-table memory loads.

**Architecture:** Add `version_major/minor/patch/pre_release` columns to `releases` for SQL-level semver ordering. Replace the broad `Applications()` and `Releases()` storage methods with purpose-built methods (`ListApplicationsPaged`, `ListReleasesPaged`, `GetLatestStableRelease`, `GetApplicationStats`). Remove the in-memory helpers `sortReleases`, `computeApplicationStats`, and `findLatestStableRelease` from the service layer.

**Tech Stack:** Go 1.25, sqlc v1.30 (packages `sqlcpg`/`sqlcite`), pgx/v5 (postgres), modernc.org/sqlite, Masterminds/semver/v3

**Design doc:** `docs/plans/2026-03-07-storage-pagination-pushdown-design.md`

---

## Setup: Create a worktree

```bash
git worktree add ../.worktrees/pagination-pushdown -b feat/pagination-pushdown
cd ../.worktrees/pagination-pushdown
```

All subsequent work happens in `../.worktrees/pagination-pushdown`.

---

### Task 1: Schema migration — add version sort columns

**Files:**
- Create: `internal/storage/sqlc/schema/postgres/005_version_sort_fields.sql`
- Create: `internal/storage/sqlc/schema/sqlite/005_version_sort_fields.sql`

**Step 1: Create postgres migration**

```sql
-- internal/storage/sqlc/schema/postgres/005_version_sort_fields.sql
ALTER TABLE releases ADD COLUMN version_major     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_minor     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_patch     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_pre_release TEXT;

-- Backfill existing rows using postgres regex
UPDATE releases SET
    version_major      = (regexp_matches(version, '^(\d+)\.'))[1]::integer,
    version_minor      = (regexp_matches(version, '^\d+\.(\d+)\.'))[1]::integer,
    version_patch      = (regexp_matches(version, '^\d+\.\d+\.(\d+)'))[1]::integer,
    version_pre_release = NULLIF(
        coalesce((regexp_matches(version, '^\d+\.\d+\.\d+-([^+]+)'))[1], ''),
        ''
    )
WHERE version ~ '^\d+\.\d+\.\d+';

CREATE INDEX idx_releases_version_sort
    ON releases(application_id, version_major DESC, version_minor DESC, version_patch DESC);
```

**Step 2: Create sqlite migration**

SQLite lacks regex functions. Extract major/minor/patch with `instr`/`substr`. `version_pre_release` is left NULL for existing rows (acceptable; re-saving a release populates it).

```sql
-- internal/storage/sqlc/schema/sqlite/005_version_sort_fields.sql
ALTER TABLE releases ADD COLUMN version_major      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_minor      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_patch      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_pre_release TEXT;

-- Backfill major (before first dot)
UPDATE releases
SET version_major = CAST(
    substr(version, 1, instr(version, '.') - 1)
AS INTEGER)
WHERE instr(version, '.') > 0;

-- Backfill minor (between first and second dot)
UPDATE releases
SET version_minor = CAST(
    substr(
        substr(version, instr(version, '.') + 1),
        1,
        instr(substr(version, instr(version, '.') + 1), '.') - 1
    )
AS INTEGER)
WHERE instr(version, '.') > 0
  AND instr(substr(version, instr(version, '.') + 1), '.') > 0;

-- Backfill patch (after second dot, before - or +)
UPDATE releases
SET version_patch = CAST(
    CASE
        WHEN instr(
            substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
            '-'
        ) > 0
        THEN substr(
            substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
            1,
            instr(
                substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
                '-'
            ) - 1
        )
        ELSE substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2)
    END
AS INTEGER)
WHERE instr(version, '.') > 0
  AND instr(substr(version, instr(version, '.') + 1), '.') > 0;

CREATE INDEX idx_releases_version_sort
    ON releases(application_id, version_major DESC, version_minor DESC, version_patch DESC);
```

**Step 3: Verify make targets still build**

```bash
make sqlc-vet
```

Expected: passes (new columns not yet in queries).

**Step 4: Commit**

```bash
git add internal/storage/sqlc/schema/
git commit -m "feat: add version sort columns migration (005)"
```

---

### Task 2: Update UpsertRelease SQL queries + add static sqlc queries

**Files:**
- Modify: `internal/storage/sqlc/queries/postgres/releases.sql`
- Modify: `internal/storage/sqlc/queries/sqlite/releases.sql`
- Modify: `internal/storage/sqlc/queries/postgres/applications.sql`
- Modify: `internal/storage/sqlc/queries/sqlite/applications.sql`

**Step 1: Update postgres releases.sql**

Replace the existing `UpsertRelease` with one that includes the new columns, and add `GetLatestStableRelease` and `GetApplicationStats`:

```sql
-- Replace UpsertRelease (add version sort columns):
-- name: UpsertRelease :exec
INSERT INTO releases (
    id, application_id, version, platform, architecture, download_url,
    checksum, checksum_type, file_size, release_notes, release_date,
    required, minimum_version, metadata, created_at,
    version_major, version_minor, version_patch, version_pre_release
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
ON CONFLICT (application_id, version, platform, architecture) DO UPDATE SET
    download_url        = EXCLUDED.download_url,
    checksum            = EXCLUDED.checksum,
    checksum_type       = EXCLUDED.checksum_type,
    file_size           = EXCLUDED.file_size,
    release_notes       = EXCLUDED.release_notes,
    release_date        = EXCLUDED.release_date,
    required            = EXCLUDED.required,
    minimum_version     = EXCLUDED.minimum_version,
    metadata            = EXCLUDED.metadata,
    version_major       = EXCLUDED.version_major,
    version_minor       = EXCLUDED.version_minor,
    version_patch       = EXCLUDED.version_patch,
    version_pre_release = EXCLUDED.version_pre_release;

-- Add at end of file:
-- name: GetLatestStableRelease :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE application_id = $1 AND platform = $2 AND architecture = $3
  AND version_pre_release IS NULL
ORDER BY version_major DESC, version_minor DESC, version_patch DESC
LIMIT 1;

-- name: GetApplicationStats :one
SELECT
    COUNT(*) AS total_releases,
    COUNT(*) FILTER (WHERE required) AS required_releases,
    COUNT(DISTINCT platform) AS platform_count,
    MAX(release_date) AS latest_release_date,
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

**Step 2: Update sqlite releases.sql**

Same changes but `?` params and `SUM(CASE...)` instead of `COUNT(*) FILTER`:

```sql
-- Replace UpsertRelease:
-- name: UpsertRelease :exec
INSERT INTO releases (
    id, application_id, version, platform, architecture, download_url,
    checksum, checksum_type, file_size, release_notes, release_date,
    required, minimum_version, metadata, created_at,
    version_major, version_minor, version_patch, version_pre_release
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (application_id, version, platform, architecture) DO UPDATE SET
    download_url        = excluded.download_url,
    checksum            = excluded.checksum,
    checksum_type       = excluded.checksum_type,
    file_size           = excluded.file_size,
    release_notes       = excluded.release_notes,
    release_date        = excluded.release_date,
    required            = excluded.required,
    minimum_version     = excluded.minimum_version,
    metadata            = excluded.metadata,
    version_major       = excluded.version_major,
    version_minor       = excluded.version_minor,
    version_patch       = excluded.version_patch,
    version_pre_release = excluded.version_pre_release;

-- Add at end:
-- name: GetLatestStableRelease :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE application_id = ? AND platform = ? AND architecture = ?
  AND version_pre_release IS NULL
ORDER BY version_major DESC, version_minor DESC, version_patch DESC
LIMIT 1;

-- name: GetApplicationStats :one
SELECT
    COUNT(*) AS total_releases,
    SUM(CASE WHEN required THEN 1 ELSE 0 END) AS required_releases,
    COUNT(DISTINCT platform) AS platform_count,
    MAX(release_date) AS latest_release_date,
    (
        SELECT version FROM releases r2
        WHERE r2.application_id = ?
        ORDER BY r2.version_major DESC, r2.version_minor DESC, r2.version_patch DESC,
                 (r2.version_pre_release IS NULL) DESC,
                 r2.version_pre_release ASC
        LIMIT 1
    ) AS latest_version
FROM releases WHERE application_id = ?;
```

**Step 3: Add paged applications query to postgres applications.sql**

```sql
-- Add at end of file:
-- name: GetApplicationsPaged :many
SELECT id, name, description, platforms, config, created_at, updated_at,
       COUNT(*) OVER() AS total_count
FROM applications
ORDER BY name
LIMIT $1 OFFSET $2;
```

**Step 4: Add paged applications query to sqlite applications.sql**

```sql
-- Add at end of file:
-- name: GetApplicationsPaged :many
SELECT id, name, description, platforms, config, created_at, updated_at,
       COUNT(*) OVER() AS total_count
FROM applications
ORDER BY name
LIMIT ? OFFSET ?;
```

**Step 5: Run sqlc generate**

```bash
make sqlc-generate
```

Expected: regenerates `internal/storage/sqlc/postgres/` and `internal/storage/sqlc/sqlite/`. Check that new `GetApplicationsPaged`, `GetLatestStableRelease`, `GetApplicationStats` functions appear in `releases.sql.go` and `applications.sql.go`.

**Step 6: Commit**

```bash
git add internal/storage/sqlc/
git commit -m "feat: add version sort columns to UpsertRelease and static sqlc queries"
```

---

### Task 3: Update SaveRelease to populate version sort columns

**Files:**
- Modify: `internal/storage/postgres.go` (`modelToPgUpsertRelease`)
- Modify: `internal/storage/sqlite.go` (equivalent converter)
- Modify: `internal/storage/memory.go` (`SaveRelease`)
- Test: `internal/storage/postgres_test.go`
- Test: `internal/storage/sqlite_test.go`
- Test: `internal/storage/memory_test.go`

The `Masterminds/semver` library is already imported. Parse the version string and extract parts.

**Step 1: Write failing test in postgres_test.go**

Find the existing `TestSaveRelease` (or similar) table-driven test and add a case that checks the version sort columns are populated. The test reads back the saved release via `GetRelease` and asserts on version fields. Since the generated `Release` struct now includes `VersionMajor` etc., assert them:

```go
// In the postgres provider test, after SaveRelease + GetRelease:
// For a release with Version "2.3.4-beta.1":
assert.Equal(t, int32(2), row.VersionMajor)
assert.Equal(t, int32(3), row.VersionMinor)
assert.Equal(t, int32(4), row.VersionPatch)
assert.Equal(t, pgtype.Text{String: "beta.1", Valid: true}, row.VersionPreRelease)
// For a stable release "1.0.0":
assert.Equal(t, pgtype.Text{Valid: false}, row.VersionPreRelease)
```

**Step 2: Run test to confirm it fails**

```bash
make test
```

Expected: compilation error because `modelToPgUpsertRelease` doesn't populate the new fields yet.

**Step 3: Write a semver parse helper in dbconvert.go**

```go
// parseSemverParts extracts major, minor, patch, and pre-release from a semver string.
// Returns zeros and empty string if the version cannot be parsed.
func parseSemverParts(version string) (major, minor, patch int64, preRelease string) {
    v, err := semver.NewVersion(version)
    if err != nil {
        return 0, 0, 0, ""
    }
    return int64(v.Major()), int64(v.Minor()), int64(v.Patch()), v.Prerelease()
}
```

Add `"github.com/Masterminds/semver/v3"` to the import in `dbconvert.go` (already in go.mod).

**Step 4: Update modelToPgUpsertRelease in postgres.go**

Find `modelToPgUpsertRelease` and add the four new fields to the returned params struct. Use `parseSemverParts`:

```go
major, minor, patch, pre := parseSemverParts(release.Version)
// In the params struct:
VersionMajor:      int32(major),
VersionMinor:      int32(minor),
VersionPatch:      int32(patch),
VersionPreRelease: pgtype.Text{String: pre, Valid: pre != ""},
```

**Step 5: Update the sqlite equivalent**

Find the equivalent upsert params builder in `sqlite.go` and populate the same four fields (SQLite uses `sql.NullString` or plain `string` depending on generated type — check the generated `sqlcite.UpsertReleaseParams` struct after sqlc-generate).

**Step 6: Update MemoryStorage.SaveRelease**

The memory provider doesn't use SQL. The `models.Release` struct needs the version sort fields added, or the memory provider computes them at query time. Since the sort columns are SQL-only, the memory provider simply stores the full `models.Release` and computes sort order inline when listing. No change to `SaveRelease` in memory.

**Step 7: Run tests**

```bash
make test
```

Expected: all tests pass.

**Step 8: Commit**

```bash
git add internal/storage/
git commit -m "feat: populate version sort columns on SaveRelease"
```

---

### Task 4: Add ReleaseFilters to models

**Files:**
- Modify: `internal/models/request.go`

**Step 1: Add the struct**

In `internal/models/request.go`, after the existing request types:

```go
// ReleaseFilters specifies optional filters for paginated release queries.
// An empty string or nil value means no filter is applied for that field.
// Platforms is an OR filter: a release matches if its platform equals any entry.
// If both Platform (via the caller) and Platforms are set, Platforms takes precedence.
type ReleaseFilters struct {
    Platforms    []string
    Architecture string
    Version      string
    Required     *bool
}
```

**Step 2: Run build to confirm no compile errors**

```bash
make build
```

**Step 3: Commit**

```bash
git add internal/models/request.go
git commit -m "feat: add ReleaseFilters struct to models"
```

---

### Task 5: Add new methods to the Storage interface (additive)

**Files:**
- Modify: `internal/storage/interface.go`

**Step 1: Add the four new methods**

```go
// ListApplicationsPaged returns a page of applications sorted by name,
// and the total count of all applications.
ListApplicationsPaged(ctx context.Context, limit, offset int) ([]*models.Application, int, error)

// ListReleasesPaged returns a filtered, sorted page of releases for an application,
// and the total count of matching releases.
// sortBy must be one of: release_date, version, platform, architecture, created_at.
// sortOrder must be "asc" or "desc".
ListReleasesPaged(ctx context.Context, appID string, filters models.ReleaseFilters, sortBy, sortOrder string, limit, offset int) ([]*models.Release, int, error)

// GetLatestStableRelease returns the highest non-prerelease version for the given
// application, platform, and architecture.
// Returns storage.ErrNotFound if no stable release exists.
GetLatestStableRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error)

// GetApplicationStats returns aggregate statistics for an application.
GetApplicationStats(ctx context.Context, appID string) (models.ApplicationStats, error)
```

**Step 2: Run build**

```bash
make build
```

Expected: compilation errors because the three providers (memory, postgres, sqlite) don't implement the new methods yet. That is expected at this step.

**Step 3: Commit**

```bash
git add internal/storage/interface.go
git commit -m "feat: add ListApplicationsPaged, ListReleasesPaged, GetLatestStableRelease, GetApplicationStats to Storage interface"
```

---

### Task 6: Implement new methods in MemoryStorage

**Files:**
- Modify: `internal/storage/memory.go`
- Modify: `internal/storage/memory_test.go`

The memory provider doesn't use SQL. It implements the sort/filter in Go. Version sort uses the semver library directly (exact, not approximate).

**Step 1: Write failing tests in memory_test.go**

Add table-driven tests for each new method. Examples:

```go
func TestMemoryStorage_ListApplicationsPaged(t *testing.T) {
    // seed 5 apps, request page 2 of size 2
    // expect 2 apps returned, total_count=5, correct names
}

func TestMemoryStorage_ListReleasesPaged(t *testing.T) {
    // seed releases with mixed platforms
    // filter by platform, expect only matching releases
    // verify sort order by version DESC
    // verify pagination boundaries (offset > total returns empty slice + correct total)
}

func TestMemoryStorage_GetLatestStableRelease(t *testing.T) {
    // seed: 1.0.0, 2.0.0-beta, 1.5.0
    // expect: 1.5.0 returned (highest stable)
    // seed: only pre-releases → expect ErrNotFound
    // wrong platform → expect ErrNotFound
}

func TestMemoryStorage_GetApplicationStats(t *testing.T) {
    // seed 3 releases: 1 required, 2 platforms, latest=2.0.0-beta
    // expect: total=3, required=1, platform_count=2, latest_version="2.0.0-beta"
    // empty app: expect zero stats, no error
}
```

**Step 2: Run tests to confirm they fail**

```bash
make test
```

Expected: compilation errors (methods not implemented).

**Step 3: Implement ListApplicationsPaged**

```go
func (m *MemoryStorage) ListApplicationsPaged(ctx context.Context, limit, offset int) ([]*models.Application, int, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    apps := make([]*models.Application, 0, len(m.applications))
    for _, app := range m.applications {
        copied := *app
        apps = append(apps, &copied)
    }
    sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })

    total := len(apps)
    if offset >= total {
        return []*models.Application{}, total, nil
    }
    end := offset + limit
    if end > total {
        end = total
    }
    return apps[offset:end], total, nil
}
```

**Step 4: Implement GetLatestStableRelease**

```go
func (m *MemoryStorage) GetLatestStableRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var latest *models.Release
    var latestVer *semver.Version

    for _, r := range m.releases[appID] {
        if r.Platform != platform || r.Architecture != arch {
            continue
        }
        v, err := semver.NewVersion(r.Version)
        if err != nil || v.Prerelease() != "" {
            continue
        }
        if latestVer == nil || v.GreaterThan(latestVer) {
            latestVer = v
            copied := *r
            latest = &copied
        }
    }
    if latest == nil {
        return nil, ErrNotFound
    }
    return latest, nil
}
```

**Step 5: Implement GetApplicationStats**

```go
func (m *MemoryStorage) GetApplicationStats(ctx context.Context, appID string) (models.ApplicationStats, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    releases := m.releases[appID]
    stats := models.ApplicationStats{TotalReleases: len(releases)}
    if len(releases) == 0 {
        return stats, nil
    }

    platforms := make(map[string]struct{})
    var latestVer *semver.Version
    var latestDate *time.Time

    for _, r := range releases {
        platforms[r.Platform] = struct{}{}
        if r.Required {
            stats.RequiredReleases++
        }
        if v, err := semver.NewVersion(r.Version); err == nil {
            if latestVer == nil || v.GreaterThan(latestVer) {
                latestVer = v
                stats.LatestVersion = r.Version
            }
        }
        if latestDate == nil || r.ReleaseDate.After(*latestDate) {
            rd := r.ReleaseDate
            latestDate = &rd
        }
    }
    stats.PlatformCount = len(platforms)
    stats.LatestReleaseDate = latestDate
    return stats, nil
}
```

**Step 6: Implement ListReleasesPaged**

Add a package-private `memorySortReleases` helper (same logic as the service's `sortReleases` — this will be deleted from the service in Task 9):

```go
func memorySortReleases(releases []*models.Release, sortBy, sortOrder string) {
    less := func(i, j int) bool {
        switch sortBy {
        case "version":
            vi, ei := semver.NewVersion(releases[i].Version)
            vj, ej := semver.NewVersion(releases[j].Version)
            if ei == nil && ej == nil {
                return vi.LessThan(vj)
            }
            return releases[i].Version < releases[j].Version
        case "platform":
            return releases[i].Platform < releases[j].Platform
        case "architecture":
            return releases[i].Architecture < releases[j].Architecture
        case "created_at":
            return releases[i].CreatedAt.Before(releases[j].CreatedAt)
        default: // release_date
            return releases[i].ReleaseDate.Before(releases[j].ReleaseDate)
        }
    }
    if sortOrder == "desc" {
        orig := less
        less = func(i, j int) bool { return orig(j, i) }
    }
    sort.Slice(releases, less)
}

func (m *MemoryStorage) ListReleasesPaged(ctx context.Context, appID string, filters models.ReleaseFilters, sortBy, sortOrder string, limit, offset int) ([]*models.Release, int, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var filtered []*models.Release
    for _, r := range m.releases[appID] {
        if filters.Architecture != "" && r.Architecture != filters.Architecture {
            continue
        }
        if filters.Version != "" && r.Version != filters.Version {
            continue
        }
        if filters.Required != nil && r.Required != *filters.Required {
            continue
        }
        if len(filters.Platforms) > 0 {
            match := false
            for _, p := range filters.Platforms {
                if r.Platform == p {
                    match = true
                    break
                }
            }
            if !match {
                continue
            }
        }
        copied := *r
        filtered = append(filtered, &copied)
    }

    memorySortReleases(filtered, sortBy, sortOrder)

    total := len(filtered)
    if offset >= total {
        return []*models.Release{}, total, nil
    }
    end := offset + limit
    if end > total {
        end = total
    }
    return filtered[offset:end], total, nil
}
```

**Step 7: Run tests**

```bash
make test
```

Expected: all tests pass (postgres and sqlite tests will still fail to compile — that's fine, they implement the interface next).

Actually, the build will fail because postgres and sqlite still don't implement the new interface methods. Temporarily add stub implementations if needed, or proceed directly to Task 7.

**Step 8: Commit**

```bash
git add internal/storage/memory.go internal/storage/memory_test.go
git commit -m "feat: implement new paginated methods in MemoryStorage"
```

---

### Task 7: Implement new methods in PostgresStorage

**Files:**
- Modify: `internal/storage/postgres.go`
- Modify: `internal/storage/postgres_test.go`

**Step 1: Write failing tests in postgres_test.go**

Mirror the memory tests: `TestPostgresStorage_ListApplicationsPaged`, `TestPostgresStorage_ListReleasesPaged`, `TestPostgresStorage_GetLatestStableRelease`, `TestPostgresStorage_GetApplicationStats`. These run against a real postgres DB (existing test infra).

**Step 2: Run tests to confirm they fail**

```bash
make test
```

Expected: compile error (interface not satisfied).

**Step 3: Implement GetLatestStableRelease**

```go
func (ps *PostgresStorage) GetLatestStableRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error) {
    row, err := ps.queries.GetLatestStableRelease(ctx, sqlcpg.GetLatestStableReleaseParams{
        ApplicationID: appID,
        Platform:      platform,
        Architecture:  arch,
    })
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrNotFound
        }
        return nil, fmt.Errorf("failed to get latest stable release: %w", err)
    }
    return pgReleaseToModel(row)
}
```

**Step 4: Implement GetApplicationStats**

```go
func (ps *PostgresStorage) GetApplicationStats(ctx context.Context, appID string) (models.ApplicationStats, error) {
    row, err := ps.queries.GetApplicationStats(ctx, appID)
    if err != nil {
        return models.ApplicationStats{}, fmt.Errorf("failed to get application stats: %w", err)
    }

    stats := models.ApplicationStats{
        TotalReleases:    int(row.TotalReleases),
        RequiredReleases: int(row.RequiredReleases),
        PlatformCount:    int(row.PlatformCount),
        LatestVersion:    row.LatestVersion.String, // pgtype.Text
    }
    if row.LatestReleaseDate.Valid {
        t := row.LatestReleaseDate.Time
        stats.LatestReleaseDate = &t
    }
    return stats, nil
}
```

**Step 5: Implement ListApplicationsPaged**

```go
func (ps *PostgresStorage) ListApplicationsPaged(ctx context.Context, limit, offset int) ([]*models.Application, int, error) {
    rows, err := ps.queries.GetApplicationsPaged(ctx, sqlcpg.GetApplicationsPagedParams{
        Limit:  int32(limit),
        Offset: int32(offset),
    })
    if err != nil {
        return nil, 0, fmt.Errorf("failed to list applications: %w", err)
    }

    total := 0
    apps := make([]*models.Application, 0, len(rows))
    for i, row := range rows {
        if i == 0 {
            total = int(row.TotalCount)
        }
        app, err := pgAppToModel(sqlcpg.Application{
            ID: row.ID, Name: row.Name, Description: row.Description,
            Platforms: row.Platforms, Config: row.Config,
            CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
        })
        if err != nil {
            return nil, 0, fmt.Errorf("failed to convert application %s: %w", row.ID, err)
        }
        apps = append(apps, app)
    }
    return apps, total, nil
}
```

Note: check the exact generated struct field names after sqlc-generate. The `GetApplicationsPaged` row type will have all application columns plus `TotalCount int64`.

**Step 6: Implement ListReleasesPaged**

This is a raw SQL method (dynamic ORDER BY). Add an allowlist map and a query builder:

```go
// pgReleaseListSortCols maps sort_by values to safe SQL ORDER BY fragments.
var pgReleaseListSortCols = map[string]string{
    "release_date": "release_date",
    "version":      "version_major DESC, version_minor DESC, version_patch DESC, (version_pre_release IS NULL) DESC, version_pre_release",
    "platform":     "platform",
    "architecture": "architecture",
    "created_at":   "created_at",
}

func (ps *PostgresStorage) ListReleasesPaged(ctx context.Context, appID string, filters models.ReleaseFilters, sortBy, sortOrder string, limit, offset int) ([]*models.Release, int, error) {
    col, ok := pgReleaseListSortCols[sortBy]
    if !ok {
        col = pgReleaseListSortCols["release_date"]
    }

    orderClause := col
    if sortBy != "version" { // version has direction embedded
        if sortOrder == "asc" {
            orderClause += " ASC"
        } else {
            orderClause += " DESC"
        }
    }

    args := []interface{}{appID}
    where := "WHERE application_id = $1"

    if filters.Architecture != "" {
        args = append(args, filters.Architecture)
        where += fmt.Sprintf(" AND architecture = $%d", len(args))
    }
    if filters.Version != "" {
        args = append(args, filters.Version)
        where += fmt.Sprintf(" AND version = $%d", len(args))
    }
    if filters.Required != nil {
        args = append(args, *filters.Required)
        where += fmt.Sprintf(" AND required = $%d", len(args))
    }
    if len(filters.Platforms) > 0 {
        args = append(args, filters.Platforms)
        where += fmt.Sprintf(" AND platform = ANY($%d::text[])", len(args))
    }

    args = append(args, limit, offset)
    query := fmt.Sprintf(`
        SELECT id, application_id, version, platform, architecture, download_url,
               checksum, checksum_type, file_size, release_notes, release_date,
               required, minimum_version, metadata, created_at,
               version_major, version_minor, version_patch, version_pre_release,
               COUNT(*) OVER() AS total_count
        FROM releases
        %s
        ORDER BY %s
        LIMIT $%d OFFSET $%d`,
        where, orderClause, len(args)-1, len(args),
    )

    rows, err := ps.pool.Query(ctx, query, args...)
    if err != nil {
        return nil, 0, fmt.Errorf("failed to list releases: %w", err)
    }
    defer rows.Close()

    var releases []*models.Release
    total := 0
    for rows.Next() {
        var row pgReleaseListRow // define this struct inline or as a type
        if err := rows.Scan(
            &row.ID, &row.ApplicationID, &row.Version, &row.Platform, &row.Architecture,
            &row.DownloadUrl, &row.Checksum, &row.ChecksumType, &row.FileSize,
            &row.ReleaseNotes, &row.ReleaseDate, &row.Required, &row.MinimumVersion,
            &row.Metadata, &row.CreatedAt,
            &row.VersionMajor, &row.VersionMinor, &row.VersionPatch, &row.VersionPreRelease,
            &row.TotalCount,
        ); err != nil {
            return nil, 0, fmt.Errorf("failed to scan release: %w", err)
        }
        if total == 0 {
            total = int(row.TotalCount)
        }
        release, err := pgReleaseToModel(sqlcpg.Release{
            // map fields from row to sqlcpg.Release
        })
        if err != nil {
            return nil, 0, err
        }
        releases = append(releases, release)
    }
    if err := rows.Err(); err != nil {
        return nil, 0, fmt.Errorf("failed to iterate releases: %w", err)
    }
    if releases == nil {
        releases = []*models.Release{}
    }
    return releases, total, nil
}
```

Define `pgReleaseListRow` as a private struct in `postgres.go` to hold the scan targets (all Release fields + TotalCount int64).

**Step 7: Run tests**

```bash
make test
```

Expected: postgres tests pass.

**Step 8: Commit**

```bash
git add internal/storage/postgres.go internal/storage/postgres_test.go
git commit -m "feat: implement paginated storage methods in PostgresStorage"
```

---

### Task 8: Implement new methods in SQLiteStorage

**Files:**
- Modify: `internal/storage/sqlite.go`
- Modify: `internal/storage/sqlite_test.go`

Follow the same pattern as Task 7, with these SQLite-specific differences:

- Parameters use `?` placeholders
- `GetApplicationStats` uses `SUM(CASE WHEN required THEN 1 ELSE 0 END)` (already in sqlc query)
- `ListReleasesPaged` raw SQL uses `?` params (positional: `?`, `?`, ...) — use a `strings.Builder` to append `?` placeholders
- For the Platforms IN filter: build `IN (?, ?, ?)` dynamically
- Use `database/sql` rows scanning (sqlite provider uses `*sql.DB`, not pgxpool)

**Step 1: Write failing tests in sqlite_test.go** (mirror postgres tests)

**Step 2: Implement all four methods** (same structure as postgres, SQLite syntax)

For the Platforms filter in SQLite `ListReleasesPaged`:
```go
if len(filters.Platforms) > 0 {
    placeholders := make([]string, len(filters.Platforms))
    for i, p := range filters.Platforms {
        args = append(args, p)
        placeholders[i] = "?"
    }
    where += fmt.Sprintf(" AND platform IN (%s)", strings.Join(placeholders, ","))
}
```

**Step 3: Run tests**

```bash
make test
```

Expected: all tests pass.

**Step 4: Commit**

```bash
git add internal/storage/sqlite.go internal/storage/sqlite_test.go
git commit -m "feat: implement paginated storage methods in SQLiteStorage"
```

---

### Task 9: Update the service layer

**Files:**
- Modify: `internal/update/service.go`
- Modify: `internal/update/service_test.go`

**Step 1: Write failing tests for the changed service methods**

In `service_test.go`, for each changed method, write a test that uses the updated storage interface. The test setup uses `NewMemoryStorage()` (which now implements the full interface). Verify the same observable behaviour as before.

Key test cases:
- `TestListApplications_Pagination`: verify page/total_count/has_more with 5 apps, limit=2, offset=2
- `TestListReleases_FilterPushdown`: verify platform filter works (not testing in-memory sort, just that results are correct)
- `TestGetApplication_Stats`: verify stats populated from `GetApplicationStats`
- `TestCheckForUpdate_StablePreference`: verify that when latest is pre-release, `GetLatestStableRelease` is used

**Step 2: Run tests to confirm they fail**

```bash
make test
```

**Step 3: Update ListApplications**

Replace:
```go
allApps, err := s.storage.Applications(ctx)
// ... in-memory slice
```
With:
```go
apps, totalCount, err := s.storage.ListApplicationsPaged(ctx, limit, offset)
if err != nil {
    return nil, NewInternalError("failed to list applications", err)
}
// Remove in-memory slicing entirely. totalCount comes from storage.
```

Update the response construction to use `totalCount` directly (no more `len(allApps)`).

**Step 4: Update ListReleases**

Construct `ReleaseFilters` from the request, then call `ListReleasesPaged`:

```go
filters := models.ReleaseFilters{
    Architecture: req.Architecture,
    Version:      req.Version,
    Required:     req.Required,
}
if req.Platform != "" {
    filters.Platforms = []string{req.Platform}
} else {
    filters.Platforms = req.Platforms
}

releases, totalCount, err := s.storage.ListReleasesPaged(ctx, req.ApplicationID, filters, req.SortBy, req.SortOrder, req.Limit, req.Offset)
if err != nil {
    return nil, NewInternalError("failed to get releases", err)
}
```

Remove the in-memory filter loop, `s.sortReleases(...)` call, and in-memory pagination block.

**Step 5: Update GetApplication**

Replace the `s.storage.Releases(ctx, appID)` + `computeApplicationStats(releases)` block with:
```go
stats, err := s.storage.GetApplicationStats(ctx, appID)
if err != nil {
    return nil, NewInternalError("failed to get application stats", err)
}
```

**Step 6: Update CheckForUpdate and GetLatestVersion**

The two call sites of `findLatestStableRelease` become:
```go
stableRelease, err := s.storage.GetLatestStableRelease(ctx, req.ApplicationID, req.Platform, req.Architecture)
if err != nil {
    if errors.Is(err, storage.ErrNotFound) {
        // No stable update available
        response.SetNoUpdateAvailable(req.CurrentVersion)
        return response, nil
    }
    return nil, NewInternalError("failed to find stable release", err)
}
latestRelease = stableRelease
```

**Step 7: Update DeleteApplication**

Remove the block:
```go
releases, err := s.storage.Releases(ctx, appID)
if err != nil {
    return NewInternalError("failed to check releases", err)
}
if len(releases) > 0 {
    return NewConflictError(...)
}
```

The FK constraint fires on delete and returns `storage.ErrHasDependencies`, which the existing `errors.Is(err, storage.ErrHasDependencies)` check already handles correctly.

**Step 8: Delete the three helper functions**

Remove `sortReleases`, `computeApplicationStats`, and `findLatestStableRelease` entirely from `service.go`. Remove the `sort` import if now unused.

**Step 9: Run tests**

```bash
make test
```

Expected: all tests pass.

**Step 10: Commit**

```bash
git add internal/update/service.go internal/update/service_test.go
git commit -m "refactor: push list pagination and stats to storage layer, remove in-memory helpers"
```

---

### Task 10: Remove old interface methods and implementations

**Files:**
- Modify: `internal/storage/interface.go`
- Modify: `internal/storage/memory.go`
- Modify: `internal/storage/postgres.go`
- Modify: `internal/storage/sqlite.go`

**Step 1: Remove from interface.go**

Delete the `Applications` and `Releases` method declarations.

**Step 2: Remove from memory.go**

Delete `func (m *MemoryStorage) Applications(...)` and `func (m *MemoryStorage) Releases(...)`.

**Step 3: Remove from postgres.go**

Delete `func (ps *PostgresStorage) Applications(...)` and `func (ps *PostgresStorage) Releases(...)`.

**Step 4: Remove from sqlite.go**

Delete equivalents.

**Step 5: Run tests**

```bash
make test
```

Expected: all tests pass. If anything in the codebase still references `Applications()` or `Releases()`, the compiler will tell you immediately.

**Step 6: Commit**

```bash
git add internal/storage/
git commit -m "refactor: remove Applications() and Releases() from Storage interface"
```

---

### Task 11: Update integration tests

**Files:**
- Modify: `internal/integration/integration_test.go`

**Step 1: Run integration tests to see current state**

```bash
make integration-test
```

Identify any failures caused by the storage interface changes.

**Step 2: Fix any broken assertions**

The integration tests exercise the full HTTP stack. They don't call storage directly, so most should pass unchanged. However, check:

- Pagination response fields (`total_count`, `page`, `page_size`, `has_more`) are still correct
- `GET /api/v1/admin/applications/{id}` still returns `stats` block

**Step 3: Run integration tests to confirm all pass**

```bash
make integration-test
```

**Step 4: Commit**

```bash
git add internal/integration/
git commit -m "test: verify integration tests pass after pagination pushdown"
```

---

### Task 12: Update docs and mark issue complete

**Files:**
- Modify: `docs/storage.md`

**Step 1: Update storage.md**

Document the four new interface methods under the storage provider section. Remove references to `Applications()` and `Releases()`. Briefly explain the version sort column approach.

**Step 2: Run docs build (optional)**

```bash
make docs-build
```

**Step 3: Commit**

```bash
git add docs/storage.md
git commit -m "docs: update storage reference for pagination pushdown"
```

---

### Task 13: Final check and PR

**Step 1: Run the full check suite**

```bash
make check
make integration-test
```

Expected: all pass.

**Step 2: Push branch and open PR**

Use the GitHub MCP tool to create a pull request targeting `main`. Reference issue #35 in the PR description.