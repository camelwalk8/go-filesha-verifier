package main

import (
	"sync"
	"time"
)

// StatsTracker manages runtime statistics for file verification operations
type StatsTracker struct {
	mutex          sync.RWMutex
	totalProcessed int64
	successCount   int64
	failureCount   int64
	pendingCount   int64
	totalDuration  time.Duration
	startTime      time.Time
}

// NewStatsTracker creates a new statistics tracker
func NewStatsTracker() *StatsTracker {
	return &StatsTracker{
		startTime: time.Now(),
	}
}

// IncrementSuccess increments the success counter and updates total duration
func (s *StatsTracker) IncrementSuccess(duration time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.successCount++
	s.totalProcessed++
	s.totalDuration += duration
}

// IncrementFailure increments the failure counter and updates total duration
func (s *StatsTracker) IncrementFailure(duration time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.failureCount++
	s.totalProcessed++
	s.totalDuration += duration
}

// SetPendingCount sets the current number of pending files
func (s *StatsTracker) SetPendingCount(count int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.pendingCount = count
}

// IncrementPending increments the pending counter
func (s *StatsTracker) IncrementPending() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.pendingCount++
}

// DecrementPending decrements the pending counter
func (s *StatsTracker) DecrementPending() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.pendingCount > 0 {
		s.pendingCount--
	}
}

// GetStatistics returns a snapshot of current statistics
func (s *StatsTracker) GetStatistics() Statistics {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return Statistics{
		TotalProcessed: s.totalProcessed,
		SuccessCount:   s.successCount,
		FailureCount:   s.failureCount,
		PendingCount:   s.pendingCount,
		TotalDuration:  s.totalDuration,
		StartTime:      s.startTime,
	}
}

// GetAverageDuration returns the average processing duration
func (s *StatsTracker) GetAverageDuration() time.Duration {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.totalProcessed == 0 {
		return 0
	}

	return s.totalDuration / time.Duration(s.totalProcessed)
}

// GetSuccessRate returns the success rate as a percentage (0-100)
func (s *StatsTracker) GetSuccessRate() float64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.totalProcessed == 0 {
		return 0.0
	}

	return (float64(s.successCount) / float64(s.totalProcessed)) * 100.0
}

// GetFailureRate returns the failure rate as a percentage (0-100)
func (s *StatsTracker) GetFailureRate() float64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.totalProcessed == 0 {
		return 0.0
	}

	return (float64(s.failureCount) / float64(s.totalProcessed)) * 100.0
}

// GetUptime returns how long the tracker has been running
func (s *StatsTracker) GetUptime() time.Duration {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return time.Since(s.startTime)
}

// GetProcessingRate returns the number of files processed per second
func (s *StatsTracker) GetProcessingRate() float64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	uptime := time.Since(s.startTime).Seconds()
	if uptime == 0 {
		return 0.0
	}

	return float64(s.totalProcessed) / uptime
}

// Reset resets all counters (useful for testing)
func (s *StatsTracker) Reset() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.totalProcessed = 0
	s.successCount = 0
	s.failureCount = 0
	s.pendingCount = 0
	s.totalDuration = 0
	s.startTime = time.Now()
}

// PrintStatistics prints a formatted summary of current statistics
func (s *StatsTracker) PrintStatistics() {
	stats := s.GetStatistics()
	avgDuration := s.GetAverageDuration()
	successRate := s.GetSuccessRate()
	failureRate := s.GetFailureRate()
	uptime := s.GetUptime()
	processingRate := s.GetProcessingRate()

	println("=== Statistics ===")
	println("Total Processed: ", stats.TotalProcessed)
	println("Success Count:   ", stats.SuccessCount)
	println("Failure Count:   ", stats.FailureCount)
	println("Pending Count:   ", stats.PendingCount)
	println("Success Rate:    ", successRate, "%")
	println("Failure Rate:    ", failureRate, "%")
	println("Average Duration:", avgDuration.String())
	println("Processing Rate: ", processingRate, " files/sec")
	println("Uptime:          ", uptime.String())
	println("==================")
}
