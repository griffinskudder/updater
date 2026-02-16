package storage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
	"updater/internal/models"
	sqlcpg "updater/internal/storage/sqlc/postgres"

	"github.com/Masterminds/semver/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStorage implements the Storage interface using PostgreSQL with sqlc-generated queries.
type PostgresStorage struct {
	pool    *pgxpool.Pool
	queries *sqlcpg.Queries
}

// NewPostgresStorage creates a new PostgreSQL storage instance.
func NewPostgresStorage(config Config) (Storage, error) {
	if config.ConnectionString == "" {
		return nil, fmt.Errorf("connection string is required for PostgreSQL storage")
	}

	pool, err := pgxpool.New(context.Background(), config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresStorage{
		pool:    pool,
		queries: sqlcpg.New(pool),
	}, nil
}

// Applications returns all registered applications.
func (ps *PostgresStorage) Applications(ctx context.Context) ([]*models.Application, error) {
	rows, err := ps.queries.GetAllApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get applications: %w", err)
	}

	apps := make([]*models.Application, 0, len(rows))
	for _, row := range rows {
		app, err := pgAppToModel(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert application %s: %w", row.ID, err)
		}
		apps = append(apps, app)
	}

	return apps, nil
}

// GetApplication retrieves an application by its ID.
func (ps *PostgresStorage) GetApplication(ctx context.Context, appID string) (*models.Application, error) {
	row, err := ps.queries.GetApplicationByID(ctx, appID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("application %s not found", appID)
		}
		return nil, fmt.Errorf("failed to get application: %w", err)
	}

	return pgAppToModel(row)
}

// SaveApplication stores or updates an application (upsert pattern).
func (ps *PostgresStorage) SaveApplication(ctx context.Context, app *models.Application) error {
	_, err := ps.queries.GetApplicationByID(ctx, app.ID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("failed to check existing application: %w", err)
		}

		// Create new application
		params, err := modelToPgCreateApp(app)
		if err != nil {
			return fmt.Errorf("failed to convert application for create: %w", err)
		}
		if err := ps.queries.CreateApplication(ctx, params); err != nil {
			return fmt.Errorf("failed to create application: %w", err)
		}
		return nil
	}

	// Update existing application
	params, err := modelToPgUpdateApp(app)
	if err != nil {
		return fmt.Errorf("failed to convert application for update: %w", err)
	}
	if err := ps.queries.UpdateApplication(ctx, params); err != nil {
		return fmt.Errorf("failed to update application: %w", err)
	}
	return nil
}

// DeleteApplication removes an application by its ID.
func (ps *PostgresStorage) DeleteApplication(ctx context.Context, appID string) error {
	err := ps.queries.DeleteApplication(ctx, appID)
	if err != nil {
		return fmt.Errorf("failed to delete application %s: %w", appID, err)
	}
	return nil
}

// Releases returns all releases for a given application.
func (ps *PostgresStorage) Releases(ctx context.Context, appID string) ([]*models.Release, error) {
	rows, err := ps.queries.GetReleasesByAppID(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get releases: %w", err)
	}

	releases := make([]*models.Release, 0, len(rows))
	for _, row := range rows {
		release, err := pgReleaseToModel(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert release %s: %w", row.ID, err)
		}
		releases = append(releases, release)
	}

	return releases, nil
}

// GetRelease retrieves a specific release by application ID, version, platform, and architecture.
func (ps *PostgresStorage) GetRelease(ctx context.Context, appID, version, platform, arch string) (*models.Release, error) {
	row, err := ps.queries.GetRelease(ctx, sqlcpg.GetReleaseParams{
		ApplicationID: appID,
		Version:       version,
		Platform:      platform,
		Architecture:  arch,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("release not found: %s@%s-%s-%s", appID, version, platform, arch)
		}
		return nil, fmt.Errorf("failed to get release: %w", err)
	}

	return pgReleaseToModel(row)
}

// SaveRelease stores or updates a release (upsert pattern).
func (ps *PostgresStorage) SaveRelease(ctx context.Context, release *models.Release) error {
	_, err := ps.queries.GetRelease(ctx, sqlcpg.GetReleaseParams{
		ApplicationID: release.ApplicationID,
		Version:       release.Version,
		Platform:      release.Platform,
		Architecture:  release.Architecture,
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("failed to check existing release: %w", err)
		}

		// Create new release
		params, err := modelToPgCreateRelease(release)
		if err != nil {
			return fmt.Errorf("failed to convert release for create: %w", err)
		}
		if err := ps.queries.CreateRelease(ctx, params); err != nil {
			return fmt.Errorf("failed to create release: %w", err)
		}
		return nil
	}

	// Update existing release
	params, err := modelToPgUpdateRelease(release)
	if err != nil {
		return fmt.Errorf("failed to convert release for update: %w", err)
	}
	if err := ps.queries.UpdateRelease(ctx, params); err != nil {
		return fmt.Errorf("failed to update release: %w", err)
	}
	return nil
}

// DeleteRelease removes a release.
func (ps *PostgresStorage) DeleteRelease(ctx context.Context, appID, version, platform, arch string) error {
	// Verify release exists first
	_, err := ps.queries.GetRelease(ctx, sqlcpg.GetReleaseParams{
		ApplicationID: appID,
		Version:       version,
		Platform:      platform,
		Architecture:  arch,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("release not found: %s@%s-%s-%s", appID, version, platform, arch)
		}
		return fmt.Errorf("failed to check release: %w", err)
	}

	if err := ps.queries.DeleteRelease(ctx, sqlcpg.DeleteReleaseParams{
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
func (ps *PostgresStorage) GetLatestRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error) {
	rows, err := ps.queries.GetReleasesByPlatformArch(ctx, sqlcpg.GetReleasesByPlatformArchParams{
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

	return pgReleaseToModel(rows[0])
}

// GetReleasesAfterVersion returns all releases after a given version for a specific platform/arch.
func (ps *PostgresStorage) GetReleasesAfterVersion(ctx context.Context, appID, currentVersion, platform, arch string) ([]*models.Release, error) {
	currentVer, err := semver.NewVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}

	rows, err := ps.queries.GetReleasesByPlatformArch(ctx, sqlcpg.GetReleasesByPlatformArchParams{
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
			release, err := pgReleaseToModel(row)
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
func (ps *PostgresStorage) Ping(ctx context.Context) error {
	return ps.pool.Ping(ctx)
}

// Close closes the storage connection.
func (ps *PostgresStorage) Close() error {
	ps.pool.Close()
	return nil
}

// Conversion helpers

func pgAppToModel(row sqlcpg.Application) (*models.Application, error) {
	platforms, err := unmarshalPlatforms(row.Platforms)
	if err != nil {
		return nil, err
	}

	config, err := unmarshalConfig(row.Config)
	if err != nil {
		return nil, err
	}

	app := &models.Application{
		ID:          row.ID,
		Name:        row.Name,
		Description: pgTextToString(row.Description),
		Platforms:   platforms,
		Config:      config,
	}

	if row.CreatedAt.Valid {
		app.CreatedAt = row.CreatedAt.Time.Format(time.RFC3339)
	}
	if row.UpdatedAt.Valid {
		app.UpdatedAt = row.UpdatedAt.Time.Format(time.RFC3339)
	}

	return app, nil
}

func modelToPgCreateApp(app *models.Application) (sqlcpg.CreateApplicationParams, error) {
	platforms, err := marshalPlatforms(app.Platforms)
	if err != nil {
		return sqlcpg.CreateApplicationParams{}, err
	}

	config, err := marshalConfig(app.Config)
	if err != nil {
		return sqlcpg.CreateApplicationParams{}, err
	}

	now := time.Now()
	return sqlcpg.CreateApplicationParams{
		ID:          app.ID,
		Name:        app.Name,
		Description: stringToPgText(app.Description),
		Platforms:   platforms,
		Config:      config,
		CreatedAt:   timeToPgTimestamptz(now),
		UpdatedAt:   timeToPgTimestamptz(now),
	}, nil
}

func modelToPgUpdateApp(app *models.Application) (sqlcpg.UpdateApplicationParams, error) {
	platforms, err := marshalPlatforms(app.Platforms)
	if err != nil {
		return sqlcpg.UpdateApplicationParams{}, err
	}

	config, err := marshalConfig(app.Config)
	if err != nil {
		return sqlcpg.UpdateApplicationParams{}, err
	}

	return sqlcpg.UpdateApplicationParams{
		ID:          app.ID,
		Name:        app.Name,
		Description: stringToPgText(app.Description),
		Platforms:   platforms,
		Config:      config,
		UpdatedAt:   timeToPgTimestamptz(time.Now()),
	}, nil
}

func pgReleaseToModel(row sqlcpg.Release) (*models.Release, error) {
	metadata, err := unmarshalMetadata(row.Metadata)
	if err != nil {
		return nil, err
	}

	release := &models.Release{
		ID:             row.ID,
		ApplicationID:  row.ApplicationID,
		Version:        row.Version,
		Platform:       row.Platform,
		Architecture:   row.Architecture,
		DownloadURL:    row.DownloadUrl,
		Checksum:       row.Checksum,
		ChecksumType:   row.ChecksumType,
		FileSize:       row.FileSize,
		ReleaseNotes:   pgTextToString(row.ReleaseNotes),
		Required:       row.Required,
		MinimumVersion: pgTextToString(row.MinimumVersion),
		Metadata:       metadata,
	}

	if row.ReleaseDate.Valid {
		release.ReleaseDate = row.ReleaseDate.Time
	}
	if row.CreatedAt.Valid {
		release.CreatedAt = row.CreatedAt.Time
		release.UpdatedAt = row.CreatedAt.Time
	}

	return release, nil
}

func modelToPgCreateRelease(r *models.Release) (sqlcpg.CreateReleaseParams, error) {
	metadata, err := marshalMetadata(r.Metadata)
	if err != nil {
		return sqlcpg.CreateReleaseParams{}, err
	}

	return sqlcpg.CreateReleaseParams{
		ID:             r.ID,
		ApplicationID:  r.ApplicationID,
		Version:        r.Version,
		Platform:       r.Platform,
		Architecture:   r.Architecture,
		DownloadUrl:    r.DownloadURL,
		Checksum:       r.Checksum,
		ChecksumType:   r.ChecksumType,
		FileSize:       r.FileSize,
		ReleaseNotes:   stringToPgText(r.ReleaseNotes),
		ReleaseDate:    timeToPgTimestamptz(r.ReleaseDate),
		Required:       r.Required,
		MinimumVersion: stringToPgText(r.MinimumVersion),
		Metadata:       metadata,
		CreatedAt:      timeToPgTimestamptz(r.CreatedAt),
	}, nil
}

func modelToPgUpdateRelease(r *models.Release) (sqlcpg.UpdateReleaseParams, error) {
	metadata, err := marshalMetadata(r.Metadata)
	if err != nil {
		return sqlcpg.UpdateReleaseParams{}, err
	}

	return sqlcpg.UpdateReleaseParams{
		ID:             r.ID,
		DownloadUrl:    r.DownloadURL,
		Checksum:       r.Checksum,
		ChecksumType:   r.ChecksumType,
		FileSize:       r.FileSize,
		ReleaseNotes:   stringToPgText(r.ReleaseNotes),
		ReleaseDate:    timeToPgTimestamptz(r.ReleaseDate),
		Required:       r.Required,
		MinimumVersion: stringToPgText(r.MinimumVersion),
		Metadata:       metadata,
	}, nil
}

// pgtype helpers

func pgTextToString(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func stringToPgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func timeToPgTimestamptz(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// CreateAPIKey is not implemented for PostgresStorage.
func (ps *PostgresStorage) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	return ErrNotFound
}

// GetAPIKeyByHash is not implemented for PostgresStorage.
func (ps *PostgresStorage) GetAPIKeyByHash(ctx context.Context, hash string) (*models.APIKey, error) {
	return nil, ErrNotFound
}

// ListAPIKeys is not implemented for PostgresStorage.
func (ps *PostgresStorage) ListAPIKeys(ctx context.Context) ([]*models.APIKey, error) {
	return nil, ErrNotFound
}

// UpdateAPIKey updates an existing API key's mutable fields.
func (ps *PostgresStorage) UpdateAPIKey(ctx context.Context, key *models.APIKey) error {
	permsJSON, err := marshalPermissions(key.Permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	rows, err := ps.queries.UpdateAPIKey(ctx, sqlcpg.UpdateAPIKeyParams{
		ID:          key.ID,
		Name:        key.Name,
		Permissions: []byte(permsJSON),
		Enabled:     key.Enabled,
		UpdatedAt:   timeToPgTimestamptz(time.Now().UTC()),
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
func (ps *PostgresStorage) DeleteAPIKey(ctx context.Context, id string) error {
	rows, err := ps.queries.DeleteAPIKey(ctx, id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
