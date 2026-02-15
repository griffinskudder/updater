# Makefile Restructure Design

**Date:** 2026-02-15
**Status:** Approved

## Problem

The Makefile is a single 163-line file that will continue growing as new target categories are added. This makes targets hard to find, the file hard to maintain, and help text must be kept in sync manually.

## Goals

1. Improve readability by splitting targets into category-based files
2. Transition all possible commands to run inside Docker containers for consistency and reduced local dependencies
3. Auto-generate help output so it stays in sync as targets are added

## Design

### File Structure

```
Makefile              -- Shared variables, includes, help target (~30 lines)
make/
  go.mk              -- Go development: build, test, fmt, vet, clean, tidy, check
  docs.mk            -- Documentation: docs-serve, docs-build, docs-clean
  docker.mk          -- Docker operations: docker-build, docker-scan, docker-run,
                         docker-dev, docker-prod, docker-obs-up, docker-obs-down,
                         docker-clean, docker-push
  db.mk              -- Database: sqlc-generate, sqlc-vet
```

The root Makefile uses `include make/*.mk` to pull in all category files automatically. New categories are added by creating a new `.mk` file -- no changes to the root Makefile needed.

### Docker-first Targets

Default targets run inside Docker containers. Only Docker is required on the host -- no local Go, sqlc, or other tooling needed.

**Go targets** run via a shared `GO_DOCKER` variable:

```makefile
GO_IMAGE       := golang:1.25-alpine
GO_MOD_CACHE   := updater-go-mod-cache
GO_BUILD_CACHE := updater-go-build-cache
GO_DOCKER      := docker run --rm \
    -v "$(CURDIR):/app" \
    -v "$(GO_MOD_CACHE):/go/pkg/mod" \
    -v "$(GO_BUILD_CACHE):/root/.cache/go-build" \
    -w /app \
    -e CGO_ENABLED=0 \
    --user "$(shell id -u):$(shell id -g)" \
    $(GO_IMAGE)
```

Target mapping:

| Target | Container Command |
|--------|-------------------|
| `build` | `$(GO_DOCKER) go build -o bin/updater ./cmd/updater` |
| `test` | `$(GO_DOCKER) go test ./...` |
| `fmt` | `$(GO_DOCKER) go fmt ./...` |
| `vet` | `$(GO_DOCKER) go vet ./...` |
| `tidy` | `$(GO_DOCKER) go mod tidy` |
| `check` | `fmt` + `vet` + `test` (all containerized) |
| `clean` | `rm -rf bin` (local, no container) |

**sqlc targets** run via the official sqlc Docker image:

| Target | Container Command |
|--------|-------------------|
| `sqlc-generate` | `docker run --rm -v "$(CURDIR):/src" -w /src sqlc/sqlc:latest generate` |
| `sqlc-vet` | `docker run --rm -v "$(CURDIR):/src" -w /src sqlc/sqlc:latest vet` |

**Docs targets** already use Docker (squidfunk/mkdocs-material). No change needed.

**Docker ops targets** inherently use Docker. No change needed.

### Volume Caching

Two named Docker volumes persist caches between runs:

- `updater-go-mod-cache` -- Go module downloads (`/go/pkg/mod`)
- `updater-go-build-cache` -- Go build cache (`/root/.cache/go-build`)

This ensures repeated runs of `make test` or `make build` are fast after the first invocation.

### Auto-documenting Help

Each target is annotated with `## Description` on the target line. Category headers use `##@` comments. The help target parses all included files:

```makefile
.DEFAULT_GOAL := help

help: ## Show this help
	@grep -h '##' $(MAKEFILE_LIST) | grep -v grep | awk -F ':.*##' '{printf "  %-18s %s\n", $$1, $$2}'
```

Example output:

```
Go Development:
  build              Build the application to bin/updater
  test               Run tests
  fmt                Format code
  vet                Vet code for issues
  clean              Clean build artifacts
  tidy               Tidy dependencies
  check              Run format, vet, and test

Documentation:
  docs-serve         Start MkDocs development server
  docs-build         Build documentation site
  docs-clean         Clean documentation build artifacts

Docker Operations:
  docker-build       Build secure Docker image
  docker-scan        Scan Docker image for vulnerabilities
  docker-run         Run container with security defaults
  docker-dev         Start development environment
  docker-prod        Run with production configuration
  docker-obs-up      Start observability stack
  docker-obs-down    Stop observability stack
  docker-clean       Clean Docker artifacts
  docker-push        Build and push Docker image

Database:
  sqlc-generate      Generate Go code from SQL schemas
  sqlc-vet           Validate SQL schemas and queries
```

### Error Handling

**Docker guard** at the top of the root Makefile:

```makefile
DOCKER := $(shell command -v docker 2>/dev/null)
ifndef DOCKER
    $(error "Docker is required but not found. Install Docker to use this Makefile.")
endif
```

**File ownership:** The `--user "$(shell id -u):$(shell id -g)"` flag ensures container-created files (e.g., `bin/updater`) match the host user, avoiding root-owned files on Linux.

### Verification

Since this is a Makefile restructure (not Go code), verification is a manual checklist:

1. Every target runs successfully
2. Help output lists all targets with correct descriptions and groupings
3. Second run of `make test` is faster than the first (cache hit)
4. Clear error message when Docker is unavailable
5. `make clean` removes build artifacts

### Documentation

- New page at `docs/makefile.md` covering the `make/` structure, Docker-first philosophy, how to add targets, prerequisites, and cache management
- Added to `mkdocs.yml` nav
- CLAUDE.md "Development Commands" section updated to reflect new structure