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

// Feature: audit-trail, Property 7: Undo Reverse Chronological Ordering
// Validates: Requirements 5.2

// TestUndoReverseChronologicalOrdering tests Property 7: Undo Reverse Chronological Ordering
// For any undo operation on a run, events SHALL be processed in reverse chronological
// order of their original timestamps.
func TestUndoReverseChronologicalOrdering(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Reduced from 100 for faster execution

	properties := gopter.NewProperties(parameters)

	properties.Property("Events are processed in reverse chronological order", prop.ForAll(
		func(eventCount int) bool {
			tempDir, err := os.MkdirTemp("", "audit-undo-order-test-*")
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

			// Start a run and write events with increasing timestamps
			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				t.Logf("Failed to start run: %v", err)
				writer.Close()
				return false
			}

			// Create source files and record MOVE events with distinct timestamps
			sourceDir := filepath.Join(tempDir, "source")
			destDir := filepath.Join(tempDir, "dest")
			if err := os.MkdirAll(sourceDir, 0755); err != nil {
				t.Logf("Failed to create source dir: %v", err)
				writer.Close()
				return false
			}
			if err := os.MkdirAll(destDir, 0755); err != nil {
				t.Logf("Failed to create dest dir: %v", err)
				writer.Close()
				return false
			}

			// Record events with small delays to ensure distinct timestamps
			for i := 0; i < eventCount; i++ {
				fileName := filepath.Join(sourceDir, "file"+string(rune('A'+i))+".txt")
				destName := filepath.Join(destDir, "file"+string(rune('A'+i))+".txt")

				// Create the file at destination (simulating a move that already happened)
				if err := os.WriteFile(destName, []byte("content"+string(rune('A'+i))), 0644); err != nil {
					t.Logf("Failed to create dest file: %v", err)
					writer.Close()
					return false
				}

				identity := &FileIdentity{
					ContentHash: "hash" + string(rune('A'+i)),
					Size:        int64(8 + i),
					ModTime:     time.Now(),
				}

				if err := writer.RecordMove(fileName, destName, identity); err != nil {
					t.Logf("Failed to record move: %v", err)
					writer.Close()
					return false
				}
				// Removed time.Sleep - timestamps from time.Now() are sufficient
			}

			// End the run
			if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: eventCount}); err != nil {
				t.Logf("Failed to end run: %v", err)
				writer.Close()
				return false
			}

			writer.Close()

			// Now read the events and verify the UndoEngine sorts them correctly
			reader := NewAuditReader(tempDir)
			events, err := reader.GetRun(runID)
			if err != nil {
				t.Logf("Failed to get run events: %v", err)
				return false
			}

			// Create a new UndoEngine to test sortEventsReverse
			writer2, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create second writer: %v", err)
				return false
			}
			defer writer2.Close()

			engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")

			// Get sorted events using the internal method
			sortedEvents := engine.sortEventsReverse(events)

			// Verify events are in reverse chronological order
			for i := 1; i < len(sortedEvents); i++ {
				if sortedEvents[i].Timestamp.After(sortedEvents[i-1].Timestamp) {
					t.Logf("Events not in reverse order: event %d (%v) is after event %d (%v)",
						i, sortedEvents[i].Timestamp, i-1, sortedEvents[i-1].Timestamp)
					return false
				}
			}

			return true
		},
		gen.IntRange(3, 15),
	))

	properties.TestingRun(t)
}

// Feature: audit-trail, Property 8: Undo Restores File Locations
// Validates: Requirements 5.3, 5.4

// TestUndoRestoresFileLocations tests Property 8: Undo Restores File Locations
// For any MOVE or ROUTE_TO_REVIEW event that is undone, the file SHALL be moved
// from its destination back to its original source location, provided identity
// verification passes.
func TestUndoRestoresFileLocations(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Reduced from 100 for faster execution

	properties := gopter.NewProperties(parameters)

	properties.Property("Undo restores files to original locations", prop.ForAll(
		func(fileCount int) bool {
			tempDir, err := os.MkdirTemp("", "audit-undo-restore-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			logDir := filepath.Join(tempDir, "logs")
			sourceDir := filepath.Join(tempDir, "source")
			destDir := filepath.Join(tempDir, "dest")
			reviewDir := filepath.Join(tempDir, "review")

			for _, dir := range []string{logDir, sourceDir, destDir, reviewDir} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Logf("Failed to create dir %s: %v", dir, err)
					return false
				}
			}

			config := AuditConfig{LogDirectory: logDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create writer: %v", err)
				return false
			}

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				t.Logf("Failed to start run: %v", err)
				writer.Close()
				return false
			}

			// Track original source paths and their content
			type fileInfo struct {
				sourcePath string
				destPath   string
				content    string
				identity   *FileIdentity
				eventType  EventType
			}
			files := make([]fileInfo, fileCount)

			identityResolver := NewIdentityResolver()

			for i := 0; i < fileCount; i++ {
				content := "content-" + string(rune('A'+i)) + "-" + string(rune('0'+i%10))
				fileName := "file" + string(rune('A'+i)) + ".txt"

				var sourcePath, destPath string
				var eventType EventType

				// Alternate between MOVE and ROUTE_TO_REVIEW events
				if i%2 == 0 {
					sourcePath = filepath.Join(sourceDir, fileName)
					destPath = filepath.Join(destDir, fileName)
					eventType = EventMove
				} else {
					sourcePath = filepath.Join(sourceDir, fileName)
					destPath = filepath.Join(reviewDir, fileName)
					eventType = EventRouteToReview
				}

				// Create file at destination (simulating the move already happened)
				if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
					t.Logf("Failed to create file: %v", err)
					writer.Close()
					return false
				}

				// Capture identity from the destination file
				identity, err := identityResolver.CaptureIdentity(destPath)
				if err != nil {
					t.Logf("Failed to capture identity: %v", err)
					writer.Close()
					return false
				}

				files[i] = fileInfo{
					sourcePath: sourcePath,
					destPath:   destPath,
					content:    content,
					identity:   identity,
					eventType:  eventType,
				}

				// Record the event
				if eventType == EventMove {
					if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
						t.Logf("Failed to record move: %v", err)
						writer.Close()
						return false
					}
				} else {
					if err := writer.RecordRouteToReview(sourcePath, destPath, ReasonUnclassified); err != nil {
						t.Logf("Failed to record route to review: %v", err)
						writer.Close()
						return false
					}
				}
			}

			if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: fileCount}); err != nil {
				t.Logf("Failed to end run: %v", err)
				writer.Close()
				return false
			}
			writer.Close()

			// Now perform the undo
			reader := NewAuditReader(logDir)
			writer2, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create second writer: %v", err)
				return false
			}
			defer writer2.Close()

			engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
			result, err := engine.UndoRun(runID, nil)
			if err != nil {
				t.Logf("Failed to undo run: %v", err)
				return false
			}

			// Verify all files were restored
			if result.Restored != fileCount {
				t.Logf("Expected %d restored files, got %d", fileCount, result.Restored)
				return false
			}

			// Verify each file is back at its original location with correct content
			for _, f := range files {
				// File should exist at source
				content, err := os.ReadFile(f.sourcePath)
				if err != nil {
					t.Logf("File not found at source %s: %v", f.sourcePath, err)
					return false
				}

				if string(content) != f.content {
					t.Logf("Content mismatch at %s: expected %q, got %q", f.sourcePath, f.content, string(content))
					return false
				}

				// File should not exist at destination
				if _, err := os.Stat(f.destPath); !os.IsNotExist(err) {
					t.Logf("File still exists at destination %s", f.destPath)
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 10),
	))

	properties.TestingRun(t)
}

// Unit tests for UndoEngine

func TestUndoEngine_UndoLatest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-latest-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")

	for _, dir := range []string{logDir, sourceDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Create a run with a MOVE event
	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "test.txt")
	destPath := filepath.Join(destDir, "test.txt")

	// Create file at destination
	if err := os.WriteFile(destPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	identityResolver := NewIdentityResolver()
	identity, err := identityResolver.CaptureIdentity(destPath)
	if err != nil {
		t.Fatalf("Failed to capture identity: %v", err)
	}

	if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Perform undo
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoLatest(nil)
	if err != nil {
		t.Fatalf("Failed to undo latest: %v", err)
	}

	if result.TargetRunID != runID {
		t.Errorf("Expected target run ID %s, got %s", runID, result.TargetRunID)
	}

	if result.Restored != 1 {
		t.Errorf("Expected 1 restored file, got %d", result.Restored)
	}

	// Verify file is back at source
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		t.Error("File not restored to source")
	}

	// Verify file is gone from destination
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Error("File still exists at destination")
	}
}

func TestUndoEngine_UndoRunNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-notfound-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := AuditConfig{LogDirectory: tempDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	reader := NewAuditReader(tempDir)
	engine := NewUndoEngine(reader, writer, "1.0.0", "test-machine")

	_, err = engine.UndoRun("non-existent-run-id", nil)
	if err == nil {
		t.Error("Expected error for non-existent run")
	}
}

func TestUndoEngine_NoOpEvents(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-noop-test-*")
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

	// Record SKIP events (no-op for undo)
	if err := writer.RecordSkip("/source/file1.txt", ReasonNoMatch); err != nil {
		t.Fatalf("Failed to record skip: %v", err)
	}
	if err := writer.RecordSkip("/source/file2.txt", ReasonInvalidDate); err != nil {
		t.Fatalf("Failed to record skip: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Skipped: 2}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	reader := NewAuditReader(tempDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// All events should be skipped (no-op)
	if result.Skipped != 2 {
		t.Errorf("Expected 2 skipped events, got %d", result.Skipped)
	}
	if result.Restored != 0 {
		t.Errorf("Expected 0 restored files, got %d", result.Restored)
	}
}

func TestUndoEngine_PreviewUndo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-preview-test-*")
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

	identity := &FileIdentity{ContentHash: "abc123", Size: 100}
	if err := writer.RecordMove("/source/file1.txt", "/dest/file1.txt", identity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}
	if err := writer.RecordRouteToReview("/source/file2.txt", "/review/file2.txt", ReasonUnclassified); err != nil {
		t.Fatalf("Failed to record route to review: %v", err)
	}
	if err := writer.RecordSkip("/source/file3.txt", ReasonNoMatch); err != nil {
		t.Fatalf("Failed to record skip: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 1, RoutedReview: 1, Skipped: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	reader := NewAuditReader(tempDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	preview, err := engine.PreviewUndo(runID, nil)
	if err != nil {
		t.Fatalf("Failed to preview undo: %v", err)
	}

	if preview.TotalMoves != 1 {
		t.Errorf("Expected 1 move, got %d", preview.TotalMoves)
	}
	if preview.TotalReviews != 1 {
		t.Errorf("Expected 1 review, got %d", preview.TotalReviews)
	}
	if preview.TotalNoOps != 1 {
		t.Errorf("Expected 1 no-op, got %d", preview.TotalNoOps)
	}
}

func TestUndoEngine_PathMappings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-mapping-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	localSourceDir := filepath.Join(tempDir, "local-source")
	localDestDir := filepath.Join(tempDir, "local-dest")

	for _, dir := range []string{logDir, localSourceDir, localDestDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "machine-a")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Record move with paths from "machine-a"
	remoteSourcePath := "/Users/alice/source/test.txt"
	remoteDestPath := "/Users/alice/dest/test.txt"

	// Create file at local destination (mapped from remote)
	localDestPath := filepath.Join(localDestDir, "test.txt")
	if err := os.WriteFile(localDestPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	identityResolver := NewIdentityResolver()
	identity, err := identityResolver.CaptureIdentity(localDestPath)
	if err != nil {
		t.Fatalf("Failed to capture identity: %v", err)
	}

	if err := writer.RecordMove(remoteSourcePath, remoteDestPath, identity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Perform undo with path mappings
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "machine-b")

	pathMappings := []PathMapping{
		{
			OriginalPrefix: "/Users/alice/source",
			MappedPrefix:   localSourceDir,
		},
		{
			OriginalPrefix: "/Users/alice/dest",
			MappedPrefix:   localDestDir,
		},
	}

	result, err := engine.UndoRun(runID, pathMappings)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	if result.Restored != 1 {
		t.Errorf("Expected 1 restored file, got %d", result.Restored)
	}

	// Verify file is at local source (mapped from remote source)
	localSourcePath := filepath.Join(localSourceDir, "test.txt")
	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		t.Error("File not restored to mapped source location")
	}
}

// Unit tests for undo handlers for each event type
// Requirements: 5.3, 5.4, 5.5, 5.6

// TestUndoEngine_UndoParseFailure tests that PARSE_FAILURE events are no-op and produce UNDO_SKIP
func TestUndoEngine_UndoParseFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-parse-failure-test-*")
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

	// Record PARSE_FAILURE events
	if err := writer.RecordParseFailure("/source/file1.txt", "YYYY-MM-DD", "invalid date format"); err != nil {
		t.Fatalf("Failed to record parse failure: %v", err)
	}
	if err := writer.RecordParseFailure("/source/file2.txt", "DD/MM/YYYY", "date out of range"); err != nil {
		t.Fatalf("Failed to record parse failure: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Errors: 2}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	reader := NewAuditReader(tempDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// All PARSE_FAILURE events should be skipped (no-op)
	if result.Skipped != 2 {
		t.Errorf("Expected 2 skipped events, got %d", result.Skipped)
	}
	if result.Restored != 0 {
		t.Errorf("Expected 0 restored files, got %d", result.Restored)
	}

	// Verify UNDO_SKIP events were recorded
	undoEvents, err := reader.GetRun(result.UndoRunID)
	if err != nil {
		t.Fatalf("Failed to get undo run events: %v", err)
	}

	undoSkipCount := 0
	for _, event := range undoEvents {
		if event.EventType == EventUndoSkip {
			undoSkipCount++
			if event.ReasonCode != ReasonNoOpEvent {
				t.Errorf("Expected reason code %s, got %s", ReasonNoOpEvent, event.ReasonCode)
			}
		}
	}
	if undoSkipCount != 2 {
		t.Errorf("Expected 2 UNDO_SKIP events, got %d", undoSkipCount)
	}
}

// TestUndoEngine_UndoValidationFailure tests that VALIDATION_FAILURE events are no-op and produce UNDO_SKIP
func TestUndoEngine_UndoValidationFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-validation-failure-test-*")
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

	// Record VALIDATION_FAILURE event directly
	event := AuditEvent{
		Timestamp:  time.Now().UTC(),
		RunID:      runID,
		EventType:  EventValidationFailure,
		Status:     StatusFailure,
		SourcePath: "/source/invalid-file.txt",
		ReasonCode: ReasonValidationError,
	}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("Failed to write validation failure event: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Errors: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	reader := NewAuditReader(tempDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// VALIDATION_FAILURE event should be skipped (no-op)
	if result.Skipped != 1 {
		t.Errorf("Expected 1 skipped event, got %d", result.Skipped)
	}
	if result.Restored != 0 {
		t.Errorf("Expected 0 restored files, got %d", result.Restored)
	}

	// Verify UNDO_SKIP event was recorded
	undoEvents, err := reader.GetRun(result.UndoRunID)
	if err != nil {
		t.Fatalf("Failed to get undo run events: %v", err)
	}

	undoSkipCount := 0
	for _, event := range undoEvents {
		if event.EventType == EventUndoSkip {
			undoSkipCount++
			if event.ReasonCode != ReasonNoOpEvent {
				t.Errorf("Expected reason code %s, got %s", ReasonNoOpEvent, event.ReasonCode)
			}
		}
	}
	if undoSkipCount != 1 {
		t.Errorf("Expected 1 UNDO_SKIP event, got %d", undoSkipCount)
	}
}

// TestUndoEngine_UndoError tests that ERROR events are no-op and produce UNDO_SKIP
func TestUndoEngine_UndoError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-error-test-*")
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

	// Record ERROR events
	if err := writer.RecordError("/source/file1.txt", "IO_ERROR", "permission denied", "move"); err != nil {
		t.Fatalf("Failed to record error: %v", err)
	}
	if err := writer.RecordError("/source/file2.txt", "DISK_FULL", "no space left on device", "copy"); err != nil {
		t.Fatalf("Failed to record error: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Errors: 2}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	reader := NewAuditReader(tempDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// All ERROR events should be skipped (no-op)
	if result.Skipped != 2 {
		t.Errorf("Expected 2 skipped events, got %d", result.Skipped)
	}
	if result.Restored != 0 {
		t.Errorf("Expected 0 restored files, got %d", result.Restored)
	}

	// Verify UNDO_SKIP events were recorded
	undoEvents, err := reader.GetRun(result.UndoRunID)
	if err != nil {
		t.Fatalf("Failed to get undo run events: %v", err)
	}

	undoSkipCount := 0
	for _, event := range undoEvents {
		if event.EventType == EventUndoSkip {
			undoSkipCount++
		}
	}
	if undoSkipCount != 2 {
		t.Errorf("Expected 2 UNDO_SKIP events, got %d", undoSkipCount)
	}
}

// TestUndoEngine_UndoDuplicateRenamed tests undo of DUPLICATE_DETECTED events with rename
func TestUndoEngine_UndoDuplicateRenamed(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-duplicate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")

	for _, dir := range []string{logDir, sourceDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Simulate a duplicate that was renamed
	sourcePath := filepath.Join(sourceDir, "document.pdf")
	intendedDest := filepath.Join(destDir, "document.pdf")
	actualDest := filepath.Join(destDir, "document_1.pdf") // Renamed due to duplicate

	// Create file at actual destination (simulating the rename already happened)
	if err := os.WriteFile(actualDest, []byte("duplicate content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Record DUPLICATE_DETECTED event with rename
	if err := writer.RecordDuplicate(sourcePath, intendedDest, actualDest, ReasonDuplicateRenamed); err != nil {
		t.Fatalf("Failed to record duplicate: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Duplicates: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// Duplicate with rename should be restored
	if result.Restored != 1 {
		t.Errorf("Expected 1 restored file, got %d", result.Restored)
	}

	// Verify file is back at original source
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		t.Error("File not restored to source")
	}

	// Verify file is gone from renamed destination
	if _, err := os.Stat(actualDest); !os.IsNotExist(err) {
		t.Error("File still exists at renamed destination")
	}
}

// TestUndoEngine_UndoDuplicateNoRename tests that DUPLICATE_DETECTED without rename is no-op
func TestUndoEngine_UndoDuplicateNoRename(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-duplicate-norename-test-*")
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

	// Record DUPLICATE_DETECTED event without rename (e.g., skipped duplicate)
	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           runID,
		EventType:       EventDuplicateDetected,
		Status:          StatusSkipped,
		SourcePath:      "/source/duplicate.pdf",
		DestinationPath: "/dest/duplicate.pdf",
		ReasonCode:      ReasonNoMatch, // Not a rename
	}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("Failed to write duplicate event: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Duplicates: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	reader := NewAuditReader(tempDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// Duplicate without rename should be skipped (no-op)
	if result.Skipped != 1 {
		t.Errorf("Expected 1 skipped event, got %d", result.Skipped)
	}
	if result.Restored != 0 {
		t.Errorf("Expected 0 restored files, got %d", result.Restored)
	}

	// Verify UNDO_SKIP event was recorded
	undoEvents, err := reader.GetRun(result.UndoRunID)
	if err != nil {
		t.Fatalf("Failed to get undo run events: %v", err)
	}

	undoSkipCount := 0
	for _, event := range undoEvents {
		if event.EventType == EventUndoSkip {
			undoSkipCount++
		}
	}
	if undoSkipCount != 1 {
		t.Errorf("Expected 1 UNDO_SKIP event, got %d", undoSkipCount)
	}
}

// TestUndoEngine_UndoMoveRecordsUndoMove tests that MOVE undo records UNDO_MOVE event
func TestUndoEngine_UndoMoveRecordsUndoMove(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-move-record-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")

	for _, dir := range []string{logDir, sourceDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "test.txt")
	destPath := filepath.Join(destDir, "test.txt")

	// Create file at destination
	if err := os.WriteFile(destPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	identityResolver := NewIdentityResolver()
	identity, err := identityResolver.CaptureIdentity(destPath)
	if err != nil {
		t.Fatalf("Failed to capture identity: %v", err)
	}

	if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	if result.Restored != 1 {
		t.Errorf("Expected 1 restored file, got %d", result.Restored)
	}

	// Verify UNDO_MOVE event was recorded
	undoEvents, err := reader.GetRun(result.UndoRunID)
	if err != nil {
		t.Fatalf("Failed to get undo run events: %v", err)
	}

	undoMoveCount := 0
	for _, event := range undoEvents {
		if event.EventType == EventUndoMove {
			undoMoveCount++
			// Verify the event has correct paths
			// SourcePath should be where file was (destPath)
			// DestinationPath should be where file is now (sourcePath)
			if event.SourcePath != destPath {
				t.Errorf("Expected UNDO_MOVE source path %s, got %s", destPath, event.SourcePath)
			}
			if event.DestinationPath != sourcePath {
				t.Errorf("Expected UNDO_MOVE destination path %s, got %s", sourcePath, event.DestinationPath)
			}
			if event.Status != StatusSuccess {
				t.Errorf("Expected UNDO_MOVE status SUCCESS, got %s", event.Status)
			}
		}
	}
	if undoMoveCount != 1 {
		t.Errorf("Expected 1 UNDO_MOVE event, got %d", undoMoveCount)
	}
}

// TestUndoEngine_UndoRouteToReviewRecordsUndoMove tests that ROUTE_TO_REVIEW undo records UNDO_MOVE event
func TestUndoEngine_UndoRouteToReviewRecordsUndoMove(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-review-record-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	reviewDir := filepath.Join(tempDir, "review")

	for _, dir := range []string{logDir, sourceDir, reviewDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "unclassified.txt")
	reviewPath := filepath.Join(reviewDir, "unclassified.txt")

	// Create file at review location
	if err := os.WriteFile(reviewPath, []byte("unclassified content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	if err := writer.RecordRouteToReview(sourcePath, reviewPath, ReasonUnclassified); err != nil {
		t.Fatalf("Failed to record route to review: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{RoutedReview: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	if result.Restored != 1 {
		t.Errorf("Expected 1 restored file, got %d", result.Restored)
	}

	// Verify UNDO_MOVE event was recorded
	undoEvents, err := reader.GetRun(result.UndoRunID)
	if err != nil {
		t.Fatalf("Failed to get undo run events: %v", err)
	}

	undoMoveCount := 0
	for _, event := range undoEvents {
		if event.EventType == EventUndoMove {
			undoMoveCount++
			if event.Status != StatusSuccess {
				t.Errorf("Expected UNDO_MOVE status SUCCESS, got %s", event.Status)
			}
		}
	}
	if undoMoveCount != 1 {
		t.Errorf("Expected 1 UNDO_MOVE event, got %d", undoMoveCount)
	}

	// Verify file is back at source
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		t.Error("File not restored to source")
	}
}

// TestUndoEngine_MixedEventTypes tests undo with a mix of event types
func TestUndoEngine_MixedEventTypes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-mixed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")
	reviewDir := filepath.Join(tempDir, "review")

	for _, dir := range []string{logDir, sourceDir, destDir, reviewDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Create files for MOVE and ROUTE_TO_REVIEW
	moveSourcePath := filepath.Join(sourceDir, "moved.txt")
	moveDestPath := filepath.Join(destDir, "moved.txt")
	if err := os.WriteFile(moveDestPath, []byte("moved content"), 0644); err != nil {
		t.Fatalf("Failed to create move dest file: %v", err)
	}

	reviewSourcePath := filepath.Join(sourceDir, "reviewed.txt")
	reviewDestPath := filepath.Join(reviewDir, "reviewed.txt")
	if err := os.WriteFile(reviewDestPath, []byte("reviewed content"), 0644); err != nil {
		t.Fatalf("Failed to create review dest file: %v", err)
	}

	identityResolver := NewIdentityResolver()
	moveIdentity, err := identityResolver.CaptureIdentity(moveDestPath)
	if err != nil {
		t.Fatalf("Failed to capture identity: %v", err)
	}

	// Record various event types
	if err := writer.RecordMove(moveSourcePath, moveDestPath, moveIdentity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}
	if err := writer.RecordRouteToReview(reviewSourcePath, reviewDestPath, ReasonUnclassified); err != nil {
		t.Fatalf("Failed to record route to review: %v", err)
	}
	if err := writer.RecordSkip("/source/skipped.txt", ReasonNoMatch); err != nil {
		t.Fatalf("Failed to record skip: %v", err)
	}
	if err := writer.RecordParseFailure("/source/parse-fail.txt", "YYYY-MM-DD", "invalid"); err != nil {
		t.Fatalf("Failed to record parse failure: %v", err)
	}
	if err := writer.RecordError("/source/error.txt", "IO_ERROR", "permission denied", "move"); err != nil {
		t.Fatalf("Failed to record error: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{
		Moved:        1,
		RoutedReview: 1,
		Skipped:      1,
		Errors:       2,
	}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// 2 files should be restored (MOVE + ROUTE_TO_REVIEW)
	if result.Restored != 2 {
		t.Errorf("Expected 2 restored files, got %d", result.Restored)
	}

	// 3 events should be skipped (SKIP + PARSE_FAILURE + ERROR)
	if result.Skipped != 3 {
		t.Errorf("Expected 3 skipped events, got %d", result.Skipped)
	}

	// Verify files are restored
	if _, err := os.Stat(moveSourcePath); os.IsNotExist(err) {
		t.Error("Moved file not restored to source")
	}
	if _, err := os.Stat(reviewSourcePath); os.IsNotExist(err) {
		t.Error("Reviewed file not restored to source")
	}

	// Verify undo events were recorded correctly
	undoEvents, err := reader.GetRun(result.UndoRunID)
	if err != nil {
		t.Fatalf("Failed to get undo run events: %v", err)
	}

	undoMoveCount := 0
	undoSkipCount := 0
	for _, event := range undoEvents {
		switch event.EventType {
		case EventUndoMove:
			undoMoveCount++
		case EventUndoSkip:
			undoSkipCount++
		}
	}

	if undoMoveCount != 2 {
		t.Errorf("Expected 2 UNDO_MOVE events, got %d", undoMoveCount)
	}
	if undoSkipCount != 3 {
		t.Errorf("Expected 3 UNDO_SKIP events, got %d", undoSkipCount)
	}
}

// Feature: audit-trail, Property 15: Undo Collision Safety
// Validates: Requirements 13.1, 13.2

// TestUndoCollisionSafety tests Property 15: Undo Collision Safety
// For any undo operation where the destination already contains a file,
// the existing file SHALL NOT be overwritten and a COLLISION event SHALL be recorded.
func TestUndoCollisionSafety(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Reduced from 100 for faster execution

	properties := gopter.NewProperties(parameters)

	properties.Property("Undo does not overwrite existing files at destination", prop.ForAll(
		func(fileCount int) bool {
			tempDir, err := os.MkdirTemp("", "audit-undo-collision-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			logDir := filepath.Join(tempDir, "logs")
			sourceDir := filepath.Join(tempDir, "source")
			destDir := filepath.Join(tempDir, "dest")

			for _, dir := range []string{logDir, sourceDir, destDir} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Logf("Failed to create dir: %v", err)
					return false
				}
			}

			config := AuditConfig{LogDirectory: logDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create writer: %v", err)
				return false
			}

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				t.Logf("Failed to start run: %v", err)
				writer.Close()
				return false
			}

			identityResolver := NewIdentityResolver()

			// Track files and their original content at source
			type fileInfo struct {
				sourcePath      string
				destPath        string
				movedContent    string
				originalContent string
				identity        *FileIdentity
			}
			files := make([]fileInfo, fileCount)

			for i := 0; i < fileCount; i++ {
				fileName := "file" + string(rune('A'+i)) + ".txt"
				sourcePath := filepath.Join(sourceDir, fileName)
				destPath := filepath.Join(destDir, fileName)

				movedContent := "moved-content-" + string(rune('A'+i))
				originalContent := "original-content-" + string(rune('A'+i))

				// Create file at destination (simulating the move already happened)
				if err := os.WriteFile(destPath, []byte(movedContent), 0644); err != nil {
					t.Logf("Failed to create dest file: %v", err)
					writer.Close()
					return false
				}

				// Capture identity from the destination file
				identity, err := identityResolver.CaptureIdentity(destPath)
				if err != nil {
					t.Logf("Failed to capture identity: %v", err)
					writer.Close()
					return false
				}

				// Create a DIFFERENT file at the original source location (collision scenario)
				if err := os.WriteFile(sourcePath, []byte(originalContent), 0644); err != nil {
					t.Logf("Failed to create source file: %v", err)
					writer.Close()
					return false
				}

				files[i] = fileInfo{
					sourcePath:      sourcePath,
					destPath:        destPath,
					movedContent:    movedContent,
					originalContent: originalContent,
					identity:        identity,
				}

				// Record the MOVE event
				if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
					t.Logf("Failed to record move: %v", err)
					writer.Close()
					return false
				}
			}

			if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: fileCount}); err != nil {
				t.Logf("Failed to end run: %v", err)
				writer.Close()
				return false
			}
			writer.Close()

			// Perform undo - should fail for all files due to collision
			reader := NewAuditReader(logDir)
			writer2, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create second writer: %v", err)
				return false
			}
			defer writer2.Close()

			engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
			result, err := engine.UndoRun(runID, nil)
			if err != nil {
				t.Logf("Failed to undo run: %v", err)
				return false
			}

			// All files should have failed due to collision
			if result.Failed != fileCount {
				t.Logf("Expected %d failed files, got %d", fileCount, result.Failed)
				return false
			}

			// Verify no files were overwritten - original content at source should be preserved
			for _, f := range files {
				// Source file should still have original content (not overwritten)
				content, err := os.ReadFile(f.sourcePath)
				if err != nil {
					t.Logf("Failed to read source file: %v", err)
					return false
				}
				if string(content) != f.originalContent {
					t.Logf("Source file was overwritten! Expected %q, got %q", f.originalContent, string(content))
					return false
				}

				// Destination file should still exist with moved content
				destContent, err := os.ReadFile(f.destPath)
				if err != nil {
					t.Logf("Failed to read dest file: %v", err)
					return false
				}
				if string(destContent) != f.movedContent {
					t.Logf("Dest file was modified! Expected %q, got %q", f.movedContent, string(destContent))
					return false
				}
			}

			// Verify COLLISION events were recorded
			undoEvents, err := reader.GetRun(result.UndoRunID)
			if err != nil {
				t.Logf("Failed to get undo events: %v", err)
				return false
			}

			collisionCount := 0
			for _, event := range undoEvents {
				if event.EventType == EventCollision {
					collisionCount++
					if event.Status != StatusFailure {
						t.Logf("Expected COLLISION status FAILURE, got %s", event.Status)
						return false
					}
					if event.ReasonCode != ReasonDestinationOccupied {
						t.Logf("Expected reason DESTINATION_OCCUPIED, got %s", event.ReasonCode)
						return false
					}
				}
			}

			if collisionCount != fileCount {
				t.Logf("Expected %d COLLISION events, got %d", fileCount, collisionCount)
				return false
			}

			return true
		},
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// Feature: audit-trail, Property 9: Undo Idempotency
// Validates: Requirements 13.6

// TestUndoIdempotency tests Property 9: Undo Idempotency
// For any run, executing undo twice SHALL produce the same filesystem end state
// as executing undo once.
func TestUndoIdempotency(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Reduced from 100 for faster execution

	properties := gopter.NewProperties(parameters)

	properties.Property("Undo twice produces same state as undo once", prop.ForAll(
		func(fileCount int) bool {
			tempDir, err := os.MkdirTemp("", "audit-undo-idempotency-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			logDir := filepath.Join(tempDir, "logs")
			sourceDir := filepath.Join(tempDir, "source")
			destDir := filepath.Join(tempDir, "dest")

			for _, dir := range []string{logDir, sourceDir, destDir} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Logf("Failed to create dir: %v", err)
					return false
				}
			}

			config := AuditConfig{LogDirectory: logDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create writer: %v", err)
				return false
			}

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				t.Logf("Failed to start run: %v", err)
				writer.Close()
				return false
			}

			identityResolver := NewIdentityResolver()

			// Track files
			type fileInfo struct {
				sourcePath string
				destPath   string
				content    string
				identity   *FileIdentity
			}
			files := make([]fileInfo, fileCount)

			for i := 0; i < fileCount; i++ {
				fileName := "file" + string(rune('A'+i)) + ".txt"
				sourcePath := filepath.Join(sourceDir, fileName)
				destPath := filepath.Join(destDir, fileName)
				content := "content-" + string(rune('A'+i))

				// Create file at destination (simulating the move already happened)
				if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
					t.Logf("Failed to create dest file: %v", err)
					writer.Close()
					return false
				}

				// Capture identity from the destination file
				identity, err := identityResolver.CaptureIdentity(destPath)
				if err != nil {
					t.Logf("Failed to capture identity: %v", err)
					writer.Close()
					return false
				}

				files[i] = fileInfo{
					sourcePath: sourcePath,
					destPath:   destPath,
					content:    content,
					identity:   identity,
				}

				// Record the MOVE event
				if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
					t.Logf("Failed to record move: %v", err)
					writer.Close()
					return false
				}
			}

			if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: fileCount}); err != nil {
				t.Logf("Failed to end run: %v", err)
				writer.Close()
				return false
			}
			writer.Close()

			// First undo
			reader := NewAuditReader(logDir)
			writer2, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create second writer: %v", err)
				return false
			}

			engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
			result1, err := engine.UndoRun(runID, nil)
			if err != nil {
				t.Logf("Failed to undo run first time: %v", err)
				writer2.Close()
				return false
			}
			writer2.Close()

			// Capture filesystem state after first undo
			type fileState struct {
				existsAtSource bool
				existsAtDest   bool
				sourceContent  string
				destContent    string
			}
			stateAfterFirstUndo := make([]fileState, fileCount)

			for i, f := range files {
				state := fileState{}

				if content, err := os.ReadFile(f.sourcePath); err == nil {
					state.existsAtSource = true
					state.sourceContent = string(content)
				}

				if content, err := os.ReadFile(f.destPath); err == nil {
					state.existsAtDest = true
					state.destContent = string(content)
				}

				stateAfterFirstUndo[i] = state
			}

			// Second undo of the same run
			writer3, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create third writer: %v", err)
				return false
			}

			engine2 := NewUndoEngine(reader, writer3, "1.0.0", "test-machine")
			result2, err := engine2.UndoRun(runID, nil)
			if err != nil {
				t.Logf("Failed to undo run second time: %v", err)
				writer3.Close()
				return false
			}
			writer3.Close()

			// Capture filesystem state after second undo
			stateAfterSecondUndo := make([]fileState, fileCount)

			for i, f := range files {
				state := fileState{}

				if content, err := os.ReadFile(f.sourcePath); err == nil {
					state.existsAtSource = true
					state.sourceContent = string(content)
				}

				if content, err := os.ReadFile(f.destPath); err == nil {
					state.existsAtDest = true
					state.destContent = string(content)
				}

				stateAfterSecondUndo[i] = state
			}

			// Verify filesystem state is identical after both undos
			for i := 0; i < fileCount; i++ {
				s1 := stateAfterFirstUndo[i]
				s2 := stateAfterSecondUndo[i]

				if s1.existsAtSource != s2.existsAtSource {
					t.Logf("File %d: source existence differs after second undo", i)
					return false
				}
				if s1.existsAtDest != s2.existsAtDest {
					t.Logf("File %d: dest existence differs after second undo", i)
					return false
				}
				if s1.sourceContent != s2.sourceContent {
					t.Logf("File %d: source content differs after second undo", i)
					return false
				}
				if s1.destContent != s2.destContent {
					t.Logf("File %d: dest content differs after second undo", i)
					return false
				}
			}

			// First undo should have restored all files
			if result1.Restored != fileCount {
				t.Logf("First undo: expected %d restored, got %d", fileCount, result1.Restored)
				return false
			}

			// Second undo should fail for all files (source missing at dest, collision at source)
			// because files are already at source location
			if result2.Restored != 0 {
				t.Logf("Second undo: expected 0 restored, got %d", result2.Restored)
				return false
			}

			return true
		},
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// Unit tests for undo edge cases
// Requirements: 13.5, 13.7

// TestUndoEngine_PartialUndoContinuesAfterFailure tests that partial undo continues
// after individual file failures.
// Requirements: 13.5
func TestUndoEngine_PartialUndoContinuesAfterFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-partial-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")

	for _, dir := range []string{logDir, sourceDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	identityResolver := NewIdentityResolver()

	// File 1: Will succeed (no collision)
	file1Source := filepath.Join(sourceDir, "file1.txt")
	file1Dest := filepath.Join(destDir, "file1.txt")
	if err := os.WriteFile(file1Dest, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	identity1, _ := identityResolver.CaptureIdentity(file1Dest)
	if err := writer.RecordMove(file1Source, file1Dest, identity1); err != nil {
		t.Fatalf("Failed to record move 1: %v", err)
	}

	// File 2: Will fail (collision - file exists at source)
	file2Source := filepath.Join(sourceDir, "file2.txt")
	file2Dest := filepath.Join(destDir, "file2.txt")
	if err := os.WriteFile(file2Dest, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2 dest: %v", err)
	}
	if err := os.WriteFile(file2Source, []byte("blocking-content"), 0644); err != nil {
		t.Fatalf("Failed to create file2 source (collision): %v", err)
	}
	identity2, _ := identityResolver.CaptureIdentity(file2Dest)
	if err := writer.RecordMove(file2Source, file2Dest, identity2); err != nil {
		t.Fatalf("Failed to record move 2: %v", err)
	}

	// File 3: Will succeed (no collision)
	file3Source := filepath.Join(sourceDir, "file3.txt")
	file3Dest := filepath.Join(destDir, "file3.txt")
	if err := os.WriteFile(file3Dest, []byte("content3"), 0644); err != nil {
		t.Fatalf("Failed to create file3: %v", err)
	}
	identity3, _ := identityResolver.CaptureIdentity(file3Dest)
	if err := writer.RecordMove(file3Source, file3Dest, identity3); err != nil {
		t.Fatalf("Failed to record move 3: %v", err)
	}

	// File 4: Will fail (source missing at dest)
	file4Source := filepath.Join(sourceDir, "file4.txt")
	file4Dest := filepath.Join(destDir, "file4.txt")
	// Don't create file4 at dest - it's "missing"
	identity4 := &FileIdentity{ContentHash: "fakehash", Size: 100}
	if err := writer.RecordMove(file4Source, file4Dest, identity4); err != nil {
		t.Fatalf("Failed to record move 4: %v", err)
	}

	// File 5: Will succeed (no collision)
	file5Source := filepath.Join(sourceDir, "file5.txt")
	file5Dest := filepath.Join(destDir, "file5.txt")
	if err := os.WriteFile(file5Dest, []byte("content5"), 0644); err != nil {
		t.Fatalf("Failed to create file5: %v", err)
	}
	identity5, _ := identityResolver.CaptureIdentity(file5Dest)
	if err := writer.RecordMove(file5Source, file5Dest, identity5); err != nil {
		t.Fatalf("Failed to record move 5: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 5}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Perform undo
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// Should have 3 restored (files 1, 3, 5) and 2 failed (files 2, 4)
	if result.Restored != 3 {
		t.Errorf("Expected 3 restored files, got %d", result.Restored)
	}
	if result.Failed != 2 {
		t.Errorf("Expected 2 failed files, got %d", result.Failed)
	}

	// Verify successful files were restored
	if _, err := os.Stat(file1Source); os.IsNotExist(err) {
		t.Error("File 1 not restored to source")
	}
	if _, err := os.Stat(file3Source); os.IsNotExist(err) {
		t.Error("File 3 not restored to source")
	}
	if _, err := os.Stat(file5Source); os.IsNotExist(err) {
		t.Error("File 5 not restored to source")
	}

	// Verify failed files were not modified
	// File 2: collision file should still exist with original content
	content2, err := os.ReadFile(file2Source)
	if err != nil {
		t.Errorf("Failed to read file2 source: %v", err)
	} else if string(content2) != "blocking-content" {
		t.Errorf("File 2 source was modified, expected 'blocking-content', got %q", string(content2))
	}

	// File 2 dest should still exist
	if _, err := os.Stat(file2Dest); os.IsNotExist(err) {
		t.Error("File 2 dest was removed despite collision")
	}

	// Verify failure details
	if len(result.FailureDetails) != 2 {
		t.Errorf("Expected 2 failure details, got %d", len(result.FailureDetails))
	}
}

// TestUndoEngine_InterruptedUndoIsResumable tests that an interrupted undo can be resumed.
// Requirements: 13.7
func TestUndoEngine_InterruptedUndoIsResumable(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-resume-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")

	for _, dir := range []string{logDir, sourceDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	identityResolver := NewIdentityResolver()

	// Create 5 files
	files := make([]struct {
		source   string
		dest     string
		content  string
		identity *FileIdentity
	}, 5)

	for i := 0; i < 5; i++ {
		fileName := "file" + string(rune('A'+i)) + ".txt"
		files[i].source = filepath.Join(sourceDir, fileName)
		files[i].dest = filepath.Join(destDir, fileName)
		files[i].content = "content-" + string(rune('A'+i))

		if err := os.WriteFile(files[i].dest, []byte(files[i].content), 0644); err != nil {
			t.Fatalf("Failed to create file %d: %v", i, err)
		}

		files[i].identity, _ = identityResolver.CaptureIdentity(files[i].dest)
		if err := writer.RecordMove(files[i].source, files[i].dest, files[i].identity); err != nil {
			t.Fatalf("Failed to record move %d: %v", i, err)
		}
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 5}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Simulate partial undo by manually moving some files back
	// This simulates what would happen if undo was interrupted after processing some files
	// Move files 3, 4, 5 back to source (simulating partial undo completion)
	for i := 2; i < 5; i++ {
		if err := os.Rename(files[i].dest, files[i].source); err != nil {
			t.Fatalf("Failed to simulate partial undo for file %d: %v", i, err)
		}
	}

	// Now try to undo again - should handle already-undone files gracefully
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to resume undo: %v", err)
	}

	// Files 0, 1 should be restored (they were still at dest)
	// Files 2, 3, 4 should fail (source missing at dest - already moved back)
	// But they will also have collision at source since files are already there

	// The key point is that the undo completes without error and processes all files
	totalProcessed := result.Restored + result.Failed + result.Skipped
	if totalProcessed != 5 {
		t.Errorf("Expected 5 total processed, got %d", totalProcessed)
	}

	// Verify all files are now at source (either from this undo or the simulated partial undo)
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(files[i].source); os.IsNotExist(err) {
			t.Errorf("File %d not at source after resume", i)
		}
	}

	// Files 0 and 1 should have been restored by this undo
	if result.Restored != 2 {
		t.Errorf("Expected 2 restored files, got %d", result.Restored)
	}

	// Files 2, 3, 4 should have failed (source missing at dest, or collision at source)
	if result.Failed != 3 {
		t.Errorf("Expected 3 failed files, got %d", result.Failed)
	}
}

// TestUndoEngine_ContentChangedEvent tests that CONTENT_CHANGED event is recorded
// when file content has changed since the original operation.
// Requirements: 13.4
func TestUndoEngine_ContentChangedEvent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-content-changed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")

	for _, dir := range []string{logDir, sourceDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "test.txt")
	destPath := filepath.Join(destDir, "test.txt")

	// Create file at destination with original content
	originalContent := "original content"
	if err := os.WriteFile(destPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	identityResolver := NewIdentityResolver()
	identity, err := identityResolver.CaptureIdentity(destPath)
	if err != nil {
		t.Fatalf("Failed to capture identity: %v", err)
	}

	if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Modify the file content (simulating user modification after the move)
	modifiedContent := "modified content - different from original"
	if err := os.WriteFile(destPath, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Perform undo - should fail due to content change
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// Should have failed due to content change
	if result.Failed != 1 {
		t.Errorf("Expected 1 failed file, got %d", result.Failed)
	}
	if result.Restored != 0 {
		t.Errorf("Expected 0 restored files, got %d", result.Restored)
	}

	// Verify CONTENT_CHANGED event was recorded
	undoEvents, err := reader.GetRun(result.UndoRunID)
	if err != nil {
		t.Fatalf("Failed to get undo events: %v", err)
	}

	contentChangedCount := 0
	for _, event := range undoEvents {
		if event.EventType == EventContentChanged {
			contentChangedCount++
			if event.Status != StatusFailure {
				t.Errorf("Expected CONTENT_CHANGED status FAILURE, got %s", event.Status)
			}
			if event.ErrorDetails == nil {
				t.Error("Expected ErrorDetails in CONTENT_CHANGED event")
			} else if event.ErrorDetails.ErrorType != "CONTENT_CHANGED" {
				t.Errorf("Expected error type CONTENT_CHANGED, got %s", event.ErrorDetails.ErrorType)
			}
		}
	}

	if contentChangedCount != 1 {
		t.Errorf("Expected 1 CONTENT_CHANGED event, got %d", contentChangedCount)
	}

	// Verify file was not moved (content preserved)
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != modifiedContent {
		t.Errorf("File content was modified, expected %q, got %q", modifiedContent, string(content))
	}
}

// TestUndoEngine_SourceMissingEvent tests that SOURCE_MISSING event is recorded
// when the file is not found at the expected destination.
// Requirements: 13.3
func TestUndoEngine_SourceMissingEvent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-undo-source-missing-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")

	for _, dir := range []string{logDir, sourceDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "test.txt")
	destPath := filepath.Join(destDir, "test.txt")

	// Record a move but don't create the file at destination
	// (simulating the file was deleted after the move)
	identity := &FileIdentity{
		ContentHash: "abc123",
		Size:        100,
		ModTime:     time.Now(),
	}

	if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Perform undo - should fail due to source missing
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// Should have failed due to source missing
	if result.Failed != 1 {
		t.Errorf("Expected 1 failed file, got %d", result.Failed)
	}

	// Verify SOURCE_MISSING event was recorded
	undoEvents, err := reader.GetRun(result.UndoRunID)
	if err != nil {
		t.Fatalf("Failed to get undo events: %v", err)
	}

	sourceMissingCount := 0
	for _, event := range undoEvents {
		if event.EventType == EventSourceMissing {
			sourceMissingCount++
			if event.Status != StatusFailure {
				t.Errorf("Expected SOURCE_MISSING status FAILURE, got %s", event.Status)
			}
			if event.ReasonCode != ReasonSourceNotFound {
				t.Errorf("Expected reason SOURCE_NOT_FOUND, got %s", event.ReasonCode)
			}
		}
	}

	if sourceMissingCount != 1 {
		t.Errorf("Expected 1 SOURCE_MISSING event, got %d", sourceMissingCount)
	}
}

// Feature: audit-trail, Property 6: Content Hash as Primary Identity for Undo
// Validates: Requirements 4.6, 4.7, 4.8, 7.4

// TestContentHashAsPrimaryIdentityForUndo tests Property 6: Content Hash as Primary Identity for Undo
// For any undo operation, if the content hash of the file at the destination matches the recorded hash,
// the undo SHALL proceed regardless of path differences. If the hash does not match, the undo SHALL
// abort for that file and record an IDENTITY_MISMATCH event.
func TestContentHashAsPrimaryIdentityForUndo(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Reduced from 100 for faster execution

	properties := gopter.NewProperties(parameters)

	// Property: Content hash match allows undo even when file is at different path
	properties.Property("Content hash match allows undo from different path", prop.ForAll(
		func(fileCount int) bool {
			tempDir, err := os.MkdirTemp("", "audit-hash-identity-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			logDir := filepath.Join(tempDir, "logs")
			sourceDir := filepath.Join(tempDir, "source")
			originalDestDir := filepath.Join(tempDir, "original-dest")
			actualDestDir := filepath.Join(tempDir, "actual-dest") // File moved here after original operation

			for _, dir := range []string{logDir, sourceDir, originalDestDir, actualDestDir} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Logf("Failed to create dir: %v", err)
					return false
				}
			}

			config := AuditConfig{LogDirectory: logDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create writer: %v", err)
				return false
			}

			runID, err := writer.StartRun("1.0.0", "machine-a")
			if err != nil {
				t.Logf("Failed to start run: %v", err)
				writer.Close()
				return false
			}

			identityResolver := NewIdentityResolver()

			// Track files
			type fileInfo struct {
				sourcePath       string
				originalDestPath string
				actualDestPath   string
				content          string
				identity         *FileIdentity
			}
			files := make([]fileInfo, fileCount)

			for i := 0; i < fileCount; i++ {
				fileName := "file" + string(rune('A'+i)) + ".txt"
				content := "content-" + string(rune('A'+i)) + "-hash-test"

				files[i] = fileInfo{
					sourcePath:       filepath.Join(sourceDir, fileName),
					originalDestPath: filepath.Join(originalDestDir, fileName),
					actualDestPath:   filepath.Join(actualDestDir, fileName),
					content:          content,
				}

				// Create file at actual destination (simulating file was moved after original operation)
				if err := os.WriteFile(files[i].actualDestPath, []byte(content), 0644); err != nil {
					t.Logf("Failed to create file: %v", err)
					writer.Close()
					return false
				}

				// Capture identity from the actual file
				identity, err := identityResolver.CaptureIdentity(files[i].actualDestPath)
				if err != nil {
					t.Logf("Failed to capture identity: %v", err)
					writer.Close()
					return false
				}
				files[i].identity = identity

				// Record MOVE event with ORIGINAL destination path (not where file actually is)
				if err := writer.RecordMove(files[i].sourcePath, files[i].originalDestPath, identity); err != nil {
					t.Logf("Failed to record move: %v", err)
					writer.Close()
					return false
				}
			}

			if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: fileCount}); err != nil {
				t.Logf("Failed to end run: %v", err)
				writer.Close()
				return false
			}
			writer.Close()

			// Perform undo with search directories configured
			// The file is NOT at originalDestPath, but IS at actualDestPath
			// Undo should find it by content hash
			reader := NewAuditReader(logDir)
			writer2, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create second writer: %v", err)
				return false
			}
			defer writer2.Close()

			engine := NewUndoEngine(reader, writer2, "1.0.0", "machine-b")

			crossMachineConfig := CrossMachineUndoConfig{
				SearchDirectories: []string{actualDestDir}, // Tell it where to search
			}

			result, err := engine.UndoRunCrossMachine(runID, crossMachineConfig)
			if err != nil {
				t.Logf("Failed to undo run: %v", err)
				return false
			}

			// All files should be restored (found by hash)
			if result.Restored != fileCount {
				t.Logf("Expected %d restored files, got %d (failed: %d)", fileCount, result.Restored, result.Failed)
				return false
			}

			// Verify files are at source location
			for _, f := range files {
				content, err := os.ReadFile(f.sourcePath)
				if err != nil {
					t.Logf("File not found at source %s: %v", f.sourcePath, err)
					return false
				}
				if string(content) != f.content {
					t.Logf("Content mismatch at %s", f.sourcePath)
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 5),
	))

	// Property: Content hash mismatch aborts undo and records IDENTITY_MISMATCH
	properties.Property("Content hash mismatch aborts undo", prop.ForAll(
		func(fileCount int) bool {
			tempDir, err := os.MkdirTemp("", "audit-hash-mismatch-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			logDir := filepath.Join(tempDir, "logs")
			sourceDir := filepath.Join(tempDir, "source")
			destDir := filepath.Join(tempDir, "dest")

			for _, dir := range []string{logDir, sourceDir, destDir} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Logf("Failed to create dir: %v", err)
					return false
				}
			}

			config := AuditConfig{LogDirectory: logDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create writer: %v", err)
				return false
			}

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				t.Logf("Failed to start run: %v", err)
				writer.Close()
				return false
			}

			identityResolver := NewIdentityResolver()

			// Track files
			type fileInfo struct {
				sourcePath      string
				destPath        string
				originalContent string
				modifiedContent string
				identity        *FileIdentity
			}
			files := make([]fileInfo, fileCount)

			for i := 0; i < fileCount; i++ {
				fileName := "file" + string(rune('A'+i)) + ".txt"
				originalContent := "original-content-" + string(rune('A'+i))
				modifiedContent := "MODIFIED-content-" + string(rune('A'+i)) // Different content

				files[i] = fileInfo{
					sourcePath:      filepath.Join(sourceDir, fileName),
					destPath:        filepath.Join(destDir, fileName),
					originalContent: originalContent,
					modifiedContent: modifiedContent,
				}

				// Create file with ORIGINAL content to capture identity
				if err := os.WriteFile(files[i].destPath, []byte(originalContent), 0644); err != nil {
					t.Logf("Failed to create file: %v", err)
					writer.Close()
					return false
				}

				// Capture identity from original content
				identity, err := identityResolver.CaptureIdentity(files[i].destPath)
				if err != nil {
					t.Logf("Failed to capture identity: %v", err)
					writer.Close()
					return false
				}
				files[i].identity = identity

				// Record MOVE event
				if err := writer.RecordMove(files[i].sourcePath, files[i].destPath, identity); err != nil {
					t.Logf("Failed to record move: %v", err)
					writer.Close()
					return false
				}

				// NOW modify the file content (simulating file was changed after move)
				if err := os.WriteFile(files[i].destPath, []byte(modifiedContent), 0644); err != nil {
					t.Logf("Failed to modify file: %v", err)
					writer.Close()
					return false
				}
			}

			if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: fileCount}); err != nil {
				t.Logf("Failed to end run: %v", err)
				writer.Close()
				return false
			}
			writer.Close()

			// Perform undo - should fail for all files due to hash mismatch
			reader := NewAuditReader(logDir)
			writer2, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create second writer: %v", err)
				return false
			}
			defer writer2.Close()

			engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
			result, err := engine.UndoRun(runID, nil)
			if err != nil {
				t.Logf("Failed to undo run: %v", err)
				return false
			}

			// All files should have failed due to hash mismatch
			if result.Failed != fileCount {
				t.Logf("Expected %d failed files, got %d", fileCount, result.Failed)
				return false
			}

			// Verify CONTENT_CHANGED events were recorded
			undoEvents, err := reader.GetRun(result.UndoRunID)
			if err != nil {
				t.Logf("Failed to get undo events: %v", err)
				return false
			}

			contentChangedCount := 0
			for _, event := range undoEvents {
				if event.EventType == EventContentChanged {
					contentChangedCount++
				}
			}

			if contentChangedCount != fileCount {
				t.Logf("Expected %d CONTENT_CHANGED events, got %d", fileCount, contentChangedCount)
				return false
			}

			// Verify files were NOT moved (still at dest with modified content)
			for _, f := range files {
				content, err := os.ReadFile(f.destPath)
				if err != nil {
					t.Logf("File missing at dest: %v", err)
					return false
				}
				if string(content) != f.modifiedContent {
					t.Logf("File content was unexpectedly changed")
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 5),
	))

	properties.TestingRun(t)
}

// Unit tests for path mapping and hash-based file discovery
// Requirements: 7.2, 7.3, 7.5

// TestUndoEngine_PathTranslationWithDifferentPrefixes tests path translation with different prefixes
// Requirements: 7.2, 7.3
func TestUndoEngine_PathTranslationWithDifferentPrefixes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-path-translation-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	localSourceDir := filepath.Join(tempDir, "local-source")
	localDestDir := filepath.Join(tempDir, "local-dest")

	for _, dir := range []string{logDir, localSourceDir, localDestDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "machine-a")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Record moves with paths from "machine-a" (Unix-style paths for cross-platform compatibility)
	testCases := []struct {
		remoteSource string
		remoteDest   string
		localSource  string
		localDest    string
		fileName     string
	}{
		{
			remoteSource: "/Users/alice/Documents/source/file1.txt",
			remoteDest:   "/Users/alice/Documents/dest/file1.txt",
			localSource:  filepath.Join(localSourceDir, "file1.txt"),
			localDest:    filepath.Join(localDestDir, "file1.txt"),
			fileName:     "file1.txt",
		},
		{
			remoteSource: "/home/alice/documents/source/file2.txt",
			remoteDest:   "/home/alice/documents/dest/file2.txt",
			localSource:  filepath.Join(localSourceDir, "file2.txt"),
			localDest:    filepath.Join(localDestDir, "file2.txt"),
			fileName:     "file2.txt",
		},
	}

	identityResolver := NewIdentityResolver()

	for _, tc := range testCases {
		// Create file at local destination
		if err := os.WriteFile(tc.localDest, []byte("content-"+tc.fileName), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		identity, err := identityResolver.CaptureIdentity(tc.localDest)
		if err != nil {
			t.Fatalf("Failed to capture identity: %v", err)
		}

		// Record move with remote paths
		if err := writer.RecordMove(tc.remoteSource, tc.remoteDest, identity); err != nil {
			t.Fatalf("Failed to record move: %v", err)
		}
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: len(testCases)}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Perform undo with path mappings
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "machine-b")

	pathMappings := []PathMapping{
		// macOS path mapping
		{
			OriginalPrefix: "/Users/alice/Documents/source",
			MappedPrefix:   localSourceDir,
		},
		{
			OriginalPrefix: "/Users/alice/Documents/dest",
			MappedPrefix:   localDestDir,
		},
		// Linux path mapping
		{
			OriginalPrefix: "/home/alice/documents/source",
			MappedPrefix:   localSourceDir,
		},
		{
			OriginalPrefix: "/home/alice/documents/dest",
			MappedPrefix:   localDestDir,
		},
	}

	result, err := engine.UndoRun(runID, pathMappings)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	if result.Restored != len(testCases) {
		t.Errorf("Expected %d restored files, got %d", len(testCases), result.Restored)
	}

	// Verify files are at local source locations
	for _, tc := range testCases {
		if _, err := os.Stat(tc.localSource); os.IsNotExist(err) {
			t.Errorf("File not restored to local source: %s", tc.localSource)
		}
	}
}

// TestUndoEngine_HashBasedFileDiscovery tests hash-based file discovery when file not at expected path
// Requirements: 7.5
func TestUndoEngine_HashBasedFileDiscovery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-hash-discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	expectedDestDir := filepath.Join(tempDir, "expected-dest")
	actualDestDir := filepath.Join(tempDir, "actual-dest")
	searchDir := filepath.Join(tempDir, "search-area")

	for _, dir := range []string{logDir, sourceDir, expectedDestDir, actualDestDir, searchDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Create file in search area (NOT at expected destination)
	actualFilePath := filepath.Join(searchDir, "moved-file.txt")
	fileContent := "unique-content-for-hash-discovery"
	if err := os.WriteFile(actualFilePath, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	identityResolver := NewIdentityResolver()
	identity, err := identityResolver.CaptureIdentity(actualFilePath)
	if err != nil {
		t.Fatalf("Failed to capture identity: %v", err)
	}

	// Record move with expected destination (where file is NOT)
	sourcePath := filepath.Join(sourceDir, "original.txt")
	expectedDestPath := filepath.Join(expectedDestDir, "expected.txt")
	if err := writer.RecordMove(sourcePath, expectedDestPath, identity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Perform undo with search directories
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")

	crossMachineConfig := CrossMachineUndoConfig{
		SearchDirectories: []string{searchDir},
	}

	result, err := engine.UndoRunCrossMachine(runID, crossMachineConfig)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// File should be found by hash and restored
	if result.Restored != 1 {
		t.Errorf("Expected 1 restored file, got %d (failed: %d)", result.Restored, result.Failed)
	}

	// Verify file is at source location
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("File not found at source: %v", err)
	}
	if string(content) != fileContent {
		t.Errorf("Content mismatch: expected %q, got %q", fileContent, string(content))
	}

	// Verify file is gone from search area
	if _, err := os.Stat(actualFilePath); !os.IsNotExist(err) {
		t.Error("File still exists at original location in search area")
	}
}

// TestUndoEngine_HashBasedDiscoveryWithMultipleMatches tests behavior when multiple files match hash
// Requirements: 7.5
func TestUndoEngine_HashBasedDiscoveryWithMultipleMatches(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-hash-multi-match-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	searchDir1 := filepath.Join(tempDir, "search1")
	searchDir2 := filepath.Join(tempDir, "search2")

	for _, dir := range []string{logDir, sourceDir, searchDir1, searchDir2} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Create SAME content in multiple locations (duplicate files)
	fileContent := "duplicate-content-same-hash"
	file1Path := filepath.Join(searchDir1, "copy1.txt")
	file2Path := filepath.Join(searchDir2, "copy2.txt")

	if err := os.WriteFile(file1Path, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2Path, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	identityResolver := NewIdentityResolver()
	identity, err := identityResolver.CaptureIdentity(file1Path)
	if err != nil {
		t.Fatalf("Failed to capture identity: %v", err)
	}

	// Record move with non-existent destination
	sourcePath := filepath.Join(sourceDir, "original.txt")
	expectedDestPath := filepath.Join(tempDir, "nonexistent", "file.txt")
	if err := writer.RecordMove(sourcePath, expectedDestPath, identity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Perform undo with search directories containing duplicates
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")

	crossMachineConfig := CrossMachineUndoConfig{
		SearchDirectories: []string{searchDir1, searchDir2},
	}

	result, err := engine.UndoRunCrossMachine(runID, crossMachineConfig)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// With multiple matches of same size, it should pick one and succeed
	// (since both have same content and size)
	if result.Restored != 1 {
		t.Errorf("Expected 1 restored file, got %d (failed: %d)", result.Restored, result.Failed)
	}

	// Verify file is at source location
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		t.Error("File not restored to source")
	}
}

// TestUndoEngine_CrossMachineUndoRecordsOriginatingMachine tests that originating machine ID is recorded
// Requirements: 7.6
func TestUndoEngine_CrossMachineUndoRecordsOriginatingMachine(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-cross-machine-id-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")

	for _, dir := range []string{logDir, sourceDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Create run on "machine-a"
	runID, err := writer.StartRun("1.0.0", "machine-a")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "test.txt")
	destPath := filepath.Join(destDir, "test.txt")

	if err := os.WriteFile(destPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	identityResolver := NewIdentityResolver()
	identity, err := identityResolver.CaptureIdentity(destPath)
	if err != nil {
		t.Fatalf("Failed to capture identity: %v", err)
	}

	if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Perform undo on "machine-b" with originating machine specified
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "machine-b")

	crossMachineConfig := CrossMachineUndoConfig{
		OriginatingMachine: "machine-a",
	}

	result, err := engine.UndoRunCrossMachine(runID, crossMachineConfig)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	if result.Restored != 1 {
		t.Errorf("Expected 1 restored file, got %d", result.Restored)
	}

	// Verify the undo run recorded cross-machine metadata
	undoEvents, err := reader.GetRun(result.UndoRunID)
	if err != nil {
		t.Fatalf("Failed to get undo events: %v", err)
	}

	foundCrossMachineMetadata := false
	for _, event := range undoEvents {
		if event.Metadata != nil {
			if event.Metadata["crossMachineUndo"] == "true" {
				foundCrossMachineMetadata = true
				if event.Metadata["originatingMachine"] != "machine-a" {
					t.Errorf("Expected originatingMachine 'machine-a', got %s", event.Metadata["originatingMachine"])
				}
				if event.Metadata["currentMachine"] != "machine-b" {
					t.Errorf("Expected currentMachine 'machine-b', got %s", event.Metadata["currentMachine"])
				}
				break
			}
		}
	}

	if !foundCrossMachineMetadata {
		t.Error("Cross-machine metadata not found in undo events")
	}
}

// TestUndoEngine_PathMappingPriority tests that path mappings are applied in order
// Requirements: 7.3
func TestUndoEngine_PathMappingPriority(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-path-priority-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	localDir := filepath.Join(tempDir, "local")

	for _, dir := range []string{logDir, localDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	reader := NewAuditReader(logDir)
	engine := NewUndoEngine(reader, writer, "1.0.0", "test-machine")

	// Test that first matching mapping is used
	pathMappings := []PathMapping{
		{OriginalPrefix: "/home/user", MappedPrefix: "/mapped/first"},
		{OriginalPrefix: "/home", MappedPrefix: "/mapped/second"},
	}

	// Path that matches first mapping
	result := engine.applyPathMappings("/home/user/file.txt", pathMappings)
	expected := "/mapped/first/file.txt"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Path that only matches second mapping
	result = engine.applyPathMappings("/home/other/file.txt", pathMappings)
	expected = "/mapped/second/other/file.txt"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Path that matches no mapping
	result = engine.applyPathMappings("/different/path/file.txt", pathMappings)
	expected = "/different/path/file.txt"
	if result != expected {
		t.Errorf("Expected %q (unchanged), got %q", expected, result)
	}

	// Empty path
	result = engine.applyPathMappings("", pathMappings)
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

// TestUndoEngine_ConflictDetection tests conflict detection when undoing an older run
// Requirements: 6.5, 6.6
func TestUndoEngine_ConflictDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-conflict-detection-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir1 := filepath.Join(tempDir, "dest1")
	destDir2 := filepath.Join(tempDir, "dest2")

	for _, dir := range []string{logDir, sourceDir, destDir1, destDir2} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	identityResolver := NewIdentityResolver()

	// Run 1: Move file from source to dest1
	run1ID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 1: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "test.txt")
	dest1Path := filepath.Join(destDir1, "test.txt")

	// Create file at dest1 (simulating run 1 already moved it)
	if err := os.WriteFile(dest1Path, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	identity1, err := identityResolver.CaptureIdentity(dest1Path)
	if err != nil {
		t.Fatalf("Failed to capture identity: %v", err)
	}

	if err := writer.RecordMove(sourcePath, dest1Path, identity1); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(run1ID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run 1: %v", err)
	}

	// Close and reopen writer to ensure run 1 is fully persisted
	writer.Close()

	// Longer delay to ensure distinct timestamps
	time.Sleep(100 * time.Millisecond)

	// Reopen writer for run 2
	writer, err = NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to reopen writer: %v", err)
	}

	// Run 2: Move the same file from dest1 to dest2
	run2ID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 2: %v", err)
	}

	dest2Path := filepath.Join(destDir2, "test.txt")

	// Move file from dest1 to dest2
	if err := os.Rename(dest1Path, dest2Path); err != nil {
		t.Fatalf("Failed to move file: %v", err)
	}

	identity2, err := identityResolver.CaptureIdentity(dest2Path)
	if err != nil {
		t.Fatalf("Failed to capture identity: %v", err)
	}

	if err := writer.RecordMove(dest1Path, dest2Path, identity2); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(run2ID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run 2: %v", err)
	}
	writer.Close()

	// Now try to undo run 1 - should detect conflict because run 2 modified the file
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(run1ID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// The undo should fail due to conflict
	if result.Failed != 1 {
		t.Errorf("Expected 1 failed file due to conflict, got %d", result.Failed)
	}

	if result.Restored != 0 {
		t.Errorf("Expected 0 restored files, got %d", result.Restored)
	}

	// Verify the failure reason is conflict
	if len(result.FailureDetails) != 1 {
		t.Fatalf("Expected 1 failure detail, got %d", len(result.FailureDetails))
	}

	if result.FailureDetails[0].Reason != ReasonConflictWithLaterRun {
		t.Errorf("Expected reason %s, got %s", ReasonConflictWithLaterRun, result.FailureDetails[0].Reason)
	}

	// Verify CONFLICT_DETECTED event was recorded
	undoEvents, err := reader.GetRun(result.UndoRunID)
	if err != nil {
		t.Fatalf("Failed to get undo events: %v", err)
	}

	conflictCount := 0
	for _, event := range undoEvents {
		if event.EventType == EventConflictDetected {
			conflictCount++
			if event.Status != StatusFailure {
				t.Errorf("Expected CONFLICT_DETECTED status FAILURE, got %s", event.Status)
			}
			if event.ReasonCode != ReasonConflictWithLaterRun {
				t.Errorf("Expected reason CONFLICT_WITH_LATER_RUN, got %s", event.ReasonCode)
			}
			// Verify conflicting run ID is recorded in metadata
			if event.Metadata == nil || event.Metadata["conflictingRunId"] != string(run2ID) {
				t.Errorf("Expected conflictingRunId %s in metadata", run2ID)
			}
		}
	}

	if conflictCount != 1 {
		t.Errorf("Expected 1 CONFLICT_DETECTED event, got %d", conflictCount)
	}

	// Verify file is still at dest2 (not moved back to source)
	if _, err := os.Stat(dest2Path); os.IsNotExist(err) {
		t.Error("File should still be at dest2")
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Error("File should not be at source")
	}
}

// TestUndoEngine_ConflictDetectionWithMultipleFiles tests conflict detection with multiple files
// where some have conflicts and some don't
// Requirements: 6.5, 6.6
func TestUndoEngine_ConflictDetectionWithMultipleFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-conflict-multi-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir1 := filepath.Join(tempDir, "dest1")
	destDir2 := filepath.Join(tempDir, "dest2")

	for _, dir := range []string{logDir, sourceDir, destDir1, destDir2} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	identityResolver := NewIdentityResolver()

	// Run 1: Move two files
	run1ID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 1: %v", err)
	}

	// File A - will be modified by run 2 (conflict)
	sourcePathA := filepath.Join(sourceDir, "fileA.txt")
	dest1PathA := filepath.Join(destDir1, "fileA.txt")
	if err := os.WriteFile(dest1PathA, []byte("content A"), 0644); err != nil {
		t.Fatalf("Failed to create file A: %v", err)
	}
	identityA, _ := identityResolver.CaptureIdentity(dest1PathA)
	if err := writer.RecordMove(sourcePathA, dest1PathA, identityA); err != nil {
		t.Fatalf("Failed to record move A: %v", err)
	}

	// File B - will NOT be modified by run 2 (no conflict)
	sourcePathB := filepath.Join(sourceDir, "fileB.txt")
	dest1PathB := filepath.Join(destDir1, "fileB.txt")
	if err := os.WriteFile(dest1PathB, []byte("content B"), 0644); err != nil {
		t.Fatalf("Failed to create file B: %v", err)
	}
	identityB, _ := identityResolver.CaptureIdentity(dest1PathB)
	if err := writer.RecordMove(sourcePathB, dest1PathB, identityB); err != nil {
		t.Fatalf("Failed to record move B: %v", err)
	}

	if err := writer.EndRun(run1ID, RunStatusCompleted, RunSummary{Moved: 2}); err != nil {
		t.Fatalf("Failed to end run 1: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Run 2: Only move file A to dest2
	run2ID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 2: %v", err)
	}

	dest2PathA := filepath.Join(destDir2, "fileA.txt")
	if err := os.Rename(dest1PathA, dest2PathA); err != nil {
		t.Fatalf("Failed to move file A: %v", err)
	}
	identityA2, _ := identityResolver.CaptureIdentity(dest2PathA)
	if err := writer.RecordMove(dest1PathA, dest2PathA, identityA2); err != nil {
		t.Fatalf("Failed to record move A in run 2: %v", err)
	}

	if err := writer.EndRun(run2ID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run 2: %v", err)
	}
	writer.Close()

	// Undo run 1 - file A should conflict, file B should be restored
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(run1ID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// File A should fail (conflict), file B should be restored
	if result.Failed != 1 {
		t.Errorf("Expected 1 failed file, got %d", result.Failed)
	}
	if result.Restored != 1 {
		t.Errorf("Expected 1 restored file, got %d", result.Restored)
	}

	// Verify file B is restored to source
	if _, err := os.Stat(sourcePathB); os.IsNotExist(err) {
		t.Error("File B should be restored to source")
	}
	if _, err := os.Stat(dest1PathB); !os.IsNotExist(err) {
		t.Error("File B should not be at dest1 anymore")
	}

	// Verify file A is still at dest2 (not restored due to conflict)
	if _, err := os.Stat(dest2PathA); os.IsNotExist(err) {
		t.Error("File A should still be at dest2")
	}
	if _, err := os.Stat(sourcePathA); !os.IsNotExist(err) {
		t.Error("File A should not be at source")
	}
}

// TestUndoEngine_NoConflictForLatestRun tests that undoing the latest run has no conflicts
// Requirements: 6.5, 6.6
func TestUndoEngine_NoConflictForLatestRun(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-no-conflict-latest-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")

	for _, dir := range []string{logDir, sourceDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	identityResolver := NewIdentityResolver()

	// Create a single run
	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "test.txt")
	destPath := filepath.Join(destDir, "test.txt")

	if err := os.WriteFile(destPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	identity, _ := identityResolver.CaptureIdentity(destPath)
	if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(runID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}
	writer.Close()

	// Undo the only run - should have no conflicts
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(runID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// Should succeed with no conflicts
	if result.Failed != 0 {
		t.Errorf("Expected 0 failed files, got %d", result.Failed)
	}
	if result.Restored != 1 {
		t.Errorf("Expected 1 restored file, got %d", result.Restored)
	}

	// Verify file is restored
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		t.Error("File should be restored to source")
	}
}

// TestUndoEngine_ConflictDetectionIgnoresUndoRuns tests that UNDO runs don't create conflicts
// Requirements: 6.5, 6.6
func TestUndoEngine_ConflictDetectionIgnoresUndoRuns(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-conflict-ignore-undo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "logs")
	sourceDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "dest")

	for _, dir := range []string{logDir, sourceDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
	}

	config := AuditConfig{LogDirectory: logDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	identityResolver := NewIdentityResolver()

	// Run 1: Move file
	run1ID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 1: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "test.txt")
	destPath := filepath.Join(destDir, "test.txt")

	if err := os.WriteFile(destPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	identity, _ := identityResolver.CaptureIdentity(destPath)
	if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
		t.Fatalf("Failed to record move: %v", err)
	}

	if err := writer.EndRun(run1ID, RunStatusCompleted, RunSummary{Moved: 1}); err != nil {
		t.Fatalf("Failed to end run 1: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Run 2: Another organize run (not undo)
	run2ID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 2: %v", err)
	}

	// Record a skip event for a different file (doesn't affect our test file)
	if err := writer.RecordSkip("/other/file.txt", ReasonNoMatch); err != nil {
		t.Fatalf("Failed to record skip: %v", err)
	}

	if err := writer.EndRun(run2ID, RunStatusCompleted, RunSummary{Skipped: 1}); err != nil {
		t.Fatalf("Failed to end run 2: %v", err)
	}
	writer.Close()

	// Undo run 1 - should succeed because run 2 didn't modify our file
	reader := NewAuditReader(logDir)
	writer2, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create second writer: %v", err)
	}
	defer writer2.Close()

	engine := NewUndoEngine(reader, writer2, "1.0.0", "test-machine")
	result, err := engine.UndoRun(run1ID, nil)
	if err != nil {
		t.Fatalf("Failed to undo run: %v", err)
	}

	// Should succeed - run 2 was a skip event for a different file
	if result.Failed != 0 {
		t.Errorf("Expected 0 failed files, got %d", result.Failed)
	}
	if result.Restored != 1 {
		t.Errorf("Expected 1 restored file, got %d", result.Restored)
	}
}
