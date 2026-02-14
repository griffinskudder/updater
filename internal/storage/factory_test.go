package storage

import (
	"os"
	"path/filepath"
	"testing"
	"updater/internal/models"
)

func TestFactory(t *testing.T) {
	factory := NewFactory()

	t.Run("GetSupportedProviders", func(t *testing.T) {
		providers := factory.GetSupportedProviders()
		expected := []string{"json", "memory", "postgres", "sqlite"}

		if len(providers) != len(expected) {
			t.Errorf("Expected %d providers, got %d", len(expected), len(providers))
		}

		for i, provider := range expected {
			if i >= len(providers) || providers[i] != provider {
				t.Errorf("Expected provider %s at index %d, got %v", provider, i, providers)
			}
		}
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		tests := []struct {
			name      string
			config    models.StorageConfig
			expectErr bool
		}{
			{
				name: "valid json config",
				config: models.StorageConfig{
					Type: "json",
					Path: "/tmp/test.json",
				},
				expectErr: false,
			},
			{
				name: "valid memory config",
				config: models.StorageConfig{
					Type: "memory",
				},
				expectErr: false,
			},
			{
				name: "invalid storage type",
				config: models.StorageConfig{
					Type: "invalid",
				},
				expectErr: true,
			},
			{
				name: "json without path",
				config: models.StorageConfig{
					Type: "json",
				},
				expectErr: true,
			},
			{
				name: "valid postgres config",
				config: models.StorageConfig{
					Type: "postgres",
					Database: models.DatabaseConfig{
						DSN: "postgres://user:pass@localhost/dbname",
					},
				},
				expectErr: false,
			},
			{
				name: "valid sqlite config",
				config: models.StorageConfig{
					Type: "sqlite",
					Database: models.DatabaseConfig{
						DSN: "file:test.db",
					},
				},
				expectErr: false,
			},
			{
				name: "postgres without DSN",
				config: models.StorageConfig{
					Type: "postgres",
				},
				expectErr: true,
			},
			{
				name: "sqlite without DSN",
				config: models.StorageConfig{
					Type: "sqlite",
				},
				expectErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := factory.ValidateConfig(tt.config)
				if tt.expectErr && err == nil {
					t.Error("Expected error but got none")
				}
				if !tt.expectErr && err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			})
		}
	})

	t.Run("Create", func(t *testing.T) {
		// Test JSON storage creation
		t.Run("JSON Storage", func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "storage_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			config := models.StorageConfig{
				Type: "json",
				Path: filepath.Join(tempDir, "test.json"),
			}

			storage, err := factory.Create(config)
			if err != nil {
				t.Errorf("Failed to create JSON storage: %v", err)
			}
			if storage != nil {
				storage.Close()
			}

			// Verify it's a JSONStorage
			_, ok := storage.(*JSONStorage)
			if !ok {
				t.Error("Expected JSONStorage instance")
			}
		})

		// Test Memory storage creation
		t.Run("Memory Storage", func(t *testing.T) {
			config := models.StorageConfig{
				Type: "memory",
			}

			storage, err := factory.Create(config)
			if err != nil {
				t.Errorf("Failed to create Memory storage: %v", err)
			}
			if storage != nil {
				storage.Close()
			}

			// Verify it's a MemoryStorage
			_, ok := storage.(*MemoryStorage)
			if !ok {
				t.Error("Expected MemoryStorage instance")
			}
		})

		// Test unsupported storage type
		t.Run("Unsupported Storage", func(t *testing.T) {
			config := models.StorageConfig{
				Type: "unsupported",
			}

			_, err := factory.Create(config)
			if err == nil {
				t.Error("Expected error for unsupported storage type")
			}
		})
	})
}

func TestConvertOptions(t *testing.T) {
	input := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	output := convertOptions(input)

	if len(output) != len(input) {
		t.Errorf("Expected %d options, got %d", len(input), len(output))
	}

	for key, expectedValue := range input {
		if actualValue, ok := output[key]; !ok {
			t.Errorf("Missing key %s in output", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected value %s for key %s, got %v", expectedValue, key, actualValue)
		}
	}
}
