# Implementation Plan: Audit Trail

## Overview

This implementation plan breaks down the Audit Trail feature into incremental coding tasks. Each task builds on previous work, with property tests placed close to implementation to catch errors early. The plan follows the existing Sorta project structure and uses Go with the gopter library for property-based testing.

## Tasks

- [x] 1. Set up audit package structure and core types
  - Create `internal/audit/` directory
  - Define core types: RunID, EventType, OperationStatus, ReasonCode enumerations
  - Define AuditEvent, FileIdentity, ErrorDetails, RunInfo structs
  - Define AuditConfig struct for configuration
  - _Requirements: 1.1, 1.2, 3.1-3.9_

- [x] 2. Implement FileIdentity capture
  - [x] 2.1 Create identity.go with IdentityResolver implementation
    - Implement CaptureIdentity to compute SHA-256 hash, size, and modTime
    - Implement VerifyIdentity to compare captured identity with file
    - Implement FindByHash to search directories for matching content hash
    - _Requirements: 4.1, 4.2, 4.3, 4.6_
  - [x] 2.2 Write property test for FileIdentity round-trip
    - **Property 5: FileIdentity Completeness for Move Operations**
    - **Validates: Requirements 4.1, 4.2, 4.3, 4.4**
  - [x] 2.3 Write unit tests for identity edge cases
    - Test empty file, large file, file with special characters in name
    - Test VerifyIdentity with matching and non-matching hashes
    - _Requirements: 4.6, 4.8_

- [x] 3. Implement AuditEvent serialization
  - [x] 3.1 Create event.go with JSON serialization for AuditEvent
    - Implement MarshalJSON and UnmarshalJSON for AuditEvent
    - Ensure ISO 8601 timestamp format
    - Handle optional fields (destinationPath, reasonCode, fileIdentity, errorDetails)
    - _Requirements: 3.1, 8.2, 8.3_
  - [x] 3.2 Write property test for JSON Lines round-trip
    - **Property 11: JSON Lines Round-Trip**
    - **Validates: Requirements 8.2, 8.3**
  - [x] 3.3 Write unit tests for event serialization
    - Test each EventType serializes correctly
    - Test optional fields are omitted when nil
    - _Requirements: 3.3, 3.4_

- [x] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Implement AuditWriter with append-only semantics
  - [x] 5.1 Create writer.go with AuditWriter implementation
    - Implement StartRun to generate UUID v4 and write RUN_START event
    - Implement WriteEvent to append JSON line and flush
    - Implement EndRun to write RUN_END event with summary
    - Implement fail-fast: return error immediately on write failure
    - _Requirements: 1.1, 1.4, 1.5, 8.1, 8.4, 11.1, 11.4_
  - [x] 5.2 Write property test for Run_ID uniqueness
    - **Property 1: Run_ID Uniqueness and Format**
    - **Validates: Requirements 1.1, 1.2**
  - [x] 5.3 Write property test for append-only integrity
    - **Property 10: Append-Only Log Integrity**
    - **Validates: Requirements 8.1, 8.5**
  - [x] 5.4 Write unit tests for writer error handling
    - Test write to read-only file fails fast
    - Test write to full disk fails fast
    - _Requirements: 11.1, 11.5_

- [x] 6. Implement event recording for all operation types
  - [x] 6.1 Add helper methods to AuditWriter for each event type
    - RecordMove(source, dest, identity) for MOVE events
    - RecordRouteToReview(source, dest, reason) for ROUTE_TO_REVIEW events
    - RecordSkip(source, reason) for SKIP events
    - RecordDuplicate(source, intended, actual, action) for DUPLICATE_DETECTED events
    - RecordParseFailure(source, pattern, reason) for PARSE_FAILURE events
    - RecordError(source, errType, errMsg, operation) for ERROR events
    - _Requirements: 2.1-2.7_
  - [x] 6.2 Write property test for event field completeness
    - **Property 3: Event Field Completeness by Type**
    - **Validates: Requirements 2.1-2.7, 3.1-3.7**
  - [x] 6.3 Write property test for one event per file
    - **Property 4: One Event Per File**
    - **Validates: Requirements 2.8**

- [x] 7. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Implement log rotation
  - [x] 8.1 Create rotation.go with rotation logic
    - Implement size-based rotation check
    - Implement time-based rotation check (daily, weekly)
    - Generate rotated filename with timestamp
    - Write ROTATION event before switching files
    - Update or create index file for segment discovery
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.6_
  - [x] 8.2 Write property test for rotation segment discoverability
    - **Property 12: Rotation Segment Discoverability**
    - **Validates: Requirements 9.3, 9.4, 9.5**
  - [x] 8.3 Write unit tests for rotation edge cases
    - Test rotation at exact size boundary
    - Test rotation mid-run continues seamlessly
    - _Requirements: 9.5_

- [x] 9. Implement AuditReader
  - [x] 9.1 Create reader.go with AuditReader implementation
    - Implement ListRuns to return all runs with summaries
    - Implement GetRun to return all events for a run ID
    - Implement GetLatestRun to find most recent run by timestamp
    - Implement FilterEvents to filter by event type
    - Handle reading across multiple rotated segments
    - _Requirements: 6.1, 15.1, 15.2, 15.3, 15.4, 15.5_
  - [x] 9.2 Write property test for event filtering
    - **Property 16: Event Filtering Correctness**
    - **Validates: Requirements 15.5**
  - [x] 9.3 Write unit tests for reader
    - Test reading empty log
    - Test reading log with multiple runs
    - Test reading across rotated segments
    - _Requirements: 6.1, 9.5_

- [x] 10. Implement log integrity validation
  - [x] 10.1 Add integrity checking to AuditReader
    - Validate last line is complete JSON on startup
    - Detect and report corrupt log files
    - Implement LOG_INITIALIZED event for new logs
    - _Requirements: 12.1, 12.2, 12.5_
  - [x] 10.2 Write unit tests for corruption detection
    - Test truncated last line detection
    - Test invalid JSON detection
    - Test missing log file handling
    - _Requirements: 12.1, 12.2, 12.4, 12.5_

- [x] 11. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 12. Implement UndoEngine core logic
  - [x] 12.1 Create undo.go with UndoEngine implementation
    - Implement UndoLatest to find and undo most recent run
    - Implement UndoRun to undo specific run by ID
    - Process events in reverse chronological order
    - Verify file identity before each undo operation
    - Create new UNDO run to record undo operations
    - _Requirements: 5.1, 5.2, 5.7, 5.8, 6.1, 6.2, 14.1, 14.2_
  - [x] 12.2 Write property test for undo reverse ordering
    - **Property 7: Undo Reverse Chronological Ordering**
    - **Validates: Requirements 5.2**
  - [x] 12.3 Write property test for undo restores locations
    - **Property 8: Undo Restores File Locations**
    - **Validates: Requirements 5.3, 5.4**

- [x] 13. Implement undo for each event type
  - [x] 13.1 Add undo handlers for each event type
    - MOVE: move file from destination back to source
    - ROUTE_TO_REVIEW: move file from review back to source
    - DUPLICATE_DETECTED: restore original filename if renamed
    - SKIP, PARSE_FAILURE, VALIDATION_FAILURE: no-op
    - Record UNDO_MOVE or UNDO_SKIP events
    - _Requirements: 5.3, 5.4, 5.5, 5.6, 14.3, 14.4_
  - [x] 13.2 Write unit tests for undo handlers
    - Test undo of each event type
    - Test no-op events produce UNDO_SKIP
    - _Requirements: 5.3-5.6_

- [x] 14. Implement undo safety and collision handling
  - [x] 14.1 Add collision and error handling to UndoEngine
    - Check destination before undo move, record COLLISION if occupied
    - Check source exists, record SOURCE_MISSING if not
    - Verify content hash, record IDENTITY_MISMATCH if different
    - Continue with remaining files after individual failures
    - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.5_
  - [x] 14.2 Write property test for undo collision safety
    - **Property 15: Undo Collision Safety**
    - **Validates: Requirements 13.1, 13.2**
  - [x] 14.3 Write property test for undo idempotency
    - **Property 9: Undo Idempotency**
    - **Validates: Requirements 13.6**
  - [x] 14.4 Write unit tests for undo edge cases
    - Test partial undo continues after failure
    - Test interrupted undo is resumable
    - _Requirements: 13.5, 13.7_

- [x] 15. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 16. Implement cross-machine undo with path mapping
  - [x] 16.1 Add path mapping support to UndoEngine
    - Accept PathMapping configuration
    - Apply mappings to translate paths before operations
    - Search by content hash when file not at expected path
    - Record originating machine ID in undo events
    - _Requirements: 7.2, 7.3, 7.5, 7.6_
  - [x] 16.2 Write property test for content hash as primary identity
    - **Property 6: Content Hash as Primary Identity for Undo**
    - **Validates: Requirements 4.6, 4.7, 4.8, 7.4**
  - [x] 16.3 Write unit tests for path mapping
    - Test path translation with different prefixes
    - Test hash-based file discovery
    - _Requirements: 7.2, 7.3, 7.5_

- [x] 17. Implement conflict detection for older run undo
  - [x] 17.1 Add conflict detection to UndoEngine
    - When undoing older run, check if file was modified by subsequent runs
    - Record CONFLICT_DETECTED event and skip conflicting files
    - _Requirements: 6.5, 6.6_
  - [x] 17.2 Write unit tests for conflict detection
    - Test undo of run 1 when run 2 modified same file
    - Test conflict event is recorded correctly
    - _Requirements: 6.5, 6.6_

- [x] 18. Implement retention and pruning
  - [x] 18.1 Add retention logic to AuditWriter
    - Check retention limits (days, run count) on startup
    - Prune oldest segments when limits exceeded
    - Never prune segments with runs younger than minimum age
    - Record RETENTION_PRUNE event when pruning
    - _Requirements: 10.1, 10.2, 10.4, 10.5_
  - [x] 18.2 Write property test for minimum retention protection
    - **Property 17: Minimum Retention Protection**
    - **Validates: Requirements 10.4**
  - [x] 18.3 Write unit tests for retention
    - Test pruning with day-based retention
    - Test pruning with run-count retention
    - Test minimum age protection
    - _Requirements: 10.1, 10.2, 10.4_

- [x] 19. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 20. Integrate audit system with Orchestrator
  - [x] 20.1 Update Orchestrator to use AuditWriter
    - Initialize AuditWriter at start of run
    - Call StartRun before processing files
    - Record events for each file operation
    - Call EndRun with summary at completion
    - Implement fail-fast: stop processing if audit write fails
    - _Requirements: 11.1, 11.4_
  - [x] 20.2 Write property test for audit-before-move ordering
    - **Property 14: Audit-Before-Move Ordering**
    - **Validates: Requirements 11.4**
  - [x] 20.3 Write property test for fail-fast behavior
    - **Property 13: Fail-Fast on Audit Write Failure**
    - **Validates: Requirements 11.1, 11.3, 11.5**

- [x] 21. Add CLI commands for audit operations
  - [x] 21.1 Add `audit list` command
    - List all runs with Run_ID, timestamp, counts, status
    - _Requirements: 15.1, 15.3_
  - [x] 21.2 Add `audit show <run-id>` command
    - Show detailed events for a specific run
    - Support filtering by event type
    - _Requirements: 15.2, 15.4, 15.5_
  - [x] 21.3 Add `audit export <run-id>` command
    - Export run audit data to file for troubleshooting
    - _Requirements: 15.6_
  - [x] 21.4 Add `undo` command
    - `undo` - undo most recent run
    - `undo <run-id>` - undo specific run
    - `undo --preview` - show what would be undone
    - Support `--path-mapping` flag for cross-machine undo
    - _Requirements: 5.1, 6.1, 7.2_
  - [x] 21.5 Write unit tests for CLI commands
    - Test each command with valid and invalid inputs
    - _Requirements: 15.1-15.6_

- [x] 22. Add audit configuration to sorta-config.json
  - [x] 22.1 Extend Configuration struct with AuditConfig
    - Add audit section to config schema
    - Set sensible defaults (rotation: 10MB, retention: 30 days, min: 7 days)
    - _Requirements: 9.1, 9.2, 10.1, 10.4_
  - [x] 22.2 Write unit tests for config parsing
    - Test default values applied when audit section missing
    - Test custom values override defaults
    - _Requirements: 9.1, 10.1_

- [x] 23. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks including tests are required for comprehensive coverage
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
- The audit package is designed to be independent and testable in isolation before integration
