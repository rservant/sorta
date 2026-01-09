# Implementation Plan: Sorta — Prefix-and-Date File Organizer

## Overview

This plan implements Sorta in Go, building components incrementally from configuration loading through file organization. Each task builds on previous work, with property-based tests validating correctness properties from the design.

## Tasks

- [x] 1. Set up project structure and dependencies
  - Initialize Go module
  - Set up directory structure: `cmd/sorta/`, `internal/config/`, `internal/scanner/`, `internal/matcher/`, `internal/classifier/`, `internal/organizer/`
  - Add testing dependencies (gopter for property-based testing)
  - _Requirements: 1.1_

- [x] 2. Implement configuration types and loader
  - [x] 2.1 Define Configuration types
    - Create `PrefixRule` struct with `Prefix` and `TargetDirectory` fields
    - Create `Configuration` struct with `SourceDirectories`, `PrefixRules`, and `ForReviewDirectory` fields
    - Define `ConfigError` types for FILE_NOT_FOUND, INVALID_JSON, VALIDATION_ERROR
    - _Requirements: 1.4, 1.5, 1.6_

  - [x] 2.2 Implement ConfigLoader
    - Implement `Load(filePath string) (*Configuration, error)` function
    - Implement `Save(config *Configuration, filePath string) error` function
    - Add validation for required fields and non-empty lists
    - _Requirements: 1.1, 1.2, 1.3, 8.1, 8.2_

  - [x] 2.3 Write property test for configuration round-trip
    - **Property 1: Configuration Round-Trip**
    - Generate random valid Configuration objects
    - Verify serialize then parse produces equivalent object
    - **Validates: Requirements 8.3**

- [x] 3. Implement ISO date parser
  - [x] 3.1 Define IsoDate type and parser
    - Create `IsoDate` struct with Year, Month, Day fields
    - Implement `ParseIsoDate(segment string) (*IsoDate, error)` function
    - Validate YYYY-MM-DD format strictly
    - Validate month range (01-12) and day range for each month
    - Handle leap year validation for February
    - _Requirements: 4.1, 4.2, 4.3_

  - [x] 3.2 Write property test for valid date extraction
    - **Property 4: Valid ISO Date Extraction**
    - Generate random valid dates across full range
    - Verify year, month, day extraction is correct
    - **Validates: Requirements 4.1, 4.3**

- [x] 4. Implement filename matcher
  - [x] 4.1 Implement FilenameMatcher
    - Create `MatchResult` type with matched flag, rule, and remainder
    - Implement `Match(filename string, rules []PrefixRule) *MatchResult` function
    - Sort rules by prefix length descending for longest-match-first
    - Perform case-insensitive prefix comparison
    - Verify single space delimiter after prefix
    - _Requirements: 3.1, 3.2, 3.3, 3.4_

  - [x] 4.2 Write property test for case-insensitive prefix matching
    - **Property 2: Case-Insensitive Prefix Matching**
    - Generate filenames with random casing variations of prefixes
    - Verify match succeeds regardless of casing
    - **Validates: Requirements 3.1, 3.2, 3.3**

  - [x] 4.3 Write property test for longest prefix wins
    - **Property 3: Longest Prefix Wins**
    - Generate overlapping prefix rules and matching filenames
    - Verify longest prefix is selected
    - **Validates: Requirements 3.4**

- [x] 5. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Implement filename normalizer
  - [x] 6.1 Implement FilenameNormalizer
    - Implement `Normalize(filename string, matchedPrefix string, canonicalPrefix string) string` function
    - Replace matched prefix portion with canonical casing
    - Preserve space delimiter and remainder exactly
    - _Requirements: 5.3, 5.4_

  - [x] 6.2 Write property test for filename normalization
    - **Property 6: Filename Normalization Preserves Structure**
    - Generate filenames with non-canonical prefix casing
    - Verify prefix is replaced and remainder preserved exactly
    - **Validates: Requirements 5.3, 5.4**

- [x] 7. Implement file classifier
  - [x] 7.1 Implement FileClassifier
    - Create `Classification` type with CLASSIFIED and UNCLASSIFIED variants
    - Create `UnclassifiedReason` enum (NO_PREFIX_MATCH, MISSING_DELIMITER, INVALID_DATE)
    - Implement `Classify(filename string, rules []PrefixRule) *Classification` function
    - Integrate matcher, date parser, and normalizer
    - Return CLASSIFIED with year, normalised filename, target directory for valid files
    - Return UNCLASSIFIED with reason for invalid files
    - _Requirements: 4.4, 6.1, 6.2, 6.3_

  - [x] 7.2 Write property test for invalid date classification
    - **Property 5: Invalid Date Classification**
    - Generate filenames with malformed or missing dates
    - Verify UNCLASSIFIED result with INVALID_DATE reason
    - **Validates: Requirements 4.4**

  - [x] 7.3 Write property test for deterministic classification
    - **Property 10: Deterministic Classification**
    - Generate random filenames and configurations
    - Run classification multiple times, verify identical results
    - **Validates: Requirements 7.5**

- [x] 8. Implement directory scanner
  - [x] 8.1 Implement DirectoryScanner
    - Create `FileEntry` struct with Name and FullPath fields
    - Implement `Scan(directory string) ([]FileEntry, error)` function
    - Return only files, exclude subdirectories
    - Handle DIRECTORY_NOT_FOUND and PERMISSION_DENIED errors
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

  - [x] 8.2 Write property test for scanner returns only files
    - **Property 9: Scanner Returns Only Files**
    - Generate directory structures with mixed files and subdirectories
    - Verify only files are returned
    - **Validates: Requirements 2.3**

- [x] 9. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 10. Implement file organizer
  - [x] 10.1 Implement FileOrganizer
    - Create `MoveResult` struct with SourcePath and DestinationPath
    - Implement `Organize(file FileEntry, classification *Classification, config *Configuration) (*MoveResult, error)` function
    - For CLASSIFIED: build path `<targetDir>/<year> <prefix>/`, create directories, move file
    - For UNCLASSIFIED: move to ForReviewDirectory with original filename
    - Handle file move errors gracefully
    - _Requirements: 5.1, 5.2, 5.5, 6.4, 6.5_

  - [x] 10.2 Write property test for unclassified filename preservation
    - **Property 7: Unclassified Filename Preservation**
    - Generate non-matching filenames
    - Verify destination filename equals source filename exactly
    - **Validates: Requirements 6.4**

  - [x] 10.3 Write property test for file content integrity
    - **Property 8: File Content Integrity**
    - Generate files with random content
    - Process and verify byte-for-byte equality via hash comparison
    - **Validates: Requirements 7.1**

- [x] 11. Implement main orchestrator
  - [x] 11.1 Create Sorta orchestrator
    - Implement `Run(configPath string) error` function
    - Load configuration
    - Scan all source directories
    - Classify and organize each file
    - Collect and report results (successes and errors)
    - _Requirements: 1.1, 2.1, 7.3, 7.4_

  - [x] 11.2 Create CLI entry point
    - Create `cmd/sorta/main.go`
    - Parse command-line arguments for config file path
    - Call orchestrator and handle exit codes
    - _Requirements: 1.1_

- [x] 12. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks including property tests are required
- Property-based tests use gopter library with minimum 100 iterations
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
