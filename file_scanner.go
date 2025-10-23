package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

/*
FileScanner discovers files in the source directory and reports them to FileTracker.

Responsibilities:
1. Periodically scan the source directory (every 2s by default)
2. Find files matching configured filters (e.g., "*.zip")
3. Find corresponding .sha256 files
4. Report discovered files to FileTracker for tracking
5. Graceful start/stop with context cancellation

Does NOT:
- Track file pairs or state (that's file_tracker.go)
- Verify hashes (that's sha_verifier.go)
- Move or delete files (that's file_operations.go)
- Process verification jobs (that's worker_pool.go)
*/

// FileScanner periodically scans the source directory for files
type FileScanner struct {
	sourceFolder string
	scanInterval time.Duration
	fileFilters  []string
	tracker      *FileTracker
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	logLevel     string
}

// NewFileScanner creates a new file scanner
func NewFileScanner(sourceFolder string, scanInterval time.Duration, fileFilters []string, tracker *FileTracker, logLevel string) *FileScanner {
	ctx, cancel := context.WithCancel(context.Background())

	return &FileScanner{
		sourceFolder: sourceFolder,
		scanInterval: scanInterval,
		fileFilters:  fileFilters,
		tracker:      tracker,
		ctx:          ctx,
		cancel:       cancel,
		logLevel:     logLevel,
	}
}

// Start begins the periodic scanning routine
func (fs *FileScanner) Start() {
	fs.wg.Add(1)
	go fs.scanLoop()

	if fs.logLevel == "DEBUG" || fs.logLevel == "INFO" {
		fmt.Printf("[Scanner] Started scanning %s every %s\n", fs.sourceFolder, fs.scanInterval)
	}
}

// Stop gracefully stops the scanning routine
func (fs *FileScanner) Stop() {
	fs.cancel()
	fs.wg.Wait()

	if fs.logLevel == "DEBUG" || fs.logLevel == "INFO" {
		fmt.Println("[Scanner] Stopped")
	}
}

// scanLoop runs the periodic scan routine
func (fs *FileScanner) scanLoop() {
	defer fs.wg.Done()

	// Perform initial scan immediately
	if err := fs.scan(); err != nil {
		fmt.Fprintf(os.Stderr, "[Scanner] Error during initial scan: %v\n", err)
	}

	ticker := time.NewTicker(fs.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := fs.scan(); err != nil {
				fmt.Fprintf(os.Stderr, "[Scanner] Error during scan: %v\n", err)
			}
		case <-fs.ctx.Done():
			return
		}
	}
}

// scan performs a single directory scan
func (fs *FileScanner) scan() error {
	if fs.logLevel == "DEBUG" {
		fmt.Printf("[Scanner] Scanning %s...\n", fs.sourceFolder)
	}

	// Read directory contents
	entries, err := os.ReadDir(fs.sourceFolder)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	dataFilesFound := 0
	sha256FilesFound := 0

	// Process each entry
	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		fullPath := filepath.Join(fs.sourceFolder, filename)

		// Check if it's a .sha256 file
		if strings.HasSuffix(filename, ".sha256") {
			// This is a SHA256 file
			fs.tracker.AddOrUpdateSHA256File(fullPath)
			sha256FilesFound++

			if fs.logLevel == "DEBUG" {
				fmt.Printf("[Scanner] Found SHA256 file: %s\n", filename)
			}
			continue
		}

		// Check if it matches any data file filter
		if fs.matchesFilter(filename) {
			// This is a data file
			info, err := entry.Info()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[Scanner] Failed to get file info for %s: %v\n", filename, err)
				continue
			}

			fileSize := info.Size()
			fs.tracker.AddOrUpdateDataFile(fullPath, fileSize)
			dataFilesFound++

			if fs.logLevel == "DEBUG" {
				fmt.Printf("[Scanner] Found data file: %s (%d bytes)\n", filename, fileSize)
			}

			// Check if corresponding .sha256 file exists
			sha256Path := fullPath + ".sha256"
			if _, err := os.Stat(sha256Path); err == nil {
				// SHA256 file exists
				fs.tracker.AddOrUpdateSHA256File(sha256Path)
				fs.tracker.MarkBothFilesPresent(filename)

				if fs.logLevel == "DEBUG" {
					fmt.Printf("[Scanner] Found complete pair: %s + %s.sha256\n", filename, filename)
				}
			}
		}
	}

	if fs.logLevel == "DEBUG" {
		fmt.Printf("[Scanner] Scan complete: %d data files, %d SHA256 files\n", dataFilesFound, sha256FilesFound)
	}

	return nil
}

// matchesFilter checks if a filename matches any of the configured filters
// Supports wildcard patterns like "*.zip", "*.tar.gz"
func (fs *FileScanner) matchesFilter(filename string) bool {
	for _, filter := range fs.fileFilters {
		matched, err := filepath.Match(filter, filename)
		if err != nil {
			// Invalid pattern, skip
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// GetPendingCount returns the current number of tracked files
func (fs *FileScanner) GetPendingCount() int {
	return fs.tracker.GetPendingCount()
}

// TriggerScan forces an immediate scan (useful for testing)
func (fs *FileScanner) TriggerScan() error {
	return fs.scan()
}
