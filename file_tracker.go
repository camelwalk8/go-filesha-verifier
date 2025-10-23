package main

import (
	"path/filepath"
	"sync"
	"time"
)

/*
FileTracker manages the lifecycle of file pairs (data file + .sha256 file).

Responsibilities:
1. Track file pairs in memory using a map keyed by data filename
2. Determine when BOTH files in a pair exist and are ready for verification
3. Track when each file pair was first seen (for retry timeout logic)
4. Identify files that have exceeded retry timeout and should move to DLQ
5. Thread-safe operations for concurrent access

Does NOT:
- Scan the file system (that's file_scanner.go)
- Verify SHA256 hashes (that's sha_verifier.go)
- Move/delete files (that's file_operations.go)
*/

// FileTracker manages file pair tracking and retry timeout logic
type FileTracker struct {
	mutex        sync.RWMutex
	files        map[string]*FilePair // Key: data filename (e.g., "data.zip")
	retryTimeout time.Duration        // How long to wait before moving to DLQ
}

// NewFileTracker creates a new file tracker with the specified retry timeout
func NewFileTracker(retryTimeout time.Duration) *FileTracker {
	return &FileTracker{
		files:        make(map[string]*FilePair),
		retryTimeout: retryTimeout,
	}
}

// AddOrUpdateDataFile adds or updates a data file in the tracker
// This is called when the scanner finds a data file (e.g., "data.zip")
func (ft *FileTracker) AddOrUpdateDataFile(dataFilePath string, dataSize int64) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	// Extract filename from path
	dataFile := filepath.Base(dataFilePath)

	// Check if we already track this file
	if pair, exists := ft.files[dataFile]; exists {
		// Update existing entry
		pair.DataFilePath = dataFilePath
		pair.DataSize = dataSize
	} else {
		// Create new entry
		ft.files[dataFile] = &FilePair{
			DataFile:     dataFile,
			DataFilePath: dataFilePath,
			DataSize:     dataSize,
			FirstSeen:    time.Now(),
			HasBothFiles: false,
		}
	}
}

// AddOrUpdateSHA256File adds or updates a .sha256 file in the tracker
// This is called when the scanner finds a .sha256 file (e.g., "data.zip.sha256")
func (ft *FileTracker) AddOrUpdateSHA256File(sha256FilePath string) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	// Extract filename from path (e.g., "data.zip.sha256")
	sha256File := filepath.Base(sha256FilePath)

	// Derive the data filename by removing ".sha256" suffix
	// "data.zip.sha256" -> "data.zip"
	dataFile := sha256File[:len(sha256File)-7] // Remove ".sha256"

	// Check if we already track this data file
	if pair, exists := ft.files[dataFile]; exists {
		// Update existing entry
		pair.SHA256File = sha256File
		pair.SHA256Path = sha256FilePath
		pair.HasBothFiles = true // Both files now exist
	} else {
		// Create new entry (data file not yet seen)
		ft.files[dataFile] = &FilePair{
			DataFile:     dataFile,
			SHA256File:   sha256File,
			SHA256Path:   sha256FilePath,
			FirstSeen:    time.Now(),
			HasBothFiles: false, // Data file not yet present
		}
	}
}

// MarkBothFilesPresent updates a file pair when both files exist
// This is called after confirming both data and .sha256 files are present
func (ft *FileTracker) MarkBothFilesPresent(dataFile string) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if pair, exists := ft.files[dataFile]; exists {
		pair.HasBothFiles = true
	}
}

// GetReadyForVerification returns all file pairs that are ready for verification
// A pair is ready when BOTH files exist (data + .sha256)
func (ft *FileTracker) GetReadyForVerification() []FilePair {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()

	var ready []FilePair

	for _, pair := range ft.files {
		// Must have both files and paths must be set
		if pair.HasBothFiles && pair.DataFilePath != "" && pair.SHA256Path != "" {
			ready = append(ready, *pair)
		}
	}

	return ready
}

// GetExpiredFiles returns file pairs that have exceeded retry timeout
// These files should be moved to DLQ
func (ft *FileTracker) GetExpiredFiles() []FilePair {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()

	var expired []FilePair
	now := time.Now()

	for _, pair := range ft.files {
		// Calculate time elapsed since first seen
		elapsed := now.Sub(pair.FirstSeen)

		// If elapsed time exceeds retry timeout, mark as expired
		if elapsed >= ft.retryTimeout {
			expired = append(expired, *pair)
		}
	}

	return expired
}

// Remove removes a file pair from tracking
// This is called after successful verification or after moving to DLQ
func (ft *FileTracker) Remove(dataFile string) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	delete(ft.files, dataFile)
}

// RemoveByPath removes a file pair by its data file path
func (ft *FileTracker) RemoveByPath(dataFilePath string) {
	dataFile := filepath.Base(dataFilePath)
	ft.Remove(dataFile)
}

// GetPendingCount returns the number of file pairs currently being tracked
func (ft *FileTracker) GetPendingCount() int {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()

	return len(ft.files)
}

// GetFilePair returns a specific file pair by data filename
func (ft *FileTracker) GetFilePair(dataFile string) (*FilePair, bool) {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()

	pair, exists := ft.files[dataFile]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	pairCopy := *pair
	return &pairCopy, true
}

// IsExpired checks if a specific file pair has exceeded retry timeout
func (ft *FileTracker) IsExpired(dataFile string) bool {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()

	pair, exists := ft.files[dataFile]
	if !exists {
		return false
	}

	elapsed := time.Since(pair.FirstSeen)
	return elapsed >= ft.retryTimeout
}

// GetAllFiles returns all tracked file pairs (for debugging)
func (ft *FileTracker) GetAllFiles() []FilePair {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()

	var all []FilePair
	for _, pair := range ft.files {
		all = append(all, *pair)
	}

	return all
}

// Clear removes all tracked files (useful for testing)
func (ft *FileTracker) Clear() {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	ft.files = make(map[string]*FilePair)
}

// UpdateRetryTimeout updates the retry timeout duration
func (ft *FileTracker) UpdateRetryTimeout(timeout time.Duration) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	ft.retryTimeout = timeout
}
