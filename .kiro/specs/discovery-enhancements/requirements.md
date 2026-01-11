# Requirements Document

## Introduction

This feature enhances the discovery command with depth limiting to prevent accidental deep crawls and an interactive mode that prompts users to accept or reject each detected rule.

## Glossary

- **Sorta**: The file organization program that processes and moves files based on configuration rules
- **Discovery_Engine**: The component that scans directories to auto-detect prefix rules from existing file structures
- **Depth_Limit**: The maximum number of directory levels to descend during scanning
- **Interactive_Mode**: A mode where the user is prompted to confirm each discovered rule
- **Target_Candidate**: An immediate subdirectory of the scan directory that may contain organized files

## Requirements

### Requirement 1: Discovery Depth Limiting

**User Story:** As a user, I want to limit how deep discovery scans, so that I don't accidentally crawl huge nested directory structures.

#### Acceptance Criteria

1. WHEN the user runs `sorta discover --depth N`, THE Discovery_Engine SHALL only scan directories up to N levels deep within each target candidate
2. THE Discovery_Engine SHALL default to unlimited depth when no --depth flag is provided
3. WHEN depth is set to 0, THE Discovery_Engine SHALL only scan files in the immediate target candidate directory
4. WHEN depth is set to 1, THE Discovery_Engine SHALL scan the target candidate and its immediate subdirectories
5. THE Discovery_Engine SHALL still skip ISO-date directories regardless of depth setting
6. WHEN files exist beyond the depth limit, THE Discovery_Engine SHALL not analyze them for prefixes

### Requirement 2: Interactive Discovery Mode

**User Story:** As a user, I want to interactively approve or reject discovered rules, so that I have fine-grained control over what gets added to my configuration.

#### Acceptance Criteria

1. WHEN the user runs `sorta discover --interactive`, THE Sorta SHALL prompt for each discovered rule
2. THE Sorta SHALL display the prefix and target directory for each rule before prompting
3. WHEN the user accepts a rule, THE Sorta SHALL add it to the configuration
4. WHEN the user rejects a rule, THE Sorta SHALL skip it without adding to configuration
5. THE Sorta SHALL provide options to accept all remaining, reject all remaining, or quit
6. WHEN not in interactive mode, THE Sorta SHALL add all discovered rules automatically (current behavior)
7. IF the terminal is not interactive, THEN THE Sorta SHALL fall back to non-interactive mode with a warning

### Requirement 3: Combined Options

**User Story:** As a user, I want to combine depth limiting with interactive mode, so that I can have full control over the discovery process.

#### Acceptance Criteria

1. THE Sorta SHALL allow --depth and --interactive flags to be used together
2. WHEN both flags are used, THE Discovery_Engine SHALL apply depth limiting first, then prompt interactively for results
