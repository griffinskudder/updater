package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// APIKey represents a stored API key. The raw key value is never persisted;
// only its SHA-256 hex hash and an 8-character display prefix are stored.
type APIKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	KeyHash     string    `json:"key_hash"`
	Prefix      string    `json:"prefix"`
	Permissions []string  `json:"permissions"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewAPIKey creates a new APIKey from a raw key string.
func NewAPIKey(id, name, rawKey string, permissions []string) *APIKey {
	now := time.Now().UTC()
	prefix := rawKey
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	return &APIKey{
		ID:          id,
		Name:        name,
		KeyHash:     HashAPIKey(rawKey),
		Prefix:      prefix,
		Permissions: permissions,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// GenerateAPIKey produces a new random API key in the format upd_<44 url-safe base64 chars>.
func GenerateAPIKey() (string, error) {
	b := make([]byte, 33) // 33 bytes → 44 base64url chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return "upd_" + base64.RawURLEncoding.EncodeToString(b), nil
}

// HashAPIKey computes the SHA-256 hex digest of a raw API key.
func HashAPIKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}

// NewKeyID generates a new UUID v4 for use as an APIKey ID.
func NewKeyID() string {
	return uuid.New().String()
}

// HasPermission returns true when the key is enabled and possesses the required permission.
func (ak *APIKey) HasPermission(required string) bool {
	if !ak.Enabled {
		return false
	}
	for _, p := range ak.Permissions {
		switch p {
		case "*", "admin":
			return true
		case "write":
			if required == "read" || required == "write" {
				return true
			}
		case required:
			return true
		}
	}
	return false
}
