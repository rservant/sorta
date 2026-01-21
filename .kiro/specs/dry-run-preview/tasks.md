# Implementation Plan: Dry Run Preview

## Overview

This implementation adds preview capabilities through a `--dry-run` flag for the run command and a new `status` command. Changes are primarily in the orchestrator and CLI.

## Tasks

- [x] 1. Add Dry Run Support to Orchestrator
  - [x] 1.1 Add RunOptions and RunResult types to orchestrator
    - Add `RunOptions` struct with `DryRun` and `Verbose` fields
    - Add `RunResult` struct with `Moved`, `ForReview`, `Skipped`, `Errors` fields
    - Add `FileOperation` struct with `Source`, `Destination`, `Prefix`, `Reason` fields
    - _Requirements: 1.1, 1.2, 1.3_

  - [x] 1.2 Implement dry-run mode in orchestrator Run method
    - Modify `Run` to accept `RunOptions`
    - When `DryRun=true`, collect operations without executing
    - Skip directory creation and file moves in dry-run mode
    - Skip audit logging in dry-run mode
    - Return `RunResult` with all planned operations
    - _Requirements: 1.1, 1.4, 1.5_

  - [x] 1.3 Write property test for filesystem immutability
    - **Property: Dry-Run Filesystem Immutability**
    - Generate random file sets and configurations
    - Verify no files created, moved, or deleted in dry-run mode
    - Verify no audit log entries written
    - _Rationale: Universal invariant - dry-run must never modify filesystem_
    - **Validates: Requirements 1.1, 1.4, 1.5, 2.6**

- [x] 2. Implement Status Command
  - [x] 2.1 Add Status method to orchestrator
    - Create `StatusResult` and `InboundStatus` types
    - Implement `Status()` method that scans without modifying
    - Group files by destination (prefix or for-review)
    - Calculate per-directory and grand totals
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

  - [x] 2.2 Write unit tests for status grouping and counts
    - Test files grouped by matching prefix
    - Test unmatched files grouped under for-review
    - Test per-directory counts are accurate
    - Test grand total equals sum of all groups
    - _Requirements: 2.2, 2.3, 2.4, 3.2_

- [x] 3. Add Output Formatting
  - [x] 3.1 Add preview output methods to output package
    - Implement `PrintDryRunResult(result *RunResult)`
    - Implement `PrintStatusResult(result *StatusResult)`
    - Implement `PrintSummary(moved, forReview, skipped int)`
    - Format source → destination for each file
    - _Requirements: 3.1, 3.2, 1.6_

  - [x] 3.2 Write unit tests for output formatting
    - Test each planned operation appears in output
    - Test source → destination format is correct
    - Test summary counts match operation counts
    - _Requirements: 1.2, 1.3, 1.6, 3.1_

- [x] 4. Update CLI
  - [x] 4.1 Add --dry-run flag to run command
    - Parse `--dry-run` flag
    - Pass to orchestrator via `RunOptions`
    - Print dry-run results using output package
    - _Requirements: 1.1, 1.2, 1.3, 1.6_

  - [x] 4.2 Add status command
    - Implement `status` subcommand
    - Call orchestrator `Status()` method
    - Print results using output package
    - Handle empty directories case
    - _Requirements: 2.1, 2.5, 2.6_

  - [x] 4.3 Write unit tests for verbose mode
    - Test verbose output includes additional file details
    - Test non-verbose output is concise
    - _Requirements: 3.4_

- [x] 5. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- The dry-run mode reuses existing classification logic but skips execution
- Status command is read-only and shares classification with dry-run
- Property test retained for filesystem immutability (universal invariant)
- Unit tests used for output formatting and counts (examples suffice)
