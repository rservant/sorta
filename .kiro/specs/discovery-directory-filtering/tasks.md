# Implementation Plan: Discovery Directory Filtering

## Overview

This implementation refines the discovery engine to extract prefixes only from files and skip ISO-date directories during scanning. The changes are localized to the discovery module with minimal impact on existing functionality.

## Tasks

- [x] 1. Implement ISO Date Directory Detection
  - [x] 1.1 Add IsISODateDirectory function to pattern.go
    - Add `ISODateDirPattern` regex matching `^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])`
    - Implement `IsISODateDirectory(dirName string) bool`
    - Return true if directory name starts with valid ISO date
    - _Requirements: 2.3_

  - [x] 1.2 Write property test for ISO date pattern detection
    - **Property 3: ISO Date Pattern Detection**
    - **Validates: Requirements 2.3**

  - [x] 1.3 Write unit tests for IsISODateDirectory
    - Test valid ISO dates: `2024-01-15`, `2024-01-15 Some Folder`
    - Test invalid patterns: `Invoice 2024-01-15`, `2024-13-01`, `2024-01-32`, `20240115`
    - _Requirements: 2.3_

- [x] 2. Update Discovery Engine for Directory Filtering
  - [x] 2.1 Update analyzeDirectoryWithCallback to skip ISO-date directories
    - Modify filepath.Walk callback to check directory names
    - Return `filepath.SkipDir` for directories starting with ISO dates
    - Ensure files in root of target candidate are still analyzed
    - _Requirements: 2.1, 2.2, 3.1, 3.2_

  - [x] 2.2 Write property test for ISO-date directory filtering
    - **Property 2: ISO-Date Directory Filtering**
    - **Validates: Requirements 2.1, 2.2, 3.1, 3.2, 3.3**

  - [x] 2.3 Write unit tests for directory filtering behavior
    - Test files in ISO-date subdirectories are skipped
    - Test files in non-ISO-date subdirectories are analyzed
    - Test nested directory structures
    - _Requirements: 2.1, 2.2, 3.1, 3.2, 3.3_

- [x] 3. Verify File-Only Prefix Extraction
  - [x] 3.1 Add unit tests to verify directories don't produce prefixes
    - Create test with directory named "Invoice 2024-01-15 Vendor"
    - Verify "Invoice" prefix is NOT extracted from directory name
    - Verify "Invoice" prefix IS extracted from file with same pattern
    - _Requirements: 1.1, 1.2, 1.3_

  - [x] 3.2 Write property test for file-only prefix extraction
    - **Property 1: File-Only Prefix Extraction**
    - **Validates: Requirements 1.1, 1.2, 1.3**

- [x] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks including property-based tests are required
- The existing `analyzeDirectory` function already only extracts prefixes from files (via `info.IsDir()` check), but we add explicit tests to verify this behavior
- The main change is adding the ISO-date directory skip logic in the filepath.Walk callback
- The `gopter` library should be used for property-based testing in Go
