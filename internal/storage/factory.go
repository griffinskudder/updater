package storage

import (
	"fmt"
	"updater/internal/models"
)

// Factory provides a centralized way to create storage instances based on configuration.
// This allows for easy extensibility and provider swapping without code changes.
type Factory struct{}

// NewFactory creates a new storage factory
func NewFactory() *Factory {
	return &Factory{}
}

// Create instantiates a storage provider based on the provided configuration.
// Supported providers:
//   - json: JSON file-based storage (thread-safe with caching)
//   - memory: In-memory storage (for testing/development)
//   - postgres: PostgreSQL database storage (production-ready)
//   - sqlite: SQLite database storage (lightweight database)
func (f *Factory) Create(config models.StorageConfig) (Storage, error) {
	// Convert models.StorageConfig to internal Config format
	storageConfig := Config{
		Type:             config.Type,
		Path:             config.Path,
		ConnectionString: config.Database.DSN,
		Options:          convertOptions(config.Options),
	}

	switch config.Type {
	case models.StorageTypeJSON:
		return NewJSONStorage(storageConfig)
	case models.StorageTypeMemory:
		return NewMemoryStorage(storageConfig)
	case models.StorageTypePostgres:
		return NewPostgresStorage(storageConfig)
	case models.StorageTypeSQLite:
		return NewSQLiteStorage(storageConfig)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Type)
	}
}

// GetSupportedProviders returns a list of all supported storage provider types
func (f *Factory) GetSupportedProviders() []string {
	return []string{models.StorageTypeJSON, models.StorageTypeMemory, models.StorageTypePostgres, models.StorageTypeSQLite}
}

// ValidateConfig validates that a storage configuration is valid for its type
func (f *Factory) ValidateConfig(config models.StorageConfig) error {
	switch config.Type {
	case models.StorageTypeJSON:
		if config.Path == "" {
			return fmt.Errorf("path is required for JSON storage")
		}
	case models.StorageTypeMemory:
		// Memory storage requires no additional configuration
	case models.StorageTypePostgres, models.StorageTypeSQLite:
		if config.Database.DSN == "" {
			return fmt.Errorf("database DSN is required for %s storage", config.Type)
		}
	default:
		return fmt.Errorf("unsupported storage type: %s", config.Type)
	}
	return nil
}

// convertOptions converts map[string]string to map[string]interface{}
func convertOptions(options map[string]string) map[string]interface{} {
	converted := make(map[string]interface{})
	for k, v := range options {
		converted[k] = v
	}
	return converted
}
