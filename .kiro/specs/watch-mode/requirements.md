# Requirements Document

## Introduction

This feature adds a watch mode to Sorta that monitors inbound directories for new files and automatically organizes them as they arrive. It also adds summary statistics after run operations and aggregate audit metrics.

## Glossary

- **Sorta**: The file organization program that processes and moves files based on configuration rules
- **Watch_Mode**: A long-running process that monitors directories for file changes
- **File_Event**: A notification that a file has been created, modified, or moved
- **Debounce_Period**: A delay before processing to allow file writes to complete
- **Run_Summary**: Statistics displayed after a run operation completes
- **Audit_Stats**: Aggregate metrics across all audit runs

## Requirements

### Requirement 1: Watch Mode

**User Story:** As a user, I want Sorta to automatically organize files as they arrive, so that I don't have to manually run the command.

#### Acceptance Criteria

1. WHEN the user runs `sorta watch`, THE Sorta SHALL monitor all configured inbound directories
2. WHEN a new file is detected in an inbound directory, THE Sorta SHALL organize it according to rules
3. THE Sorta SHALL wait for a configurable debounce period before processing (default 2 seconds)
4. THE Sorta SHALL handle files that are still being written (wait for stable file size)
5. WHEN a file is organized, THE Sorta SHALL log the operation to the audit trail
6. THE Sorta SHALL continue running until interrupted (Ctrl+C)
7. WHEN interrupted, THE Sorta SHALL gracefully shut down and display a summary
8. THE Sorta SHALL ignore temporary files (patterns like .tmp, .part, .download)

### Requirement 2: Watch Mode Configuration

**User Story:** As a user, I want to configure watch mode behavior, so that I can tune it for my workflow.

#### Acceptance Criteria

1. THE Configuration SHALL support a `watch` section with settings
2. THE watch configuration SHALL include `debounceSeconds` (default: 2)
3. THE watch configuration SHALL include `ignorePatterns` for files to skip
4. THE watch configuration SHALL include `stableThresholdMs` for file stability detection (default: 1000)
5. WHEN the user runs `sorta watch --debounce N`, THE Sorta SHALL override the configured debounce period

### Requirement 3: Run Summary Statistics

**User Story:** As a user, I want to see a summary after running organization, so that I know what happened.

#### Acceptance Criteria

1. WHEN `sorta run` completes, THE Sorta SHALL display summary statistics
2. THE summary SHALL include count of files moved to organized destinations
3. THE summary SHALL include count of files moved to for-review
4. THE summary SHALL include count of files skipped (already organized, errors, etc.)
5. THE summary SHALL include total processing time
6. WHEN verbose mode is enabled, THE summary SHALL include per-prefix breakdown

### Requirement 4: Audit Statistics Command

**User Story:** As a user, I want to see aggregate statistics across all runs, so that I can understand my organization patterns over time.

#### Acceptance Criteria

1. WHEN the user runs `sorta audit stats`, THE Sorta SHALL display aggregate metrics
2. THE stats SHALL include total files organized across all runs
3. THE stats SHALL include files organized per prefix (top N)
4. THE stats SHALL include files sent to for-review across all runs
5. THE stats SHALL include total runs and date range
6. THE stats SHALL include undo operations performed
7. WHEN --since flag is provided, THE Sorta SHALL filter stats to that time period
