// Package watcher provides file system monitoring for automatic file organization.
package watcher

import (
	"path/filepath"
	"strings"
)

// DefaultIgnorePatterns returns the default patterns for temporary files to ignore.
func DefaultIgnorePatterns() []string {
	return []string{
		"*.tmp",
		"*.part",
		"*.download",
		"*.crdownload", // Chrome partial downloads
		"*.partial",    // Generic partial file
		".~*",          // Hidden temp files (e.g., .~lock)
	}
}

// FileFilter handles filtering of files based on ignore patterns.
type FileFilter struct {
	patterns []string
}

// NewFileFilter creates a new FileFilter with the given patterns.
// If patterns is nil or empty, default patterns are used.
func NewFileFilter(patterns []string) *FileFilter {
	if len(patterns) == 0 {
		patterns = DefaultIgnorePatterns()
	}
	return &FileFilter{
		patterns: patterns,
	}
}

// ShouldIgnore checks if a file path matches any of the ignore patterns.
// It matches against the filename (base name) only.
// Patterns support glob syntax:
//   - * matches any sequence of non-separator characters
//   - ? matches any single non-separator character
//   - [abc] matches any character in the set
//   - [a-z] matches any character in the range
func (f *FileFilter) ShouldIgnore(path string) bool {
	filename := filepath.Base(path)

	for _, pattern := range f.patterns {
		// Try exact glob match on filename
		if matched, err := filepath.Match(pattern, filename); err == nil && matched {
			return true
		}

		// Also check if the pattern matches as a suffix (for extension patterns)
		// This handles cases like ".tmp" matching "file.tmp"
		if strings.HasPrefix(pattern, ".") && !strings.Contains(pattern, "*") {
			if strings.HasSuffix(strings.ToLower(filename), strings.ToLower(pattern)) {
				return true
			}
		}
	}
	return false
}

// GetPatterns returns the current ignore patterns.
func (f *FileFilter) GetPatterns() []string {
	result := make([]string, len(f.patterns))
	copy(result, f.patterns)
	return result
}

// AddPattern adds a new pattern to the filter.
func (f *FileFilter) AddPattern(pattern string) {
	f.patterns = append(f.patterns, pattern)
}

// IsTemporaryFile is a convenience function that checks if a file is a temporary file
// using the default ignore patterns.
func IsTemporaryFile(path string) bool {
	filter := NewFileFilter(nil)
	return filter.ShouldIgnore(path)
}
