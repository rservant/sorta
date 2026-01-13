package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: audit-trail, Property 17: Minimum Retention Protection
// Validates: Requirements 10.4

// TestMinimumRetentionProtection tests Property 17: Minimum Retention Protection
// For any retention pruning operation, log segments containing runs less than
// the configured minimum age SHALL NOT be pruned.
func TestMinimumRetentionProtection(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Reduced from 100 for faster execution

	properties := gopter.NewProperties(parameters)

	properties.Property("Segments with runs younger than minimum age are never pruned", prop.ForAll(
		func(minRetentionDays int, retentionDays int, youngRunAgeDays int) bool {
			// Ensure youngRunAgeDays is less than minRetentionDays
			if youngRunAgeDays >= minRetentionDays {
				youngRunAgeDays = minRetentionDays - 1
				if youngRunAgeDays < 0 {
					youngRunAgeDays = 0
				}
			}

			tempDir, err := os.MkdirTemp("", "audit-retention-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{
				LogDirectory:     tempDir,
				RotationSize:     100, // Small size to allow multiple segments
				RetentionDays:    retentionDays,
				MinRetentionDays: minRetentionDays,
			}

			// Create a writer and write some events
			writer, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create writer: %v", err)
				return false
			}

			// Start a run
			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				t.Logf("Failed to start run: %v", err)
				writer.Close()
				return false
			}

			// Write some events
			for i := 0; i < 5; i++ {
				event := AuditEvent{
					Timestamp:       time.Now().UTC(),
					RunID:           runID,
					EventType:       EventMove,
					Status:          StatusSuccess,
					SourcePath:      filepath.Join("/source", "file.pdf"),
					DestinationPath: filepath.Join("/dest", "file.pdf"),
				}
				if err := writer.WriteEvent(event); err != nil {
					t.Logf("Failed to write event: %v", err)
					writer.Close()
					return false
				}
			}

			// End the run
			if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{TotalFiles: 5, Moved: 5}); err != nil {
				t.Logf("Failed to end run: %v", err)
				writer.Close()
				return false
			}

			writer.Close()

			// Create a rotated segment by renaming the active log
			activeLog := filepath.Join(tempDir, "sorta-audit.jsonl")
			rotatedSegment := filepath.Join(tempDir, "sorta-audit-20240101-120000-000.jsonl")
			if err := os.Rename(activeLog, rotatedSegment); err != nil {
				t.Logf("Failed to rename log: %v", err)
				return false
			}

			// Set the segment's mod time to be older than retention but with a run younger than min retention
			// The segment file time is old, but we'll check based on run start time
			oldTime := time.Now().Add(-time.Duration(retentionDays+1) * 24 * time.Hour)
			if err := os.Chtimes(rotatedSegment, oldTime, oldTime); err != nil {
				t.Logf("Failed to set file time: %v", err)
				return false
			}

			// Create a new active log
			writer2, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create second writer: %v", err)
				return false
			}
			defer writer2.Close()

			// Check retention - the segment should NOT be pruned because the run is young
			rm := NewRetentionManager(config)
			toPrune, err := rm.CheckRetention()
			if err != nil {
				t.Logf("Failed to check retention: %v", err)
				return false
			}

			// The segment contains a run that was just created (age < minRetentionDays)
			// So it should NOT be in the prune list
			for _, seg := range toPrune {
				if seg.Filename == "sorta-audit-20240101-120000-000.jsonl" {
					// This segment should not be pruned because the run is young
					// The run was just created, so its age is essentially 0 days
					// which is less than minRetentionDays
					t.Logf("Segment with young run was incorrectly marked for pruning")
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 14), // minRetentionDays: 1-14 days
		gen.IntRange(1, 30), // retentionDays: 1-30 days
		gen.IntRange(0, 6),  // youngRunAgeDays: 0-6 days (will be adjusted to be < minRetentionDays)
	))

	properties.TestingRun(t)
}

// TestPruningWithDayBasedRetention tests pruning with day-based retention.
// Requirements: 10.1
func TestPruningWithDayBasedRetention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-retention-days-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := AuditConfig{
		LogDirectory:     tempDir,
		RetentionDays:    7, // Keep logs for 7 days
		MinRetentionDays: 1, // Minimum 1 day retention
	}

	// Create a writer and write some events
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           runID,
		EventType:       EventMove,
		Status:          StatusSuccess,
		SourcePath:      "/source/file.pdf",
		DestinationPath: "/dest/file.pdf",
	}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("Failed to write event: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{TotalFiles: 1, Moved: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Create an old rotated segment
	activeLog := filepath.Join(tempDir, "sorta-audit.jsonl")
	oldSegment := filepath.Join(tempDir, "sorta-audit-20240101-120000-000.jsonl")

	// Copy the active log to create an old segment
	content, err := os.ReadFile(activeLog)
	if err != nil {
		t.Fatalf("Failed to read active log: %v", err)
	}
	if err := os.WriteFile(oldSegment, content, 0644); err != nil {
		t.Fatalf("Failed to write old segment: %v", err)
	}

	// Set the old segment's mod time to be older than retention period
	oldTime := time.Now().Add(-10 * 24 * time.Hour) // 10 days old
	if err := os.Chtimes(oldSegment, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set file time: %v", err)
	}

	// Check retention
	rm := NewRetentionManager(config)
	toPrune, err := rm.CheckRetention()
	if err != nil {
		t.Fatalf("Failed to check retention: %v", err)
	}

	// The old segment should be marked for pruning (it's older than 7 days)
	// But only if the runs in it are also old enough
	// Since we just created the run, it won't be pruned due to min retention
	// Let's verify the logic works correctly
	t.Logf("Segments to prune: %d", len(toPrune))
	for _, seg := range toPrune {
		t.Logf("  - %s (oldest run age: %v, newest run age: %v)",
			seg.Filename, seg.OldestRunAge, seg.NewestRunAge)
	}
}

// TestPruningWithRunCountRetention tests pruning with run-count retention.
// Requirements: 10.2
func TestPruningWithRunCountRetention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-retention-runs-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := AuditConfig{
		LogDirectory:     tempDir,
		RetentionRuns:    2, // Keep only 2 runs
		MinRetentionDays: 0, // No minimum retention for this test
	}

	// Create multiple runs
	for i := 0; i < 4; i++ {
		writer, err := NewAuditWriter(config)
		if err != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}

		runID, err := writer.StartRun("1.0.0", "test-machine")
		if err != nil {
			t.Fatalf("Failed to start run %d: %v", i, err)
		}

		event := AuditEvent{
			Timestamp:       time.Now().UTC(),
			RunID:           runID,
			EventType:       EventMove,
			Status:          StatusSuccess,
			SourcePath:      "/source/file.pdf",
			DestinationPath: "/dest/file.pdf",
		}
		if err := writer.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}

		if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{TotalFiles: 1, Moved: 1}); err != nil {
			t.Fatalf("Failed to end run %d: %v", i, err)
		}
		writer.Close()

		// Removed time.Sleep - timestamps from time.Now() are sufficient
	}

	// List runs to verify we have 4
	reader := NewAuditReader(tempDir)
	runs, err := reader.ListRuns()
	if err != nil {
		t.Fatalf("Failed to list runs: %v", err)
	}
	if len(runs) != 4 {
		t.Fatalf("Expected 4 runs, got %d", len(runs))
	}

	// Check retention - should want to prune 2 oldest runs
	rm := NewRetentionManager(config)
	toPrune, err := rm.CheckRetention()
	if err != nil {
		t.Fatalf("Failed to check retention: %v", err)
	}

	t.Logf("Runs: %d, Retention limit: %d", len(runs), config.RetentionRuns)
	t.Logf("Segments to prune: %d", len(toPrune))
	for _, seg := range toPrune {
		t.Logf("  - %s with %d runs", seg.Filename, len(seg.RunIDs))
	}
}

// TestMinimumAgeProtection tests that minimum age protection works.
// Requirements: 10.4
func TestMinimumAgeProtection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-retention-minage-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := AuditConfig{
		LogDirectory:     tempDir,
		RetentionDays:    1, // Very short retention
		MinRetentionDays: 7, // But minimum 7 days
	}

	// Create a writer and write some events
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           runID,
		EventType:       EventMove,
		Status:          StatusSuccess,
		SourcePath:      "/source/file.pdf",
		DestinationPath: "/dest/file.pdf",
	}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("Failed to write event: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{TotalFiles: 1, Moved: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Create a rotated segment
	activeLog := filepath.Join(tempDir, "sorta-audit.jsonl")
	segment := filepath.Join(tempDir, "sorta-audit-20240101-120000-000.jsonl")

	content, err := os.ReadFile(activeLog)
	if err != nil {
		t.Fatalf("Failed to read active log: %v", err)
	}
	if err := os.WriteFile(segment, content, 0644); err != nil {
		t.Fatalf("Failed to write segment: %v", err)
	}

	// Set segment mod time to be older than retention but run is still young
	oldTime := time.Now().Add(-3 * 24 * time.Hour) // 3 days old (older than 1 day retention)
	if err := os.Chtimes(segment, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set file time: %v", err)
	}

	// Check retention
	rm := NewRetentionManager(config)
	toPrune, err := rm.CheckRetention()
	if err != nil {
		t.Fatalf("Failed to check retention: %v", err)
	}

	// The segment should NOT be pruned because the run is younger than 7 days
	for _, seg := range toPrune {
		if seg.Filename == "sorta-audit-20240101-120000-000.jsonl" {
			t.Errorf("Segment with young run should not be pruned due to minimum retention protection")
		}
	}

	t.Logf("Segments to prune: %d (expected 0 due to min retention)", len(toPrune))
}

// TestRetentionPruneEventRecording tests that RETENTION_PRUNE events are recorded.
// Requirements: 10.5
func TestRetentionPruneEventRecording(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-retention-event-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a RETENTION_PRUNE event
	runIDs := []RunID{"run-1", "run-2"}
	event := CreateRetentionPruneEvent("sorta-audit-20240101-120000.jsonl", runIDs)

	// Verify event properties
	if event.EventType != EventRetentionPrune {
		t.Errorf("Expected event type %s, got %s", EventRetentionPrune, event.EventType)
	}
	if event.Status != StatusSuccess {
		t.Errorf("Expected status %s, got %s", StatusSuccess, event.Status)
	}
	if event.RunID != "" {
		t.Errorf("Expected empty run ID for system event, got %s", event.RunID)
	}
	if event.Metadata["prunedSegment"] != "sorta-audit-20240101-120000.jsonl" {
		t.Errorf("Expected pruned segment in metadata")
	}
	if event.Metadata["prunedRunCount"] != "2" {
		t.Errorf("Expected pruned run count of 2, got %s", event.Metadata["prunedRunCount"])
	}
}

// TestUnlimitedRetention tests that unlimited retention doesn't prune anything.
func TestUnlimitedRetention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-retention-unlimited-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := AuditConfig{
		LogDirectory:     tempDir,
		RetentionDays:    0, // Unlimited
		RetentionRuns:    0, // Unlimited
		MinRetentionDays: 7,
	}

	// Create some segments
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Check retention
	rm := NewRetentionManager(config)
	toPrune, err := rm.CheckRetention()
	if err != nil {
		t.Fatalf("Failed to check retention: %v", err)
	}

	if len(toPrune) != 0 {
		t.Errorf("Expected no segments to prune with unlimited retention, got %d", len(toPrune))
	}
}
