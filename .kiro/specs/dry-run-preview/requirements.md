# Requirements Document

## Introduction

This feature adds preview capabilities to Sorta, allowing users to see what would happen during file organization without actually moving files. It includes a `--dry-run` flag for the run command and a new `status` command to show pending files across all inbound directories.

## Glossary

- **Sorta**: The file organization program that processes and moves files based on configuration rules
- **Dry_Run**: A mode where operations are simulated but not executed
- **Pending_File**: A file in an inbound directory that matches organization rules but hasn't been moved yet
- **Preview_Output**: The formatted display of what operations would occur
- **Status_Report**: A summary of pending files across all configured inbound directories

## Requirements

### Requirement 1: Dry Run Mode

**User Story:** As a user, I want to preview what files would be moved without actually moving them, so that I can verify the organization rules before committing.

#### Acceptance Criteria

1. WHEN the user runs `sorta run --dry-run`, THE Sorta SHALL simulate all file operations without modifying the filesystem
2. WHEN in dry-run mode, THE Sorta SHALL display each file that would be moved along with its destination path
3. WHEN in dry-run mode, THE Sorta SHALL display files that would go to for-review directories
4. THE Sorta SHALL NOT create any directories during dry-run mode
5. THE Sorta SHALL NOT write to the audit log during dry-run mode
6. WHEN dry-run completes, THE Sorta SHALL display a summary count of files that would be moved, reviewed, and skipped

### Requirement 2: Status Command

**User Story:** As a user, I want to see all pending files across my inbound directories, so that I can understand what needs to be organized.

#### Acceptance Criteria

1. WHEN the user runs `sorta status`, THE Sorta SHALL scan all configured inbound directories
2. THE Sorta SHALL display pending files grouped by their destination (matched prefix or for-review)
3. THE Sorta SHALL display the total count of pending files per inbound directory
4. THE Sorta SHALL display a grand total of all pending files
5. WHEN no pending files exist, THE Sorta SHALL display a message indicating all directories are organized
6. THE Sorta SHALL NOT modify any files or directories when running status

### Requirement 3: Output Formatting

**User Story:** As a user, I want clear and readable output from preview operations, so that I can easily understand the results.

#### Acceptance Criteria

1. WHEN displaying dry-run results, THE Sorta SHALL show source path and destination path for each file
2. WHEN displaying status results, THE Sorta SHALL group files by destination directory
3. THE Sorta SHALL use consistent formatting with the existing verbose output style
4. WHEN verbose mode is enabled with dry-run, THE Sorta SHALL show additional details about rule matching
