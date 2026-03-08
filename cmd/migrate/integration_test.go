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
