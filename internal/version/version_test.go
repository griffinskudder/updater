package version

import (
	"testing"
)

func TestGetInfo(t *testing.T) {
	info := GetInfo()

	// Test that fields are populated (even if "unknown")
	if info.Version == "" {
		t.Error("Version should not be empty")
	}
	if info.GitCommit == "" {
		t.Error("GitCommit should not be empty")
	}
	if info.BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}
	if info.InstanceID == "" {
		t.Error("InstanceID should not be empty")
	}
	if info.Hostname == "" {
		t.Error("Hostname should not be empty")
	}

	// Test caching - subsequent calls should return same instance ID
	info2 := GetInfo()
	if info.InstanceID != info2.InstanceID {
		t.Errorf("InstanceID should be cached, got %s then %s", info.InstanceID, info2.InstanceID)
	}
	if info.Hostname != info2.Hostname {
		t.Errorf("Hostname should be cached, got %s then %s", info.Hostname, info2.Hostname)
	}
}

func TestInfoString(t *testing.T) {
	tests := []struct {
		name     string
		info     Info
		expected string
	}{
		{
			name: "full version info",
			info: Info{
				Version:   "1.2.3",
				GitCommit: "abc1234",
				BuildDate: "2026-02-21T10:00:00Z",
			},
			expected: "updater version 1.2.3 (commit: abc1234, built: 2026-02-21T10:00:00Z)",
		},
		{
			name: "unknown values",
			info: Info{
				Version:   "unknown",
				GitCommit: "unknown",
				BuildDate: "unknown",
			},
			expected: "updater version unknown (commit: unknown, built: unknown)",
		},
		{
			name: "dirty version",
			info: Info{
				Version:   "v1.0.0-dirty",
				GitCommit: "abc1234",
				BuildDate: "2026-02-21T10:00:00Z",
			},
			expected: "updater version v1.0.0-dirty (commit: abc1234, built: 2026-02-21T10:00:00Z)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetHostname(t *testing.T) {
	hostname := getHostname()

	// Should never be empty (fallback to "unknown")
	if hostname == "" {
		t.Error("getHostname() should return non-empty string")
	}
}