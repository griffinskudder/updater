package storage

import (
	"reflect"
	"testing"
	"updater/internal/models"
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
			name: "full config",
			input: models.ApplicationConfig{
				AutoUpdate:     true,
				UpdateInterval: 3600,
				MinVersion:     "1.0.0",
				CustomFields:   map[string]string{"key": "value"},
			},
		},
		{
			name:  "empty config",
			input: models.ApplicationConfig{},
		},
		{
			name: "config with nil custom fields",
			input: models.ApplicationConfig{
				AutoUpdate: false,
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

			if result.AutoUpdate != tt.input.AutoUpdate {
				t.Errorf("AutoUpdate: expected %v, got %v", tt.input.AutoUpdate, result.AutoUpdate)
			}
			if result.UpdateInterval != tt.input.UpdateInterval {
				t.Errorf("UpdateInterval: expected %v, got %v", tt.input.UpdateInterval, result.UpdateInterval)
			}
		})
	}
}

func TestUnmarshalConfigFromString(t *testing.T) {
	result, err := unmarshalConfigFromString(`{"auto_update":true,"update_interval":7200}`)
	if err != nil {
		t.Fatalf("unmarshalConfigFromString error: %v", err)
	}
	if !result.AutoUpdate {
		t.Error("expected AutoUpdate to be true")
	}
	if result.UpdateInterval != 7200 {
		t.Errorf("expected UpdateInterval 7200, got %d", result.UpdateInterval)
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
