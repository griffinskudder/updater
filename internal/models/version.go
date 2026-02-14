// Package models provides core data structures for the updater service.
// This file contains version-related types and semantic versioning implementation.
//
// Design Decisions:
// - Follows Semantic Versioning 2.0.0 specification (semver.org)
// - Supports custom version schemes while defaulting to semver
// - Preserves original version string for exact representation
// - Provides comprehensive comparison operations for update logic
package models

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Version represents a semantic version with support for pre-release and build metadata.
//
// Design Rationale:
// - Major/Minor/Patch follow semver specification for update compatibility
// - Pre-release versions (e.g., "alpha", "beta", "rc.1") are handled separately
// - Build metadata is preserved but doesn't affect version precedence
// - Raw field preserves the original version string for exact representation
// - Supports both strict semver and custom versioning schemes
type Version struct {
	Major int    `json:"major"`           // Major version number (breaking changes)
	Minor int    `json:"minor"`           // Minor version number (backward compatible features)
	Patch int    `json:"patch"`           // Patch version number (backward compatible bug fixes)
	Pre   string `json:"pre,omitempty"`   // Pre-release identifier (alpha, beta, rc.1, etc.)
	Build string `json:"build,omitempty"` // Build metadata (commit hash, build number, etc.)
	Raw   string `json:"raw"`             // Original version string for exact representation
}

// ParseVersion parses a version string into a Version struct.
//
// Supports formats:
// - "1.2.3" - Standard semantic version
// - "1.2.3-alpha" - Pre-release version
// - "1.2.3+build.123" - Version with build metadata
// - "1.2.3-beta.1+build.456" - Complete semantic version
// - "1.0" or "1" - Partial versions (missing components default to 0)
//
// Design Notes:
// - Flexible parsing to accommodate various version formats
// - Preserves original string for exact representation
// - Validates numeric components to prevent invalid versions
func ParseVersion(v string) (*Version, error) {
	if v == "" {
		return nil, errors.New("version string cannot be empty")
	}

	version := &Version{Raw: v}

	// Handle pre-release and build metadata
	mainVersion := v
	if idx := strings.Index(v, "+"); idx != -1 {
		version.Build = v[idx+1:]
		mainVersion = v[:idx]
	}
	if idx := strings.Index(mainVersion, "-"); idx != -1 {
		version.Pre = mainVersion[idx+1:]
		mainVersion = mainVersion[:idx]
	}

	// Parse major.minor.patch
	parts := strings.Split(mainVersion, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid version format: %s", v)
	}

	var err error
	version.Major, err = strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", parts[0])
	}

	if len(parts) > 1 {
		version.Minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid minor version: %s", parts[1])
		}
	}

	if len(parts) > 2 {
		version.Patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid patch version: %s", parts[2])
		}
	}

	return version, nil
}

func (v *Version) String() string {
	if v.Raw != "" {
		return v.Raw
	}

	result := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		result += "-" + v.Pre
	}
	if v.Build != "" {
		result += "+" + v.Build
	}
	return result
}

// Compare compares two versions according to semantic versioning precedence rules.
//
// Returns:
//   -1 if v < other
//    0 if v == other
//   +1 if v > other
//
// Comparison Logic:
// 1. Major, minor, patch versions are compared numerically
// 2. Pre-release versions have lower precedence than normal versions
// 3. Pre-release versions are compared lexically
// 4. Build metadata is ignored in comparisons (per semver spec)
//
// Examples:
//   1.0.0 > 0.9.9
//   1.0.0 > 1.0.0-alpha
//   1.0.0-beta > 1.0.0-alpha
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		return compareInt(v.Major, other.Major)
	}
	if v.Minor != other.Minor {
		return compareInt(v.Minor, other.Minor)
	}
	if v.Patch != other.Patch {
		return compareInt(v.Patch, other.Patch)
	}

	// Handle pre-release versions
	if v.Pre == "" && other.Pre != "" {
		return 1 // non-prerelease > prerelease
	}
	if v.Pre != "" && other.Pre == "" {
		return -1 // prerelease < non-prerelease
	}
	if v.Pre != "" && other.Pre != "" {
		return strings.Compare(v.Pre, other.Pre)
	}

	return 0
}

func (v *Version) Equal(other *Version) bool {
	return v.Compare(other) == 0
}

func (v *Version) GreaterThan(other *Version) bool {
	return v.Compare(other) > 0
}

func (v *Version) LessThan(other *Version) bool {
	return v.Compare(other) < 0
}

func (v *Version) GreaterThanOrEqual(other *Version) bool {
	return v.Compare(other) >= 0
}

func (v *Version) LessThanOrEqual(other *Version) bool {
	return v.Compare(other) <= 0
}

func compareInt(a, b int) int {
	if a > b {
		return 1
	}
	if a < b {
		return -1
	}
	return 0
}

// VersionConstraint represents a version constraint for dependency or compatibility checking.
//
// Supported Operators:
// - "=" or "==" or "" - Exact match
// - "!=" - Not equal
// - ">" - Greater than
// - ">=" - Greater than or equal
// - "<" - Less than
// - "<=" - Less than or equal
//
// Design Purpose:
// - Enables flexible version requirements (e.g., minimum version constraints)
// - Used for application compatibility checking
// - Supports both exact and range-based version matching
type VersionConstraint struct {
	Operator string `json:"operator"` // Comparison operator (=, !=, >, >=, <, <=)
	Version  string `json:"version"`  // Version string to compare against
}

func (vc *VersionConstraint) Check(version *Version) (bool, error) {
	constraint, err := ParseVersion(vc.Version)
	if err != nil {
		return false, fmt.Errorf("invalid constraint version: %w", err)
	}

	switch vc.Operator {
	case "=", "==", "":
		return version.Equal(constraint), nil
	case "!=":
		return !version.Equal(constraint), nil
	case ">":
		return version.GreaterThan(constraint), nil
	case ">=":
		return version.GreaterThanOrEqual(constraint), nil
	case "<":
		return version.LessThan(constraint), nil
	case "<=":
		return version.LessThanOrEqual(constraint), nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", vc.Operator)
	}
}