package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

/*
WorkerPool manages concurrent verification workers.

Responsibilities:
1. Maintain a fixed pool of worker goroutines (e.g., 10 workers)
2. Distribute verification jobs to available workers via buffered channel
3. Orchestrate the verification process:
   - Call sha_verifier.go to verify hash
   - Call file_operations.go to move/delete files
   - Call csv_logger.go to log results
   - Call statistics.go to update metrics
4. Handle both success and failure cases
5. Graceful start/stop with proper cleanup

Does NOT:
- Scan for files (that's file_scanner.go)
- Track file pairs (that's file_tracker.go)
- Compute hashes directly (delegates to sha_verifier.go)
*/

// WorkerPoolManager manages the worker pool lifecycle
type WorkerPoolManager struct {
	jobQueue         chan VerificationJob
	numWorkers       int
	csvLogger        *CSVLogger
	statsTracker     *StatsTracker
	fileTracker      *FileTracker
	verifiedFolder   string
	dlqFolder        string
	removeFromSource bool
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	logLevel         string
}

// NewWorkerPoolManager creates a new worker pool manager
func NewWorkerPoolManager(
	queueSize int,
	numWorkers int,
	csvLogger *CSVLogger,
	statsTracker *StatsTracker,
	fileTracker *FileTracker,
	verifiedFolder string,
	dlqFolder string,
	removeFromSource bool,
	logLevel string,
) *WorkerPoolManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPoolManager{
		jobQueue:         make(chan VerificationJob, queueSize),
		numWorkers:       numWorkers,
		csvLogger:        csvLogger,
		statsTracker:     statsTracker,
		fileTracker:      fileTracker,
		verifiedFolder:   verifiedFolder,
		dlqFolder:        dlqFolder,
		removeFromSource: removeFromSource,
		ctx:              ctx,
		cancel:           cancel,
		logLevel:         logLevel,
	}
}

// Start launches all worker goroutines
func (wpm *WorkerPoolManager) Start() {
	for i := 0; i < wpm.numWorkers; i++ {
		wpm.wg.Add(1)
		go wpm.worker(i)
	}

	if wpm.logLevel == "DEBUG" || wpm.logLevel == "INFO" {
		fmt.Printf("[WorkerPool] Started %d workers\n", wpm.numWorkers)
	}
}

// Stop gracefully stops all workers
func (wpm *WorkerPoolManager) Stop() {
	// Close job queue to signal workers to finish
	close(wpm.jobQueue)

	// Wait for all workers to complete
	wpm.wg.Wait()

	// Cancel context
	wpm.cancel()

	if wpm.logLevel == "DEBUG" || wpm.logLevel == "INFO" {
		fmt.Println("[WorkerPool] All workers stopped")
	}
}

// SubmitJob submits a verification job to the worker pool
// Returns true if job was submitted, false if queue is full
func (wpm *WorkerPoolManager) SubmitJob(job VerificationJob) bool {
	select {
	case wpm.jobQueue <- job:
		return true
	default:
		// Queue is full
		if wpm.logLevel == "WARN" || wpm.logLevel == "DEBUG" {
			fmt.Fprintf(os.Stderr, "[WorkerPool] Queue full, dropping job for %s\n", job.FilePair.DataFile)
		}
		return false
	}
}

// SubmitJobBlocking submits a job and blocks until it's accepted
func (wpm *WorkerPoolManager) SubmitJobBlocking(job VerificationJob) {
	wpm.jobQueue <- job
}

// worker is the main worker goroutine that processes verification jobs
func (wpm *WorkerPoolManager) worker(workerID int) {
	defer wpm.wg.Done()

	if wpm.logLevel == "DEBUG" {
		fmt.Printf("[Worker %d] Started\n", workerID)
	}

	for job := range wpm.jobQueue {
		wpm.processJob(workerID, job)
	}

	if wpm.logLevel == "DEBUG" {
		fmt.Printf("[Worker %d] Stopped\n", workerID)
	}
}

// processJob processes a single verification job
func (wpm *WorkerPoolManager) processJob(workerID int, job VerificationJob) {
	startTime := time.Now()

	if wpm.logLevel == "DEBUG" {
		fmt.Printf("[Worker %d] Processing %s\n", workerID, job.FilePair.DataFile)
	}

	// Check if files still exist (they might have been moved/deleted)
	if !FileExists(job.FilePair.DataFilePath) || !FileExists(job.FilePair.SHA256Path) {
		if wpm.logLevel == "DEBUG" {
			fmt.Printf("[Worker %d] Files no longer exist for %s, skipping\n", workerID, job.FilePair.DataFile)
		}
		wpm.fileTracker.Remove(job.FilePair.DataFile)
		return
	}

	// Perform SHA256 verification
	computedHash, expectedHash, err := VerifyFile(
		job.FilePair.DataFilePath,
		job.FilePair.SHA256Path,
		job.BufferSize,
	)

	duration := time.Since(startTime)

	// Create verification result
	result := VerificationResult{
		Job:          job,
		Success:      err == nil,
		ComputedHash: computedHash,
		ExpectedHash: expectedHash,
		Duration:     duration,
		Timestamp:    time.Now(),
	}

	if err != nil {
		result.ErrorMessage = err.Error()
	}

	// Handle result
	if result.Success {
		wpm.handleSuccess(workerID, result)
	} else {
		wpm.handleFailure(workerID, result)
	}
}

// handleSuccess handles a successful verification
func (wpm *WorkerPoolManager) handleSuccess(workerID int, result VerificationResult) {
	if wpm.logLevel == "DEBUG" || wpm.logLevel == "INFO" {
		fmt.Printf("[Worker %d] ✓ SUCCESS: %s (%.2f KB, %.3fs)\n",
			workerID,
			result.Job.FilePair.DataFile,
			float64(result.Job.FilePair.DataSize)/1024.0,
			result.Duration.Seconds())
	}

	// Move data file to verified folder
	newPath, err := MoveToVerified(result.Job.FilePair.DataFilePath, wpm.verifiedFolder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Worker %d] Failed to move %s to verified folder: %v\n",
			workerID, result.Job.FilePair.DataFile, err)
		wpm.statsTracker.IncrementFailure(result.Duration)
		return
	}

	if wpm.logLevel == "DEBUG" {
		fmt.Printf("[Worker %d] Moved to: %s\n", workerID, newPath)
	}

	// Delete SHA256 file from source
	if err := DeleteFile(result.Job.FilePair.SHA256Path); err != nil {
		fmt.Fprintf(os.Stderr, "[Worker %d] Failed to delete SHA256 file %s: %v\n",
			workerID, result.Job.FilePair.SHA256File, err)
		// Continue anyway - data file was moved successfully
	}

	// Remove from tracker
	wpm.fileTracker.Remove(result.Job.FilePair.DataFile)

	// Update statistics
	wpm.statsTracker.IncrementSuccess(result.Duration)

	// Log to CSV
	csvEntry := CreateCSVLogEntry(result)
	if err := wpm.csvLogger.LogVerification(csvEntry); err != nil {
		fmt.Fprintf(os.Stderr, "[Worker %d] Failed to log verification: %v\n", workerID, err)
	}
}

// handleFailure handles a failed verification
func (wpm *WorkerPoolManager) handleFailure(workerID int, result VerificationResult) {
	if wpm.logLevel == "DEBUG" || wpm.logLevel == "WARN" {
		fmt.Fprintf(os.Stderr, "[Worker %d] ✗ FAILURE: %s - %s\n",
			workerID, result.Job.FilePair.DataFile, result.ErrorMessage)
		fmt.Fprintf(os.Stderr, "[Worker %d]   Expected: %s\n", workerID, result.ExpectedHash)
		fmt.Fprintf(os.Stderr, "[Worker %d]   Computed: %s\n", workerID, result.ComputedHash)
	}

	// Check if retry deadline has been exceeded
	if time.Now().After(result.Job.RetryDeadline) {
		// Retry timeout exceeded, move to DLQ
		if wpm.logLevel == "INFO" || wpm.logLevel == "DEBUG" {
			fmt.Printf("[Worker %d] Retry timeout exceeded for %s, moving to DLQ\n",
				workerID, result.Job.FilePair.DataFile)
		}

		if err := MoveToDLQ(result.Job.FilePair.DataFilePath, result.Job.FilePair.SHA256Path, wpm.dlqFolder); err != nil {
			fmt.Fprintf(os.Stderr, "[Worker %d] Failed to move %s to DLQ: %v\n",
				workerID, result.Job.FilePair.DataFile, err)
		} else {
			if wpm.logLevel == "DEBUG" {
				fmt.Printf("[Worker %d] Moved to DLQ: %s\n", workerID, result.Job.FilePair.DataFile)
			}
		}

		// Remove from tracker
		wpm.fileTracker.Remove(result.Job.FilePair.DataFile)

		// Update statistics
		wpm.statsTracker.IncrementFailure(result.Duration)
	} else {
		// Retry deadline not exceeded yet, keep in tracker for retry
		if wpm.logLevel == "DEBUG" {
			timeRemaining := time.Until(result.Job.RetryDeadline)
			fmt.Printf("[Worker %d] Will retry %s (%.0f seconds remaining)\n",
				workerID, result.Job.FilePair.DataFile, timeRemaining.Seconds())
		}
		// File remains in tracker, will be picked up in next scan
	}
}

// GetQueueLength returns the current number of jobs in the queue
func (wpm *WorkerPoolManager) GetQueueLength() int {
	return len(wpm.jobQueue)
}

// GetQueueCapacity returns the maximum queue capacity
func (wpm *WorkerPoolManager) GetQueueCapacity() int {
	return cap(wpm.jobQueue)
}
