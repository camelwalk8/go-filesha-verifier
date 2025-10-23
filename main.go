package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

/*
Main application entry point.

Orchestrates all components:
1. Load configuration
2. Initialize CSV logger, statistics tracker, file tracker
3. Start file scanner
4. Start worker pool
5. Run coordinator loop that:
   - Submits ready files to worker pool
   - Handles expired files (move to DLQ)
   - Logs periodic statistics
6. Handle graceful shutdown on SIGINT/SIGTERM
*/

// Build-time variables injected via -ldflags during compilation
var (
	version   string // Application version (e.g., "v1.0.0")
	buildTime string // Build timestamp in ISO 8601 format
	buildID   string // SHA256 hash of the compiled binary
	release   string // Release type: PRODUCTION, DEVELOPMENT, etc.
)

func main() {

	PrintVersionInfo("Go FTP Transfer Service", release, version, buildTime, buildID)

	if release == "PRODUCTION" {
		fmt.Println("\nValidating release version...")
		if err := ValidateVersion(version, buildTime, buildID); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Release verification failed\n%v\n", err)
			fmt.Fprintln(os.Stderr, "\nThis indicates a potential version mismatch or tampering.")
			fmt.Fprintln(os.Stderr, "Please ensure you're running the correct binary with matching release notes.")
			os.Exit(1)
		}
		fmt.Println("✓ Release verification successful - Binary matches release notes")
	} else {
		fmt.Printf("\nRunning in %s mode - Skipping version validation\n", release)
	}

	// Load configuration
	config, err := LoadConfig("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Print configuration
	PrintConfig(config)

	// Initialize CSV logger
	csvLogger, err := NewCSVLogger(
		config.Spec.Output.VerificationFile,
		config.Spec.Output.StatsFile,
		config.Spec.Output.FlushInterval,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create CSV logger: %v\n", err)
		os.Exit(1)
	}
	defer csvLogger.Close()

	// Initialize statistics tracker
	statsTracker := NewStatsTracker()

	// Initialize file tracker
	fileTracker := NewFileTracker(config.Spec.Verification.RetryTimeout)

	// Initialize file scanner
	scanner := NewFileScanner(
		config.Spec.Source.Folder,
		config.Spec.Source.PeriodicScanInterval,
		config.Spec.Verification.FileFilters,
		fileTracker,
		config.Spec.Logging.Level,
	)

	// Initialize worker pool
	workerPool := NewWorkerPoolManager(
		config.Spec.Concurrency.QueueSize,
		config.Spec.Concurrency.Workers,
		csvLogger,
		statsTracker,
		fileTracker,
		config.Spec.Destination.VerifiedFolder,
		config.Spec.Destination.DlqFolder,
		config.Spec.Destination.RemoveFromSource,
		config.Spec.Logging.Level,
	)

	// Start components
	scanner.Start()
	workerPool.Start()

	fmt.Println("=== Application Started ===")
	fmt.Printf("Press Ctrl+C to stop\n")
	fmt.Println("===========================")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create context for coordinator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run coordinator in background
	coordinatorDone := make(chan struct{})
	go coordinator(ctx, config, fileTracker, workerPool, statsTracker, csvLogger, coordinatorDone)

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n[Main] Shutdown signal received, stopping gracefully...")

	// Cancel coordinator context
	cancel()

	// Wait for coordinator to finish
	<-coordinatorDone

	// Stop scanner
	scanner.Stop()

	// Stop worker pool
	workerPool.Stop()

	// Final statistics
	fmt.Println("\n=== Final Statistics ===")
	statsTracker.PrintStatistics()
	fmt.Println("========================")

	fmt.Println("[Main] Shutdown complete")
}

// coordinator is the main control loop that submits jobs and handles timeouts
func coordinator(
	ctx context.Context,
	config *Config,
	fileTracker *FileTracker,
	workerPool *WorkerPoolManager,
	statsTracker *StatsTracker,
	csvLogger *CSVLogger,
	done chan struct{},
) {
	defer close(done)

	// Coordinator runs every 1 second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Stats logging ticker
	statsTicker := time.NewTicker(30 * time.Second)
	defer statsTicker.Stop()

	retryTimeout := config.Spec.Verification.RetryTimeout
	bufferSize := config.Spec.Verification.BufferSize
	logLevel := config.Spec.Logging.Level

	for {
		select {
		case <-ticker.C:
			// Get files ready for verification
			readyFiles := fileTracker.GetReadyForVerification()

			if logLevel == "DEBUG" && len(readyFiles) > 0 {
				fmt.Printf("[Coordinator] Found %d files ready for verification\n", len(readyFiles))
			}

			// Submit verification jobs
			for _, filePair := range readyFiles {
				// Calculate retry deadline based on first seen time
				retryDeadline := filePair.FirstSeen.Add(retryTimeout)

				// Create verification job
				job := VerificationJob{
					FilePair:      filePair,
					RetryDeadline: retryDeadline,
					BufferSize:    bufferSize,
				}

				// Submit job to worker pool
				if !workerPool.SubmitJob(job) {
					if logLevel == "WARN" || logLevel == "DEBUG" {
						fmt.Fprintf(os.Stderr, "[Coordinator] Worker queue full, job for %s will retry later\n", filePair.DataFile)
					}
				}
			}

			// Update pending count in statistics
			pendingCount := int64(fileTracker.GetPendingCount())
			statsTracker.SetPendingCount(pendingCount)

		case <-statsTicker.C:
			// Log periodic statistics
			stats := statsTracker.GetStatistics()
			statsEntry := CreateStatsEntry(stats)
			if err := csvLogger.LogStats(statsEntry); err != nil {
				fmt.Fprintf(os.Stderr, "[Coordinator] Failed to log stats: %v\n", err)
			}

			if logLevel == "INFO" || logLevel == "DEBUG" {
				fmt.Printf("[Stats] Processed: %d | Success: %d | Failed: %d | Pending: %d | Queue: %d/%d\n",
					stats.TotalProcessed,
					stats.SuccessCount,
					stats.FailureCount,
					stats.PendingCount,
					workerPool.GetQueueLength(),
					workerPool.GetQueueCapacity(),
				)
			}

		case <-ctx.Done():
			// Shutdown signal received
			if logLevel == "DEBUG" || logLevel == "INFO" {
				fmt.Println("[Coordinator] Stopping...")
			}
			return
		}
	}
}
