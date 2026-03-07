# Security Fixes: Swagger UI SRI and Bootstrap Key Exposure

Date: 2026-03-08
Issues: #51, #52

## Overview

Two small, independent security fixes.

## Issue #51: Swagger UI CDN Assets Without Subresource Integrity

### Problem

`internal/api/handlers_openapi.go` loads `swagger-ui.css` and `swagger-ui-bundle.js` from `https://unpkg.com/swagger-ui-dist@5/` with no `integrity` attribute. If unpkg.com is compromised or the package is tampered with, malicious scripts execute in the context of users entering bearer tokens into the Swagger UI.

### Fix

Pin both assets to a specific `swagger-ui-dist@5.x.y` patch version and add `integrity="sha384-..."` and `crossorigin="anonymous"` attributes to the `<link>` and `<script>` tags in the `swaggerUIHTML` constant.

SRI hashes are computed as base64-encoded SHA-384 digests of the exact files served by unpkg at the pinned version URL, using `openssl dgst -sha384 -binary | base64`.

No test changes required. The handler test already covers correct HTTP response behaviour.

## Issue #52: Bootstrap Key JSON Serialization and File Permissions

### Problem

Two independent defects:

1. `SecurityConfig.BootstrapKey` in `internal/models/config.go` is tagged `json:"bootstrap_key"`. Any JSON serialization of a `Config` struct (debug endpoints, audit log entries, config dumps) would expose the bootstrap key in plaintext.

2. `SaveExample` in `internal/config/config.go` writes the generated config file with `0644` permissions (world-readable), rather than `0600` (owner-only).

### Fix

1. Change the struct tag from `json:"bootstrap_key"` to `json:"-"`. The `yaml:"bootstrap_key"` tag is unchanged, so YAML config file loading is unaffected. JSON deserialization of the bootstrap key (not used anywhere in the codebase) is also dropped, but this is intentional — the key should only ever arrive via YAML file or environment variable.

2. Change `os.WriteFile(filePath, data, 0644)` to `os.WriteFile(filePath, data, 0600)` in `SaveExample`.

No test changes required. Existing tests load config from YAML and inspect Go struct fields directly — neither path is affected by the JSON tag change.

## Files to Modify

| File | Change |
|------|--------|
| `internal/api/handlers_openapi.go` | Pin Swagger UI version; add `integrity` and `crossorigin` attributes |
| `internal/models/config.go` | `json:"bootstrap_key"` -> `json:"-"` |
| `internal/config/config.go` | `0644` -> `0600` in `SaveExample` |
