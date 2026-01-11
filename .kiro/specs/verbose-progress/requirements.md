# Requirements Document

## Introduction

This feature adds a verbose output option (`-v`/`--verbose`) to all sorta commands and a progress indicator when running in non-verbose mode. The verbose mode provides detailed operational information for debugging and transparency, while the progress indicator gives users feedback during longer operations without cluttering the output.

## Glossary

- **CLI**: Command Line Interface - the sorta command-line application
- **Verbose_Mode**: A flag-enabled mode that outputs detailed information about each operation
- **Progress_Indicator**: A visual feedback mechanism showing operation progress (e.g., spinner, progress bar, or status updates)
- **Command**: A sorta subcommand (config, add-source, discover, run, audit, undo)

## Requirements

### Requirement 1: Global Verbose Flag

**User Story:** As a user, I want to enable verbose output for any command, so that I can see detailed information about what sorta is doing.

#### Acceptance Criteria

1. THE CLI SHALL accept `-v` and `--verbose` flags before any command
2. WHEN the verbose flag is provided, THE CLI SHALL pass verbose mode to the executed command
3. THE CLI SHALL display the verbose flag in help output alongside existing flags

### Requirement 2: Verbose Output for Run Command

**User Story:** As a user, I want to see detailed progress when running file organization in verbose mode, so that I can understand exactly what operations are being performed.

#### Acceptance Criteria

1. WHEN verbose mode is enabled during the run command, THE CLI SHALL display each file being processed with its source path
2. WHEN verbose mode is enabled and a file is moved, THE CLI SHALL display the source and destination paths
3. WHEN verbose mode is enabled and a file is skipped, THE CLI SHALL display the skip reason
4. WHEN verbose mode is enabled and a file is routed to review, THE CLI SHALL display the review routing reason
5. WHEN verbose mode is enabled and an error occurs, THE CLI SHALL display detailed error information

### Requirement 3: Verbose Output for Discover Command

**User Story:** As a user, I want to see detailed discovery progress in verbose mode, so that I can understand what patterns are being detected.

#### Acceptance Criteria

1. WHEN verbose mode is enabled during discover, THE CLI SHALL display each directory being scanned
2. WHEN verbose mode is enabled during discover, THE CLI SHALL display each file being analyzed
3. WHEN verbose mode is enabled during discover, THE CLI SHALL display detected patterns as they are found

### Requirement 4: Verbose Output for Undo Command

**User Story:** As a user, I want to see detailed undo progress in verbose mode, so that I can track which files are being restored.

#### Acceptance Criteria

1. WHEN verbose mode is enabled during undo, THE CLI SHALL display each file being restored with source and destination
2. WHEN verbose mode is enabled during undo, THE CLI SHALL display skip reasons for files that cannot be restored
3. WHEN verbose mode is enabled during undo, THE CLI SHALL display verification status for file identity checks

### Requirement 5: Progress Indicator for Non-Verbose Mode

**User Story:** As a user, I want to see progress feedback during long operations without verbose output, so that I know the application is working.

#### Acceptance Criteria

1. WHEN verbose mode is NOT enabled during the run command, THE CLI SHALL display a progress indicator
2. WHEN verbose mode is NOT enabled during the discover command, THE CLI SHALL display a progress indicator
3. WHEN verbose mode is NOT enabled during the undo command, THE CLI SHALL display a progress indicator
4. THE Progress_Indicator SHALL show the current operation count and total (e.g., "Processing file 5/20...")
5. THE Progress_Indicator SHALL update in place without scrolling the terminal (using carriage return)
6. WHEN the operation completes, THE CLI SHALL clear the progress indicator and show the final summary

### Requirement 6: Terminal Detection

**User Story:** As a user running sorta in scripts or pipes, I want progress indicators to be suppressed automatically, so that output remains clean for parsing.

#### Acceptance Criteria

1. WHEN stdout is not a terminal (TTY), THE CLI SHALL suppress progress indicators
2. WHEN stdout is not a terminal, THE CLI SHALL still output final summaries and results
3. WHEN verbose mode is enabled, THE CLI SHALL output verbose information regardless of TTY status
