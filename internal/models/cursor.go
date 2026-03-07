// Package models - Keyset pagination cursor types.
// Cursors are opaque base64-encoded JSON tokens produced by the server.
// Clients must treat them as opaque strings and must not construct or modify them.
package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// ReleaseCursor encodes the position of the last item on a releases page.
// All sort-field values are always present; only the field matching SortBy is used
// for the keyset comparison. Clients must not inspect or modify cursors.
type ReleaseCursor struct {
	SortBy            string    `json:"sort_by"`
	SortOrder         string    `json:"sort_order"`
	ID                string    `json:"id"`
	ReleaseDate       time.Time `json:"release_date"`
	VersionMajor      int64     `json:"version_major"`
	VersionMinor      int64     `json:"version_minor"`
	VersionPatch      int64     `json:"version_patch"`
	VersionIsStable   bool      `json:"version_is_stable"`
	VersionPreRelease string    `json:"version_pre_release"`
	Platform          string    `json:"platform"`
	Architecture      string    `json:"architecture"`
	CreatedAt         time.Time `json:"created_at"`
}

// Encode serialises the cursor to an opaque base64-encoded JSON string.
func (c *ReleaseCursor) Encode() (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("encode release cursor: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DecodeReleaseCursor deserialises a cursor produced by ReleaseCursor.Encode.
// Returns an error if the string is not valid base64 or not valid JSON.
func DecodeReleaseCursor(s string) (*ReleaseCursor, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}
	var c ReleaseCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}
	return &c, nil
}

// ApplicationCursor encodes the position of the last item on an applications page.
// Applications are always sorted by created_at DESC, id DESC.
// Clients must not inspect or modify cursors.
type ApplicationCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

// Encode serialises the cursor to an opaque base64-encoded JSON string.
func (c *ApplicationCursor) Encode() (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("encode application cursor: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DecodeApplicationCursor deserialises a cursor produced by ApplicationCursor.Encode.
// Returns an error if the string is not valid base64 or not valid JSON.
func DecodeApplicationCursor(s string) (*ApplicationCursor, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}
	var c ApplicationCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}
	return &c, nil
}