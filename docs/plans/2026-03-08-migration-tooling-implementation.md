# Database Migration Tooling Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the hand-rolled SQLite migrator with goose, add a `cmd/migrate/` binary, and unify migration management for both PostgreSQL and SQLite.

**Architecture:** A shared `internal/storage/migrations` package embeds goose-annotated SQL files. A thin `cmd/migrate/` binary parses CLI flags and delegates to goose's Go API. The `updater` binary no longer auto-migrates; migrations run explicitly via the migrate binary.

**Tech Stack:** goose v3 (pure Go, no CGO), embed.FS, sqlc (unchanged), modernc.org/sqlite, pgx/v5

---

### Task 1: Add goose dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Add goose to go.mod**

Run: `make tidy` after adding the import (will happen naturally in Task 6 when Go files import it)

For now, fetch the dependency:

```bash
# Run inside Docker to stay consistent:
docker run --rm -v "$(CURDIR):/src" -w /src golang:1.25-alpine go get github.com/pressly/goose/v3
```

Or add a Make target wrapper. The dependency will be pulled when the first Go file imports it, so this can also be deferred to Task 6.

**Step 2: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add goose v3 dependency for database migrations"
```

---

### Task 2: Create consolidated PostgreSQL migration file

**Files:**
- Create: `internal/storage/migrations/postgres/001_initial.sql`

**Step 1: Create the directory**

```bash
mkdir -p internal/storage/migrations/postgres
```

**Step 2: Write the consolidated migration**

Combine the final schema state from existing files 001-005 into a single goose-annotated migration. Key changes from the originals:
- Version sort columns are defined inline in the `releases` CREATE TABLE (no ALTER TABLE or backfill needed since there's no pre-existing data)
- FK is RESTRICT from the start (not CASCADE then altered)
- All indexes from 001 and 002 are present

```sql
-- +goose Up

-- Applications table
CREATE TABLE applications (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    platforms JSONB NOT NULL DEFAULT '[]',
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Releases table
CREATE TABLE releases (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL,
    version TEXT NOT NULL,
    platform TEXT NOT NULL,
    architecture TEXT NOT NULL,
    download_url TEXT NOT NULL,
    checksum TEXT NOT NULL,
    checksum_type TEXT NOT NULL DEFAULT 'sha256',
    file_size BIGINT NOT NULL,
    release_notes TEXT,
    release_date TIMESTAMPTZ NOT NULL,
    required BOOLEAN NOT NULL DEFAULT FALSE,
    minimum_version TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version_major BIGINT NOT NULL DEFAULT 0,
    version_minor BIGINT NOT NULL DEFAULT 0,
    version_patch BIGINT NOT NULL DEFAULT 0,
    version_pre_release TEXT,

    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE RESTRICT,
    UNIQUE(application_id, version, platform, architecture)
);

-- API keys table
CREATE TABLE api_keys (
    id          TEXT        NOT NULL PRIMARY KEY,
    name        TEXT        NOT NULL,
    key_hash    TEXT        NOT NULL UNIQUE,
    prefix      TEXT        NOT NULL,
    permissions JSONB       NOT NULL DEFAULT '[]',
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Application indexes
CREATE INDEX idx_applications_name ON applications(name);
CREATE INDEX idx_applications_platforms ON applications USING GIN(platforms);

-- Release indexes
CREATE INDEX idx_releases_app_platform_arch ON releases(application_id, platform, architecture);
CREATE INDEX idx_releases_version ON releases(version);
CREATE INDEX idx_releases_date ON releases(release_date DESC);
CREATE INDEX idx_releases_required ON releases(required);
CREATE INDEX idx_releases_required_date ON releases(required, release_date DESC);
CREATE INDEX idx_releases_metadata_gin ON releases USING GIN(metadata);
CREATE INDEX idx_releases_app_version ON releases(application_id, version);
CREATE INDEX idx_releases_version_sort ON releases(application_id, version_major DESC, version_minor DESC, version_patch DESC);

-- API key indexes
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);

-- Function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers
CREATE TRIGGER update_applications_updated_at
    BEFORE UPDATE ON applications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;
DROP TRIGGER IF EXISTS update_applications_updated_at ON applications;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS releases;
DROP TABLE IF EXISTS applications;
```

**Step 3: Commit**

```bash
git add internal/storage/migrations/postgres/001_initial.sql
git commit -m "feat(migrations): add consolidated PostgreSQL schema with goose annotations"
```

---

### Task 3: Create consolidated SQLite migration file

**Files:**
- Create: `internal/storage/migrations/sqlite/001_initial.sql`

**Step 1: Create the directory**

```bash
mkdir -p internal/storage/migrations/sqlite
```

**Step 2: Write the consolidated migration**

Combine the final schema state from existing files 001-006. Key changes:
- Version sort columns inline in CREATE TABLE
- Triggers use RFC3339 format from the start (006 fix applied)
- FK is CASCADE (SQLite cannot ALTER FK; RESTRICT enforced at service layer)
- No backfill logic needed

```sql
-- +goose Up

-- Applications table (SQLite version)
CREATE TABLE applications (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    platforms TEXT NOT NULL DEFAULT '[]',
    config TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Releases table (SQLite version)
CREATE TABLE releases (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL,
    version TEXT NOT NULL,
    platform TEXT NOT NULL,
    architecture TEXT NOT NULL,
    download_url TEXT NOT NULL,
    checksum TEXT NOT NULL,
    checksum_type TEXT NOT NULL DEFAULT 'sha256',
    file_size INTEGER NOT NULL,
    release_notes TEXT,
    release_date TEXT NOT NULL,
    required BOOLEAN NOT NULL DEFAULT 0,
    minimum_version TEXT,
    metadata TEXT DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    version_major INTEGER NOT NULL DEFAULT 0,
    version_minor INTEGER NOT NULL DEFAULT 0,
    version_patch INTEGER NOT NULL DEFAULT 0,
    version_pre_release TEXT,

    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE,
    UNIQUE(application_id, version, platform, architecture)
);

-- API keys table (SQLite version)
CREATE TABLE api_keys (
    id          TEXT NOT NULL PRIMARY KEY,
    name        TEXT NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,
    prefix      TEXT NOT NULL,
    permissions TEXT NOT NULL DEFAULT '[]',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Application indexes
CREATE INDEX idx_applications_name ON applications(name);

-- Release indexes
CREATE INDEX idx_releases_app_platform_arch ON releases(application_id, platform, architecture);
CREATE INDEX idx_releases_version ON releases(version);
CREATE INDEX idx_releases_date ON releases(release_date DESC);
CREATE INDEX idx_releases_required ON releases(required);
CREATE INDEX idx_releases_required_date ON releases(required, release_date DESC);
CREATE INDEX idx_releases_app_version ON releases(application_id, version);
CREATE INDEX idx_releases_version_sort ON releases(application_id, version_major DESC, version_minor DESC, version_patch DESC);

-- SQLite-specific indexes
CREATE INDEX idx_applications_config ON applications(config) WHERE config != '{}';

-- API key indexes
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);

-- Triggers (RFC3339 format)
CREATE TRIGGER update_applications_updated_at
    AFTER UPDATE ON applications
    FOR EACH ROW
BEGIN
    UPDATE applications SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;

CREATE TRIGGER update_api_keys_updated_at
    AFTER UPDATE ON api_keys
    FOR EACH ROW
BEGIN
    UPDATE api_keys SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;

-- +goose Down
DROP TRIGGER IF EXISTS update_api_keys_updated_at;
DROP TRIGGER IF EXISTS update_applications_updated_at;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS releases;
DROP TABLE IF EXISTS applications;
```

**Step 3: Commit**

```bash
git add internal/storage/migrations/sqlite/001_initial.sql
git commit -m "feat(migrations): add consolidated SQLite schema with goose annotations"
```

---

### Task 4: Move query files to migration directories

**Files:**
- Move: `internal/storage/sqlc/queries/postgres/api_keys.sql` -> `internal/storage/migrations/postgres/`
- Move: `internal/storage/sqlc/queries/postgres/applications.sql` -> `internal/storage/migrations/postgres/`
- Move: `internal/storage/sqlc/queries/postgres/releases.sql` -> `internal/storage/migrations/postgres/`
- Move: `internal/storage/sqlc/queries/sqlite/api_keys.sql` -> `internal/storage/migrations/sqlite/`
- Move: `internal/storage/sqlc/queries/sqlite/applications.sql` -> `internal/storage/migrations/sqlite/`
- Move: `internal/storage/sqlc/queries/sqlite/releases.sql` -> `internal/storage/migrations/sqlite/`

**Step 1: Move files**

```bash
mv internal/storage/sqlc/queries/postgres/*.sql internal/storage/migrations/postgres/
mv internal/storage/sqlc/queries/sqlite/*.sql internal/storage/migrations/sqlite/
```

**Step 2: Commit**

```bash
git add internal/storage/migrations/ internal/storage/sqlc/queries/
git commit -m "refactor(storage): move query files alongside migration files"
```

---

### Task 5: Update sqlc.yaml and verify generated code

**Files:**
- Modify: `sqlc.yaml`

**Step 1: Update sqlc.yaml**

Change schema and queries paths to point at the new migration directories:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/storage/migrations/postgres"
    schema: "internal/storage/migrations/postgres"
    gen:
      go:
        package: "sqlcpg"
        out: "internal/storage/sqlc/postgres"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_prepared_queries: false
        emit_interface: false
        emit_exact_table_names: false
        emit_empty_slices: true
  - engine: "sqlite"
    queries: "internal/storage/migrations/sqlite"
    schema: "internal/storage/migrations/sqlite"
    gen:
      go:
        package: "sqlcite"
        out: "internal/storage/sqlc/sqlite"
        emit_json_tags: true
        emit_prepared_queries: false
        emit_interface: false
        emit_exact_table_names: false
        emit_empty_slices: true
```

**Step 2: Run sqlc-generate and verify zero diff**

```bash
make sqlc-generate
git diff internal/storage/sqlc/postgres/ internal/storage/sqlc/sqlite/
```

Expected: zero diff in generated Go files. If there are differences, the consolidated migration schema doesn't match the original. Fix the migration files until the diff is empty.

**Step 3: Commit**

```bash
git add sqlc.yaml
git commit -m "build(sqlc): point schema and queries at migration directories"
```

---

### Task 6: Delete old schema and query directories

**Files:**
- Delete: `internal/storage/sqlc/schema/` (entire directory)
- Delete: `internal/storage/sqlc/queries/` (entire directory, should be empty after Task 4)

**Step 1: Remove old directories**

```bash
rm -rf internal/storage/sqlc/schema/ internal/storage/sqlc/queries/
```

**Step 2: Verify sqlc still works**

```bash
make sqlc-generate
make sqlc-vet
```

Expected: both pass with no errors.

**Step 3: Commit**

```bash
git add internal/storage/sqlc/schema/ internal/storage/sqlc/queries/
git commit -m "refactor(storage): remove old schema and query directories"
```

---

### Task 7: Create migrations embed package

**Files:**
- Create: `internal/storage/migrations/migrations.go`
- Create: `internal/storage/migrations/migrations_test.go`

**Step 1: Write the failing test**

```go
package migrations

import (
	"io/fs"
	"testing"
)

func TestPostgresFSContainsMigration(t *testing.T) {
	entries, err := fs.ReadDir(PostgresFS, "postgres")
	if err != nil {
		t.Fatalf("failed to read postgres dir: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Name() == "001_initial.sql" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 001_initial.sql in embedded postgres FS")
	}
}

func TestSQLiteFSContainsMigration(t *testing.T) {
	entries, err := fs.ReadDir(SQLiteFS, "sqlite")
	if err != nil {
		t.Fatalf("failed to read sqlite dir: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Name() == "001_initial.sql" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 001_initial.sql in embedded sqlite FS")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `make test`
Expected: FAIL -- `PostgresFS` and `SQLiteFS` not defined.

**Step 3: Write the implementation**

```go
// Package migrations embeds goose-annotated SQL migration files for all
// supported database engines. These embedded filesystems are used by the
// migrate binary at runtime and by tests for schema setup.
package migrations

import "embed"

// PostgresFS contains goose-annotated PostgreSQL migration files.
//
//go:embed postgres/*.sql
var PostgresFS embed.FS

// SQLiteFS contains goose-annotated SQLite migration files.
//
//go:embed sqlite/*.sql
var SQLiteFS embed.FS
```

**Step 4: Run test to verify it passes**

Run: `make test`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/storage/migrations/migrations.go internal/storage/migrations/migrations_test.go
git commit -m "feat(migrations): add embed package for goose migration files"
```

---

### Task 8: Write migrate binary (TDD)

**Files:**
- Create: `cmd/migrate/migrate.go`
- Create: `cmd/migrate/migrate_test.go`

**Step 1: Write the failing test**

Test flag parsing and dialect validation. The actual goose operations are tested via integration tests (Task 12).

```go
package main

import (
	"testing"
)

func TestParseArgs_ValidPostgres(t *testing.T) {
	args := []string{"--dialect", "postgres", "--dsn", "postgres://localhost/test", "up"}
	cfg, cmd, err := parseArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.dialect != "postgres" {
		t.Errorf("expected dialect postgres, got %s", cfg.dialect)
	}
	if cfg.dsn != "postgres://localhost/test" {
		t.Errorf("expected dsn postgres://localhost/test, got %s", cfg.dsn)
	}
	if cmd != "up" {
		t.Errorf("expected command up, got %s", cmd)
	}
}

func TestParseArgs_ValidSQLite(t *testing.T) {
	args := []string{"--dialect", "sqlite", "--dsn", "./data/test.db", "status"}
	cfg, cmd, err := parseArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.dialect != "sqlite" {
		t.Errorf("expected dialect sqlite, got %s", cfg.dialect)
	}
	if cmd != "status" {
		t.Errorf("expected command status, got %s", cmd)
	}
}

func TestParseArgs_MissingDialect(t *testing.T) {
	args := []string{"--dsn", "postgres://localhost/test", "up"}
	_, _, err := parseArgs(args)
	if err == nil {
		t.Error("expected error for missing dialect")
	}
}

func TestParseArgs_InvalidDialect(t *testing.T) {
	args := []string{"--dialect", "mysql", "--dsn", "test", "up"}
	_, _, err := parseArgs(args)
	if err == nil {
		t.Error("expected error for invalid dialect")
	}
}

func TestParseArgs_MissingDSN(t *testing.T) {
	args := []string{"--dialect", "postgres", "up"}
	_, _, err := parseArgs(args)
	if err == nil {
		t.Error("expected error for missing dsn")
	}
}

func TestParseArgs_MissingCommand(t *testing.T) {
	args := []string{"--dialect", "postgres", "--dsn", "test"}
	_, _, err := parseArgs(args)
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestParseArgs_InvalidCommand(t *testing.T) {
	args := []string{"--dialect", "postgres", "--dsn", "test", "invalid"}
	_, _, err := parseArgs(args)
	if err == nil {
		t.Error("expected error for invalid command")
	}
}

func TestParseArgs_UpToWithVersion(t *testing.T) {
	args := []string{"--dialect", "postgres", "--dsn", "test", "up-to", "3"}
	cfg, cmd, err := parseArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "up-to" {
		t.Errorf("expected command up-to, got %s", cmd)
	}
	if cfg.version != 3 {
		t.Errorf("expected version 3, got %d", cfg.version)
	}
}

func TestParseArgs_DownToWithVersion(t *testing.T) {
	args := []string{"--dialect", "postgres", "--dsn", "test", "down-to", "0"}
	cfg, cmd, err := parseArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "down-to" {
		t.Errorf("expected command down-to, got %s", cmd)
	}
	if cfg.version != 0 {
		t.Errorf("expected version 0, got %d", cfg.version)
	}
}

func TestParseArgs_UpToMissingVersion(t *testing.T) {
	args := []string{"--dialect", "postgres", "--dsn", "test", "up-to"}
	_, _, err := parseArgs(args)
	if err == nil {
		t.Error("expected error for up-to without version")
	}
}

func TestGooseDialect_Postgres(t *testing.T) {
	d := gooseDialect("postgres")
	if d != "postgres" {
		t.Errorf("expected postgres, got %s", d)
	}
}

func TestGooseDialect_SQLite(t *testing.T) {
	d := gooseDialect("sqlite")
	if d != "sqlite3" {
		t.Errorf("expected sqlite3, got %s", d)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `make test`
Expected: FAIL -- `parseArgs` and `gooseDialect` not defined.

**Step 3: Write the implementation**

```go
// Command migrate applies database migrations using goose.
//
// Usage:
//
//	migrate --dialect postgres --dsn "postgres://..." up
//	migrate --dialect sqlite --dsn "./data/updater.db" status
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strconv"

	"github.com/pressly/goose/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"updater/internal/storage/migrations"
)

// validCommands lists the goose commands the migrate binary supports.
var validCommands = map[string]bool{
	"up":       true,
	"up-to":    true,
	"down":     true,
	"down-to":  true,
	"status":   true,
	"version":  true,
	"redo":     true,
	"reset":    true,
	"validate": true,
}

// versionCommands are commands that require a VERSION argument.
var versionCommands = map[string]bool{
	"up-to":   true,
	"down-to": true,
}

type migrateConfig struct {
	dialect string
	dsn     string
	verbose bool
	version int64
}

func main() {
	args := os.Args[1:]
	cfg, cmd, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := runMigration(cfg, cmd); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
}

// parseArgs parses CLI arguments into a migrateConfig and command string.
func parseArgs(args []string) (migrateConfig, string, error) {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	dialect := fs.String("dialect", "", "Database dialect: postgres or sqlite")
	dsn := fs.String("dsn", "", "Database connection string")
	verbose := fs.Bool("verbose", false, "Enable verbose output")
	fs.BoolVar(verbose, "v", false, "Enable verbose output (shorthand)")

	if err := fs.Parse(args); err != nil {
		return migrateConfig{}, "", err
	}

	if *dialect == "" {
		return migrateConfig{}, "", fmt.Errorf("--dialect is required (postgres or sqlite)")
	}
	if *dialect != "postgres" && *dialect != "sqlite" {
		return migrateConfig{}, "", fmt.Errorf("invalid dialect %q: must be postgres or sqlite", *dialect)
	}
	if *dsn == "" {
		return migrateConfig{}, "", fmt.Errorf("--dsn is required")
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return migrateConfig{}, "", fmt.Errorf("command is required (up, down, status, version, redo, reset, validate, up-to, down-to)")
	}

	cmd := remaining[0]
	if !validCommands[cmd] {
		return migrateConfig{}, "", fmt.Errorf("invalid command %q", cmd)
	}

	cfg := migrateConfig{
		dialect: *dialect,
		dsn:     *dsn,
		verbose: *verbose,
	}

	if versionCommands[cmd] {
		if len(remaining) < 2 {
			return migrateConfig{}, "", fmt.Errorf("%s requires a VERSION argument", cmd)
		}
		v, err := strconv.ParseInt(remaining[1], 10, 64)
		if err != nil {
			return migrateConfig{}, "", fmt.Errorf("invalid version %q: %w", remaining[1], err)
		}
		cfg.version = v
	}

	return cfg, cmd, nil
}

// gooseDialect maps our dialect names to goose dialect strings.
func gooseDialect(dialect string) string {
	if dialect == "sqlite" {
		return "sqlite3"
	}
	return dialect
}

// driverName maps our dialect names to database/sql driver names.
func driverName(dialect string) string {
	if dialect == "postgres" {
		return "pgx"
	}
	return "sqlite"
}

// migrationFS returns the embedded filesystem and subdirectory for the given dialect.
func migrationFS(dialect string) (fs.FS, string) {
	if dialect == "postgres" {
		return migrations.PostgresFS, "postgres"
	}
	return migrations.SQLiteFS, "sqlite"
}

func runMigration(cfg migrateConfig, cmd string) error {
	goose.SetVerbose(cfg.verbose)

	if err := goose.SetDialect(gooseDialect(cfg.dialect)); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	embedFS, dir := migrationFS(cfg.dialect)
	goose.SetBaseFS(embedFS)

	db, err := sql.Open(driverName(cfg.dialect), cfg.dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	switch cmd {
	case "up":
		return goose.Up(db, dir)
	case "up-to":
		return goose.UpTo(db, dir, cfg.version)
	case "down":
		return goose.Down(db, dir)
	case "down-to":
		return goose.DownTo(db, dir, cfg.version)
	case "status":
		return goose.Status(db, dir)
	case "version":
		return goose.Version(db, dir)
	case "redo":
		return goose.Redo(db, dir)
	case "reset":
		return goose.Reset(db, dir)
	case "validate":
		return goose.Validate(db, dir)
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `make test`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/migrate/
git commit -m "feat(migrate): add migration CLI binary using goose"
```

---

### Task 9: Write integration test for migrate binary

**Files:**
- Create: `cmd/migrate/integration_test.go`

**Step 1: Write the integration test**

This test verifies that goose can apply and roll back migrations against a real SQLite database.

```go
//go:build integration

package main

import (
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"updater/internal/storage/migrations"
)

func TestSQLiteMigrationUpDown(t *testing.T) {
	// Create a temporary database file
	f, err := os.CreateTemp(t.TempDir(), "migrate-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	f.Close()
	dsn := f.Name()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}
	goose.SetBaseFS(migrations.SQLiteFS)

	// Apply migrations
	if err := goose.Up(db, "sqlite"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	// Verify tables exist
	tables := []string{"applications", "releases", "api_keys", "goose_db_version"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found after migration up: %v", table, err)
		}
	}

	// Verify version
	ver, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	if ver != 1 {
		t.Errorf("expected version 1, got %d", ver)
	}

	// Roll back
	if err := goose.Down(db, "sqlite"); err != nil {
		t.Fatalf("goose down: %v", err)
	}

	// Verify tables are gone
	for _, table := range []string{"applications", "releases", "api_keys"} {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err == nil {
			t.Errorf("table %s should not exist after migration down", table)
		}
	}
}
```

**Step 2: Run test to verify it passes**

Run: `make integration-test` (or `go test -tags integration ./cmd/migrate/...`)
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/migrate/integration_test.go
git commit -m "test(migrate): add integration test for SQLite migration up/down"
```

---

### Task 10: Remove hand-rolled SQLite migrator and update storage

**Files:**
- Modify: `internal/storage/sqlite.go`
- Modify: `internal/storage/sqlite_test.go`

**Step 1: Update sqlite.go**

Remove:
1. The `embed` import and `//go:embed sqlc/schema/sqlite` directive (line 20-21)
2. The `runSQLiteMigrations` call in `NewSQLiteStorage` (line 63-66)
3. The `runSQLiteMigrations` function (lines 826-878)
4. Remove unused imports: `"io/fs"`, `"sort"` (check if they're used elsewhere in the file first)

In `NewSQLiteStorage`, remove the migration call but keep everything else:

```go
// NewSQLiteStorage creates a new SQLite storage instance.
// The database must be migrated before use (see cmd/migrate).
func NewSQLiteStorage(dsn string) (Storage, error) {
	if dsn == "" {
		return nil, fmt.Errorf("connection string is required for SQLite storage")
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return &SQLiteStorage{
		db:      db,
		queries: sqlcite.New(db),
	}, nil
}
```

**Step 2: Update the test helper in sqlite_test.go**

The `newSQLiteTestStorage` helper needs to run goose migrations before creating storage, since `NewSQLiteStorage` no longer auto-migrates. Because `:memory:` databases are per-connection and `NewSQLiteStorage` opens its own connection, the test helper must create the `*sql.DB` directly and construct the storage from it.

Add an unexported constructor `newSQLiteStorageFromDB` to `sqlite.go`:

```go
// newSQLiteStorageFromDB creates a SQLiteStorage from an existing *sql.DB.
// Used by tests to share a pre-migrated in-memory database connection.
func newSQLiteStorageFromDB(db *sql.DB) *SQLiteStorage {
	return &SQLiteStorage{
		db:      db,
		queries: sqlcite.New(db),
	}
}
```

Update `sqlite_test.go`:

```go
import (
	// ... existing imports ...
	"database/sql"

	"github.com/pressly/goose/v3"
	"updater/internal/storage/migrations"
)

func newSQLiteTestStorage(t *testing.T) Storage {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("failed to enable WAL mode: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	goose.SetBaseFS(migrations.SQLiteFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("failed to set goose dialect: %v", err)
	}
	if err := goose.Up(db, "sqlite"); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	s := newSQLiteStorageFromDB(db)
	t.Cleanup(func() { s.Close() })
	return s
}
```

**Step 3: Run tests to verify they pass**

Run: `make test`
Expected: All existing SQLite tests PASS with the new test helper.

**Step 4: Commit**

```bash
git add internal/storage/sqlite.go internal/storage/sqlite_test.go
git commit -m "refactor(storage): remove hand-rolled SQLite migrator, use goose"
```

---

### Task 11: Update integration tests

**Files:**
- Modify: `internal/integration/integration_test.go` (if it creates SQLite storage)

**Step 1: Check if integration tests create SQLite storage directly**

Read `internal/integration/integration_test.go` and find any calls to `storage.NewSQLiteStorage`. If found, update them to use the same goose-based setup pattern from Task 10.

If integration tests only use `memory` storage, no changes needed.

**Step 2: Run integration tests**

Run: `make integration-test`
Expected: PASS

**Step 3: Commit (if changes were needed)**

```bash
git add internal/integration/
git commit -m "test(integration): update integration tests for goose migrations"
```

---

### Task 12: Update Dockerfile

**Files:**
- Modify: `Dockerfile`

**Step 1: Add migrate binary build step**

After the existing healthcheck build (line 58-65), add:

```dockerfile
# Build the migration binary
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
    -a \
    -ldflags='-w -s -extldflags "-static"' \
    -o migrate \
    ./cmd/migrate
```

**Step 2: Copy migrate binary to runtime stage**

After line 84 (`COPY --from=builder ... /usr/local/bin/healthcheck`), add:

```dockerfile
COPY --from=builder --chown=65532:65532 /build/migrate /usr/local/bin/migrate
```

**Step 3: Commit**

```bash
git add Dockerfile
git commit -m "build(docker): add migrate binary to container image"
```

---

### Task 13: Add Make targets

**Files:**
- Modify: `make/db.mk`

**Step 1: Add migration and sqlc-diff targets**

Append to `make/db.mk`:

```makefile
##@ Migrations

.PHONY: migrate-up migrate-down migrate-status migrate-create sqlc-diff

DIALECT ?= sqlite
DSN     ?= ./data/updater.db

migrate-up: ## Apply pending migrations (DIALECT=sqlite DSN=./data/updater.db)
	$(GO_DOCKER) go run ./cmd/migrate --dialect $(DIALECT) --dsn "$(DSN)" up

migrate-down: ## Roll back one migration (DIALECT=sqlite DSN=./data/updater.db)
	$(GO_DOCKER) go run ./cmd/migrate --dialect $(DIALECT) --dsn "$(DSN)" down

migrate-status: ## Show migration status (DIALECT=sqlite DSN=./data/updater.db)
	$(GO_DOCKER) go run ./cmd/migrate --dialect $(DIALECT) --dsn "$(DSN)" status

migrate-create: ## Create a new migration file (NAME=add_column DIALECT=sqlite)
ifndef NAME
	$(error NAME is required, e.g. make migrate-create NAME=add_column DIALECT=sqlite)
endif
	@echo "-- +goose Up\n\n-- +goose Down" > internal/storage/migrations/$(DIALECT)/$$(printf '%03d' $$(($$(ls internal/storage/migrations/$(DIALECT)/*.sql 2>/dev/null | grep -c '_') + 1)))_$(NAME).sql
	@echo "Created migration: internal/storage/migrations/$(DIALECT)/$$(printf '%03d' $$(($$(ls internal/storage/migrations/$(DIALECT)/*.sql 2>/dev/null | grep -c '_') + 1)))_$(NAME).sql"

sqlc-diff: ## Fail if sqlc-generate would change generated output
	@echo "Checking for sqlc drift..."
	docker run --rm -v "$(CURDIR):/src" -w /src $(SQLC_IMAGE) generate
	@if [ -n "$$(git diff --name-only internal/storage/sqlc/)" ]; then \
		echo "FAIL: sqlc-generate produced changes:"; \
		git diff --name-only internal/storage/sqlc/; \
		git checkout -- internal/storage/sqlc/; \
		exit 1; \
	fi
	@echo "OK: generated code is up to date"
```

**Step 2: Update .PHONY line**

Update the existing `.PHONY` at the top of the file to include new targets:

```makefile
.PHONY: sqlc-generate sqlc-vet migrate-up migrate-down migrate-status migrate-create sqlc-diff
```

**Step 3: Commit**

```bash
git add make/db.mk
git commit -m "build(make): add migration and sqlc-diff targets"
```

---

### Task 14: Update CI workflow

**Files:**
- Modify: `.github/workflows/ci.yml`

**Step 1: Add sqlc-diff check to the spec job**

In `.github/workflows/ci.yml`, add a step to the `spec` job after the existing `sqlc-vet` step:

```yaml
      - name: Check sqlc drift
        # Equivalent to: make sqlc-diff
        run: |
          make sqlc-generate
          if [ -n "$(git diff --name-only internal/storage/sqlc/)" ]; then
            echo "FAIL: sqlc-generate produced changes:"
            git diff --name-only internal/storage/sqlc/
            exit 1
          fi
```

**Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add sqlc-diff check to spec job"
```

---

### Task 15: Update build target

**Files:**
- Modify: `make/go.mk`

**Step 1: Add migrate binary to the build target**

The `build` target currently only builds the updater binary. Add the migrate binary:

```makefile
build: ## Build the application to bin/updater and bin/migrate
	$(GO_DOCKER) go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) ./cmd/$(APP_NAME)
	$(GO_DOCKER) go build -ldflags "-w -s" -o $(BIN_DIR)/migrate ./cmd/migrate
```

**Step 2: Commit**

```bash
git add make/go.mk
git commit -m "build(make): include migrate binary in build target"
```

---

### Task 16: Update documentation

**Files:**
- Modify: `docs/storage.md`
- Create: `docs/migrations.md`
- Modify: `mkdocs.yml` (add migrations page to nav)

**Step 1: Create docs/migrations.md**

Write a migrations guide covering:
- Overview: what goose is, how it's used
- The migrate binary: usage, flags, commands
- Directory structure: where migration files live
- Creating new migrations: naming convention, goose annotations
- Running migrations: Make targets, Docker, Kubernetes init container
- Development workflow: create migration -> run sqlc-generate -> run migrate -> test
- Diagram: migration flow (mermaid)

**Step 2: Update docs/storage.md**

Remove any references to the hand-rolled SQLite migrator. Add a note that migrations are managed by goose via the `migrate` binary, with a link to the migrations guide.

**Step 3: Add to mkdocs nav**

Add the migrations page to `mkdocs.yml` nav configuration.

**Step 4: Commit**

```bash
git add docs/migrations.md docs/storage.md mkdocs.yml
git commit -m "docs: add migration guide and update storage docs"
```

---

### Task 17: Final verification

**Step 1: Run make check**

```bash
make check
```

Expected: fmt-check, vet, and all tests pass.

**Step 2: Run integration tests**

```bash
make integration-test
```

Expected: PASS

**Step 3: Run sqlc verification**

```bash
make sqlc-vet
make sqlc-diff
```

Expected: Both pass.

**Step 4: Build Docker image**

```bash
make docker-build
```

Expected: Image builds with both `updater` and `migrate` binaries.

**Step 5: Test migrate binary in Docker**

```bash
docker run --rm --entrypoint /usr/local/bin/migrate <image> --dialect sqlite --dsn /tmp/test.db up
```

Expected: Migration applied, version 1.