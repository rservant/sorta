# Implementation Plan: Verbose Output and Progress Indicators

## Overview

This plan implements verbose output (`-v`/`--verbose`) for all commands and progress indicators for non-verbose mode. The implementation follows a bottom-up approach: first creating the output package, then integrating it into the CLI and commands.

## Tasks

- [x] 1. Create the output package
  - [x] 1.1 Create `internal/output/output.go` with Config, Output struct, and core methods
    - Implement `New()`, `DefaultConfig()`, `Verbose()`, `Info()`, `Error()`
    - Implement TTY detection using `os.Stdout.Fd()` and `term.IsTerminal()`
    - _Requirements: 6.1, 6.2, 6.3_
  - [x] 1.2 Implement progress indicator methods
    - Implement `StartProgress()`, `UpdateProgress()`, `EndProgress()`
    - Use carriage return (`\r`) for in-place updates
    - Suppress progress when not TTY or when verbose mode is enabled
    - _Requirements: 5.1, 5.4, 5.5, 5.6_
  - [x] 1.3 Write unit tests for output package
    - Test TTY detection behavior with mocked writers
    - Test progress format matches "Processing file N/M..." pattern
    - Test verbose output only appears when enabled
    - _Requirements: 5.4, 6.1, 6.3_

- [x] 2. Extend argument parser for verbose flag
  - [x] 2.1 Update `ParseResult` struct and `parseArgs` function in `cmd/sorta/main.go`
    - Add `Verbose bool` field to ParseResult
    - Parse `-v` and `--verbose` flags before command
    - Support `-v=true`/`--verbose=true` format
    - _Requirements: 1.1, 1.2_
  - [x] 2.2 Update `printUsage()` to document verbose flag
    - Add verbose flag to flags section
    - _Requirements: 1.3_
  - [x] 2.3 Write property test for verbose flag parsing
    - **Property 1: Verbose Flag Parsing**
    - **Validates: Requirements 1.1, 1.2**

- [x] 3. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Integrate output package into run command
  - [x] 4.1 Add `ProgressCallback` to orchestrator Options
    - Update `internal/orchestrator/orchestrator.go` to accept and call progress callback
    - Call callback after each file is processed
    - _Requirements: 5.1_
  - [x] 4.2 Update `runRunCommand` to use output package
    - Create Output instance with verbose config
    - Pass progress callback to orchestrator
    - Output verbose information for each file operation
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 5.1_
  - [x] 4.3 Write property test for run command verbose output
    - **Property 2: Verbose Output Content for Run Command**
    - **Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5**

- [x] 5. Integrate output package into discover command
  - [x] 5.1 Add callback support to discovery module
    - Create `DiscoveryCallback` type and `DiscoveryEvent` struct
    - Add `DiscoverWithCallback` function to `internal/discovery/discovery.go`
    - Call callback for each directory scanned, file analyzed, and pattern found
    - _Requirements: 3.1, 3.2, 3.3, 5.2_
  - [x] 5.2 Update `runDiscoverCommand` to use output package
    - Create Output instance with verbose config
    - Pass callback to discovery
    - Output verbose information for each discovery event
    - _Requirements: 3.1, 3.2, 3.3, 5.2_
  - [x] 5.3 Write property test for discover command verbose output
    - **Property 3: Verbose Output Content for Discover Command**
    - **Validates: Requirements 3.1, 3.2, 3.3**

- [x] 6. Integrate output package into undo command
  - [x] 6.1 Add callback support to undo engine
    - Create callback type for undo progress reporting
    - Update `internal/audit/undo.go` to accept and call progress callback
    - _Requirements: 4.1, 4.2, 4.3, 5.3_
  - [x] 6.2 Update `runUndoCommand` to use output package
    - Create Output instance with verbose config
    - Pass callback to undo engine
    - Output verbose information for each undo operation
    - _Requirements: 4.1, 4.2, 4.3, 5.3_
  - [x] 6.3 Write property test for undo command verbose output
    - **Property 4: Verbose Output Content for Undo Command**
    - **Validates: Requirements 4.1, 4.2, 4.3**

- [x] 7. Integrate output package into remaining commands
  - [x] 7.1 Update `runConfigCommand` to use output package
    - Pass Output instance for consistent output handling
    - _Requirements: 1.2_
  - [x] 7.2 Update `runAddSourceCommand` to use output package
    - Pass Output instance for consistent output handling
    - _Requirements: 1.2_
  - [x] 7.3 Update `runAuditCommand` to use output package
    - Pass Output instance for consistent output handling
    - _Requirements: 1.2_

- [x] 8. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 9. Write property test for progress indicator format
  - **Property 5: Progress Indicator Format and Lifecycle**
  - **Validates: Requirements 5.1, 5.2, 5.3, 5.4, 5.5, 5.6**

- [x] 10. Write property test for TTY detection behavior
  - **Property 6: TTY Detection Behavior**
  - **Validates: Requirements 6.1, 6.2, 6.3**

- [x] 11. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks are required for comprehensive implementation
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
- The `golang.org/x/term` package is used for TTY detection
