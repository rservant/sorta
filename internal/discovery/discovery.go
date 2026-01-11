// Package discovery handles auto-discovery of prefix rules from existing file structures.
package discovery

import (
	"os"
	"path/filepath"
	"strings"

	"sorta/internal/config"
)

// DiscoveredRule represents a prefix rule found during discovery.
type DiscoveredRule struct {
	Prefix          string
	TargetDirectory string
}

// DiscoveryResult contains the results of a discovery scan.
type DiscoveryResult struct {
	NewRules      []DiscoveredRule // Rules to be added
	SkippedRules  []DiscoveredRule // Rules skipped (duplicate prefix)
	ScannedDirs   int              // Number of directories scanned
	FilesAnalyzed int              // Number of files analyzed
}

// DiscoveryEventType represents the type of discovery event.
type DiscoveryEventType string

const (
	// EventTypeDir indicates a directory is being scanned.
	EventTypeDir DiscoveryEventType = "dir"
	// EventTypeFile indicates a file is being analyzed.
	EventTypeFile DiscoveryEventType = "file"
	// EventTypePattern indicates a pattern was found.
	EventTypePattern DiscoveryEventType = "pattern"
)

// DiscoveryEvent represents a discovery progress event.
type DiscoveryEvent struct {
	Type    DiscoveryEventType // "dir", "file", "pattern"
	Path    string             // Path being processed
	Pattern string             // Only for "pattern" type - the detected pattern
	Current int                // Current progress count
	Total   int                // Total items (if known, 0 otherwise)
}

// DiscoveryCallback is called during discovery operations.
type DiscoveryCallback func(event DiscoveryEvent)

// scanTargetCandidates finds immediate subdirectories of the scan directory.
// Returns only immediate child directories, not nested ones.
func scanTargetCandidates(scanDir string) ([]string, error) {
	entries, err := os.ReadDir(scanDir)
	if err != nil {
		return nil, err
	}

	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() {
			candidates = append(candidates, filepath.Join(scanDir, entry.Name()))
		}
	}

	return candidates, nil
}

// analyzeDirectory recursively scans all files within a directory
// and returns unique prefixes found using pattern detection.
func analyzeDirectory(dir string) ([]string, error) {
	return analyzeDirectoryWithCallback(dir, nil, nil)
}

// analyzeDirectoryWithCallback recursively scans all files within a directory
// and returns unique prefixes found using pattern detection.
// It calls the callback for each file analyzed and pattern found.
// It skips subdirectories whose names start with an ISO date (YYYY-MM-DD).
// Prefixes are extracted only from files, never from directory names.
func analyzeDirectoryWithCallback(dir string, callback DiscoveryCallback, fileCounter *int) ([]string, error) {
	prefixSet := make(map[string]bool)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't access
			return nil
		}

		if info.IsDir() {
			// Don't skip the root directory itself
			if path != dir {
				// Skip directories that start with an ISO date
				if IsISODateDirectory(info.Name()) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Call callback for file being analyzed
		if callback != nil && fileCounter != nil {
			*fileCounter++
			callback(DiscoveryEvent{
				Type:    EventTypeFile,
				Path:    path,
				Current: *fileCounter,
			})
		}

		// Extract prefix from filename
		prefix, matched := ExtractPrefixFromFilename(info.Name())
		if matched {
			// Check if this is a new prefix (case-insensitive)
			lowerPrefix := strings.ToLower(prefix)
			if !prefixSet[lowerPrefix] {
				prefixSet[lowerPrefix] = true
				// Store the original case version
				prefixSet[prefix] = true

				// Call callback for pattern found
				if callback != nil {
					callback(DiscoveryEvent{
						Type:    EventTypePattern,
						Path:    path,
						Pattern: prefix,
					})
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert set to slice
	var prefixes []string
	for prefix := range prefixSet {
		prefixes = append(prefixes, prefix)
	}

	return prefixes, nil
}

// Discover scans a directory and returns discovered prefix rules.
// It examines immediate subdirectories of scanDir, analyzes files within each,
// and generates prefix rules for any patterns found.
// Existing prefixes in the configuration are skipped.
func Discover(scanDir string, existingConfig *config.Configuration) (*DiscoveryResult, error) {
	return DiscoverWithCallback(scanDir, existingConfig, nil)
}

// DiscoverWithCallback scans a directory and returns discovered prefix rules with progress reporting.
// It examines immediate subdirectories of scanDir, analyzes files within each,
// and generates prefix rules for any patterns found.
// Existing prefixes in the configuration are skipped.
// The callback is called for each directory scanned, file analyzed, and pattern found.
func DiscoverWithCallback(scanDir string, existingConfig *config.Configuration, callback DiscoveryCallback) (*DiscoveryResult, error) {
	result := &DiscoveryResult{
		NewRules:     []DiscoveredRule{},
		SkippedRules: []DiscoveredRule{},
	}

	// Get immediate subdirectories as candidates
	candidates, err := scanTargetCandidates(scanDir)
	if err != nil {
		return nil, err
	}

	// Track which prefixes we've already seen during this discovery
	// to avoid duplicates within the same scan
	seenPrefixes := make(map[string]bool)

	// Track file count for progress reporting
	fileCounter := 0

	for i, candidateDir := range candidates {
		result.ScannedDirs++

		// Call callback for directory being scanned
		if callback != nil {
			callback(DiscoveryEvent{
				Type:    EventTypeDir,
				Path:    candidateDir,
				Current: i + 1,
				Total:   len(candidates),
			})
		}

		// Analyze the directory for prefixes with callback support
		prefixes, err := analyzeDirectoryWithCallback(candidateDir, callback, &fileCounter)
		if err != nil {
			// Log warning but continue with other directories
			continue
		}

		// Count files analyzed
		filepath.Walk(candidateDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				result.FilesAnalyzed++
			}
			return nil
		})

		// Process each discovered prefix
		for _, prefix := range prefixes {
			lowerPrefix := strings.ToLower(prefix)

			// Skip if we've already seen this prefix in this scan
			if seenPrefixes[lowerPrefix] {
				continue
			}
			seenPrefixes[lowerPrefix] = true

			rule := DiscoveredRule{
				Prefix:          prefix,
				TargetDirectory: candidateDir,
			}

			// Check if prefix already exists in config (case-insensitive)
			if existingConfig != nil && existingConfig.HasPrefix(prefix) {
				result.SkippedRules = append(result.SkippedRules, rule)
			} else {
				result.NewRules = append(result.NewRules, rule)
			}
		}
	}

	return result, nil
}
