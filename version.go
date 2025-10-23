package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// VersionInfo represents version information from release notes
type VersionInfo struct {
	Version  string `yaml:"version"`
	Datetime string `yaml:"datetime"`
	BuildID  string `yaml:"build_id"`
}

// ReleaseNotes represents the structure of the release notes YAML file
type ReleaseNotes struct {
	Versions []VersionInfo `yaml:"versions"`
}

// ValidateVersion validates that the embedded build version matches the release notes.
//
// This function:
//  1. Locates the release notes YAML file
//  2. Reads the first version entry
//  3. Compares embedded version info with release notes
//  4. Returns error if there's a mismatch
//
// Parameters:
//   - version: Embedded version string from build
//   - buildTime: Embedded build timestamp from build
//   - buildID: Embedded SHA256 hash from build
//
// Returns:
//   - error: If validation fails or file cannot be read
//
// This validation ensures that:
//   - The binary matches its release notes
//   - No tampering has occurred
//   - Deployment is using the correct version
func ValidateVersion(version, buildTime, buildID string) error {
	// Get release notes file path
	rnFile, err := GetReleaseNotesFile()
	if err != nil {
		return fmt.Errorf("failed to locate release notes file: %w", err)
	}

	// Read version info from release notes
	releaseInfo, err := GetFirstVersionInfo(rnFile)
	if err != nil {
		return fmt.Errorf("failed to read version info from %s: %w", rnFile, err)
	}

	// Compare versions
	if version != releaseInfo.Version ||
		buildTime != releaseInfo.Datetime ||
		buildID != releaseInfo.BuildID {
		return fmt.Errorf(
			"version mismatch detected:\n"+
				"  Binary:        version=%s, buildTime=%s, buildID=%s\n"+
				"  Release Notes: version=%s, buildTime=%s, buildID=%s",
			version, buildTime, buildID,
			releaseInfo.Version, releaseInfo.Datetime, releaseInfo.BuildID,
		)
	}

	return nil
}

// GetReleaseNotesFile searches for the release notes YAML file in the binary's directory.
//
// This function automatically locates the .RN.yaml file without requiring it to be
// passed as an argument. It searches in the same directory where the binary is running.
//
// Example:
//
//	If binary is at: /opt/apps/go-ftp-transfer/ftp-uploader
//	It will search:  /opt/apps/go-ftp-transfer/*.RN.yaml
//
// Returns:
//   - string: Full path to the .RN.yaml file
//   - error: If file not found or directory cannot be determined
func GetReleaseNotesFile() (string, error) {
	// Get the directory where the binary is located
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	dir := filepath.Dir(exePath)

	// Search for any file ending with .RN.yaml in the binary's directory
	rnFile, err := SearchFile(dir, "*.RN.yaml")
	if err != nil {
		return "", fmt.Errorf(".RN.yaml file not found in %s", dir)
	}

	return rnFile, nil
}

// GetFirstVersionInfo reads the first version entry from the release notes YAML file.
//
// Parameters:
//   - filename: Path to the release notes YAML file
//
// Returns:
//   - VersionInfo: First version entry from the file
//   - error: If file cannot be read or parsed
func GetFirstVersionInfo(filename string) (VersionInfo, error) {
	// Read the YAML file
	data, err := os.ReadFile(filename)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the YAML data
	var releaseNotes ReleaseNotes
	err = yaml.Unmarshal(data, &releaseNotes)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Return the first version info if available
	if len(releaseNotes.Versions) > 0 {
		return releaseNotes.Versions[0], nil
	}

	// Return error if no versions found
	return VersionInfo{}, fmt.Errorf("no version entries found in release notes")
}

// SearchFile searches for a file matching the given pattern in the specified directory.
//
// Parameters:
//   - dir: Directory to search in
//   - pattern: Glob pattern to match (e.g., "*.RN.yaml")
//
// Returns:
//   - string: Full path to the first matching file
//   - error: If no files match or directory cannot be read
func SearchFile(dir, pattern string) (string, error) {
	// Build the full search pattern
	searchPattern := filepath.Join(dir, pattern)

	// Find matching files
	matches, err := filepath.Glob(searchPattern)
	if err != nil {
		return "", fmt.Errorf("failed to search for pattern %s: %w", pattern, err)
	}

	// Check if any files were found
	if len(matches) == 0 {
		return "", fmt.Errorf("no files matching pattern %s found in %s", pattern, dir)
	}

	// Return the first match
	return matches[0], nil
}

// PrintVersionInfo prints the application version information in a formatted way.
//
// Parameters:
//   - appName: Name of the application
//   - release: Release type (PRODUCTION, DEVELOPMENT, etc.)
//   - version: Version string
//   - buildTime: Build timestamp
//   - buildID: Build ID (SHA256 hash)
func PrintVersionInfo(appName, release, version, buildTime, buildID string) {
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("  %s\n", appName)
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("  Release:    %s\n", release)
	fmt.Printf("  Version:    %s\n", version)
	fmt.Printf("  Build Time: %s\n", buildTime)
	fmt.Printf("  Build ID:   %s\n", buildID)
	fmt.Println(strings.Repeat("=", 70))
}
