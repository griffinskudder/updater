package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseCursor_RoundTrip(t *testing.T) {
	original := &ReleaseCursor{
		SortBy:            "release_date",
		SortOrder:         "desc",
		ID:                "app1-1.0.0-linux-amd64",
		ReleaseDate:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		VersionMajor:      1,
		VersionMinor:      2,
		VersionPatch:      3,
		VersionIsStable:   true,
		VersionPreRelease: "",
		Platform:          "linux",
		Architecture:      "amd64",
		CreatedAt:         time.Date(2024, 1, 10, 9, 0, 0, 0, time.UTC),
	}

	encoded, err := original.Encode()
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)

	decoded, err := DecodeReleaseCursor(encoded)
	require.NoError(t, err)
	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.SortBy, decoded.SortBy)
	assert.Equal(t, original.SortOrder, decoded.SortOrder)
	assert.True(t, original.ReleaseDate.Equal(decoded.ReleaseDate))
	assert.Equal(t, original.VersionMajor, decoded.VersionMajor)
	assert.Equal(t, original.VersionIsStable, decoded.VersionIsStable)
	assert.Equal(t, original.Platform, decoded.Platform)
	assert.True(t, original.CreatedAt.Equal(decoded.CreatedAt))
}

func TestReleaseCursor_PreRelease_RoundTrip(t *testing.T) {
	original := &ReleaseCursor{
		SortBy:            "version",
		SortOrder:         "desc",
		ID:                "app1-1.0.0-beta.1-linux-amd64",
		VersionMajor:      1,
		VersionMinor:      0,
		VersionPatch:      0,
		VersionIsStable:   false,
		VersionPreRelease: "beta.1",
	}

	encoded, err := original.Encode()
	require.NoError(t, err)

	decoded, err := DecodeReleaseCursor(encoded)
	require.NoError(t, err)
	assert.Equal(t, original.VersionIsStable, decoded.VersionIsStable)
	assert.Equal(t, original.VersionPreRelease, decoded.VersionPreRelease)
}

func TestDecodeReleaseCursor_InvalidBase64(t *testing.T) {
	_, err := DecodeReleaseCursor("not-valid-base64!!!")
	assert.Error(t, err)
}

func TestDecodeReleaseCursor_InvalidJSON(t *testing.T) {
	// base64("not json")
	_, err := DecodeReleaseCursor("bm90IGpzb24=")
	assert.Error(t, err)
}

func TestApplicationCursor_RoundTrip(t *testing.T) {
	original := &ApplicationCursor{
		CreatedAt: time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC),
		ID:        "my-app",
	}

	encoded, err := original.Encode()
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)

	decoded, err := DecodeApplicationCursor(encoded)
	require.NoError(t, err)
	assert.Equal(t, original.ID, decoded.ID)
	assert.True(t, original.CreatedAt.Equal(decoded.CreatedAt))
}

func TestDecodeApplicationCursor_Invalid(t *testing.T) {
	_, err := DecodeApplicationCursor("not-base64!!!")
	assert.Error(t, err)
}