# CI Pipeline Hardening Implementation

**Date:** 2026-03-08
**Design:** [CI Pipeline Hardening Design](2026-03-08-ci-hardening-design.md)

## Summary

Replaced the single sequential `ci` job with six parallel jobs, added Go module caching via `actions/setup-go`, and added `sqlc-vet` to catch SQL query/schema mismatches on every PR.

## Changes

### `.github/workflows/ci.yml`

Replaced the `ci` job with four new focused jobs, all running in parallel alongside the existing `security` and `race` jobs:

| Old job | New jobs |
|---------|----------|
| `ci` (sequential: fmt-check, vet, cover, openapi-validate, integration-test) | `lint` (fmt-check, vet) |
| | `test` (unit coverage with 70% threshold) |
| | `integration` (integration tests) |
| | `spec` (openapi-validate, sqlc-vet) |
| `security` | `security` (unchanged) |
| `race` | `race` (unchanged) |

> `security` and `race` existed prior to this change and are not modified.

Key changes per job:

**`lint`**: Uses `actions/setup-go` (native Go with automatic module caching). Runs `gofmt -l` and `go vet ./...` directly instead of via Docker-wrapped make targets. Equivalent to `make fmt-check` and `make vet`.

**`test`**: Uses `actions/setup-go`. Runs `go list -tags integration` + `go test -tags integration -coverpkg` + `go tool cover` natively, mirroring `make cover` exactly. Applies the 70% threshold inline. Coverage includes integration tests.

**`integration`**: Uses `actions/setup-go`. Runs `go test -tags integration ./internal/integration/...` natively. Equivalent to `make integration-test`.

**`spec`**: No Go toolchain required. Calls `make openapi-validate` and `make sqlc-vet` directly; both use Docker images (Redocly CLI and sqlc respectively) which are available on ubuntu runners. The `sqlc-vet` step is new — it was not present in the original pipeline.

### `docs/ci.md`

New reference page covering: pipeline overview with Mermaid diagram, per-job details, Makefile correspondence, local equivalents, and branch protection configuration guidance.

### `mkdocs.yml`

Added `CI Pipeline: ci.md` to the main nav. Added CI hardening design and implementation docs to the Design Docs section.

## Behaviour Changes

- **Coverage scope**: unchanged. The `test` job applies `-tags integration` to both `go list` and `go test`, matching `make cover` exactly. Coverage includes integration tests.
- **`sqlc-vet` is now a required CI check**: SQL query/schema validation runs on every PR.
- **Go module caching**: `actions/setup-go` caches `$GOPATH/pkg/mod` and `~/.cache/go/build` per Go version and `go.sum` hash, reducing module download time on subsequent runs.
- **Job names changed**: the release workflow's CI verification step checks all runs dynamically by SHA, so the rename from `ci` to `lint`/`test`/`integration`/`spec` is automatically picked up.
