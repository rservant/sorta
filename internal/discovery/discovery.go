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
	prefixSet := make(map[string]bool)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't access
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// Extract prefix from filename
		prefix, matched := ExtractPrefixFromFilename(info.Name())
		if matched {
			// Store prefix in lowercase for case-insensitive uniqueness
			prefixSet[strings.ToLower(prefix)] = true
			// But we want to preserve the original case, so store it again
			// Actually, let's store the original prefix
			prefixSet[prefix] = true
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

	for _, candidateDir := range candidates {
		result.ScannedDirs++

		// Analyze the directory for prefixes
		prefixes, err := analyzeDirectory(candidateDir)
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
