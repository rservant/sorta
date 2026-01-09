// Package scanner handles directory scanning for Sorta.
package scanner

import (
	"errors"
	"os"
	"path/filepath"
)

// ScanErrorType represents the type of scanning error.
type ScanErrorType string

const (
	// DirectoryNotFound indicates the directory does not exist.
	DirectoryNotFound ScanErrorType = "DIRECTORY_NOT_FOUND"
	// PermissionDenied indicates insufficient permissions to read the directory.
	PermissionDenied ScanErrorType = "PERMISSION_DENIED"
)

// ScanError represents an error that occurred during directory scanning.
type ScanError struct {
	Type ScanErrorType
	Path string
	Err  error
}

func (e *ScanError) Error() string {
	return string(e.Type) + ": " + e.Path
}

func (e *ScanError) Unwrap() error {
	return e.Err
}

// FileEntry represents a file found during scanning.
type FileEntry struct {
	Name     string // Filename only
	FullPath string // Absolute path
}

// Scan enumerates files in the given directory without recursion.
// It returns only files, excluding subdirectories.
func Scan(directory string) ([]FileEntry, error) {
	// Check if directory exists
	info, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ScanError{
				Type: DirectoryNotFound,
				Path: directory,
				Err:  err,
			}
		}
		if os.IsPermission(err) {
			return nil, &ScanError{
				Type: PermissionDenied,
				Path: directory,
				Err:  err,
			}
		}
		return nil, err
	}

	// Verify it's a directory
	if !info.IsDir() {
		return nil, &ScanError{
			Type: DirectoryNotFound,
			Path: directory,
			Err:  errors.New("path is not a directory"),
		}
	}

	// Read directory entries
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsPermission(err) {
			return nil, &ScanError{
				Type: PermissionDenied,
				Path: directory,
				Err:  err,
			}
		}
		return nil, err
	}

	// Collect only files (not directories)
	var files []FileEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(directory, entry.Name())
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			absPath = fullPath
		}

		files = append(files, FileEntry{
			Name:     entry.Name(),
			FullPath: absPath,
		})
	}

	return files, nil
}
