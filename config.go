package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads and parses the configuration file
func LoadConfig(configPath string) (*Config, error) {
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create destination folders if they don't exist
	if err := createDestinationFolders(&config); err != nil {
		return nil, fmt.Errorf("failed to create destination folders: %w", err)
	}

	return &config, nil
}

// validateConfig ensures all required fields are present and valid
func validateConfig(cfg *Config) error {
	// Validate source folder
	if cfg.Spec.Source.Folder == "" {
		return fmt.Errorf("source.folder cannot be empty")
	}

	// Check if source folder exists
	if _, err := os.Stat(cfg.Spec.Source.Folder); os.IsNotExist(err) {
		return fmt.Errorf("source folder does not exist: %s", cfg.Spec.Source.Folder)
	}

	// Validate scan interval
	if cfg.Spec.Source.PeriodicScanInterval <= 0 {
		return fmt.Errorf("source.periodicScanInterval must be positive")
	}

	// Validate retry timeout
	if cfg.Spec.Verification.RetryTimeout <= 0 {
		return fmt.Errorf("verification.retryTimeout must be positive")
	}

	// Validate buffer size
	if cfg.Spec.Verification.BufferSize <= 0 {
		return fmt.Errorf("verification.bufferSize must be positive")
	}

	// Validate file filters
	if len(cfg.Spec.Verification.FileFilters) == 0 {
		return fmt.Errorf("verification.fileFilters cannot be empty")
	}

	// Validate destination folders
	if cfg.Spec.Destination.VerifiedFolder == "" {
		return fmt.Errorf("destination.verifiedFolder cannot be empty")
	}
	if cfg.Spec.Destination.DlqFolder == "" {
		return fmt.Errorf("destination.dlqFolder cannot be empty")
	}

	// Validate concurrency settings
	if cfg.Spec.Concurrency.Workers <= 0 {
		return fmt.Errorf("concurrency.workers must be positive")
	}
	if cfg.Spec.Concurrency.QueueSize <= 0 {
		return fmt.Errorf("concurrency.queueSize must be positive")
	}

	// Validate output settings
	if cfg.Spec.Output.VerificationFile == "" {
		return fmt.Errorf("output.verificationFile cannot be empty")
	}
	if cfg.Spec.Output.StatsFile == "" {
		return fmt.Errorf("output.statsFile cannot be empty")
	}
	if cfg.Spec.Output.FlushInterval <= 0 {
		return fmt.Errorf("output.flushInterval must be positive")
	}

	// Validate logging level
	validLevels := map[string]bool{"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true}
	if !validLevels[cfg.Spec.Logging.Level] {
		return fmt.Errorf("logging.level must be one of: DEBUG, INFO, WARN, ERROR")
	}

	return nil
}

// createDestinationFolders creates verified and DLQ folders if they don't exist
func createDestinationFolders(cfg *Config) error {
	// Create verified folder
	verifiedPath := cfg.Spec.Destination.VerifiedFolder
	if err := os.MkdirAll(verifiedPath, 0755); err != nil {
		return fmt.Errorf("failed to create verified folder %s: %w", verifiedPath, err)
	}

	// Create DLQ folder
	dlqPath := cfg.Spec.Destination.DlqFolder
	if err := os.MkdirAll(dlqPath, 0755); err != nil {
		return fmt.Errorf("failed to create DLQ folder %s: %w", dlqPath, err)
	}

	return nil
}

// GetAbsolutePath converts a relative path to absolute based on config file location
func GetAbsolutePath(configDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(configDir, path)
}

// PrintConfig displays the loaded configuration (for debugging)
func PrintConfig(cfg *Config) {
	fmt.Println("=== Configuration Loaded ===")
	fmt.Printf("App Name:        %s\n", cfg.AppName)
	fmt.Printf("Version:         %s\n", cfg.AppVersion)
	fmt.Printf("Source Folder:   %s\n", cfg.Spec.Source.Folder)
	fmt.Printf("Scan Interval:   %s\n", cfg.Spec.Source.PeriodicScanInterval)
	fmt.Printf("Retry Timeout:   %s\n", cfg.Spec.Verification.RetryTimeout)
	fmt.Printf("Buffer Size:     %d bytes\n", cfg.Spec.Verification.BufferSize)
	fmt.Printf("File Filters:    %v\n", cfg.Spec.Verification.FileFilters)
	fmt.Printf("Verified Folder: %s\n", cfg.Spec.Destination.VerifiedFolder)
	fmt.Printf("DLQ Folder:      %s\n", cfg.Spec.Destination.DlqFolder)
	fmt.Printf("Workers:         %d\n", cfg.Spec.Concurrency.Workers)
	fmt.Printf("Queue Size:      %d\n", cfg.Spec.Concurrency.QueueSize)
	fmt.Printf("Log Level:       %s\n", cfg.Spec.Logging.Level)
	fmt.Println("============================")
}
