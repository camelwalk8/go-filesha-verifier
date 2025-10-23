package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MoveToVerified moves a successfully verified data file to the verified folder
// Returns the new file path or an error
func MoveToVerified(sourceFilePath, verifiedFolder string) (string, error) {
	// Get the filename from the source path
	filename := filepath.Base(sourceFilePath)

	// Build destination path
	destPath := filepath.Join(verifiedFolder, filename)

	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		// File exists, create unique name with timestamp
		destPath = getUniqueFilePath(verifiedFolder, filename)
	}

	// Move file (rename if on same filesystem, otherwise copy+delete)
	if err := moveFile(sourceFilePath, destPath); err != nil {
		return "", fmt.Errorf("failed to move file to verified folder: %w", err)
	}

	return destPath, nil
}

// MoveToDLQ moves both data file and SHA256 file to the DLQ folder
// Returns error if either move fails
func MoveToDLQ(dataFilePath, sha256FilePath, dlqFolder string) error {
	// Move data file
	dataFilename := filepath.Base(dataFilePath)
	dataDest := filepath.Join(dlqFolder, dataFilename)

	// Check if destination already exists
	if _, err := os.Stat(dataDest); err == nil {
		dataDest = getUniqueFilePath(dlqFolder, dataFilename)
	}

	if err := moveFile(dataFilePath, dataDest); err != nil {
		return fmt.Errorf("failed to move data file to DLQ: %w", err)
	}

	// Move SHA256 file
	sha256Filename := filepath.Base(sha256FilePath)
	sha256Dest := filepath.Join(dlqFolder, sha256Filename)

	// Check if destination already exists
	if _, err := os.Stat(sha256Dest); err == nil {
		sha256Dest = getUniqueFilePath(dlqFolder, sha256Filename)
	}

	if err := moveFile(sha256FilePath, sha256Dest); err != nil {
		// Data file already moved, log warning but continue
		return fmt.Errorf("failed to move SHA256 file to DLQ: %w", err)
	}

	return nil
}

// DeleteFile removes a file from the filesystem
func DeleteFile(filePath string) error {
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}
	return nil
}

// DeleteBothFiles removes both data and SHA256 files
func DeleteBothFiles(dataFilePath, sha256FilePath string) error {
	// Delete data file
	if err := DeleteFile(dataFilePath); err != nil {
		return err
	}

	// Delete SHA256 file
	if err := DeleteFile(sha256FilePath); err != nil {
		return err
	}

	return nil
}

// moveFile moves a file from source to destination
// Uses os.Rename for same filesystem, otherwise copies and deletes
func moveFile(sourcePath, destPath string) error {
	// Try rename first (fast, atomic on same filesystem)
	err := os.Rename(sourcePath, destPath)
	if err == nil {
		return nil
	}

	// Rename failed (possibly cross-filesystem), do copy+delete
	if err := copyFile(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Delete source after successful copy
	if err := os.Remove(sourcePath); err != nil {
		return fmt.Errorf("failed to delete source file after copy: %w", err)
	}

	return nil
}

// copyFile copies a file from source to destination
func copyFile(sourcePath, destPath string) error {
	// Open source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy contents
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	// Copy file permissions
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	if err := os.Chmod(destPath, sourceInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set destination file permissions: %w", err)
	}

	return nil
}

// getUniqueFilePath generates a unique file path by appending timestamp
func getUniqueFilePath(dir, filename string) string {
	ext := filepath.Ext(filename)
	nameWithoutExt := filename[:len(filename)-len(ext)]

	// Use nanosecond timestamp for uniqueness
	timestamp := fmt.Sprintf("%d", os.Getpid())
	uniqueName := fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext)

	return filepath.Join(dir, uniqueName)
}

// FileExists checks if a file exists
func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// GetFileSize returns the size of a file in bytes
func GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file size: %w", err)
	}
	return info.Size(), nil
}
