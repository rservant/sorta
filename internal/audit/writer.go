// Package audit provides audit trail functionality for Sorta file operations.
package audit

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditWriter handles all write operations to the audit log.
// It implements append-only semantics with fail-fast behavior.
type AuditWriter struct {
	mu              sync.Mutex
	file            *os.File
	writer          *bufio.Writer
	logPath         string
	currentRun      *RunID
	config          AuditConfig
	rotationManager *RotationManager
}

// NewAuditWriter creates a new AuditWriter with the given configuration.
// It creates the log directory if it doesn't exist and opens the log file for appending.
// If the log file is missing, it creates a new one and writes a LOG_INITIALIZED event.
// Requirements: 8.1, 11.5, 12.1
func NewAuditWriter(config AuditConfig) (*AuditWriter, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(config.LogDirectory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(config.LogDirectory, "sorta-audit.jsonl")

	// Check if this is a new log file
	isNewLog := false
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		isNewLog = true
	}

	// Open file for appending (create if doesn't exist)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}

	writer := &AuditWriter{
		file:            file,
		writer:          bufio.NewWriter(file),
		logPath:         logPath,
		config:          config,
		rotationManager: NewRotationManager(config),
	}

	// Write LOG_INITIALIZED event for new logs
	// Requirements: 12.1
	if isNewLog {
		if err := writer.writeLogInitialized(); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to write LOG_INITIALIZED event: %w", err)
		}
	}

	return writer, nil
}

// GenerateRunID generates a new UUID v4 format Run ID.
// Requirements: 1.1, 1.2
func GenerateRunID() (RunID, error) {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}

	// Set version (4) and variant (RFC 4122)
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant RFC 4122

	return RunID(fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	)), nil
}

// StartRun initializes a new run and writes the RUN_START event.
// It generates a unique Run ID and records the run start timestamp.
// Requirements: 1.1, 1.4, 11.4
func (w *AuditWriter) StartRun(appVersion string, machineID string) (RunID, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Generate unique Run ID
	runID, err := GenerateRunID()
	if err != nil {
		return "", fmt.Errorf("failed to generate run ID: %w", err)
	}

	// Create RUN_START event
	event := AuditEvent{
		Timestamp: time.Now().UTC(),
		RunID:     runID,
		EventType: EventRunStart,
		Status:    StatusSuccess,
		Metadata: map[string]string{
			"appVersion": appVersion,
			"machineId":  machineID,
		},
	}

	// Write the event (fail-fast on error)
	if err := w.writeEventLocked(event); err != nil {
		return "", fmt.Errorf("failed to write RUN_START event: %w", err)
	}

	w.currentRun = &runID
	return runID, nil
}

// StartUndoRun initializes a new UNDO run and writes the RUN_START event.
// It generates a unique Run ID and records the target run being undone.
// Requirements: 14.1, 14.2
func (w *AuditWriter) StartUndoRun(appVersion string, machineID string, targetRunID RunID) (RunID, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Generate unique Run ID
	runID, err := GenerateRunID()
	if err != nil {
		return "", fmt.Errorf("failed to generate run ID: %w", err)
	}

	// Create RUN_START event with UNDO type and target run ID
	event := AuditEvent{
		Timestamp: time.Now().UTC(),
		RunID:     runID,
		EventType: EventRunStart,
		Status:    StatusSuccess,
		Metadata: map[string]string{
			"appVersion":   appVersion,
			"machineId":    machineID,
			"runType":      string(RunTypeUndo),
			"undoTargetId": string(targetRunID),
		},
	}

	// Write the event (fail-fast on error)
	if err := w.writeEventLocked(event); err != nil {
		return "", fmt.Errorf("failed to write RUN_START event: %w", err)
	}

	w.currentRun = &runID
	return runID, nil
}

// WriteEvent writes a single audit event to the log.
// It fails fast if the write cannot be completed.
// Requirements: 8.1, 8.4, 11.1, 11.4
func (w *AuditWriter) WriteEvent(event AuditEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.writeEventLocked(event)
}

// writeEventLocked writes an event while holding the lock.
// It marshals the event to JSON, appends a newline, and flushes to disk.
// It also checks for rotation needs after writing.
func (w *AuditWriter) writeEventLocked(event AuditEvent) error {
	// Marshal event to JSON
	data, err := event.MarshalJSONLine()
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Write JSON line with newline
	if _, err := w.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}
	if _, err := w.writer.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Flush to ensure durability (Requirements: 8.4)
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush event: %w", err)
	}

	// Sync to disk for durability
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync event to disk: %w", err)
	}

	// Check if rotation is needed after writing (not for ROTATION events to avoid infinite loop)
	if event.EventType != EventRotation {
		if err := w.checkAndRotate(); err != nil {
			return fmt.Errorf("failed to check/perform rotation: %w", err)
		}
	}

	return nil
}

// checkAndRotate checks if rotation is needed and performs it if so.
// Requirements: 9.1, 9.2, 9.5, 9.6
func (w *AuditWriter) checkAndRotate() error {
	needsRotation, err := w.rotationManager.NeedsRotation(w.logPath)
	if err != nil {
		return err
	}

	if !needsRotation {
		return nil
	}

	// Generate the rotated filename once to ensure consistency
	rotatedFilename := w.rotationManager.GenerateRotatedFilename()

	// Write ROTATION event before switching files
	// Requirements: 9.6
	var runID RunID
	if w.currentRun != nil {
		runID = *w.currentRun
	}
	rotationEvent := CreateRotationEvent(runID, filepath.Base(w.logPath), rotatedFilename)

	// Marshal and write the rotation event directly (without recursion)
	data, err := rotationEvent.MarshalJSONLine()
	if err != nil {
		return fmt.Errorf("failed to marshal rotation event: %w", err)
	}
	if _, err := w.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write rotation event: %w", err)
	}
	if _, err := w.writer.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write rotation event newline: %w", err)
	}
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush rotation event: %w", err)
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync rotation event: %w", err)
	}

	// Close current file
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close file for rotation: %w", err)
	}

	// Perform rotation with the pre-generated filename
	_, err = w.rotationManager.RotateWithFilename(w.logPath, rotatedFilename)
	if err != nil {
		return fmt.Errorf("failed to rotate log: %w", err)
	}

	// Open new log file
	file, err := os.OpenFile(w.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open new log file after rotation: %w", err)
	}

	w.file = file
	w.writer = bufio.NewWriter(file)

	return nil
}

// EndRun records the run completion status and summary.
// Requirements: 1.5
func (w *AuditWriter) EndRun(runID RunID, status RunStatus, summary RunSummary) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Create RUN_END event
	event := AuditEvent{
		Timestamp: time.Now().UTC(),
		RunID:     runID,
		EventType: EventRunEnd,
		Status:    runStatusToOperationStatus(status),
		Metadata: map[string]string{
			"status":       string(status),
			"totalFiles":   fmt.Sprintf("%d", summary.TotalFiles),
			"moved":        fmt.Sprintf("%d", summary.Moved),
			"skipped":      fmt.Sprintf("%d", summary.Skipped),
			"routedReview": fmt.Sprintf("%d", summary.RoutedReview),
			"duplicates":   fmt.Sprintf("%d", summary.Duplicates),
			"errors":       fmt.Sprintf("%d", summary.Errors),
		},
	}

	// Write the event (fail-fast on error)
	if err := w.writeEventLocked(event); err != nil {
		return fmt.Errorf("failed to write RUN_END event: %w", err)
	}

	w.currentRun = nil
	return nil
}

// runStatusToOperationStatus converts RunStatus to OperationStatus.
func runStatusToOperationStatus(status RunStatus) OperationStatus {
	switch status {
	case RunStatusCompleted:
		return StatusSuccess
	case RunStatusFailed:
		return StatusFailure
	case RunStatusInterrupted:
		return StatusFailure
	default:
		return StatusSuccess
	}
}

// Close flushes any buffered data and closes the audit log file.
func (w *AuditWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush on close: %w", err)
	}

	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close audit log: %w", err)
	}

	return nil
}

// CurrentRunID returns the current run ID, or nil if no run is active.
func (w *AuditWriter) CurrentRunID() *RunID {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentRun
}

// LogPath returns the path to the current audit log file.
func (w *AuditWriter) LogPath() string {
	return w.logPath
}

// RecordMove records a MOVE event when a file is moved to a classified destination.
// Requirements: 2.1
func (w *AuditWriter) RecordMove(source, dest string, identity *FileIdentity) error {
	if w.currentRun == nil {
		return fmt.Errorf("no active run: call StartRun first")
	}

	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           *w.currentRun,
		EventType:       EventMove,
		Status:          StatusSuccess,
		SourcePath:      source,
		DestinationPath: dest,
		FileIdentity:    identity,
	}

	return w.WriteEvent(event)
}

// RecordRouteToReview records a ROUTE_TO_REVIEW event when a file is routed to the review directory.
// Requirements: 2.2
func (w *AuditWriter) RecordRouteToReview(source, dest string, reason ReasonCode) error {
	if w.currentRun == nil {
		return fmt.Errorf("no active run: call StartRun first")
	}

	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           *w.currentRun,
		EventType:       EventRouteToReview,
		Status:          StatusSuccess,
		SourcePath:      source,
		DestinationPath: dest,
		ReasonCode:      reason,
	}

	return w.WriteEvent(event)
}

// RecordSkip records a SKIP event when a file is skipped.
// Requirements: 2.3
func (w *AuditWriter) RecordSkip(source string, reason ReasonCode) error {
	if w.currentRun == nil {
		return fmt.Errorf("no active run: call StartRun first")
	}

	event := AuditEvent{
		Timestamp:  time.Now().UTC(),
		RunID:      *w.currentRun,
		EventType:  EventSkip,
		Status:     StatusSkipped,
		SourcePath: source,
		ReasonCode: reason,
	}

	return w.WriteEvent(event)
}

// RecordDuplicate records a DUPLICATE_DETECTED event when a duplicate file is detected.
// Requirements: 2.4
func (w *AuditWriter) RecordDuplicate(source, intendedDest, actualDest string, action ReasonCode) error {
	if w.currentRun == nil {
		return fmt.Errorf("no active run: call StartRun first")
	}

	event := AuditEvent{
		Timestamp:       time.Now().UTC(),
		RunID:           *w.currentRun,
		EventType:       EventDuplicateDetected,
		Status:          StatusSuccess,
		SourcePath:      source,
		DestinationPath: actualDest,
		ReasonCode:      action,
		Metadata: map[string]string{
			"intendedDestination": intendedDest,
		},
	}

	return w.WriteEvent(event)
}

// RecordParseFailure records a PARSE_FAILURE event when date parsing fails.
// Requirements: 2.5
func (w *AuditWriter) RecordParseFailure(source, pattern, reason string) error {
	if w.currentRun == nil {
		return fmt.Errorf("no active run: call StartRun first")
	}

	event := AuditEvent{
		Timestamp:  time.Now().UTC(),
		RunID:      *w.currentRun,
		EventType:  EventParseFailure,
		Status:     StatusFailure,
		SourcePath: source,
		ReasonCode: ReasonParseError,
		Metadata: map[string]string{
			"pattern": pattern,
			"reason":  reason,
		},
	}

	return w.WriteEvent(event)
}

// RecordError records an ERROR event when an error occurs during file processing.
// Requirements: 2.7
func (w *AuditWriter) RecordError(source, errType, errMsg, operation string) error {
	if w.currentRun == nil {
		return fmt.Errorf("no active run: call StartRun first")
	}

	event := AuditEvent{
		Timestamp:  time.Now().UTC(),
		RunID:      *w.currentRun,
		EventType:  EventError,
		Status:     StatusFailure,
		SourcePath: source,
		ErrorDetails: &ErrorDetails{
			ErrorType:    errType,
			ErrorMessage: errMsg,
			Operation:    operation,
		},
	}

	return w.WriteEvent(event)
}

// writeLogInitialized writes a LOG_INITIALIZED event when a new log file is created.
// This is called internally when NewAuditWriter creates a new log file.
// Requirements: 12.1
func (w *AuditWriter) writeLogInitialized() error {
	event := AuditEvent{
		Timestamp: time.Now().UTC(),
		RunID:     "", // No run ID for system events
		EventType: EventLogInitialized,
		Status:    StatusSuccess,
		Metadata: map[string]string{
			"logPath": w.logPath,
		},
	}

	// Write directly without going through WriteEvent to avoid run ID check
	data, err := event.MarshalJSONLine()
	if err != nil {
		return fmt.Errorf("failed to marshal LOG_INITIALIZED event: %w", err)
	}

	if _, err := w.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write LOG_INITIALIZED event: %w", err)
	}
	if _, err := w.writer.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush LOG_INITIALIZED event: %w", err)
	}

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync LOG_INITIALIZED event: %w", err)
	}

	return nil
}

// CheckAndPruneRetention checks retention limits and prunes old segments if needed.
// This should be called on startup to enforce retention policies.
// Requirements: 10.1, 10.2, 10.4, 10.5
func (w *AuditWriter) CheckAndPruneRetention() (*PruneResult, error) {
	rm := NewRetentionManager(w.config)
	return rm.Prune(w)
}

// GetConfig returns the audit configuration.
func (w *AuditWriter) GetConfig() AuditConfig {
	return w.config
}
