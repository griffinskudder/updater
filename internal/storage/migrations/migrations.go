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
