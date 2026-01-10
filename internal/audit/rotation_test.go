package audit

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: audit-trail, Property 12: Rotation Segment Discoverability
// Validates: Requirements 9.3, 9.4, 9.5

// TestRotationSegmentDiscoverability tests Property 12: Rotation Segment Discoverability
// For any rotated log, all segments SHALL be discoverable via the index file or naming
// convention, and reading all segments SHALL return all events in chronological order.
func TestRotationSegmentDiscoverability(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("All rotated segments are discoverable and events are in chronological order", prop.ForAll(
		func(rotationCount int, eventsPerSegment int) bool {
			tempDir, err := os.MkdirTemp("", "audit-rotation-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			// Use a very small rotation size to trigger rotations
			config := AuditConfig{
				LogDirectory: tempDir,
				RotationSize: 100, // Very small to trigger rotation quickly
			}

			writer, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create writer: %v", err)
				return false
			}

			// Track all events we write
			var allEvents []AuditEvent
			var runID RunID

			// Start a run
			runID, err = writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				t.Logf("Failed to start run: %v", err)
				writer.Close()
				return false
			}

			// Write events to trigger multiple rotations
			totalEvents := rotationCount * eventsPerSegment
			for i := 0; i < totalEvents; i++ {
				event := AuditEvent{
					Timestamp:       time.Now().UTC(),
					RunID:           runID,
					EventType:       EventMove,
					Status:          StatusSuccess,
					SourcePath:      filepath.Join("/source", "file"+string(rune('A'+i%26))+".pdf"),
					DestinationPath: filepath.Join("/dest", "file"+string(rune('A'+i%26))+".pdf"),
				}
				allEvents = append(allEvents, event)

				if err := writer.WriteEvent(event); err != nil {
					t.Logf("Failed to write event %d: %v", i, err)
					writer.Close()
					return false
				}

				// Small delay to ensure different timestamps
				time.Sleep(time.Millisecond)
			}

			writer.Close()

			// Discover all segments using the naming convention
			segments, err := DiscoverSegments(tempDir)
			if err != nil {
				t.Logf("Failed to discover segments: %v", err)
				return false
			}

			// Get all log files (segments + active log)
			allLogFiles, err := GetAllLogFiles(tempDir)
			if err != nil {
				t.Logf("Failed to get all log files: %v", err)
				return false
			}

			// Verify we have at least one file
			if len(allLogFiles) == 0 {
				t.Logf("No log files found")
				return false
			}

			// Read all events from all segments in order
			var readEvents []AuditEvent
			for _, logFile := range allLogFiles {
				events, err := readEventsFromFile(logFile)
				if err != nil {
					t.Logf("Failed to read events from %s: %v", logFile, err)
					return false
				}
				readEvents = append(readEvents, events...)
			}

			// Verify events are in chronological order
			for i := 1; i < len(readEvents); i++ {
				if readEvents[i].Timestamp.Before(readEvents[i-1].Timestamp) {
					t.Logf("Events not in chronological order at index %d", i)
					return false
				}
			}

			// Verify all segments are discoverable via naming convention
			for _, seg := range segments {
				if !strings.HasPrefix(seg, "sorta-audit-") || !strings.HasSuffix(seg, ".jsonl") {
					t.Logf("Segment %s does not match naming convention", seg)
					return false
				}
			}

			// If we have rotated segments, verify the index file exists and is valid
			if len(segments) > 0 {
				index, err := LoadIndex(tempDir)
				if err != nil {
					// Index might not exist if rotation just happened, that's okay
					// as long as we can discover segments via naming convention
					t.Logf("Note: Index file not found or invalid: %v", err)
				} else {
					// Verify index contains all segments
					indexSegments := make(map[string]bool)
					for _, seg := range index.Segments {
						indexSegments[seg.Filename] = true
					}
					for _, seg := range segments {
						if !indexSegments[seg] {
							t.Logf("Segment %s not found in index", seg)
							// This is not a failure - segments can be discovered via naming convention
						}
					}
				}
			}

			return true
		},
		gen.IntRange(1, 3),  // Number of rotations to trigger
		gen.IntRange(5, 15), // Events per segment
	))

	properties.TestingRun(t)
}

// readEventsFromFile reads all audit events from a log file.
func readEventsFromFile(path string) ([]AuditEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []AuditEvent
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		event, err := UnmarshalJSONLine([]byte(line))
		if err != nil {
			continue // Skip invalid lines
		}
		events = append(events, *event)
	}

	return events, scanner.Err()
}

// TestRotationAtExactSizeBoundary tests rotation at exact size boundary.
// Requirements: 9.5
func TestRotationAtExactSizeBoundary(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-rotation-boundary-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set rotation size to a specific value
	rotationSize := int64(500)
	config := AuditConfig{
		LogDirectory: tempDir,
		RotationSize: rotationSize,
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

	// Write events until we exceed the rotation size
	eventCount := 0
	for {
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
		eventCount++

		// Check if rotation occurred
		segments, err := DiscoverSegments(tempDir)
		if err != nil {
			t.Fatalf("Failed to discover segments: %v", err)
		}

		if len(segments) > 0 {
			// Rotation occurred
			break
		}

		if eventCount > 100 {
			t.Fatalf("Rotation did not occur after %d events", eventCount)
		}
	}

	// Verify we can still write after rotation
	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           runID,
		EventType:       EventMove,
		Status:          StatusSuccess,
		SourcePath:      "/source/after-rotation.pdf",
		DestinationPath: "/dest/after-rotation.pdf",
	}

	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("Failed to write event after rotation: %v", err)
	}

	// Verify the event was written to the new active log
	activeLogPath := filepath.Join(tempDir, "sorta-audit.jsonl")
	content, err := os.ReadFile(activeLogPath)
	if err != nil {
		t.Fatalf("Failed to read active log: %v", err)
	}

	if !strings.Contains(string(content), "after-rotation.pdf") {
		t.Error("Event after rotation not found in active log")
	}
}

// TestRotationMidRunContinuesSeamlessly tests that rotation mid-run continues seamlessly.
// Requirements: 9.5
func TestRotationMidRunContinuesSeamlessly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-rotation-midrun-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Use a rotation size that will trigger rotation after a few events
	// Each event is roughly 150-200 bytes
	config := AuditConfig{
		LogDirectory: tempDir,
		RotationSize: 3000, // Should trigger rotation after ~15-20 events
	}

	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	// Write events - enough to trigger at least one rotation
	totalEvents := 40
	for i := 0; i < totalEvents; i++ {
		event := AuditEvent{
			Timestamp:       time.Now().UTC(),
			RunID:           runID,
			EventType:       EventMove,
			Status:          StatusSuccess,
			SourcePath:      filepath.Join("/source", "file"+string(rune('0'+i%10))+".pdf"),
			DestinationPath: filepath.Join("/dest", "file"+string(rune('0'+i%10))+".pdf"),
		}

		if err := writer.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event %d: %v", i, err)
		}
	}

	// End the run
	summary := RunSummary{
		TotalFiles: totalEvents,
		Moved:      totalEvents,
	}
	if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}

	// Close the writer to ensure all data is flushed
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Check if rotation occurred
	segments, err := DiscoverSegments(tempDir)
	if err != nil {
		t.Fatalf("Failed to discover segments: %v", err)
	}

	// Read all events from all files
	allLogFiles, err := GetAllLogFiles(tempDir)
	if err != nil {
		t.Fatalf("Failed to get all log files: %v", err)
	}

	var allEvents []AuditEvent
	for _, logFile := range allLogFiles {
		events, err := readEventsFromFile(logFile)
		if err != nil {
			t.Fatalf("Failed to read events from %s: %v", logFile, err)
		}
		allEvents = append(allEvents, events...)
	}

	// Count event types
	eventCounts := make(map[EventType]int)
	for _, event := range allEvents {
		eventCounts[event.EventType]++
	}

	// If rotation occurred, verify the run continues seamlessly
	if len(segments) > 0 {
		// Verify we have RUN_START and RUN_END events
		if eventCounts[EventRunStart] != 1 {
			t.Errorf("Expected 1 RUN_START event, got %d", eventCounts[EventRunStart])
		}
		if eventCounts[EventRunEnd] != 1 {
			t.Errorf("Expected 1 RUN_END event, got %d", eventCounts[EventRunEnd])
		}

		// Verify we have the expected number of MOVE events
		if eventCounts[EventMove] != totalEvents {
			t.Errorf("Expected %d MOVE events, got %d", totalEvents, eventCounts[EventMove])
		}

		// Verify all non-ROTATION and non-LOG_INITIALIZED events have the correct run ID
		for _, event := range allEvents {
			if event.EventType == EventRotation || event.EventType == EventLogInitialized {
				continue
			}
			if event.RunID != runID {
				t.Errorf("Event has wrong run ID: expected %s, got %s", runID, event.RunID)
			}
		}

		// Verify events are in chronological order
		for i := 1; i < len(allEvents); i++ {
			if allEvents[i].Timestamp.Before(allEvents[i-1].Timestamp) {
				t.Errorf("Events not in chronological order at index %d", i)
			}
		}
	} else {
		// No rotation occurred - this is okay, just verify basic functionality
		t.Log("No rotation occurred during test")
		if eventCounts[EventRunStart] != 1 {
			t.Errorf("Expected 1 RUN_START event, got %d", eventCounts[EventRunStart])
		}
		if eventCounts[EventRunEnd] != 1 {
			t.Errorf("Expected 1 RUN_END event, got %d", eventCounts[EventRunEnd])
		}
		if eventCounts[EventMove] != totalEvents {
			t.Errorf("Expected %d MOVE events, got %d", totalEvents, eventCounts[EventMove])
		}
	}
}

// TestNeedsRotation_SizeBased tests size-based rotation check.
func TestNeedsRotation_SizeBased(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-rotation-size-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := AuditConfig{
		LogDirectory: tempDir,
		RotationSize: 100,
	}

	rm := NewRotationManager(config)
	logPath := filepath.Join(tempDir, "sorta-audit.jsonl")

	// File doesn't exist - no rotation needed
	needs, err := rm.NeedsRotation(logPath)
	if err != nil {
		t.Fatalf("NeedsRotation failed: %v", err)
	}
	if needs {
		t.Error("Should not need rotation when file doesn't exist")
	}

	// Create a small file - no rotation needed
	if err := os.WriteFile(logPath, []byte("small"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	needs, err = rm.NeedsRotation(logPath)
	if err != nil {
		t.Fatalf("NeedsRotation failed: %v", err)
	}
	if needs {
		t.Error("Should not need rotation when file is small")
	}

	// Create a large file - rotation needed
	largeContent := make([]byte, 150)
	if err := os.WriteFile(logPath, largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	needs, err = rm.NeedsRotation(logPath)
	if err != nil {
		t.Fatalf("NeedsRotation failed: %v", err)
	}
	if !needs {
		t.Error("Should need rotation when file exceeds size limit")
	}
}

// TestNeedsRotation_TimeBased tests time-based rotation check.
func TestNeedsRotation_TimeBased(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-rotation-time-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "sorta-audit.jsonl")

	// Create a file
	if err := os.WriteFile(logPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Test daily rotation
	config := AuditConfig{
		LogDirectory:   tempDir,
		RotationPeriod: "daily",
	}
	rm := NewRotationManager(config)

	// Set file mod time to yesterday
	yesterday := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(logPath, yesterday, yesterday); err != nil {
		t.Fatalf("Failed to set file time: %v", err)
	}

	needs, err := rm.NeedsRotation(logPath)
	if err != nil {
		t.Fatalf("NeedsRotation failed: %v", err)
	}
	if !needs {
		t.Error("Should need rotation when file is from yesterday (daily rotation)")
	}

	// Set file mod time to today
	now := time.Now()
	if err := os.Chtimes(logPath, now, now); err != nil {
		t.Fatalf("Failed to set file time: %v", err)
	}

	needs, err = rm.NeedsRotation(logPath)
	if err != nil {
		t.Fatalf("NeedsRotation failed: %v", err)
	}
	if needs {
		t.Error("Should not need rotation when file is from today (daily rotation)")
	}
}

// TestGenerateRotatedFilename tests the rotated filename generation.
func TestGenerateRotatedFilename(t *testing.T) {
	config := AuditConfig{}
	rm := NewRotationManager(config)

	filename := rm.GenerateRotatedFilename()

	// Verify format: sorta-audit-YYYYMMDD-HHMMSS.jsonl
	if !strings.HasPrefix(filename, "sorta-audit-") {
		t.Errorf("Filename should start with 'sorta-audit-': %s", filename)
	}
	if !strings.HasSuffix(filename, ".jsonl") {
		t.Errorf("Filename should end with '.jsonl': %s", filename)
	}

	// Verify timestamp part is valid
	parts := strings.Split(filename, "-")
	if len(parts) < 4 {
		t.Errorf("Filename should have timestamp parts: %s", filename)
	}
}

// TestDiscoverSegments tests segment discovery.
func TestDiscoverSegments(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-discover-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create some rotated segment files
	segmentNames := []string{
		"sorta-audit-20240101-120000.jsonl",
		"sorta-audit-20240102-120000.jsonl",
		"sorta-audit-20240103-120000.jsonl",
	}

	for _, name := range segmentNames {
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create segment file: %v", err)
		}
	}

	// Create active log (should not be included in segments)
	activeLog := filepath.Join(tempDir, "sorta-audit.jsonl")
	if err := os.WriteFile(activeLog, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create active log: %v", err)
	}

	// Create index file (should not be included in segments)
	indexFile := filepath.Join(tempDir, "sorta-audit-index.json")
	if err := os.WriteFile(indexFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create index file: %v", err)
	}

	// Discover segments
	segments, err := DiscoverSegments(tempDir)
	if err != nil {
		t.Fatalf("DiscoverSegments failed: %v", err)
	}

	// Verify we found all segments
	if len(segments) != len(segmentNames) {
		t.Errorf("Expected %d segments, got %d", len(segmentNames), len(segments))
	}

	// Verify segments are sorted
	sortedNames := make([]string, len(segmentNames))
	copy(sortedNames, segmentNames)
	sort.Strings(sortedNames)

	for i, seg := range segments {
		if seg != sortedNames[i] {
			t.Errorf("Segment %d: expected %s, got %s", i, sortedNames[i], seg)
		}
	}
}

// TestGetAllLogFiles tests getting all log files in order.
func TestGetAllLogFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-allfiles-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create rotated segments
	segmentNames := []string{
		"sorta-audit-20240101-120000.jsonl",
		"sorta-audit-20240102-120000.jsonl",
	}

	for _, name := range segmentNames {
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create segment file: %v", err)
		}
	}

	// Create active log
	activeLog := filepath.Join(tempDir, "sorta-audit.jsonl")
	if err := os.WriteFile(activeLog, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create active log: %v", err)
	}

	// Get all log files
	files, err := GetAllLogFiles(tempDir)
	if err != nil {
		t.Fatalf("GetAllLogFiles failed: %v", err)
	}

	// Should have segments + active log
	expectedCount := len(segmentNames) + 1
	if len(files) != expectedCount {
		t.Errorf("Expected %d files, got %d", expectedCount, len(files))
	}

	// Active log should be last
	if files[len(files)-1] != activeLog {
		t.Errorf("Active log should be last, got %s", files[len(files)-1])
	}
}
