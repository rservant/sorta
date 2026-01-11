# Implementation Plan: Rename Source/Target to Inbound/Outbound

## Overview

This plan implements a terminology refactoring across the Sorta codebase, changing "source" to "inbound" and "target" to "outbound" in configuration, code, CLI, and documentation.

## Tasks

- [x] 1. Update core configuration types
  - [x] 1.1 Rename fields in `internal/config/config.go`
    - Change `SourceDirectories` to `InboundDirectories` with JSON tag `inboundDirectories`
    - Change `TargetDirectory` to `OutboundDirectory` with JSON tag `outboundDirectory`
    - Rename `HasSourceDirectory` to `HasInboundDirectory`
    - Rename `AddSourceDirectory` to `AddInboundDirectory`
    - Update validation error messages
    - _Requirements: 1.1, 1.2, 2.1, 2.2, 2.3_

  - [x] 1.2 Write property test for configuration round-trip
    - **Property 1: Configuration JSON Round-Trip**
    - **Validates: Requirements 1.1, 1.2, 1.3, 1.4**

  - [x] 1.3 Update `internal/config/config_test.go`
    - Update all test JSON literals to use new keys
    - Update test assertions to use new field names
    - Update property test generators
    - _Requirements: 5.1, 5.2_

- [x] 2. Update classifier component
  - [x] 2.1 Rename field in `internal/classifier/classifier.go`
    - Change `TargetDirectory` to `OutboundDirectory` in ClassificationResult struct
    - Update all references to the field
    - _Requirements: 2.2_

  - [x] 2.2 Update `internal/classifier/classifier_test.go`
    - Update test data to use `OutboundDirectory`
    - Update assertions
    - _Requirements: 5.2_

- [x] 3. Update matcher tests
  - [x] 3.1 Update `internal/matcher/matcher_test.go`
    - Update test data to use `OutboundDirectory`
    - _Requirements: 5.2_

- [x] 4. Update orchestrator component
  - [x] 4.1 Update `internal/orchestrator/orchestrator.go`
    - Change `cfg.SourceDirectories` to `cfg.InboundDirectories`
    - Change `classification.TargetDirectory` to `classification.OutboundDirectory`
    - _Requirements: 2.1, 2.2_

  - [x] 4.2 Update `internal/orchestrator/orchestrator_test.go`
    - Update test configurations to use new field names
    - _Requirements: 5.1, 5.2_

- [x] 5. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Update CLI
  - [x] 6.1 Update `cmd/sorta/main.go`
    - Rename `add-source` command to `add-inbound`
    - Rename `runAddSourceCommand` to `runAddInboundCommand`
    - Update all display functions to use "Inbound" and "Outbound" terminology
    - Update help text and usage messages
    - Update field references throughout
    - _Requirements: 3.1, 3.2, 3.3_

  - [x] 6.2 Update `cmd/sorta/main_test.go`
    - Update tests to use `add-inbound` command
    - Update assertions for new terminology
    - _Requirements: 5.2_

- [x] 7. Update test data and documentation
  - [x] 7.1 Update `testdata/sorta-config.json`
    - Change `sourceDirectories` to `inboundDirectories`
    - Change `targetDirectory` to `outboundDirectory` in all prefix rules
    - _Requirements: 5.1_

  - [x] 7.2 Update `README.md`
    - Replace "source" with "inbound" for scan directories
    - Replace "target" with "outbound" for destination directories
    - Update CLI examples to use `add-inbound`
    - _Requirements: 4.1, 4.2_

- [x] 8. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- This is a mechanical refactoring - all behavior remains unchanged
- The existing property tests in config_test.go already cover round-trip and duplicate prevention
- After renaming, the existing tests will validate the new field names work correctly
