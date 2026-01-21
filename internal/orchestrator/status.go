// Package orchestrator coordinates the file organization workflow for Sorta.
package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"

	"sorta/internal/classifier"
	"sorta/internal/config"
	"sorta/internal/organizer"
	"sorta/internal/scanner"
)

// StatusResult contains the status analysis results.
// Requirements: 2.1, 2.2, 2.3, 2.4 - Status command output structure
type StatusResult struct {
	ByInbound  map[string]*InboundStatus // Status per inbound directory
	GrandTotal int                       // Total count of all pending files
}

// InboundStatus contains status for one inbound directory.
// Requirements: 2.2, 2.3 - Group files by destination with per-directory counts
type InboundStatus struct {
	Directory     string              // The inbound directory path
	ByDestination map[string][]string // destination -> list of file paths
	Total         int                 // Total files in this inbound directory
}

// Status analyzes pending files without modifying anything.
// It scans all configured inbound directories, classifies each file to determine
// its destination, groups files by their destination (matched prefix or for-review),
// and calculates per-directory counts and grand total.
// Requirements: 2.1, 2.2, 2.3, 2.4, 2.6 - Status command implementation
func (o *Orchestrator) Status() (*StatusResult, error) {
	result := &StatusResult{
		ByInbound:  make(map[string]*InboundStatus),
		GrandTotal: 0,
	}

	// Determine scan options from config
	scanOpts := scanner.DefaultScanOptions()
	scanOpts.MaxDepth = o.config.GetScanDepth()
	scanOpts.SymlinkPolicy = o.config.GetSymlinkPolicy()

	// Scan all configured inbound directories
	// Requirements: 2.1 - Scan all configured inbound directories
	for _, inboundDir := range o.config.InboundDirectories {
		inboundStatus := &InboundStatus{
			Directory:     inboundDir,
			ByDestination: make(map[string][]string),
			Total:         0,
		}

		// Check if directory exists before scanning
		if _, err := os.Stat(inboundDir); os.IsNotExist(err) {
			// Skip non-existent directories but still include them in results
			// with empty status (consistent with error handling approach)
			result.ByInbound[inboundDir] = inboundStatus
			continue
		}

		// Scan the directory for files
		files, err := scanner.ScanWithOptions(inboundDir, scanOpts)
		if err != nil {
			// Skip directories with scan errors but still include them in results
			result.ByInbound[inboundDir] = inboundStatus
			continue
		}

		// Classify each file and group by destination
		// Requirements: 2.2 - Group files by destination (matched prefix or for-review)
		for _, file := range files {
			destination := classifyFileDestination(file, o.config)
			inboundStatus.ByDestination[destination] = append(
				inboundStatus.ByDestination[destination],
				file.FullPath,
			)
			inboundStatus.Total++
		}

		result.ByInbound[inboundDir] = inboundStatus
		// Requirements: 2.4 - Grand total equals sum of all per-directory counts
		result.GrandTotal += inboundStatus.Total
	}

	return result, nil
}

// classifyFileDestination determines the destination for a file without moving it.
// Returns the destination directory path (either organized location or for-review).
// Requirements: 2.2 - Classify files to determine destination
func classifyFileDestination(file scanner.FileEntry, cfg *config.Configuration) string {
	// Classify the file using existing classifier
	classification := classifier.Classify(file.Name, cfg.PrefixRules)

	if classification.IsUnclassified() {
		// File would go to for-review directory
		return organizer.GetForReviewPath(filepath.Dir(file.FullPath))
	}

	// File is classified - would be moved to organized location
	prefix := extractPrefixFromNormalisedFilename(classification.NormalisedFilename)
	subfolder := fmt.Sprintf("%d %s", classification.Year, prefix)
	return filepath.Join(classification.OutboundDirectory, subfolder)
}

// Orchestrator wraps configuration for status operations.
// This provides a cleaner API for the Status method.
type Orchestrator struct {
	config *config.Configuration
}

// NewOrchestrator creates a new Orchestrator with the given configuration.
func NewOrchestrator(cfg *config.Configuration) *Orchestrator {
	return &Orchestrator{
		config: cfg,
	}
}

// NewOrchestratorFromPath creates a new Orchestrator by loading configuration from a file.
func NewOrchestratorFromPath(configPath string) (*Orchestrator, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return &Orchestrator{config: cfg}, nil
}

// StatusFromPath is a convenience function that creates an orchestrator and runs Status.
// This provides a simpler API for callers who just want status results.
func StatusFromPath(configPath string) (*StatusResult, error) {
	o, err := NewOrchestratorFromPath(configPath)
	if err != nil {
		return nil, err
	}
	return o.Status()
}
