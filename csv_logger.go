package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"sync"
	"time"
)

// CSVLogger handles buffered CSV logging for verification results and statistics
type CSVLogger struct {
	verificationFile   *os.File
	statsFile          *os.File
	verificationWriter *csv.Writer
	statsWriter        *csv.Writer
	flushInterval      time.Duration
	mutex              sync.Mutex
	stopChan           chan struct{}
	wg                 sync.WaitGroup
}

// NewCSVLogger creates a new CSV logger and starts the periodic flush routine
func NewCSVLogger(verificationFilePath, statsFilePath string, flushInterval time.Duration) (*CSVLogger, error) {
	// Open verification CSV file
	verificationFile, err := os.OpenFile(verificationFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open verification CSV file: %w", err)
	}

	// Open stats CSV file
	statsFile, err := os.OpenFile(statsFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		verificationFile.Close()
		return nil, fmt.Errorf("failed to open stats CSV file: %w", err)
	}

	// Create CSV writers
	verificationWriter := csv.NewWriter(verificationFile)
	statsWriter := csv.NewWriter(statsFile)

	logger := &CSVLogger{
		verificationFile:   verificationFile,
		statsFile:          statsFile,
		verificationWriter: verificationWriter,
		statsWriter:        statsWriter,
		flushInterval:      flushInterval,
		stopChan:           make(chan struct{}),
	}

	// Write headers if files are new (empty)
	if err := logger.writeHeadersIfNeeded(verificationFilePath, statsFilePath); err != nil {
		logger.Close()
		return nil, err
	}

	// Start periodic flush routine
	logger.wg.Add(1)
	go logger.periodicFlush()

	return logger, nil
}

// writeHeadersIfNeeded writes CSV headers if files are empty
func (l *CSVLogger) writeHeadersIfNeeded(verificationPath, statsPath string) error {
	// Check verification file size
	verificationInfo, err := os.Stat(verificationPath)
	if err != nil {
		return fmt.Errorf("failed to stat verification file: %w", err)
	}

	if verificationInfo.Size() == 0 {
		// Write verification CSV header
		header := []string{"Timestamp", "Filename", "SHA256", "Size_Bytes", "Size_KB", "Duration_Seconds"}
		if err := l.verificationWriter.Write(header); err != nil {
			return fmt.Errorf("failed to write verification header: %w", err)
		}
	}

	// Check stats file size
	statsInfo, err := os.Stat(statsPath)
	if err != nil {
		return fmt.Errorf("failed to stat stats file: %w", err)
	}

	if statsInfo.Size() == 0 {
		// Write stats CSV header
		header := []string{"Timestamp", "TotalProcessed", "SuccessCount", "FailureCount", "PendingCount", "AverageDuration"}
		if err := l.statsWriter.Write(header); err != nil {
			return fmt.Errorf("failed to write stats header: %w", err)
		}
	}

	return nil
}

// LogVerification logs a successful verification to the verification CSV
func (l *CSVLogger) LogVerification(entry CSVLogEntry) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	record := []string{
		entry.Timestamp,
		entry.Filename,
		entry.SHA256,
		fmt.Sprintf("%d", entry.SizeBytes),
		fmt.Sprintf("%.2f", entry.SizeKB),
		fmt.Sprintf("%.4f", entry.Duration),
	}

	if err := l.verificationWriter.Write(record); err != nil {
		return fmt.Errorf("failed to write verification record: %w", err)
	}

	return nil
}

// LogStats logs statistics to the stats CSV
func (l *CSVLogger) LogStats(entry StatsEntry) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	record := []string{
		entry.Timestamp,
		fmt.Sprintf("%d", entry.TotalProcessed),
		fmt.Sprintf("%d", entry.SuccessCount),
		fmt.Sprintf("%d", entry.FailureCount),
		fmt.Sprintf("%d", entry.PendingCount),
		fmt.Sprintf("%.4f", entry.AverageDuration),
	}

	if err := l.statsWriter.Write(record); err != nil {
		return fmt.Errorf("failed to write stats record: %w", err)
	}

	return nil
}

// Flush forces all buffered data to be written to disk
func (l *CSVLogger) Flush() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Flush verification writer
	l.verificationWriter.Flush()
	if err := l.verificationWriter.Error(); err != nil {
		return fmt.Errorf("failed to flush verification writer: %w", err)
	}

	// Flush stats writer
	l.statsWriter.Flush()
	if err := l.statsWriter.Error(); err != nil {
		return fmt.Errorf("failed to flush stats writer: %w", err)
	}

	// Sync file to disk
	if err := l.verificationFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync verification file: %w", err)
	}

	if err := l.statsFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync stats file: %w", err)
	}

	return nil
}

// periodicFlush flushes CSV data at regular intervals
func (l *CSVLogger) periodicFlush() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := l.Flush(); err != nil {
				fmt.Fprintf(os.Stderr, "Error during periodic flush: %v\n", err)
			}
		case <-l.stopChan:
			// Final flush before shutdown
			if err := l.Flush(); err != nil {
				fmt.Fprintf(os.Stderr, "Error during final flush: %v\n", err)
			}
			return
		}
	}
}

// Close stops the periodic flush routine and closes all files
func (l *CSVLogger) Close() error {
	// Signal stop to periodic flush routine
	close(l.stopChan)

	// Wait for periodic flush routine to complete
	l.wg.Wait()

	// Final flush
	if err := l.Flush(); err != nil {
		return err
	}

	// Close files
	var errs []error

	if err := l.verificationFile.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close verification file: %w", err))
	}

	if err := l.statsFile.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close stats file: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing CSV logger: %v", errs)
	}

	return nil
}

// CreateCSVLogEntry creates a CSVLogEntry from a VerificationResult
func CreateCSVLogEntry(result VerificationResult) CSVLogEntry {
	sizeKB := float64(result.Job.FilePair.DataSize) / 1024.0
	durationSeconds := result.Duration.Seconds()

	return CSVLogEntry{
		Timestamp: result.Timestamp.Format("2006-01-02 15:04:05"),
		Filename:  result.Job.FilePair.DataFile,
		SHA256:    result.ComputedHash,
		SizeBytes: result.Job.FilePair.DataSize,
		SizeKB:    sizeKB,
		Duration:  durationSeconds,
	}
}

// CreateStatsEntry creates a StatsEntry from Statistics
func CreateStatsEntry(stats Statistics) StatsEntry {
	avgDuration := 0.0
	if stats.TotalProcessed > 0 {
		avgDuration = stats.TotalDuration.Seconds() / float64(stats.TotalProcessed)
	}

	return StatsEntry{
		Timestamp:       time.Now().Format("2006-01-02 15:04:05"),
		TotalProcessed:  stats.TotalProcessed,
		SuccessCount:    stats.SuccessCount,
		FailureCount:    stats.FailureCount,
		PendingCount:    stats.PendingCount,
		AverageDuration: avgDuration,
	}
}
