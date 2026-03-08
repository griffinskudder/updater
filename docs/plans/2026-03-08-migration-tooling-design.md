# Database Migration Tooling Design

**Issue:** [#71](https://github.com/griffinskudder/updater/issues/71)
**Date:** 2026-03-08
**Status:** Approved

## Problem

The project has no unified migration system. SQLite has a hand-rolled migrator that runs on startup. PostgreSQL has no migration mechanism at all. Schema files exist as numbered SQL files with no tooling to apply, track, or roll back changes.

## Decision

Use [goose](https://github.com/pressly/goose) (pure Go, no CGO) as the migration library, exposed via a separate `cmd/migrate/` binary that embeds migration files and ships in the same Docker image as the `updater` binary.

### Why goose

- Works directly with `*sql.DB` (reuses existing connection patterns)
- Native `embed.FS` support
- Single-file migrations with `-- +goose Up` / `-- +goose Down` annotations
- Sequential numbered filenames match existing `001_*.sql` convention
- sqlc natively parses goose annotations (ignores `-- +goose Down` blocks)

### Why a separate binary (not a subcommand)

- Keeps the `updater` binary focused on serving
- Different entrypoint to the same Docker image (`--entrypoint /bin/migrate`)
- Clean fit for Kubernetes init containers and CI pipelines
- No subcommand framework needed in the main binary

## Architecture

### Directory Structure

```
cmd/migrate/
  migrate.go              # Entry point: parse args, call goose API
internal/storage/
  migrations/
    postgres/
      001_initial.sql     # Goose-annotated, sqlc reads this too
    sqlite/
      001_initial.sql     # Goose-annotated, sqlc reads this too
  queries/
    postgres/
      queries.sql         # SQL queries for sqlc (moved from sqlc/schema/postgres/)
    sqlite/
      queries.sql         # SQL queries for sqlc (moved from sqlc/schema/sqlite/)
  sqlc/
    postgres/             # Generated code (unchanged)
    sqlite/               # Generated code (unchanged)
```

Migration files serve dual purpose: goose uses them for migrations, sqlc uses them as schema source. The `-- +goose Down` sections are ignored by sqlc. Query files live in a separate `queries/` directory because goose attempted to parse `.sql` query files as migrations when they were co-located with migration files.

### Deleted

- `internal/storage/sqlc/schema/` -- replaced by `internal/storage/migrations/`
- `runSQLiteMigrations()` in `internal/storage/sqlite.go` -- replaced by the `migrate` binary
- `schema_migrations` table -- replaced by goose's `goose_db_version` table

## CLI Interface

```
migrate --dialect postgres --dsn "postgres://..." up
migrate --dialect sqlite --dsn "./data/updater.db" status
migrate --dialect postgres --dsn "..." down
```

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--dialect` | Yes | `postgres` or `sqlite` |
| `--dsn` | Yes | Database connection string |
| `--verbose` / `-v` | No | Enable verbose output |

### Commands

All commands map directly to goose's Go API:

| Command | Description |
|---------|-------------|
| `up` | Apply all pending migrations |
| `up-to VERSION` | Migrate up to a specific version |
| `down` | Roll back one migration |
| `down-to VERSION` | Roll back to a specific version |
| `status` | Show migration status |
| `version` | Print current schema version |
| `redo` | Re-run the latest migration |
| `reset` | Roll back all migrations |

## Migration Files

Existing migrations collapse into a single `001_initial.sql` per engine. The content is the final schema state (all current migrations applied in sequence).

### PostgreSQL `001_initial.sql`

Combined result of existing 001-005: `applications`, `releases`, `api_keys` tables with all indexes, triggers, FK constraints, and semver sort columns.

### SQLite `001_initial.sql`

Combined result of existing 001-006: same tables adapted for SQLite types, plus datetime trigger fixes.

### Format

```sql
-- +goose Up
CREATE TABLE applications (...);
CREATE TABLE releases (...);
CREATE TABLE api_keys (...);
-- indexes, triggers, etc.

-- +goose Down
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS releases;
DROP TABLE IF EXISTS applications;
```

## Docker Changes

The Dockerfile builds both binaries in the build stage and copies both into the distroless final image.

```dockerfile
RUN go build -o /bin/updater ./cmd/updater
RUN go build -o /bin/migrate ./cmd/migrate

COPY --from=builder /bin/updater /bin/updater
COPY --from=builder /bin/migrate /bin/migrate
```

Default entrypoint remains `updater`. Migrations run via:

```
docker run --entrypoint /bin/migrate myimage --dialect postgres --dsn "..." up
```

Kubernetes init container:

```yaml
initContainers:
  - name: migrate
    image: myimage
    command: ["/bin/migrate"]
    args: ["--dialect", "postgres", "--dsn", "$(DSN)", "up"]
```

## Make Targets

New targets added to `make/db.mk`:

| Target | Description |
|--------|-------------|
| `make migrate-up` | Apply pending migrations (requires `DIALECT`, `DSN`) |
| `make migrate-down` | Roll back one migration |
| `make migrate-status` | Show migration status |
| `make migrate-create` | Create a new migration file |
| `make sqlc-diff` | Fail if `sqlc-generate` would change generated output (CI check) |

All migrate targets run the `migrate` binary inside Docker, consistent with the project's "everything runs in Docker" pattern.

## sqlc Configuration

`sqlc.yaml` updated to point `schema:` at the new migration directories:

```yaml
sql:
  - engine: postgresql
    schema: "internal/storage/migrations/postgres/"
    queries: "internal/storage/queries/postgres/"
    # ...
  - engine: sqlite
    schema: "internal/storage/migrations/sqlite/"
    queries: "internal/storage/queries/sqlite/"
    # ...
```

## Verification

After moving files and adding goose annotations, run `make sqlc-generate` and verify the generated Go code has zero diff. This confirms the schema is equivalent.

A new `make sqlc-diff` CI target runs `sqlc-generate` and fails if there are uncommitted changes to generated files, catching schema drift permanently.

## Testing

- Unit tests for the `cmd/migrate/` binary (flag parsing, dialect selection)
- Integration tests that run migrations against in-memory SQLite and verify schema state
- PostgreSQL integration tests (gated on `POSTGRES_TEST_DSN`)
- Verification that `make sqlc-generate` produces identical output after migration file restructuring