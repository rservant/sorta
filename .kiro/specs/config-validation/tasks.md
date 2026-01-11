# Implementation Plan: Config Validation

## Overview

This implementation adds configuration validation, symlink handling policy, and scan depth limiting. Changes span config, scanner, and CLI modules.

## Tasks

- [ ] 1. Implement Configuration Validator
  - [ ] 1.1 Create validator component
    - Create `internal/config/validator.go`
    - Add `ValidationError` and `ValidationResult` types
    - Implement `Validate(cfg *Configuration) *ValidationResult`
    - _Requirements: 1.1_

  - [ ] 1.2 Implement path existence validation
    - Implement `ValidatePaths(cfg *Configuration) []ValidationError`
    - Check inbound directories exist and are accessible
    - Check outbound directories exist or parent is writable
    - _Requirements: 1.2, 1.3_

  - [ ] 1.3 Implement prefix rule validation
    - Implement `ValidatePrefixRules(cfg *Configuration) []ValidationError`
    - Detect duplicate prefixes (case-insensitive)
    - Detect overlapping outbound directories
    - _Requirements: 1.4, 1.5_

  - [ ]* 1.4 Write property test for validation error reporting
    - **Property 1: Validation Reports All Errors**
    - **Validates: Requirements 1.1, 1.7, 1.8**

  - [ ]* 1.5 Write property test for path validation
    - **Property 2: Path Existence Validation**
    - **Validates: Requirements 1.2, 1.3**

  - [ ]* 1.6 Write property test for duplicate/overlap detection
    - **Property 3: Duplicate and Overlap Detection**
    - **Validates: Requirements 1.4, 1.5**

- [ ] 2. Implement Symlink Policy
  - [ ] 2.1 Add symlinkPolicy to configuration
    - Add `SymlinkPolicy` field to `Configuration` struct
    - Add `GetSymlinkPolicy()` method with "skip" default
    - Validate policy value in validator
    - _Requirements: 2.1, 2.5, 2.6_

  - [ ] 2.2 Update scanner for symlink handling
    - Add `ScanOptions` struct with `SymlinkPolicy` field
    - Implement `ScanWithOptions(dir string, opts ScanOptions)`
    - Handle "follow", "skip", "error" policies
    - _Requirements: 2.2, 2.3, 2.4_

  - [ ]* 2.3 Write property test for symlink policy validation
    - **Property 4: Symlink Policy Validation**
    - **Validates: Requirements 2.1, 2.6**

  - [ ]* 2.4 Write property test for symlink policy behavior
    - **Property 5: Symlink Policy Behavior**
    - **Validates: Requirements 2.2, 2.3, 2.4**

- [ ] 3. Implement Scan Depth Limiting
  - [ ] 3.1 Add scanDepth to configuration
    - Add `ScanDepth` field to `Configuration` struct
    - Add `GetScanDepth()` method with 0 default
    - Validate scanDepth is non-negative
    - _Requirements: 3.1, 3.4, 3.7_

  - [ ] 3.2 Update scanner for depth limiting
    - Add `MaxDepth` to `ScanOptions`
    - Implement depth tracking in scanner
    - Skip directories beyond configured depth
    - _Requirements: 3.2, 3.3, 3.6_

  - [ ]* 3.3 Write property test for scan depth limiting
    - **Property 6: Scan Depth Limiting**
    - **Validates: Requirements 3.1, 3.2, 3.5, 3.6, 3.7**

- [ ] 4. Update CLI
  - [ ] 4.1 Add --validate flag to config command
    - Parse `--validate` flag
    - Run validator and display results
    - Return non-zero exit code on failure
    - _Requirements: 1.1, 1.6, 1.7, 1.8_

  - [ ] 4.2 Add --depth flag to run command
    - Parse `--depth N` flag
    - Override configured scanDepth
    - Pass to scanner via options
    - _Requirements: 3.5_

  - [ ] 4.3 Add runtime path validation to run command
    - Validate inbound directories before processing
    - Skip non-existent directories with warning
    - Continue with remaining directories
    - _Requirements: 4.1, 4.2, 4.3, 4.4_

  - [ ]* 4.4 Write property test for runtime path validation
    - **Property 7: Runtime Path Validation**
    - **Validates: Requirements 4.1, 4.2**

- [ ] 5. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are property-based tests
- Symlink tests require creating actual symlinks in test fixtures
- Scan depth 0 means immediate directory only (current default behavior)
