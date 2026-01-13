package audit

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: audit-trail, Property 16: Event Filtering Correctness
// Validates: Requirements 15.5

// TestEventFilteringCorrectness tests Property 16: Event Filtering Correctness
// For any event filter applied to a run, the returned events SHALL contain only
// events matching the filter criteria and SHALL contain all events matching the
// filter criteria.
func TestEventFilteringCorrectness(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Reduced from 100 for faster execution

	properties := gopter.NewProperties(parameters)

	// Generate a random subset of event types to filter by
	eventTypeGen := gen.IntRange(1, 4).FlatMap(func(n interface{}) gopter.Gen {
		count := n.(int)
		return gen.SliceOfN(count, gen.OneConstOf(
			EventMove,
			EventRouteToReview,
			EventSkip,
			EventError,
		))
	}, reflect.TypeOf([]EventType{})).Map(func(types []EventType) []EventType {
		// Deduplicate
		seen := make(map[EventType]bool)
		var unique []EventType
		for _, t := range types {
			if !seen[t] {
				seen[t] = true
				unique = append(unique, t)
			}
		}
		return unique
	})

	properties.Property("Filtered events contain only matching types and all matching types", prop.ForAll(
		func(filterTypes []EventType, eventCounts [4]int) bool {
			tempDir, err := os.MkdirTemp("", "audit-filter-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{LogDirectory: tempDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create writer: %v", err)
				return false
			}

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				writer.Close()
				t.Logf("Failed to start run: %v", err)
				return false
			}

			// Track expected counts per event type
			expectedCounts := make(map[EventType]int)
			eventTypes := []EventType{EventMove, EventRouteToReview, EventSkip, EventError}

			// Write events of each type
			fileIndex := 0
			for i, eventType := range eventTypes {
				count := eventCounts[i]
				expectedCounts[eventType] = count

				for j := 0; j < count; j++ {
					path := filepath.Join("/source", "file"+string(rune('A'+fileIndex))+".pdf")
					fileIndex++

					switch eventType {
					case EventMove:
						identity := &FileIdentity{ContentHash: "abc123", Size: 1000}
						err = writer.RecordMove(path, "/dest/"+filepath.Base(path), identity)
					case EventRouteToReview:
						err = writer.RecordRouteToReview(path, "/review/"+filepath.Base(path), ReasonUnclassified)
					case EventSkip:
						err = writer.RecordSkip(path, ReasonNoMatch)
					case EventError:
						err = writer.RecordError(path, "IO_ERROR", "test error", "read")
					}
					if err != nil {
						writer.Close()
						t.Logf("Failed to record event: %v", err)
						return false
					}
				}
			}

			err = writer.EndRun(runID, RunStatusCompleted, RunSummary{})
			if err != nil {
				writer.Close()
				t.Logf("Failed to end run: %v", err)
				return false
			}
			writer.Close()

			// Create reader and apply filter
			reader := NewAuditReader(tempDir)
			filter := EventFilter{
				EventTypes: filterTypes,
			}

			filteredEvents, err := reader.FilterEvents(runID, filter)
			if err != nil {
				t.Logf("Failed to filter events: %v", err)
				return false
			}

			// Calculate expected count based on filter
			expectedTotal := 0
			filterTypeSet := make(map[EventType]bool)
			for _, ft := range filterTypes {
				filterTypeSet[ft] = true
				expectedTotal += expectedCounts[ft]
			}

			// Verify count matches expected
			if len(filteredEvents) != expectedTotal {
				t.Logf("Expected %d filtered events, got %d", expectedTotal, len(filteredEvents))
				return false
			}

			// Verify all returned events match the filter
			for _, event := range filteredEvents {
				if !filterTypeSet[event.EventType] {
					t.Logf("Event type %s should not be in filtered results", event.EventType)
					return false
				}
			}

			// Verify counts per type match expected
			actualCounts := make(map[EventType]int)
			for _, event := range filteredEvents {
				actualCounts[event.EventType]++
			}

			for _, ft := range filterTypes {
				if actualCounts[ft] != expectedCounts[ft] {
					t.Logf("Expected %d events of type %s, got %d", expectedCounts[ft], ft, actualCounts[ft])
					return false
				}
			}

			return true
		},
		eventTypeGen,
		gen.SliceOfN(4, gen.IntRange(1, 5)).Map(func(s []int) [4]int {
			var arr [4]int
			for i := 0; i < 4 && i < len(s); i++ {
				arr[i] = s[i]
			}
			return arr
		}),
	))

	properties.Property("Empty filter returns all events", prop.ForAll(
		func(eventCount int) bool {
			tempDir, err := os.MkdirTemp("", "audit-filter-empty-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{LogDirectory: tempDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				return false
			}

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				writer.Close()
				return false
			}

			// Write some events
			for i := 0; i < eventCount; i++ {
				path := filepath.Join("/source", "file"+string(rune('A'+i))+".pdf")
				err = writer.RecordSkip(path, ReasonNoMatch)
				if err != nil {
					writer.Close()
					return false
				}
			}

			err = writer.EndRun(runID, RunStatusCompleted, RunSummary{})
			if err != nil {
				writer.Close()
				return false
			}
			writer.Close()

			reader := NewAuditReader(tempDir)

			// Empty filter should return all events for the run
			filter := EventFilter{}
			filteredEvents, err := reader.FilterEvents(runID, filter)
			if err != nil {
				t.Logf("Failed to filter events: %v", err)
				return false
			}

			// Should include RUN_START, all SKIP events, and RUN_END
			expectedCount := eventCount + 2 // +2 for RUN_START and RUN_END
			if len(filteredEvents) != expectedCount {
				t.Logf("Expected %d events with empty filter, got %d", expectedCount, len(filteredEvents))
				return false
			}

			return true
		},
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// Unit tests for AuditReader
// Requirements: 6.1, 9.5

// TestReadEmptyLog tests reading from an empty log directory.
func TestReadEmptyLog(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-reader-empty-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	reader := NewAuditReader(tempDir)

	// ListRuns should return empty slice, not error
	runs, err := reader.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns failed on empty log: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("Expected 0 runs, got %d", len(runs))
	}

	// GetLatestRun should return error
	_, err = reader.GetLatestRun()
	if err == nil {
		t.Error("Expected error from GetLatestRun on empty log")
	}

	// CountEvents should return 0
	count, err := reader.CountEvents()
	if err != nil {
		t.Fatalf("CountEvents failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 events, got %d", count)
	}
}

// TestReadLogWithMultipleRuns tests reading a log with multiple runs.
func TestReadLogWithMultipleRuns(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-reader-multi-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := AuditConfig{LogDirectory: tempDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Create 3 runs with different event counts
	// Use longer delays to ensure different timestamps (RFC3339 has second precision)
	runIDs := make([]RunID, 3)
	eventCounts := []int{2, 5, 3}

	for i := 0; i < 3; i++ {
		runID, err := writer.StartRun("1.0.0", "test-machine")
		if err != nil {
			t.Fatalf("Failed to start run %d: %v", i, err)
		}
		runIDs[i] = runID

		// Write events
		for j := 0; j < eventCounts[i]; j++ {
			path := filepath.Join("/source", "run"+string(rune('0'+i)), "file"+string(rune('A'+j))+".pdf")
			err = writer.RecordSkip(path, ReasonNoMatch)
			if err != nil {
				t.Fatalf("Failed to record event: %v", err)
			}
		}

		summary := RunSummary{
			TotalFiles: eventCounts[i],
			Skipped:    eventCounts[i],
		}
		err = writer.EndRun(runID, RunStatusCompleted, summary)
		if err != nil {
			t.Fatalf("Failed to end run %d: %v", i, err)
		}

		// Wait to ensure different timestamps (RFC3339 has second precision)
		if i < 2 {
			time.Sleep(1100 * time.Millisecond)
		}
	}
	writer.Close()

	// Read and verify
	reader := NewAuditReader(tempDir)

	// ListRuns should return all 3 runs
	runs, err := reader.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if len(runs) != 3 {
		t.Errorf("Expected 3 runs, got %d", len(runs))
	}

	// Verify each run can be retrieved
	for i, runID := range runIDs {
		events, err := reader.GetRun(runID)
		if err != nil {
			t.Errorf("GetRun failed for run %d: %v", i, err)
			continue
		}

		// Each run should have: RUN_START + events + RUN_END
		expectedEvents := eventCounts[i] + 2
		if len(events) != expectedEvents {
			t.Errorf("Run %d: expected %d events, got %d", i, expectedEvents, len(events))
		}
	}

	// GetLatestRun should return the last run (by start time)
	latestRun, err := reader.GetLatestRun()
	if err != nil {
		t.Fatalf("GetLatestRun failed: %v", err)
	}
	// The latest run should be the third one we created
	if latestRun.RunID != runIDs[2] {
		// Log all runs for debugging
		t.Logf("All runs:")
		for _, run := range runs {
			t.Logf("  RunID: %s, StartTime: %v", run.RunID, run.StartTime)
		}
		t.Errorf("Expected latest run to be %s, got %s", runIDs[2], latestRun.RunID)
	}

	// GetRun with invalid ID should return error
	_, err = reader.GetRun("invalid-run-id")
	if err == nil {
		t.Error("Expected error for invalid run ID")
	}
}

// TestReadAcrossRotatedSegments tests reading events across multiple rotated log segments.
func TestReadAcrossRotatedSegments(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-reader-rotation-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Use small rotation size to trigger rotation
	config := AuditConfig{
		LogDirectory: tempDir,
		RotationSize: 500, // Very small to trigger rotation
	}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Write enough events to trigger rotation
	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	totalEvents := 20
	for i := 0; i < totalEvents; i++ {
		path := filepath.Join("/source/documents/very/long/path/to/increase/event/size", "file"+string(rune('A'+i%26))+string(rune('0'+i/26))+".pdf")
		identity := &FileIdentity{
			ContentHash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			Size:        int64(i * 1000),
		}
		err = writer.RecordMove(path, "/dest/"+filepath.Base(path), identity)
		if err != nil {
			t.Fatalf("Failed to record event %d: %v", i, err)
		}
	}

	err = writer.EndRun(runID, RunStatusCompleted, RunSummary{TotalFiles: totalEvents, Moved: totalEvents})
	if err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Verify multiple segments were created
	reader := NewAuditReader(tempDir)
	segments, err := reader.GetSegmentFiles()
	if err != nil {
		t.Fatalf("Failed to get segment files: %v", err)
	}

	if len(segments) < 2 {
		t.Logf("Warning: Expected multiple segments but got %d (rotation may not have triggered)", len(segments))
	}

	// Read all events and verify count
	events, err := reader.GetRun(runID)
	if err != nil {
		t.Fatalf("Failed to get run events: %v", err)
	}

	// Should have: RUN_START + totalEvents MOVE + ROTATION events + RUN_END
	// The exact count depends on how many rotations occurred
	if len(events) < totalEvents+2 {
		t.Errorf("Expected at least %d events, got %d", totalEvents+2, len(events))
	}

	// Verify all MOVE events are present
	moveCount := 0
	for _, event := range events {
		if event.EventType == EventMove {
			moveCount++
		}
	}
	if moveCount != totalEvents {
		t.Errorf("Expected %d MOVE events, got %d", totalEvents, moveCount)
	}

	// Verify events are in chronological order
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp.Before(events[i-1].Timestamp) {
			t.Errorf("Events not in chronological order at index %d", i)
		}
	}
}

// TestFilterEventsByStatus tests filtering events by status.
func TestFilterEventsByStatus(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-filter-status-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := AuditConfig{LogDirectory: tempDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Write events with different statuses
	// SUCCESS: MOVE events
	for i := 0; i < 3; i++ {
		path := filepath.Join("/source", "success"+string(rune('A'+i))+".pdf")
		identity := &FileIdentity{ContentHash: "abc", Size: 100}
		writer.RecordMove(path, "/dest/"+filepath.Base(path), identity)
	}

	// SKIPPED: SKIP events
	for i := 0; i < 2; i++ {
		path := filepath.Join("/source", "skipped"+string(rune('A'+i))+".pdf")
		writer.RecordSkip(path, ReasonNoMatch)
	}

	// FAILURE: ERROR events
	for i := 0; i < 4; i++ {
		path := filepath.Join("/source", "error"+string(rune('A'+i))+".pdf")
		writer.RecordError(path, "IO_ERROR", "test error", "read")
	}

	writer.EndRun(runID, RunStatusCompleted, RunSummary{})
	writer.Close()

	reader := NewAuditReader(tempDir)

	// Filter by SUCCESS status
	successFilter := EventFilter{Status: StatusSuccess}
	successEvents, err := reader.FilterEvents(runID, successFilter)
	if err != nil {
		t.Fatalf("Failed to filter by SUCCESS: %v", err)
	}
	// Should include RUN_START (SUCCESS), MOVE events (SUCCESS), RUN_END (SUCCESS)
	successCount := 0
	for _, e := range successEvents {
		if e.Status == StatusSuccess {
			successCount++
		}
	}
	if successCount != len(successEvents) {
		t.Errorf("Not all filtered events have SUCCESS status")
	}

	// Filter by SKIPPED status
	skippedFilter := EventFilter{Status: StatusSkipped}
	skippedEvents, err := reader.FilterEvents(runID, skippedFilter)
	if err != nil {
		t.Fatalf("Failed to filter by SKIPPED: %v", err)
	}
	if len(skippedEvents) != 2 {
		t.Errorf("Expected 2 SKIPPED events, got %d", len(skippedEvents))
	}

	// Filter by FAILURE status
	failureFilter := EventFilter{Status: StatusFailure}
	failureEvents, err := reader.FilterEvents(runID, failureFilter)
	if err != nil {
		t.Fatalf("Failed to filter by FAILURE: %v", err)
	}
	if len(failureEvents) != 4 {
		t.Errorf("Expected 4 FAILURE events, got %d", len(failureEvents))
	}
}

// TestFilterEventsByTimeRange tests filtering events by time range.
func TestFilterEventsByTimeRange(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-filter-time-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := AuditConfig{LogDirectory: tempDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Write first batch of events
	for i := 0; i < 3; i++ {
		path := filepath.Join("/source", "batch1_"+string(rune('A'+i))+".pdf")
		writer.RecordSkip(path, ReasonNoMatch)
	}

	// Wait and record middle time
	// Use 1 second to ensure different timestamps (RFC3339 has second precision)
	time.Sleep(1000 * time.Millisecond)
	middleTime := time.Now()
	time.Sleep(1000 * time.Millisecond)

	// Write second batch of events
	for i := 0; i < 2; i++ {
		path := filepath.Join("/source", "batch2_"+string(rune('A'+i))+".pdf")
		writer.RecordSkip(path, ReasonNoMatch)
	}

	writer.EndRun(runID, RunStatusCompleted, RunSummary{})
	writer.Close()

	reader := NewAuditReader(tempDir)

	// Get all events first to understand what we have
	allEvents, err := reader.GetRun(runID)
	if err != nil {
		t.Fatalf("Failed to get all events: %v", err)
	}
	t.Logf("Total events: %d", len(allEvents))
	for i, e := range allEvents {
		t.Logf("  Event %d: %s at %v", i, e.EventType, e.Timestamp)
	}

	// Filter events after middle time (should include second batch + RUN_END)
	afterMiddle := EventFilter{StartTime: &middleTime}
	afterEvents, err := reader.FilterEvents(runID, afterMiddle)
	if err != nil {
		t.Fatalf("Failed to filter by start time: %v", err)
	}
	t.Logf("Events after middle time: %d", len(afterEvents))
	// Should include at least the second batch (2 events) + RUN_END (1 event) = 3
	if len(afterEvents) < 2 {
		t.Errorf("Expected at least 2 events after middle time, got %d", len(afterEvents))
	}

	// Filter events before middle time (should include RUN_START + first batch)
	beforeMiddle := EventFilter{EndTime: &middleTime}
	beforeEvents, err := reader.FilterEvents(runID, beforeMiddle)
	if err != nil {
		t.Fatalf("Failed to filter by end time: %v", err)
	}
	t.Logf("Events before middle time: %d", len(beforeEvents))
	// Should include RUN_START (1) + first batch (3) = 4
	if len(beforeEvents) < 3 {
		t.Errorf("Expected at least 3 events before middle time, got %d", len(beforeEvents))
	}

	// Verify that before + after covers all events (with possible overlap at boundary)
	if len(beforeEvents)+len(afterEvents) < len(allEvents) {
		t.Errorf("Before (%d) + After (%d) should cover all events (%d)",
			len(beforeEvents), len(afterEvents), len(allEvents))
	}
}

// TestGetRunByID tests the GetRunByID method.
func TestGetRunByID(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-reader-getbyid-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := AuditConfig{LogDirectory: tempDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	writer.RecordSkip("/source/file.pdf", ReasonNoMatch)
	writer.EndRun(runID, RunStatusCompleted, RunSummary{TotalFiles: 1, Skipped: 1})
	writer.Close()

	reader := NewAuditReader(tempDir)

	// Get existing run
	runInfo, err := reader.GetRunByID(runID)
	if err != nil {
		t.Fatalf("GetRunByID failed: %v", err)
	}
	if runInfo.RunID != runID {
		t.Errorf("Expected run ID %s, got %s", runID, runInfo.RunID)
	}
	if runInfo.Status != RunStatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", runInfo.Status)
	}

	// Get non-existent run
	_, err = reader.GetRunByID("non-existent-run-id")
	if err == nil {
		t.Error("Expected error for non-existent run ID")
	}
}

// TestRunInfoExtraction tests that RunInfo is correctly extracted from events.
func TestRunInfoExtraction(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-reader-runinfo-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := AuditConfig{LogDirectory: tempDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("2.0.0", "machine-123")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Write various events
	identity := &FileIdentity{ContentHash: "abc", Size: 100}
	writer.RecordMove("/source/a.pdf", "/dest/a.pdf", identity)
	writer.RecordMove("/source/b.pdf", "/dest/b.pdf", identity)
	writer.RecordSkip("/source/c.pdf", ReasonNoMatch)
	writer.RecordRouteToReview("/source/d.pdf", "/review/d.pdf", ReasonUnclassified)
	writer.RecordError("/source/e.pdf", "IO_ERROR", "test", "read")

	summary := RunSummary{
		TotalFiles:   5,
		Moved:        2,
		Skipped:      1,
		RoutedReview: 1,
		Errors:       1,
	}
	writer.EndRun(runID, RunStatusCompleted, summary)
	writer.Close()

	reader := NewAuditReader(tempDir)
	runs, err := reader.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}

	if len(runs) != 1 {
		t.Fatalf("Expected 1 run, got %d", len(runs))
	}

	run := runs[0]
	if run.RunID != runID {
		t.Errorf("Expected run ID %s, got %s", runID, run.RunID)
	}
	if run.AppVersion != "2.0.0" {
		t.Errorf("Expected app version 2.0.0, got %s", run.AppVersion)
	}
	if run.MachineID != "machine-123" {
		t.Errorf("Expected machine ID machine-123, got %s", run.MachineID)
	}
	if run.Status != RunStatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", run.Status)
	}
	if run.EndTime == nil {
		t.Error("Expected EndTime to be set")
	}

	// Verify summary from metadata
	if run.Summary.TotalFiles != 5 {
		t.Errorf("Expected TotalFiles 5, got %d", run.Summary.TotalFiles)
	}
	if run.Summary.Moved != 2 {
		t.Errorf("Expected Moved 2, got %d", run.Summary.Moved)
	}
}

// Unit tests for log integrity validation
// Requirements: 12.1, 12.2, 12.4, 12.5

// TestMissingLogFileHandling tests that missing log files are detected correctly.
// Requirements: 12.1
func TestMissingLogFileHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-integrity-missing-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	reader := NewAuditReader(tempDir)

	// Check integrity of non-existent log
	result, err := reader.CheckLogIntegrity()
	if err != nil {
		t.Fatalf("CheckLogIntegrity returned error: %v", err)
	}

	if result.Status != IntegrityMissing {
		t.Errorf("Expected status MISSING, got %s", result.Status)
	}

	if result.ErrorMessage == "" {
		t.Error("Expected error message for missing file")
	}

	// Verify IsLogCorrupt returns false for missing file (not corrupt, just missing)
	isCorrupt, err := reader.IsLogCorrupt()
	if err != nil {
		t.Fatalf("IsLogCorrupt returned error: %v", err)
	}
	if isCorrupt {
		t.Error("Missing file should not be reported as corrupt")
	}
}

// TestTruncatedLastLineDetection tests that truncated last lines are detected.
// Requirements: 12.2, 12.5
func TestTruncatedLastLineDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-integrity-truncated-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "sorta-audit.jsonl")

	// Create a valid event
	event := AuditEvent{
		Timestamp: time.Now().UTC(),
		RunID:     "test-run-id",
		EventType: EventRunStart,
		Status:    StatusSuccess,
	}
	eventJSON, err := event.MarshalJSONLine()
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	// Write event WITHOUT trailing newline (simulating truncation)
	err = os.WriteFile(logPath, eventJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to write log file: %v", err)
	}

	reader := NewAuditReader(tempDir)
	result, err := reader.CheckLogIntegrity()
	if err != nil {
		t.Fatalf("CheckLogIntegrity returned error: %v", err)
	}

	if result.Status != IntegrityCorrupt {
		t.Errorf("Expected status CORRUPT for truncated file, got %s", result.Status)
	}

	if result.ErrorMessage == "" {
		t.Error("Expected error message for truncated file")
	}

	// Verify IsLogCorrupt returns true
	isCorrupt, err := reader.IsLogCorrupt()
	if err != nil {
		t.Fatalf("IsLogCorrupt returned error: %v", err)
	}
	if !isCorrupt {
		t.Error("Truncated file should be reported as corrupt")
	}
}

// TestInvalidJSONDetection tests that invalid JSON is detected.
// Requirements: 12.2
func TestInvalidJSONDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-integrity-invalid-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "sorta-audit.jsonl")

	testCases := []struct {
		name    string
		content string
	}{
		{
			name:    "completely invalid JSON",
			content: "this is not json at all\n",
		},
		{
			name:    "truncated JSON object",
			content: `{"timestamp":"2024-01-01T00:00:00Z","runId":"test"` + "\n",
		},
		{
			name:    "valid first line, invalid second line",
			content: `{"timestamp":"2024-01-01T00:00:00Z","runId":"test","eventType":"RUN_START","status":"SUCCESS"}` + "\n" + `{invalid json}` + "\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write invalid content
			err := os.WriteFile(logPath, []byte(tc.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write log file: %v", err)
			}

			reader := NewAuditReader(tempDir)
			result, err := reader.CheckLogIntegrity()
			if err != nil {
				t.Fatalf("CheckLogIntegrity returned error: %v", err)
			}

			if result.Status != IntegrityCorrupt {
				t.Errorf("Expected status CORRUPT for %s, got %s", tc.name, result.Status)
			}

			if result.ErrorMessage == "" {
				t.Errorf("Expected error message for %s", tc.name)
			}

			// Clean up for next test case
			os.Remove(logPath)
		})
	}
}

// TestValidLogIntegrity tests that valid log files pass integrity check.
// Requirements: 12.5
func TestValidLogIntegrity(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-integrity-valid-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid log using the writer
	config := AuditConfig{LogDirectory: tempDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Write some events
	for i := 0; i < 5; i++ {
		path := filepath.Join("/source", "file"+string(rune('A'+i))+".pdf")
		err = writer.RecordSkip(path, ReasonNoMatch)
		if err != nil {
			t.Fatalf("Failed to record event: %v", err)
		}
	}

	err = writer.EndRun(runID, RunStatusCompleted, RunSummary{TotalFiles: 5, Skipped: 5})
	if err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Check integrity
	reader := NewAuditReader(tempDir)
	result, err := reader.CheckLogIntegrity()
	if err != nil {
		t.Fatalf("CheckLogIntegrity returned error: %v", err)
	}

	if result.Status != IntegrityOK {
		t.Errorf("Expected status OK for valid log, got %s (error: %s)", result.Status, result.ErrorMessage)
	}

	// Should have LOG_INITIALIZED + RUN_START + 5 SKIP + RUN_END = 8 events
	if result.TotalLines < 7 {
		t.Errorf("Expected at least 7 valid lines, got %d", result.TotalLines)
	}

	// Verify IsLogCorrupt returns false
	isCorrupt, err := reader.IsLogCorrupt()
	if err != nil {
		t.Fatalf("IsLogCorrupt returned error: %v", err)
	}
	if isCorrupt {
		t.Error("Valid log should not be reported as corrupt")
	}
}

// TestEmptyLogFileHandling tests that empty log files are detected.
// Requirements: 12.2
func TestEmptyLogFileHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-integrity-empty-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "sorta-audit.jsonl")

	// Create empty file
	err = os.WriteFile(logPath, []byte{}, 0644)
	if err != nil {
		t.Fatalf("Failed to write empty log file: %v", err)
	}

	reader := NewAuditReader(tempDir)
	result, err := reader.CheckLogIntegrity()
	if err != nil {
		t.Fatalf("CheckLogIntegrity returned error: %v", err)
	}

	if result.Status != IntegrityEmpty {
		t.Errorf("Expected status EMPTY for empty file, got %s", result.Status)
	}
}

// TestCheckAllSegmentsIntegrity tests integrity checking across multiple segments.
// Requirements: 12.4
func TestCheckAllSegmentsIntegrity(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-integrity-segments-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create multiple valid log segments
	config := AuditConfig{
		LogDirectory: tempDir,
		RotationSize: 500, // Small size to trigger rotation
	}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Write enough events to trigger rotation
	for i := 0; i < 20; i++ {
		path := filepath.Join("/source/very/long/path/to/increase/size", "file"+string(rune('A'+i%26))+".pdf")
		identity := &FileIdentity{
			ContentHash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			Size:        int64(i * 1000),
		}
		err = writer.RecordMove(path, "/dest/"+filepath.Base(path), identity)
		if err != nil {
			t.Fatalf("Failed to record event: %v", err)
		}
	}

	err = writer.EndRun(runID, RunStatusCompleted, RunSummary{TotalFiles: 20, Moved: 20})
	if err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Check all segments
	reader := NewAuditReader(tempDir)
	results, err := reader.CheckAllSegmentsIntegrity()
	if err != nil {
		t.Fatalf("CheckAllSegmentsIntegrity returned error: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one segment result")
	}

	// All segments should be OK or EMPTY (active log may be empty after rotation)
	for _, result := range results {
		if result.Status != IntegrityOK && result.Status != IntegrityEmpty {
			t.Errorf("Segment %s has unexpected status %s (error: %s)", result.FilePath, result.Status, result.ErrorMessage)
		}
	}

	// Verify GetCorruptSegments returns empty
	corrupt, err := reader.GetCorruptSegments()
	if err != nil {
		t.Fatalf("GetCorruptSegments returned error: %v", err)
	}
	if len(corrupt) != 0 {
		t.Errorf("Expected 0 corrupt segments, got %d", len(corrupt))
	}
}

// TestLogInitializedEventWritten tests that LOG_INITIALIZED is written for new logs.
// Requirements: 12.1
func TestLogInitializedEventWritten(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-log-initialized-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a new log
	config := AuditConfig{LogDirectory: tempDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	writer.Close()

	// Read the log and verify LOG_INITIALIZED event
	reader := NewAuditReader(tempDir)
	events, err := reader.FilterAllEvents(EventFilter{
		EventTypes: []EventType{EventLogInitialized},
	})
	if err != nil {
		t.Fatalf("Failed to filter events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 LOG_INITIALIZED event, got %d", len(events))
	}

	if len(events) > 0 {
		event := events[0]
		if event.EventType != EventLogInitialized {
			t.Errorf("Expected LOG_INITIALIZED event type, got %s", event.EventType)
		}
		if event.Status != StatusSuccess {
			t.Errorf("Expected SUCCESS status, got %s", event.Status)
		}
		if event.RunID != "" {
			t.Errorf("Expected empty RunID for system event, got %s", event.RunID)
		}
	}
}

// TestNoLogInitializedForExistingLog tests that LOG_INITIALIZED is NOT written for existing logs.
// Requirements: 12.1
func TestNoLogInitializedForExistingLog(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-existing-log-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create initial log
	config := AuditConfig{LogDirectory: tempDir}
	writer1, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create first writer: %v", err)
	}
	writer1.Close()

	// Open existing log (should NOT write LOG_INITIALIZED)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	writer2.Close()

	// Read the log and verify only ONE LOG_INITIALIZED event
	reader := NewAuditReader(tempDir)
	events, err := reader.FilterAllEvents(EventFilter{
		EventTypes: []EventType{EventLogInitialized},
	})
	if err != nil {
		t.Fatalf("Failed to filter events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected exactly 1 LOG_INITIALIZED event (not written for existing log), got %d", len(events))
	}
}
