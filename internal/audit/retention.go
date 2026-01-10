// Package audit provides audit trail functionality for Sorta file operations.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// RetentionManager handles log retention and pruning logic.
// Requirements: 10.1, 10.2, 10.4, 10.5
type RetentionManager struct {
	config AuditConfig
	reader *AuditReader
}

// NewRetentionManager creates a new RetentionManager with the given configuration.
func NewRetentionManager(config AuditConfig) *RetentionManager {
	return &RetentionManager{
		config: config,
		reader: NewAuditReader(config.LogDirectory),
	}
}

// SegmentRunInfo contains information about runs in a segment.
type SegmentRunInfo struct {
	Filename     string
	FilePath     string
	Size         int64
	ModTime      time.Time
	RunIDs       []RunID
	OldestRunAge time.Duration // Age of the oldest run in this segment
	NewestRunAge time.Duration // Age of the newest run in this segment
}

// PruneResult contains the result of a pruning operation.
type PruneResult struct {
	PrunedSegments  []string // Filenames of pruned segments
	PrunedRuns      []RunID  // Run IDs that were pruned
	SkippedSegments []string // Segments skipped due to minimum retention
	TotalBytesFreed int64
}

// CheckRetention checks if any segments need to be pruned based on retention limits.
// Returns segments that should be pruned.
// Requirements: 10.1, 10.2, 10.4
func (rm *RetentionManager) CheckRetention() ([]SegmentRunInfo, error) {
	// If both retention limits are unlimited, nothing to prune
	if rm.config.RetentionDays == 0 && rm.config.RetentionRuns == 0 {
		return nil, nil
	}

	// Get all segment information
	segmentInfos, err := rm.getSegmentRunInfos()
	if err != nil {
		return nil, fmt.Errorf("failed to get segment info: %w", err)
	}

	if len(segmentInfos) == 0 {
		return nil, nil
	}

	var toPrune []SegmentRunInfo
	now := time.Now()
	minRetentionDays := rm.config.MinRetentionDays
	if minRetentionDays == 0 {
		minRetentionDays = 7 // Default minimum retention
	}
	minRetentionDuration := time.Duration(minRetentionDays) * 24 * time.Hour

	// Check day-based retention
	if rm.config.RetentionDays > 0 {
		retentionDuration := time.Duration(rm.config.RetentionDays) * 24 * time.Hour
		for _, seg := range segmentInfos {
			// Skip the active log
			if seg.Filename == "sorta-audit.jsonl" {
				continue
			}

			// Check if segment is older than retention period
			segmentAge := now.Sub(seg.ModTime)
			if segmentAge > retentionDuration {
				// Check minimum retention protection
				if seg.NewestRunAge < minRetentionDuration {
					// Skip - contains runs younger than minimum age
					continue
				}
				toPrune = append(toPrune, seg)
			}
		}
	}

	// Check run-count based retention
	if rm.config.RetentionRuns > 0 {
		runs, err := rm.reader.ListRuns()
		if err != nil {
			return nil, fmt.Errorf("failed to list runs: %w", err)
		}

		if len(runs) > rm.config.RetentionRuns {
			// Sort runs by start time (oldest first)
			sort.Slice(runs, func(i, j int) bool {
				return runs[i].StartTime.Before(runs[j].StartTime)
			})

			// Find runs to prune (oldest ones exceeding the limit)
			runsToRemove := len(runs) - rm.config.RetentionRuns
			runsToPrune := make(map[RunID]bool)
			for i := 0; i < runsToRemove; i++ {
				run := runs[i]
				// Check minimum retention protection
				runAge := now.Sub(run.StartTime)
				if runAge >= minRetentionDuration {
					runsToPrune[run.RunID] = true
				}
			}

			// Find segments containing only runs to prune
			for _, seg := range segmentInfos {
				// Skip the active log
				if seg.Filename == "sorta-audit.jsonl" {
					continue
				}

				// Check if all runs in this segment should be pruned
				allRunsToPrune := true
				for _, runID := range seg.RunIDs {
					if !runsToPrune[runID] {
						allRunsToPrune = false
						break
					}
				}

				if allRunsToPrune && len(seg.RunIDs) > 0 {
					// Check minimum retention protection
					if seg.NewestRunAge >= minRetentionDuration {
						// Check if not already in toPrune list
						found := false
						for _, p := range toPrune {
							if p.Filename == seg.Filename {
								found = true
								break
							}
						}
						if !found {
							toPrune = append(toPrune, seg)
						}
					}
				}
			}
		}
	}

	return toPrune, nil
}

// Prune removes segments that exceed retention limits.
// It records RETENTION_PRUNE events for each pruned segment.
// Requirements: 10.2, 10.5
func (rm *RetentionManager) Prune(writer *AuditWriter) (*PruneResult, error) {
	toPrune, err := rm.CheckRetention()
	if err != nil {
		return nil, err
	}

	result := &PruneResult{
		PrunedSegments:  []string{},
		PrunedRuns:      []RunID{},
		SkippedSegments: []string{},
	}

	if len(toPrune) == 0 {
		return result, nil
	}

	for _, seg := range toPrune {
		// Record RETENTION_PRUNE event before deleting
		if writer != nil {
			event := CreateRetentionPruneEvent(seg.Filename, seg.RunIDs)
			if err := writer.WriteEvent(event); err != nil {
				return result, fmt.Errorf("failed to write RETENTION_PRUNE event: %w", err)
			}
		}

		// Delete the segment file
		if err := os.Remove(seg.FilePath); err != nil {
			return result, fmt.Errorf("failed to remove segment %s: %w", seg.Filename, err)
		}

		result.PrunedSegments = append(result.PrunedSegments, seg.Filename)
		result.PrunedRuns = append(result.PrunedRuns, seg.RunIDs...)
		result.TotalBytesFreed += seg.Size
	}

	// Update the rotation index to remove pruned segments
	if err := rm.updateIndexAfterPrune(result.PrunedSegments); err != nil {
		// Log warning but don't fail - the segments are already deleted
		fmt.Fprintf(os.Stderr, "warning: failed to update index after prune: %v\n", err)
	}

	return result, nil
}

// getSegmentRunInfos collects information about all segments and their runs.
func (rm *RetentionManager) getSegmentRunInfos() ([]SegmentRunInfo, error) {
	logFiles, err := GetAllLogFiles(rm.config.LogDirectory)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var infos []SegmentRunInfo

	for _, filePath := range logFiles {
		info, err := os.Stat(filePath)
		if err != nil {
			continue // Skip files we can't stat
		}

		segInfo := SegmentRunInfo{
			Filename: filepath.Base(filePath),
			FilePath: filePath,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
		}

		// Read events from this segment to find run IDs
		events, err := rm.reader.readEventsFromFile(filePath)
		if err != nil {
			// If we can't read the file, still include it with no run info
			infos = append(infos, segInfo)
			continue
		}

		// Collect unique run IDs and find oldest/newest run times
		runIDSet := make(map[RunID]bool)
		var oldestRunTime, newestRunTime time.Time

		for _, event := range events {
			if event.RunID == "" {
				continue // Skip system events
			}
			if !runIDSet[event.RunID] {
				runIDSet[event.RunID] = true
				segInfo.RunIDs = append(segInfo.RunIDs, event.RunID)
			}

			// Track run start times for age calculation
			if event.EventType == EventRunStart {
				if oldestRunTime.IsZero() || event.Timestamp.Before(oldestRunTime) {
					oldestRunTime = event.Timestamp
				}
				if newestRunTime.IsZero() || event.Timestamp.After(newestRunTime) {
					newestRunTime = event.Timestamp
				}
			}
		}

		// Calculate ages
		if !oldestRunTime.IsZero() {
			segInfo.OldestRunAge = now.Sub(oldestRunTime)
		}
		if !newestRunTime.IsZero() {
			segInfo.NewestRunAge = now.Sub(newestRunTime)
		}

		infos = append(infos, segInfo)
	}

	return infos, nil
}

// updateIndexAfterPrune removes pruned segments from the rotation index.
func (rm *RetentionManager) updateIndexAfterPrune(prunedSegments []string) error {
	indexPath := filepath.Join(rm.config.LogDirectory, "sorta-audit-index.json")

	index, err := LoadIndex(rm.config.LogDirectory)
	if err != nil {
		// Index doesn't exist or is corrupt - nothing to update
		return nil
	}

	// Create a set of pruned segment names for quick lookup
	prunedSet := make(map[string]bool)
	for _, seg := range prunedSegments {
		prunedSet[seg] = true
	}

	// Filter out pruned segments
	var remainingSegments []SegmentInfo
	for _, seg := range index.Segments {
		if !prunedSet[seg.Filename] {
			remainingSegments = append(remainingSegments, seg)
		}
	}

	index.Segments = remainingSegments
	index.LastUpdated = time.Now()

	// Write updated index
	data, err := jsonMarshalIndent(index)
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	return nil
}

// jsonMarshalIndent is a helper to marshal with indentation.
func jsonMarshalIndent(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// CreateRetentionPruneEvent creates a RETENTION_PRUNE event.
// Requirements: 10.5
func CreateRetentionPruneEvent(filename string, prunedRunIDs []RunID) AuditEvent {
	runIDStrings := make([]string, len(prunedRunIDs))
	for i, id := range prunedRunIDs {
		runIDStrings[i] = string(id)
	}

	return AuditEvent{
		Timestamp: time.Now().UTC(),
		RunID:     "", // System event, no run ID
		EventType: EventRetentionPrune,
		Status:    StatusSuccess,
		Metadata: map[string]string{
			"prunedSegment":  filename,
			"prunedRunCount": fmt.Sprintf("%d", len(prunedRunIDs)),
		},
	}
}

// CheckAndPruneOnStartup checks retention limits and prunes if needed.
// This should be called when the AuditWriter is initialized.
// Requirements: 10.1, 10.2
func (rm *RetentionManager) CheckAndPruneOnStartup(writer *AuditWriter) (*PruneResult, error) {
	return rm.Prune(writer)
}

// GetSegmentsToWarn returns segments that will be pruned soon (within warning threshold).
// This can be used to warn users before pruning occurs.
// Requirements: 10.3
func (rm *RetentionManager) GetSegmentsToWarn(warningDays int) ([]SegmentRunInfo, error) {
	if rm.config.RetentionDays == 0 {
		return nil, nil
	}

	segmentInfos, err := rm.getSegmentRunInfos()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	warningThreshold := time.Duration(rm.config.RetentionDays-warningDays) * 24 * time.Hour

	var toWarn []SegmentRunInfo
	for _, seg := range segmentInfos {
		if seg.Filename == "sorta-audit.jsonl" {
			continue
		}

		segmentAge := now.Sub(seg.ModTime)
		if segmentAge > warningThreshold {
			toWarn = append(toWarn, seg)
		}
	}

	return toWarn, nil
}
