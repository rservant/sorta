# Implementation Plan: Dry Run Preview

## Overview

This implementation adds preview capabilities through a `--dry-run` flag for the run command and a new `status` command. Changes are primarily in the orchestrator and CLI.

## Tasks

- [ ] 1. Add Dry Run Support to Orchestrator
  - [ ] 1.1 Add RunOptions and RunResult types to orchestrator
    - Add `RunOptions` struct with `DryRun` and `Verbose` fields
    - Add `RunResult` struct with `Moved`, `ForReview`, `Skipped`, `Errors` fields
    - Add `FileOperation` struct with `Source`, `Destination`, `Prefix`, `Reason` fields
    - _Requirements: 1.1, 1.2, 1.3_

  - [ ] 1.2 Implement dry-run mode in orchestrator Run method
    - Modify `Run` to accept `RunOptions`
    - When `DryRun=true`, collect operations without executing
    - Skip directory creation and file moves in dry-run mode
    - Skip audit logging in dry-run mode
    - Return `RunResult` with all planned operations
    - _Requirements: 1.1, 1.4, 1.5_

  - [ ]* 1.3 Write property test for filesystem immutability
    - **Property 1: Dry-Run Filesystem Immutability**
    - **Validates: Requirements 1.1, 1.4, 1.5, 2.6**

- [ ] 2. Implement Status Command
  - [ ] 2.1 Add Status method to orchestrator
    - Create `StatusResult` and `InboundStatus` types
    - Implement `Status()` method that scans without modifying
    - Group files by destination (prefix or for-review)
    - Calculate per-directory and grand totals
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

  - [ ]* 2.2 Write property test for status grouping and counts
    - **Property 3: Status Grouping and Counts**
    - **Validates: Requirements 2.2, 2.3, 2.4, 3.2**

- [ ] 3. Add Output Formatting
  - [ ] 3.1 Add preview output methods to output package
    - Implement `PrintDryRunResult(result *RunResult)`
    - Implement `PrintStatusResult(result *StatusResult)`
    - Implement `PrintSummary(moved, forReview, skipped int)`
    - Format source → destination for each file
    - _Requirements: 3.1, 3.2, 1.6_

  - [ ]* 3.2 Write property test for output completeness
    - **Property 2: Dry-Run Output Completeness**
    - **Validates: Requirements 1.2, 1.3, 3.1**

  - [ ]* 3.3 Write property test for summary accuracy
    - **Property 4: Summary Count Accuracy**
    - **Validates: Requirements 1.6**

- [ ] 4. Update CLI
  - [ ] 4.1 Add --dry-run flag to run command
    - Parse `--dry-run` flag
    - Pass to orchestrator via `RunOptions`
    - Print dry-run results using output package
    - _Requirements: 1.1, 1.2, 1.3, 1.6_

  - [ ] 4.2 Add status command
    - Implement `status` subcommand
    - Call orchestrator `Status()` method
    - Print results using output package
    - Handle empty directories case
    - _Requirements: 2.1, 2.5, 2.6_

  - [ ]* 4.3 Write property test for verbose mode detail
    - **Property 5: Verbose Mode Additional Detail**
    - **Validates: Requirements 3.4**

- [ ] 5. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are property-based tests
- The dry-run mode reuses existing classification logic but skips execution
- Status command is read-only and shares classification with dry-run
