// Package version provides build-time metadata for the updater service.
// These variables are populated via -ldflags during the Docker build process.
package version

import (
	"fmt"
	"os"
	"sync"

	"github.com/google/uuid"
)

var (
	// Version is the semantic version or git commit hash (e.g., "v1.0.0" or "a1b2c3d").
	// Set via: -ldflags "-X updater/internal/version.Version=..."
	Version = "unknown"

	// BuildDate is the ISO 8601 UTC timestamp when the binary was built.
	// Set via: -ldflags "-X updater/internal/version.BuildDate=..."
	BuildDate = "unknown"

	// GitCommit is the git commit SHA of the source code.
	// Set via: -ldflags "-X updater/internal/version.GitCommit=..."
	GitCommit = "unknown"
)

// Info holds all build metadata and runtime information.
type Info struct {
	Version    string `json:"version"`
	GitCommit  string `json:"git_commit"`
	BuildDate  string `json:"build_date"`
	InstanceID string `json:"instance_id"`
	Hostname   string `json:"hostname"`
}

var (
	once sync.Once
	info Info
)

// GetInfo returns build metadata and runtime information.
// Instance ID and hostname are computed once on first call and cached.
func GetInfo() Info {
	once.Do(func() {
		info = Info{
			Version:    Version,
			GitCommit:  GitCommit,
			BuildDate:  BuildDate,
			InstanceID: uuid.New().String(),
			Hostname:   getHostname(),
		}
	})
	return info
}

// getHostname returns the system hostname, fallback to "unknown" on error.
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// String formats version info for CLI display.
func (i Info) String() string {
	return fmt.Sprintf("updater version %s (commit: %s, built: %s)", i.Version, i.GitCommit, i.BuildDate)
}
