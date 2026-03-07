package storage

import (
	"reflect"
	"testing"
	"updater/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalUnmarshalPlatforms(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{name: "multiple platforms", input: []string{"windows", "linux", "darwin"}, expected: []string{"windows", "linux", "darwin"}},
		{name: "single platform", input: []string{"windows"}, expected: []string{"windows"}},
		{name: "empty slice", input: []string{}, expected: []string{}},
		{name: "nil slice", input: nil, expected: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := marshalPlatforms(tt.input)
			if err != nil {
				t.Fatalf("marshalPlatforms error: %v", err)
			}

			result, err := unmarshalPlatforms(data)
			if err != nil {
				t.Fatalf("unmarshalPlatforms error: %v", err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestUnmarshalPlatformsFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{name: "valid JSON array", input: `["windows","linux"]`, expected: []string{"windows", "linux"}},
		{name: "empty array", input: `[]`, expected: []string{}},
		{name: "empty string", input: "", expected: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := unmarshalPlatformsFromString(tt.input)
			if err != nil {
				t.Fatalf("unmarshalPlatformsFromString error: %v", err)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMarshalUnmarshalConfig(t *testing.T) {
	tests := []struct {
		name  string
		input models.ApplicationConfig
	}{
		{
			name: "config with custom fields",
			input: models.ApplicationConfig{
				CustomFields: map[string]string{"key": "value"},
			},
		},
		{
			name:  "empty config",
			input: models.ApplicationConfig{},
		},
		{
			name: "config with nil custom fields",
			input: models.ApplicationConfig{
				CustomFields: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := marshalConfig(tt.input)
			if err != nil {
				t.Fatalf("marshalConfig error: %v", err)
			}

			result, err := unmarshalConfig(data)
			if err != nil {
				t.Fatalf("unmarshalConfig error: %v", err)
			}

			if len(tt.input.CustomFields) > 0 {
				if !reflect.DeepEqual(result.CustomFields, tt.input.CustomFields) {
					t.Errorf("CustomFields: expected %v, got %v", tt.input.CustomFields, result.CustomFields)
				}
			}
		})
	}
}

func TestUnmarshalConfigFromString(t *testing.T) {
	result, err := unmarshalConfigFromString(`{"custom_fields":{"env":"staging"}}`)
	if err != nil {
		t.Fatalf("unmarshalConfigFromString error: %v", err)
	}
	if result.CustomFields["env"] != "staging" {
		t.Errorf("expected custom_fields.env=staging, got %v", result.CustomFields)
	}
}

func TestMarshalUnmarshalMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "with data",
			input:    map[string]string{"key": "value", "another": "entry"},
			expected: map[string]string{"key": "value", "another": "entry"},
		},
		{
			name:     "empty map",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name:     "nil map",
			input:    nil,
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := marshalMetadata(tt.input)
			if err != nil {
				t.Fatalf("marshalMetadata error: %v", err)
			}

			result, err := unmarshalMetadata(data)
			if err != nil {
				t.Fatalf("unmarshalMetadata error: %v", err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestUnmarshalMetadataFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{name: "valid JSON", input: `{"key":"value"}`, expected: map[string]string{"key": "value"}},
		{name: "empty object", input: `{}`, expected: map[string]string{}},
		{name: "empty string", input: "", expected: make(map[string]string)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := unmarshalMetadataFromString(tt.input)
			if err != nil {
				t.Fatalf("unmarshalMetadataFromString error: %v", err)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestUnmarshalPlatformsInvalidJSON(t *testing.T) {
	_, err := unmarshalPlatforms([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestUnmarshalConfigInvalidJSON(t *testing.T) {
	_, err := unmarshalConfig([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestUnmarshalMetadataInvalidJSON(t *testing.T) {
	_, err := unmarshalMetadata([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMarshalPermissions(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"nil slice", nil, "[]"},
		{"empty slice", []string{}, "[]"},
		{"single", []string{"read"}, `["read"]`},
		{"multiple", []string{"read", "write"}, `["read","write"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := marshalPermissions(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUnmarshalPermissions(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"empty string", "", []string{}, false},
		{"empty array", "[]", []string{}, false},
		{"single", `["read"]`, []string{"read"}, false},
		{"multiple", `["read","write"]`, []string{"read", "write"}, false},
		{"invalid json", "not-json", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unmarshalPermissions(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseSemverParts(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantMajor int64
		wantMinor int64
		wantPatch int64
		wantPre   string
	}{
		{
			name:      "stable version",
			version:   "1.5.0",
			wantMajor: 1,
			wantMinor: 5,
			wantPatch: 0,
			wantPre:   "",
		},
		{
			name:      "pre-release version",
			version:   "2.3.4-beta.1",
			wantMajor: 2,
			wantMinor: 3,
			wantPatch: 4,
			wantPre:   "beta.1",
		},
		{
			name:      "invalid version returns zeros",
			version:   "not-a-version",
			wantMajor: 0,
			wantMinor: 0,
			wantPatch: 0,
			wantPre:   "",
		},
		{
			name:      "empty string returns zeros",
			version:   "",
			wantMajor: 0,
			wantMinor: 0,
			wantPatch: 0,
			wantPre:   "",
		},
		{
			name:      "alpha pre-release",
			version:   "0.1.0-alpha",
			wantMajor: 0,
			wantMinor: 1,
			wantPatch: 0,
			wantPre:   "alpha",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, patch, pre := parseSemverParts(tt.version)
			assert.Equal(t, tt.wantMajor, major, "major")
			assert.Equal(t, tt.wantMinor, minor, "minor")
			assert.Equal(t, tt.wantPatch, patch, "patch")
			assert.Equal(t, tt.wantPre, pre, "preRelease")
		})
	}
}
