// Package audit provides audit trail functionality for Sorta file operations.
package audit

import (
	"fmt"
	"sort"
	"time"
)

// AuditStats contains aggregate metrics across all audit runs.
// Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6
type AuditStats struct {
	TotalOrganized int            // Total files organized across all runs
	TotalForReview int            // Total files sent to for-review
	TotalRuns      int            // Number of organize runs
	TotalUndos     int            // Number of undo operations
	ByPrefix       map[string]int // Files per prefix (top N)
	FirstRun       time.Time      // Earliest run timestamp
	LastRun        time.Time      // Most recent run timestamp
}

// StatsOptions configures stats aggregation.
// Requirements: 4.7
type StatsOptions struct {
	Since *time.Time // Filter to runs after this time
	TopN  int        // Number of top prefixes to show (0 = all)
}

// AggregateStats computes metrics across all audit logs.
// It reads all audit log files from the given directory and aggregates
// statistics based on the provided options.
// Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7
func AggregateStats(logDir string, opts StatsOptions) (*AuditStats, error) {
	reader := NewAuditReader(logDir)

	// Get all runs
	runs, err := reader.ListRuns()
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}

	stats := &AuditStats{
		ByPrefix: make(map[string]int),
	}

	// Track all prefix counts before limiting to top N
	allPrefixCounts := make(map[string]int)

	for _, run := range runs {
		// Apply time filter if specified
		if opts.Since != nil && run.StartTime.Before(*opts.Since) {
			continue
		}

		// Track run counts by type
		if run.RunType == RunTypeUndo {
			stats.TotalUndos++
		} else {
			stats.TotalRuns++
		}

		// Update date range
		if stats.FirstRun.IsZero() || run.StartTime.Before(stats.FirstRun) {
			stats.FirstRun = run.StartTime
		}
		if stats.LastRun.IsZero() || run.StartTime.After(stats.LastRun) {
			stats.LastRun = run.StartTime
		}

		// Aggregate totals from run summary
		stats.TotalOrganized += run.Summary.Moved
		stats.TotalForReview += run.Summary.RoutedReview

		// Get detailed events for this run to extract prefix information
		events, err := reader.GetRun(run.RunID)
		if err != nil {
			// Log warning but continue with other runs
			continue
		}

		// Extract prefix counts from MOVE events
		for _, event := range events {
			if event.EventType == EventMove && event.Status == StatusSuccess {
				prefix := extractPrefix(event.DestinationPath)
				if prefix != "" {
					allPrefixCounts[prefix]++
				}
			}
		}
	}

	// Apply top N filtering to prefix counts
	stats.ByPrefix = filterTopN(allPrefixCounts, opts.TopN)

	return stats, nil
}

// extractPrefix extracts the prefix (first directory component after the base)
// from a destination path. For example:
// "/organized/invoices/2024/file.pdf" -> "invoices"
// "/organized/receipts/file.pdf" -> "receipts"
func extractPrefix(destPath string) string {
	if destPath == "" {
		return ""
	}

	// Split path into components
	// We want to find the first meaningful directory after any base path
	// Typically the structure is: /base/prefix/... or prefix/...

	// Remove leading slash if present
	path := destPath
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	// Split by path separator
	var components []string
	current := ""
	for _, c := range path {
		if c == '/' || c == '\\' {
			if current != "" {
				components = append(components, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		components = append(components, current)
	}

	// Return the first directory component (prefix)
	// Skip common base directories like "organized", "output", etc.
	for _, comp := range components {
		// Skip common base directory names
		if comp == "organized" || comp == "output" || comp == "." || comp == ".." {
			continue
		}
		// Return the first meaningful component
		return comp
	}

	return ""
}

// filterTopN returns the top N entries from a map by value.
// If n <= 0, returns all entries.
func filterTopN(counts map[string]int, n int) map[string]int {
	if n <= 0 || len(counts) <= n {
		// Return a copy of all counts
		result := make(map[string]int, len(counts))
		for k, v := range counts {
			result[k] = v
		}
		return result
	}

	// Sort by count descending
	type kv struct {
		key   string
		value int
	}
	var sorted []kv
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		// Sort by value descending, then by key ascending for stability
		if sorted[i].value != sorted[j].value {
			return sorted[i].value > sorted[j].value
		}
		return sorted[i].key < sorted[j].key
	})

	// Take top N
	result := make(map[string]int, n)
	for i := 0; i < n && i < len(sorted); i++ {
		result[sorted[i].key] = sorted[i].value
	}

	return result
}
