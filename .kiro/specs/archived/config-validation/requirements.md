# Requirements Document

## Introduction

This feature adds configuration validation capabilities to Sorta, including path existence checking, a dedicated validation command, and a configurable symlink handling policy.

## Glossary

- **Sorta**: The file organization program that processes and moves files based on configuration rules
- **Configuration**: The JSON file containing inbound directories, prefix rules, and settings
- **Validation_Report**: A summary of configuration issues found during validation
- **Symlink**: A symbolic link to a file or directory
- **Symlink_Policy**: The configured behavior for handling symbolic links (follow, skip, or error)

## Requirements

### Requirement 1: Configuration Validation Command

**User Story:** As a user, I want to validate my configuration file, so that I can catch errors before running organization.

#### Acceptance Criteria

1. WHEN the user runs `sorta config --validate`, THE Sorta SHALL check the configuration for errors
2. THE Sorta SHALL verify that all inbound directories exist and are accessible
3. THE Sorta SHALL verify that all outbound directories exist or can be created
4. THE Sorta SHALL check for duplicate prefix rules (case-insensitive)
5. THE Sorta SHALL check for overlapping outbound directories that could cause conflicts
6. WHEN validation passes, THE Sorta SHALL display a success message
7. WHEN validation fails, THE Sorta SHALL display all errors found with clear descriptions
8. THE Sorta SHALL return a non-zero exit code when validation fails

### Requirement 2: Symlink Handling Policy

**User Story:** As a user, I want to configure how Sorta handles symbolic links, so that I can control behavior based on my filesystem setup.

#### Acceptance Criteria

1. THE Configuration SHALL support a `symlinkPolicy` field with values: "follow", "skip", or "error"
2. WHEN symlinkPolicy is "follow", THE Sorta SHALL follow symlinks and process the target file/directory
3. WHEN symlinkPolicy is "skip", THE Sorta SHALL ignore symlinks during scanning
4. WHEN symlinkPolicy is "error", THE Sorta SHALL report an error when encountering a symlink
5. THE Sorta SHALL default to "skip" when no symlinkPolicy is configured
6. WHEN validating configuration, THE Sorta SHALL verify symlinkPolicy has a valid value

### Requirement 3: Scan Depth Limiting

**User Story:** As a user, I want to limit how deep Sorta scans inbound directories, so that I can prevent accidental processing of deeply nested structures.

#### Acceptance Criteria

1. THE Configuration SHALL support a `scanDepth` field specifying maximum directory depth to scan
2. WHEN scanDepth is 0, THE Scanner SHALL only process files in the immediate inbound directory
3. WHEN scanDepth is 1, THE Scanner SHALL process files in the inbound directory and its immediate subdirectories
4. WHEN scanDepth is not configured, THE Scanner SHALL default to 0 (immediate directory only)
5. WHEN the user runs `sorta run --depth N`, THE Sorta SHALL override the configured scanDepth
6. THE Scanner SHALL not descend into directories beyond the configured depth
7. WHEN validating configuration, THE Sorta SHALL verify scanDepth is a non-negative integer

### Requirement 4: Path Validation During Run

**User Story:** As a user, I want Sorta to validate paths before starting organization, so that I get early feedback on configuration issues.

#### Acceptance Criteria

1. WHEN running `sorta run`, THE Sorta SHALL validate inbound directories exist before processing
2. IF an inbound directory does not exist, THEN THE Sorta SHALL report an error and skip that directory
3. THE Sorta SHALL create outbound directories as needed (existing behavior)
4. WHEN verbose mode is enabled, THE Sorta SHALL report which directories were validated
