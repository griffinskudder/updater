# Cursor Sort Validation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add defence-in-depth validation of `SortBy` and `SortOrder` fields in `DecodeReleaseCursor` to prevent tampered cursors from producing incorrect pagination results.

**Architecture:** Extract shared valid-value variables from the inline slice in `ListReleasesRequest.Validate()`, then reuse them in `DecodeReleaseCursor` post-unmarshal validation. Both consumers are in the `models` package so variables stay unexported.

**Tech Stack:** Go 1.25, testify (assert/require)

---

### Task 1: Extract shared valid-value variables

**Files:**
- Modify: `internal/models/request.go:204-219`

**Step 1: Write the shared variables**

Add package-level variables above `ListReleasesRequest.Validate()`. Place them after the imports/constants section but before any function. Add them near the top of the file after the `import` block and existing package-level declarations:

```go
// validReleaseSortFields lists the permitted values for the sort_by field
// in release list requests and cursors.
var validReleaseSortFields = []string{"version", "release_date", "platform", "architecture", "created_at"}

// validSortOrders lists the permitted values for the sort_order field
// in release list requests and cursors.
var validSortOrders = []string{"asc", "desc"}
```

**Step 2: Replace the inline slice in `ListReleasesRequest.Validate()`**

Replace lines 204-220 of `internal/models/request.go`. The `SortOrder` check should use the shared variable, and the `SortBy` check should use `validReleaseSortFields` instead of the local `validSortFields`:

Before:
```go
	if r.SortOrder != "" && r.SortOrder != "asc" && r.SortOrder != "desc" {
		return errors.New("sort_order must be 'asc' or 'desc'")
	}

	validSortFields := []string{"version", "release_date", "platform", "architecture", "created_at"}
	if r.SortBy != "" {
		found := false
		for _, field := range validSortFields {
			if r.SortBy == field {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid sort_by field: %s", r.SortBy)
		}
	}
```

After:
```go
	if r.SortOrder != "" && !containsString(validSortOrders, r.SortOrder) {
		return errors.New("sort_order must be 'asc' or 'desc'")
	}

	if r.SortBy != "" && !containsString(validReleaseSortFields, r.SortBy) {
		return fmt.Errorf("invalid sort_by field: %s", r.SortBy)
	}
```

**Step 3: Add the `containsString` helper**

Add a small unexported helper at the bottom of `request.go` (or near the other unexported helpers like `isValidPlatform`):

```go
// containsString reports whether slice contains s.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
```

**Step 4: Run existing tests to verify no regressions**

Run: `make test`
Expected: All tests pass. The refactored `Validate()` behaves identically.

**Step 5: Commit**

```
fix(models): extract shared sort validation variables

Extract validReleaseSortFields and validSortOrders from the inline
slice in ListReleasesRequest.Validate() for reuse by cursor decoding.

Refs #44
```

---

### Task 2: Add SortBy/SortOrder validation to DecodeReleaseCursor (TDD)

**Files:**
- Modify: `internal/models/cursor_test.go`
- Modify: `internal/models/cursor.go:40-52`

**Step 1: Write the failing tests**

Add a table-driven test to `internal/models/cursor_test.go`:

```go
func TestDecodeReleaseCursor_InvalidSortBy(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		wantErr   string
	}{
		{
			name:      "invalid sort_by",
			sortBy:    "injected",
			sortOrder: "desc",
			wantErr:   "invalid cursor sort_by",
		},
		{
			name:      "empty sort_by",
			sortBy:    "",
			sortOrder: "desc",
			wantErr:   "invalid cursor sort_by",
		},
		{
			name:      "invalid sort_order",
			sortBy:    "release_date",
			sortOrder: "sideways",
			wantErr:   "invalid cursor sort_order",
		},
		{
			name:      "empty sort_order",
			sortBy:    "release_date",
			sortOrder: "",
			wantErr:   "invalid cursor sort_order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursor := &ReleaseCursor{
				SortBy:    tt.sortBy,
				SortOrder: tt.sortOrder,
				ID:        "test-id",
			}
			encoded, err := cursor.Encode()
			require.NoError(t, err)

			_, err = DecodeReleaseCursor(encoded)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestDecodeReleaseCursor_AllValidSortFields(t *testing.T) {
	sortFields := []string{"version", "release_date", "platform", "architecture", "created_at"}
	for _, field := range sortFields {
		t.Run(field, func(t *testing.T) {
			cursor := &ReleaseCursor{
				SortBy:    field,
				SortOrder: "desc",
				ID:        "test-id",
			}
			encoded, err := cursor.Encode()
			require.NoError(t, err)

			decoded, err := DecodeReleaseCursor(encoded)
			require.NoError(t, err)
			assert.Equal(t, field, decoded.SortBy)
		})
	}
}

func TestDecodeReleaseCursor_AllValidSortOrders(t *testing.T) {
	for _, order := range []string{"asc", "desc"} {
		t.Run(order, func(t *testing.T) {
			cursor := &ReleaseCursor{
				SortBy:    "release_date",
				SortOrder: order,
				ID:        "test-id",
			}
			encoded, err := cursor.Encode()
			require.NoError(t, err)

			decoded, err := DecodeReleaseCursor(encoded)
			require.NoError(t, err)
			assert.Equal(t, order, decoded.SortOrder)
		})
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `make test`
Expected: `TestDecodeReleaseCursor_InvalidSortBy` FAILS (no validation yet, invalid cursors decode successfully). The two valid-value tests should PASS.

**Step 3: Implement validation in `DecodeReleaseCursor`**

Update `internal/models/cursor.go:40-52`. Add validation after the JSON unmarshal:

```go
// DecodeReleaseCursor deserialises a cursor produced by ReleaseCursor.Encode.
// Returns an error if the string is not valid base64, not valid JSON, or
// contains invalid sort_by / sort_order values.
func DecodeReleaseCursor(s string) (*ReleaseCursor, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}
	var c ReleaseCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}
	if !containsString(validReleaseSortFields, c.SortBy) {
		return nil, fmt.Errorf("invalid cursor sort_by: %q", c.SortBy)
	}
	if !containsString(validSortOrders, c.SortOrder) {
		return nil, fmt.Errorf("invalid cursor sort_order: %q", c.SortOrder)
	}
	return &c, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `make test`
Expected: All tests pass.

**Step 5: Run full pre-commit check**

Run: `make check`
Expected: Format, vet, and all tests pass.

**Step 6: Commit**

```
fix(models): validate SortBy and SortOrder in DecodeReleaseCursor

Add defence-in-depth validation that rejects cursors with unrecognised
sort_by or sort_order values. Prevents tampered cursors from reaching
the storage layer switch statement and producing incorrect pagination.

Fixes #44
```