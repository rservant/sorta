# Requirements Document

## Introduction

This specification defines the Audit Trail capability for Sorta, a local file-organizing application. The audit trail provides complete traceability of all file operations, enabling human review, confidence in system behavior, and full automatic undo—including undo across machines when the same filesystem paths are accessible.

## Glossary

- **Run**: A single execution of the Sorta application that processes files. Each run is atomic from an auditing perspective.
- **Run_ID**: A unique identifier assigned to each run, used to group all events from that execution.
- **Event**: A single audit record representing one file operation or outcome during a run.
- **Operation**: An action performed on a file (move, skip, route-to-review, duplicate-rename, etc.).
- **Source_Path**: The original filesystem path of a file before an operation.
- **Destination_Path**: The filesystem path where a file was moved or would be moved.
- **Normalized_Filename**: A filename transformed to match canonical prefix casing and format.
- **Normalized_Prefix**: The canonical form of a prefix as defined in configuration.
- **Review_Directory**: The `for-review` subdirectory where unclassified or invalid files are routed.
- **Duplicate**: A file that would overwrite an existing file at the destination; identified by matching destination path.
- **Content_Hash**: A cryptographic hash of file contents used for identity verification.
- **File_Identity**: The combination of attributes used to uniquely identify a file across machines.
- **Rotation_Segment**: A single log file within a rotated log series.
- **Audit_Log**: The append-only file(s) containing all audit events.
- **Machine_Identifier**: A stable identifier for the machine where Sorta executed.

## Requirements

### Requirement 1: Run Identification

**User Story:** As a user, I want each program execution to have a unique identifier, so that I can trace all operations from a specific run and undo them as a group.

#### Acceptance Criteria

1. WHEN Sorta begins execution, THE Audit_System SHALL generate a unique Run_ID for that run.
2. THE Run_ID SHALL be a UUID v4 or equivalent globally unique identifier.
3. WHEN any event is recorded, THE Audit_System SHALL include the Run_ID in that event record.
4. THE Audit_System SHALL record the run start timestamp as the first event of each run.
5. THE Audit_System SHALL record the run completion status as the final event of each run.

### Requirement 2: Event Recording for File Operations

**User Story:** As a user, I want every file operation logged with one record per file, so that I have complete visibility into what happened to each file.

#### Acceptance Criteria

1. WHEN a file is moved to a classified destination, THE Audit_System SHALL record a MOVE event with source path, destination path, and file identity.
2. WHEN a file is routed to the review directory, THE Audit_System SHALL record a ROUTE_TO_REVIEW event with source path, destination path, and reason code.
3. WHEN a file is skipped, THE Audit_System SHALL record a SKIP event with source path and reason code.
4. WHEN a duplicate is detected, THE Audit_System SHALL record a DUPLICATE_DETECTED event with source path, intended destination, actual destination, and duplicate resolution action.
5. WHEN a date parse failure occurs, THE Audit_System SHALL record a PARSE_FAILURE event with source path, attempted pattern, and failure reason.
6. WHEN a validation failure occurs, THE Audit_System SHALL record a VALIDATION_FAILURE event with source path and validation error details.
7. WHEN an error occurs during file processing, THE Audit_System SHALL record an ERROR event with source path, error type, and error message.
8. THE Audit_System SHALL record exactly one primary event per file processed, with no summarization-only logging.

### Requirement 3: Event Data Completeness

**User Story:** As a user, I want each audit record to contain all information needed for review and undo, so that I can understand what happened and reverse it if needed.

#### Acceptance Criteria

1. THE Audit_System SHALL include a timestamp in ISO 8601 format for every event.
2. THE Audit_System SHALL include the Run_ID for every event.
3. THE Audit_System SHALL include an event type identifier for every event.
4. THE Audit_System SHALL include an operation status (success, failure, skipped) for every event.
5. THE Audit_System SHALL include the source path for every file-related event.
6. WHEN an operation has a destination, THE Audit_System SHALL include the destination path.
7. WHEN an operation is skipped or routed to review, THE Audit_System SHALL include a reason code.
8. THE Audit_System SHALL include the application version for every run.
9. THE Audit_System SHALL include the machine identifier for every run.

### Requirement 4: File Identity for Cross-Machine Undo

**User Story:** As a user, I want to undo operations on a different machine than where they were performed, so that I can recover from mistakes regardless of which machine I use.

#### Acceptance Criteria

1. THE Audit_System SHALL record a content hash (SHA-256) of the file before any move operation.
2. THE Audit_System SHALL record the file size in bytes before any move operation.
3. THE Audit_System SHALL record the file modification timestamp before any move operation.
4. WHEN a move operation completes, THE Audit_System SHALL record the content hash of the file at the destination.
5. THE Audit_System SHALL record both source and destination paths as they appeared on the originating machine.
6. WHEN performing undo, THE Undo_System SHALL use content hash as the primary identity match.
7. WHEN content hash matches but path differs, THE Undo_System SHALL proceed with undo after logging the path discrepancy.
8. WHEN content hash does not match, THE Undo_System SHALL abort that specific undo operation and record an IDENTITY_MISMATCH event.
9. WHEN multiple files match the same content hash, THE Undo_System SHALL use file size and modification time as secondary discriminators.
10. WHEN identity remains ambiguous after all checks, THE Undo_System SHALL abort that undo operation and record an AMBIGUOUS_IDENTITY event.

### Requirement 5: Undo Most Recent Run

**User Story:** As a user, I want to undo the most recent run, so that I can quickly reverse mistakes from my last execution.

#### Acceptance Criteria

1. WHEN the user requests undo of the most recent run, THE Undo_System SHALL identify the run with the latest start timestamp.
2. THE Undo_System SHALL process events from the most recent run in reverse chronological order.
3. FOR each MOVE event, THE Undo_System SHALL move the file from destination back to source.
4. FOR each ROUTE_TO_REVIEW event, THE Undo_System SHALL move the file from review directory back to source.
5. FOR each DUPLICATE_DETECTED event where a rename occurred, THE Undo_System SHALL restore the original filename.
6. FOR SKIP, PARSE_FAILURE, and VALIDATION_FAILURE events, THE Undo_System SHALL take no action (no-op).
7. THE Undo_System SHALL verify file identity before each undo operation.
8. THE Undo_System SHALL record all undo operations as new audit events in a new run.

### Requirement 6: Undo Any Prior Run

**User Story:** As a user, I want to undo any specific prior run, so that I can selectively reverse operations from any point in history.

#### Acceptance Criteria

1. WHEN the user specifies a Run_ID, THE Undo_System SHALL locate all events for that run across all log segments.
2. THE Undo_System SHALL validate that the specified Run_ID exists in the audit log.
3. IF the Run_ID does not exist, THE Undo_System SHALL report an error and take no action.
4. THE Undo_System SHALL apply the same undo logic as for most recent run.
5. WHEN undoing an older run, THE Undo_System SHALL detect conflicts with subsequent runs and report them.
6. WHEN a file has been modified by a subsequent run, THE Undo_System SHALL skip that file and record a CONFLICT_DETECTED event.

### Requirement 7: Undo Across Machines

**User Story:** As a user, I want to use an audit log from one machine to undo operations on another machine, so that I can manage files across my devices.

#### Acceptance Criteria

1. THE Audit_Log format SHALL be portable across operating systems.
2. THE Undo_System SHALL accept a path mapping configuration to translate paths between machines.
3. WHEN source paths differ due to mount points or drive letters, THE Undo_System SHALL apply path mappings before attempting operations.
4. THE Undo_System SHALL use content hash as the authoritative file identity, not path.
5. WHEN a file cannot be located at the expected path, THE Undo_System SHALL search configured directories for a content hash match.
6. THE Undo_System SHALL record the originating machine identifier when performing cross-machine undo.
7. WHEN cross-machine undo encounters path resolution failures, THE Undo_System SHALL record detailed diagnostics.

### Requirement 8: Append-Only Log Storage

**User Story:** As a user, I want the audit log to be append-only, so that historical records cannot be accidentally modified or corrupted.

#### Acceptance Criteria

1. THE Audit_System SHALL only append new records to the log file, never modifying existing records.
2. THE Audit_System SHALL use a structured, machine-readable format (JSON Lines).
3. THE Audit_System SHALL write each event as a complete, self-contained record.
4. THE Audit_System SHALL flush writes to disk after each event to ensure durability.
5. THE Audit_System SHALL not delete or truncate log files during normal operation.

### Requirement 9: Log Rotation

**User Story:** As a user, I want log files to rotate automatically, so that individual files remain manageable in size.

#### Acceptance Criteria

1. THE Audit_System SHALL support configurable rotation based on file size.
2. THE Audit_System SHALL support configurable rotation based on time period (daily, weekly).
3. WHEN rotation occurs, THE Audit_System SHALL create a new log segment with a sequential or timestamped name.
4. THE Audit_System SHALL maintain an index or naming convention that allows discovery of all segments.
5. WHEN rotation occurs mid-run, THE Audit_System SHALL continue the run seamlessly in the new segment.
6. THE Audit_System SHALL record a ROTATION event when creating a new segment.

### Requirement 10: Log Retention

**User Story:** As a user, I want to configure how long audit logs are retained, so that I can balance storage usage with undo capability.

#### Acceptance Criteria

1. THE Audit_System SHALL support configurable retention period (days, count of runs, or unlimited).
2. WHEN retention limits are exceeded, THE Audit_System SHALL prune the oldest log segments.
3. BEFORE pruning, THE Audit_System SHALL warn that undo capability for affected runs will be lost.
4. THE Audit_System SHALL never prune logs for runs less than a configurable minimum age (default: 7 days).
5. THE Audit_System SHALL record a RETENTION_PRUNE event when removing old segments.
6. THE Audit_System SHALL treat audit logs as user data worthy of backup.

### Requirement 11: Fail-Fast on Audit Failure

**User Story:** As a user, I want the program to stop immediately if it cannot write audit records, so that no file operations occur without being logged.

#### Acceptance Criteria

1. IF an audit record cannot be written, THE Audit_System SHALL immediately halt all file operations.
2. THE Audit_System SHALL report the audit write failure to the user with specific error details.
3. THE Audit_System SHALL not attempt to continue processing files after an audit write failure.
4. THE Audit_System SHALL ensure no file move occurs unless its corresponding audit record is durably written first.
5. WHEN the audit log file cannot be created or opened, THE Audit_System SHALL fail before processing any files.

### Requirement 12: Handling Corrupt or Missing Logs

**User Story:** As a user, I want clear behavior when audit logs are damaged or missing, so that I understand the system state and limitations.

#### Acceptance Criteria

1. WHEN an audit log file is missing, THE Audit_System SHALL create a new log and record a LOG_INITIALIZED event.
2. WHEN an audit log file is corrupt, THE Audit_System SHALL report the corruption and refuse to append to it.
3. WHEN corruption is detected, THE Audit_System SHALL offer to archive the corrupt file and start fresh.
4. THE Undo_System SHALL report which runs are unavailable due to missing or corrupt log segments.
5. THE Audit_System SHALL validate log integrity on startup by checking the last record is complete.

### Requirement 13: Undo Safety and Collision Handling

**User Story:** As a user, I want undo operations to handle edge cases safely, so that I don't lose data during recovery.

#### Acceptance Criteria

1. WHEN the undo destination already has a file, THE Undo_System SHALL not overwrite it.
2. WHEN destination collision occurs, THE Undo_System SHALL record a COLLISION event and skip that file.
3. WHEN the source file for undo is missing, THE Undo_System SHALL record a SOURCE_MISSING event and continue with other files.
4. WHEN file content has changed since the original operation, THE Undo_System SHALL record a CONTENT_CHANGED event and skip that file.
5. THE Undo_System SHALL support partial undo, continuing with remaining files after individual failures.
6. THE Undo_System SHALL be idempotent: running undo twice on the same run SHALL produce the same end state.
7. WHEN undo is interrupted, THE Undo_System SHALL be resumable from the last successful operation.

### Requirement 14: Undo Auditability

**User Story:** As a user, I want undo operations to be logged just like regular operations, so that I have a complete history of all changes.

#### Acceptance Criteria

1. WHEN an undo operation begins, THE Audit_System SHALL create a new run with type UNDO.
2. THE Audit_System SHALL record the original Run_ID being undone in the undo run's metadata.
3. FOR each file restored, THE Audit_System SHALL record an UNDO_MOVE event with source, destination, and identity verification result.
4. FOR each file skipped during undo, THE Audit_System SHALL record an UNDO_SKIP event with reason.
5. THE Audit_System SHALL record undo completion status and summary statistics.

### Requirement 15: User-Facing Audit Views

**User Story:** As a user, I want to view audit history in human-readable formats, so that I can review what happened and make informed decisions.

#### Acceptance Criteria

1. THE Audit_System SHALL provide a command to list all runs with summary statistics.
2. THE Audit_System SHALL provide a command to show detailed events for a specific run.
3. THE run list view SHALL show: Run_ID, timestamp, file counts (moved, skipped, review, errors), and status.
4. THE run detail view SHALL show: each event with timestamp, operation, source, destination, and outcome.
5. THE Audit_System SHALL support filtering events by type (moves only, errors only, etc.).
6. THE Audit_System SHALL support exporting a run's audit data for troubleshooting (local file export only).

### Requirement 16: Error Recording Detail

**User Story:** As a user, I want errors to be recorded with sufficient detail for troubleshooting, so that I can diagnose and resolve issues.

#### Acceptance Criteria

1. WHEN an error occurs, THE Audit_System SHALL record the error type/category.
2. WHEN an error occurs, THE Audit_System SHALL record the error message.
3. WHEN an error occurs, THE Audit_System SHALL record the operation that failed.
4. THE Audit_System SHALL NOT record sensitive system information (environment variables, credentials).
5. THE Audit_System SHALL record stack traces or error chains when available, redacting sensitive paths if configured.
