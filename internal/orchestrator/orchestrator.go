// Package orchestrator coordinates the file organization workflow for Sorta.
package orchestrator

import (
	"fmt"

	"sorta/internal/classifier"
	"sorta/internal/config"
	"sorta/internal/organizer"
	"sorta/internal/scanner"
)

// Result represents the outcome of organizing a single file.
type Result struct {
	SourcePath      string
	DestinationPath string
	Success         bool
	Error           error
}

// Summary represents the overall results of a Sorta run.
type Summary struct {
	TotalFiles   int
	SuccessCount int
	ErrorCount   int
	Results      []Result
	ScanErrors   []error
}

// Run executes the Sorta file organization workflow.
// It loads configuration, scans source directories, classifies files,
// and organizes them according to the rules.
func Run(configPath string) (*Summary, error) {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	summary := &Summary{
		Results:    make([]Result, 0),
		ScanErrors: make([]error, 0),
	}

	// Scan all source directories and collect files
	var allFiles []scanner.FileEntry
	for _, sourceDir := range cfg.SourceDirectories {
		files, err := scanner.Scan(sourceDir)
		if err != nil {
			// Log error and continue with remaining directories (Requirement 2.2)
			summary.ScanErrors = append(summary.ScanErrors, fmt.Errorf("failed to scan %s: %w", sourceDir, err))
			continue
		}
		allFiles = append(allFiles, files...)
	}

	summary.TotalFiles = len(allFiles)

	// Process each file
	for _, file := range allFiles {
		result := processFile(file, cfg)
		summary.Results = append(summary.Results, result)
		if result.Success {
			summary.SuccessCount++
		} else {
			summary.ErrorCount++
		}
	}

	return summary, nil
}

// processFile classifies and organizes a single file.
func processFile(file scanner.FileEntry, cfg *config.Configuration) Result {
	// Classify the file
	classification := classifier.Classify(file.Name, cfg.PrefixRules)

	// Organize (move) the file
	moveResult, err := organizer.Organize(file, classification, cfg)
	if err != nil {
		return Result{
			SourcePath: file.FullPath,
			Success:    false,
			Error:      err,
		}
	}

	return Result{
		SourcePath:      moveResult.SourcePath,
		DestinationPath: moveResult.DestinationPath,
		Success:         true,
	}
}

// HasErrors returns true if there were any errors during the run.
func (s *Summary) HasErrors() bool {
	return s.ErrorCount > 0 || len(s.ScanErrors) > 0
}

// PrintSummary returns a formatted summary string.
func (s *Summary) PrintSummary() string {
	return fmt.Sprintf("Processed %d files: %d successful, %d errors",
		s.TotalFiles, s.SuccessCount, s.ErrorCount)
}
