// Package audit provides audit trail functionality for Sorta file operations.
package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// IntegrityStatus represents the result of a log integrity check.
type IntegrityStatus string

const (
	// IntegrityOK indicates the log file is valid and complete.
	IntegrityOK IntegrityStatus = "OK"
	// IntegrityMissing indicates the log file does not exist.
	IntegrityMissing IntegrityStatus = "MISSING"
	// IntegrityCorrupt indicates the log file has corruption (e.g., truncated last line).
	IntegrityCorrupt IntegrityStatus = "CORRUPT"
	// IntegrityEmpty indicates the log file exists but is empty.
	IntegrityEmpty IntegrityStatus = "EMPTY"
)

// LogIntegrityResult contains the result of a log integrity check.
type LogIntegrityResult struct {
	Status       IntegrityStatus // Overall integrity status
	FilePath     string          // Path to the log file checked
	TotalLines   int             // Number of valid lines in the file
	ErrorMessage string          // Description of any error found
	ErrorLine    int             // Line number where error was found (0 if N/A)
}

// EventFilter defines criteria for filtering audit events.
type EventFilter struct {
	EventTypes []EventType     // Filter by event types (empty = all types)
	Status     OperationStatus // Filter by status (empty = all statuses)
	StartTime  *time.Time      // Filter events after this time
	EndTime    *time.Time      // Filter events before this time
}

// AuditReader reads and parses audit events from log files.
// It handles reading across multiple rotated segments.
// Requirements: 6.1, 15.1, 15.2, 15.3, 15.4, 15.5
type AuditReader struct {
	logDir string
}

// NewAuditReader creates a new AuditReader for the given log directory.
func NewAuditReader(logDir string) *AuditReader {
	return &AuditReader{
		logDir: logDir,
	}
}

// ListRuns returns all runs with summary information.
// Requirements: 15.1, 15.3
func (r *AuditReader) ListRuns() ([]RunInfo, error) {
	events, err := r.readAllEvents()
	if err != nil {
		return nil, fmt.Errorf("failed to read events: %w", err)
	}

	return r.extractRunInfos(events), nil
}

// GetRun returns all events for a specific run.
// Requirements: 6.1, 15.2
func (r *AuditReader) GetRun(runID RunID) ([]AuditEvent, error) {
	events, err := r.readAllEvents()
	if err != nil {
		return nil, fmt.Errorf("failed to read events: %w", err)
	}

	var runEvents []AuditEvent
	for _, event := range events {
		if event.RunID == runID {
			runEvents = append(runEvents, event)
		}
	}

	if len(runEvents) == 0 {
		return nil, fmt.Errorf("run not found: %s", runID)
	}

	return runEvents, nil
}

// GetLatestRun returns the most recent run by start timestamp.
// Requirements: 15.1
func (r *AuditReader) GetLatestRun() (*RunInfo, error) {
	runs, err := r.ListRuns()
	if err != nil {
		return nil, err
	}

	if len(runs) == 0 {
		return nil, fmt.Errorf("no runs found")
	}

	// Sort by start time descending and return the first
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartTime.After(runs[j].StartTime)
	})

	return &runs[0], nil
}

// FilterEvents returns events matching the filter criteria for a specific run.
// Requirements: 15.5
func (r *AuditReader) FilterEvents(runID RunID, filter EventFilter) ([]AuditEvent, error) {
	events, err := r.GetRun(runID)
	if err != nil {
		return nil, err
	}

	return r.applyFilter(events, filter), nil
}

// FilterAllEvents returns events matching the filter criteria across all runs.
// Requirements: 15.5
func (r *AuditReader) FilterAllEvents(filter EventFilter) ([]AuditEvent, error) {
	events, err := r.readAllEvents()
	if err != nil {
		return nil, fmt.Errorf("failed to read events: %w", err)
	}

	return r.applyFilter(events, filter), nil
}

// applyFilter filters events based on the given criteria.
func (r *AuditReader) applyFilter(events []AuditEvent, filter EventFilter) []AuditEvent {
	var filtered []AuditEvent

	for _, event := range events {
		if r.matchesFilter(event, filter) {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// matchesFilter checks if an event matches the filter criteria.
func (r *AuditReader) matchesFilter(event AuditEvent, filter EventFilter) bool {
	// Check event type filter
	if len(filter.EventTypes) > 0 {
		found := false
		for _, et := range filter.EventTypes {
			if event.EventType == et {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check status filter
	if filter.Status != "" && event.Status != filter.Status {
		return false
	}

	// Check time range filters
	if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
		return false
	}

	return true
}

// readAllEvents reads all events from all log segments in chronological order.
// Requirements: 9.5
func (r *AuditReader) readAllEvents() ([]AuditEvent, error) {
	logFiles, err := GetAllLogFiles(r.logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get log files: %w", err)
	}

	if len(logFiles) == 0 {
		return []AuditEvent{}, nil
	}

	var allEvents []AuditEvent
	for _, logFile := range logFiles {
		events, err := r.readEventsFromFile(logFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read events from %s: %w", logFile, err)
		}
		allEvents = append(allEvents, events...)
	}

	return allEvents, nil
}

// readEventsFromFile reads all events from a single log file.
func (r *AuditReader) readEventsFromFile(filePath string) ([]AuditEvent, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	var events []AuditEvent
	scanner := bufio.NewScanner(file)

	// Increase buffer size for potentially long lines
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		event, err := UnmarshalJSONLine(line)
		if err != nil {
			return nil, fmt.Errorf("failed to parse line %d: %w", lineNum, err)
		}
		events = append(events, *event)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return events, nil
}

// extractRunInfos extracts RunInfo from a list of events.
func (r *AuditReader) extractRunInfos(events []AuditEvent) []RunInfo {
	// Group events by run ID, skipping system events with empty RunID
	runEvents := make(map[RunID][]AuditEvent)
	for _, event := range events {
		// Skip system events (LOG_INITIALIZED, etc.) that have no RunID
		if event.RunID == "" {
			continue
		}
		runEvents[event.RunID] = append(runEvents[event.RunID], event)
	}

	var runs []RunInfo
	for runID, events := range runEvents {
		runInfo := r.buildRunInfo(runID, events)
		runs = append(runs, runInfo)
	}

	// Sort by start time (oldest first)
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartTime.Before(runs[j].StartTime)
	})

	return runs
}

// buildRunInfo constructs a RunInfo from a list of events for a single run.
func (r *AuditReader) buildRunInfo(runID RunID, events []AuditEvent) RunInfo {
	info := RunInfo{
		RunID:   runID,
		Status:  RunStatusInProgress, // Default until we find RUN_END
		RunType: RunTypeOrganize,     // Default
		Summary: RunSummary{},
	}

	for _, event := range events {
		switch event.EventType {
		case EventRunStart:
			info.StartTime = event.Timestamp
			if event.Metadata != nil {
				info.AppVersion = event.Metadata["appVersion"]
				info.MachineID = event.Metadata["machineId"]
				if runType, ok := event.Metadata["runType"]; ok {
					info.RunType = RunType(runType)
				}
				if undoTarget, ok := event.Metadata["undoTargetId"]; ok {
					targetID := RunID(undoTarget)
					info.UndoTargetID = &targetID
					info.RunType = RunTypeUndo
				}
			}

		case EventRunEnd:
			endTime := event.Timestamp
			info.EndTime = &endTime
			if event.Metadata != nil {
				if status, ok := event.Metadata["status"]; ok {
					info.Status = RunStatus(status)
				}
				// Parse summary from metadata
				info.Summary = r.parseSummaryFromMetadata(event.Metadata)
			}

		case EventMove:
			info.Summary.TotalFiles++
			info.Summary.Moved++

		case EventRouteToReview:
			info.Summary.TotalFiles++
			info.Summary.RoutedReview++

		case EventSkip:
			info.Summary.TotalFiles++
			info.Summary.Skipped++

		case EventDuplicateDetected:
			info.Summary.TotalFiles++
			info.Summary.Duplicates++

		case EventError, EventParseFailure, EventValidationFailure:
			info.Summary.TotalFiles++
			info.Summary.Errors++
		}
	}

	return info
}

// parseSummaryFromMetadata parses RunSummary from event metadata.
func (r *AuditReader) parseSummaryFromMetadata(metadata map[string]string) RunSummary {
	summary := RunSummary{}

	if v, ok := metadata["totalFiles"]; ok {
		summary.TotalFiles, _ = strconv.Atoi(v)
	}
	if v, ok := metadata["moved"]; ok {
		summary.Moved, _ = strconv.Atoi(v)
	}
	if v, ok := metadata["skipped"]; ok {
		summary.Skipped, _ = strconv.Atoi(v)
	}
	if v, ok := metadata["routedReview"]; ok {
		summary.RoutedReview, _ = strconv.Atoi(v)
	}
	if v, ok := metadata["duplicates"]; ok {
		summary.Duplicates, _ = strconv.Atoi(v)
	}
	if v, ok := metadata["errors"]; ok {
		summary.Errors, _ = strconv.Atoi(v)
	}

	return summary
}

// GetRunByID returns the RunInfo for a specific run ID.
// Returns an error if the run is not found.
func (r *AuditReader) GetRunByID(runID RunID) (*RunInfo, error) {
	runs, err := r.ListRuns()
	if err != nil {
		return nil, err
	}

	for _, run := range runs {
		if run.RunID == runID {
			return &run, nil
		}
	}

	return nil, fmt.Errorf("run not found: %s", runID)
}

// GetLogDirectory returns the log directory path.
func (r *AuditReader) GetLogDirectory() string {
	return r.logDir
}

// ReadEventsInRange reads events within a time range across all segments.
func (r *AuditReader) ReadEventsInRange(start, end time.Time) ([]AuditEvent, error) {
	filter := EventFilter{
		StartTime: &start,
		EndTime:   &end,
	}
	return r.FilterAllEvents(filter)
}

// GetSegmentFiles returns all log segment files in chronological order.
// This is useful for debugging and testing rotation functionality.
func (r *AuditReader) GetSegmentFiles() ([]string, error) {
	return GetAllLogFiles(r.logDir)
}

// CountEvents returns the total number of events across all segments.
func (r *AuditReader) CountEvents() (int, error) {
	events, err := r.readAllEvents()
	if err != nil {
		return 0, err
	}
	return len(events), nil
}

// GetActiveLogPath returns the path to the active (current) log file.
func (r *AuditReader) GetActiveLogPath() string {
	return filepath.Join(r.logDir, "sorta-audit.jsonl")
}

// CheckLogIntegrity validates the integrity of the active log file.
// It checks that the file exists, is readable, and that the last line is complete JSON.
// Requirements: 12.1, 12.2, 12.5
func (r *AuditReader) CheckLogIntegrity() (*LogIntegrityResult, error) {
	logPath := r.GetActiveLogPath()
	return r.CheckFileIntegrity(logPath)
}

// CheckFileIntegrity validates the integrity of a specific log file.
// Requirements: 12.1, 12.2, 12.5
func (r *AuditReader) CheckFileIntegrity(filePath string) (*LogIntegrityResult, error) {
	result := &LogIntegrityResult{
		FilePath: filePath,
	}

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		result.Status = IntegrityMissing
		result.ErrorMessage = "log file does not exist"
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat log file: %w", err)
	}

	// Check if file is empty
	if fileInfo.Size() == 0 {
		result.Status = IntegrityEmpty
		result.ErrorMessage = "log file is empty"
		return result, nil
	}

	// Open file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Validate all lines are complete JSON
	validLines, corruptLine, corruptErr := r.validateJSONLines(file)
	result.TotalLines = validLines

	if corruptErr != nil {
		result.Status = IntegrityCorrupt
		result.ErrorLine = corruptLine
		result.ErrorMessage = corruptErr.Error()
		return result, nil
	}

	result.Status = IntegrityOK
	return result, nil
}

// validateJSONLines reads through the file and validates each line is valid JSON.
// Returns the number of valid lines, the line number of any corruption, and an error if corrupt.
func (r *AuditReader) validateJSONLines(file *os.File) (validLines int, corruptLine int, err error) {
	scanner := bufio.NewScanner(file)

	// Increase buffer size for potentially long lines
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Validate JSON
		if !json.Valid(line) {
			return validLines, lineNum, fmt.Errorf("invalid JSON at line %d", lineNum)
		}

		// Try to unmarshal to verify it's a valid AuditEvent
		var event AuditEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return validLines, lineNum, fmt.Errorf("failed to parse event at line %d: %w", lineNum, err)
		}

		validLines++
	}

	if err := scanner.Err(); err != nil {
		return validLines, lineNum, fmt.Errorf("error reading file: %w", err)
	}

	// Check for truncated last line by checking if file ends with newline
	if err := r.checkLastLineComplete(file); err != nil {
		return validLines, lineNum, err
	}

	return validLines, 0, nil
}

// checkLastLineComplete verifies the file ends with a newline character.
// A missing trailing newline indicates a truncated last line.
func (r *AuditReader) checkLastLineComplete(file *os.File) error {
	// Seek to end of file
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if fileInfo.Size() == 0 {
		return nil // Empty file is OK
	}

	// Read the last byte
	_, err = file.Seek(-1, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to end: %w", err)
	}

	lastByte := make([]byte, 1)
	_, err = file.Read(lastByte)
	if err != nil {
		return fmt.Errorf("failed to read last byte: %w", err)
	}

	if lastByte[0] != '\n' {
		return fmt.Errorf("truncated last line: file does not end with newline")
	}

	return nil
}

// CheckAllSegmentsIntegrity validates the integrity of all log segments.
// Returns a slice of results, one for each segment file.
// Requirements: 12.4, 12.5
func (r *AuditReader) CheckAllSegmentsIntegrity() ([]LogIntegrityResult, error) {
	logFiles, err := GetAllLogFiles(r.logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get log files: %w", err)
	}

	if len(logFiles) == 0 {
		// No log files exist - return single result indicating missing
		return []LogIntegrityResult{{
			Status:       IntegrityMissing,
			FilePath:     r.GetActiveLogPath(),
			ErrorMessage: "no log files found",
		}}, nil
	}

	var results []LogIntegrityResult
	for _, logFile := range logFiles {
		result, err := r.CheckFileIntegrity(logFile)
		if err != nil {
			return nil, fmt.Errorf("failed to check integrity of %s: %w", logFile, err)
		}
		results = append(results, *result)
	}

	return results, nil
}

// IsLogCorrupt returns true if the active log file is corrupt.
// Requirements: 12.2
func (r *AuditReader) IsLogCorrupt() (bool, error) {
	result, err := r.CheckLogIntegrity()
	if err != nil {
		return false, err
	}
	return result.Status == IntegrityCorrupt, nil
}

// GetCorruptSegments returns a list of corrupt log segments.
// Requirements: 12.4
func (r *AuditReader) GetCorruptSegments() ([]LogIntegrityResult, error) {
	results, err := r.CheckAllSegmentsIntegrity()
	if err != nil {
		return nil, err
	}

	var corrupt []LogIntegrityResult
	for _, result := range results {
		if result.Status == IntegrityCorrupt {
			corrupt = append(corrupt, result)
		}
	}

	return corrupt, nil
}
