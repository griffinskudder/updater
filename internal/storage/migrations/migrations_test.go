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
