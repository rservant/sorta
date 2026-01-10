package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: audit-trail, Property 1: Run_ID Uniqueness and Format
// Validates: Requirements 1.1, 1.2

// uuidV4Regex matches UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
// where y is one of 8, 9, a, or b
var uuidV4Regex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestRunIDUniquenessAndFormat tests Property 1: Run_ID Uniqueness and Format
// For any collection of generated Run_IDs, each Run_ID SHALL be unique and
// SHALL match the UUID v4 format pattern.
func TestRunIDUniquenessAndFormat(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Generated Run_IDs are unique and match UUID v4 format", prop.ForAll(
		func(count int) bool {
			// Generate multiple Run_IDs
			runIDs := make(map[RunID]bool)
			for i := 0; i < count; i++ {
				runID, err := GenerateRunID()
				if err != nil {
					t.Logf("GenerateRunID failed: %v", err)
					return false
				}

				// Check UUID v4 format
				if !uuidV4Regex.MatchString(string(runID)) {
					t.Logf("Run_ID does not match UUID v4 format: %s", runID)
					return false
				}

				// Check uniqueness
				if runIDs[runID] {
					t.Logf("Duplicate Run_ID generated: %s", runID)
					return false
				}
				runIDs[runID] = true
			}
			return true
		},
		// Generate counts between 10 and 50 for each test iteration
		gen.IntRange(10, 50),
	))

	properties.TestingRun(t)
}

// Helper function to create a temporary directory for tests
func createTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "audit-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return dir
}

// Helper function to clean up temporary directory
func cleanupTempDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("Warning: failed to cleanup temp dir: %v", err)
	}
}

// Feature: audit-trail, Property 10: Append-Only Log Integrity
// Validates: Requirements 8.1, 8.5

// TestAppendOnlyLogIntegrity tests Property 10: Append-Only Log Integrity
// For any sequence of write operations to the audit log, existing records
// SHALL remain unchanged and the log file size SHALL never decrease during
// normal operation.
func TestAppendOnlyLogIntegrity(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Append-only log integrity is maintained", prop.ForAll(
		func(eventCount int) bool {
			// Create temp directory for this test
			tempDir, err := os.MkdirTemp("", "audit-append-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{
				LogDirectory: tempDir,
			}

			writer, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create writer: %v", err)
				return false
			}
			defer writer.Close()

			logPath := writer.LogPath()
			var previousSize int64 = 0
			var previousContent []byte

			// Start a run
			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				t.Logf("Failed to start run: %v", err)
				return false
			}

			// Check size increased after StartRun
			info, err := os.Stat(logPath)
			if err != nil {
				t.Logf("Failed to stat log file: %v", err)
				return false
			}
			if info.Size() <= previousSize {
				t.Logf("Log size did not increase after StartRun: %d <= %d", info.Size(), previousSize)
				return false
			}
			previousSize = info.Size()
			previousContent, _ = os.ReadFile(logPath)

			// Write multiple events
			for i := 0; i < eventCount; i++ {
				event := AuditEvent{
					RunID:      runID,
					EventType:  EventMove,
					Status:     StatusSuccess,
					SourcePath: filepath.Join("/source", "file"+string(rune('0'+i%10))+".pdf"),
				}

				if err := writer.WriteEvent(event); err != nil {
					t.Logf("Failed to write event: %v", err)
					return false
				}

				// Check size increased
				info, err := os.Stat(logPath)
				if err != nil {
					t.Logf("Failed to stat log file: %v", err)
					return false
				}
				if info.Size() <= previousSize {
					t.Logf("Log size did not increase after event %d: %d <= %d", i, info.Size(), previousSize)
					return false
				}

				// Check previous content is preserved
				currentContent, _ := os.ReadFile(logPath)
				if len(currentContent) < len(previousContent) {
					t.Logf("Log content shrunk")
					return false
				}
				for j := 0; j < len(previousContent); j++ {
					if currentContent[j] != previousContent[j] {
						t.Logf("Previous content was modified at byte %d", j)
						return false
					}
				}

				previousSize = info.Size()
				previousContent = currentContent
			}

			return true
		},
		// Generate event counts between 5 and 20
		gen.IntRange(5, 20),
	))

	properties.TestingRun(t)
}

// Unit tests for writer error handling
// Requirements: 11.1, 11.5

// TestWriterErrorHandling_ReadOnlyFile tests that writing to a read-only file fails fast.
func TestWriterErrorHandling_ReadOnlyFile(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	// Create a log file and make it read-only
	logPath := filepath.Join(tempDir, "sorta-audit.jsonl")
	if err := os.WriteFile(logPath, []byte{}, 0444); err != nil {
		t.Fatalf("Failed to create read-only file: %v", err)
	}

	config := AuditConfig{
		LogDirectory: tempDir,
	}

	// Creating writer should fail because file is read-only
	_, err := NewAuditWriter(config)
	if err == nil {
		t.Error("Expected error when opening read-only file for writing")
	}
}

// TestWriterErrorHandling_NonExistentDirectory tests that creating a writer
// in a non-existent directory creates the directory.
func TestWriterErrorHandling_NonExistentDirectory(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	// Use a nested non-existent directory
	nestedDir := filepath.Join(tempDir, "nested", "audit", "logs")

	config := AuditConfig{
		LogDirectory: nestedDir,
	}

	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Expected writer to create nested directory, got error: %v", err)
	}
	defer writer.Close()

	// Verify directory was created
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Error("Expected nested directory to be created")
	}
}

// TestWriterErrorHandling_WriteAfterClose tests that writing after close fails.
func TestWriterErrorHandling_WriteAfterClose(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	config := AuditConfig{
		LogDirectory: tempDir,
	}

	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Start a run
	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Close the writer
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Try to write after close - should fail
	event := AuditEvent{
		RunID:     runID,
		EventType: EventMove,
		Status:    StatusSuccess,
	}

	err = writer.WriteEvent(event)
	if err == nil {
		t.Error("Expected error when writing after close")
	}
}

// TestWriterStartRun_GeneratesUniqueID tests that StartRun generates unique IDs.
func TestWriterStartRun_GeneratesUniqueID(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	config := AuditConfig{
		LogDirectory: tempDir,
	}

	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Start multiple runs and verify IDs are unique
	runIDs := make(map[RunID]bool)
	for i := 0; i < 10; i++ {
		runID, err := writer.StartRun("1.0.0", "test-machine")
		if err != nil {
			t.Fatalf("Failed to start run %d: %v", i, err)
		}

		if runIDs[runID] {
			t.Errorf("Duplicate run ID generated: %s", runID)
		}
		runIDs[runID] = true

		// End the run so we can start another
		err = writer.EndRun(runID, RunStatusCompleted, RunSummary{})
		if err != nil {
			t.Fatalf("Failed to end run %d: %v", i, err)
		}
	}
}

// TestWriterEndRun_RecordsSummary tests that EndRun records the summary correctly.
func TestWriterEndRun_RecordsSummary(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	config := AuditConfig{
		LogDirectory: tempDir,
	}

	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	summary := RunSummary{
		TotalFiles:   100,
		Moved:        80,
		Skipped:      10,
		RoutedReview: 5,
		Duplicates:   3,
		Errors:       2,
	}

	err = writer.EndRun(runID, RunStatusCompleted, summary)
	if err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}

	// Read the log file and verify the RUN_END event
	content, err := os.ReadFile(writer.LogPath())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if !containsStr(logContent, `"eventType":"RUN_END"`) {
		t.Error("Log should contain RUN_END event")
	}
	if !containsStr(logContent, `"totalFiles":"100"`) {
		t.Error("Log should contain totalFiles in metadata")
	}
	if !containsStr(logContent, `"moved":"80"`) {
		t.Error("Log should contain moved count in metadata")
	}
}

// TestWriterCurrentRunID tests the CurrentRunID method.
func TestWriterCurrentRunID(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	config := AuditConfig{
		LogDirectory: tempDir,
	}

	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Initially no run is active
	if writer.CurrentRunID() != nil {
		t.Error("Expected nil CurrentRunID before starting a run")
	}

	// Start a run
	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// CurrentRunID should return the run ID
	currentID := writer.CurrentRunID()
	if currentID == nil {
		t.Error("Expected non-nil CurrentRunID after starting a run")
	} else if *currentID != runID {
		t.Errorf("CurrentRunID mismatch: expected %s, got %s", runID, *currentID)
	}

	// End the run
	err = writer.EndRun(runID, RunStatusCompleted, RunSummary{})
	if err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}

	// CurrentRunID should be nil again
	if writer.CurrentRunID() != nil {
		t.Error("Expected nil CurrentRunID after ending a run")
	}
}

// containsStr checks if substr is in s.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Feature: audit-trail, Property 3: Event Field Completeness by Type
// Validates: Requirements 2.1-2.7, 3.1-3.7

// TestEventFieldCompletenessByType tests Property 3: Event Field Completeness by Type
// For any audit event:
// - All events SHALL have timestamp (ISO 8601), runId, eventType, and status fields
// - MOVE events SHALL have sourcePath, destinationPath, and fileIdentity
// - ROUTE_TO_REVIEW events SHALL have sourcePath, destinationPath, and reasonCode
// - SKIP events SHALL have sourcePath and reasonCode
// - DUPLICATE_DETECTED events SHALL have sourcePath, intended destination, actual destination
// - ERROR events SHALL have sourcePath, errorType, and errorMessage
func TestEventFieldCompletenessByType(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("MOVE events have required fields", prop.ForAll(
		func(source, dest string, hash string, size int64) bool {
			tempDir, err := os.MkdirTemp("", "audit-event-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{LogDirectory: tempDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				return false
			}
			defer writer.Close()

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				return false
			}

			identity := &FileIdentity{
				ContentHash: hash,
				Size:        size,
			}

			err = writer.RecordMove(source, dest, identity)
			if err != nil {
				return false
			}

			// Read and parse the log
			content, err := os.ReadFile(writer.LogPath())
			if err != nil {
				return false
			}

			// Find the MOVE event line
			lines := splitLines(string(content))
			for _, line := range lines {
				if containsStr(line, `"eventType":"MOVE"`) {
					event, err := UnmarshalJSONLine([]byte(line))
					if err != nil {
						t.Logf("Failed to unmarshal MOVE event: %v", err)
						return false
					}

					// Verify required fields
					if event.RunID != runID {
						t.Logf("MOVE event missing correct runId")
						return false
					}
					if event.EventType != EventMove {
						t.Logf("MOVE event has wrong eventType")
						return false
					}
					if event.SourcePath != source {
						t.Logf("MOVE event missing sourcePath")
						return false
					}
					if event.DestinationPath != dest {
						t.Logf("MOVE event missing destinationPath")
						return false
					}
					if event.FileIdentity == nil {
						t.Logf("MOVE event missing fileIdentity")
						return false
					}
					if event.FileIdentity.ContentHash != hash {
						t.Logf("MOVE event fileIdentity has wrong contentHash")
						return false
					}
					return true
				}
			}
			t.Logf("MOVE event not found in log")
			return false
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.Int64Range(0, 1000000),
	))

	properties.Property("ROUTE_TO_REVIEW events have required fields", prop.ForAll(
		func(source, dest string) bool {
			tempDir, err := os.MkdirTemp("", "audit-event-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{LogDirectory: tempDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				return false
			}
			defer writer.Close()

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				return false
			}

			err = writer.RecordRouteToReview(source, dest, ReasonUnclassified)
			if err != nil {
				return false
			}

			content, err := os.ReadFile(writer.LogPath())
			if err != nil {
				return false
			}

			lines := splitLines(string(content))
			for _, line := range lines {
				if containsStr(line, `"eventType":"ROUTE_TO_REVIEW"`) {
					event, err := UnmarshalJSONLine([]byte(line))
					if err != nil {
						t.Logf("Failed to unmarshal ROUTE_TO_REVIEW event: %v", err)
						return false
					}

					if event.RunID != runID {
						t.Logf("ROUTE_TO_REVIEW event missing correct runId")
						return false
					}
					if event.SourcePath != source {
						t.Logf("ROUTE_TO_REVIEW event missing sourcePath")
						return false
					}
					if event.DestinationPath != dest {
						t.Logf("ROUTE_TO_REVIEW event missing destinationPath")
						return false
					}
					if event.ReasonCode != ReasonUnclassified {
						t.Logf("ROUTE_TO_REVIEW event missing reasonCode")
						return false
					}
					return true
				}
			}
			t.Logf("ROUTE_TO_REVIEW event not found in log")
			return false
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.Property("SKIP events have required fields", prop.ForAll(
		func(source string) bool {
			tempDir, err := os.MkdirTemp("", "audit-event-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{LogDirectory: tempDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				return false
			}
			defer writer.Close()

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				return false
			}

			err = writer.RecordSkip(source, ReasonNoMatch)
			if err != nil {
				return false
			}

			content, err := os.ReadFile(writer.LogPath())
			if err != nil {
				return false
			}

			lines := splitLines(string(content))
			for _, line := range lines {
				if containsStr(line, `"eventType":"SKIP"`) {
					event, err := UnmarshalJSONLine([]byte(line))
					if err != nil {
						t.Logf("Failed to unmarshal SKIP event: %v", err)
						return false
					}

					if event.RunID != runID {
						t.Logf("SKIP event missing correct runId")
						return false
					}
					if event.SourcePath != source {
						t.Logf("SKIP event missing sourcePath")
						return false
					}
					if event.ReasonCode != ReasonNoMatch {
						t.Logf("SKIP event missing reasonCode")
						return false
					}
					if event.Status != StatusSkipped {
						t.Logf("SKIP event should have SKIPPED status")
						return false
					}
					return true
				}
			}
			t.Logf("SKIP event not found in log")
			return false
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.Property("DUPLICATE_DETECTED events have required fields", prop.ForAll(
		func(source, intended, actual string) bool {
			tempDir, err := os.MkdirTemp("", "audit-event-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{LogDirectory: tempDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				return false
			}
			defer writer.Close()

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				return false
			}

			err = writer.RecordDuplicate(source, intended, actual, ReasonDuplicateRenamed)
			if err != nil {
				return false
			}

			content, err := os.ReadFile(writer.LogPath())
			if err != nil {
				return false
			}

			lines := splitLines(string(content))
			for _, line := range lines {
				if containsStr(line, `"eventType":"DUPLICATE_DETECTED"`) {
					event, err := UnmarshalJSONLine([]byte(line))
					if err != nil {
						t.Logf("Failed to unmarshal DUPLICATE_DETECTED event: %v", err)
						return false
					}

					if event.RunID != runID {
						t.Logf("DUPLICATE_DETECTED event missing correct runId")
						return false
					}
					if event.SourcePath != source {
						t.Logf("DUPLICATE_DETECTED event missing sourcePath")
						return false
					}
					if event.DestinationPath != actual {
						t.Logf("DUPLICATE_DETECTED event missing actual destination")
						return false
					}
					if event.Metadata == nil || event.Metadata["intendedDestination"] != intended {
						t.Logf("DUPLICATE_DETECTED event missing intendedDestination in metadata")
						return false
					}
					return true
				}
			}
			t.Logf("DUPLICATE_DETECTED event not found in log")
			return false
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.Property("ERROR events have required fields", prop.ForAll(
		func(source, errType, errMsg, operation string) bool {
			tempDir, err := os.MkdirTemp("", "audit-event-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{LogDirectory: tempDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				return false
			}
			defer writer.Close()

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				return false
			}

			err = writer.RecordError(source, errType, errMsg, operation)
			if err != nil {
				return false
			}

			content, err := os.ReadFile(writer.LogPath())
			if err != nil {
				return false
			}

			lines := splitLines(string(content))
			for _, line := range lines {
				if containsStr(line, `"eventType":"ERROR"`) {
					event, err := UnmarshalJSONLine([]byte(line))
					if err != nil {
						t.Logf("Failed to unmarshal ERROR event: %v", err)
						return false
					}

					if event.RunID != runID {
						t.Logf("ERROR event missing correct runId")
						return false
					}
					if event.SourcePath != source {
						t.Logf("ERROR event missing sourcePath")
						return false
					}
					if event.ErrorDetails == nil {
						t.Logf("ERROR event missing errorDetails")
						return false
					}
					if event.ErrorDetails.ErrorType != errType {
						t.Logf("ERROR event has wrong errorType")
						return false
					}
					if event.ErrorDetails.ErrorMessage != errMsg {
						t.Logf("ERROR event has wrong errorMessage")
						return false
					}
					if event.ErrorDetails.Operation != operation {
						t.Logf("ERROR event has wrong operation")
						return false
					}
					if event.Status != StatusFailure {
						t.Logf("ERROR event should have FAILURE status")
						return false
					}
					return true
				}
			}
			t.Logf("ERROR event not found in log")
			return false
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.Property("PARSE_FAILURE events have required fields", prop.ForAll(
		func(source, pattern, reason string) bool {
			tempDir, err := os.MkdirTemp("", "audit-event-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{LogDirectory: tempDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				return false
			}
			defer writer.Close()

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				return false
			}

			err = writer.RecordParseFailure(source, pattern, reason)
			if err != nil {
				return false
			}

			content, err := os.ReadFile(writer.LogPath())
			if err != nil {
				return false
			}

			lines := splitLines(string(content))
			for _, line := range lines {
				if containsStr(line, `"eventType":"PARSE_FAILURE"`) {
					event, err := UnmarshalJSONLine([]byte(line))
					if err != nil {
						t.Logf("Failed to unmarshal PARSE_FAILURE event: %v", err)
						return false
					}

					if event.RunID != runID {
						t.Logf("PARSE_FAILURE event missing correct runId")
						return false
					}
					if event.SourcePath != source {
						t.Logf("PARSE_FAILURE event missing sourcePath")
						return false
					}
					if event.Metadata == nil {
						t.Logf("PARSE_FAILURE event missing metadata")
						return false
					}
					if event.Metadata["pattern"] != pattern {
						t.Logf("PARSE_FAILURE event missing pattern in metadata")
						return false
					}
					if event.Metadata["reason"] != reason {
						t.Logf("PARSE_FAILURE event missing reason in metadata")
						return false
					}
					if event.Status != StatusFailure {
						t.Logf("PARSE_FAILURE event should have FAILURE status")
						return false
					}
					return true
				}
			}
			t.Logf("PARSE_FAILURE event not found in log")
			return false
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// Feature: audit-trail, Property 4: One Event Per File
// Validates: Requirements 2.8

// TestOneEventPerFile tests Property 4: One Event Per File
// For any file processed in a run, there SHALL be exactly one primary event
// (MOVE, ROUTE_TO_REVIEW, SKIP, or ERROR) recorded for that file.
func TestOneEventPerFile(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Primary event types that count as "one event per file"
	primaryEventTypes := map[EventType]bool{
		EventMove:          true,
		EventRouteToReview: true,
		EventSkip:          true,
		EventError:         true,
	}

	properties.Property("Each file has exactly one primary event", prop.ForAll(
		func(fileCount int) bool {
			tempDir, err := os.MkdirTemp("", "audit-one-event-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{LogDirectory: tempDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				return false
			}
			defer writer.Close()

			_, err = writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				return false
			}

			// Generate unique file paths and record exactly one event per file
			filePaths := make([]string, fileCount)
			for i := 0; i < fileCount; i++ {
				filePaths[i] = filepath.Join("/source", "file"+string(rune('A'+i%26))+string(rune('0'+i/26))+".pdf")
			}

			// Record one event per file with varying event types
			for i, path := range filePaths {
				switch i % 4 {
				case 0:
					identity := &FileIdentity{ContentHash: "abc123", Size: 1000}
					err = writer.RecordMove(path, "/dest/"+filepath.Base(path), identity)
				case 1:
					err = writer.RecordRouteToReview(path, "/review/"+filepath.Base(path), ReasonUnclassified)
				case 2:
					err = writer.RecordSkip(path, ReasonNoMatch)
				case 3:
					err = writer.RecordError(path, "IO_ERROR", "file not readable", "read")
				}
				if err != nil {
					t.Logf("Failed to record event for %s: %v", path, err)
					return false
				}
			}

			// Read and parse the log
			content, err := os.ReadFile(writer.LogPath())
			if err != nil {
				return false
			}

			// Count primary events per file
			eventCountPerFile := make(map[string]int)
			lines := splitLines(string(content))
			for _, line := range lines {
				if line == "" {
					continue
				}
				event, err := UnmarshalJSONLine([]byte(line))
				if err != nil {
					continue
				}

				// Only count primary event types
				if primaryEventTypes[event.EventType] && event.SourcePath != "" {
					eventCountPerFile[event.SourcePath]++
				}
			}

			// Verify each file has exactly one primary event
			for _, path := range filePaths {
				count := eventCountPerFile[path]
				if count != 1 {
					t.Logf("File %s has %d primary events, expected 1", path, count)
					return false
				}
			}

			return true
		},
		gen.IntRange(5, 30),
	))

	properties.Property("Recording multiple events for same file results in multiple entries", prop.ForAll(
		func(eventCount int) bool {
			// This test verifies that if we DO record multiple events for the same file,
			// they all appear in the log (the system doesn't deduplicate).
			// This is important because the "one event per file" property is about
			// the CALLER's responsibility, not the system's enforcement.
			tempDir, err := os.MkdirTemp("", "audit-multi-event-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			config := AuditConfig{LogDirectory: tempDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				return false
			}
			defer writer.Close()

			_, err = writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				return false
			}

			// Record multiple events for the same file
			sameFilePath := "/source/same-file.pdf"
			for i := 0; i < eventCount; i++ {
				err = writer.RecordSkip(sameFilePath, ReasonNoMatch)
				if err != nil {
					return false
				}
			}

			// Read and count events for this file
			content, err := os.ReadFile(writer.LogPath())
			if err != nil {
				return false
			}

			count := 0
			lines := splitLines(string(content))
			for _, line := range lines {
				if containsStr(line, `"eventType":"SKIP"`) && containsStr(line, sameFilePath) {
					count++
				}
			}

			// All events should be recorded (system doesn't deduplicate)
			if count != eventCount {
				t.Logf("Expected %d SKIP events for same file, got %d", eventCount, count)
				return false
			}

			return true
		},
		gen.IntRange(2, 10),
	))

	properties.TestingRun(t)
}
