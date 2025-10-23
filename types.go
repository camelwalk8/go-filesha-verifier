package main

import "time"

// ============================================================================
// Configuration Types
// ============================================================================

// Config represents the complete application configuration
type Config struct {
	AppVersion  string `yaml:"appVersion"`
	Kind        string `yaml:"kind"`
	AppName     string `yaml:"appName"`
	Description string `yaml:"description"`
	Spec        Spec   `yaml:"spec"`
}

// Spec contains all operational specifications
type Spec struct {
	Source       SourceConfig       `yaml:"source"`
	Verification VerificationConfig `yaml:"verification"`
	Destination  DestinationConfig  `yaml:"destination"`
	Concurrency  ConcurrencyConfig  `yaml:"concurrency"`
	Output       OutputConfig       `yaml:"output"`
	Logging      LoggingConfig      `yaml:"logging"`
}

// SourceConfig defines source folder settings
type SourceConfig struct {
	Folder               string        `yaml:"folder"`
	PeriodicScanInterval time.Duration `yaml:"periodicScanInterval"`
}

// VerificationConfig defines verification behavior
type VerificationConfig struct {
	RetryTimeout time.Duration `yaml:"retryTimeout"`
	BufferSize   int           `yaml:"bufferSize"`
	FileFilters  []string      `yaml:"fileFilters"`
}

// DestinationConfig defines destination folders
type DestinationConfig struct {
	VerifiedFolder   string `yaml:"verifiedFolder"`
	DlqFolder        string `yaml:"dlqFolder"`
	RemoveFromSource bool   `yaml:"removeFromSource"`
}

// ConcurrencyConfig defines worker pool settings
type ConcurrencyConfig struct {
	Workers   int `yaml:"workers"`
	QueueSize int `yaml:"queueSize"`
}

// OutputConfig defines logging output settings
type OutputConfig struct {
	VerificationFile string        `yaml:"verificationFile"`
	StatsFile        string        `yaml:"statsFile"`
	FlushInterval    time.Duration `yaml:"flushInterval"`
}

// LoggingConfig defines logging level
type LoggingConfig struct {
	Level string `yaml:"level"`
}

// ============================================================================
// Domain Types
// ============================================================================

// FilePair represents a data file and its corresponding SHA256 file
type FilePair struct {
	DataFile     string    // e.g., "data.zip"
	DataFilePath string    // Full path to data file
	SHA256File   string    // e.g., "data.zip.sha256"
	SHA256Path   string    // Full path to SHA256 file
	DataSize     int64     // Size in bytes
	FirstSeen    time.Time // When first detected
	HasBothFiles bool      // True when both data and .sha256 exist
}

// VerificationJob represents a job to be processed by workers
type VerificationJob struct {
	FilePair      FilePair
	RetryDeadline time.Time // Time when we give up and move to DLQ
	BufferSize    int       // Buffer size for reading file
}

// VerificationResult represents the outcome of a verification attempt
type VerificationResult struct {
	Job          VerificationJob
	Success      bool
	ErrorMessage string
	ComputedHash string
	ExpectedHash string
	Duration     time.Duration
	Timestamp    time.Time
}

// ============================================================================
// CSV Log Entry Types
// ============================================================================

// CSVLogEntry represents a single row in verification.csv
type CSVLogEntry struct {
	Timestamp string
	Filename  string
	SHA256    string
	SizeBytes int64
	SizeKB    float64
	Duration  float64 // seconds
}

// StatsEntry represents a single row in stats.csv
type StatsEntry struct {
	Timestamp       string
	TotalProcessed  int64
	SuccessCount    int64
	FailureCount    int64
	PendingCount    int64
	AverageDuration float64
}

// ============================================================================
// Statistics Types
// ============================================================================

// Statistics tracks runtime metrics
type Statistics struct {
	TotalProcessed int64
	SuccessCount   int64
	FailureCount   int64
	PendingCount   int64
	TotalDuration  time.Duration
	StartTime      time.Time
}

// ============================================================================
// File Tracker Types
// ============================================================================

// FileTrackerState maintains state of all tracked files
type FileTrackerState struct {
	Files         map[string]*FilePair // Key: data filename (without .sha256)
	RetryDeadline time.Duration        // How long to retry before DLQ
}

// ============================================================================
// Worker Pool Types
// ============================================================================

// WorkerPool manages concurrent verification workers
type WorkerPool struct {
	JobQueue    chan VerificationJob
	ResultQueue chan VerificationResult
	Workers     int
}
