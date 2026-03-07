# Keyset Pagination Bugfixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix eight correctness bugs identified in code review of the keyset cursor pagination feature (PR #42), and restore the CI coverage threshold.

**Architecture:** Fixes span three layers: SQL query restructuring (postgres.go, sqlite.go) to give stable `total_count` across pages; service layer (service.go) to fix cursor emission logic and error handling; model layer (response.go) to honour the OpenAPI contract; and storage layer (memory.go) to fix cursor-not-found behaviour. Each task is independently testable and committable.

**Tech Stack:** Go 1.25, SQLite (mattn/go-sqlite3 via database/sql), pgx v5, testify, semver v3.

---

## Background

Issues raised in PR #42 code review. GitHub issues created for deferred items: #43 (memory keyset purity) and #44 (DecodeReleaseCursor validation).

Fixes in this plan (in execution order):

| # | Review item | File(s) |
|---|-------------|---------|
| 1 | Integration test hits releases endpoint twice | `internal/integration/integration_test.go` |
| 2 | `omitempty` drops `next_cursor` from JSON | `internal/models/response.go` |
| 3 | Cursor emission condition + encode error handling + non-semver | `internal/update/service.go` |
| 4 | `COUNT(*) OVER()` counts post-cursor rows (postgres) | `internal/storage/postgres.go` |
| 5 | `COUNT(*) OVER()` counts post-cursor rows (sqlite) | `internal/storage/sqlite.go` |
| 6 | Pre-release sort direction mismatch (postgres) | `internal/storage/postgres.go` |
| 7 | Pre-release sort direction mismatch (sqlite) | `internal/storage/sqlite.go` |
| 8 | Memory store cursor-not-found returns page 1 | `internal/storage/memory.go` |

---

## Task 1: Fix integration test copy-paste bug (review item #3)

**Files:**
- Modify: `internal/integration/integration_test.go:1221`

### Step 1: Verify the bug

Read `internal/integration/integration_test.go` lines 1213–1224. Confirm that line 1221 uses
`/api/v1/updates/max-limit-test-app/releases?limit=501` — the same URL as line 1214. The comment
says "applications endpoint" but the URL is wrong.

### Step 2: Run the test to confirm it currently passes for the wrong reason

```bash
make test
```

The test passes because both requests hit the same (releases) endpoint. No failure is detected.

### Step 3: Fix the URL

In `internal/integration/integration_test.go`, change line 1221 from:

```go
resp2, err := http.Get(server.URL + "/api/v1/updates/max-limit-test-app/releases?limit=501")
```

To:

```go
resp2, err := http.Get(server.URL + "/api/v1/applications?limit=501")
```

The test server is created without auth (`Security.EnableAuth` is not set, defaults to false), so
the applications endpoint is reachable without a key.

### Step 4: Run the tests and verify they still pass

```bash
make test
```

Expected: PASS. The applications endpoint also validates `limit` before auth, so 422 is returned.

### Step 5: Commit

```bash
git add internal/integration/integration_test.go
git commit -m "test: fix TestIntegration_MaxLimitValidation to test applications endpoint"
```

---

## Task 2: Fix `next_cursor` omitempty (review item #2)

The OpenAPI spec declares `next_cursor` as `required` in both list response schemas and the examples
show `"next_cursor": ""`. The Go structs use `omitempty`, which drops the field when empty,
violating the contract.

**Files:**
- Modify: `internal/models/response.go:63,167`

### Step 1: Write a failing test

In `internal/models/response_test.go` (create if it does not exist), add:

```go
package models

import (
    "encoding/json"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestListReleasesResponse_NextCursorAlwaysPresent(t *testing.T) {
    resp := ListReleasesResponse{
        Releases:   []ReleaseInfo{},
        TotalCount: 0,
        NextCursor: "",
    }
    b, err := json.Marshal(resp)
    require.NoError(t, err)

    var m map[string]interface{}
    require.NoError(t, json.Unmarshal(b, &m))
    _, ok := m["next_cursor"]
    assert.True(t, ok, "next_cursor must be present in JSON even when empty")
}

func TestListApplicationsResponse_NextCursorAlwaysPresent(t *testing.T) {
    resp := ListApplicationsResponse{
        Applications: []ApplicationSummary{},
        TotalCount:   0,
        NextCursor:   "",
    }
    b, err := json.Marshal(resp)
    require.NoError(t, err)

    var m map[string]interface{}
    require.NoError(t, json.Unmarshal(b, &m))
    _, ok := m["next_cursor"]
    assert.True(t, ok, "next_cursor must be present in JSON even when empty")
}
```

### Step 2: Run to confirm failure

```bash
make test
```

Expected: FAIL — `next_cursor` key is absent from the marshalled JSON.

### Step 3: Remove `omitempty` from both structs

In `internal/models/response.go`:

Change line 63:
```go
NextCursor string `json:"next_cursor,omitempty"`
```
To:
```go
NextCursor string `json:"next_cursor"`
```

Change line 167:
```go
NextCursor string `json:"next_cursor,omitempty"`
```
To:
```go
NextCursor string `json:"next_cursor"`
```

### Step 4: Run tests and verify they pass

```bash
make test
```

Expected: PASS.

### Step 5: Commit

```bash
git add internal/models/response.go internal/models/response_test.go
git commit -m "fix: always include next_cursor in list responses to match OpenAPI contract"
```

---

## Task 3: Fix service.go cursor logic (review items #5, #6, #10)

Three problems in `internal/update/service.go`, all in the cursor-generation blocks of
`ListReleases` (lines ~218–245) and `ListApplications` (lines ~390–405):

- **#5** — Cursor emission uses `len < totalCount` (coupled to broken COUNT). Change to `len == req.Limit`.
- **#6** — `c.Encode()` errors are silently discarded (`nextCursor, _ = c.Encode()`). Return an internal error instead.
- **#10** — Cursor is skipped entirely when `semver.NewVersion(last.Version)` fails. For non-version
  sorts the semver fields are unused; emit the cursor anyway. For version sort, only skip if semver
  parse fails (and log it).

**Files:**
- Modify: `internal/update/service.go:218–245` (ListReleases block)
- Modify: `internal/update/service.go:390–405` (ListApplications block)

### Step 1: Write failing tests

In `internal/update/service_test.go`, add the following tests. Use `NewMemoryStorage` as the
backing store (already imported).

```go
func TestListReleases_CursorEmittedWhenPageFull(t *testing.T) {
    // Exactly limit items exist; cursor must be emitted.
    store, err := storage.NewMemoryStorage()
    require.NoError(t, err)
    defer store.Close()
    ctx := context.Background()

    app := &models.Application{ID: "app1", Name: "App1", Platforms: []string{"windows"}, Config: models.ApplicationConfig{}}
    require.NoError(t, store.SaveApplication(ctx, app))

    now := time.Now()
    for i := range 3 {
        r := &models.Release{
            ID: fmt.Sprintf("r%d", i), ApplicationID: "app1",
            Version: fmt.Sprintf("1.0.%d", i), Platform: "windows", Architecture: "amd64",
            DownloadURL: "http://example.com", Checksum: "abc", ChecksumType: "sha256",
            ReleaseDate: now.Add(time.Duration(i) * time.Second), CreatedAt: now.Add(time.Duration(i) * time.Second),
        }
        require.NoError(t, store.SaveRelease(ctx, r))
    }

    svc := NewService(store)
    req := &models.ListReleasesRequest{
        ApplicationID: "app1",
        Limit:         3, // exactly 3 items exist
        SortBy:        "release_date",
        SortOrder:     "desc",
    }
    resp, err := svc.ListReleases(ctx, req)
    require.NoError(t, err)
    assert.Len(t, resp.Releases, 3)
    // With len == limit, cursor should be emitted (we can't know there's no next page without fetching it).
    assert.NotEmpty(t, resp.NextCursor, "cursor must be emitted when page is full")
}

func TestListReleases_NoCursorWhenPagePartial(t *testing.T) {
    store, err := storage.NewMemoryStorage()
    require.NoError(t, err)
    defer store.Close()
    ctx := context.Background()

    app := &models.Application{ID: "app1", Name: "App1", Platforms: []string{"windows"}, Config: models.ApplicationConfig{}}
    require.NoError(t, store.SaveApplication(ctx, app))

    now := time.Now()
    for i := range 2 {
        r := &models.Release{
            ID: fmt.Sprintf("r%d", i), ApplicationID: "app1",
            Version: fmt.Sprintf("1.0.%d", i), Platform: "windows", Architecture: "amd64",
            DownloadURL: "http://example.com", Checksum: "abc", ChecksumType: "sha256",
            ReleaseDate: now.Add(time.Duration(i) * time.Second), CreatedAt: now.Add(time.Duration(i) * time.Second),
        }
        require.NoError(t, store.SaveRelease(ctx, r))
    }

    svc := NewService(store)
    req := &models.ListReleasesRequest{
        ApplicationID: "app1",
        Limit:         10, // only 2 items exist, page is partial
        SortBy:        "release_date",
        SortOrder:     "desc",
    }
    resp, err := svc.ListReleases(ctx, req)
    require.NoError(t, err)
    assert.Len(t, resp.Releases, 2)
    assert.Empty(t, resp.NextCursor, "cursor must not be emitted when page is partial")
}

func TestListReleases_NonSemverVersionNonVersionSort(t *testing.T) {
    // Non-semver version string must not prevent cursor emission when sorting by a non-version field.
    store, err := storage.NewMemoryStorage()
    require.NoError(t, err)
    defer store.Close()
    ctx := context.Background()

    app := &models.Application{ID: "app1", Name: "App1", Platforms: []string{"windows"}, Config: models.ApplicationConfig{}}
    require.NoError(t, store.SaveApplication(ctx, app))

    now := time.Now()
    for i := range 3 {
        r := &models.Release{
            ID: fmt.Sprintf("r%d", i), ApplicationID: "app1",
            Version: fmt.Sprintf("not-semver-%d", i), Platform: "windows", Architecture: "amd64",
            DownloadURL: "http://example.com", Checksum: "abc", ChecksumType: "sha256",
            ReleaseDate: now.Add(time.Duration(i) * time.Second), CreatedAt: now.Add(time.Duration(i) * time.Second),
        }
        require.NoError(t, store.SaveRelease(ctx, r))
    }

    svc := NewService(store)
    req := &models.ListReleasesRequest{
        ApplicationID: "app1",
        Limit:         3,
        SortBy:        "release_date",
        SortOrder:     "desc",
    }
    resp, err := svc.ListReleases(ctx, req)
    require.NoError(t, err)
    assert.NotEmpty(t, resp.NextCursor, "cursor must be emitted for non-semver versions when not sorting by version")
}

func TestListApplications_CursorEmittedWhenPageFull(t *testing.T) {
    store, err := storage.NewMemoryStorage()
    require.NoError(t, err)
    defer store.Close()
    ctx := context.Background()

    now := time.Now()
    for i := range 3 {
        app := &models.Application{
            ID: fmt.Sprintf("app%d", i), Name: fmt.Sprintf("App%d", i),
            Platforms: []string{"windows"}, Config: models.ApplicationConfig{},
            CreatedAt: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
        }
        require.NoError(t, store.SaveApplication(ctx, app))
    }

    svc := NewService(store)
    req := &models.ListApplicationsRequest{Limit: 3}
    resp, err := svc.ListApplications(ctx, req)
    require.NoError(t, err)
    assert.Len(t, resp.Applications, 3)
    assert.NotEmpty(t, resp.NextCursor, "cursor must be emitted when page is full")
}
```

### Step 2: Run to confirm failures

```bash
make test
```

Expected: `TestListReleases_CursorEmittedWhenPageFull` and `TestListApplications_CursorEmittedWhenPageFull` FAIL (no cursor emitted when len == limit), `TestListReleases_NonSemverVersionNonVersionSort` FAIL (cursor missing).

### Step 3: Rewrite the cursor block in ListReleases

Replace the cursor-generation block in `service.go` (currently lines ~218–244):

```go
var nextCursor string
if len(releases) > 0 && len(releases) == req.Limit {
    last := releases[len(releases)-1]
    c := &models.ReleaseCursor{
        SortBy:       req.SortBy,
        SortOrder:    req.SortOrder,
        ID:           last.ID,
        ReleaseDate:  last.ReleaseDate,
        Platform:     last.Platform,
        Architecture: last.Architecture,
        CreatedAt:    last.CreatedAt,
    }
    generateCursor := true
    if req.SortBy == "version" {
        sv, err := semver.NewVersion(last.Version)
        if err != nil {
            // Cannot generate a keyset cursor for version sort when the last item has a
            // non-semver version. Pagination appears complete. This should not occur in
            // normal operation since the API validates versions on write.
            generateCursor = false
        } else {
            c.VersionMajor = int64(sv.Major())      //#nosec G115 -- validated at API layer
            c.VersionMinor = int64(sv.Minor())      //#nosec G115 -- validated at API layer
            c.VersionPatch = int64(sv.Patch())      //#nosec G115 -- validated at API layer
            c.VersionIsStable = sv.Prerelease() == ""
            c.VersionPreRelease = sv.Prerelease()
        }
    }
    if generateCursor {
        encoded, err := c.Encode()
        if err != nil {
            return nil, NewInternalError("failed to encode pagination cursor", err)
        }
        nextCursor = encoded
    }
}
```

### Step 4: Rewrite the cursor block in ListApplications

Replace the cursor-generation block in `service.go` (currently lines ~390–399):

```go
var nextCursor string
if len(apps) > 0 && len(apps) == req.Limit {
    last := apps[len(apps)-1]
    createdAt, _ := time.Parse(time.RFC3339, last.CreatedAt)
    c := &models.ApplicationCursor{
        CreatedAt: createdAt,
        ID:        last.ID,
    }
    encoded, err := c.Encode()
    if err != nil {
        return nil, NewInternalError("failed to encode pagination cursor", err)
    }
    nextCursor = encoded
}
```

### Step 5: Run tests and verify they pass

```bash
make test
```

Expected: PASS.

### Step 6: Commit

```bash
git add internal/update/service.go internal/update/service_test.go
git commit -m "fix: cursor emission uses len==limit, handle encode errors, support non-semver versions"
```

---

## Task 4: Fix COUNT(*) OVER() in postgres.go (review item #1)

`COUNT(*) OVER()` is a window function evaluated after the WHERE clause. When a keyset cursor
`WHERE` condition is present, it counts only the rows remaining after the cursor — not the stable
total across all pages.

Fix: restructure both `ListApplicationsPaged` and `ListReleasesPaged` to use a subquery that
applies business filters and the window function, with the keyset condition in the outer WHERE.

```
Inner query:  WHERE [business filters]  →  COUNT(*) OVER() = full business-filtered total
Outer query:  WHERE [keyset condition]  →  pagination applied AFTER count is stable
```

**Files:**
- Modify: `internal/storage/postgres.go` — `ListApplicationsPaged` and `ListReleasesPaged`

No unit tests here (requires a running PostgreSQL). The integration tests in `internal/integration/`
cover this path. Read the relevant function bodies before editing.

### Step 1: Refactor ListApplicationsPaged

The WHERE clause in this function contains only the keyset cursor condition (no business filters
for applications). Move it to the outer query:

Replace the query string in `ListApplicationsPaged`:

```go
query := fmt.Sprintf(`
    SELECT id, name, description, platforms, config, created_at, updated_at, total_count
    FROM (
        SELECT id, name, description, platforms, config, created_at, updated_at,
               COUNT(*) OVER() AS total_count
        FROM applications
    ) AS counted
    %s
    ORDER BY created_at DESC, id DESC
    LIMIT $1`,
    where)
```

The `where` variable (keyset condition) now applies to the outer query instead of the inner. The
inner query counts ALL applications; the outer query restricts to the current page.

### Step 2: Refactor ListReleasesPaged

This function mixes business filter args and keyset args in one `where` variable. Split them:

Rename the current `where` variable to `businessWhere`. After all business filter conditions are
appended, build a separate `keysetWhere` string for the cursor condition. Use the same `args` slice
— business filter args come first, then keyset args, then limit.

The query becomes:

```go
query := fmt.Sprintf(`
    SELECT id, application_id, version, platform, architecture, download_url,
           checksum, checksum_type, file_size, release_notes, release_date,
           required, minimum_version, metadata, created_at,
           version_major, version_minor, version_patch, version_pre_release,
           total_count
    FROM (
        SELECT id, application_id, version, platform, architecture, download_url,
               checksum, checksum_type, file_size, release_notes, release_date,
               required, minimum_version, metadata, created_at,
               version_major, version_minor, version_patch, version_pre_release,
               COUNT(*) OVER() AS total_count
        FROM releases
        %s
    ) AS counted
    %s
    ORDER BY %s
    LIMIT $%d`,
    businessWhere, keysetWhere, orderClause, len(args),
)
```

Where `keysetWhere` is either `""` (no cursor) or `"WHERE (...)"` containing only the cursor
conditions.

Full refactor of the keyset block — before the limit is appended:

```go
// keysetWhere is empty unless a cursor is provided.
keysetWhere := ""
if cursor != nil {
    n := len(args)
    switch sortBy {
    case "version":
        isStable := 0
        if cursor.VersionIsStable {
            isStable = 1
        }
        args = append(args,
            cursor.VersionMajor,
            cursor.VersionMinor,
            cursor.VersionPatch,
            isStable,
            cursor.VersionPreRelease,
            cursor.ID,
        )
        keysetWhere = fmt.Sprintf(`WHERE (
  version_major < $%d
  OR (version_major = $%d AND version_minor < $%d)
  OR (version_major = $%d AND version_minor = $%d AND version_patch < $%d)
  OR (version_major = $%d AND version_minor = $%d AND version_patch = $%d AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END < $%d)
  OR (version_major = $%d AND version_minor = $%d AND version_patch = $%d AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END = $%d AND COALESCE(version_pre_release, '') > $%d)
  OR (version_major = $%d AND version_minor = $%d AND version_patch = $%d AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END = $%d AND COALESCE(version_pre_release, '') = $%d AND id < $%d)
)`,
            n+1,
            n+1, n+2,
            n+1, n+2, n+3,
            n+1, n+2, n+3, n+4,
            n+1, n+2, n+3, n+4, n+5,
            n+1, n+2, n+3, n+4, n+5, n+6,
        )
    case "platform":
        args = append(args, cursor.Platform, cursor.ID)
        if sortOrder == "desc" {
            keysetWhere = fmt.Sprintf("WHERE ((platform < $%d) OR (platform = $%d AND id < $%d))", n+1, n+1, n+2)
        } else {
            keysetWhere = fmt.Sprintf("WHERE ((platform > $%d) OR (platform = $%d AND id > $%d))", n+1, n+1, n+2)
        }
    case "architecture":
        args = append(args, cursor.Architecture, cursor.ID)
        if sortOrder == "desc" {
            keysetWhere = fmt.Sprintf("WHERE ((architecture < $%d) OR (architecture = $%d AND id < $%d))", n+1, n+1, n+2)
        } else {
            keysetWhere = fmt.Sprintf("WHERE ((architecture > $%d) OR (architecture = $%d AND id > $%d))", n+1, n+1, n+2)
        }
    case "created_at":
        args = append(args, cursor.CreatedAt.UTC().Format(time.RFC3339), cursor.ID)
        if sortOrder == "desc" {
            keysetWhere = fmt.Sprintf("WHERE ((created_at < $%d::timestamptz) OR (created_at = $%d::timestamptz AND id < $%d))", n+1, n+1, n+2)
        } else {
            keysetWhere = fmt.Sprintf("WHERE ((created_at > $%d::timestamptz) OR (created_at = $%d::timestamptz AND id > $%d))", n+1, n+1, n+2)
        }
    default: // release_date
        args = append(args, cursor.ReleaseDate.UTC().Format(time.RFC3339), cursor.ID)
        if sortOrder == "desc" {
            keysetWhere = fmt.Sprintf("WHERE ((release_date < $%d::timestamptz) OR (release_date = $%d::timestamptz AND id < $%d))", n+1, n+1, n+2)
        } else {
            keysetWhere = fmt.Sprintf("WHERE ((release_date > $%d::timestamptz) OR (release_date = $%d::timestamptz AND id > $%d))", n+1, n+1, n+2)
        }
    }
}

args = append(args, int64(limit))
```

### Step 3: Verify the code compiles

```bash
make vet
```

Expected: no errors.

### Step 4: Commit

```bash
git add internal/storage/postgres.go
git commit -m "fix: use subquery so COUNT(*) OVER() counts all business-filtered rows in postgres"
```

---

## Task 5: Fix COUNT(*) OVER() in sqlite.go (review item #1)

Same structural fix as Task 4 but for `SQLiteStorage`. SQLite uses `?` positional placeholders
instead of `$N`.

**Files:**
- Modify: `internal/storage/sqlite.go` — `ListApplicationsPaged` and `ListReleasesPaged`
- Modify: `internal/storage/sqlite_test.go` — add pagination total_count stability test

### Step 1: Write a failing test

In `internal/storage/sqlite_test.go`, add:

```go
func TestSQLiteStorage_ListApplicationsPaged_TotalCountStable(t *testing.T) {
    store, err := NewSQLiteStorage(":memory:")
    require.NoError(t, err)
    defer store.Close()
    ctx := context.Background()

    now := time.Now().UTC()
    for i := range 5 {
        app := &models.Application{
            ID: fmt.Sprintf("app-%d", i), Name: fmt.Sprintf("App %d", i),
            Platforms: []string{"windows"}, Config: models.ApplicationConfig{},
            CreatedAt: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
            UpdatedAt: now.Format(time.RFC3339),
        }
        require.NoError(t, store.SaveApplication(ctx, app))
    }

    // Page 1
    page1, total1, err := store.ListApplicationsPaged(ctx, 2, nil)
    require.NoError(t, err)
    assert.Len(t, page1, 2)
    assert.Equal(t, 5, total1, "total_count on page 1 should be 5")

    // Page 2 (using cursor from page 1)
    createdAt1, _ := time.Parse(time.RFC3339, page1[len(page1)-1].CreatedAt)
    cursor := &models.ApplicationCursor{CreatedAt: createdAt1, ID: page1[len(page1)-1].ID}
    page2, total2, err := store.ListApplicationsPaged(ctx, 2, cursor)
    require.NoError(t, err)
    assert.Len(t, page2, 2)
    assert.Equal(t, 5, total2, "total_count on page 2 must equal total_count on page 1")
}

func TestSQLiteStorage_ListReleasesPaged_TotalCountStable(t *testing.T) {
    store, err := NewSQLiteStorage(":memory:")
    require.NoError(t, err)
    defer store.Close()
    ctx := context.Background()

    app := &models.Application{
        ID: "app1", Name: "App1", Platforms: []string{"windows"},
        Config: models.ApplicationConfig{},
        CreatedAt: time.Now().UTC().Format(time.RFC3339),
        UpdatedAt: time.Now().UTC().Format(time.RFC3339),
    }
    require.NoError(t, store.SaveApplication(ctx, app))

    now := time.Now().UTC()
    for i := range 5 {
        r := &models.Release{
            ID: fmt.Sprintf("r%d", i), ApplicationID: "app1",
            Version: fmt.Sprintf("1.0.%d", i), Platform: "windows", Architecture: "amd64",
            DownloadURL: "http://example.com", Checksum: "abc", ChecksumType: "sha256",
            ReleaseDate: now.Add(time.Duration(i) * time.Second),
            CreatedAt:   now.Add(time.Duration(i) * time.Second),
        }
        require.NoError(t, store.SaveRelease(ctx, r))
    }

    // Page 1
    page1, total1, err := store.ListReleasesPaged(ctx, "app1", models.ReleaseFilters{}, "release_date", "desc", 2, nil)
    require.NoError(t, err)
    assert.Len(t, page1, 2)
    assert.Equal(t, 5, total1)

    // Page 2
    cursor := &models.ReleaseCursor{
        SortBy: "release_date", SortOrder: "desc",
        ID: page1[len(page1)-1].ID, ReleaseDate: page1[len(page1)-1].ReleaseDate,
    }
    page2, total2, err := store.ListReleasesPaged(ctx, "app1", models.ReleaseFilters{}, "release_date", "desc", 2, cursor)
    require.NoError(t, err)
    assert.Len(t, page2, 2)
    assert.Equal(t, 5, total2, "total_count on page 2 must equal total_count on page 1")
}
```

### Step 2: Run to confirm failure

```bash
make test
```

Expected: FAIL — `total_count` on page 2 is 3 (remaining rows), not 5.

### Step 3: Refactor ListApplicationsPaged

Apply the same subquery restructuring as in Task 4. The SQLite version uses `?` placeholders and
its `where` variable already only holds the cursor condition (no business filters for applications):

```go
query := fmt.Sprintf(`
    SELECT id, name, description, platforms, config, created_at, updated_at, total_count
    FROM (
        SELECT id, name, description, platforms, config, created_at, updated_at,
               COUNT(*) OVER() AS total_count
        FROM applications
    ) AS counted
    %s
    ORDER BY created_at DESC, id DESC
    LIMIT ?`,
    where)
```

The `where` variable and `args` are unchanged — they still hold the optional cursor condition and
the limit. The only change is moving the WHERE from the inner query to the outer.

### Step 4: Refactor ListReleasesPaged

Same as Task 4 but with `?` placeholders. Rename `where` to `businessWhere`, build a separate
`keysetWhere`:

```go
// Build business filter WHERE
args := []interface{}{appID}
businessWhere := "WHERE application_id = ?"
if filters.Architecture != "" {
    args = append(args, filters.Architecture)
    businessWhere += " AND architecture = ?"
}
if filters.Version != "" {
    args = append(args, filters.Version)
    businessWhere += " AND version = ?"
}
if filters.Required != nil {
    args = append(args, *filters.Required)
    businessWhere += " AND required = ?"
}
if len(filters.Platforms) > 0 {
    placeholders := make([]string, len(filters.Platforms))
    for i, p := range filters.Platforms {
        placeholders[i] = "?"
        args = append(args, p)
    }
    businessWhere += " AND platform IN (" + strings.Join(placeholders, ",") + ")"
}

// Build keyset cursor WHERE (applied to outer query)
keysetWhere := ""
if cursor != nil {
    switch sortBy {
    case "version":
        isStable := 0
        if cursor.VersionIsStable {
            isStable = 1
        }
        args = append(args,
            cursor.VersionMajor,
            cursor.VersionMajor, cursor.VersionMinor,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch, isStable,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch, isStable, cursor.VersionPreRelease,
            cursor.VersionMajor, cursor.VersionMinor, cursor.VersionPatch, isStable, cursor.VersionPreRelease, cursor.ID,
        )
        keysetWhere = `WHERE (
  version_major < ?
  OR (version_major = ? AND version_minor < ?)
  OR (version_major = ? AND version_minor = ? AND version_patch < ?)
  OR (version_major = ? AND version_minor = ? AND version_patch = ? AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END < ?)
  OR (version_major = ? AND version_minor = ? AND version_patch = ? AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END = ? AND COALESCE(version_pre_release, '') > ?)
  OR (version_major = ? AND version_minor = ? AND version_patch = ? AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END = ? AND COALESCE(version_pre_release, '') = ? AND id < ?)
)`
    case "platform":
        args = append(args, cursor.Platform, cursor.Platform, cursor.ID)
        if sortOrder == "desc" {
            keysetWhere = "WHERE ((platform < ?) OR (platform = ? AND id < ?))"
        } else {
            keysetWhere = "WHERE ((platform > ?) OR (platform = ? AND id > ?))"
        }
    case "architecture":
        args = append(args, cursor.Architecture, cursor.Architecture, cursor.ID)
        if sortOrder == "desc" {
            keysetWhere = "WHERE ((architecture < ?) OR (architecture = ? AND id < ?))"
        } else {
            keysetWhere = "WHERE ((architecture > ?) OR (architecture = ? AND id > ?))"
        }
    case "created_at":
        createdAtStr := cursor.CreatedAt.UTC().Format(time.RFC3339)
        args = append(args, createdAtStr, createdAtStr, cursor.ID)
        if sortOrder == "desc" {
            keysetWhere = "WHERE ((created_at < ?) OR (created_at = ? AND id < ?))"
        } else {
            keysetWhere = "WHERE ((created_at > ?) OR (created_at = ? AND id > ?))"
        }
    default: // release_date
        releaseDateStr := cursor.ReleaseDate.UTC().Format(time.RFC3339)
        args = append(args, releaseDateStr, releaseDateStr, cursor.ID)
        if sortOrder == "desc" {
            keysetWhere = "WHERE ((release_date < ?) OR (release_date = ? AND id < ?))"
        } else {
            keysetWhere = "WHERE ((release_date > ?) OR (release_date = ? AND id > ?))"
        }
    }
}

args = append(args, int64(limit))
query := fmt.Sprintf(`
    SELECT id, application_id, version, platform, architecture, download_url,
           checksum, checksum_type, file_size, release_notes, release_date,
           required, minimum_version, metadata, created_at,
           version_major, version_minor, version_patch, version_pre_release,
           total_count
    FROM (
        SELECT id, application_id, version, platform, architecture, download_url,
               checksum, checksum_type, file_size, release_notes, release_date,
               required, minimum_version, metadata, created_at,
               version_major, version_minor, version_patch, version_pre_release,
               COUNT(*) OVER() AS total_count
        FROM releases
        %s
    ) AS counted
    %s
    ORDER BY %s
    LIMIT ?`,
    businessWhere, keysetWhere, orderClause,
)
```

### Step 5: Run tests and verify they pass

```bash
make test
```

Expected: PASS.

### Step 6: Commit

```bash
git add internal/storage/sqlite.go internal/storage/sqlite_test.go
git commit -m "fix: use subquery so COUNT(*) OVER() counts all business-filtered rows in sqlite"
```

---

## Task 6: Fix pre-release sort direction mismatch in postgres.go (review item #4)

The SQL ORDER BY for version sort uses `version_pre_release` with no explicit direction, defaulting
to ASC. This gives: `stable, alpha, beta, rc` — the opposite of what the semver library produces
(`stable, rc, beta, alpha`) in the memory store.

Fix: add `DESC` to the `version_pre_release` column in the ORDER BY fragment, and flip the keyset
comparison from `>` to `<` (since "after" in DESC order means alphabetically smaller).

**Files:**
- Modify: `internal/storage/postgres.go` — `pgReleaseListSortCols` map and the version keyset condition

### Step 1: Update the ORDER BY fragment

In `pgReleaseListSortCols`, change the `"version"` entry:

```go
// Before:
"version": "version_major DESC, version_minor DESC, version_patch DESC, (version_pre_release IS NULL) DESC, version_pre_release",

// After:
"version": "version_major DESC, version_minor DESC, version_patch DESC, (version_pre_release IS NULL) DESC, version_pre_release DESC",
```

### Step 2: Flip the keyset pre-release comparison

In the version keyset WHERE block (the `case "version":` in `ListReleasesPaged`), find the line:

```go
OR (version_major = $%d AND version_minor = $%d AND version_patch = $%d AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END = $%d AND COALESCE(version_pre_release, '') > $%d)
```

Change `> $%d` to `< $%d`:

```go
OR (version_major = $%d AND version_minor = $%d AND version_patch = $%d AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END = $%d AND COALESCE(version_pre_release, '') < $%d)
```

### Step 3: Verify compilation

```bash
make vet
```

Expected: no errors.

### Step 4: Commit

```bash
git add internal/storage/postgres.go
git commit -m "fix: sort version_pre_release DESC in postgres to match semver ordering"
```

---

## Task 7: Fix pre-release sort direction mismatch in sqlite.go (review item #4)

Same as Task 6 but for SQLite.

**Files:**
- Modify: `internal/storage/sqlite.go` — `sqliteReleaseListSortCols` map and version keyset condition
- Modify: `internal/storage/sqlite_test.go` — add pre-release sort order test

### Step 1: Write a failing test

In `internal/storage/sqlite_test.go`, add:

```go
func TestSQLiteStorage_ListReleasesPaged_VersionSortPreReleaseOrder(t *testing.T) {
    // Verifies that version sort with desc puts stable > rc > beta > alpha (not stable > alpha > beta > rc).
    store, err := NewSQLiteStorage(":memory:")
    require.NoError(t, err)
    defer store.Close()
    ctx := context.Background()

    app := &models.Application{
        ID: "app1", Name: "App1", Platforms: []string{"windows"},
        Config: models.ApplicationConfig{},
        CreatedAt: time.Now().UTC().Format(time.RFC3339),
        UpdatedAt: time.Now().UTC().Format(time.RFC3339),
    }
    require.NoError(t, store.SaveApplication(ctx, app))

    now := time.Now().UTC()
    releases := []struct{ id, ver string }{
        {"r1", "1.0.0"},
        {"r2", "1.0.0-alpha"},
        {"r3", "1.0.0-beta"},
        {"r4", "1.0.0-rc"},
    }
    for i, rel := range releases {
        r := &models.Release{
            ID: rel.id, ApplicationID: "app1", Version: rel.ver,
            Platform: "windows", Architecture: "amd64",
            DownloadURL: "http://example.com", Checksum: "abc", ChecksumType: "sha256",
            ReleaseDate: now.Add(time.Duration(i) * time.Second),
            CreatedAt:   now.Add(time.Duration(i) * time.Second),
        }
        require.NoError(t, store.SaveRelease(ctx, r))
    }

    // Version sort desc: stable, rc, beta, alpha
    results, _, err := store.ListReleasesPaged(ctx, "app1", models.ReleaseFilters{}, "version", "desc", 10, nil)
    require.NoError(t, err)
    require.Len(t, results, 4)
    assert.Equal(t, "1.0.0", results[0].Version)
    assert.Equal(t, "1.0.0-rc", results[1].Version)
    assert.Equal(t, "1.0.0-beta", results[2].Version)
    assert.Equal(t, "1.0.0-alpha", results[3].Version)
}
```

### Step 2: Run to confirm failure

```bash
make test
```

Expected: FAIL — order is `1.0.0, 1.0.0-alpha, 1.0.0-beta, 1.0.0-rc` (ASC) instead of
`1.0.0, 1.0.0-rc, 1.0.0-beta, 1.0.0-alpha` (DESC).

### Step 3: Apply the fix

In `sqliteReleaseListSortCols`:

```go
// Before:
"version": "version_major DESC, version_minor DESC, version_patch DESC, (version_pre_release IS NULL) DESC, version_pre_release",

// After:
"version": "version_major DESC, version_minor DESC, version_patch DESC, (version_pre_release IS NULL) DESC, version_pre_release DESC",
```

In the SQLite version keyset WHERE block (`case "version":` in `ListReleasesPaged`), flip `> ?`
to `< ?` on the pre-release comparison line:

```
// Before:
  OR (version_major = ? AND version_minor = ? AND version_patch = ? AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END = ? AND COALESCE(version_pre_release, '') > ?)

// After:
  OR (version_major = ? AND version_minor = ? AND version_patch = ? AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END = ? AND COALESCE(version_pre_release, '') < ?)
```

### Step 4: Run tests and verify they pass

```bash
make test
```

Expected: PASS.

### Step 5: Commit

```bash
git add internal/storage/sqlite.go internal/storage/sqlite_test.go
git commit -m "fix: sort version_pre_release DESC in sqlite to match semver ordering"
```

---

## Task 8: Fix memory store cursor-not-found behaviour (review item #7)

When a cursor ID is absent from the sorted slice (item deleted between page requests), the memory
store silently returns results from `start=0` (page 1 again). The SQL backends return zero rows
in this case because the keyset WHERE matches nothing.

**Files:**
- Modify: `internal/storage/memory.go` — `ListApplicationsPaged` and `ListReleasesPaged`
- Modify: `internal/storage/memory_test.go` — add not-found cursor tests

### Step 1: Write failing tests

In `internal/storage/memory_test.go`, add:

```go
func TestMemoryStorage_ListApplicationsPaged_CursorNotFound(t *testing.T) {
    store, err := NewMemoryStorage()
    require.NoError(t, err)
    defer store.Close()
    ctx := context.Background()

    now := time.Now()
    for i := range 3 {
        app := &models.Application{
            ID: fmt.Sprintf("app-%d", i), Name: fmt.Sprintf("App %d", i),
            Platforms: []string{"windows"}, Config: models.ApplicationConfig{},
            CreatedAt: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
            UpdatedAt: now.Format(time.RFC3339),
        }
        require.NoError(t, store.SaveApplication(ctx, app))
    }

    // Cursor points to a non-existent item.
    cursor := &models.ApplicationCursor{
        CreatedAt: now.Add(-1 * time.Hour),
        ID:        "does-not-exist",
    }
    results, _, err := store.ListApplicationsPaged(ctx, 10, cursor)
    require.NoError(t, err)
    assert.Empty(t, results, "cursor pointing to deleted item must return empty slice, not page 1")
}

func TestMemoryStorage_ListReleasesPaged_CursorNotFound(t *testing.T) {
    store, err := NewMemoryStorage()
    require.NoError(t, err)
    defer store.Close()
    ctx := context.Background()

    app := &models.Application{
        ID: "app1", Name: "App1", Platforms: []string{"windows"},
        Config: models.ApplicationConfig{},
        CreatedAt: time.Now().Format(time.RFC3339),
    }
    require.NoError(t, store.SaveApplication(ctx, app))

    now := time.Now()
    for i := range 3 {
        r := &models.Release{
            ID: fmt.Sprintf("r%d", i), ApplicationID: "app1",
            Version: fmt.Sprintf("1.0.%d", i), Platform: "windows", Architecture: "amd64",
            DownloadURL: "http://example.com", Checksum: "abc", ChecksumType: "sha256",
            ReleaseDate: now.Add(time.Duration(i) * time.Second),
            CreatedAt:   now.Add(time.Duration(i) * time.Second),
        }
        require.NoError(t, store.SaveRelease(ctx, r))
    }

    cursor := &models.ReleaseCursor{
        SortBy: "release_date", SortOrder: "desc",
        ID: "does-not-exist", ReleaseDate: now,
    }
    results, _, err := store.ListReleasesPaged(ctx, "app1", models.ReleaseFilters{}, "release_date", "desc", 10, cursor)
    require.NoError(t, err)
    assert.Empty(t, results, "cursor pointing to deleted item must return empty slice, not page 1")
}
```

### Step 2: Run to confirm failure

```bash
make test
```

Expected: FAIL — both functions return non-empty results when the cursor ID is not found.

### Step 3: Fix ListApplicationsPaged

In `memory.go`, replace the cursor-scan block:

```go
// Before:
start := 0
if cursor != nil {
    for idx, app := range apps {
        if app.ID == cursor.ID {
            start = idx + 1
            break
        }
    }
}

// After:
start := 0
if cursor != nil {
    found := false
    for idx, app := range apps {
        if app.ID == cursor.ID {
            start = idx + 1
            found = true
            break
        }
    }
    if !found {
        return []*models.Application{}, total, nil
    }
}
```

### Step 4: Fix ListReleasesPaged

Same pattern:

```go
// Before:
start := 0
if cursor != nil {
    for idx, r := range filtered {
        if r.ID == cursor.ID {
            start = idx + 1
            break
        }
    }
}

// After:
start := 0
if cursor != nil {
    found := false
    for idx, r := range filtered {
        if r.ID == cursor.ID {
            start = idx + 1
            found = true
            break
        }
    }
    if !found {
        return []*models.Release{}, total, nil
    }
}
```

### Step 5: Run tests and verify they pass

```bash
make test
```

Expected: PASS.

### Step 6: Commit

```bash
git add internal/storage/memory.go internal/storage/memory_test.go
git commit -m "fix: return empty slice when cursor ID not found in memory store"
```

---

## Task 9: Verify CI coverage threshold

Run the full test suite with coverage and check against the 65% threshold.

### Step 1: Run coverage

```bash
make cover
```

Look at the `total` line at the bottom. If coverage is ≥ 65%, the CI gate passes — done.

If coverage is below 65%, identify the uncovered packages and add targeted tests for the
highest-impact gaps. Key areas that are likely under-covered after this PR:

- `internal/models/cursor.go` — encode/decode paths
- `internal/update/service.go` — `ListReleases` and `ListApplications` pagination branches

For each gap: write the failing test, confirm it fails, implement only what's needed to cover it,
confirm it passes, commit.

### Step 2: Run integration tests

```bash
make integration-test
```

Expected: PASS.

### Step 3: Run OpenAPI validation

```bash
make openapi-validate
```

Expected: PASS.

### Step 4: Final commit (if coverage tests were added)

```bash
git add <any new test files>
git commit -m "test: add coverage for pagination cursor paths"
```

---

## Execution Checklist

- [ ] Task 1 — Fix integration test copy-paste
- [ ] Task 2 — Fix omitempty on NextCursor
- [ ] Task 3 — Fix service.go cursor logic
- [ ] Task 4 — Fix COUNT(*) OVER() in postgres.go
- [ ] Task 5 — Fix COUNT(*) OVER() in sqlite.go
- [ ] Task 6 — Fix pre-release sort direction in postgres.go
- [ ] Task 7 — Fix pre-release sort direction in sqlite.go
- [ ] Task 8 — Fix memory store cursor-not-found
- [ ] Task 9 — Verify coverage threshold passes