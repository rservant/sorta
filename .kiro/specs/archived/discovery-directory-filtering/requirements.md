# Requirements Document

## Introduction

This feature refines the auto-discovery behavior in Sorta to improve prefix detection accuracy. The discovery process should extract prefixes only from files, not from directory names. Additionally, directories that start with an ISO date should be skipped during scanning to avoid false positives from date-organized folder structures.

## Glossary

- **Sorta**: The file organization program that processes and moves files based on configuration rules
- **Discovery_Engine**: The component that scans directories to auto-detect prefix rules from existing file structures
- **Scan_Directory**: The root directory provided to the discover command for scanning
- **Target_Directory_Candidate**: An immediate subdirectory of the Scan_Directory that may contain organized files
- **Prefix_Pattern**: The filename pattern `<prefix> <ISO_Date> <other_info>` used to identify file organization
- **ISO_Date**: A date in YYYY-MM-DD format (e.g., 2024-01-15)
- **ISO_Date_Directory**: A directory whose name starts with an ISO date pattern
- **Prefix_Source**: The origin from which a prefix is extracted (files only, not directories)

## Requirements

### Requirement 1: Prefix Extraction Source Restriction

**User Story:** As a user, I want prefixes to be extracted only from filenames, so that directory names don't create false prefix rules.

#### Acceptance Criteria

1. WHEN analyzing a directory for prefixes, THE Discovery_Engine SHALL extract prefixes only from files
2. THE Discovery_Engine SHALL NOT extract prefixes from directory names within the scanned structure
3. WHEN a directory name matches the Prefix_Pattern, THE Discovery_Engine SHALL ignore it for prefix extraction purposes

### Requirement 2: ISO Date Directory Exclusion

**User Story:** As a user, I want directories starting with ISO dates to be skipped during scanning, so that date-organized folders don't interfere with prefix discovery.

#### Acceptance Criteria

1. WHEN scanning for files within a Target_Directory_Candidate, THE Discovery_Engine SHALL skip subdirectories whose names start with an ISO_Date
2. THE Discovery_Engine SHALL continue scanning non-ISO-date subdirectories recursively
3. WHEN a directory name starts with a pattern matching `YYYY-MM-DD`, THE Discovery_Engine SHALL treat it as an ISO_Date_Directory
4. THE Discovery_Engine SHALL still scan files in the root of the Target_Directory_Candidate regardless of its name

### Requirement 3: Recursive Scanning with Filtering

**User Story:** As a user, I want the discovery to scan subdirectories for files while respecting the filtering rules, so that I get accurate prefix detection from nested file structures.

#### Acceptance Criteria

1. WHEN scanning a Target_Directory_Candidate, THE Discovery_Engine SHALL recursively enter non-ISO-date subdirectories
2. WHEN a subdirectory is an ISO_Date_Directory, THE Discovery_Engine SHALL skip it and all its contents
3. THE Discovery_Engine SHALL analyze all files found in non-skipped directories for prefix patterns
4. WHEN reporting discovery results, THE Discovery_Engine SHALL only include prefixes found in files
