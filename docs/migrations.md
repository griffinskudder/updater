# Migrations

Database schema changes are managed by [goose](https://github.com/pressly/goose), a zero-dependency Go migration tool. The project ships a `migrate` binary (`cmd/migrate`) that embeds all migration files and applies them via goose at the CLI.

## Overview

```mermaid
flowchart LR
    A[SQL migration files] -->|embed.FS| B[migrate binary]
    B -->|goose| C[Database]
    A -->|sqlc generate| D[Type-safe Go code]
    D --> E[Storage layer]
    E --> C
```

The migration files serve two purposes:

1. **Schema management** -- goose reads the `-- +goose Up` / `-- +goose Down` annotations to apply or roll back changes.
2. **Query generation** -- sqlc reads the same schema files (alongside query files) to generate type-safe Go code.

This means a single set of SQL files is the source of truth for both the live database schema and the generated query code.

## Directory Structure

```
internal/storage/migrations/
    migrations.go              # embed.FS declarations
    migrations_test.go         # Validates embedded files
    postgres/
        001_initial.sql        # First PostgreSQL migration
    sqlite/
        001_initial.sql        # First SQLite migration
```

Each dialect has its own subdirectory because PostgreSQL and SQLite use different types and syntax (e.g., `JSONB` vs `TEXT`, `TIMESTAMPTZ` vs ISO8601 strings, `GIN` indexes). The `migrations.go` file embeds both directories using `//go:embed` so the `migrate` binary needs no external files at runtime.

## CLI Usage

```
migrate --dialect <postgres|sqlite> --dsn <connection-string> [--verbose] <command> [VERSION]
```

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--dialect` | Yes | Database engine: `postgres` or `sqlite` |
| `--dsn` | Yes | Connection string (PostgreSQL URI or SQLite file path) |
| `--verbose`, `-v` | No | Enable verbose goose output |

### Commands

| Command | Description |
|---------|-------------|
| `up` | Apply all pending migrations |
| `up-to VERSION` | Migrate up to a specific version |
| `down` | Roll back the most recent migration |
| `down-to VERSION` | Roll back to a specific version |
| `status` | Show the status of all migrations |
| `version` | Print the current migration version |
| `redo` | Roll back and re-apply the latest migration |
| `reset` | Roll back all migrations |

### Examples

```bash
# Apply all pending migrations to a PostgreSQL database
migrate --dialect postgres --dsn "postgres://user:pass@localhost:5432/updater?sslmode=disable" up

# Check migration status on an SQLite database
migrate --dialect sqlite --dsn "./data/updater.db" status

# Roll back the latest migration with verbose output
migrate --dialect postgres --dsn "postgres://..." -v down
```

## Make Targets

The Makefile provides convenience targets that run the `migrate` binary inside Docker. The default dialect is `sqlite` with DSN `./data/updater.db`. Override with `DIALECT` and `DSN` variables.

| Target | Description |
|--------|-------------|
| `make migrate-up` | Apply pending migrations |
| `make migrate-down` | Roll back one migration |
| `make migrate-status` | Show migration status |
| `make migrate-create NAME=<name>` | Create a new migration file |

```bash
# SQLite (defaults)
make migrate-up

# PostgreSQL
make migrate-up DIALECT=postgres DSN="postgres://user:pass@localhost:5432/updater"

# Create a new migration
make migrate-create NAME=add_audit_log DIALECT=postgres
```

## Creating New Migrations

### Naming Convention

Migration files use a three-digit sequential prefix followed by a descriptive snake_case name:

```
NNN_description.sql
```

For example: `002_add_audit_log.sql`, `003_add_release_channels.sql`.

Use `make migrate-create` to generate the file with the correct sequence number and boilerplate.

### Goose Annotations

Every migration file must contain `-- +goose Up` and `-- +goose Down` annotations. Goose uses these markers to determine which SQL to run in each direction.

```sql
-- +goose Up
CREATE TABLE audit_log (
    id TEXT PRIMARY KEY,
    action TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
```

### Multi-statement Blocks

Triggers, functions, and other multi-statement constructs must be wrapped in `-- +goose StatementBegin` / `-- +goose StatementEnd` annotations. Without these, goose splits on semicolons and the statements fail.

```sql
-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS update_updated_at_column();
```

### Dialect-specific Files

Because PostgreSQL and SQLite use different SQL syntax, every migration must be created in both dialect directories. Use the `DIALECT` variable with `make migrate-create` and repeat for the other dialect:

```bash
make migrate-create NAME=add_audit_log DIALECT=postgres
make migrate-create NAME=add_audit_log DIALECT=sqlite
```

Then edit both files with the appropriate engine-specific SQL.

## Development Workflow

When making schema changes, follow this sequence:

```mermaid
flowchart TD
    A[Create migration files] --> B[Run sqlc-generate]
    B --> C[Update Go storage code]
    C --> D[Run migrate-up]
    D --> E[Run tests]
    E --> F{Tests pass?}
    F -->|Yes| G[Commit]
    F -->|No| H[Fix and repeat]
    H --> B
```

1. **Create migration files** -- `make migrate-create NAME=<name> DIALECT=postgres` and repeat for `sqlite`. Write the Up and Down SQL for both dialects.
2. **Regenerate sqlc code** -- `make sqlc-generate`. This picks up the new schema and regenerates the type-safe query code.
3. **Update Go code** -- Modify storage provider code and queries as needed for the new schema.
4. **Apply migrations** -- `make migrate-up` to verify the migration applies cleanly.
5. **Run tests** -- `make check` to verify everything passes.
6. **Commit** -- Commit the migration files, generated code, and Go changes together.

## Docker and Kubernetes

### Docker Image

The `migrate` binary is included in the production Docker image at `/usr/local/bin/migrate`. Run it as an entrypoint override or in a shell:

```bash
docker run --rm myregistry/updater:latest \
    /usr/local/bin/migrate \
    --dialect postgres \
    --dsn "postgres://user:pass@db:5432/updater?sslmode=disable" \
    up
```

### Kubernetes Init Container

A common pattern is to run migrations as an init container before the main application starts:

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      initContainers:
        - name: migrate
          image: myregistry/updater:latest
          command:
            - /usr/local/bin/migrate
            - --dialect
            - postgres
            - --dsn
            - $(DATABASE_DSN)
            - up
          env:
            - name: DATABASE_DSN
              valueFrom:
                secretKeyRef:
                  name: db-credentials
                  key: dsn
      containers:
        - name: updater
          image: myregistry/updater:latest
```

This ensures the database schema is up to date before the application accepts traffic.

## Relationship to sqlc

The migration files under `internal/storage/migrations/<dialect>/` define the schema. The query files under `internal/storage/queries/<dialect>/` define the queries. sqlc reads both to generate Go code:

```mermaid
flowchart TD
    subgraph migrations["internal/storage/migrations/"]
        M1[postgres/001_initial.sql]
        M2[sqlite/001_initial.sql]
    end

    subgraph sqlc_queries["internal/storage/queries/"]
        Q1[postgres/queries.sql]
        Q2[sqlite/queries.sql]
    end

    subgraph generated["Generated Go code"]
        G1[sqlc/postgres/*.go]
        G2[sqlc/sqlite/*.go]
    end

    M1 --> G1
    Q1 --> G1
    M2 --> G2
    Q2 --> G2
```

The sqlc configuration references the migration files as the schema source. When you add a new migration, run `make sqlc-generate` to regenerate the query code against the updated schema.
