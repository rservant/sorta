# Implementation Plan: Watch Mode

## Overview

This implementation adds file system watching, run summaries, and audit statistics. New watcher component and stats aggregation are created.

## Tasks

- [ ] 1. Implement File Watcher
  - [ ] 1.1 Create watcher component
    - Create `internal/watcher/watcher.go`
    - Add `Watcher`, `WatchConfig`, `WatchSummary` types
    - Implement `Start(dirs []string) error`
    - Implement `Stop() *WatchSummary`
    - Use fsnotify for file system events
    - _Requirements: 1.1, 1.6, 1.7_

  - [ ] 1.2 Implement debounce handler
    - Create `internal/watcher/debounce.go`
    - Add `Debouncer` type with configurable delay
    - Coalesce rapid events for same file
    - _Requirements: 1.3_

  - [ ] 1.3 Implement file stability checker
    - Create `internal/watcher/stability.go`
    - Add `StabilityChecker` type
    - Wait for file size to stabilize before processing
    - _Requirements: 1.4_

  - [ ] 1.4 Implement temporary file filtering
    - Add ignore pattern matching
    - Skip files matching .tmp, .part, .download, etc.
    - Support configurable patterns
    - _Requirements: 1.8_

  - [ ] 1.5 Write unit tests for watcher
    - Test new file triggers organization
    - Test rapid events are debounced
    - Test file processed after size stabilizes
    - Test .tmp, .part, .download files are ignored
    - Test audit events logged for each operation
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.8_

- [ ] 2. Add Watch Configuration
  - [ ] 2.1 Add watch config to configuration
    - Add `Watch` field to `Configuration` struct
    - Add `WatchConfig` struct with debounce, stability, ignore fields
    - Implement defaults (debounce: 2s, stability: 1000ms)
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

  - [ ] 2.2 Write unit tests for watch configuration
    - Test default debounce is 2s
    - Test default stability is 1000ms
    - Test custom values override defaults
    - Test ignore patterns are applied
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_

- [ ] 3. Implement Run Summary
  - [ ] 3.1 Create summary generator
    - Create `internal/orchestrator/summary.go`
    - Add `RunSummary` type
    - Implement `GenerateSummary(result *RunResult, verbose bool)`
    - Calculate moved, for-review, skipped, errors, duration
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

  - [ ] 3.2 Add per-prefix breakdown for verbose mode
    - Track counts per prefix during run
    - Include in summary when verbose=true
    - _Requirements: 3.6_

  - [ ] 3.3 Integrate summary into run command
    - Generate summary after run completes
    - Display using output package
    - _Requirements: 3.1_

  - [ ] 3.4 Write unit tests for run summary
    - Test moved count matches actual moves
    - Test for-review count matches routed files
    - Test skipped count matches skipped files
    - Test error count matches failures
    - Test duration is calculated correctly
    - Test verbose mode includes per-prefix breakdown
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

- [ ] 4. Implement Audit Statistics
  - [ ] 4.1 Create stats aggregator
    - Create `internal/audit/stats.go`
    - Add `AuditStats` and `StatsOptions` types
    - Implement `AggregateStats(logDir string, opts StatsOptions)`
    - _Requirements: 4.1_

  - [ ] 4.2 Implement metric aggregation
    - Count total organized, for-review, runs, undos
    - Track per-prefix counts (top N)
    - Calculate date range
    - _Requirements: 4.2, 4.3, 4.4, 4.5, 4.6_

  - [ ] 4.3 Implement time filtering
    - Support --since flag for filtering
    - Only include runs after specified time
    - _Requirements: 4.7_

  - [ ] 4.4 Write property test for stats aggregation
    - **Property: Stats Totals Equal Sum of Parts**
    - Generate audit logs with random events
    - Verify total organized equals sum of per-prefix counts
    - Verify total for-review equals sum across runs
    - Verify --since filtering excludes older runs
    - _Rationale: Mathematical invariant - aggregated totals must equal sum of components_
    - **Validates: Requirements 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7**

- [ ] 5. Update CLI
  - [ ] 5.1 Add watch command
    - Implement `watch` subcommand
    - Parse `--debounce N` flag
    - Start watcher with configured directories
    - Handle interrupt signal for graceful shutdown
    - Display summary on exit
    - _Requirements: 1.1, 1.6, 1.7, 2.5_

  - [ ] 5.2 Add audit stats command
    - Implement `audit stats` subcommand
    - Parse `--since` flag
    - Display aggregated statistics
    - _Requirements: 4.1, 4.7_

- [ ] 6. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Watch mode uses fsnotify library for cross-platform file watching
- Debounce and stability checks prevent processing incomplete files
- Stats aggregation reads all audit log files in the configured directory
- Property test retained for stats aggregation (mathematical invariant: totals = sum of parts)
- Unit tests used elsewhere as examples provide sufficient coverage
