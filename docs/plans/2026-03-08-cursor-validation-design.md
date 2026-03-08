# Cursor Validation Design

**Date:** 2026-03-08
**Issue:** [#44](https://github.com/griffinskudder/updater/issues/44) -- Validate SortBy and SortOrder fields in DecodeReleaseCursor

## Problem

`DecodeReleaseCursor` validates base64 encoding and JSON structure but does not validate that `SortBy` and `SortOrder` contain permitted values. A tampered cursor with an unrecognised `SortBy` reaches the storage `switch` statement and falls through to the `default` case, producing incorrect pagination results with no error.

The service layer has two guards (request validation and cursor/request mismatch check) that make exploitation unlikely, but defence in depth requires validation at the cursor decoding boundary itself.

## Scope

- `DecodeReleaseCursor` only. `DecodeApplicationCursor` has no `SortBy`/`SortOrder` fields.
- No user-facing API changes. No docs changes required.

## Approach

Extract shared valid-value variables used by both `ListReleasesRequest.Validate()` and `DecodeReleaseCursor`, eliminating duplication.

### Changes

1. **`request.go`** -- Define package-level unexported variables:
   - `validReleaseSortFields = []string{"version", "release_date", "platform", "architecture", "created_at"}`
   - `validSortOrders = []string{"asc", "desc"}`
   - Replace the inline `validSortFields` slice in `ListReleasesRequest.Validate()` with `validReleaseSortFields`.

2. **`cursor.go`** -- After JSON unmarshal in `DecodeReleaseCursor`, validate:
   - `SortBy` is in `validReleaseSortFields`
   - `SortOrder` is in `validSortOrders`
   - Return descriptive error on mismatch.

3. **`cursor_test.go`** -- Table-driven tests:
   - Invalid `SortBy` returns error
   - Invalid `SortOrder` returns error
   - All valid `SortBy` values succeed
   - Both valid `SortOrder` values succeed

## Non-goals

- Changing `DecodeApplicationCursor` (no sort fields to validate).
- Changing storage layer switch statements (already guarded by this fix).
- Exporting the valid-value variables (both consumers are in the same package).