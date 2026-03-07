# Storage

The updater service uses a pluggable storage layer that supports multiple backends for persisting application and release metadata. All providers implement the same `Storage` interface, enabling seamless switching between backends based on deployment needs.

## Architecture

```mermaid
graph TD
    A[Storage Interface] --> C[Memory Storage]
    A --> D[PostgreSQL Storage]
    A --> E[SQLite Storage]
    G[Configuration] --> A
    D --> H[sqlc Generated Queries]
    E --> I[sqlc Generated Queries]
    H --> J[PostgreSQL Database]
    I --> K[SQLite Database]
```

## Provider Comparison

| Feature | Memory | SQLite | PostgreSQL |
|---------|--------|--------|------------|
| Persistence | None (in-memory) | File-based | Server-based |
| Concurrency | Thread-safe (RWMutex) | Single writer (WAL mode) | Full concurrent access |
| Setup | No setup needed | No external dependencies | Requires PostgreSQL server |
| Performance | Fastest (no I/O) | Good for medium datasets | Best for large datasets |
| Use Case | Testing, development | Single-server deployments | Production, multi-server |
| CGO Required | No | No (pure Go driver) | No |
| Schema Management | Automatic | Automatic (embedded DDL) | Manual migration required |

## Storage Interface

All providers implement 19 methods covering application, release, and API key CRUD operations, plus pagination, filtering, aggregate statistics, and health and lifecycle management:

```mermaid
classDiagram
    class Storage {
        <<interface>>
        +ListApplicationsPaged(ctx, limit, cursor) []*Application, int, error
        +GetApplication(ctx, appID) *Application, error
        +SaveApplication(ctx, app) error
        +DeleteApplication(ctx, appID) error
        +ListReleasesPaged(ctx, appID, filters, sortBy, sortOrder, limit, cursor) []*Release, int, error
        +GetRelease(ctx, appID, version, platform, arch) *Release, error
        +SaveRelease(ctx, release) error
        +DeleteRelease(ctx, appID, version, platform, arch) error
        +GetLatestRelease(ctx, appID, platform, arch) *Release, error
        +GetLatestStableRelease(ctx, appID, platform, arch) *Release, error
        +GetReleasesAfterVersion(ctx, appID, version, platform, arch) []*Release, error
        +GetApplicationStats(ctx, appID) ApplicationStats, error
        +Ping(ctx) error
        +Close() error
        +CreateAPIKey(ctx, key) error
        +GetAPIKeyByHash(ctx, hash) *APIKey, error
        +ListAPIKeys(ctx) []*APIKey, error
        +UpdateAPIKey(ctx, key) error
        +DeleteAPIKey(ctx, id) error
    }
```

### Pagination and Query Methods

The four purpose-built query methods push pagination, filtering, and aggregation down to the storage layer rather than loading all records into memory. Both list methods use keyset (cursor) pagination: a `cursor` argument encodes the last item seen, and the storage layer appends a keyset `WHERE` condition so the database skips directly to the next page without scanning discarded rows.

#### `ListApplicationsPaged`

```go
ListApplicationsPaged(ctx context.Context, limit int, cursor *models.ApplicationCursor) ([]*models.Application, int, error)
```

Returns a page of applications sorted by `created_at DESC, id DESC`, and the total count of all applications. When `cursor` is non-nil the query returns only items that follow the cursor item (keyset pagination). Pass `nil` to fetch the first page. The total count reflects all applications regardless of cursor position.

#### `ListReleasesPaged`

```go
ListReleasesPaged(ctx context.Context, appID string, filters models.ReleaseFilters, sortBy, sortOrder string, limit int, cursor *models.ReleaseCursor) ([]*models.Release, int, error)
```

Returns a filtered, sorted page of releases for the given application, and the total count of matching releases. The `filters` parameter narrows the result set before pagination is applied (see [ReleaseFilters](#releasefilters) below). When `cursor` is non-nil the query returns only items that follow the cursor item (keyset pagination). Pass `nil` to fetch the first page.

`sortBy` must be one of: `release_date`, `version`, `platform`, `architecture`, `created_at`.
`sortOrder` must be `"asc"` or `"desc"`.

When `sortBy` is `"version"`, releases are ordered using the dedicated version sort columns (see [Semver Sort Columns](#semver-sort-columns)); for all other columns, the value is used directly in the `ORDER BY` clause.

#### `GetLatestStableRelease`

```go
GetLatestStableRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error)
```

Returns the highest non-prerelease version for the given application, platform, and architecture. Ordering is performed at the SQL level using the version sort columns. Returns `storage.ErrNotFound` if no stable release exists.

#### `GetApplicationStats`

```go
GetApplicationStats(ctx context.Context, appID string) (models.ApplicationStats, error)
```

Returns aggregate statistics for an application computed in a single query. The returned `ApplicationStats` struct contains:

| Field | Type | Description |
|---|---|---|
| `TotalReleases` | `int` | Total number of releases for the application |
| `RequiredReleases` | `int` | Number of releases marked as required |
| `PlatformCount` | `int` | Number of distinct platforms with releases |
| `LatestVersion` | `string` | Version string of the most recent stable release |
| `LatestReleaseDate` | `*time.Time` | Release date of the most recent release |

### ReleaseFilters

`models.ReleaseFilters` specifies optional filters for `ListReleasesPaged`. A zero value or empty field means no filter is applied for that field.

| Field | Type | Description |
|---|---|---|
| `Platforms` | `[]string` | OR filter — a release matches if its platform equals any entry in the list |
| `Architecture` | `string` | Exact match on release architecture |
| `Version` | `string` | Exact match on release version string |
| `Required` | `*bool` | Filter by the required flag; `nil` means no filter |

### Semver Sort Columns

To enable SQL-level semantic version ordering without a regex function, each row in the `releases` table stores four additional integer/text columns populated by `SaveRelease`:

| Column | Type | Description |
|---|---|---|
| `version_major` | integer | Major version component |
| `version_minor` | integer | Minor version component |
| `version_patch` | integer | Patch version component |
| `version_pre_release` | text (nullable) | Pre-release label, or `NULL` for stable releases |

When sorting by version the storage layer constructs an `ORDER BY` clause over these four columns. Stable releases (`version_pre_release IS NULL`) sort above pre-release builds. Pre-release ordering within the same `major.minor.patch` is lexicographic, which approximates but does not fully replicate the SemVer 2.0 pre-release precedence rules.

## Configuration

### Memory Storage

```yaml
storage:
  type: memory
```

### SQLite Storage

```yaml
storage:
  type: sqlite
  database:
    dsn: ./data/updater.db
```

### PostgreSQL Storage

```yaml
storage:
  type: postgres
  database:
    dsn: postgres://user:password@localhost:5432/updater?sslmode=disable
```

## Provider Details

### Memory Storage

Stores data in Go maps protected by `sync.RWMutex`. Returns copies of all data to prevent external mutation. Data is lost on service restart. Ideal for testing and development environments.

### SQLite Storage

Uses the `modernc.org/sqlite` pure-Go driver (no CGO required). The schema is automatically created on startup using an embedded SQL file with `IF NOT EXISTS` guards. Key characteristics:

- **WAL mode** enabled for better concurrent read performance
- **Foreign keys** enabled for referential integrity
- **Single connection** to prevent concurrency issues with SQLite's single-writer model
- **Semver sort columns**: `version_major`, `version_minor`, `version_patch`, `version_pre_release` columns enable SQL-level version ordering
- **Upsert pattern**: SQL upserts (`INSERT ... ON CONFLICT`) for `SaveApplication` and `SaveRelease`

### PostgreSQL Storage

Uses `pgx/v5` with connection pooling via `pgxpool`. All queries are generated by sqlc for type safety. Key characteristics:

- **Connection pooling** for efficient resource usage
- **JSONB columns** for platforms, config, and metadata fields
- **Timestamptz** for proper timezone-aware timestamps
- **Cascade deletes** from applications to releases
- **Semver sort columns**: `version_major`, `version_minor`, `version_patch`, `version_pre_release` columns enable SQL-level version ordering
- **Upsert pattern**: SQL upserts (`INSERT ... ON CONFLICT`) for `SaveApplication` and `SaveRelease`

## Database Schema

Both database providers share the same logical schema with engine-specific type differences:

```mermaid
erDiagram
    applications {
        TEXT id PK
        TEXT name
        TEXT description
        JSON platforms
        JSON config
        TIMESTAMP created_at
        TIMESTAMP updated_at
    }
    releases {
        TEXT id PK
        TEXT application_id FK
        TEXT version
        INTEGER version_major
        INTEGER version_minor
        INTEGER version_patch
        TEXT version_pre_release
        TEXT platform
        TEXT architecture
        TEXT download_url
        TEXT checksum
        TEXT checksum_type
        BIGINT file_size
        TEXT release_notes
        TIMESTAMP release_date
        BOOLEAN required
        TEXT minimum_version
        JSON metadata
        TIMESTAMP created_at
    }
    api_keys {
        TEXT id PK
        TEXT name
        TEXT key_hash UK
        TEXT prefix
        TEXT permissions
        INTEGER enabled
        TIMESTAMP created_at
        TIMESTAMP updated_at
    }
    applications ||--o{ releases : "has many"
```

The `api_keys.permissions` column stores a JSON array of permission strings (e.g. `["admin"]`). The `enabled` column uses `INTEGER` (0/1) in SQLite and `BOOLEAN` in PostgreSQL.

### Type Differences

| Model Type | PostgreSQL | SQLite |
|-----------|-----------|--------|
| JSON fields | `JSONB` (binary, indexed) | `TEXT` (string) |
| Timestamps | `TIMESTAMPTZ` | `TEXT` (ISO8601) |
| Large integers | `BIGINT` | `INTEGER` |
| Nullable strings | `pgtype.Text` | `sql.NullString` |

## Type Conversion

Shared conversion helpers in `dbconvert.go` handle JSON marshaling for:

- **Platforms**: `[]string` to/from JSON
- **Config**: `ApplicationConfig` struct to/from JSON
- **Metadata**: `map[string]string` to/from JSON

Each provider has additional helpers for engine-specific type conversions (e.g., `pgtype.Text` for PostgreSQL, `sql.NullString` for SQLite).

## Database Schema Docs

The auto-generated schema reference (including ER diagrams per table) lives in
the [Database](db/README.md) section. It is generated from the live
PostgreSQL schema by running:

```bash
make docs-db
```

## Testing

- **Memory**: Full CRUD tests with concurrency testing
- **SQLite**: Full CRUD tests using `:memory:` (always runs, no external DB)
- **PostgreSQL**: Full CRUD tests skipped unless `POSTGRES_TEST_DSN` env var is set
