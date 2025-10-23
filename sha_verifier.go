package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadSHA256File reads the expected SHA256 hash from a .sha256 file
// The file typically contains the hash in hex format, optionally followed by the filename
// Example formats:
//
//	abc123def456...  data.zip
//	abc123def456...
func ReadSHA256File(sha256Path string) (string, error) {
	data, err := os.ReadFile(sha256Path)
	if err != nil {
		return "", fmt.Errorf("failed to read SHA256 file: %w", err)
	}

	// Convert to string and clean up
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", fmt.Errorf("SHA256 file is empty")
	}

	// Split by whitespace (hash might be followed by filename)
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return "", fmt.Errorf("SHA256 file has invalid format")
	}

	// First part is the hash
	hash := strings.ToLower(parts[0])

	// Validate hash format (should be 64 hex characters for SHA256)
	if len(hash) != 64 {
		return "", fmt.Errorf("invalid SHA256 hash length: expected 64, got %d", len(hash))
	}

	// Validate it's a valid hex string
	if _, err := hex.DecodeString(hash); err != nil {
		return "", fmt.Errorf("invalid SHA256 hash format: %w", err)
	}

	return hash, nil
}

// ComputeFileSHA256 computes the SHA256 hash of a file using the specified buffer size
// Returns the hash in lowercase hexadecimal format
func ComputeFileSHA256(filePath string, bufferSize int) (string, error) {
	// Open file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create SHA256 hasher
	hasher := sha256.New()

	// Create buffer with specified size for efficient reading
	buffer := make([]byte, bufferSize)

	// Read file in chunks and update hash
	for {
		bytesRead, err := file.Read(buffer)
		if bytesRead > 0 {
			hasher.Write(buffer[:bytesRead])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
	}

	// Get final hash
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return hashString, nil
}

// VerifyFile verifies that a data file matches its SHA256 checksum
// Returns computed hash, expected hash, and any error
func VerifyFile(dataFilePath, sha256FilePath string, bufferSize int) (computed string, expected string, err error) {
	// Read expected hash from .sha256 file
	expectedHash, err := ReadSHA256File(sha256FilePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read expected hash: %w", err)
	}

	// Compute actual hash of data file
	computedHash, err := ComputeFileSHA256(dataFilePath, bufferSize)
	if err != nil {
		return "", expectedHash, fmt.Errorf("failed to compute hash: %w", err)
	}

	// Compare hashes (case-insensitive)
	if strings.ToLower(computedHash) != strings.ToLower(expectedHash) {
		return computedHash, expectedHash, fmt.Errorf("hash mismatch")
	}

	return computedHash, expectedHash, nil
}

// VerifyFileMatch is a boolean helper that returns true if verification succeeds
func VerifyFileMatch(dataFilePath, sha256FilePath string, bufferSize int) bool {
	_, _, err := VerifyFile(dataFilePath, sha256FilePath, bufferSize)
	return err == nil
}
