package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
	"updater/internal/models"
	sqlcite "updater/internal/storage/sqlc/sqlite"

	"github.com/Masterminds/semver/v3"
	_ "modernc.org/sqlite"
)

//go:embed sqlc/schema/sqlite
var sqliteMigrationsFS embed.FS

// SQLiteStorage implements the Storage interface using SQLite with sqlc-generated queries.
type SQLiteStorage struct {
	db      *sql.DB
	queries *sqlcite.Queries
}

// NewSQLiteStorage creates a new SQLite storage instance.
// It automatically creates tables using the embedded schema if they do not exist.
func NewSQLiteStorage(config Config) (Storage, error) {
	if config.ConnectionString == "" {
		return nil, fmt.Errorf("connection string is required for SQLite storage")
	}

	db, err := sql.Open("sqlite", config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Limit to 1 open connection for SQLite to prevent concurrency issues.
	// SQLite only supports one writer at a time, and :memory: databases are
	// per-connection, so multiple connections would see different databases.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := runSQLiteMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &SQLiteStorage{
		db:      db,
		queries: sqlcite.New(db),
	}, nil
}

// Applications returns all registered applications.
func (ss *SQLiteStorage) Applications(ctx context.Context) ([]*models.Application, error) {
	rows, err := ss.queries.GetAllApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get applications: %w", err)
	}

	apps := make([]*models.Application, 0, len(rows))
	for _, row := range rows {
		app, err := sqliteAppToModel(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert application %s: %w", row.ID, err)
		}
		apps = append(apps, app)
	}

	return apps, nil
}

// GetApplication retrieves an application by its ID.
func (ss *SQLiteStorage) GetApplication(ctx context.Context, appID string) (*models.Application, error) {
	row, err := ss.queries.GetApplicationByID(ctx, appID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("application %s not found", appID)
		}
		return nil, fmt.Errorf("failed to get application: %w", err)
	}

	return sqliteAppToModel(row)
}

// SaveApplication stores or updates an application (upsert pattern).
func (ss *SQLiteStorage) SaveApplication(ctx context.Context, app *models.Application) error {
	_, err := ss.queries.GetApplicationByID(ctx, app.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to check existing application: %w", err)
		}

		// Create new application
		params, err := modelToSqliteCreateApp(app)
		if err != nil {
			return fmt.Errorf("failed to convert application for create: %w", err)
		}
		if err := ss.queries.CreateApplication(ctx, params); err != nil {
			return fmt.Errorf("failed to create application: %w", err)
		}
		return nil
	}

	// Update existing application
	params, err := modelToSqliteUpdateApp(app)
	if err != nil {
		return fmt.Errorf("failed to convert application for update: %w", err)
	}
	if err := ss.queries.UpdateApplication(ctx, params); err != nil {
		return fmt.Errorf("failed to update application: %w", err)
	}
	return nil
}

// DeleteApplication removes an application by its ID.
func (ss *SQLiteStorage) DeleteApplication(ctx context.Context, appID string) error {
	err := ss.queries.DeleteApplication(ctx, appID)
	if err != nil {
		return fmt.Errorf("failed to delete application %s: %w", appID, err)
	}
	return nil
}

// Releases returns all releases for a given application.
func (ss *SQLiteStorage) Releases(ctx context.Context, appID string) ([]*models.Release, error) {
	rows, err := ss.queries.GetReleasesByAppID(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get releases: %w", err)
	}

	releases := make([]*models.Release, 0, len(rows))
	for _, row := range rows {
		release, err := sqliteReleaseToModel(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert release %s: %w", row.ID, err)
		}
		releases = append(releases, release)
	}

	return releases, nil
}

// GetRelease retrieves a specific release by application ID, version, platform, and architecture.
func (ss *SQLiteStorage) GetRelease(ctx context.Context, appID, version, platform, arch string) (*models.Release, error) {
	row, err := ss.queries.GetRelease(ctx, sqlcite.GetReleaseParams{
		ApplicationID: appID,
		Version:       version,
		Platform:      platform,
		Architecture:  arch,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("release not found: %s@%s-%s-%s", appID, version, platform, arch)
		}
		return nil, fmt.Errorf("failed to get release: %w", err)
	}

	return sqliteReleaseToModel(row)
}

// SaveRelease stores or updates a release (upsert pattern).
func (ss *SQLiteStorage) SaveRelease(ctx context.Context, release *models.Release) error {
	_, err := ss.queries.GetRelease(ctx, sqlcite.GetReleaseParams{
		ApplicationID: release.ApplicationID,
		Version:       release.Version,
		Platform:      release.Platform,
		Architecture:  release.Architecture,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to check existing release: %w", err)
		}

		// Create new release
		params, err := modelToSqliteCreateRelease(release)
		if err != nil {
			return fmt.Errorf("failed to convert release for create: %w", err)
		}
		if err := ss.queries.CreateRelease(ctx, params); err != nil {
			return fmt.Errorf("failed to create release: %w", err)
		}
		return nil
	}

	// Update existing release
	params, err := modelToSqliteUpdateRelease(release)
	if err != nil {
		return fmt.Errorf("failed to convert release for update: %w", err)
	}
	if err := ss.queries.UpdateRelease(ctx, params); err != nil {
		return fmt.Errorf("failed to update release: %w", err)
	}
	return nil
}

// DeleteRelease removes a release.
func (ss *SQLiteStorage) DeleteRelease(ctx context.Context, appID, version, platform, arch string) error {
	// Verify release exists first
	_, err := ss.queries.GetRelease(ctx, sqlcite.GetReleaseParams{
		ApplicationID: appID,
		Version:       version,
		Platform:      platform,
		Architecture:  arch,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("release not found: %s@%s-%s-%s", appID, version, platform, arch)
		}
		return fmt.Errorf("failed to check release: %w", err)
	}

	if err := ss.queries.DeleteRelease(ctx, sqlcite.DeleteReleaseParams{
		ApplicationID: appID,
		Version:       version,
		Platform:      platform,
		Architecture:  arch,
	}); err != nil {
		return fmt.Errorf("failed to delete release: %w", err)
	}
	return nil
}

// GetLatestRelease returns the latest release for a given application, platform, and architecture.
func (ss *SQLiteStorage) GetLatestRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error) {
	rows, err := ss.queries.GetReleasesByPlatformArch(ctx, sqlcite.GetReleasesByPlatformArchParams{
		ApplicationID: appID,
		Platform:      platform,
		Architecture:  arch,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get releases: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no releases found for %s on %s-%s", appID, platform, arch)
	}

	// Sort by semantic version (latest first)
	sort.Slice(rows, func(i, j int) bool {
		vi, ei := semver.NewVersion(rows[i].Version)
		vj, ej := semver.NewVersion(rows[j].Version)
		if ei == nil && ej == nil {
			return vi.GreaterThan(vj)
		}
		return rows[i].Version > rows[j].Version
	})

	return sqliteReleaseToModel(rows[0])
}

// GetReleasesAfterVersion returns all releases after a given version for a specific platform/arch.
func (ss *SQLiteStorage) GetReleasesAfterVersion(ctx context.Context, appID, currentVersion, platform, arch string) ([]*models.Release, error) {
	currentVer, err := semver.NewVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}

	rows, err := ss.queries.GetReleasesByPlatformArch(ctx, sqlcite.GetReleasesByPlatformArchParams{
		ApplicationID: appID,
		Platform:      platform,
		Architecture:  arch,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get releases: %w", err)
	}

	var newerReleases []*models.Release
	for _, row := range rows {
		releaseVer, err := semver.NewVersion(row.Version)
		if err != nil {
			continue
		}
		if releaseVer.GreaterThan(currentVer) {
			release, err := sqliteReleaseToModel(row)
			if err != nil {
				continue
			}
			newerReleases = append(newerReleases, release)
		}
	}

	// Sort by version (latest first)
	sort.Slice(newerReleases, func(i, j int) bool {
		vi, _ := semver.NewVersion(newerReleases[i].Version)
		vj, _ := semver.NewVersion(newerReleases[j].Version)
		return vi.GreaterThan(vj)
	})

	if newerReleases == nil {
		newerReleases = []*models.Release{}
	}

	return newerReleases, nil
}

// Ping verifies the storage backend is reachable and operational.
func (ss *SQLiteStorage) Ping(ctx context.Context) error {
	return ss.db.PingContext(ctx)
}

// Close closes the storage connection.
func (ss *SQLiteStorage) Close() error {
	return ss.db.Close()
}

// Conversion helpers

func sqliteAppToModel(row sqlcite.Application) (*models.Application, error) {
	platforms, err := unmarshalPlatformsFromString(row.Platforms)
	if err != nil {
		return nil, err
	}

	config, err := unmarshalConfigFromString(row.Config)
	if err != nil {
		return nil, err
	}

	return &models.Application{
		ID:          row.ID,
		Name:        row.Name,
		Description: nullStringToString(row.Description),
		Platforms:   platforms,
		Config:      config,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

func modelToSqliteCreateApp(app *models.Application) (sqlcite.CreateApplicationParams, error) {
	platforms, err := marshalPlatforms(app.Platforms)
	if err != nil {
		return sqlcite.CreateApplicationParams{}, err
	}

	config, err := marshalConfig(app.Config)
	if err != nil {
		return sqlcite.CreateApplicationParams{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	return sqlcite.CreateApplicationParams{
		ID:          app.ID,
		Name:        app.Name,
		Description: stringToNullString(app.Description),
		Platforms:   string(platforms),
		Config:      string(config),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func modelToSqliteUpdateApp(app *models.Application) (sqlcite.UpdateApplicationParams, error) {
	platforms, err := marshalPlatforms(app.Platforms)
	if err != nil {
		return sqlcite.UpdateApplicationParams{}, err
	}

	config, err := marshalConfig(app.Config)
	if err != nil {
		return sqlcite.UpdateApplicationParams{}, err
	}

	return sqlcite.UpdateApplicationParams{
		ID:          app.ID,
		Name:        app.Name,
		Description: stringToNullString(app.Description),
		Platforms:   string(platforms),
		Config:      string(config),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func sqliteReleaseToModel(row sqlcite.Release) (*models.Release, error) {
	metadata, err := unmarshalMetadataFromString(nullStringToString(row.Metadata))
	if err != nil {
		return nil, err
	}

	releaseDate, _ := time.Parse(time.RFC3339, row.ReleaseDate)
	createdAt, _ := time.Parse(time.RFC3339, row.CreatedAt)

	return &models.Release{
		ID:             row.ID,
		ApplicationID:  row.ApplicationID,
		Version:        row.Version,
		Platform:       row.Platform,
		Architecture:   row.Architecture,
		DownloadURL:    row.DownloadUrl,
		Checksum:       row.Checksum,
		ChecksumType:   row.ChecksumType,
		FileSize:       row.FileSize,
		ReleaseNotes:   nullStringToString(row.ReleaseNotes),
		ReleaseDate:    releaseDate,
		Required:       row.Required,
		MinimumVersion: nullStringToString(row.MinimumVersion),
		Metadata:       metadata,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}, nil
}

func modelToSqliteCreateRelease(r *models.Release) (sqlcite.CreateReleaseParams, error) {
	metadata, err := marshalMetadata(r.Metadata)
	if err != nil {
		return sqlcite.CreateReleaseParams{}, err
	}

	return sqlcite.CreateReleaseParams{
		ID:             r.ID,
		ApplicationID:  r.ApplicationID,
		Version:        r.Version,
		Platform:       r.Platform,
		Architecture:   r.Architecture,
		DownloadUrl:    r.DownloadURL,
		Checksum:       r.Checksum,
		ChecksumType:   r.ChecksumType,
		FileSize:       r.FileSize,
		ReleaseNotes:   stringToNullString(r.ReleaseNotes),
		ReleaseDate:    r.ReleaseDate.UTC().Format(time.RFC3339),
		Required:       r.Required,
		MinimumVersion: stringToNullString(r.MinimumVersion),
		Metadata:       stringToNullString(string(metadata)),
		CreatedAt:      r.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func modelToSqliteUpdateRelease(r *models.Release) (sqlcite.UpdateReleaseParams, error) {
	metadata, err := marshalMetadata(r.Metadata)
	if err != nil {
		return sqlcite.UpdateReleaseParams{}, err
	}

	return sqlcite.UpdateReleaseParams{
		ID:             r.ID,
		DownloadUrl:    r.DownloadURL,
		Checksum:       r.Checksum,
		ChecksumType:   r.ChecksumType,
		FileSize:       r.FileSize,
		ReleaseNotes:   stringToNullString(r.ReleaseNotes),
		ReleaseDate:    r.ReleaseDate.UTC().Format(time.RFC3339),
		Required:       r.Required,
		MinimumVersion: stringToNullString(r.MinimumVersion),
		Metadata:       stringToNullString(string(metadata)),
	}, nil
}

// sql.NullString helpers

func nullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func stringToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// sqliteAPIKeyToModel converts a sqlcite.ApiKey row to a *models.APIKey.
func sqliteAPIKeyToModel(row sqlcite.ApiKey) (*models.APIKey, error) {
	perms, err := unmarshalPermissions(row.Permissions)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal permissions for key %s: %w", row.ID, err)
	}

	createdAt, _ := time.Parse(time.RFC3339, row.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, row.UpdatedAt)

	return &models.APIKey{
		ID:          row.ID,
		Name:        row.Name,
		KeyHash:     row.KeyHash,
		Prefix:      row.Prefix,
		Permissions: perms,
		Enabled:     row.Enabled != 0,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// CreateAPIKey persists a new API key.
func (ss *SQLiteStorage) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	perms, err := marshalPermissions(key.Permissions)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	var enabled int64
	if key.Enabled {
		enabled = 1
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := ss.queries.CreateAPIKey(ctx, sqlcite.CreateAPIKeyParams{
		ID:          key.ID,
		Name:        key.Name,
		KeyHash:     key.KeyHash,
		Prefix:      key.Prefix,
		Permissions: perms,
		Enabled:     enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		return fmt.Errorf("failed to create api key: %w", err)
	}
	return nil
}

// GetAPIKeyByHash retrieves an API key by its SHA-256 hash.
func (ss *SQLiteStorage) GetAPIKeyByHash(ctx context.Context, hash string) (*models.APIKey, error) {
	row, err := ss.queries.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get api key: %w", err)
	}
	return sqliteAPIKeyToModel(row)
}

// ListAPIKeys returns all stored API keys.
func (ss *SQLiteStorage) ListAPIKeys(ctx context.Context) ([]*models.APIKey, error) {
	rows, err := ss.queries.ListAPIKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list api keys: %w", err)
	}

	keys := make([]*models.APIKey, 0, len(rows))
	for _, row := range rows {
		key, err := sqliteAPIKeyToModel(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert api key %s: %w", row.ID, err)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// UpdateAPIKey updates an existing API key's mutable fields.
func (ss *SQLiteStorage) UpdateAPIKey(ctx context.Context, key *models.APIKey) error {
	perms, err := marshalPermissions(key.Permissions)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	var enabled int64
	if key.Enabled {
		enabled = 1
	}

	rows, err := ss.queries.UpdateAPIKey(ctx, sqlcite.UpdateAPIKeyParams{
		Name:        key.Name,
		Permissions: perms,
		Enabled:     enabled,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		ID:          key.ID,
	})
	if err != nil {
		return fmt.Errorf("update api key: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAPIKey removes an API key by its ID.
func (ss *SQLiteStorage) DeleteAPIKey(ctx context.Context, id string) error {
	rows, err := ss.queries.DeleteAPIKey(ctx, id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// runSQLiteMigrations applies any unapplied migration files from the embedded
// sqlc/schema/sqlite directory. Applied migrations are tracked in a
// schema_migrations table so each file is executed at most once.
func runSQLiteMigrations(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name       TEXT NOT NULL PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(sqliteMigrationsFS, "sqlc/schema/sqlite")
	if err != nil {
		return fmt.Errorf("read migration dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE name = ?", name).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if count > 0 {
			continue
		}

		data, err := fs.ReadFile(sqliteMigrationsFS, "sqlc/schema/sqlite/"+name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.Exec(string(data)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (name) VALUES (?)", name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}
