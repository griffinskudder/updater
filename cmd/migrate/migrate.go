// Command migrate applies database migrations using goose.
//
// Usage:
//
//	migrate --dialect postgres --dsn "postgres://..." up
//	migrate --dialect sqlite --dsn "./data/updater.db" status
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strconv"
	"text/tabwriter"

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

// gooseDialect maps our dialect names to goose Dialect constants.
func gooseDialect(dialect string) goose.Dialect {
	if dialect == "sqlite" {
		return goose.DialectSQLite3
	}
	return goose.DialectPostgres
}

// driverName maps our dialect names to database/sql driver names.
func driverName(dialect string) string {
	if dialect == "postgres" {
		return "pgx"
	}
	return "sqlite"
}

// migrationFS returns the embedded filesystem for the given dialect,
// scoped to the dialect's subdirectory.
func migrationFS(dialect string) (fs.FS, error) {
	if dialect == "postgres" {
		return fs.Sub(migrations.PostgresFS, "postgres")
	}
	return fs.Sub(migrations.SQLiteFS, "sqlite")
}

func runMigration(cfg migrateConfig, cmd string) error {
	db, err := sql.Open(driverName(cfg.dialect), cfg.dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	fsys, err := migrationFS(cfg.dialect)
	if err != nil {
		return fmt.Errorf("migration filesystem: %w", err)
	}

	provider, err := goose.NewProvider(gooseDialect(cfg.dialect), db, fsys,
		goose.WithVerbose(cfg.verbose),
	)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	ctx := context.Background()

	switch cmd {
	case "up":
		_, err = provider.Up(ctx)
	case "up-to":
		_, err = provider.UpTo(ctx, cfg.version)
	case "down":
		_, err = provider.Down(ctx)
	case "down-to":
		_, err = provider.DownTo(ctx, cfg.version)
	case "status":
		return printStatus(ctx, provider)
	case "version":
		return printVersion(ctx, provider)
	case "redo":
		if _, err = provider.Down(ctx); err != nil {
			return fmt.Errorf("redo down: %w", err)
		}
		_, err = provider.UpByOne(ctx)
	case "reset":
		_, err = provider.DownTo(ctx, 0)
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
	return err
}

// printStatus prints the status of all migrations in a tabular format.
func printStatus(ctx context.Context, p *goose.Provider) error {
	results, err := p.Status(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "Applied At\tMigration")
	fmt.Fprintln(w, "=========\t=========")
	for _, r := range results {
		if r.State == goose.StateApplied {
			fmt.Fprintf(w, "%s\t%s\n", r.AppliedAt.Format("20060102150405"), r.Source.Path)
		} else {
			fmt.Fprintf(w, "Pending\t%s\n", r.Source.Path)
		}
	}
	return w.Flush()
}

// printVersion prints the current database schema version.
func printVersion(ctx context.Context, p *goose.Provider) error {
	ver, err := p.GetDBVersion(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("goose: version %d\n", ver)
	return nil
}
