# Keyset Pagination Design

**Date:** 2026-03-07
**Issue:** [#40](https://github.com/griffinskudder/updater/issues/40)
**Status:** Approved

## Background

Both list endpoints (`GET /updates/{app_id}/releases`, `GET /applications`) support
offset-based pagination via `limit` and `offset` query parameters. Two gaps remain:

1. No server-side maximum limit enforcement — a client can request arbitrarily large pages.
2. Offset pagination degrades at scale — `LIMIT n OFFSET m` requires the database to scan
   and discard the first `m` rows on every request.

Breaking changes to the API are acceptable.

## Decision

Replace offset pagination with keyset (cursor) pagination on both list endpoints. Enforce
a maximum page size of 500, returning 400 if exceeded.

## Max Limit Enforcement

A constant `MaxPageSize = 500` is added to the `models` package. Both
`ListReleasesRequest.Validate()` and the new `ListApplicationsRequest.Validate()` return a
400 error if `Limit > MaxPageSize`. The default limit (when unset) remains 50.

## Cursor Design

Cursors are opaque base64-encoded JSON strings. They are produced by the server and consumed
by clients as-is. Clients must not construct or modify cursors.

No HMAC signing is applied. All cursor values are parameterised query arguments (no SQL
injection risk), `application_id` is taken from the URL path (no cross-app leakage), and the
cursor encodes only values already present in the response body (no information disclosure).
Cursor tampering produces unexpected page boundaries but no security violation.

### Release cursor

```json
{
  "sort_by": "release_date",
  "sort_order": "desc",
  "id": "<uuid>",
  "release_date": "<RFC3339>",
  "version_major": 0,
  "version_minor": 0,
  "version_patch": 0,
  "version_is_stable": true,
  "version_pre_release": "",
  "platform": "",
  "architecture": "",
  "created_at": "<RFC3339>"
}
```

All sort-field values are always encoded. Only the field matching `sort_by` is used for the
keyset comparison; the rest are zero values. The cursor is validated against the current
request's `sort_by` and `sort_order` — a mismatch returns 400.

### Application cursor

```json
{
  "created_at": "<RFC3339>",
  "id": "<string>"
}
```

Applications are sorted by `created_at DESC, id DESC`.

## Keyset WHERE Clause

The dynamic SQL builder in each storage implementation appends a keyset condition to the
existing WHERE clause when a cursor is present. The condition is parameterised.

### Scalar sort fields (release_date, platform, architecture, created_at)

For `sort_order=desc`:
```sql
AND ((col < $cursor_val) OR (col = $cursor_val AND id < $cursor_id))
```

For `sort_order=asc`:
```sql
AND ((col > $cursor_val) OR (col = $cursor_val AND id > $cursor_id))
```

### Version sort (always DESC, sort_order is ignored — existing behaviour)

Version ordering uses the stored numeric columns. The keyset condition uses the expanded
multi-condition form to handle mixed sort directions within the composite key:

```sql
AND (
  version_major < $c_major
  OR (version_major = $c_major AND version_minor < $c_minor)
  OR (version_major = $c_major AND version_minor = $c_minor AND version_patch < $c_patch)
  OR (version_major = $c_major AND version_minor = $c_minor AND version_patch = $c_patch
      AND is_stable < $c_is_stable)
  OR (version_major = $c_major AND version_minor = $c_minor AND version_patch = $c_patch
      AND is_stable = $c_is_stable
      AND COALESCE(version_pre_release, '') > $c_pre_release)
  OR (version_major = $c_major AND version_minor = $c_minor AND version_patch = $c_patch
      AND is_stable = $c_is_stable
      AND COALESCE(version_pre_release, '') = $c_pre_release
      AND id < $c_id)
)
```

Where `is_stable` is `CASE WHEN version_pre_release IS NULL THEN 1 ELSE 0 END`.

## Response Shape

Both list responses adopt the same pagination envelope. `page`, `page_size`, and `has_more`
are removed. `next_cursor` is added.

```json
{
  "releases": [...],
  "total_count": 42,
  "next_cursor": "<opaque string>"
}
```

`next_cursor` is populated whenever more results exist (including the first page).
`next_cursor` is an empty string when the result set is exhausted.

`total_count` is retained and continues to use `COUNT(*) OVER()` — it reflects the total
number of rows matching the filters, independent of the cursor.

## API Changes

### GET /updates/{app_id}/releases

| Parameter | Change |
|-----------|--------|
| `limit` | Max 500; 400 if exceeded |
| `offset` | Removed |
| `after` | New — opaque cursor string |

### GET /applications

| Parameter | Change |
|-----------|--------|
| `limit` | Max 500; 400 if exceeded |
| `offset` | Removed |
| `after` | New — opaque cursor string |

## Layers Changed

| Layer | Change |
|-------|--------|
| `internal/models/request.go` | Add `MaxPageSize`; add `After` to `ListReleasesRequest`; add `ListApplicationsRequest` |
| `internal/models/response.go` | Drop `Page`, `PageSize`, `HasMore`; add `NextCursor` to both list responses |
| `internal/models/cursor.go` | New file — `ReleaseCursor` and `ApplicationCursor` structs with encode/decode |
| `internal/update/service.go` | Decode cursor; build `NextCursor` from last result; change `ListApplications` signature |
| `internal/update/interface.go` | Update `ListApplications` signature |
| `internal/storage/interface.go` | Replace `offset int` with cursor params on both paged methods |
| `internal/storage/postgres.go` | Extend dynamic SQL builder with keyset WHERE |
| `internal/storage/sqlite.go` | Extend dynamic SQL builder with keyset WHERE |
| `internal/storage/memory.go` | Cursor-based slicing in Go |
| `internal/api/handlers.go` | Parse `after` param; remove `offset` parsing |
| `internal/api/handlers_applications.go` | Use `ListApplicationsRequest` |
| `internal/api/openapi/openapi.yaml` | Update both list endpoints |

## Error Handling

| Condition | Status | Code |
|-----------|--------|------|
| `limit > 500` | 400 | `INVALID_REQUEST` |
| `limit < 0` | 400 | `INVALID_REQUEST` |
| Cursor malformed (bad base64 or JSON) | 400 | `INVALID_REQUEST` |
| Cursor `sort_by`/`sort_order` mismatch with request | 400 | `INVALID_REQUEST` |

## Testing

- **Cursor**: encode/decode round-trip; mismatch detection; malformed input returns error
- **Models**: `ListApplicationsRequest` Validate/Normalize; max limit on both request types
- **Storage**: keyset correctness for each of the 5 release sort fields and application
  `created_at`; no duplicates or gaps across pages; empty cursor returns first page; cursor
  from last item produces empty next page
- **Service**: `NextCursor` populated when results remain; empty on last page; cursor
  validated before storage call
- **Handler**: 400 on `limit > 500`; 400 on malformed cursor; updated response shape