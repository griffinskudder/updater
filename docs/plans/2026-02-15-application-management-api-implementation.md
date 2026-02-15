# Application Management API Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add full CRUD HTTP endpoints for managing applications and a delete endpoint for releases, with referential integrity enforcement.

**Architecture:** Extend the existing `ServiceInterface` with six new methods, add `DeleteApplication` to the `Storage` interface, implement across all four backends, create HTTP handlers in a new file, and register routes with appropriate auth/permission middleware. Database schemas get a migration to change FK from `CASCADE` to `RESTRICT`.

**Tech Stack:** Go 1.25, gorilla/mux, sqlc, pgx/v5 (PostgreSQL), go-sqlite3 (SQLite), httptest (testing)

---

### Task 1: Add `DeleteApplication` to Storage Interface and Errors

**Files:**
- Modify: `internal/storage/interface.go:11-43`
- Create: `internal/storage/errors.go`
- Modify: `internal/update/errors.go:28-63`

**Step 1: Add `DeleteApplication` to the Storage interface**

Add the following method to the `Storage` interface in `internal/storage/interface.go`, after the `SaveApplication` method (line 19):

```go
// DeleteApplication removes an application by its ID
DeleteApplication(ctx context.Context, appID string) error
```

**Step 2: Create storage sentinel errors**

Create `internal/storage/errors.go`:

```go
package storage

import "errors"

// ErrHasDependencies indicates a resource cannot be deleted because other resources depend on it.
var ErrHasDependencies = errors.New("resource has dependent records")
```

**Step 3: Add `NewConflictError` and `NewNotFoundError` to service errors**

Add to `internal/update/errors.go`:

```go
func NewConflictError(message string) *ServiceError {
	return &ServiceError{
		Code:       models.ErrorCodeConflict,
		Message:    message,
		StatusCode: http.StatusConflict,
	}
}

func NewNotFoundError(message string) *ServiceError {
	return &ServiceError{
		Code:       models.ErrorCodeNotFound,
		Message:    message,
		StatusCode: http.StatusNotFound,
	}
}
```

**Step 4: Verify compilation**

Run: `go build ./...`
Expected: Compilation fails because four storage backends and the `InstrumentedStorage` wrapper do not implement `DeleteApplication`. This is expected -- we will fix it in the next tasks.

**Step 5: Commit**

```bash
git add internal/storage/interface.go internal/storage/errors.go internal/update/errors.go
git commit -m "feat: add DeleteApplication to Storage interface and error types"
```

---

### Task 2: Implement `DeleteApplication` in Memory Storage

**Files:**
- Modify: `internal/storage/memory.go`
- Modify: `internal/storage/memory_test.go`

**Step 1: Write the failing test**

Add to `internal/storage/memory_test.go`:

```go
func TestMemoryStorage_DeleteApplication(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, s *MemoryStorage)
		appID     string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "delete existing application",
			setup: func(t *testing.T, s *MemoryStorage) {
				app := models.NewApplication("test-app", "Test App", []string{"windows"})
				err := s.SaveApplication(context.Background(), app)
				if err != nil {
					t.Fatal(err)
				}
			},
			appID:   "test-app",
			wantErr: false,
		},
		{
			name:      "delete non-existent application",
			setup:     func(t *testing.T, s *MemoryStorage) {},
			appID:     "non-existent",
			wantErr:   true,
			errSubstr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := NewMemoryStorage(Config{})
			tt.setup(t, s)

			err := s.DeleteApplication(context.Background(), tt.appID)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify application is gone
			_, err = s.GetApplication(context.Background(), tt.appID)
			if err == nil {
				t.Fatal("expected error when getting deleted application")
			}
		})
	}
}
```

Ensure `"strings"` and `"context"` are in the import list of the test file.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/ -run TestMemoryStorage_DeleteApplication -v`
Expected: FAIL -- `DeleteApplication` method does not exist on `MemoryStorage`.

**Step 3: Implement `DeleteApplication` on `MemoryStorage`**

Add to `internal/storage/memory.go`, after the `SaveApplication` method:

```go
// DeleteApplication removes an application by its ID
func (m *MemoryStorage) DeleteApplication(ctx context.Context, appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.applications[appID]; !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	delete(m.applications, appID)
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/storage/ -run TestMemoryStorage_DeleteApplication -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/storage/memory.go internal/storage/memory_test.go
git commit -m "feat: implement DeleteApplication on MemoryStorage"
```

---

### Task 3: Implement `DeleteApplication` in JSON Storage

**Files:**
- Modify: `internal/storage/json.go`
- Modify: `internal/storage/json_test.go`

**Step 1: Write the failing test**

Add to `internal/storage/json_test.go` a test similar to `TestMemoryStorage_DeleteApplication` but using `NewJSONStorage`. Use a temp directory for the JSON file:

```go
func TestJSONStorage_DeleteApplication(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, s *JSONStorage)
		appID     string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "delete existing application",
			setup: func(t *testing.T, s *JSONStorage) {
				app := models.NewApplication("test-app", "Test App", []string{"windows"})
				err := s.SaveApplication(context.Background(), app)
				if err != nil {
					t.Fatal(err)
				}
			},
			appID:   "test-app",
			wantErr: false,
		},
		{
			name:      "delete non-existent application",
			setup:     func(t *testing.T, s *JSONStorage) {},
			appID:     "non-existent",
			wantErr:   true,
			errSubstr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			s, err := NewJSONStorage(Config{Path: tmpDir + "/test.json"})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			tt.setup(t, s)

			err = s.DeleteApplication(context.Background(), tt.appID)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify application is gone
			_, err = s.GetApplication(context.Background(), tt.appID)
			if err == nil {
				t.Fatal("expected error when getting deleted application")
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/ -run TestJSONStorage_DeleteApplication -v`
Expected: FAIL

**Step 3: Implement `DeleteApplication` on `JSONStorage`**

Add to `internal/storage/json.go`, after the `SaveApplication` method. Follow the existing pattern -- acquire write lock, load data, modify, save:

```go
// DeleteApplication removes an application by its ID
func (j *JSONStorage) DeleteApplication(ctx context.Context, appID string) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if err := j.loadData(); err != nil {
		return fmt.Errorf("failed to load data: %w", err)
	}

	found := false
	for i, app := range j.data.Applications {
		if app.ID == appID {
			j.data.Applications = append(j.data.Applications[:i], j.data.Applications[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("application %s not found", appID)
	}

	return j.saveData(j.data)
}
```

Check the `JSONData` struct and `loadData`/`saveData` patterns in `json.go` to match the exact field names and patterns used.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/storage/ -run TestJSONStorage_DeleteApplication -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/storage/json.go internal/storage/json_test.go
git commit -m "feat: implement DeleteApplication on JSONStorage"
```

---

### Task 4: Implement `DeleteApplication` in PostgreSQL and SQLite Storage

**Files:**
- Modify: `internal/storage/postgres.go`
- Modify: `internal/storage/sqlite.go`
- Modify: `internal/storage/postgres_test.go`
- Modify: `internal/storage/sqlite_test.go`

**Important context:** The sqlc-generated `DeleteApplication` method already exists in both `internal/storage/sqlc/postgres/applications.sql.go` and `internal/storage/sqlc/sqlite/applications.sql.go`. The existing schemas already have `FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE`.

**Step 1: Implement `DeleteApplication` on `PostgresStorage`**

Add to `internal/storage/postgres.go`, after the `SaveApplication` method:

```go
// DeleteApplication removes an application by its ID.
func (ps *PostgresStorage) DeleteApplication(ctx context.Context, appID string) error {
	err := ps.queries.DeleteApplication(ctx, appID)
	if err != nil {
		return fmt.Errorf("failed to delete application %s: %w", appID, err)
	}
	return nil
}
```

**Step 2: Implement `DeleteApplication` on `SQLiteStorage`**

Add to `internal/storage/sqlite.go`, after the `SaveApplication` method:

```go
// DeleteApplication removes an application by its ID.
func (ss *SQLiteStorage) DeleteApplication(ctx context.Context, appID string) error {
	err := ss.queries.DeleteApplication(ctx, appID)
	if err != nil {
		return fmt.Errorf("failed to delete application %s: %w", appID, err)
	}
	return nil
}
```

Check the exact query method signature in the generated sqlc code to ensure the parameter type matches (should be `string`).

**Step 3: Add tests for both backends**

Add tests to `internal/storage/postgres_test.go` and `internal/storage/sqlite_test.go` if database test infrastructure exists. If these tests require a running database, add a `t.Skip` guard:

```go
func TestPostgresStorage_DeleteApplication(t *testing.T) {
	// Skip if no database available
	connStr := os.Getenv("TEST_POSTGRES_URL")
	if connStr == "" {
		t.Skip("TEST_POSTGRES_URL not set, skipping PostgreSQL tests")
	}
	// ... test implementation similar to memory test
}
```

**Step 4: Verify compilation**

Run: `go build ./...`
Expected: Still fails because `InstrumentedStorage` does not implement `DeleteApplication`.

**Step 5: Commit**

```bash
git add internal/storage/postgres.go internal/storage/sqlite.go internal/storage/postgres_test.go internal/storage/sqlite_test.go
git commit -m "feat: implement DeleteApplication on PostgreSQL and SQLite storage"
```

---

### Task 5: Implement `DeleteApplication` on InstrumentedStorage

**Files:**
- Modify: `internal/observability/storage.go`
- Modify: `internal/observability/storage_test.go`

**Step 1: Add `DeleteApplication` to `InstrumentedStorage`**

Add to `internal/observability/storage.go`, after the `SaveApplication` method (follow the exact same tracing pattern as the other methods):

```go
func (s *InstrumentedStorage) DeleteApplication(ctx context.Context, appID string) error {
	ctx, span := s.startSpan(ctx, "DeleteApplication",
		attribute.String("app_id", appID),
	)
	start := time.Now()
	err := s.wrapped.DeleteApplication(ctx, appID)
	s.record(ctx, span, "DeleteApplication", start, err)
	return err
}
```

Check the exact import for `attribute` and match the pattern used by the existing methods in the file.

**Step 2: Verify compilation succeeds**

Run: `go build ./...`
Expected: SUCCESS -- all implementations of `Storage` interface now include `DeleteApplication`.

**Step 3: Run all tests**

Run: `go test ./...`
Expected: All existing tests pass.

**Step 4: Commit**

```bash
git add internal/observability/storage.go internal/observability/storage_test.go
git commit -m "feat: add DeleteApplication to InstrumentedStorage wrapper"
```

---

### Task 6: Add Database Migration for FK RESTRICT

**Files:**
- Create: `internal/storage/sqlc/schema/postgres/003_fk_restrict.sql`
- Create: `internal/storage/sqlc/schema/sqlite/003_fk_restrict.sql`

**Important context:** The existing schemas use `ON DELETE CASCADE`. The approved design requires `ON DELETE RESTRICT` so that deleting an application with existing releases is rejected by the database.

**Step 1: Create PostgreSQL migration**

Create `internal/storage/sqlc/schema/postgres/003_fk_restrict.sql`:

```sql
-- Change releases FK from CASCADE to RESTRICT
-- This prevents deleting an application that still has releases
ALTER TABLE releases DROP CONSTRAINT releases_application_id_fkey;
ALTER TABLE releases ADD CONSTRAINT releases_application_id_fkey
    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE RESTRICT;
```

Note: Check the exact constraint name by looking at the PostgreSQL default naming convention or the `001_initial.sql`. The default name for an unnamed FK in PostgreSQL is `{table}_{column}_fkey`.

**Step 2: Create SQLite migration**

SQLite does not support `ALTER TABLE ... DROP CONSTRAINT`. For SQLite, the FK behavior is enforced at the application level (the service checks for releases before deleting). Create a placeholder migration:

Create `internal/storage/sqlc/schema/sqlite/003_fk_restrict.sql`:

```sql
-- SQLite does not support altering FK constraints.
-- Referential integrity for DeleteApplication is enforced at the service layer.
-- This file exists for migration numbering parity with PostgreSQL.
```

**Step 3: Regenerate sqlc code**

Run: `sqlc generate`
Expected: Code regenerates successfully. The generated Go code should not change since the queries are the same.

**Step 4: Verify compilation and tests**

Run: `go build ./... && go test ./...`
Expected: All pass.

**Step 5: Commit**

```bash
git add internal/storage/sqlc/
git commit -m "feat: add migration to change FK from CASCADE to RESTRICT"
```

---

### Task 7: Add Application CRUD Methods to Service

**Files:**
- Modify: `internal/update/interface.go`
- Modify: `internal/update/service.go`
- Modify: `internal/update/service_test.go`

**Step 1: Write failing tests for `CreateApplication`**

Add to `internal/update/service_test.go`:

```go
func TestService_CreateApplication(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, s storage.Storage)
		req       *models.CreateApplicationRequest
		wantErr   bool
		wantCode  int
	}{
		{
			name:  "create valid application",
			setup: func(t *testing.T, s storage.Storage) {},
			req: &models.CreateApplicationRequest{
				ID:        "my-app",
				Name:      "My App",
				Platforms: []string{"windows", "linux"},
			},
			wantErr: false,
		},
		{
			name: "create duplicate application",
			setup: func(t *testing.T, s storage.Storage) {
				app := models.NewApplication("my-app", "Existing App", []string{"windows"})
				s.SaveApplication(context.Background(), app)
			},
			req: &models.CreateApplicationRequest{
				ID:        "my-app",
				Name:      "My App",
				Platforms: []string{"windows"},
			},
			wantErr:  true,
			wantCode: http.StatusConflict,
		},
		{
			name:  "create with invalid request - missing name",
			setup: func(t *testing.T, s storage.Storage) {},
			req: &models.CreateApplicationRequest{
				ID:        "my-app",
				Name:      "",
				Platforms: []string{"windows"},
			},
			wantErr:  true,
			wantCode: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, _ := storage.NewMemoryStorage(storage.Config{})
			tt.setup(t, store)
			svc := NewService(store)

			resp, err := svc.CreateApplication(context.Background(), tt.req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				var svcErr *ServiceError
				if errors.As(err, &svcErr) && tt.wantCode != 0 {
					if svcErr.StatusCode != tt.wantCode {
						t.Errorf("expected status %d, got %d", tt.wantCode, svcErr.StatusCode)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.ID != tt.req.ID {
				t.Errorf("expected ID %q, got %q", tt.req.ID, resp.ID)
			}
		})
	}
}
```

**Step 2: Write failing tests for remaining methods**

Add similar table-driven tests for:
- `TestService_GetApplication` -- success, not found
- `TestService_ListApplications` -- empty list, multiple apps, pagination
- `TestService_UpdateApplication` -- success, not found, partial update (only name)
- `TestService_DeleteApplication` -- success, not found, has releases (expect conflict)
- `TestService_DeleteRelease` -- success, not found

Each test should follow the same pattern: create memory storage, setup test data, call service method, assert result.

For `DeleteApplication` with releases, the test should:
1. Save an application
2. Save a release for that application
3. Call `DeleteApplication`
4. Assert error is `409 Conflict`

**Step 3: Run tests to verify they fail**

Run: `go test ./internal/update/ -run "TestService_(Create|Get|List|Update|Delete)" -v`
Expected: FAIL -- methods do not exist on `Service`.

**Step 4: Extend `ServiceInterface`**

Update `internal/update/interface.go` to add the new methods:

```go
type ServiceInterface interface {
	CheckForUpdate(ctx context.Context, req *models.UpdateCheckRequest) (*models.UpdateCheckResponse, error)
	GetLatestVersion(ctx context.Context, req *models.LatestVersionRequest) (*models.LatestVersionResponse, error)
	ListReleases(ctx context.Context, req *models.ListReleasesRequest) (*models.ListReleasesResponse, error)
	RegisterRelease(ctx context.Context, req *models.RegisterReleaseRequest) (*models.RegisterReleaseResponse, error)

	CreateApplication(ctx context.Context, req *models.CreateApplicationRequest) (*models.CreateApplicationResponse, error)
	GetApplication(ctx context.Context, appID string) (*models.ApplicationInfoResponse, error)
	ListApplications(ctx context.Context, limit, offset int) (*models.ListApplicationsResponse, error)
	UpdateApplication(ctx context.Context, appID string, req *models.UpdateApplicationRequest) (*models.UpdateApplicationResponse, error)
	DeleteApplication(ctx context.Context, appID string) error
	DeleteRelease(ctx context.Context, req *models.DeleteReleaseRequest) (*models.DeleteReleaseResponse, error)
}
```

**Step 5: Implement all six methods on `Service`**

Add to `internal/update/service.go`. Key implementations:

**CreateApplication:**
```go
func (s *Service) CreateApplication(ctx context.Context, req *models.CreateApplicationRequest) (*models.CreateApplicationResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, NewValidationError("invalid request", err)
	}
	req.Normalize()

	// Check for duplicate
	existing, err := s.storage.GetApplication(ctx, req.ID)
	if err == nil && existing != nil {
		return nil, NewConflictError(fmt.Sprintf("application '%s' already exists", req.ID))
	}

	app := models.NewApplication(req.ID, req.Name, req.Platforms)
	app.Description = req.Description
	app.Config = req.Config
	now := time.Now()
	app.CreatedAt = now.Format(time.RFC3339)
	app.UpdatedAt = now.Format(time.RFC3339)

	if err := s.storage.SaveApplication(ctx, app); err != nil {
		return nil, NewInternalError("failed to save application", err)
	}

	return &models.CreateApplicationResponse{
		ID:        app.ID,
		Message:   fmt.Sprintf("Application '%s' created successfully", app.Name),
		CreatedAt: now,
	}, nil
}
```

**GetApplication:**
```go
func (s *Service) GetApplication(ctx context.Context, appID string) (*models.ApplicationInfoResponse, error) {
	app, err := s.storage.GetApplication(ctx, appID)
	if err != nil {
		return nil, NewApplicationNotFoundError(appID)
	}

	// Compute stats
	releases, err := s.storage.Releases(ctx, appID)
	if err != nil {
		return nil, NewInternalError("failed to get releases", err)
	}

	stats := models.ApplicationStats{
		TotalReleases: len(releases),
	}

	platforms := make(map[string]bool)
	var latestRelease *models.Release
	requiredCount := 0
	for _, r := range releases {
		platforms[r.Platform] = true
		if r.Required {
			requiredCount++
		}
		if latestRelease == nil {
			latestRelease = r
			continue
		}
		latestVer, err1 := semver.NewVersion(latestRelease.Version)
		releaseVer, err2 := semver.NewVersion(r.Version)
		if err1 == nil && err2 == nil && releaseVer.GreaterThan(latestVer) {
			latestRelease = r
		}
	}
	stats.PlatformCount = len(platforms)
	stats.RequiredReleases = requiredCount
	if latestRelease != nil {
		stats.LatestVersion = latestRelease.Version
		stats.LatestReleaseDate = &latestRelease.ReleaseDate
	}

	// Parse timestamps
	createdAt, _ := time.Parse(time.RFC3339, app.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, app.UpdatedAt)

	return &models.ApplicationInfoResponse{
		ID:          app.ID,
		Name:        app.Name,
		Description: app.Description,
		Platforms:   app.Platforms,
		Config:      app.Config,
		Stats:       stats,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}
```

**ListApplications:**
```go
func (s *Service) ListApplications(ctx context.Context, limit, offset int) (*models.ListApplicationsResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	allApps, err := s.storage.Applications(ctx)
	if err != nil {
		return nil, NewInternalError("failed to get applications", err)
	}

	totalCount := len(allApps)
	start := offset
	end := start + limit
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}

	paginatedApps := allApps[start:end]
	summaries := make([]models.ApplicationSummary, len(paginatedApps))
	for i, app := range paginatedApps {
		summaries[i].FromApplication(app)
		createdAt, _ := time.Parse(time.RFC3339, app.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, app.UpdatedAt)
		summaries[i].CreatedAt = createdAt
		summaries[i].UpdatedAt = updatedAt
	}

	return &models.ListApplicationsResponse{
		Applications: summaries,
		TotalCount:   totalCount,
		Page:         (offset / limit) + 1,
		PageSize:     limit,
		HasMore:      end < totalCount,
	}, nil
}
```

**UpdateApplication:**
```go
func (s *Service) UpdateApplication(ctx context.Context, appID string, req *models.UpdateApplicationRequest) (*models.UpdateApplicationResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, NewValidationError("invalid request", err)
	}
	req.Normalize()

	app, err := s.storage.GetApplication(ctx, appID)
	if err != nil {
		return nil, NewApplicationNotFoundError(appID)
	}

	// Apply partial updates
	if req.Name != nil {
		app.Name = *req.Name
	}
	if req.Description != nil {
		app.Description = *req.Description
	}
	if req.Platforms != nil {
		app.Platforms = req.Platforms
	}
	if req.Config != nil {
		app.Config = *req.Config
	}
	now := time.Now()
	app.UpdatedAt = now.Format(time.RFC3339)

	if err := s.storage.SaveApplication(ctx, app); err != nil {
		return nil, NewInternalError("failed to update application", err)
	}

	return &models.UpdateApplicationResponse{
		ID:        app.ID,
		Message:   fmt.Sprintf("Application '%s' updated successfully", app.Name),
		UpdatedAt: now,
	}, nil
}
```

**DeleteApplication:**
```go
func (s *Service) DeleteApplication(ctx context.Context, appID string) error {
	// Verify application exists
	_, err := s.storage.GetApplication(ctx, appID)
	if err != nil {
		return NewApplicationNotFoundError(appID)
	}

	// Check for existing releases (service-layer enforcement for memory/JSON backends)
	releases, err := s.storage.Releases(ctx, appID)
	if err != nil {
		return NewInternalError("failed to check releases", err)
	}
	if len(releases) > 0 {
		return NewConflictError(fmt.Sprintf("cannot delete application '%s': has %d existing releases", appID, len(releases)))
	}

	// Attempt delete (database backends enforce FK RESTRICT as a second layer)
	if err := s.storage.DeleteApplication(ctx, appID); err != nil {
		if errors.Is(err, storage.ErrHasDependencies) {
			return NewConflictError(fmt.Sprintf("cannot delete application '%s': has existing releases", appID))
		}
		return NewInternalError("failed to delete application", err)
	}

	return nil
}
```

**DeleteRelease:**
```go
func (s *Service) DeleteRelease(ctx context.Context, req *models.DeleteReleaseRequest) (*models.DeleteReleaseResponse, error) {
	// Verify the release exists
	release, err := s.storage.GetRelease(ctx, req.ApplicationID, req.ReleaseID, "", "")
	if err != nil {
		// The DeleteReleaseRequest uses ReleaseID but storage uses version/platform/arch compound key.
		// We need to reconsider how to identify releases for deletion.
		return nil, NewNotFoundError(fmt.Sprintf("release not found: %s", req.ReleaseID))
	}

	if err := s.storage.DeleteRelease(ctx, release.ApplicationID, release.Version, release.Platform, release.Architecture); err != nil {
		return nil, NewInternalError("failed to delete release", err)
	}

	return &models.DeleteReleaseResponse{
		ID:      release.ID,
		Message: fmt.Sprintf("Release %s deleted successfully", release.Version),
	}, nil
}
```

**Important:** The `DeleteReleaseRequest` model uses `ApplicationID` and `ReleaseID`, but the storage `DeleteRelease` method uses `appID, version, platform, arch`. The HTTP handler will extract these from the URL path (`{version}/{platform}/{arch}`) and call storage directly. Adjust the service method to accept the compound key instead:

```go
func (s *Service) DeleteRelease(ctx context.Context, appID, version, platform, arch string) (*models.DeleteReleaseResponse, error) {
	// Verify the release exists
	release, err := s.storage.GetRelease(ctx, appID, version, platform, arch)
	if err != nil {
		return nil, NewNotFoundError(fmt.Sprintf("release not found: %s@%s-%s-%s", appID, version, platform, arch))
	}

	if err := s.storage.DeleteRelease(ctx, appID, version, platform, arch); err != nil {
		return nil, NewInternalError("failed to delete release", err)
	}

	return &models.DeleteReleaseResponse{
		ID:      release.ID,
		Message: fmt.Sprintf("Release %s for %s-%s deleted successfully", version, platform, arch),
	}, nil
}
```

Update `ServiceInterface` accordingly -- change `DeleteRelease` signature to use the compound key.

Add `"time"` and `"errors"` to the imports in `service.go`. Add `"updater/internal/storage"` for the `ErrHasDependencies` sentinel.

**Step 6: Run tests to verify they pass**

Run: `go test ./internal/update/ -v`
Expected: All tests pass.

**Step 7: Commit**

```bash
git add internal/update/interface.go internal/update/service.go internal/update/service_test.go
git commit -m "feat: add application CRUD and delete release to service layer"
```

---

### Task 8: Create Application Management HTTP Handlers

**Files:**
- Create: `internal/api/handlers_applications.go`
- Create: `internal/api/handlers_applications_test.go`

**Step 1: Write failing handler tests**

Create `internal/api/handlers_applications_test.go` with tests for each handler. Use `httptest.NewRecorder` and a real `Handlers` instance with memory storage. Pattern:

```go
func TestHandlers_CreateApplication(t *testing.T) {
	store, _ := storage.NewMemoryStorage(storage.Config{})
	svc := update.NewService(store)
	h := NewHandlers(svc, WithStorage(store))

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid create",
			body:       `{"id":"test-app","name":"Test App","platforms":["windows"]}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid json",
			body:       `{invalid`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing required field",
			body:       `{"id":"test-app","name":"","platforms":["windows"]}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/applications", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.CreateApplication(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}
```

Add similar tests for `GetApplication`, `ListApplications`, `UpdateApplication`, `DeleteApplication`, and `DeleteRelease`. For path variable tests, use `mux.SetURLVars` to inject route variables:

```go
req = mux.SetURLVars(req, map[string]string{"app_id": "test-app"})
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -run "TestHandlers_(Create|Get|List|Update|Delete)Application" -v`
Expected: FAIL -- handler methods do not exist.

**Step 3: Implement handlers**

Create `internal/api/handlers_applications.go`:

```go
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"updater/internal/models"

	"github.com/gorilla/mux"
)

// CreateApplication handles application creation requests
// POST /api/v1/applications
func (h *Handlers) CreateApplication(w http.ResponseWriter, r *http.Request) {
	securityContext := GetSecurityContext(r)

	slog.Warn("Application creation attempt",
		"event", "security_audit",
		"api_key", getAPIKeyName(securityContext),
		"client_ip", getClientIP(r))

	contentType := r.Header.Get("Content-Type")
	if contentType == "" || !strings.HasPrefix(contentType, "application/json") {
		h.writeErrorResponse(w, http.StatusUnsupportedMediaType, models.ErrorCodeBadRequest, "Content-Type must be application/json")
		return
	}

	var req models.CreateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeInvalidRequest, "Invalid JSON body")
		return
	}

	response, err := h.updateService.CreateApplication(r.Context(), &req)
	if err != nil {
		slog.Warn("Application creation failed",
			"event", "security_audit",
			"app_id", req.ID,
			"api_key", getAPIKeyName(securityContext),
			"error", err.Error())
		h.writeServiceErrorResponse(w, err)
		return
	}

	slog.Info("Application created successfully",
		"event", "security_audit",
		"app_id", req.ID,
		"api_key", getAPIKeyName(securityContext))

	h.writeJSONResponse(w, http.StatusCreated, response)
}

// GetApplication handles application retrieval requests
// GET /api/v1/applications/{app_id}
func (h *Handlers) GetApplication(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]

	response, err := h.updateService.GetApplication(r.Context(), appID)
	if err != nil {
		h.writeServiceErrorResponse(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// ListApplications handles application listing requests
// GET /api/v1/applications
func (h *Handlers) ListApplications(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0

	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetParam := r.URL.Query().Get("offset"); offsetParam != "" {
		if o, err := strconv.Atoi(offsetParam); err == nil && o >= 0 {
			offset = o
		}
	}

	response, err := h.updateService.ListApplications(r.Context(), limit, offset)
	if err != nil {
		h.writeServiceErrorResponse(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// UpdateApplication handles application update requests
// PUT /api/v1/applications/{app_id}
func (h *Handlers) UpdateApplication(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]

	securityContext := GetSecurityContext(r)

	slog.Warn("Application update attempt",
		"event", "security_audit",
		"app_id", appID,
		"api_key", getAPIKeyName(securityContext),
		"client_ip", getClientIP(r))

	contentType := r.Header.Get("Content-Type")
	if contentType == "" || !strings.HasPrefix(contentType, "application/json") {
		h.writeErrorResponse(w, http.StatusUnsupportedMediaType, models.ErrorCodeBadRequest, "Content-Type must be application/json")
		return
	}

	var req models.UpdateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, models.ErrorCodeInvalidRequest, "Invalid JSON body")
		return
	}

	response, err := h.updateService.UpdateApplication(r.Context(), appID, &req)
	if err != nil {
		slog.Warn("Application update failed",
			"event", "security_audit",
			"app_id", appID,
			"api_key", getAPIKeyName(securityContext),
			"error", err.Error())
		h.writeServiceErrorResponse(w, err)
		return
	}

	slog.Info("Application updated successfully",
		"event", "security_audit",
		"app_id", appID,
		"api_key", getAPIKeyName(securityContext))

	h.writeJSONResponse(w, http.StatusOK, response)
}

// DeleteApplication handles application deletion requests
// DELETE /api/v1/applications/{app_id}
func (h *Handlers) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]

	securityContext := GetSecurityContext(r)

	slog.Warn("Application deletion attempt",
		"event", "security_audit",
		"app_id", appID,
		"api_key", getAPIKeyName(securityContext),
		"client_ip", getClientIP(r))

	err := h.updateService.DeleteApplication(r.Context(), appID)
	if err != nil {
		slog.Warn("Application deletion failed",
			"event", "security_audit",
			"app_id", appID,
			"api_key", getAPIKeyName(securityContext),
			"error", err.Error())
		h.writeServiceErrorResponse(w, err)
		return
	}

	slog.Info("Application deleted successfully",
		"event", "security_audit",
		"app_id", appID,
		"api_key", getAPIKeyName(securityContext))

	w.WriteHeader(http.StatusNoContent)
}

// DeleteRelease handles release deletion requests
// DELETE /api/v1/updates/{app_id}/releases/{version}/{platform}/{arch}
func (h *Handlers) DeleteRelease(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["app_id"]
	version := vars["version"]
	platform := vars["platform"]
	arch := vars["arch"]

	securityContext := GetSecurityContext(r)

	slog.Warn("Release deletion attempt",
		"event", "security_audit",
		"app_id", appID,
		"version", version,
		"platform", platform,
		"architecture", arch,
		"api_key", getAPIKeyName(securityContext),
		"client_ip", getClientIP(r))

	response, err := h.updateService.DeleteRelease(r.Context(), appID, version, platform, arch)
	if err != nil {
		slog.Warn("Release deletion failed",
			"event", "security_audit",
			"app_id", appID,
			"version", version,
			"api_key", getAPIKeyName(securityContext),
			"error", err.Error())
		h.writeServiceErrorResponse(w, err)
		return
	}

	slog.Info("Release deleted successfully",
		"event", "security_audit",
		"app_id", appID,
		"version", version,
		"api_key", getAPIKeyName(securityContext))

	h.writeJSONResponse(w, http.StatusOK, response)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestHandlers_(Create|Get|List|Update|Delete)" -v`
Expected: All pass.

**Step 5: Commit**

```bash
git add internal/api/handlers_applications.go internal/api/handlers_applications_test.go
git commit -m "feat: add application management and release deletion HTTP handlers"
```

---

### Task 9: Register Routes

**Files:**
- Modify: `internal/api/routes.go:31-103`

**Step 1: Add application management routes**

In the `SetupRoutes` function in `internal/api/routes.go`, add the application management endpoints. Add them inside the `if config.Security.EnableAuth` block, following the existing pattern of creating subrouters with middleware:

Inside the `if config.Security.EnableAuth` block, after the existing `writeAPI` routes:

```go
// Application management endpoints (read permission)
appReadAPI := api.PathPrefix("/applications").Subrouter()
appReadAPI.Use(authMiddleware(config.Security))
appReadAPI.Use(RequirePermission(PermissionRead))
appReadAPI.HandleFunc("", handlers.ListApplications).Methods("GET")
appReadAPI.HandleFunc("/{app_id}", handlers.GetApplication).Methods("GET")

// Application management endpoints (write permission)
appWriteAPI := api.PathPrefix("/applications").Subrouter()
appWriteAPI.Use(authMiddleware(config.Security))
appWriteAPI.Use(RequirePermission(PermissionWrite))
appWriteAPI.HandleFunc("", handlers.CreateApplication).Methods("POST")

// Application management endpoints (admin permission)
appAdminAPI := api.PathPrefix("/applications").Subrouter()
appAdminAPI.Use(authMiddleware(config.Security))
appAdminAPI.Use(RequirePermission(PermissionAdmin))
appAdminAPI.HandleFunc("/{app_id}", handlers.UpdateApplication).Methods("PUT")
appAdminAPI.HandleFunc("/{app_id}", handlers.DeleteApplication).Methods("DELETE")

// Release deletion (admin permission)
adminAPI := api.PathPrefix("").Subrouter()
adminAPI.Use(authMiddleware(config.Security))
adminAPI.Use(RequirePermission(PermissionAdmin))
adminAPI.HandleFunc("/updates/{app_id}/releases/{version}/{platform}/{arch}", handlers.DeleteRelease).Methods("DELETE")
```

In the `else` block (auth disabled), add unprotected equivalents:

```go
api.HandleFunc("/applications", handlers.ListApplications).Methods("GET")
api.HandleFunc("/applications/{app_id}", handlers.GetApplication).Methods("GET")
api.HandleFunc("/applications", handlers.CreateApplication).Methods("POST")
api.HandleFunc("/applications/{app_id}", handlers.UpdateApplication).Methods("PUT")
api.HandleFunc("/applications/{app_id}", handlers.DeleteApplication).Methods("DELETE")
api.HandleFunc("/updates/{app_id}/releases/{version}/{platform}/{arch}", handlers.DeleteRelease).Methods("DELETE")
```

Also add the new targets to the `.PHONY` declaration at the top of `routes.go` if applicable (note: this is Go, not Make, so just ensure correct imports).

**Step 2: Run all tests**

Run: `go test ./...`
Expected: All tests pass.

**Step 3: Commit**

```bash
git add internal/api/routes.go
git commit -m "feat: register application management and release deletion routes"
```

---

### Task 10: Add Integration Tests

**Files:**
- Modify: `internal/integration/integration_test.go`

**Step 1: Read the existing integration test file**

Read `internal/integration/integration_test.go` to understand the setup pattern (how the server is created, how requests are made, how auth headers are set).

**Step 2: Add application lifecycle integration test**

```go
func TestApplicationLifecycle(t *testing.T) {
	// Use the existing test setup pattern from the file
	// Create app -> Get app -> List apps -> Update app -> Delete app

	// 1. Create application
	// POST /api/v1/applications with write auth
	// Assert 201

	// 2. Get application
	// GET /api/v1/applications/test-app with read auth
	// Assert 200, verify fields

	// 3. List applications
	// GET /api/v1/applications with read auth
	// Assert 200, verify test-app in list

	// 4. Update application
	// PUT /api/v1/applications/test-app with admin auth
	// Assert 200

	// 5. Delete application
	// DELETE /api/v1/applications/test-app with admin auth
	// Assert 204

	// 6. Verify deleted
	// GET /api/v1/applications/test-app with read auth
	// Assert 404
}
```

**Step 3: Add referential integrity integration test**

```go
func TestDeleteApplicationWithReleases(t *testing.T) {
	// 1. Create application
	// 2. Register a release for it
	// 3. Try to delete the application -> expect 409
	// 4. Delete the release
	// 5. Delete the application -> expect 204
}
```

**Step 4: Run integration tests**

Run: `go test ./internal/integration/ -v`
Expected: All pass.

**Step 5: Commit**

```bash
git add internal/integration/integration_test.go
git commit -m "test: add application lifecycle and referential integrity integration tests"
```

---

### Task 11: Write Documentation

**Files:**
- Create: `docs/api.md`
- Modify: `mkdocs.yml`
- Modify: `docs/ARCHITECTURE.md`

**Step 1: Create API documentation**

Create `docs/api.md` with:
- Overview of all endpoints (existing update endpoints + new application management endpoints)
- Authentication and permissions table
- Request/response examples for each endpoint (use JSON code blocks)
- Error codes and their meanings
- Endpoint flow diagrams using mermaid

**Step 2: Update mkdocs nav**

Add `- API: api.md` to the nav in `mkdocs.yml`, after "Use Cases" and before "Architecture".

**Step 3: Update architecture docs**

Update the endpoint table in `docs/ARCHITECTURE.md` to include the new application management endpoints and delete release endpoint.

**Step 4: Verify docs build**

Run: `make docs-build`
Expected: MkDocs builds successfully with the new page.

**Step 5: Commit**

```bash
git add docs/api.md mkdocs.yml docs/ARCHITECTURE.md
git commit -m "docs: add API reference and update architecture docs"
```

---

### Task 12: Final Verification

**Step 1: Run full test suite**

Run: `go test ./...`
Expected: All tests pass.

**Step 2: Run vet**

Run: `go vet ./...`
Expected: No issues.

**Step 3: Run fmt**

Run: `go fmt ./...`
Expected: No formatting changes needed.

**Step 4: Verify build**

Run: `go build ./cmd/updater`
Expected: Builds successfully.

**Step 5: Review all changes**

Run: `git diff main --stat`
Verify all expected files are modified/created and nothing unexpected is included.

**Step 6: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: final adjustments from application management API review"
```