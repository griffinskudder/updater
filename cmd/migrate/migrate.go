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

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"updater/internal/storage/migrations"
)

// validCommands lists the goose commands the migrate binary supports.
var validCommands = map[string]bool{
	"up":      true,
	"up-to":   true,
	"down":    true,
	"down-to": true,
	"status":  true,
	"version": true,
	"redo":    true,
	"reset":   true,
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
	flagSet := flag.NewFlagSet("migrate", flag.ContinueOnError)
	dialect := flagSet.String("dialect", "", "Database dialect: postgres or sqlite")
	dsn := flagSet.String("dsn", "", "Database connection string")
	verbose := flagSet.Bool("verbose", false, "Enable verbose output")
	flagSet.BoolVar(verbose, "v", false, "Enable verbose output (shorthand)")

	if err := flagSet.Parse(args); err != nil {
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

	remaining := flagSet.Args()
	if len(remaining) == 0 {
		return migrateConfig{}, "", fmt.Errorf("command is required (up, down, status, version, redo, reset, up-to, down-to)")
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
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}
