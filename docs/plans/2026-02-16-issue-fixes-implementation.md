# Issue Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all 9 open GitHub issues across three focused PRs: security bugs (#1-3), behavioural bugs (#4-6, #8), and CI quality (#7, #9).

**Architecture:** Each PR is an independent branch off `main`, merged in order. Security PR first; bugs PR second; CI quality third. All changes are minimal — no refactoring beyond what each issue requires. Tests are written before implementation (TDD).

**Tech Stack:** Go 1.25, `net/url` (stdlib), `sync` (stdlib), `sort` (stdlib), GitHub Actions, Docker (Makefile targets).

---

## PR 1: `fix/security` — Closes #1, #2, #3

---

### Task 1: Branch

**Step 1: Create and switch to the branch**

```bash
git checkout -b fix/security
```

Expected: prompt shows `fix/security`.

**Step 2: Commit**

No files changed yet — just the branch.

---

### Task 2: Fix JSON storage file permissions (#1)

**Files:**
- Modify: `internal/storage/json.go:66` and `internal/storage/json.go:135`
- Test: `internal/storage/json_test.go`

**Step 1: Write the failing tests**

In `internal/storage/json_test.go`, add after the `TestNewJSONStorage` function:

```go
func TestNewJSONStorage_FilePermissions(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("permission bits are not enforced on Windows")
	}
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "subdir", "test.json")

	storage, err := NewJSONStorage(Config{Type: "json", Path: filePath})
	require.NoError(t, err)
	defer storage.Close()

	// Directory must be traversable by owner only.
	dirInfo, err := os.Stat(filepath.Dir(filePath))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm(),
		"directory should be 0700 (owner rwx only)")

	// Data file must be readable/writable by owner only.
	fileInfo, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fileInfo.Mode().Perm(),
		"data file should be 0600 (owner rw only)")
}
```

Add `"os"` to the import block of `json_test.go` (it is already imported, verify with a search).

**Step 2: Run the tests to confirm they fail**

```bash
make test
```

Expected: `FAIL` — directory permission is `0644`, file permission is `0644`.

**Step 3: Fix the permissions**

In `internal/storage/json.go`:

Line 66, change:
```go
if err := os.MkdirAll(filepath.Dir(j.filePath), 0644); err != nil {
```
to:
```go
if err := os.MkdirAll(filepath.Dir(j.filePath), 0700); err != nil {
```

Line 135, change:
```go
if err := os.WriteFile(j.filePath, fileData, 0644); err != nil {
```
to:
```go
if err := os.WriteFile(j.filePath, fileData, 0600); err != nil {
```

**Step 4: Run the tests to confirm they pass**

```bash
make test
```

Expected: `ok  	updater/internal/storage`

**Step 5: Commit**

```bash
git add internal/storage/json.go internal/storage/json_test.go
git commit -m "fix(storage): restrict JSON directory to 0700 and data file to 0600

Closes #1"
```

---

### Task 3: Fix admin session cookie missing Secure flag (#2)

**Files:**
- Modify: `internal/api/handlers_admin.go:146-163`
- Test: `internal/api/handlers_admin_test.go`

**Step 1: Update the existing login cookie test**

In `internal/api/handlers_admin_test.go`, find `TestAdminLogin_POST_ValidKey_Redirects`.
Inside the cookie loop, add assertions for `Secure` and `SameSite` alongside the existing `HttpOnly` check:

```go
for _, c := range cookies {
    if c.Name == "admin_session" {
        assert.Equal(t, "admin-key", c.Value)
        assert.True(t, c.HttpOnly)
        assert.True(t, c.Secure, "login cookie must carry Secure flag")
        assert.Equal(t, http.SameSiteStrictMode, c.SameSite, "login cookie must be SameSite=Strict")
        found = true
    }
}
```

**Step 2: Update the existing logout cookie test**

In `TestAdminLogout_ClearsCookieAndRedirects`, update the cookie loop:

```go
for _, c := range rec.Result().Cookies() {
    if c.Name == "admin_session" {
        assert.Equal(t, -1, c.MaxAge)
        assert.True(t, c.HttpOnly, "logout cookie must be HttpOnly")
        assert.True(t, c.Secure, "logout cookie must carry Secure flag")
        assert.Equal(t, http.SameSiteStrictMode, c.SameSite, "logout cookie must be SameSite=Strict")
    }
}
```

**Step 3: Run the tests to confirm they fail**

```bash
make test
```

Expected: `FAIL` — `Secure` is `false`, `SameSite` is `0`.

**Step 4: Fix the login cookie**

In `internal/api/handlers_admin.go`, replace lines 146-152:

```go
http.SetCookie(w, &http.Cookie{
    Name:     "admin_session",
    Value:    key,
    Path:     "/admin",
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteStrictMode,
})
```

**Step 5: Fix the logout cookie**

Replace lines 158-163:

```go
http.SetCookie(w, &http.Cookie{
    Name:     "admin_session",
    Value:    "",
    Path:     "/admin",
    MaxAge:   -1,
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteStrictMode,
})
```

**Step 6: Run the tests to confirm they pass**

```bash
make test
```

Expected: `ok  	updater/internal/api`

**Step 7: Commit**

```bash
git add internal/api/handlers_admin.go internal/api/handlers_admin_test.go
git commit -m "fix(api): add Secure flag to admin session cookie and fix logout attributes

Closes #2"
```

---

### Task 4: Fix flash message URL encoding (#3)

**Files:**
- Modify: `internal/api/handlers_admin.go:115-121`
- Test: `internal/api/handlers_admin_test.go`

**Step 1: Write the failing tests**

Add to `internal/api/handlers_admin_test.go`:

```go
func TestAddFlash_EncodesSpecialCharacters(t *testing.T) {
	// A message containing URL-significant characters must not inject extra params.
	result := addFlash("/admin/apps", "error: path=foo&admin=true", "error")
	u, err := url.Parse(result)
	require.NoError(t, err)
	assert.Equal(t, "error: path=foo&admin=true", u.Query().Get("flash"),
		"flash message must be fully decoded back to the original string")
	assert.Equal(t, "error", u.Query().Get("flash_type"))
	assert.Len(t, u.Query(), 2, "must have exactly 2 query params — no injection")
}

func TestAddFlash_WithExistingQueryString(t *testing.T) {
	result := addFlash("/admin/apps?page=2", "saved", "success")
	u, err := url.Parse(result)
	require.NoError(t, err)
	assert.Equal(t, "saved", u.Query().Get("flash"))
	assert.Equal(t, "2", u.Query().Get("page"), "existing query params must be preserved")
}
```

`"net/url"` is already imported in `handlers_admin_test.go` (it is used for `url.Values` in the login test). Verify.

**Step 2: Run the tests to confirm they fail**

```bash
make test
```

Expected: `FAIL` — `TestAddFlash_EncodesSpecialCharacters` fails because the raw `&` breaks the query string.

**Step 3: Rewrite addFlash**

In `internal/api/handlers_admin.go`, replace the `addFlash` function (lines 115-121):

```go
// addFlash appends flash query params to a redirect URL.
// msg and flashType are URL-encoded so special characters cannot inject extra parameters.
func addFlash(base, msg, flashType string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set("flash", msg)
	q.Set("flash_type", flashType)
	u.RawQuery = q.Encode()
	return u.String()
}
```

**Step 4: Add `"net/url"` to the import block**

In `internal/api/handlers_admin.go`, update the import block to include `"net/url"` and remove `"strings"` (which was only used by the old `addFlash`):

```go
import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"updater/internal/models"

	"github.com/gorilla/mux"
)
```

**Step 5: Run vet to catch any remaining unused imports**

```bash
make vet
```

Expected: no errors.

**Step 6: Run the tests to confirm they pass**

```bash
make test
```

Expected: `ok  	updater/internal/api`

**Step 7: Commit**

```bash
git add internal/api/handlers_admin.go internal/api/handlers_admin_test.go
git commit -m "fix(api): URL-encode flash message query parameters in addFlash

Closes #3"
```

---

### Task 5: Open PR 1

**Step 1: Push the branch**

```bash
git push -u origin fix/security
```

**Step 2: Open pull request**

```bash
gh pr create \
  --title "fix(security): cookie Secure flag, flash URL encoding, file permissions" \
  --body "Closes #1, #2, #3

## Changes
- Restrict JSON storage directory to \`0700\` and data file to \`0600\`
- Add \`Secure: true\` to admin session cookie; align logout cookie attributes
- URL-encode flash message parameters with \`url.Values\` to prevent query injection

## Test plan
- [ ] \`make test\` passes
- [ ] \`make vet\` passes" \
  --base main
```

**Step 3: After review and CI pass, merge and pull main**

```bash
git checkout main && git pull
```

---

## PR 2: `fix/bugs` — Closes #4, #5, #6, #8

---

### Task 6: Branch

```bash
git checkout main && git checkout -b fix/bugs
```

---

### Task 7: Fix Dockerfile Go version mismatch (#4)

**Files:**
- Modify: `Dockerfile:7`

**Step 1: Apply the fix**

In `Dockerfile`, change line 7 from:
```dockerfile
FROM golang:1.26-alpine AS builder
```
to:
```dockerfile
FROM golang:1.25-alpine AS builder
```

**Step 2: Verify the Makefile already uses 1.25**

`Makefile` line 11 already sets `GO_IMAGE := golang:1.25-alpine`. The Dockerfile is now consistent.

**Step 3: Commit**

```bash
git add Dockerfile
git commit -m "fix(docker): align builder image with go.mod toolchain (1.25)

Closes #4"
```

---

### Task 8: Fix Dockerfile static-binary check (#5)

**Files:**
- Modify: `Dockerfile:60`

**Step 1: Apply the fix**

In `Dockerfile`, change line 60 from:
```sh
RUN ldd updater 2>&1 | grep -q "not a dynamic executable" || echo "Binary is statically linked"
```
to:
```sh
RUN ldd updater 2>&1 | grep -q "not a dynamic executable" || \
  (echo "FAILED: binary is not statically linked" && exit 1)
```

**Step 2: Commit**

```bash
git add Dockerfile
git commit -m "fix(docker): fail build when binary is not statically linked

Closes #5"
```

---

### Task 9: Fix TOCTOU race in loadData and add race-detection CI (#6, #8)

**Files:**
- Modify: `internal/storage/json.go:83-124` (`loadData` function)
- Modify: `.github/workflows/ci.yml`
- Test: `internal/storage/json_test.go`

**Step 1: Write the failing concurrent test**

Add to `internal/storage/json_test.go` (add `"sync"` to imports):

```go
func TestJSONStorage_ConcurrentLoad(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	// Expire the cache to force all goroutines to hit the slow path.
	storage.mu.Lock()
	storage.cacheExpiry = time.Time{}
	storage.mu.Unlock()

	const n = 20
	errs := make(chan error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			errs <- storage.loadData()
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
	storage.mu.RLock()
	assert.NotNil(t, storage.data)
	storage.mu.RUnlock()
}
```

**Step 2: Run the test with race detection to confirm the race**

The race detector requires CGO, which the Docker target disables. Run natively if Go is installed locally:

```bash
go test -race ./internal/storage/... -run TestJSONStorage_ConcurrentLoad -v
```

Expected: `DATA RACE` reported (this confirms the existing bug).

If you don't have Go installed locally, skip this verification step and trust the CI race job (added in Task 9 step 5 below).

**Step 3: Rewrite loadData with double-checked locking**

Replace the entire `loadData` function in `internal/storage/json.go` (lines 83-124):

```go
// loadData loads data from the JSON file with caching.
// It uses double-checked locking: a fast read-lock path for cache hits,
// and a write-lock slow path with re-validation to prevent TOCTOU races.
func (j *JSONStorage) loadData() error {
	// Fast path: cache is still valid.
	j.mu.RLock()
	if j.data != nil && time.Now().Before(j.cacheExpiry) {
		j.mu.RUnlock()
		return nil
	}
	j.mu.RUnlock()

	// Slow path: acquire write lock and re-validate before doing any I/O.
	j.mu.Lock()
	defer j.mu.Unlock()

	// Another goroutine may have loaded while we waited for the write lock.
	if j.data != nil && time.Now().Before(j.cacheExpiry) {
		return nil
	}

	// Stat and all subsequent reads are done under the write lock.
	info, err := os.Stat(j.filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// If the file hasn't changed, extend the cache and return.
	if j.data != nil && !info.ModTime().After(j.lastModified) {
		j.cacheExpiry = time.Now().Add(j.cacheTTL)
		return nil
	}

	fileData, err := os.ReadFile(j.filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var data JSONData
	if err := json.Unmarshal(fileData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	j.data = &data
	j.lastModified = info.ModTime()
	j.cacheExpiry = time.Now().Add(j.cacheTTL)
	return nil
}
```

**Step 4: Run tests to confirm they pass**

```bash
make test
```

Expected: `ok  	updater/internal/storage`

**Step 5: Add race-detection job to CI**

In `.github/workflows/ci.yml`, add a new `race` job after the `security` job:

```yaml
  race:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: 'go.mod'

      - name: Race detector
        run: go test -race ./...
```

Note: This job sets up Go natively (not via Docker) so CGO is available for the race detector. This matches the pattern already used by the `security` job.

**Step 6: Commit**

```bash
git add internal/storage/json.go internal/storage/json_test.go .github/workflows/ci.yml
git commit -m "fix(storage): eliminate TOCTOU race in loadData with double-checked locking

Add race-detection CI job to catch concurrent storage and rate-limit bugs.

Closes #6, #8"
```

---

### Task 10: Open PR 2

**Step 1: Push the branch**

```bash
git push -u origin fix/bugs
```

**Step 2: Open pull request**

```bash
gh pr create \
  --title "fix(bugs): Dockerfile Go version, static-binary check, loadData TOCTOU race, race CI" \
  --body "Closes #4, #5, #6, #8

## Changes
- Align Dockerfile builder image with \`go.mod\` toolchain (\`golang:1.25-alpine\`)
- Static-binary check now fails the build on non-static output
- \`loadData\` rewritten with double-checked locking to eliminate TOCTOU race
- New \`race\` CI job runs \`go test -race ./...\` on every PR

## Test plan
- [ ] \`make test\` passes
- [ ] \`make vet\` passes
- [ ] CI race job passes" \
  --base main
```

**Step 3: After review and CI pass, merge and pull main**

```bash
git checkout main && git pull
```

---

## PR 3: `feat/ci-quality` — Closes #7, #9

---

### Task 11: Branch

```bash
git checkout main && git checkout -b feat/ci-quality
```

---

### Task 12: Replace bubble sort with sort.Slice (#7)

**Files:**
- Modify: `internal/update/service.go:350-357`

**Step 1: Locate the bubble sort**

The `sortReleases` method on `*Service` (around line 313) ends with nested loops:

```go
for i := 0; i < len(releases)-1; i++ {
    for j := 0; j < len(releases)-i-1; j++ {
        if less(j+1, j) {
            releases[j], releases[j+1] = releases[j+1], releases[j]
        }
    }
}
```

**Step 2: Run existing tests to establish baseline**

```bash
make test
```

Expected: all pass. Note the output.

**Step 3: Replace the loops**

Delete the nested loop block and replace with:

```go
sort.Slice(releases, less)
```

The `"sort"` import is already present in `service.go` (it is used elsewhere in the file). If not, add it.

**Step 4: Run tests to confirm they still pass**

```bash
make test
```

Expected: `ok  	updater/internal/update` — same results as baseline.

**Step 5: Commit**

```bash
git add internal/update/service.go
git commit -m "perf(update): replace O(n²) bubble sort with sort.Slice in sortReleases

Closes #7"
```

---

### Task 13: Add integration tests to CI (#9)

**Files:**
- Modify: `.github/workflows/ci.yml`

**Step 1: Verify integration tests pass locally**

```bash
make test
```

The `make test` target runs `go test ./...` which includes `./internal/integration/...`. Check that the integration package passes. If it fails, fix before proceeding.

**Step 2: Confirm the integration tests need no external services**

The tests in `internal/integration/integration_test.go` use:
- `t.TempDir()` for storage
- `httptest.NewServer` for the HTTP layer

No database or running server is required. The tests are fully self-contained.

**Step 3: Add the integration test step to CI**

In `.github/workflows/ci.yml`, add a step in the `ci` job after the `Validate OpenAPI spec` step:

```yaml
      - name: Integration tests
        run: make test
```

Note: `make test` already runs `go test ./...`, which includes the integration package. This step makes the intent explicit in the CI log output. If you want a truly separate, labelled step, you can use `go test -v ./internal/integration/...` instead — but that requires adding `actions/setup-go` to the `ci` job, since `ci` currently relies on Docker-based Make targets. Keeping it as `make test` avoids that change.

Actually, since `make test` already covers integration tests, the most accurate fix for issue #9 is adding a clearly-labelled step that documents this intent. Replace the existing `Coverage` step comment:

Alternatively: the current `Coverage` step already runs all tests via `make cover` (which calls `go test ./...`). The issue is that integration tests were *implicitly* running but not visibly tracked. Add an explicit step:

In the `ci` job, after `Validate OpenAPI spec`:

```yaml
      - name: Integration tests
        run: make test
```

This ensures integration tests are visibly executed and fail the job if they regress.

**Step 4: Run tests to confirm integration tests pass**

```bash
make test
```

Expected: `ok  	updater/internal/integration`

**Step 5: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: make integration tests an explicit CI step

Closes #9"
```

---

### Task 14: Open PR 3

**Step 1: Push the branch**

```bash
git push -u origin feat/ci-quality
```

**Step 2: Open pull request**

```bash
gh pr create \
  --title "feat(ci): replace bubble sort with sort.Slice, add integration test CI step" \
  --body "Closes #7, #9

## Changes
- Replace O(n²) bubble sort in \`sortReleases\` with \`sort.Slice\` (O(n log n))
- Add explicit integration test step to CI so regressions are visibly tracked

## Test plan
- [ ] \`make test\` passes (including integration package)
- [ ] \`make vet\` passes" \
  --base main
```

**Step 3: After review and CI pass, merge**

```bash
git checkout main && git pull
```

---

## Summary

| PR | Branch | Issues | Key files |
|----|--------|--------|-----------|
| 1 | `fix/security` | #1 #2 #3 | `internal/storage/json.go`, `internal/api/handlers_admin.go` |
| 2 | `fix/bugs` | #4 #5 #6 #8 | `Dockerfile`, `internal/storage/json.go`, `.github/workflows/ci.yml` |
| 3 | `feat/ci-quality` | #7 #9 | `internal/update/service.go`, `.github/workflows/ci.yml` |

Run `make check` (format + vet + test) before opening each PR.