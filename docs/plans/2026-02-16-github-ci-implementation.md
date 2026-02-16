# GitHub CI Pipeline Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add GitHub Actions CI with format check, vet, test, coverage ratchet, OpenAPI validation, and gosec security scanning.

**Architecture:** Two parallel GitHub Actions jobs — `ci` (Docker-in-Docker, runs existing Make targets) and `security` (native Go, runs gosec). Coverage is enforced via a committed threshold file `.github/coverage-threshold`. Three new Make targets (`fmt-check`, `cover`, `security`) mirror CI behaviour locally.

**Tech Stack:** GitHub Actions, Docker-in-Docker, gosec (`securego/gosec`), `github/codeql-action` for SARIF upload, existing `golang:1.25-alpine` Docker image.

---

## Context

**Root Makefile** (`Makefile`) defines the `GO_DOCKER` variable used by all Go Make targets:

```makefile
GO_IMAGE   := golang:1.25-alpine
GO_MOD_CACHE   := $(APP_NAME)-go-mod-cache
GO_BUILD_CACHE := $(APP_NAME)-go-build-cache
GO_DOCKER  := docker run --rm \
    -v "$(CURDIR):/app" \
    -v "$(GO_MOD_CACHE):/go/pkg/mod" \
    -v "$(GO_BUILD_CACHE):/root/.cache/go-build" \
    -w /app \
    -e CGO_ENABLED=0 \
    $(GO_IMAGE)
```

Go targets live in `make/go.mk`. The `$(GO_DOCKER)` variable is available to all targets in `make/*.mk` via the root Makefile includes.

**Key constraint:** No CGO. `CGO_ENABLED=0` is already set in `GO_DOCKER`.

---

### Task 1: Add `fmt-check` Make target

**Files:**
- Modify: `make/go.mk`

**Step 1: Add the target**

Open `make/go.mk`. After the `fmt` target, add:

```makefile
fmt-check: ## Check formatting without modifying files
	$(GO_DOCKER) sh -c 'unformatted=$$(gofmt -l .); [ -z "$$unformatted" ] || (printf "Unformatted files:\n$$unformatted\n" && exit 1)'
```

Note the Makefile quoting rules: `$$` escapes `$` in Make, producing a single `$` in the shell. The `sh -c` wrapper is needed because `gofmt -l` is a shell pipeline.

Also add `fmt-check` to the `.PHONY` line at the top of `make/go.mk`.

**Step 2: Verify it passes on well-formatted code**

Run:
```bash
make fmt-check
```

Expected: exits 0 with no output (all files are already formatted).

**Step 3: Verify it fails on unformatted code**

Temporarily add a blank line in the wrong place in any `.go` file (e.g., add two blank lines where one is expected), then run:

```bash
make fmt-check
```

Expected: output like:
```
Unformatted files:
internal/api/handler.go
make: *** [fmt-check] Error 1
```

Revert the temporary change.

**Step 4: Commit**

```bash
git add make/go.mk
git commit -m "feat(make): add fmt-check target"
```

---

### Task 2: Add `cover` Make target

**Files:**
- Modify: `make/go.mk`

**Step 1: Add the target**

In `make/go.mk`, after the `test` target, add:

```makefile
cover: ## Run tests with coverage report
	$(GO_DOCKER) sh -c 'go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | grep ^total'
```

Add `cover` to the `.PHONY` line.

**Step 2: Run it and capture the output**

```bash
make cover
```

Expected output ends with a line like:
```
total:          (statements)    82.3%
```

(The exact percentage will vary. Note it down — you will need it in Task 3.)

**Step 3: Verify the output is parseable**

Run:
```bash
make cover | awk '/^total/ {gsub(/%/, "", $3); print $3}'
```

Expected: a bare float, e.g. `82.3`. This is the shell expression CI will use.

**Step 4: Add `coverage.out` to `.gitignore`**

The coverage profile file should not be committed. Add it:

```bash
echo "coverage.out" >> .gitignore
```

Verify:
```bash
git status
```

Expected: `coverage.out` not shown as untracked (or not present yet — either is fine).

**Step 5: Commit**

```bash
git add make/go.mk .gitignore
git commit -m "feat(make): add cover target"
```

---

### Task 3: Set the initial coverage threshold

**Files:**
- Create: `.github/coverage-threshold`

**Step 1: Run cover and get the percentage**

```bash
make cover | awk '/^total/ {gsub(/%/, "", $3); print $3}'
```

Note the output, e.g. `82.3`.

**Step 2: Write the threshold file**

Create `.github/coverage-threshold` with exactly the percentage value on a single line, no trailing newline:

```
82.3
```

(Replace `82.3` with the actual value from Step 1.)

**Step 3: Commit**

```bash
git add .github/coverage-threshold
git commit -m "ci: set initial coverage threshold"
```

---

### Task 4: Add `security` Make target

**Files:**
- Modify: `make/go.mk`

**Step 1: Add the target**

In `make/go.mk`, at the end, add:

```makefile
security: ## Run gosec security scanner
	docker run --rm \
		-v "$(CURDIR):/app" \
		-w /app \
		securego/gosec:latest \
		-severity high -confidence medium ./...
```

Note: this target uses its own `docker run` directly (not `$(GO_DOCKER)`) because it uses a different image (`securego/gosec:latest` instead of `golang:1.25-alpine`).

Add `security` to the `.PHONY` line.

**Step 2: Run it**

```bash
make security
```

Expected: either exits 0 with no findings, or prints any High severity + Medium/High confidence findings. If findings are printed, investigate each one — they may be legitimate issues to fix or false positives to suppress with `// #nosec G###`.

If any findings are legitimate, fix them now before continuing.

**Step 3: Commit**

```bash
git add make/go.mk
git commit -m "feat(make): add security target"
```

---

### Task 5: Create the GitHub Actions workflow

**Files:**
- Create: `.github/workflows/ci.yml`

**Step 1: Create the workflows directory**

```bash
mkdir -p .github/workflows
```

**Step 2: Write the workflow file**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Check formatting
        run: make fmt-check

      - name: Vet
        run: make vet

      - name: Test
        run: make test

      - name: Coverage
        run: |
          COVERAGE=$(make cover | awk '/^total/ {gsub(/%/, "", $3); print $3}')
          THRESHOLD=$(cat .github/coverage-threshold)
          echo "Coverage:  ${COVERAGE}%"
          echo "Threshold: ${THRESHOLD}%"
          awk "BEGIN {
            if (${COVERAGE} + 0 < ${THRESHOLD} + 0) {
              print \"FAIL: coverage \" ${COVERAGE} \"% is below threshold \" ${THRESHOLD} \"%\"
              exit 1
            }
            print \"PASS: coverage \" ${COVERAGE} \"% meets threshold \" ${THRESHOLD} \"%\"
          }"

      - name: Validate OpenAPI spec
        run: make openapi-validate

  security:
    runs-on: ubuntu-latest
    permissions:
      security-events: write
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.0'

      - name: Install gosec
        run: go install github.com/securego/gosec/v2/cmd/gosec@latest

      - name: Generate SARIF report
        run: gosec -no-fail -fmt sarif -out gosec.sarif ./...

      - name: Upload to GitHub Security tab
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: gosec.sarif

      - name: Enforce security threshold
        run: gosec -severity high -confidence medium ./...
```

**Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add GitHub Actions CI workflow"
```

---

### Task 6: Validate CI passes

**Step 1: Push to a branch and open a PR**

```bash
git checkout -b ci/setup
git push -u origin ci/setup
```

Then open a PR targeting `main` on GitHub.

**Step 2: Watch the Actions tab**

Navigate to the PR on GitHub → Actions tab. Both `ci` and `security` jobs should appear running in parallel.

Expected outcome for each job:

`ci` job:
- `Check formatting` — green
- `Vet` — green
- `Test` — green
- `Coverage` — green, prints `PASS: coverage X% meets threshold Y%`
- `Validate OpenAPI spec` — green

`security` job:
- `Install gosec` — green
- `Generate SARIF report` — green (always succeeds due to `-no-fail`)
- `Upload to GitHub Security tab` — green
- `Enforce security threshold` — green (no High + Medium/High findings)

**Step 3: If any job fails, investigate**

Common failure modes:

| Failure | Fix |
|---------|-----|
| `fmt-check` fails | Run `make fmt` locally, commit the result |
| `Cover` fails with parse error | Check `make cover` output manually; ensure the `total:` line is present |
| `Coverage` below threshold | Either increase test coverage or lower the threshold if the initial value was wrong |
| `gosec` threshold fails | Fix the finding or suppress with `// #nosec G###` if it is a false positive |
| `openapi-validate` fails | Fix the OpenAPI spec at `internal/api/openapi/openapi.yaml` |

**Step 4: Merge the PR**

Once all checks are green, merge via GitHub UI.

---

### Task 7: Update documentation

**Files:**
- Modify: `docs/plans/2026-02-16-github-ci-implementation.md` — already exists (this file)

No additional documentation is needed beyond the design doc (`docs/plans/2026-02-16-github-ci-design.md`) already committed. The Make targets are self-documenting via `## comments` visible in `make help`.