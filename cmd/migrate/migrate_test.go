package main

import (
	"testing"

	"github.com/pressly/goose/v3"
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

func TestParseArgs_VerboseLong(t *testing.T) {
	args := []string{"--dialect", "postgres", "--dsn", "test", "--verbose", "up"}
	cfg, _, err := parseArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.verbose {
		t.Error("expected verbose to be true with --verbose flag")
	}
}

func TestParseArgs_VerboseShort(t *testing.T) {
	args := []string{"--dialect", "postgres", "--dsn", "test", "-v", "up"}
	cfg, _, err := parseArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.verbose {
		t.Error("expected verbose to be true with -v flag")
	}
}

func TestParseArgs_VerboseDefaultFalse(t *testing.T) {
	args := []string{"--dialect", "postgres", "--dsn", "test", "up"}
	cfg, _, err := parseArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.verbose {
		t.Error("expected verbose to default to false")
	}
}

func TestGooseDialect_Postgres(t *testing.T) {
	d := gooseDialect("postgres")
	if d != goose.DialectPostgres {
		t.Errorf("expected DialectPostgres, got %s", d)
	}
}

func TestGooseDialect_SQLite(t *testing.T) {
	d := gooseDialect("sqlite")
	if d != goose.DialectSQLite3 {
		t.Errorf("expected DialectSQLite3, got %s", d)
	}
}
