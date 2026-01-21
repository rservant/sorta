// Package orchestrator provides the summary generator for run operations.
package orchestrator

import (
	"time"
)

// RunSummary contains statistics from a run operation.
// Requirements: 3.1, 3.2, 3.3, 3.4, 3.5 - Run summary statistics
type RunSummary struct {
	Moved     int            // Files moved to organized destinations
	ForReview int            // Files moved to for-review
	Skipped   int            // Files skipped (already organized, errors, etc.)
	Errors    int            // Errors encountered
	Duration  time.Duration  // Total processing time
	ByPrefix  map[string]int // Per-prefix counts (only populated in verbose mode)
}

// GenerateSummary creates a summary from a run result.
// When verbose is true, the ByPrefix map is populated with per-prefix breakdown.
// Requirements: 3.1, 3.2, 3.3, 3.4, 3.5 - Summary statistics calculation
func GenerateSummary(result *RunResult, duration time.Duration, verbose bool) *RunSummary {
	if result == nil {
		return &RunSummary{
			Duration: duration,
			ByPrefix: nil,
		}
	}

	summary := &RunSummary{
		Moved:     len(result.Moved),
		ForReview: len(result.ForReview),
		Skipped:   len(result.Skipped),
		Errors:    len(result.Errors),
		Duration:  duration,
	}

	// Only populate ByPrefix in verbose mode
	// Requirements: 3.6 - Per-prefix breakdown in verbose mode
	if verbose {
		summary.ByPrefix = make(map[string]int)
		for _, op := range result.Moved {
			if op.Prefix != "" {
				summary.ByPrefix[op.Prefix]++
			}
		}
	}

	return summary
}
