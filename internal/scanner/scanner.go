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
	// SymlinkError indicates a symlink was encountered with "error" policy.
	SymlinkError ScanErrorType = "SYMLINK_ERROR"
)

// Symlink policy constants
const (
	SymlinkPolicyFollow = "follow"
	SymlinkPolicySkip   = "skip"
	SymlinkPolicyError  = "error"
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

// ScanOptions configures scanning behavior.
type ScanOptions struct {
	MaxDepth      int    // Maximum depth to scan (0 = immediate only, -1 = unlimited)
	SymlinkPolicy string // "follow", "skip", or "error"
}

// DefaultScanOptions returns the default scan options.
func DefaultScanOptions() ScanOptions {
	return ScanOptions{
		MaxDepth:      0,
		SymlinkPolicy: SymlinkPolicySkip,
	}
}

// FileEntry represents a file found during scanning.
type FileEntry struct {
	Name     string // Filename only
	FullPath string // Absolute path
}

// Scan enumerates files in the given directory without recursion.
// It returns only files, excluding subdirectories.
// This is a convenience wrapper around ScanWithOptions with default options.
func Scan(directory string) ([]FileEntry, error) {
	return ScanWithOptions(directory, DefaultScanOptions())
}

// ScanWithOptions scans directory with configurable options.
func ScanWithOptions(directory string, opts ScanOptions) ([]FileEntry, error) {
	// Check if directory exists
	info, err := os.Lstat(directory)
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

	// Handle symlink at root directory level
	if info.Mode()&os.ModeSymlink != 0 {
		switch opts.SymlinkPolicy {
		case SymlinkPolicyError:
			return nil, &ScanError{
				Type: SymlinkError,
				Path: directory,
				Err:  errors.New("symlink encountered with error policy"),
			}
		case SymlinkPolicySkip:
			return []FileEntry{}, nil
		case SymlinkPolicyFollow:
			// Follow the symlink
			info, err = os.Stat(directory)
			if err != nil {
				return nil, err
			}
		}
	}

	// Verify it's a directory
	if !info.IsDir() {
		return nil, &ScanError{
			Type: DirectoryNotFound,
			Path: directory,
			Err:  errors.New("path is not a directory"),
		}
	}

	return scanDirectory(directory, opts, 0)
}

// scanDirectory recursively scans a directory up to the specified depth.
func scanDirectory(directory string, opts ScanOptions, currentDepth int) ([]FileEntry, error) {
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

	var files []FileEntry
	for _, entry := range entries {
		fullPath := filepath.Join(directory, entry.Name())
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			absPath = fullPath
		}

		// Check if entry is a symlink
		info, err := os.Lstat(fullPath)
		if err != nil {
			continue // Skip entries we can't stat
		}

		isSymlink := info.Mode()&os.ModeSymlink != 0

		if isSymlink {
			switch opts.SymlinkPolicy {
			case SymlinkPolicyError:
				return nil, &ScanError{
					Type: SymlinkError,
					Path: fullPath,
					Err:  errors.New("symlink encountered with error policy"),
				}
			case SymlinkPolicySkip:
				continue // Skip this entry
			case SymlinkPolicyFollow:
				// Follow the symlink to get the target info
				info, err = os.Stat(fullPath)
				if err != nil {
					continue // Skip broken symlinks
				}
			}
		}

		if info.IsDir() {
			// Check if we should recurse into subdirectories
			// MaxDepth of -1 means unlimited, 0 means immediate only
			if opts.MaxDepth == -1 || currentDepth < opts.MaxDepth {
				subFiles, err := scanDirectory(fullPath, opts, currentDepth+1)
				if err != nil {
					return nil, err
				}
				files = append(files, subFiles...)
			}
			continue
		}

		files = append(files, FileEntry{
			Name:     entry.Name(),
			FullPath: absPath,
		})
	}

	return files, nil
}
