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
func NewPostgresStorage(dsn string) (Storage, error) {
	if dsn == "" {
		return nil, fmt.Errorf("connection string is required for PostgreSQL storage")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
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
	params, err := modelToPgUpsertApp(app)
	if err != nil {
		return fmt.Errorf("failed to convert application for upsert: %w", err)
	}
	if err := ps.queries.UpsertApplication(ctx, params); err != nil {
		return fmt.Errorf("failed to upsert application: %w", err)
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
	params, err := modelToPgUpsertRelease(release)
	if err != nil {
		return fmt.Errorf("failed to convert release for upsert: %w", err)
	}
	if err := ps.queries.UpsertRelease(ctx, params); err != nil {
		return fmt.Errorf("failed to upsert release: %w", err)
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

func modelToPgUpsertApp(app *models.Application) (sqlcpg.UpsertApplicationParams, error) {
	platforms, err := marshalPlatforms(app.Platforms)
	if err != nil {
		return sqlcpg.UpsertApplicationParams{}, err
	}

	config, err := marshalConfig(app.Config)
	if err != nil {
		return sqlcpg.UpsertApplicationParams{}, err
	}

	now := time.Now()
	return sqlcpg.UpsertApplicationParams{
		ID:          app.ID,
		Name:        app.Name,
		Description: stringToPgText(app.Description),
		Platforms:   platforms,
		Config:      config,
		CreatedAt:   timeToPgTimestamptz(now),
		UpdatedAt:   timeToPgTimestamptz(now),
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

func modelToPgUpsertRelease(r *models.Release) (sqlcpg.UpsertReleaseParams, error) {
	metadata, err := marshalMetadata(r.Metadata)
	if err != nil {
		return sqlcpg.UpsertReleaseParams{}, err
	}

	major, minor, patch, pre := parseSemverParts(r.Version)

	return sqlcpg.UpsertReleaseParams{
		ID:                r.ID,
		ApplicationID:     r.ApplicationID,
		Version:           r.Version,
		Platform:          r.Platform,
		Architecture:      r.Architecture,
		DownloadUrl:       r.DownloadURL,
		Checksum:          r.Checksum,
		ChecksumType:      r.ChecksumType,
		FileSize:          r.FileSize,
		ReleaseNotes:      stringToPgText(r.ReleaseNotes),
		ReleaseDate:       timeToPgTimestamptz(r.ReleaseDate),
		Required:          r.Required,
		MinimumVersion:    stringToPgText(r.MinimumVersion),
		Metadata:          metadata,
		CreatedAt:         timeToPgTimestamptz(r.CreatedAt),
		VersionMajor:      major,
		VersionMinor:      minor,
		VersionPatch:      patch,
		VersionPreRelease: pgtype.Text{String: pre, Valid: pre != ""},
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

// postgresAPIKeyToModel converts a sqlcpg.ApiKey row to a *models.APIKey.
func postgresAPIKeyToModel(row sqlcpg.ApiKey) (*models.APIKey, error) {
	perms, err := unmarshalPermissions(string(row.Permissions))
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal permissions for key %s: %w", row.ID, err)
	}

	key := &models.APIKey{
		ID:          row.ID,
		Name:        row.Name,
		KeyHash:     row.KeyHash,
		Prefix:      row.Prefix,
		Permissions: perms,
		Enabled:     row.Enabled,
	}

	if row.CreatedAt.Valid {
		key.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		key.UpdatedAt = row.UpdatedAt.Time
	}

	return key, nil
}

// CreateAPIKey persists a new API key.
func (ps *PostgresStorage) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	permsJSON, err := marshalPermissions(key.Permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	if err := ps.queries.CreateAPIKey(ctx, sqlcpg.CreateAPIKeyParams{
		ID:          key.ID,
		Name:        key.Name,
		KeyHash:     key.KeyHash,
		Prefix:      key.Prefix,
		Permissions: []byte(permsJSON),
		Enabled:     key.Enabled,
		CreatedAt:   timeToPgTimestamptz(key.CreatedAt),
		UpdatedAt:   timeToPgTimestamptz(key.UpdatedAt),
	}); err != nil {
		return fmt.Errorf("failed to create api key: %w", err)
	}
	return nil
}

// GetAPIKeyByHash retrieves an API key by its SHA-256 hash.
func (ps *PostgresStorage) GetAPIKeyByHash(ctx context.Context, hash string) (*models.APIKey, error) {
	row, err := ps.queries.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get api key: %w", err)
	}
	return postgresAPIKeyToModel(row)
}

// ListAPIKeys returns all stored API keys.
func (ps *PostgresStorage) ListAPIKeys(ctx context.Context) ([]*models.APIKey, error) {
	rows, err := ps.queries.ListAPIKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list api keys: %w", err)
	}

	keys := make([]*models.APIKey, 0, len(rows))
	for _, row := range rows {
		key, err := postgresAPIKeyToModel(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert api key %s: %w", row.ID, err)
		}
		keys = append(keys, key)
	}
	return keys, nil
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

// ListApplicationsPaged returns a page of applications sorted by created_at DESC, id DESC
// and the total count. cursor, when non-nil, positions the query after the given item.
func (ps *PostgresStorage) ListApplicationsPaged(ctx context.Context, limit int, cursor *models.ApplicationCursor) ([]*models.Application, int, error) {
	args := []interface{}{}
	keysetWhere := ""
	if cursor != nil {
		args = append(args,
			cursor.CreatedAt.UTC().Format(time.RFC3339),
			cursor.ID,
		)
		keysetWhere = fmt.Sprintf(`WHERE (created_at < $%d::timestamptz OR (created_at = $%d::timestamptz AND id < $%d))`,
			len(args)-1, len(args)-1, len(args))
	}
	args = append(args, int64(limit))

	query := fmt.Sprintf(`
		SELECT id, name, description, platforms, config, created_at, updated_at, total_count
		FROM (
		    SELECT id, name, description, platforms, config, created_at, updated_at,
		           COUNT(*) OVER() AS total_count
		    FROM applications
		) AS counted
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d`,
		keysetWhere, len(args))

	pgxRows, err := ps.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list applications: %w", err)
	}
	defer pgxRows.Close()

	total := 0
	apps := make([]*models.Application, 0)
	for pgxRows.Next() {
		var (
			id, name             string
			description          pgtype.Text
			platforms, config    []byte
			createdAt, updatedAt pgtype.Timestamptz
			totalCount           int64
		)
		if err := pgxRows.Scan(&id, &name, &description, &platforms, &config, &createdAt, &updatedAt, &totalCount); err != nil {
			return nil, 0, fmt.Errorf("failed to scan application: %w", err)
		}
		if total == 0 {
			total = int(totalCount)
		}
		app, err := pgAppToModel(sqlcpg.Application{
			ID:          id,
			Name:        name,
			Description: description,
			Platforms:   platforms,
			Config:      config,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("failed to convert application %s: %w", id, err)
		}
		apps = append(apps, app)
	}
	if err := pgxRows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate applications: %w", err)
	}
	return apps, total, nil
}

// pgReleaseListSortCols maps sortBy values to safe SQL ORDER BY fragments.
// Version sort uses the split numeric columns for correct semver ordering.
// Using an allowlist prevents SQL injection from untrusted sortBy values.
var pgReleaseListSortCols = map[string]string{
	"release_date": "release_date",
	"version":      "version_major DESC, version_minor DESC, version_patch DESC, (version_pre_release IS NULL) DESC, version_pre_release",
	"platform":     "platform",
	"architecture": "architecture",
	"created_at":   "created_at",
}

// ListReleasesPaged returns a filtered, sorted page of releases and the total count.
func (ps *PostgresStorage) ListReleasesPaged(ctx context.Context, appID string, filters models.ReleaseFilters, sortBy, sortOrder string, limit int, cursor *models.ReleaseCursor) ([]*models.Release, int, error) {
	col, ok := pgReleaseListSortCols[sortBy]
	if !ok {
		col = pgReleaseListSortCols["release_date"]
	}

	// Version sort has direction embedded; other columns get an explicit direction suffix.
	orderClause := col
	if sortBy != "version" {
		if sortOrder == "asc" {
			orderClause += " ASC"
		} else {
			orderClause += " DESC"
		}
	}

	args := []interface{}{appID}
	businessWhere := "WHERE application_id = $1"

	if filters.Architecture != "" {
		args = append(args, filters.Architecture)
		businessWhere += fmt.Sprintf(" AND architecture = $%d", len(args))
	}
	if filters.Version != "" {
		args = append(args, filters.Version)
		businessWhere += fmt.Sprintf(" AND version = $%d", len(args))
	}
	if filters.Required != nil {
		args = append(args, *filters.Required)
		businessWhere += fmt.Sprintf(" AND required = $%d", len(args))
	}
	if len(filters.Platforms) > 0 {
		args = append(args, filters.Platforms)
		businessWhere += fmt.Sprintf(" AND platform = ANY($%d::text[])", len(args))
	}

	// Keyset cursor condition (applied to outer query only).
	keysetWhere := ""
	if cursor != nil {
		n := len(args)
		switch sortBy {
		case "version":
			// Version is always DESC regardless of sortOrder.
			// Stable releases (no prerelease) sort higher than prereleases.
			isStable := 0
			if cursor.VersionIsStable {
				isStable = 1
			}
			args = append(args,
				cursor.VersionMajor,
				cursor.VersionMinor,
				cursor.VersionPatch,
				isStable,
				cursor.VersionPreRelease,
				cursor.ID,
			)
			keysetWhere = fmt.Sprintf(`WHERE (
  version_major < $%d
  OR (version_major = $%d AND version_minor < $%d)
  OR (version_major = $%d AND version_minor = $%d AND version_patch < $%d)
  OR (version_major = $%d AND version_minor = $%d AND version_patch = $%d AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END < $%d)
  OR (version_major = $%d AND version_minor = $%d AND version_patch = $%d AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END = $%d AND COALESCE(version_pre_release, '') > $%d)
  OR (version_major = $%d AND version_minor = $%d AND version_patch = $%d AND CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END = $%d AND COALESCE(version_pre_release, '') = $%d AND id < $%d)
)`,
				n+1,
				n+1, n+2,
				n+1, n+2, n+3,
				n+1, n+2, n+3, n+4,
				n+1, n+2, n+3, n+4, n+5,
				n+1, n+2, n+3, n+4, n+5, n+6,
			)
		case "platform":
			args = append(args, cursor.Platform, cursor.ID)
			if sortOrder == "desc" {
				keysetWhere = fmt.Sprintf("WHERE ((platform < $%d) OR (platform = $%d AND id < $%d))", n+1, n+1, n+2)
			} else {
				keysetWhere = fmt.Sprintf("WHERE ((platform > $%d) OR (platform = $%d AND id > $%d))", n+1, n+1, n+2)
			}
		case "architecture":
			args = append(args, cursor.Architecture, cursor.ID)
			if sortOrder == "desc" {
				keysetWhere = fmt.Sprintf("WHERE ((architecture < $%d) OR (architecture = $%d AND id < $%d))", n+1, n+1, n+2)
			} else {
				keysetWhere = fmt.Sprintf("WHERE ((architecture > $%d) OR (architecture = $%d AND id > $%d))", n+1, n+1, n+2)
			}
		case "created_at":
			args = append(args, cursor.CreatedAt.UTC().Format(time.RFC3339), cursor.ID)
			if sortOrder == "desc" {
				keysetWhere = fmt.Sprintf("WHERE ((created_at < $%d::timestamptz) OR (created_at = $%d::timestamptz AND id < $%d))", n+1, n+1, n+2)
			} else {
				keysetWhere = fmt.Sprintf("WHERE ((created_at > $%d::timestamptz) OR (created_at = $%d::timestamptz AND id > $%d))", n+1, n+1, n+2)
			}
		default: // release_date
			args = append(args, cursor.ReleaseDate.UTC().Format(time.RFC3339), cursor.ID)
			if sortOrder == "desc" {
				keysetWhere = fmt.Sprintf("WHERE ((release_date < $%d::timestamptz) OR (release_date = $%d::timestamptz AND id < $%d))", n+1, n+1, n+2)
			} else {
				keysetWhere = fmt.Sprintf("WHERE ((release_date > $%d::timestamptz) OR (release_date = $%d::timestamptz AND id > $%d))", n+1, n+1, n+2)
			}
		}
	}

	args = append(args, int64(limit))
	query := fmt.Sprintf(`
		SELECT id, application_id, version, platform, architecture, download_url,
		       checksum, checksum_type, file_size, release_notes, release_date,
		       required, minimum_version, metadata, created_at,
		       version_major, version_minor, version_patch, version_pre_release,
		       total_count
		FROM (
		    SELECT id, application_id, version, platform, architecture, download_url,
		           checksum, checksum_type, file_size, release_notes, release_date,
		           required, minimum_version, metadata, created_at,
		           version_major, version_minor, version_patch, version_pre_release,
		           COUNT(*) OVER() AS total_count
		    FROM releases
		    %s
		) AS counted
		%s
		ORDER BY %s
		LIMIT $%d`,
		businessWhere, keysetWhere, orderClause, len(args),
	)

	pgxRows, err := ps.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query releases: %w", err)
	}
	defer pgxRows.Close()

	releases := make([]*models.Release, 0)
	total := 0
	for pgxRows.Next() {
		var (
			id, appIDField, version, platform, arch, downloadURL string
			checksum, checksumType                               string
			fileSize                                             int64
			releaseNotes                                         pgtype.Text
			releaseDate                                          pgtype.Timestamptz
			required                                             bool
			minimumVersion                                       pgtype.Text
			metadata                                             []byte
			createdAt                                            pgtype.Timestamptz
			versionMajor, versionMinor, versionPatch             int64
			versionPreRelease                                    pgtype.Text
			totalCount                                           int64
		)
		if err := pgxRows.Scan(
			&id, &appIDField, &version, &platform, &arch, &downloadURL,
			&checksum, &checksumType, &fileSize,
			&releaseNotes, &releaseDate, &required, &minimumVersion,
			&metadata, &createdAt,
			&versionMajor, &versionMinor, &versionPatch, &versionPreRelease,
			&totalCount,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan release: %w", err)
		}
		if total == 0 {
			total = int(totalCount)
		}
		row := sqlcpg.Release{
			ID:                id,
			ApplicationID:     appIDField,
			Version:           version,
			Platform:          platform,
			Architecture:      arch,
			DownloadUrl:       downloadURL,
			Checksum:          checksum,
			ChecksumType:      checksumType,
			FileSize:          fileSize,
			ReleaseNotes:      releaseNotes,
			ReleaseDate:       releaseDate,
			Required:          required,
			MinimumVersion:    minimumVersion,
			Metadata:          metadata,
			CreatedAt:         createdAt,
			VersionMajor:      versionMajor,
			VersionMinor:      versionMinor,
			VersionPatch:      versionPatch,
			VersionPreRelease: versionPreRelease,
		}
		release, err := pgReleaseToModel(row)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to convert release %s: %w", id, err)
		}
		releases = append(releases, release)
	}
	if err := pgxRows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate releases: %w", err)
	}
	return releases, total, nil
}

// GetLatestStableRelease returns the highest non-prerelease version for the given platform/arch.
// Returns ErrNotFound if no stable release exists.
func (ps *PostgresStorage) GetLatestStableRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error) {
	row, err := ps.queries.GetLatestStableRelease(ctx, sqlcpg.GetLatestStableReleaseParams{
		ApplicationID: appID,
		Platform:      platform,
		Architecture:  arch,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get latest stable release: %w", err)
	}
	return pgReleaseToModel(row)
}

// GetApplicationStats returns aggregate statistics for an application.
func (ps *PostgresStorage) GetApplicationStats(ctx context.Context, appID string) (models.ApplicationStats, error) {
	row, err := ps.queries.GetApplicationStats(ctx, appID)
	if err != nil {
		return models.ApplicationStats{}, fmt.Errorf("failed to get application stats: %w", err)
	}

	stats := models.ApplicationStats{
		TotalReleases:    int(row.TotalReleases),
		RequiredReleases: int(row.RequiredReleases),
		PlatformCount:    int(row.PlatformCount),
		LatestVersion:    row.LatestVersion,
	}

	// LatestReleaseDate is interface{} in the generated code because sqlc cannot
	// determine the concrete type for aggregated nullable timestamps.
	// pgx/v5 returns pgtype.Timestamptz for TIMESTAMPTZ columns.
	if row.LatestReleaseDate != nil {
		if ts, ok := row.LatestReleaseDate.(pgtype.Timestamptz); ok && ts.Valid {
			t := ts.Time
			stats.LatestReleaseDate = &t
		}
	}

	return stats, nil
}
