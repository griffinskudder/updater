package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"updater/internal/models"
)

func TestGenerateAPIKey(t *testing.T) {
	key, err := models.GenerateAPIKey()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, "upd_"), "key must start with upd_")
	assert.Len(t, key, 48, "upd_ (4) + 44 base64url chars = 48")
}

func TestHashAPIKey(t *testing.T) {
	hash1 := models.HashAPIKey("upd_abc123")
	hash2 := models.HashAPIKey("upd_abc123")
	hash3 := models.HashAPIKey("upd_different")
	assert.Equal(t, hash1, hash2, "same input must produce same hash")
	assert.NotEqual(t, hash1, hash3, "different inputs must produce different hashes")
	assert.Len(t, hash1, 64, "SHA-256 hex is 64 characters")
}

func TestAPIKeyHasPermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		enabled     bool
		check       string
		want        bool
	}{
		{"admin grants read", []string{"admin"}, true, "read", true},
		{"admin grants write", []string{"admin"}, true, "write", true},
		{"admin grants admin", []string{"admin"}, true, "admin", true},
		{"write grants read", []string{"write"}, true, "read", true},
		{"write grants write", []string{"write"}, true, "write", true},
		{"write denied admin", []string{"write"}, true, "admin", false},
		{"read only", []string{"read"}, true, "read", true},
		{"read denied write", []string{"read"}, true, "write", false},
		{"wildcard grants all", []string{"*"}, true, "admin", true},
		{"disabled key denied", []string{"admin"}, false, "read", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &models.APIKey{Permissions: tt.permissions, Enabled: tt.enabled}
			assert.Equal(t, tt.want, key.HasPermission(tt.check))
		})
	}
}

func TestNewAPIKey(t *testing.T) {
	raw := "upd_testkey123456789012345678901234567890123"
	key := models.NewAPIKey("test-id", "test", raw, []string{"read"})
	assert.Equal(t, "test-id", key.ID)
	assert.Equal(t, "test", key.Name)
	assert.Equal(t, models.HashAPIKey(raw), key.KeyHash)
	assert.Equal(t, raw[:8], key.Prefix)
	assert.True(t, key.Enabled)

	// short key: prefix equals the key itself when len <= 8
	shortKey := models.NewAPIKey("id2", "short", "upd_ab", []string{})
	assert.Equal(t, "upd_ab", shortKey.Prefix)
}
