package storage

import (
	"encoding/json"
	"fmt"
	"updater/internal/models"
)

// marshalPlatforms converts a string slice of platforms to JSON bytes.
func marshalPlatforms(platforms []string) ([]byte, error) {
	if platforms == nil {
		platforms = []string{}
	}
	return json.Marshal(platforms)
}

// unmarshalPlatforms converts JSON bytes to a string slice of platforms.
func unmarshalPlatforms(data []byte) ([]string, error) {
	if len(data) == 0 {
		return []string{}, nil
	}
	var platforms []string
	if err := json.Unmarshal(data, &platforms); err != nil {
		return nil, fmt.Errorf("failed to unmarshal platforms: %w", err)
	}
	if platforms == nil {
		platforms = []string{}
	}
	return platforms, nil
}

// unmarshalPlatformsFromString converts a JSON string to a string slice of platforms.
func unmarshalPlatformsFromString(data string) ([]string, error) {
	return unmarshalPlatforms([]byte(data))
}

// marshalConfig converts an ApplicationConfig to JSON bytes.
func marshalConfig(config models.ApplicationConfig) ([]byte, error) {
	return json.Marshal(config)
}

// unmarshalConfig converts JSON bytes to an ApplicationConfig.
func unmarshalConfig(data []byte) (models.ApplicationConfig, error) {
	var config models.ApplicationConfig
	if len(data) == 0 {
		return config, nil
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return config, nil
}

// unmarshalConfigFromString converts a JSON string to an ApplicationConfig.
func unmarshalConfigFromString(data string) (models.ApplicationConfig, error) {
	return unmarshalConfig([]byte(data))
}

// marshalMetadata converts a metadata map to JSON bytes.
func marshalMetadata(metadata map[string]string) ([]byte, error) {
	if metadata == nil {
		return json.Marshal(map[string]string{})
	}
	return json.Marshal(metadata)
}

// unmarshalMetadata converts JSON bytes to a metadata map.
func unmarshalMetadata(data []byte) (map[string]string, error) {
	if len(data) == 0 {
		return make(map[string]string), nil
	}
	var metadata map[string]string
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return metadata, nil
}

// unmarshalMetadataFromString converts a JSON string to a metadata map.
func unmarshalMetadataFromString(data string) (map[string]string, error) {
	return unmarshalMetadata([]byte(data))
}

// marshalPermissions serialises a permissions slice to a JSON string.
func marshalPermissions(perms []string) (string, error) {
	if perms == nil {
		perms = []string{}
	}
	b, err := json.Marshal(perms)
	if err != nil {
		return "", fmt.Errorf("marshal permissions: %w", err)
	}
	return string(b), nil
}

// unmarshalPermissions parses a JSON string into a permissions slice.
func unmarshalPermissions(data string) ([]string, error) {
	if data == "" {
		return []string{}, nil
	}
	var perms []string
	if err := json.Unmarshal([]byte(data), &perms); err != nil {
		return nil, fmt.Errorf("unmarshal permissions: %w", err)
	}
	if perms == nil {
		perms = []string{}
	}
	return perms, nil
}
