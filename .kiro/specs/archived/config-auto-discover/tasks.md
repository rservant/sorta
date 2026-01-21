# Implementation Plan: Config Auto-Discover

## Overview

This implementation adds configuration management commands and auto-discovery capabilities to Sorta. The work is organized to build incrementally: first updating the config module, then adding the discovery engine, then refactoring the CLI, and finally updating the organizer for new behaviors.

## Tasks

- [x] 1. Update Configuration Module
  - [x] 1.1 Remove forReviewDirectory from Configuration struct and update validation
    - Remove `ForReviewDirectory` field from `Configuration` struct
    - Update `Validate()` to not require forReviewDirectory
    - Update JSON tags accordingly
    - _Requirements: 9.2_

  - [x] 1.2 Add helper methods to Configuration
    - Implement `HasPrefix(prefix string) bool` for case-insensitive prefix check
    - Implement `AddPrefixRule(rule PrefixRule) bool` that returns false if duplicate
    - Implement `HasSourceDirectory(dir string) bool`
    - Implement `AddSourceDirectory(dir string) bool` that returns false if duplicate
    - _Requirements: 3.4, 7.3, 7.4_

  - [x] 1.3 Add LoadOrCreate function
    - Implement `LoadOrCreate(filePath string) (*Configuration, error)` that returns empty config if file doesn't exist
    - _Requirements: 3.3, 7.2_

  - [x] 1.4 Write property test for source directory duplicate prevention
    - **Property 3: Source Directory Duplicate Prevention**
    - **Validates: Requirements 3.4**

  - [x] 1.5 Write property test for prefix rule duplicate prevention
    - **Property 7: Prefix Rule Duplicate Prevention**
    - **Validates: Requirements 7.3, 7.4, 7.5**

  - [x] 1.6 Write property test for configuration round-trip
    - **Property 11: Configuration Round-Trip**
    - **Validates: Requirements 11.2**

- [x] 2. Implement Pattern Detection
  - [x] 2.1 Create pattern detection module
    - Create `internal/discovery/pattern.go`
    - Implement `ExtractPrefixFromFilename(filename string) (prefix string, matched bool)`
    - Use regex: `^([A-Za-z][A-Za-z0-9]*)\s+(\d{4}-\d{2}-\d{2})\s+(.+)$`
    - _Requirements: 6.2, 6.4_

  - [x] 2.2 Write property test for prefix extraction
    - **Property 6: Prefix Extraction**
    - **Validates: Requirements 6.2, 6.3, 6.4**

- [x] 3. Implement Discovery Engine
  - [x] 3.1 Create discovery module structure
    - Create `internal/discovery/discovery.go`
    - Define `DiscoveredRule` and `DiscoveryResult` types
    - _Requirements: 4.1_

  - [x] 3.2 Implement target directory candidate scanning
    - Implement `scanTargetCandidates(scanDir string) ([]string, error)`
    - Return only immediate subdirectories
    - _Requirements: 5.1, 5.2_

  - [x] 3.3 Implement recursive file analysis
    - Implement `analyzeDirectory(dir string) ([]string, error)` to find all prefixes
    - Recursively scan all files within the directory
    - Use pattern detection to extract prefixes
    - Return unique prefixes found
    - _Requirements: 6.1, 6.3_

  - [x] 3.4 Implement main Discover function
    - Implement `Discover(scanDir string, existingConfig *Configuration) (*DiscoveryResult, error)`
    - Scan candidates, analyze each, filter duplicates
    - _Requirements: 5.3, 6.5, 7.3, 7.4, 7.5_

  - [x] 3.5 Write property test for candidate directory detection
    - **Property 4: Candidate Directory Detection**
    - **Validates: Requirements 5.1, 5.2, 5.3**

  - [x] 3.6 Write property test for recursive file analysis
    - **Property 5: Recursive File Analysis**
    - **Validates: Requirements 6.1**

- [x] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Implement Duplicate File Handling
  - [x] 5.1 Create duplicate handling module
    - Create `internal/organizer/duplicate.go`
    - Implement `FileExists(path string) bool`
    - Implement `GenerateDuplicateName(destDir, filename string) string`
    - Handle `_duplicate` and `_duplicate_N` naming
    - _Requirements: 10.1, 10.2, 10.3_

  - [x] 5.2 Write property test for duplicate file naming
    - **Property 10: Duplicate File Naming**
    - **Validates: Requirements 10.1, 10.2, 10.3, 10.4, 10.5**

- [x] 6. Implement For-Review Subdirectory Logic
  - [x] 6.1 Add for-review path generation
    - Add `GetForReviewPath(sourceDir string) string` function
    - Update organizer to use per-source for-review directories
    - _Requirements: 9.1, 9.3, 9.4_

  - [x] 6.2 Write property test for for-review path generation
    - **Property 9: For-Review Path Generation**
    - **Validates: Requirements 9.1, 9.3**

- [x] 7. Refactor CLI for Subcommands
  - [x] 7.1 Implement CLI argument parser
    - Create argument parsing logic for subcommands
    - Implement `-c`/`--config` flag handling with default `sorta-config.json`
    - Implement `parseArgs(args []string) (command string, cmdArgs []string, configPath string, err error)`
    - _Requirements: 1.1, 1.2, 1.3, 1.4_

  - [x] 7.2 Write property test for config flag parsing
    - **Property 1: Config Flag Parsing**
    - **Validates: Requirements 1.2**

  - [x] 7.3 Implement config command
    - Display source directories
    - Display prefix rules with prefixes and target directories
    - Handle missing/invalid config errors
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_

  - [x] 7.4 Write property test for config display completeness
    - **Property 2: Config Display Completeness**
    - **Validates: Requirements 2.2, 2.3**

  - [x] 7.5 Implement add-source command
    - Add directory to sourceDirectories
    - Create new config if doesn't exist
    - Skip duplicates
    - Save updated config
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

  - [x] 7.6 Implement discover command
    - Call discovery engine
    - Display results (new rules, skipped rules)
    - Update and save config
    - _Requirements: 4.1, 4.2, 4.3, 8.1, 8.2, 8.3, 8.4_

  - [x] 7.7 Write property test for discovery output completeness
    - **Property 8: Discovery Output Completeness**
    - **Validates: Requirements 8.2, 8.4**

  - [x] 7.8 Implement run command
    - Migrate existing orchestrator logic to run subcommand
    - Update to use new for-review subdirectory logic
    - Update to use duplicate file handling
    - _Requirements: 10.4, 10.5_

- [x] 8. Update Orchestrator and Organizer
  - [x] 8.1 Update orchestrator to use new config structure
    - Remove dependency on forReviewDirectory
    - Use GetForReviewPath for each source directory
    - _Requirements: 9.1, 9.3_

  - [x] 8.2 Update organizer to handle duplicates
    - Check for existing files before moving
    - Use GenerateDuplicateName when conflict detected
    - Report duplicate moves in results
    - _Requirements: 10.1, 10.4, 10.5_

- [x] 9. Final Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks including property-based tests are required
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
- The `gopter` library should be used for property-based testing in Go
