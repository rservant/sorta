// Package audit provides audit trail functionality for Sorta file operations.
package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// UndoResult contains the result of an undo operation.
type UndoResult struct {
	UndoRunID      RunID       // The run ID of the undo operation itself
	TargetRunID    RunID       // The run ID that was undone
	TotalEvents    int         // Total events processed
	Restored       int         // Files successfully restored
	Skipped        int         // Files skipped (no-op events)
	Failed         int         // Files that failed to restore
	FailureDetails []UndoError // Details of failures
}

// UndoError contains details about a failed undo operation.
type UndoError struct {
	SourcePath string     // Original source path
	DestPath   string     // Destination path (where file currently is)
	Reason     ReasonCode // Reason for failure
	Message    string     // Detailed error message
}

// UndoPreview shows what would be undone without executing.
type UndoPreview struct {
	TargetRunID  RunID              // The run ID that would be undone
	EventsToUndo []UndoPreviewEvent // Events that would be processed
	TotalMoves   int                // Number of MOVE events to undo
	TotalReviews int                // Number of ROUTE_TO_REVIEW events to undo
	TotalNoOps   int                // Number of no-op events (SKIP, etc.)
}

// UndoPreviewEvent represents a single event in the undo preview.
type UndoPreviewEvent struct {
	EventType   EventType // Original event type
	SourcePath  string    // Original source path
	DestPath    string    // Current destination path
	WillRestore bool      // Whether this event will result in a file move
}

// CrossMachineUndoConfig holds configuration for cross-machine undo operations.
// Requirements: 7.2, 7.3, 7.5, 7.6
type CrossMachineUndoConfig struct {
	PathMappings       []PathMapping // Path translations between machines
	SearchDirectories  []string      // Directories to search when file not at expected path
	OriginatingMachine string        // Machine ID where the original run was executed
}

// UndoEngine orchestrates undo operations.
// It processes events in reverse chronological order and verifies file identity
// before each undo operation.
// Requirements: 5.1, 5.2, 5.7, 5.8, 6.1, 6.2, 14.1, 14.2
type UndoEngine struct {
	reader           *AuditReader
	writer           *AuditWriter
	identityResolver *IdentityResolver
	appVersion       string
	machineID        string
}

// NewUndoEngine creates a new UndoEngine with the given reader and writer.
func NewUndoEngine(reader *AuditReader, writer *AuditWriter, appVersion, machineID string) *UndoEngine {
	return &UndoEngine{
		reader:           reader,
		writer:           writer,
		identityResolver: NewIdentityResolver(),
		appVersion:       appVersion,
		machineID:        machineID,
	}
}

// UndoLatest undoes the most recent run.
// Requirements: 5.1
func (e *UndoEngine) UndoLatest(pathMappings []PathMapping) (*UndoResult, error) {
	// Find the most recent run
	latestRun, err := e.reader.GetLatestRun()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest run: %w", err)
	}

	// Don't undo UNDO runs
	if latestRun.RunType == RunTypeUndo {
		return nil, fmt.Errorf("cannot undo an UNDO run; find the original run to undo")
	}

	return e.UndoRun(latestRun.RunID, pathMappings)
}

// UndoLatestCrossMachine undoes the most recent run with cross-machine support.
// Requirements: 5.1, 7.2, 7.3, 7.5, 7.6
func (e *UndoEngine) UndoLatestCrossMachine(config CrossMachineUndoConfig) (*UndoResult, error) {
	// Find the most recent run
	latestRun, err := e.reader.GetLatestRun()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest run: %w", err)
	}

	// Don't undo UNDO runs
	if latestRun.RunType == RunTypeUndo {
		return nil, fmt.Errorf("cannot undo an UNDO run; find the original run to undo")
	}

	return e.UndoRunCrossMachine(latestRun.RunID, config)
}

// UndoRun undoes a specific run by ID.
// Requirements: 5.2, 5.7, 5.8, 6.1, 6.2, 14.1, 14.2
func (e *UndoEngine) UndoRun(runID RunID, pathMappings []PathMapping) (*UndoResult, error) {
	config := CrossMachineUndoConfig{
		PathMappings: pathMappings,
	}
	return e.UndoRunCrossMachine(runID, config)
}

// UndoRunCrossMachine undoes a specific run by ID with cross-machine support.
// It supports path mappings, hash-based file discovery, and records originating machine ID.
// Requirements: 5.2, 5.7, 5.8, 6.1, 6.2, 6.5, 6.6, 7.2, 7.3, 7.5, 7.6, 14.1, 14.2
func (e *UndoEngine) UndoRunCrossMachine(runID RunID, config CrossMachineUndoConfig) (*UndoResult, error) {
	// Validate that the run exists
	// Requirements: 6.2
	runInfo, err := e.reader.GetRunByID(runID)
	if err != nil {
		return nil, fmt.Errorf("run not found: %s", runID)
	}

	// Don't undo UNDO runs
	if runInfo.RunType == RunTypeUndo {
		return nil, fmt.Errorf("cannot undo an UNDO run")
	}

	// Get all events for the run
	events, err := e.reader.GetRun(runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get events for run %s: %w", runID, err)
	}

	// Build conflict map for older run undo
	// Requirements: 6.5, 6.6
	conflictMap, err := e.buildConflictMap(runID, runInfo.StartTime)
	if err != nil {
		return nil, fmt.Errorf("failed to build conflict map: %w", err)
	}

	// Start a new UNDO run
	// Requirements: 14.1
	undoRunID, err := e.startUndoRunCrossMachine(runID, config.OriginatingMachine)
	if err != nil {
		return nil, fmt.Errorf("failed to start undo run: %w", err)
	}

	result := &UndoResult{
		UndoRunID:   undoRunID,
		TargetRunID: runID,
	}

	// Sort events in reverse chronological order
	// Requirements: 5.2
	sortedEvents := e.sortEventsReverse(events)
	result.TotalEvents = len(sortedEvents)

	// Process each event
	for _, event := range sortedEvents {
		// Check for conflicts with subsequent runs before undoing
		// Requirements: 6.5, 6.6
		if conflict := e.checkConflict(event, conflictMap, config.PathMappings); conflict != nil {
			e.recordConflictDetected(event.SourcePath, event.DestinationPath, conflict.ConflictingRunID)
			result.Failed++
			result.FailureDetails = append(result.FailureDetails, UndoError{
				SourcePath: event.SourcePath,
				DestPath:   event.DestinationPath,
				Reason:     ReasonConflictWithLaterRun,
				Message:    fmt.Sprintf("file was modified by subsequent run %s", conflict.ConflictingRunID),
			})
			continue
		}

		wasNoOp, undoErr := e.undoEventCrossMachine(event, config)
		if undoErr != nil {
			result.Failed++
			result.FailureDetails = append(result.FailureDetails, *undoErr)
		} else if wasNoOp {
			result.Skipped++
		} else {
			result.Restored++
		}
	}

	// End the undo run
	summary := RunSummary{
		TotalFiles: result.TotalEvents,
		Moved:      result.Restored,
		Skipped:    result.Skipped,
		Errors:     result.Failed,
	}

	status := RunStatusCompleted
	if result.Failed > 0 && result.Restored == 0 {
		status = RunStatusFailed
	}

	if err := e.writer.EndRun(undoRunID, status, summary); err != nil {
		return result, fmt.Errorf("failed to end undo run: %w", err)
	}

	return result, nil
}

// PreviewUndo shows what would be undone without executing.
func (e *UndoEngine) PreviewUndo(runID RunID, pathMappings []PathMapping) (*UndoPreview, error) {
	config := CrossMachineUndoConfig{
		PathMappings: pathMappings,
	}
	return e.PreviewUndoCrossMachine(runID, config)
}

// PreviewUndoCrossMachine shows what would be undone without executing, with cross-machine support.
func (e *UndoEngine) PreviewUndoCrossMachine(runID RunID, config CrossMachineUndoConfig) (*UndoPreview, error) {
	// Validate that the run exists
	_, err := e.reader.GetRunByID(runID)
	if err != nil {
		return nil, fmt.Errorf("run not found: %s", runID)
	}

	// Get all events for the run
	events, err := e.reader.GetRun(runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get events for run %s: %w", runID, err)
	}

	preview := &UndoPreview{
		TargetRunID: runID,
	}

	// Sort events in reverse chronological order
	sortedEvents := e.sortEventsReverse(events)

	for _, event := range sortedEvents {
		previewEvent := UndoPreviewEvent{
			EventType:  event.EventType,
			SourcePath: e.applyPathMappings(event.SourcePath, config.PathMappings),
			DestPath:   e.applyPathMappings(event.DestinationPath, config.PathMappings),
		}

		switch event.EventType {
		case EventMove:
			previewEvent.WillRestore = true
			preview.TotalMoves++
		case EventRouteToReview:
			previewEvent.WillRestore = true
			preview.TotalReviews++
		case EventSkip, EventParseFailure, EventValidationFailure:
			previewEvent.WillRestore = false
			preview.TotalNoOps++
		default:
			// RUN_START, RUN_END, etc. are not included in preview
			continue
		}

		preview.EventsToUndo = append(preview.EventsToUndo, previewEvent)
	}

	return preview, nil
}

// startUndoRun starts a new run of type UNDO and records the target run ID.
// Requirements: 14.1, 14.2
func (e *UndoEngine) startUndoRun(targetRunID RunID) (RunID, error) {
	return e.startUndoRunCrossMachine(targetRunID, "")
}

// startUndoRunCrossMachine starts a new run of type UNDO with cross-machine support.
// It records the target run ID and optionally the originating machine ID.
// Requirements: 7.6, 14.1, 14.2
func (e *UndoEngine) startUndoRunCrossMachine(targetRunID RunID, originatingMachine string) (RunID, error) {
	// Use StartUndoRun to properly initialize the run and set currentRun
	undoRunID, err := e.writer.StartUndoRun(e.appVersion, e.machineID, targetRunID)
	if err != nil {
		return "", err
	}

	// If originating machine is specified and different from current machine,
	// record it in the run metadata
	// Requirements: 7.6
	if originatingMachine != "" && originatingMachine != e.machineID {
		event := AuditEvent{
			Timestamp: time.Now().UTC(),
			RunID:     undoRunID,
			EventType: EventUndoSkip, // Using a system event to record metadata
			Status:    StatusSuccess,
			Metadata: map[string]string{
				"originatingMachine": originatingMachine,
				"currentMachine":     e.machineID,
				"crossMachineUndo":   "true",
			},
		}
		// Write the cross-machine metadata event
		e.writer.WriteEvent(event)
	}

	return undoRunID, nil
}

// sortEventsReverse sorts events in reverse chronological order.
// Requirements: 5.2
func (e *UndoEngine) sortEventsReverse(events []AuditEvent) []AuditEvent {
	// Filter out non-file events (RUN_START, RUN_END, ROTATION, etc.)
	var fileEvents []AuditEvent
	for _, event := range events {
		if e.isFileEvent(event.EventType) {
			fileEvents = append(fileEvents, event)
		}
	}

	// Sort in reverse chronological order (newest first)
	sort.Slice(fileEvents, func(i, j int) bool {
		return fileEvents[i].Timestamp.After(fileEvents[j].Timestamp)
	})

	return fileEvents
}

// isFileEvent returns true if the event type is a file operation event.
func (e *UndoEngine) isFileEvent(eventType EventType) bool {
	switch eventType {
	case EventMove, EventRouteToReview, EventSkip, EventDuplicateDetected,
		EventParseFailure, EventValidationFailure, EventError:
		return true
	default:
		return false
	}
}

// isNoOpEvent returns true if the event type requires no action during undo.
// Requirements: 5.6
func (e *UndoEngine) isNoOpEvent(eventType EventType) bool {
	switch eventType {
	case EventSkip, EventParseFailure, EventValidationFailure, EventError:
		return true
	default:
		return false
	}
}

// undoEventWithNoOpFlag processes a single event for undo and returns whether it was a no-op.
// Returns (wasNoOp, error) where wasNoOp is true if the event required no action.
// Requirements: 5.3, 5.4, 5.5, 5.6, 5.7
func (e *UndoEngine) undoEventWithNoOpFlag(event AuditEvent, pathMappings []PathMapping) (bool, *UndoError) {
	config := CrossMachineUndoConfig{
		PathMappings: pathMappings,
	}
	return e.undoEventCrossMachine(event, config)
}

// undoEventCrossMachine processes a single event for undo with cross-machine support.
// It supports path mappings and hash-based file discovery.
// Returns (wasNoOp, error) where wasNoOp is true if the event required no action.
// Requirements: 5.3, 5.4, 5.5, 5.6, 5.7, 7.2, 7.3, 7.5
func (e *UndoEngine) undoEventCrossMachine(event AuditEvent, config CrossMachineUndoConfig) (bool, *UndoError) {
	switch event.EventType {
	case EventMove:
		return false, e.undoMoveCrossMachine(event, config)
	case EventRouteToReview:
		return false, e.undoRouteToReviewCrossMachine(event, config)
	case EventDuplicateDetected:
		return e.undoDuplicateCrossMachine(event, config)
	case EventSkip, EventParseFailure, EventValidationFailure:
		// No-op events - record UNDO_SKIP
		// Requirements: 5.6
		e.recordUndoSkip(event.SourcePath, ReasonNoOpEvent)
		return true, nil
	case EventError:
		// Error events are also no-op for undo
		e.recordUndoSkip(event.SourcePath, ReasonNoOpEvent)
		return true, nil
	default:
		// Unknown event type - skip
		return true, nil
	}
}

// undoEvent processes a single event for undo.
// Requirements: 5.3, 5.4, 5.5, 5.6, 5.7
func (e *UndoEngine) undoEvent(event AuditEvent, pathMappings []PathMapping) *UndoError {
	_, err := e.undoEventWithNoOpFlag(event, pathMappings)
	return err
}

// undoMove undoes a MOVE event by moving the file back to its original location.
// Requirements: 5.3, 5.7, 13.1, 13.2, 13.3, 13.4, 13.5
func (e *UndoEngine) undoMove(event AuditEvent, pathMappings []PathMapping) *UndoError {
	config := CrossMachineUndoConfig{
		PathMappings: pathMappings,
	}
	return e.undoMoveCrossMachine(event, config)
}

// undoMoveCrossMachine undoes a MOVE event with cross-machine support.
// It uses content hash as primary identity and searches configured directories
// when the file is not at the expected path.
// Requirements: 5.3, 5.7, 7.3, 7.4, 7.5, 13.1, 13.2, 13.3, 13.4, 13.5
func (e *UndoEngine) undoMoveCrossMachine(event AuditEvent, config CrossMachineUndoConfig) *UndoError {
	sourcePath := e.applyPathMappings(event.SourcePath, config.PathMappings)
	destPath := e.applyPathMappings(event.DestinationPath, config.PathMappings)

	// Try to find the file - first at expected destination, then by hash
	// Requirements: 7.4, 7.5
	actualFilePath, findErr := e.findFileForUndo(destPath, event.FileIdentity, config.SearchDirectories)
	if findErr != nil {
		e.recordSourceMissing(sourcePath, destPath)
		return &UndoError{
			SourcePath: sourcePath,
			DestPath:   destPath,
			Reason:     ReasonSourceNotFound,
			Message:    findErr.Message,
		}
	}

	// If file was found at a different path, log the discrepancy
	// Requirements: 7.4
	if actualFilePath != destPath {
		e.recordPathDiscrepancy(sourcePath, destPath, actualFilePath)
	}

	// Verify file identity before undo
	// Requirements: 5.7, 13.4
	if event.FileIdentity != nil {
		match, err := e.identityResolver.VerifyIdentity(actualFilePath, *event.FileIdentity)
		if err != nil {
			e.recordIdentityMismatch(sourcePath, actualFilePath, fmt.Sprintf("identity verification error: %v", err))
			return &UndoError{
				SourcePath: sourcePath,
				DestPath:   actualFilePath,
				Reason:     ReasonIdentityMismatch,
				Message:    fmt.Sprintf("identity verification error: %v", err),
			}
		}

		switch match {
		case IdentityNotFound:
			e.recordSourceMissing(sourcePath, actualFilePath)
			return &UndoError{
				SourcePath: sourcePath,
				DestPath:   actualFilePath,
				Reason:     ReasonSourceNotFound,
				Message:    "file not found at destination",
			}
		case IdentityHashMismatch:
			// File exists but content has changed - use CONTENT_CHANGED event
			// Requirements: 13.4
			e.recordContentChanged(sourcePath, actualFilePath, "file content has changed since original operation")
			return &UndoError{
				SourcePath: sourcePath,
				DestPath:   actualFilePath,
				Reason:     ReasonIdentityMismatch,
				Message:    "file content has changed since original operation",
			}
		case IdentitySizeMismatch:
			// File exists but size differs - also indicates content change
			e.recordContentChanged(sourcePath, actualFilePath, "file size has changed since original operation")
			return &UndoError{
				SourcePath: sourcePath,
				DestPath:   actualFilePath,
				Reason:     ReasonIdentityMismatch,
				Message:    "file size has changed since original operation",
			}
		}
	}

	// Check if destination (original source) already has a file
	// Requirements: 13.1, 13.2
	if _, err := os.Stat(sourcePath); err == nil {
		e.recordCollision(sourcePath, actualFilePath)
		return &UndoError{
			SourcePath: sourcePath,
			DestPath:   actualFilePath,
			Reason:     ReasonDestinationOccupied,
			Message:    "original location already has a file",
		}
	}

	// Ensure the source directory exists
	sourceDir := filepath.Dir(sourcePath)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		e.recordUndoError(sourcePath, actualFilePath, err)
		return &UndoError{
			SourcePath: sourcePath,
			DestPath:   actualFilePath,
			Reason:     ReasonSourceNotFound,
			Message:    fmt.Sprintf("failed to create source directory: %v", err),
		}
	}

	// Perform the undo move
	if err := os.Rename(actualFilePath, sourcePath); err != nil {
		e.recordUndoError(sourcePath, actualFilePath, err)
		return &UndoError{
			SourcePath: sourcePath,
			DestPath:   actualFilePath,
			Reason:     ReasonSourceNotFound,
			Message:    fmt.Sprintf("failed to move file: %v", err),
		}
	}

	// Record successful undo
	e.recordUndoMove(sourcePath, actualFilePath, event.FileIdentity)
	return nil
}

// findFileForUndo attempts to locate a file for undo operations.
// It first checks the expected path, then searches by content hash if configured.
// Requirements: 7.4, 7.5
func (e *UndoEngine) findFileForUndo(expectedPath string, identity *FileIdentity, searchDirs []string) (string, *UndoError) {
	// First, check if file exists at expected path
	if _, err := os.Stat(expectedPath); err == nil {
		return expectedPath, nil
	}

	// If no identity or no search directories, we can't search by hash
	if identity == nil || len(searchDirs) == 0 {
		return "", &UndoError{
			DestPath: expectedPath,
			Reason:   ReasonSourceNotFound,
			Message:  "file not found at expected destination",
		}
	}

	// Search by content hash in configured directories
	// Requirements: 7.5
	matches, err := e.identityResolver.FindByHash(identity.ContentHash, searchDirs)
	if err != nil {
		return "", &UndoError{
			DestPath: expectedPath,
			Reason:   ReasonSourceNotFound,
			Message:  fmt.Sprintf("error searching for file by hash: %v", err),
		}
	}

	if len(matches) == 0 {
		return "", &UndoError{
			DestPath: expectedPath,
			Reason:   ReasonSourceNotFound,
			Message:  "file not found at expected destination or by content hash",
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}

	// Multiple matches - try to disambiguate using size and mod time
	// Requirements: 4.9
	for _, match := range matches {
		matchIdentity, err := e.identityResolver.CaptureIdentity(match)
		if err != nil {
			continue
		}
		if matchIdentity.Size == identity.Size {
			// Found a match with same size - use it
			return match, nil
		}
	}

	// Still ambiguous - record and fail
	return "", &UndoError{
		DestPath: expectedPath,
		Reason:   ReasonIdentityMismatch,
		Message:  fmt.Sprintf("ambiguous identity: found %d files with matching hash", len(matches)),
	}
}

// recordPathDiscrepancy records when a file was found at a different path than expected.
// Requirements: 7.4
func (e *UndoEngine) recordPathDiscrepancy(sourcePath, expectedPath, actualPath string) {
	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           *e.writer.CurrentRunID(),
		EventType:       EventUndoMove,
		Status:          StatusSuccess,
		SourcePath:      sourcePath,
		DestinationPath: actualPath,
		Metadata: map[string]string{
			"expectedPath":    expectedPath,
			"actualPath":      actualPath,
			"pathDiscrepancy": "true",
		},
	}
	e.writer.WriteEvent(event)
}

// undoRouteToReview undoes a ROUTE_TO_REVIEW event.
// Requirements: 5.4
func (e *UndoEngine) undoRouteToReview(event AuditEvent, pathMappings []PathMapping) *UndoError {
	config := CrossMachineUndoConfig{
		PathMappings: pathMappings,
	}
	return e.undoRouteToReviewCrossMachine(event, config)
}

// undoRouteToReviewCrossMachine undoes a ROUTE_TO_REVIEW event with cross-machine support.
// Requirements: 5.4, 7.3, 7.5
func (e *UndoEngine) undoRouteToReviewCrossMachine(event AuditEvent, config CrossMachineUndoConfig) *UndoError {
	sourcePath := e.applyPathMappings(event.SourcePath, config.PathMappings)
	destPath := e.applyPathMappings(event.DestinationPath, config.PathMappings)

	// Check if file exists at review location
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		// Try to find by hash if search directories are configured
		if event.FileIdentity != nil && len(config.SearchDirectories) > 0 {
			matches, findErr := e.identityResolver.FindByHash(event.FileIdentity.ContentHash, config.SearchDirectories)
			if findErr == nil && len(matches) == 1 {
				destPath = matches[0]
			} else {
				e.recordSourceMissing(sourcePath, destPath)
				return &UndoError{
					SourcePath: sourcePath,
					DestPath:   destPath,
					Reason:     ReasonSourceNotFound,
					Message:    "file not found in review directory",
				}
			}
		} else {
			e.recordSourceMissing(sourcePath, destPath)
			return &UndoError{
				SourcePath: sourcePath,
				DestPath:   destPath,
				Reason:     ReasonSourceNotFound,
				Message:    "file not found in review directory",
			}
		}
	}

	// Check if destination (original source) already has a file
	if _, err := os.Stat(sourcePath); err == nil {
		e.recordCollision(sourcePath, destPath)
		return &UndoError{
			SourcePath: sourcePath,
			DestPath:   destPath,
			Reason:     ReasonDestinationOccupied,
			Message:    "original location already has a file",
		}
	}

	// Ensure the source directory exists
	sourceDir := filepath.Dir(sourcePath)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		e.recordUndoError(sourcePath, destPath, err)
		return &UndoError{
			SourcePath: sourcePath,
			DestPath:   destPath,
			Reason:     ReasonSourceNotFound,
			Message:    fmt.Sprintf("failed to create source directory: %v", err),
		}
	}

	// Perform the undo move
	if err := os.Rename(destPath, sourcePath); err != nil {
		e.recordUndoError(sourcePath, destPath, err)
		return &UndoError{
			SourcePath: sourcePath,
			DestPath:   destPath,
			Reason:     ReasonSourceNotFound,
			Message:    fmt.Sprintf("failed to move file: %v", err),
		}
	}

	// Record successful undo
	e.recordUndoMove(sourcePath, destPath, nil)
	return nil
}

// undoDuplicate undoes a DUPLICATE_DETECTED event.
// Requirements: 5.5
func (e *UndoEngine) undoDuplicate(event AuditEvent, pathMappings []PathMapping) *UndoError {
	config := CrossMachineUndoConfig{
		PathMappings: pathMappings,
	}
	_, err := e.undoDuplicateCrossMachine(event, config)
	return err
}

// undoDuplicateWithNoOpFlag undoes a DUPLICATE_DETECTED event and returns whether it was a no-op.
// Requirements: 5.5
func (e *UndoEngine) undoDuplicateWithNoOpFlag(event AuditEvent, pathMappings []PathMapping) (bool, *UndoError) {
	config := CrossMachineUndoConfig{
		PathMappings: pathMappings,
	}
	return e.undoDuplicateCrossMachine(event, config)
}

// undoDuplicateCrossMachine undoes a DUPLICATE_DETECTED event with cross-machine support.
// Requirements: 5.5, 7.3, 7.5
func (e *UndoEngine) undoDuplicateCrossMachine(event AuditEvent, config CrossMachineUndoConfig) (bool, *UndoError) {
	// For duplicates that were renamed, we need to restore the original filename
	if event.ReasonCode != ReasonDuplicateRenamed {
		// If not renamed, it's a no-op
		e.recordUndoSkip(event.SourcePath, ReasonNoOpEvent)
		return true, nil
	}

	sourcePath := e.applyPathMappings(event.SourcePath, config.PathMappings)
	actualDest := e.applyPathMappings(event.DestinationPath, config.PathMappings)

	// Get the intended destination from metadata
	intendedDest := ""
	if event.Metadata != nil {
		intendedDest = e.applyPathMappings(event.Metadata["intendedDestination"], config.PathMappings)
	}

	if intendedDest == "" {
		// Can't undo without knowing the intended destination
		e.recordUndoSkip(sourcePath, ReasonNoOpEvent)
		return true, nil
	}

	// Check if file exists at actual destination
	if _, err := os.Stat(actualDest); os.IsNotExist(err) {
		// Try to find by hash if search directories are configured
		if event.FileIdentity != nil && len(config.SearchDirectories) > 0 {
			matches, findErr := e.identityResolver.FindByHash(event.FileIdentity.ContentHash, config.SearchDirectories)
			if findErr == nil && len(matches) == 1 {
				actualDest = matches[0]
			} else {
				e.recordSourceMissing(sourcePath, actualDest)
				return false, &UndoError{
					SourcePath: sourcePath,
					DestPath:   actualDest,
					Reason:     ReasonSourceNotFound,
					Message:    "renamed file not found",
				}
			}
		} else {
			e.recordSourceMissing(sourcePath, actualDest)
			return false, &UndoError{
				SourcePath: sourcePath,
				DestPath:   actualDest,
				Reason:     ReasonSourceNotFound,
				Message:    "renamed file not found",
			}
		}
	}

	// Move file back to original source
	if _, err := os.Stat(sourcePath); err == nil {
		e.recordCollision(sourcePath, actualDest)
		return false, &UndoError{
			SourcePath: sourcePath,
			DestPath:   actualDest,
			Reason:     ReasonDestinationOccupied,
			Message:    "original location already has a file",
		}
	}

	// Ensure the source directory exists
	sourceDir := filepath.Dir(sourcePath)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		e.recordUndoError(sourcePath, actualDest, err)
		return false, &UndoError{
			SourcePath: sourcePath,
			DestPath:   actualDest,
			Reason:     ReasonSourceNotFound,
			Message:    fmt.Sprintf("failed to create source directory: %v", err),
		}
	}

	if err := os.Rename(actualDest, sourcePath); err != nil {
		e.recordUndoError(sourcePath, actualDest, err)
		return false, &UndoError{
			SourcePath: sourcePath,
			DestPath:   actualDest,
			Reason:     ReasonSourceNotFound,
			Message:    fmt.Sprintf("failed to restore file: %v", err),
		}
	}

	e.recordUndoMove(sourcePath, actualDest, nil)
	return false, nil
}

// applyPathMappings applies path mappings to translate paths between machines.
// Requirements: 7.2, 7.3
func (e *UndoEngine) applyPathMappings(path string, mappings []PathMapping) string {
	if path == "" {
		return path
	}

	for _, mapping := range mappings {
		if len(path) >= len(mapping.OriginalPrefix) &&
			path[:len(mapping.OriginalPrefix)] == mapping.OriginalPrefix {
			return mapping.MappedPrefix + path[len(mapping.OriginalPrefix):]
		}
	}

	return path
}

// recordUndoMove records an UNDO_MOVE event.
// Requirements: 14.3
func (e *UndoEngine) recordUndoMove(sourcePath, destPath string, identity *FileIdentity) {
	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           *e.writer.CurrentRunID(),
		EventType:       EventUndoMove,
		Status:          StatusSuccess,
		SourcePath:      destPath,   // Where file was (before undo)
		DestinationPath: sourcePath, // Where file is now (after undo)
		FileIdentity:    identity,
	}
	e.writer.WriteEvent(event)
}

// recordUndoSkip records an UNDO_SKIP event.
// Requirements: 14.4
func (e *UndoEngine) recordUndoSkip(sourcePath string, reason ReasonCode) {
	event := AuditEvent{
		Timestamp:  time.Now().UTC(),
		RunID:      *e.writer.CurrentRunID(),
		EventType:  EventUndoSkip,
		Status:     StatusSkipped,
		SourcePath: sourcePath,
		ReasonCode: reason,
	}
	e.writer.WriteEvent(event)
}

// recordIdentityMismatch records an IDENTITY_MISMATCH event.
func (e *UndoEngine) recordIdentityMismatch(sourcePath, destPath, message string) {
	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           *e.writer.CurrentRunID(),
		EventType:       EventIdentityMismatch,
		Status:          StatusFailure,
		SourcePath:      sourcePath,
		DestinationPath: destPath,
		ErrorDetails: &ErrorDetails{
			ErrorType:    "IDENTITY_MISMATCH",
			ErrorMessage: message,
			Operation:    "undo_verify",
		},
	}
	e.writer.WriteEvent(event)
}

// recordContentChanged records a CONTENT_CHANGED event when file content has changed.
// Requirements: 13.4
func (e *UndoEngine) recordContentChanged(sourcePath, destPath, message string) {
	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           *e.writer.CurrentRunID(),
		EventType:       EventContentChanged,
		Status:          StatusFailure,
		SourcePath:      sourcePath,
		DestinationPath: destPath,
		ReasonCode:      ReasonIdentityMismatch,
		ErrorDetails: &ErrorDetails{
			ErrorType:    "CONTENT_CHANGED",
			ErrorMessage: message,
			Operation:    "undo_verify",
		},
	}
	e.writer.WriteEvent(event)
}

// recordSourceMissing records a SOURCE_MISSING event.
func (e *UndoEngine) recordSourceMissing(sourcePath, destPath string) {
	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           *e.writer.CurrentRunID(),
		EventType:       EventSourceMissing,
		Status:          StatusFailure,
		SourcePath:      sourcePath,
		DestinationPath: destPath,
		ReasonCode:      ReasonSourceNotFound,
	}
	e.writer.WriteEvent(event)
}

// recordCollision records a COLLISION event.
func (e *UndoEngine) recordCollision(sourcePath, destPath string) {
	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           *e.writer.CurrentRunID(),
		EventType:       EventCollision,
		Status:          StatusFailure,
		SourcePath:      sourcePath,
		DestinationPath: destPath,
		ReasonCode:      ReasonDestinationOccupied,
	}
	e.writer.WriteEvent(event)
}

// recordUndoError records an ERROR event during undo.
func (e *UndoEngine) recordUndoError(sourcePath, destPath string, err error) {
	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           *e.writer.CurrentRunID(),
		EventType:       EventError,
		Status:          StatusFailure,
		SourcePath:      sourcePath,
		DestinationPath: destPath,
		ErrorDetails: &ErrorDetails{
			ErrorType:    "UNDO_ERROR",
			ErrorMessage: err.Error(),
			Operation:    "undo_move",
		},
	}
	e.writer.WriteEvent(event)
}

// ConflictInfo contains information about a conflict with a subsequent run.
type ConflictInfo struct {
	ConflictingRunID RunID     // The run that modified the file
	ConflictingPath  string    // The path that was modified
	EventType        EventType // The type of event that caused the conflict
}

// buildConflictMap builds a map of files that were modified by runs after the target run.
// This is used to detect conflicts when undoing an older run.
// Requirements: 6.5, 6.6
func (e *UndoEngine) buildConflictMap(targetRunID RunID, targetRunStartTime time.Time) (map[string]*ConflictInfo, error) {
	conflictMap := make(map[string]*ConflictInfo)

	// Get all runs
	runs, err := e.reader.ListRuns()
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}

	// Find the index of the target run in the list
	// Runs are sorted by start time (oldest first), so runs after the target index are subsequent
	targetIndex := -1
	for i, run := range runs {
		if run.RunID == targetRunID {
			targetIndex = i
			break
		}
	}

	if targetIndex == -1 {
		// Target run not found - no conflicts possible
		return conflictMap, nil
	}

	// Find runs that are subsequent to the target run
	// A run is subsequent if:
	// 1. It started after the target run (by timestamp), OR
	// 2. It has the same start time but appears later in the list (by log order)
	for i, run := range runs {
		// Skip the target run itself
		if run.RunID == targetRunID {
			continue
		}

		// Skip runs that started before the target run
		if run.StartTime.Before(targetRunStartTime) {
			continue
		}

		// If timestamps are equal, use list order as tiebreaker
		// Runs appearing later in the list are considered subsequent
		if run.StartTime.Equal(targetRunStartTime) && i <= targetIndex {
			continue
		}

		// Skip UNDO runs - they don't create conflicts
		if run.RunType == RunTypeUndo {
			continue
		}

		// Get events for this subsequent run
		events, err := e.reader.GetRun(run.RunID)
		if err != nil {
			// Log but continue - we don't want to fail the entire undo
			continue
		}

		// Check each event for file modifications
		for _, event := range events {
			// Only file operation events can create conflicts
			if !e.isFileModificationEvent(event.EventType) {
				continue
			}

			// Track both source and destination paths as potentially conflicting
			if event.SourcePath != "" {
				conflictMap[event.SourcePath] = &ConflictInfo{
					ConflictingRunID: run.RunID,
					ConflictingPath:  event.SourcePath,
					EventType:        event.EventType,
				}
			}
			if event.DestinationPath != "" {
				conflictMap[event.DestinationPath] = &ConflictInfo{
					ConflictingRunID: run.RunID,
					ConflictingPath:  event.DestinationPath,
					EventType:        event.EventType,
				}
			}
		}
	}

	return conflictMap, nil
}

// isFileModificationEvent returns true if the event type represents a file modification.
// These are events that could create conflicts when undoing an older run.
func (e *UndoEngine) isFileModificationEvent(eventType EventType) bool {
	switch eventType {
	case EventMove, EventRouteToReview, EventDuplicateDetected:
		return true
	default:
		return false
	}
}

// checkConflict checks if an event conflicts with subsequent runs.
// Returns ConflictInfo if a conflict is found, nil otherwise.
// Requirements: 6.5, 6.6
func (e *UndoEngine) checkConflict(event AuditEvent, conflictMap map[string]*ConflictInfo, pathMappings []PathMapping) *ConflictInfo {
	// Only check conflicts for file modification events
	if !e.isFileModificationEvent(event.EventType) {
		return nil
	}

	// Apply path mappings to get the actual paths we'll be working with
	sourcePath := e.applyPathMappings(event.SourcePath, pathMappings)
	destPath := e.applyPathMappings(event.DestinationPath, pathMappings)

	// Check if the destination path (where the file currently is) was modified by a later run
	// This is the primary conflict check - if a later run moved this file somewhere else,
	// we can't undo the original move
	if destPath != "" {
		if conflict, exists := conflictMap[destPath]; exists {
			return conflict
		}
	}

	// Also check the original source path - if a later run put a different file there,
	// we might have a conflict
	if sourcePath != "" {
		if conflict, exists := conflictMap[sourcePath]; exists {
			return conflict
		}
	}

	// Check unmapped paths as well (for cross-machine scenarios)
	if event.DestinationPath != "" && event.DestinationPath != destPath {
		if conflict, exists := conflictMap[event.DestinationPath]; exists {
			return conflict
		}
	}
	if event.SourcePath != "" && event.SourcePath != sourcePath {
		if conflict, exists := conflictMap[event.SourcePath]; exists {
			return conflict
		}
	}

	return nil
}

// recordConflictDetected records a CONFLICT_DETECTED event when a file was modified by a subsequent run.
// Requirements: 6.5, 6.6
func (e *UndoEngine) recordConflictDetected(sourcePath, destPath string, conflictingRunID RunID) {
	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           *e.writer.CurrentRunID(),
		EventType:       EventConflictDetected,
		Status:          StatusFailure,
		SourcePath:      sourcePath,
		DestinationPath: destPath,
		ReasonCode:      ReasonConflictWithLaterRun,
		Metadata: map[string]string{
			"conflictingRunId": string(conflictingRunID),
		},
	}
	e.writer.WriteEvent(event)
}
