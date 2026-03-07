# Keyset Pagination Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace offset pagination with keyset (cursor) pagination on both list endpoints, and enforce a 500-row maximum page size.

**Architecture:** Cursors are opaque base64-encoded JSON produced by the service layer from the last item on each page. Storage receives a typed cursor struct and appends a keyset WHERE clause to the existing dynamic SQL query. Memory storage finds the cursor item by ID and slices the sorted array. All five release sort fields are supported for cursor-based pagination.

**Tech Stack:** Go 1.25, PostgreSQL (pgx), SQLite (database/sql), Masterminds/semver

**Design doc:** `docs/plans/2026-03-07-keyset-pagination-design.md`

---

## Key Rules Before You Start

- All commands run inside Docker via Makefile targets (`make test`, `make build`).
- CGO is forbidden except in tests where the race detector is used.
- SQLite stores `created_at` and `release_date` as TEXT (ISO8601 strings produced by Go's database/sql + SQLite driver). When building keyset conditions against SQLite timestamps, format the cursor value as `t.UTC().Format("2006-01-02T15:04:05Z07:00")` — verify the exact format by inspecting an existing saved value in a storage test.
- PostgreSQL stores timestamps as TIMESTAMPTZ; pass `time.Time` directly.
- `parseSemverParts` in `internal/storage/dbconvert.go` extracts `major, minor, patch, preRelease` from a version string. Use it when building version cursors from storage results.
- `total_count` in responses is `COUNT(*) OVER()`, which counts rows matching the full WHERE clause including the keyset condition. On the first page (no cursor) this equals the true total. On subsequent pages it equals the remaining count. This is intentional.
- `next_cursor` is populated when `len(page) < totalCount` — see service layer tasks.

---

## Task 1: MaxPageSize constant and ListReleasesRequest.After

**Files:**
- Modify: `internal/models/request.go`
- Modify: `internal/models/request_test.go`

**Step 1: Write the failing tests**

In `internal/models/request_test.go`, add to the `ListReleasesRequest` validation test table:

```go
{
    name: "limit exceeds max page size",
    req:  ListReleasesRequest{ApplicationID: "app", Limit: 501},
    wantErr: true,
},
{
    name: "limit at max page size is valid",
    req:  ListReleasesRequest{ApplicationID: "app", Limit: 500},
    wantErr: false,
},
```

**Step 2: Run to confirm failure**

```
make test
```

Expected: FAIL — MaxPageSize is not defined yet.

**Step 3: Implement**

In `internal/models/request.go`, add after the existing constants:

```go
// MaxPageSize is the maximum number of items that can be requested per page.
const MaxPageSize = 500
```

Add `After string` to `ListReleasesRequest`:

```go
type ListReleasesRequest struct {
    ApplicationID string   `json:"application_id" validate:"required"`
    Platform      string   `json:"platform,omitempty"`
    Architecture  string   `json:"architecture,omitempty"`
    Version       string   `json:"version,omitempty"`
    Required      *bool    `json:"required,omitempty"`
    Limit         int      `json:"limit,omitempty"`
    After         string   `json:"after,omitempty"`
    SortBy        string   `json:"sort_by,omitempty"`
    SortOrder     string   `json:"sort_order,omitempty"`
    Platforms     []string `json:"platforms,omitempty"`
}
```

In `ListReleasesRequest.Validate()`, add after the existing `Limit < 0` check:

```go
if r.Limit > MaxPageSize {
    return fmt.Errorf("limit cannot exceed %d", MaxPageSize)
}
```

**Step 4: Run tests**

```
make test
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/models/request.go internal/models/request_test.go
git commit -m "feat: add MaxPageSize constant and After field to ListReleasesRequest"
```

---

## Task 2: ListApplicationsRequest model

**Files:**
- Modify: `internal/models/request.go`
- Modify: `internal/models/request_test.go`

**Step 1: Write failing tests**

Add a new test function in `internal/models/request_test.go`:

```go
func TestListApplicationsRequest_Validate(t *testing.T) {
    tests := []struct {
        name    string
        req     ListApplicationsRequest
        wantErr bool
    }{
        {name: "valid defaults", req: ListApplicationsRequest{}, wantErr: false},
        {name: "valid limit", req: ListApplicationsRequest{Limit: 100}, wantErr: false},
        {name: "negative limit", req: ListApplicationsRequest{Limit: -1}, wantErr: true},
        {name: "limit exceeds max", req: ListApplicationsRequest{Limit: 501}, wantErr: true},
        {name: "limit at max", req: ListApplicationsRequest{Limit: 500}, wantErr: false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.req.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

func TestListApplicationsRequest_Normalize(t *testing.T) {
    req := ListApplicationsRequest{}
    req.Normalize()
    if req.Limit != 50 {
        t.Errorf("expected default limit 50, got %d", req.Limit)
    }
}
```

**Step 2: Run to confirm failure**

```
make test
```

**Step 3: Implement**

Add to `internal/models/request.go`:

```go
type ListApplicationsRequest struct {
    Limit int    `json:"limit,omitempty"`
    After string `json:"after,omitempty"`
}

func (r *ListApplicationsRequest) Validate() error {
    if r.Limit < 0 {
        return errors.New("limit cannot be negative")
    }
    if r.Limit > MaxPageSize {
        return fmt.Errorf("limit cannot exceed %d", MaxPageSize)
    }
    return nil
}

func (r *ListApplicationsRequest) Normalize() {
    if r.Limit == 0 {
        r.Limit = 50
    }
}
```

**Step 4: Run tests**

```
make test
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/models/request.go internal/models/request_test.go
git commit -m "feat: add ListApplicationsRequest model"
```

---

## Task 3: Cursor types

**Files:**
- Create: `internal/models/cursor.go`
- Create: `internal/models/cursor_test.go`

**Step 1: Write failing tests**

Create `internal/models/cursor_test.go`:

```go
package models

import (
    "testing"
    "time"
)

func TestReleaseCursor_RoundTrip(t *testing.T) {
    original := &ReleaseCursor{
        SortBy:            "release_date",
        SortOrder:         "desc",
        ID:                "test-id",
        ReleaseDate:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
        VersionMajor:      1,
        VersionMinor:      2,
        VersionPatch:      3,
        VersionIsStable:   true,
        VersionPreRelease: "",
        Platform:          "linux",
        Architecture:      "amd64",
        CreatedAt:         time.Date(2024, 1, 10, 9, 0, 0, 0, time.UTC),
    }

    encoded, err := original.Encode()
    if err != nil {
        t.Fatalf("Encode() error: %v", err)
    }
    if encoded == "" {
        t.Fatal("Encode() returned empty string")
    }

    decoded, err := DecodeReleaseCursor(encoded)
    if err != nil {
        t.Fatalf("DecodeReleaseCursor() error: %v", err)
    }
    if decoded.ID != original.ID {
        t.Errorf("ID mismatch: got %q, want %q", decoded.ID, original.ID)
    }
    if decoded.SortBy != original.SortBy {
        t.Errorf("SortBy mismatch: got %q, want %q", decoded.SortBy, original.SortBy)
    }
    if !decoded.ReleaseDate.Equal(original.ReleaseDate) {
        t.Errorf("ReleaseDate mismatch: got %v, want %v", decoded.ReleaseDate, original.ReleaseDate)
    }
}

func TestDecodeReleaseCursor_Invalid(t *testing.T) {
    _, err := DecodeReleaseCursor("not-base64!!!")
    if err == nil {
        t.Error("expected error for invalid base64")
    }

    _, err = DecodeReleaseCursor("aW52YWxpZA==") // base64("invalid")
    if err == nil {
        t.Error("expected error for invalid JSON")
    }
}

func TestApplicationCursor_RoundTrip(t *testing.T) {
    original := &ApplicationCursor{
        CreatedAt: time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC),
        ID:        "my-app",
    }

    encoded, err := original.Encode()
    if err != nil {
        t.Fatalf("Encode() error: %v", err)
    }

    decoded, err := DecodeApplicationCursor(encoded)
    if err != nil {
        t.Fatalf("DecodeApplicationCursor() error: %v", err)
    }
    if decoded.ID != original.ID {
        t.Errorf("ID mismatch: got %q, want %q", decoded.ID, original.ID)
    }
    if !decoded.CreatedAt.Equal(original.CreatedAt) {
        t.Errorf("CreatedAt mismatch: got %v, want %v", decoded.CreatedAt, original.CreatedAt)
    }
}
```

**Step 2: Run to confirm failure**

```
make test
```

**Step 3: Implement**

Create `internal/models/cursor.go`:

```go
// Package models - Keyset pagination cursor types.
// Cursors are opaque base64-encoded JSON tokens produced by the server.
// Clients must treat them as opaque strings and must not construct or modify them.
package models

import (
    "encoding/base64"
    "encoding/json"
    "fmt"
    "time"
)

// ReleaseCursor encodes the position of the last item on a releases page.
// All sort-field values are always present; only the field matching SortBy is used
// for the keyset comparison.
type ReleaseCursor struct {
    SortBy            string    `json:"sort_by"`
    SortOrder         string    `json:"sort_order"`
    ID                string    `json:"id"`
    ReleaseDate       time.Time `json:"release_date"`
    VersionMajor      int64     `json:"version_major"`
    VersionMinor      int64     `json:"version_minor"`
    VersionPatch      int64     `json:"version_patch"`
    VersionIsStable   bool      `json:"version_is_stable"`
    VersionPreRelease string    `json:"version_pre_release"`
    Platform          string    `json:"platform"`
    Architecture      string    `json:"architecture"`
    CreatedAt         time.Time `json:"created_at"`
}

func (c *ReleaseCursor) Encode() (string, error) {
    b, err := json.Marshal(c)
    if err != nil {
        return "", fmt.Errorf("encode cursor: %w", err)
    }
    return base64.StdEncoding.EncodeToString(b), nil
}

func DecodeReleaseCursor(s string) (*ReleaseCursor, error) {
    b, err := base64.StdEncoding.DecodeString(s)
    if err != nil {
        return nil, fmt.Errorf("invalid cursor encoding: %w", err)
    }
    var c ReleaseCursor
    if err := json.Unmarshal(b, &c); err != nil {
        return nil, fmt.Errorf("invalid cursor format: %w", err)
    }
    return &c, nil
}

// ApplicationCursor encodes the position of the last item on an applications page.
// Applications are always sorted by created_at DESC, id DESC.
type ApplicationCursor struct {
    CreatedAt time.Time `json:"created_at"`
    ID        string    `json:"id"`
}

func (c *ApplicationCursor) Encode() (string, error) {
    b, err := json.Marshal(c)
    if err != nil {
        return "", fmt.Errorf("encode cursor: %w", err)
    }
    return base64.StdEncoding.EncodeToString(b), nil
}

func DecodeApplicationCursor(s string) (*ApplicationCursor, error) {
    b, err := base64.StdEncoding.DecodeString(s)
    if err != nil {
        return nil, fmt.Errorf("invalid cursor encoding: %w", err)
    }
    var c ApplicationCursor
    if err := json.Unmarshal(b, &c); err != nil {
        return nil, fmt.Errorf("invalid cursor format: %w", err)
    }
    return &c, nil
}
```

**Step 4: Run tests**

```
make test
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/models/cursor.go internal/models/cursor_test.go
git commit -m "feat: add ReleaseCursor and ApplicationCursor types"
```

---

## Task 4: Update response types

Drop `Page`, `PageSize`, `HasMore` from both list responses; add `NextCursor`.

**Files:**
- Modify: `internal/models/response.go`
- Modify: `internal/models/response_test.go` (if it references the removed fields)
- Modify: `internal/update/service_test.go` (any assertions on Page/PageSize/HasMore)
- Modify: `internal/api/handlers_test.go`, `internal/api/handlers_applications_test.go`
- Modify: `internal/integration/integration_test.go`

**Step 1: Update response structs**

In `internal/models/response.go`, change `ListReleasesResponse`:

```go
type ListReleasesResponse struct {
    Releases   []ReleaseInfo `json:"releases"`
    TotalCount int           `json:"total_count"`
    NextCursor string        `json:"next_cursor"`
}
```

Change `ListApplicationsResponse`:

```go
type ListApplicationsResponse struct {
    Applications []ApplicationSummary `json:"applications"`
    TotalCount   int                  `json:"total_count"`
    NextCursor   string               `json:"next_cursor"`
}
```

**Step 2: Build to find all broken references**

```
make build
```

The build will fail listing every file that references `Page`, `PageSize`, or `HasMore` on these types. Fix each:

- In `internal/update/service.go`: remove `Page`, `PageSize`, `HasMore` from the struct literals in `ListReleases` and `ListApplications`. Add `NextCursor: ""` (the service task will fill this in properly later).
- In `internal/update/service_test.go`: remove assertions on `Page`, `PageSize`, `HasMore`.
- In handler tests: remove assertions on `Page`, `PageSize`, `HasMore` from JSON response comparisons.
- In `internal/integration/integration_test.go`: same.

**Step 3: Build and test**

```
make build
make test
```

Expected: builds cleanly; tests pass (with `NextCursor` always `""` for now).

**Step 4: Commit**

```bash
git add internal/models/response.go internal/update/service.go internal/update/service_test.go \
        internal/api/handlers_test.go internal/api/handlers_applications_test.go \
        internal/integration/integration_test.go
git commit -m "feat: replace Page/PageSize/HasMore with NextCursor in list responses"
```

---

## Task 5: Storage interface + all implementations

**This task must be completed in a single commit** — changing the interface signature breaks compilation until every implementation is updated.

**Files:**
- Modify: `internal/storage/interface.go`
- Modify: `internal/storage/memory.go`
- Modify: `internal/storage/memory_test.go`
- Modify: `internal/storage/postgres.go`
- Modify: `internal/storage/postgres_test.go`
- Modify: `internal/storage/sqlite.go`
- Modify: `internal/storage/sqlite_test.go`
- Modify: `internal/observability/storage.go`
- Modify: `internal/observability/storage_test.go`

### Step 1: Update storage interface

In `internal/storage/interface.go`, change the two paged methods:

```go
// ListApplicationsPaged returns a page of applications sorted by created_at DESC, id DESC,
// and the total count of applications matching the cursor condition.
// after is nil for the first page.
ListApplicationsPaged(ctx context.Context, limit int, after *models.ApplicationCursor) ([]*models.Application, int, error)

// ListReleasesPaged returns a filtered, sorted page of releases for an application,
// and the total count of matching releases satisfying the cursor condition.
// sortBy must be one of: release_date, version, platform, architecture, created_at.
// sortOrder must be "asc" or "desc". after is nil for the first page.
ListReleasesPaged(ctx context.Context, appID string, filters models.ReleaseFilters, sortBy, sortOrder string, limit int, after *models.ReleaseCursor) ([]*models.Release, int, error)
```

### Step 2: Update memory storage

In `internal/storage/memory.go`, replace `ListApplicationsPaged`:

```go
func (m *MemoryStorage) ListApplicationsPaged(ctx context.Context, limit int, after *models.ApplicationCursor) ([]*models.Application, int, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    apps := make([]*models.Application, 0, len(m.applications))
    for _, app := range m.applications {
        copied := *app
        apps = append(apps, &copied)
    }

    // Sort by created_at DESC, id DESC
    sort.Slice(apps, func(i, j int) bool {
        ti, _ := time.Parse(time.RFC3339, apps[i].CreatedAt)
        tj, _ := time.Parse(time.RFC3339, apps[j].CreatedAt)
        if ti.Equal(tj) {
            return apps[i].ID > apps[j].ID
        }
        return ti.After(tj)
    })

    remaining := apps
    if after != nil {
        cursorIdx := -1
        for i, a := range apps {
            if a.ID == after.ID {
                cursorIdx = i
                break
            }
        }
        if cursorIdx >= 0 {
            remaining = apps[cursorIdx+1:]
        }
    }

    total := len(remaining)
    if limit > 0 && len(remaining) > limit {
        remaining = remaining[:limit]
    }
    return remaining, total, nil
}
```

Replace `ListReleasesPaged` (keep the existing filter logic and `memorySortReleases` call; add ID tiebreaker and cursor slicing):

First, update `memorySortReleases` to add an ID tiebreaker so cursor positions are deterministic. For each case, when the primary sort values are equal, sort by `id ASC` (direction does not matter as long as it is consistent — use `id ASC` always):

```go
func memorySortReleases(releases []*models.Release, sortBy, sortOrder string) {
    if len(releases) <= 1 {
        return
    }
    less := func(i, j int) bool {
        ri, rj := releases[i], releases[j]
        switch sortBy {
        case "version":
            vi, ei := semver.NewVersion(ri.Version)
            vj, ej := semver.NewVersion(rj.Version)
            if ei == nil && ej == nil {
                if vi.Equal(vj) {
                    return ri.ID < rj.ID
                }
                return vi.LessThan(vj)
            }
            if ri.Version == rj.Version {
                return ri.ID < rj.ID
            }
            return ri.Version < rj.Version
        case "platform":
            if ri.Platform == rj.Platform {
                return ri.ID < rj.ID
            }
            return ri.Platform < rj.Platform
        case "architecture":
            if ri.Architecture == rj.Architecture {
                return ri.ID < rj.ID
            }
            return ri.Architecture < rj.Architecture
        case "created_at":
            if ri.CreatedAt.Equal(rj.CreatedAt) {
                return ri.ID < rj.ID
            }
            return ri.CreatedAt.Before(rj.CreatedAt)
        default: // release_date
            if ri.ReleaseDate.Equal(rj.ReleaseDate) {
                return ri.ID < rj.ID
            }
            return ri.ReleaseDate.Before(rj.ReleaseDate)
        }
    }
    if sortOrder == "desc" {
        orig := less
        less = func(i, j int) bool { return orig(j, i) }
    }
    sort.Slice(releases, less)
}
```

Then update `ListReleasesPaged` — keep the existing filter block, change only the pagination part:

```go
func (m *MemoryStorage) ListReleasesPaged(ctx context.Context, appID string, filters models.ReleaseFilters, sortBy, sortOrder string, limit int, after *models.ReleaseCursor) ([]*models.Release, int, error) {
    // ... existing filter block unchanged ...

    memorySortReleases(filtered, sortBy, sortOrder)

    remaining := filtered
    if after != nil {
        cursorIdx := -1
        for i, r := range filtered {
            if r.ID == after.ID {
                cursorIdx = i
                break
            }
        }
        if cursorIdx >= 0 {
            remaining = filtered[cursorIdx+1:]
        }
    }

    total := len(remaining)
    if limit > 0 && len(remaining) > limit {
        remaining = remaining[:limit]
    }
    return remaining, total, nil
}
```

### Step 3: Update observability storage

In `internal/observability/storage.go`, update both wrappers to match the new signatures (just pass `after` through):

```go
func (s *InstrumentedStorage) ListApplicationsPaged(ctx context.Context, limit int, after *models.ApplicationCursor) ([]*models.Application, int, error) {
    ctx, span := s.startSpan(ctx, "ListApplicationsPaged")
    start := time.Now()
    apps, total, err := s.inner.ListApplicationsPaged(ctx, limit, after)
    s.record(ctx, span, "ListApplicationsPaged", start, err)
    return apps, total, err
}

func (s *InstrumentedStorage) ListReleasesPaged(ctx context.Context, appID string, filters models.ReleaseFilters, sortBy, sortOrder string, limit int, after *models.ReleaseCursor) ([]*models.Release, int, error) {
    ctx, span := s.startSpan(ctx, "ListReleasesPaged", attribute.String("app_id", appID))
    start := time.Now()
    releases, total, err := s.inner.ListReleasesPaged(ctx, appID, filters, sortBy, sortOrder, limit, after)
    s.record(ctx, span, "ListReleasesPaged", start, err)
    return releases, total, err
}
```

### Step 4: Update postgres storage

In `internal/storage/postgres.go`:

**4a. Add helper function for the ORDER BY clause** (add id tiebreaker, remove OFFSET):

Update the section after `pgReleaseListSortCols` that builds `orderClause`:

```go
// Version sort has direction embedded and always uses id DESC as tiebreaker.
// Other sort columns get an explicit direction suffix with a matching id tiebreaker.
var orderClause string
if sortBy == "version" {
    orderClause = col + ", id DESC"
} else if sortOrder == "asc" {
    orderClause = col + " ASC, id ASC"
} else {
    orderClause = col + " DESC, id DESC"
}
```

**4b. Add the keyset WHERE builder:**

Add this function in `postgres.go` (before `ListReleasesPaged`):

```go
// buildPgReleaseKeyset appends a keyset WHERE fragment and args for the cursor.
// Returns the clause string and the updated args slice.
func buildPgReleaseKeyset(cursor *models.ReleaseCursor, args []interface{}) (string, []interface{}) {
    n := len(args)
    switch cursor.SortBy {
    case "platform":
        op := ltGt(cursor.SortOrder)
        args = append(args, cursor.Platform, cursor.Platform, cursor.ID)
        return fmt.Sprintf("AND ((platform %s $%d) OR (platform = $%d AND id %s $%d))",
            op, n+1, n+2, op, n+3), args
    case "architecture":
        op := ltGt(cursor.SortOrder)
        args = append(args, cursor.Architecture, cursor.Architecture, cursor.ID)
        return fmt.Sprintf("AND ((architecture %s $%d) OR (architecture = $%d AND id %s $%d))",
            op, n+1, n+2, op, n+3), args
    case "created_at":
        op := ltGt(cursor.SortOrder)
        args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
        return fmt.Sprintf("AND ((created_at %s $%d) OR (created_at = $%d AND id %s $%d))",
            op, n+1, n+2, op, n+3), args
    case "release_date":
        op := ltGt(cursor.SortOrder)
        args = append(args, cursor.ReleaseDate, cursor.ReleaseDate, cursor.ID)
        return fmt.Sprintf("AND ((release_date %s $%d) OR (release_date = $%d AND id %s $%d))",
            op, n+1, n+2, op, n+3), args
    case "version": // always DESC
        isStable := 0
        if cursor.VersionIsStable {
            isStable = 1
        }
        pre := cursor.VersionPreRelease
        args = append(args,
            cursor.VersionMajor,
            cursor.VersionMajor, cursor.VersionMinor,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch, isStable,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch, isStable, pre,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch, isStable, pre, cursor.ID,
        )
        return fmt.Sprintf(`AND (
            version_major < $%d
            OR (version_major = $%d AND version_minor < $%d)
            OR (version_major = $%d AND version_minor = $%d AND version_patch < $%d)
            OR (version_major = $%d AND version_minor = $%d AND version_patch = $%d
                AND (CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END) < $%d)
            OR (version_major = $%d AND version_minor = $%d AND version_patch = $%d
                AND (CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END) = $%d
                AND COALESCE(version_pre_release, '') > $%d)
            OR (version_major = $%d AND version_minor = $%d AND version_patch = $%d
                AND (CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END) = $%d
                AND COALESCE(version_pre_release, '') = $%d
                AND id < $%d)
        )`,
            n+1,
            n+2, n+3,
            n+4, n+5, n+6,
            n+7, n+8, n+9, n+10,
            n+11, n+12, n+13, n+14, n+15,
            n+16, n+17, n+18, n+19, n+20, n+21,
        ), args
    default:
        return "", args
    }
}

// ltGt returns "<" for desc and ">" for asc sort order.
func ltGt(sortOrder string) string {
    if sortOrder == "asc" {
        return ">"
    }
    return "<"
}
```

**4c. Update `ListReleasesPaged` signature and query:**

Change signature to `limit int, after *models.ReleaseCursor`. Remove the `offset` lines. After building the existing `where` clause, add:

```go
if after != nil {
    keysetClause, newArgs := buildPgReleaseKeyset(after, args)
    where += " " + keysetClause
    args = newArgs
}

args = append(args, int64(limit))
query := fmt.Sprintf(`
    SELECT id, application_id, version, platform, architecture, download_url,
           checksum, checksum_type, file_size, release_notes, release_date,
           required, minimum_version, metadata, created_at,
           version_major, version_minor, version_patch, version_pre_release,
           COUNT(*) OVER() AS total_count
    FROM releases
    %s
    ORDER BY %s
    LIMIT $%d`,
    where, orderClause, len(args),
)
```

**4d. Replace `ListApplicationsPaged` with a dynamic SQL version** (currently uses sqlc):

```go
func (ps *PostgresStorage) ListApplicationsPaged(ctx context.Context, limit int, after *models.ApplicationCursor) ([]*models.Application, int, error) {
    args := []interface{}{}
    where := "WHERE TRUE"

    if after != nil {
        args = append(args, after.CreatedAt, after.CreatedAt, after.ID)
        where += fmt.Sprintf(
            " AND ((created_at < $1) OR (created_at = $2 AND id < $3))",
        )
    }

    args = append(args, int64(limit))
    query := fmt.Sprintf(`
        SELECT id, name, description, platforms, config, created_at, updated_at,
               COUNT(*) OVER() AS total_count
        FROM applications
        %s
        ORDER BY created_at DESC, id DESC
        LIMIT $%d`,
        where, len(args),
    )

    // NOTE: fix the WHERE parameter indices to be relative to the args slice.
    // When after != nil, args has 3 cursor args + 1 limit = 4 total.
    // The WHERE clause above hard-codes $1/$2/$3 which is correct when cursor is first.
    // Re-derive parameter positions properly:
    // (See implementation note below)
```

> **Implementation note:** The `WHERE` clause above hard-codes `$1`/`$2`/`$3` which works when the cursor args are added first. If `after == nil`, there are no cursor args and `$1` is the limit. Fix by computing positions from `len(args)` before appending:

```go
func (ps *PostgresStorage) ListApplicationsPaged(ctx context.Context, limit int, after *models.ApplicationCursor) ([]*models.Application, int, error) {
    args := []interface{}{}
    where := "WHERE TRUE"

    if after != nil {
        n := len(args) // 0
        args = append(args, after.CreatedAt, after.CreatedAt, after.ID)
        where += fmt.Sprintf(
            " AND ((created_at < $%d) OR (created_at = $%d AND id < $%d))",
            n+1, n+2, n+3,
        )
    }

    args = append(args, int64(limit))
    query := fmt.Sprintf(`
        SELECT id, name, description, platforms, config, created_at, updated_at,
               COUNT(*) OVER() AS total_count
        FROM applications
        %s
        ORDER BY created_at DESC, id DESC
        LIMIT $%d`,
        where, len(args),
    )

    rows, err := ps.pool.Query(ctx, query, args...)
    if err != nil {
        return nil, 0, fmt.Errorf("failed to query applications: %w", err)
    }
    defer rows.Close()

    var apps []*models.Application
    total := 0
    for rows.Next() {
        var (
            id, name, description string
            platforms, config      []byte
            createdAt, updatedAt  pgtype.Timestamptz
            totalCount            int64
        )
        if err := rows.Scan(&id, &name, &description, &platforms, &config, &createdAt, &updatedAt, &totalCount); err != nil {
            return nil, 0, fmt.Errorf("failed to scan application: %w", err)
        }
        if total == 0 {
            total = int(totalCount)
        }
        // convert to models.Application using existing helper pattern
        // ... (model the conversion after the existing GetApplicationByID scan in postgres.go)
    }
    return apps, total, nil
}
```

Study the existing `GetApplicationByID` scan in `postgres.go` to get the exact scan and conversion pattern. Do not duplicate that logic — extract a shared `scanPgApplication` helper if one doesn't exist.

### Step 5: Update sqlite storage

Mirror the postgres changes in `internal/storage/sqlite.go`. Key differences:

- SQLite uses `?` positional params (no `$N`).
- SQLite stores timestamps as TEXT. When building keyset conditions for `release_date` and `created_at`, format the cursor timestamp as a string matching what the SQLite driver stores. Check `sqlite_test.go` for a saved record to confirm the format. Most SQLite Go drivers use RFC3339 or `"2006-01-02 15:04:05"`. Use `cursor.ReleaseDate.UTC().Format(time.RFC3339)` and test.

Add `buildSQLiteReleaseKeyset` mirroring `buildPgReleaseKeyset` but with `?` params:

```go
func buildSQLiteReleaseKeyset(cursor *models.ReleaseCursor, args []interface{}) (string, []interface{}) {
    switch cursor.SortBy {
    case "platform":
        op := ltGt(cursor.SortOrder)
        args = append(args, cursor.Platform, cursor.Platform, cursor.ID)
        return fmt.Sprintf("AND ((platform %s ?) OR (platform = ? AND id %s ?))", op, op), args
    case "architecture":
        op := ltGt(cursor.SortOrder)
        args = append(args, cursor.Architecture, cursor.Architecture, cursor.ID)
        return fmt.Sprintf("AND ((architecture %s ?) OR (architecture = ? AND id %s ?))", op, op), args
    case "release_date":
        op := ltGt(cursor.SortOrder)
        val := cursor.ReleaseDate.UTC().Format(time.RFC3339)
        args = append(args, val, val, cursor.ID)
        return fmt.Sprintf("AND ((release_date %s ?) OR (release_date = ? AND id %s ?))", op, op), args
    case "created_at":
        op := ltGt(cursor.SortOrder)
        val := cursor.CreatedAt.UTC().Format(time.RFC3339)
        args = append(args, val, val, cursor.ID)
        return fmt.Sprintf("AND ((created_at %s ?) OR (created_at = ? AND id %s ?))", op, op), args
    case "version":
        isStable := 0
        if cursor.VersionIsStable {
            isStable = 1
        }
        pre := cursor.VersionPreRelease
        args = append(args,
            cursor.VersionMajor,
            cursor.VersionMajor, cursor.VersionMinor,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch, isStable,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch, isStable, pre,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch, isStable, pre, cursor.ID,
        )
        return `AND (
            version_major < ?
            OR (version_major = ? AND version_minor < ?)
            OR (version_major = ? AND version_minor = ? AND version_patch < ?)
            OR (version_major = ? AND version_minor = ? AND version_patch = ?
                AND (CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END) < ?)
            OR (version_major = ? AND version_minor = ? AND version_patch = ?
                AND (CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END) = ?
                AND COALESCE(version_pre_release, '') > ?)
            OR (version_major = ? AND version_minor = ? AND version_patch = ?
                AND (CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END) = ?
                AND COALESCE(version_pre_release, '') = ?
                AND id < ?)
        )`, args
    default:
        return "", args
    }
}
```

Replace `ListApplicationsPaged` with a dynamic SQL version (no sqlc). SQLite's `created_at` is TEXT so the cursor value must be formatted as a string. Replicate the existing sqlite application scan pattern from `GetApplicationByID`.

Also update the `ListReleasesPaged` ORDER BY to add id tiebreaker and remove OFFSET:

```go
// Replace: args = append(args, int64(limit), int64(offset))
// With:
if after != nil {
    keysetClause, newArgs := buildSQLiteReleaseKeyset(after, args)
    where += " " + keysetClause
    args = newArgs
}
args = append(args, int64(limit))
// Remove OFFSET ? from query
```

### Step 6: Update storage tests

In `memory_test.go`, `postgres_test.go`, `sqlite_test.go`, `storage_test.go`:

- Change all calls to `ListReleasesPaged(..., limit, offset)` to `ListReleasesPaged(..., limit, nil)` for first-page tests.
- Change all calls to `ListApplicationsPaged(ctx, limit, offset)` to `ListApplicationsPaged(ctx, limit, nil)`.
- Add cursor pagination tests for each sort field (see test pattern below).

**Cursor pagination test pattern** (add to `memory_test.go` first, then replicate for postgres/sqlite):

```go
func TestMemoryStorage_ListReleasesPaged_Keyset(t *testing.T) {
    s := newTestMemoryStorage(t)
    // Insert 5 releases with distinct release_dates for app "app1"
    // ... insert releases r1..r5 with different dates ...

    // Page 1
    page1, total1, err := s.ListReleasesPaged(ctx, "app1", models.ReleaseFilters{},
        "release_date", "desc", 2, nil)
    if err != nil || len(page1) != 2 || total1 != 5 {
        t.Fatalf("page1: got %d items, total %d, err %v", len(page1), total1, err)
    }

    // Build cursor from last item on page 1
    cursor := &models.ReleaseCursor{
        SortBy:      "release_date",
        SortOrder:   "desc",
        ID:          page1[1].ID,
        ReleaseDate: page1[1].ReleaseDate,
    }

    // Page 2
    page2, total2, err := s.ListReleasesPaged(ctx, "app1", models.ReleaseFilters{},
        "release_date", "desc", 2, cursor)
    if err != nil || len(page2) != 2 || total2 != 3 {
        t.Fatalf("page2: got %d items, total %d, err %v", len(page2), total2, err)
    }
    // page2 must not overlap page1
    for _, r := range page2 {
        for _, r1 := range page1 {
            if r.ID == r1.ID {
                t.Errorf("duplicate item %s across pages", r.ID)
            }
        }
    }

    // Page 3 (last)
    cursor3 := &models.ReleaseCursor{
        SortBy:      "release_date",
        SortOrder:   "desc",
        ID:          page2[1].ID,
        ReleaseDate: page2[1].ReleaseDate,
    }
    page3, total3, err := s.ListReleasesPaged(ctx, "app1", models.ReleaseFilters{},
        "release_date", "desc", 2, cursor3)
    if err != nil || len(page3) != 1 || total3 != 1 {
        t.Fatalf("page3: got %d items, total %d, err %v", len(page3), total3, err)
    }
}
```

Add similar tests for `version`, `platform`, `architecture`, `created_at` sort fields in memory tests. The postgres and sqlite tests only need one representative sort field (e.g., `release_date`) since the SQL builder is the same for all scalar fields.

### Step 7: Build and run all tests

```
make build
make test
```

Expected: all tests pass.

### Step 8: Commit

```bash
git add internal/storage/interface.go internal/storage/memory.go internal/storage/memory_test.go \
        internal/storage/postgres.go internal/storage/postgres_test.go \
        internal/storage/sqlite.go internal/storage/sqlite_test.go \
        internal/observability/storage.go internal/observability/storage_test.go
git commit -m "feat: replace offset with keyset cursor in storage paged list methods"
```

---

## Task 6: Service layer — cursor encode/decode

**Files:**
- Modify: `internal/update/service.go`
- Modify: `internal/update/interface.go`
- Modify: `internal/update/service_test.go`

### Step 1: Add cursor builder helper

Add this private function to `internal/update/service.go` (import `semver` — it is already imported):

```go
// buildReleaseCursor constructs a ReleaseCursor from the last release on a page.
func buildReleaseCursor(r *models.Release, sortBy, sortOrder string) (*models.ReleaseCursor, error) {
    c := &models.ReleaseCursor{
        SortBy:    sortBy,
        SortOrder: sortOrder,
        ID:        r.ID,
    }
    switch sortBy {
    case "release_date":
        c.ReleaseDate = r.ReleaseDate
    case "version":
        v, err := semver.NewVersion(r.Version)
        if err != nil {
            return nil, fmt.Errorf("build cursor: parse version %q: %w", r.Version, err)
        }
        c.VersionMajor = int64(v.Major())
        c.VersionMinor = int64(v.Minor())
        c.VersionPatch = int64(v.Patch())
        c.VersionIsStable = v.Prerelease() == ""
        c.VersionPreRelease = v.Prerelease()
    case "platform":
        c.Platform = r.Platform
    case "architecture":
        c.Architecture = r.Architecture
    case "created_at":
        c.CreatedAt = r.CreatedAt
    }
    return c, nil
}
```

### Step 2: Update ListReleases

Replace the current `ListReleases` in `service.go`:

```go
func (s *Service) ListReleases(ctx context.Context, req *models.ListReleasesRequest) (*models.ListReleasesResponse, error) {
    if err := req.Validate(); err != nil {
        return nil, NewValidationError("invalid request", err)
    }
    req.Normalize()

    var after *models.ReleaseCursor
    if req.After != "" {
        c, err := models.DecodeReleaseCursor(req.After)
        if err != nil {
            return nil, NewValidationError("invalid cursor", err)
        }
        if c.SortBy != req.SortBy || c.SortOrder != req.SortOrder {
            return nil, NewValidationError("cursor sort_by/sort_order does not match request", nil)
        }
        after = c
    }

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

    releases, totalCount, err := s.storage.ListReleasesPaged(ctx, req.ApplicationID, filters, req.SortBy, req.SortOrder, req.Limit, after)
    if err != nil {
        return nil, NewInternalError("failed to get releases", err)
    }

    releaseInfos := make([]models.ReleaseInfo, len(releases))
    for i, r := range releases {
        releaseInfos[i].FromRelease(r)
    }

    var nextCursor string
    if len(releases) < totalCount {
        last := releases[len(releases)-1]
        c, err := buildReleaseCursor(last, req.SortBy, req.SortOrder)
        if err != nil {
            return nil, NewInternalError("failed to build next cursor", err)
        }
        encoded, err := c.Encode()
        if err != nil {
            return nil, NewInternalError("failed to encode next cursor", err)
        }
        nextCursor = encoded
    }

    return &models.ListReleasesResponse{
        Releases:   releaseInfos,
        TotalCount: totalCount,
        NextCursor: nextCursor,
    }, nil
}
```

### Step 3: Update ListApplications

Change `ServiceInterface` in `interface.go`:

```go
// ListApplications returns a paginated list of applications
ListApplications(ctx context.Context, req *models.ListApplicationsRequest) (*models.ListApplicationsResponse, error)
```

Replace `ListApplications` in `service.go`:

```go
func (s *Service) ListApplications(ctx context.Context, req *models.ListApplicationsRequest) (*models.ListApplicationsResponse, error) {
    if err := req.Validate(); err != nil {
        return nil, NewValidationError("invalid request", err)
    }
    req.Normalize()

    var after *models.ApplicationCursor
    if req.After != "" {
        c, err := models.DecodeApplicationCursor(req.After)
        if err != nil {
            return nil, NewValidationError("invalid cursor", err)
        }
        after = c
    }

    apps, totalCount, err := s.storage.ListApplicationsPaged(ctx, req.Limit, after)
    if err != nil {
        return nil, NewInternalError("failed to list applications", err)
    }

    summaries := make([]models.ApplicationSummary, len(apps))
    for i, app := range apps {
        summaries[i].FromApplication(app)
        summaries[i].CreatedAt, _ = time.Parse(time.RFC3339, app.CreatedAt)
        summaries[i].UpdatedAt, _ = time.Parse(time.RFC3339, app.UpdatedAt)
    }

    var nextCursor string
    if len(apps) < totalCount {
        last := apps[len(apps)-1]
        createdAt, _ := time.Parse(time.RFC3339, last.CreatedAt)
        c := &models.ApplicationCursor{
            CreatedAt: createdAt,
            ID:        last.ID,
        }
        encoded, err := c.Encode()
        if err != nil {
            return nil, NewInternalError("failed to encode next cursor", err)
        }
        nextCursor = encoded
    }

    return &models.ListApplicationsResponse{
        Applications: summaries,
        TotalCount:   totalCount,
        NextCursor:   nextCursor,
    }, nil
}
```

### Step 4: Update service tests

In `service_test.go`:
- Update all `ListApplications(ctx, limit, offset)` calls to `ListApplications(ctx, &models.ListApplicationsRequest{Limit: limit})`.
- Add tests for cursor round-trip: call `ListReleases` with a limit of 1, take `NextCursor` from the response, call again with `After: nextCursor`, assert second page items differ from first.
- Add test: cursor `sort_by` mismatch returns validation error.
- Add test: malformed `After` string returns validation error.

### Step 5: Build and run tests

```
make build
make test
```

Expected: all tests pass.

### Step 6: Commit

```bash
git add internal/update/service.go internal/update/interface.go internal/update/service_test.go
git commit -m "feat: implement cursor encode/decode and next_cursor in service layer"
```

---

## Task 7: Update HTTP handlers

**Files:**
- Modify: `internal/api/handlers.go` (`ListReleases`)
- Modify: `internal/api/handlers_applications.go` (`ListApplications`)
- Modify: `internal/api/handlers_test.go`
- Modify: `internal/api/handlers_applications_test.go`

### Step 1: Update ListReleases handler

In `handlers.go`, `ListReleases` method: remove `offset` parsing, add `after`:

```go
func (h *Handlers) ListReleases(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    appID := vars["app_id"]

    req := &models.ListReleasesRequest{
        ApplicationID: appID,
        Platform:      r.URL.Query().Get("platform"),
        Architecture:  r.URL.Query().Get("architecture"),
        Version:       r.URL.Query().Get("version"),
        SortBy:        r.URL.Query().Get("sort_by"),
        SortOrder:     r.URL.Query().Get("sort_order"),
        After:         r.URL.Query().Get("after"),
    }

    if requiredParam := r.URL.Query().Get("required"); requiredParam != "" {
        if required, err := strconv.ParseBool(requiredParam); err == nil {
            req.Required = &required
        }
    }

    if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
        if limit, err := strconv.Atoi(limitParam); err == nil {
            req.Limit = limit
        }
    }

    if platforms := r.URL.Query().Get("platforms"); platforms != "" {
        req.Platforms = splitAndTrim(platforms, ",")
    }

    response, err := h.updateService.ListReleases(r.Context(), req)
    if err != nil {
        h.writeServiceErrorResponse(w, err)
        return
    }

    h.writeJSONResponse(w, http.StatusOK, response)
}
```

### Step 2: Update ListApplications handler

In `handlers_applications.go`, replace the inline limit/offset parsing with `ListApplicationsRequest`:

```go
func (h *Handlers) ListApplications(w http.ResponseWriter, r *http.Request) {
    req := &models.ListApplicationsRequest{
        After: r.URL.Query().Get("after"),
    }

    if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
        if parsed, err := strconv.Atoi(limitParam); err == nil {
            req.Limit = parsed
        }
    }

    response, err := h.updateService.ListApplications(r.Context(), req)
    if err != nil {
        h.writeServiceErrorResponse(w, err)
        return
    }

    h.writeJSONResponse(w, http.StatusOK, response)
}
```

Remove the `strconv` import if `strconv.Atoi` was the only use (it still is, so keep it).

### Step 3: Update handler tests

In `handlers_test.go` and `handlers_applications_test.go`:
- Remove any assertions on `offset` query parameter behaviour.
- Add test: `GET /updates/{app_id}/releases?limit=501` returns 400.
- Add test: `GET /applications?limit=501` returns 400.
- Add test: `GET /updates/{app_id}/releases?after=invalid!!!` returns 400.
- Verify that response JSON contains `next_cursor` (not `page`/`page_size`/`has_more`).

### Step 4: Build and run tests

```
make build
make test
```

Expected: all tests pass.

### Step 5: Commit

```bash
git add internal/api/handlers.go internal/api/handlers_applications.go \
        internal/api/handlers_test.go internal/api/handlers_applications_test.go
git commit -m "feat: update list handlers to use cursor pagination and ListApplicationsRequest"
```

---

## Task 8: Update OpenAPI spec

**Files:**
- Modify: `internal/api/openapi/openapi.yaml`

### Step 1: Update releases list endpoint

For `GET /updates/{app_id}/releases`:
- Remove `offset` parameter.
- Add `after` query parameter (type: string, description: "Opaque pagination cursor from `next_cursor`").
- Add `limit` maximum of 500.
- In the response schema for `ListReleasesResponse`: remove `page`, `page_size`, `has_more`; add `next_cursor` (type: string).

### Step 2: Update applications list endpoint

For `GET /applications`:
- Remove `offset` parameter.
- Add `after` query parameter.
- Add `limit` maximum of 500.
- In the response schema for `ListApplicationsResponse`: same changes.

### Step 3: Validate

```
make openapi-validate
```

Expected: no errors.

### Step 4: Commit

```bash
git add internal/api/openapi/openapi.yaml
git commit -m "docs: update OpenAPI spec for keyset pagination"
```

---

## Task 9: Integration tests

**Files:**
- Modify: `internal/integration/integration_test.go`

### Step 1: Update existing list tests

- Remove any use of `offset` query parameter.
- Update JSON assertions to check for `next_cursor` instead of `page`/`page_size`/`has_more`.

### Step 2: Add cursor pagination integration test

Add a test that:
1. Creates an app with 5+ releases.
2. Lists with `limit=2`, asserts `next_cursor` is non-empty.
3. Lists again with `after=<next_cursor>`, asserts different items, no duplicates.
4. Continues until `next_cursor` is empty.
5. Asserts all original releases are covered.

### Step 3: Run integration tests

```
make integration-test
```

Expected: all pass.

### Step 4: Commit

```bash
git add internal/integration/integration_test.go
git commit -m "test: update integration tests for keyset pagination"
```

---

## Task 10: Update documentation

**Files:**
- Modify: `docs/storage.md` (if it documents `ListReleasesPaged`/`ListApplicationsPaged` signatures)
- Modify: `docs/ARCHITECTURE.md` (if it references offset pagination)
- Modify: `mkdocs.yml` (add design doc to nav if not present)
- Run `make docs-build` to verify docs build cleanly.

### Step 1: Update storage.md

Update any mention of `offset` parameter in `ListReleasesPaged`/`ListApplicationsPaged` to describe the cursor parameter instead.

### Step 2: Update ARCHITECTURE.md

Update the pagination section (if any) to describe keyset pagination.

### Step 3: Verify docs build

```
make docs-build
```

Expected: no errors.

### Step 4: Commit

```bash
git add docs/
git commit -m "docs: update storage and architecture docs for keyset pagination"
```

---

## Final verification

```
make check
make integration-test
```

Both must pass before creating a pull request.