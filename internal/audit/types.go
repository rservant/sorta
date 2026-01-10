// Package audit provides audit trail functionality for Sorta file operations.
// It implements an append-only event log that enables complete traceability,
// reversibility, and cross-machine portability for all file operations.
package audit

import "time"

// RunID is a unique identifier for each program execution.
// It uses UUID v4 format: "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx"
type RunID string

// EventType represents the type of audit event.
type EventType string

const (
	// Run lifecycle events
	EventRunStart EventType = "RUN_START"
	EventRunEnd   EventType = "RUN_END"

	// File operation events
	EventMove              EventType = "MOVE"
	EventRouteToReview     EventType = "ROUTE_TO_REVIEW"
	EventSkip              EventType = "SKIP"
	EventDuplicateDetected EventType = "DUPLICATE_DETECTED"
	EventParseFailure      EventType = "PARSE_FAILURE"
	EventValidationFailure EventType = "VALIDATION_FAILURE"
	EventError             EventType = "ERROR"

	// Undo events
	EventUndoMove          EventType = "UNDO_MOVE"
	EventUndoSkip          EventType = "UNDO_SKIP"
	EventIdentityMismatch  EventType = "IDENTITY_MISMATCH"
	EventAmbiguousIdentity EventType = "AMBIGUOUS_IDENTITY"
	EventCollision         EventType = "COLLISION"
	EventSourceMissing     EventType = "SOURCE_MISSING"
	EventContentChanged    EventType = "CONTENT_CHANGED"
	EventConflictDetected  EventType = "CONFLICT_DETECTED"

	// System events
	EventRotation       EventType = "ROTATION"
	EventRetentionPrune EventType = "RETENTION_PRUNE"
	EventLogInitialized EventType = "LOG_INITIALIZED"
)

// OperationStatus represents the outcome of an operation.
type OperationStatus string

const (
	StatusSuccess OperationStatus = "SUCCESS"
	StatusFailure OperationStatus = "FAILURE"
	StatusSkipped OperationStatus = "SKIPPED"
)

// ReasonCode provides detailed reason for skip or review routing.
type ReasonCode string

const (
	// Skip reasons
	ReasonNoMatch          ReasonCode = "NO_MATCH"
	ReasonInvalidDate      ReasonCode = "INVALID_DATE"
	ReasonAlreadyProcessed ReasonCode = "ALREADY_PROCESSED"

	// Review routing reasons
	ReasonUnclassified    ReasonCode = "UNCLASSIFIED"
	ReasonParseError      ReasonCode = "PARSE_ERROR"
	ReasonValidationError ReasonCode = "VALIDATION_ERROR"

	// Duplicate reasons
	ReasonDuplicateRenamed ReasonCode = "DUPLICATE_RENAMED"

	// Undo skip reasons
	ReasonNoOpEvent            ReasonCode = "NO_OP_EVENT"
	ReasonIdentityMismatch     ReasonCode = "IDENTITY_MISMATCH"
	ReasonDestinationOccupied  ReasonCode = "DESTINATION_OCCUPIED"
	ReasonSourceNotFound       ReasonCode = "SOURCE_NOT_FOUND"
	ReasonConflictWithLaterRun ReasonCode = "CONFLICT_WITH_LATER_RUN"
)

// RunStatus represents the status of a run.
type RunStatus string

const (
	RunStatusInProgress  RunStatus = "IN_PROGRESS"
	RunStatusCompleted   RunStatus = "COMPLETED"
	RunStatusFailed      RunStatus = "FAILED"
	RunStatusInterrupted RunStatus = "INTERRUPTED"
)

// RunType represents the type of run.
type RunType string

const (
	RunTypeOrganize RunType = "ORGANIZE"
	RunTypeUndo     RunType = "UNDO"
)

// FileIdentity captures the attributes used to uniquely identify a file across machines.
type FileIdentity struct {
	ContentHash string    `json:"contentHash"` // SHA-256 hex string
	Size        int64     `json:"size"`        // File size in bytes
	ModTime     time.Time `json:"modTime"`     // File modification timestamp
}

// ErrorDetails contains detailed information about an error.
type ErrorDetails struct {
	ErrorType    string `json:"errorType"`
	ErrorMessage string `json:"errorMessage"`
	Operation    string `json:"operation"`
}

// AuditEvent represents a single audit record for a file operation or system event.
type AuditEvent struct {
	Timestamp       time.Time         `json:"timestamp"`                 // ISO 8601 format
	RunID           RunID             `json:"runId"`                     // Run identifier
	EventType       EventType         `json:"eventType"`                 // Type of event
	Status          OperationStatus   `json:"status"`                    // Operation outcome
	SourcePath      string            `json:"sourcePath,omitempty"`      // Original file path
	DestinationPath string            `json:"destinationPath,omitempty"` // Target file path
	ReasonCode      ReasonCode        `json:"reasonCode,omitempty"`      // Reason for skip/review
	FileIdentity    *FileIdentity     `json:"fileIdentity,omitempty"`    // File identity for moves
	ErrorDetails    *ErrorDetails     `json:"errorDetails,omitempty"`    // Error information
	Metadata        map[string]string `json:"metadata,omitempty"`        // Additional metadata
}

// RunSummary contains statistics for a completed run.
type RunSummary struct {
	TotalFiles   int `json:"totalFiles"`
	Moved        int `json:"moved"`
	Skipped      int `json:"skipped"`
	RoutedReview int `json:"routedReview"`
	Duplicates   int `json:"duplicates"`
	Errors       int `json:"errors"`
}

// RunInfo contains metadata and summary for a run.
type RunInfo struct {
	RunID        RunID      `json:"runId"`
	StartTime    time.Time  `json:"startTime"`
	EndTime      *time.Time `json:"endTime,omitempty"`
	Status       RunStatus  `json:"status"`
	RunType      RunType    `json:"runType"`
	AppVersion   string     `json:"appVersion"`
	MachineID    string     `json:"machineId"`
	Summary      RunSummary `json:"summary"`
	UndoTargetID *RunID     `json:"undoTargetId,omitempty"` // For UNDO runs
}

// PathMapping defines a path translation for cross-machine undo.
type PathMapping struct {
	OriginalPrefix string `json:"originalPrefix"` // e.g., "/Users/alice/Documents"
	MappedPrefix   string `json:"mappedPrefix"`   // e.g., "/home/bob/Documents"
}

// AuditConfig holds configuration for the audit system.
type AuditConfig struct {
	LogDirectory     string `json:"logDirectory"`
	RotationSize     int64  `json:"rotationSizeBytes"` // Rotate when file exceeds this size
	RotationPeriod   string `json:"rotationPeriod"`    // "daily", "weekly", or ""
	RetentionDays    int    `json:"retentionDays"`     // 0 = unlimited
	RetentionRuns    int    `json:"retentionRuns"`     // 0 = unlimited
	MinRetentionDays int    `json:"minRetentionDays"`  // Default: 7
}

// DefaultAuditConfig returns an AuditConfig with sensible defaults.
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		LogDirectory:     ".sorta/audit",
		RotationSize:     10 * 1024 * 1024, // 10MB
		RotationPeriod:   "",               // No time-based rotation by default
		RetentionDays:    30,
		RetentionRuns:    0, // Unlimited
		MinRetentionDays: 7,
	}
}
