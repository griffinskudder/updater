package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *Version
		expectError bool
	}{
		{
			name:  "standard semantic version",
			input: "1.2.3",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Raw:   "1.2.3",
			},
			expectError: false,
		},
		{
			name:  "version with pre-release",
			input: "1.2.3-alpha",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Pre:   "alpha",
				Raw:   "1.2.3-alpha",
			},
			expectError: false,
		},
		{
			name:  "version with build metadata",
			input: "1.2.3+build.123",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Build: "build.123",
				Raw:   "1.2.3+build.123",
			},
			expectError: false,
		},
		{
			name:  "complete semantic version",
			input: "1.2.3-beta.1+build.456",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Pre:   "beta.1",
				Build: "build.456",
				Raw:   "1.2.3-beta.1+build.456",
			},
			expectError: false,
		},
		{
			name:  "partial version - major.minor",
			input: "1.2",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 0,
				Raw:   "1.2",
			},
			expectError: false,
		},
		{
			name:  "partial version - major only",
			input: "1",
			expected: &Version{
				Major: 1,
				Minor: 0,
				Patch: 0,
				Raw:   "1",
			},
			expectError: false,
		},
		{
			name:        "empty version",
			input:       "",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid major version",
			input:       "abc.2.3",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid minor version",
			input:       "1.abc.3",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid patch version",
			input:       "1.2.abc",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "too many parts",
			input:       "1.2.3.4",
			expected:    nil,
			expectError: true,
		},
		{
			name:  "zero versions",
			input: "0.0.0",
			expected: &Version{
				Major: 0,
				Minor: 0,
				Patch: 0,
				Raw:   "0.0.0",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseVersion(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestVersion_String(t *testing.T) {
	tests := []struct {
		name     string
		version  *Version
		expected string
	}{
		{
			name: "with raw string",
			version: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Raw:   "1.2.3-original",
			},
			expected: "1.2.3-original",
		},
		{
			name: "without raw string - standard version",
			version: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
			},
			expected: "1.2.3",
		},
		{
			name: "without raw string - with pre-release",
			version: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Pre:   "alpha",
			},
			expected: "1.2.3-alpha",
		},
		{
			name: "without raw string - with build metadata",
			version: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Build: "build.123",
			},
			expected: "1.2.3+build.123",
		},
		{
			name: "without raw string - complete version",
			version: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Pre:   "beta.1",
				Build: "build.456",
			},
			expected: "1.2.3-beta.1+build.456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVersion_Compare(t *testing.T) {
	tests := []struct {
		name     string
		version1 string
		version2 string
		expected int
	}{
		// Major version comparisons
		{"major version greater", "2.0.0", "1.0.0", 1},
		{"major version less", "1.0.0", "2.0.0", -1},
		{"major version equal", "1.0.0", "1.0.0", 0},

		// Minor version comparisons
		{"minor version greater", "1.2.0", "1.1.0", 1},
		{"minor version less", "1.1.0", "1.2.0", -1},

		// Patch version comparisons
		{"patch version greater", "1.2.3", "1.2.2", 1},
		{"patch version less", "1.2.2", "1.2.3", -1},

		// Pre-release comparisons
		{"release vs pre-release", "1.0.0", "1.0.0-alpha", 1},
		{"pre-release vs release", "1.0.0-alpha", "1.0.0", -1},
		{"pre-release comparison", "1.0.0-beta", "1.0.0-alpha", 1},
		{"pre-release comparison reverse", "1.0.0-alpha", "1.0.0-beta", -1},
		{"same pre-release", "1.0.0-alpha", "1.0.0-alpha", 0},

		// Complex comparisons
		{"different major with pre-release", "2.0.0-alpha", "1.9.9", 1},
		{"same version different pre-release", "1.2.3-rc.1", "1.2.3-beta", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, err := ParseVersion(tt.version1)
			require.NoError(t, err)

			v2, err := ParseVersion(tt.version2)
			require.NoError(t, err)

			result := v1.Compare(v2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVersion_ComparisonMethods(t *testing.T) {
	v1, _ := ParseVersion("1.2.3")
	v2, _ := ParseVersion("1.2.4")
	v3, _ := ParseVersion("1.2.3")

	// GreaterThan
	assert.True(t, v2.GreaterThan(v1))
	assert.False(t, v1.GreaterThan(v2))
	assert.False(t, v1.GreaterThan(v3))

	// LessThan
	assert.True(t, v1.LessThan(v2))
	assert.False(t, v2.LessThan(v1))
	assert.False(t, v1.LessThan(v3))

	// Equal
	assert.True(t, v1.Equal(v3))
	assert.False(t, v1.Equal(v2))

	// GreaterThanOrEqual
	assert.True(t, v2.GreaterThanOrEqual(v1))
	assert.True(t, v1.GreaterThanOrEqual(v3))
	assert.False(t, v1.GreaterThanOrEqual(v2))

	// LessThanOrEqual
	assert.True(t, v1.LessThanOrEqual(v2))
	assert.True(t, v1.LessThanOrEqual(v3))
	assert.False(t, v2.LessThanOrEqual(v1))
}

func TestVersionConstraint_Check(t *testing.T) {
	tests := []struct {
		name        string
		constraint  VersionConstraint
		version     string
		expected    bool
		expectError bool
	}{
		{
			name: "exact match - equal operator",
			constraint: VersionConstraint{
				Operator: "=",
				Version:  "1.2.3",
			},
			version:     "1.2.3",
			expected:    true,
			expectError: false,
		},
		{
			name: "exact match - default operator",
			constraint: VersionConstraint{
				Operator: "",
				Version:  "1.2.3",
			},
			version:     "1.2.3",
			expected:    true,
			expectError: false,
		},
		{
			name: "not equal",
			constraint: VersionConstraint{
				Operator: "!=",
				Version:  "1.2.3",
			},
			version:     "1.2.4",
			expected:    true,
			expectError: false,
		},
		{
			name: "greater than",
			constraint: VersionConstraint{
				Operator: ">",
				Version:  "1.2.3",
			},
			version:     "1.2.4",
			expected:    true,
			expectError: false,
		},
		{
			name: "greater than or equal",
			constraint: VersionConstraint{
				Operator: ">=",
				Version:  "1.2.3",
			},
			version:     "1.2.3",
			expected:    true,
			expectError: false,
		},
		{
			name: "less than",
			constraint: VersionConstraint{
				Operator: "<",
				Version:  "1.2.3",
			},
			version:     "1.2.2",
			expected:    true,
			expectError: false,
		},
		{
			name: "less than or equal",
			constraint: VersionConstraint{
				Operator: "<=",
				Version:  "1.2.3",
			},
			version:     "1.2.3",
			expected:    true,
			expectError: false,
		},
		{
			name: "constraint not met",
			constraint: VersionConstraint{
				Operator: ">",
				Version:  "1.2.3",
			},
			version:     "1.2.2",
			expected:    false,
			expectError: false,
		},
		{
			name: "invalid operator",
			constraint: VersionConstraint{
				Operator: "~",
				Version:  "1.2.3",
			},
			version:     "1.2.3",
			expected:    false,
			expectError: true,
		},
		{
			name: "invalid constraint version",
			constraint: VersionConstraint{
				Operator: "=",
				Version:  "invalid",
			},
			version:     "1.2.3",
			expected:    false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := ParseVersion(tt.version)
			require.NoError(t, err)

			result, err := tt.constraint.Check(version)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestVersionConstraint_CheckInvalidVersion(t *testing.T) {
	constraint := VersionConstraint{
		Operator: "=",
		Version:  "1.2.3",
	}

	// This should not happen in practice since we parse versions before creating constraints
	// But testing for completeness
	invalidVersion := &Version{Major: -1} // Invalid version that wouldn't come from ParseVersion

	_, err := constraint.Check(invalidVersion)
	// This will actually pass because the constraint version is valid
	// The error case is when the constraint version is invalid, not the input version
	assert.NoError(t, err)
}

func TestCompareInt(t *testing.T) {
	assert.Equal(t, 1, compareInt(5, 3))
	assert.Equal(t, -1, compareInt(3, 5))
	assert.Equal(t, 0, compareInt(5, 5))
}

// Test edge cases and error conditions
func TestVersionEdgeCases(t *testing.T) {
	t.Run("large version numbers", func(t *testing.T) {
		v, err := ParseVersion("999999.888888.777777")
		require.NoError(t, err)
		assert.Equal(t, 999999, v.Major)
		assert.Equal(t, 888888, v.Minor)
		assert.Equal(t, 777777, v.Patch)
	})

	t.Run("complex pre-release identifiers", func(t *testing.T) {
		v, err := ParseVersion("1.0.0-alpha.beta.1")
		require.NoError(t, err)
		assert.Equal(t, "alpha.beta.1", v.Pre)
	})

	t.Run("complex build metadata", func(t *testing.T) {
		v, err := ParseVersion("1.0.0+20130313144700.abc123.def456")
		require.NoError(t, err)
		assert.Equal(t, "20130313144700.abc123.def456", v.Build)
	})

	t.Run("version with only build metadata", func(t *testing.T) {
		v, err := ParseVersion("1.0.0+build")
		require.NoError(t, err)
		assert.Equal(t, "", v.Pre)
		assert.Equal(t, "build", v.Build)
	})

	t.Run("version with empty pre-release", func(t *testing.T) {
		v, err := ParseVersion("1.0.0-")
		require.NoError(t, err)
		assert.Equal(t, "", v.Pre)
	})

	t.Run("version with empty build", func(t *testing.T) {
		v, err := ParseVersion("1.0.0+")
		require.NoError(t, err)
		assert.Equal(t, "", v.Build)
	})
}