# Requirements Document

## Introduction

Sorta is a file organization utility that declutters hard drives by moving files into structured directories based on filename prefixes and embedded ISO dates. It reads configuration from a JSON file and deterministically organizes files according to prefix rules, placing unclassifiable files into a review directory.

## Glossary

- **Sorta**: The file organization program that processes and moves files based on configuration rules
- **Source_Directory**: A directory configured to be scanned for files to organize
- **Prefix_Rule**: A configuration mapping that associates a filename prefix with a target directory
- **Target_Directory**: The destination directory where matched files are moved
- **For_Review_Directory**: The directory where unclassified files are moved
- **ISO_Date**: A date in YYYY-MM-DD format embedded in filenames
- **Normalised_Prefix**: The canonical casing of a prefix as defined in configuration
- **Year_Prefix_Subfolder**: A subdirectory within a target directory named `<year> <normalised prefix>`
- **Normalised_Filename**: The original filename rewritten with the prefix segment using normalised casing

## Requirements

### Requirement 1: Configuration Loading

**User Story:** As a user, I want to configure Sorta via a JSON file, so that I can define my organization rules without modifying code.

#### Acceptance Criteria

1. WHEN Sorta starts, THE Sorta SHALL load configuration from a JSON file
2. WHEN the configuration file is missing, THEN THE Sorta SHALL report an error and exit gracefully
3. WHEN the configuration file contains invalid JSON, THEN THE Sorta SHALL report a parsing error and exit gracefully
4. THE Configuration SHALL support a list of one or more source directories
5. THE Configuration SHALL support a set of prefix rules mapping prefixes to target directories
6. THE Configuration SHALL support a single For Review directory path

### Requirement 2: Directory Scanning

**User Story:** As a user, I want Sorta to scan my configured source directories, so that all files in those locations are evaluated for organization.

#### Acceptance Criteria

1. WHEN Sorta runs, THE Sorta SHALL scan each configured Source_Directory for files
2. WHEN a Source_Directory does not exist, THEN THE Sorta SHALL report an error for that directory and continue with remaining directories
3. THE Sorta SHALL evaluate only files (not subdirectories) in each Source_Directory
4. THE Sorta SHALL not recursively scan subdirectories within Source_Directories

### Requirement 3: Prefix Matching

**User Story:** As a user, I want files to be matched against prefix rules case-insensitively, so that variations in filename casing are handled consistently.

#### Acceptance Criteria

1. WHEN a filename begins with a configured prefix, THE Sorta SHALL match it to that Prefix_Rule
2. THE Sorta SHALL perform prefix matching case-insensitively
3. WHEN a filename matches a prefix, THE Sorta SHALL require a single space delimiter immediately following the prefix
4. WHEN a filename matches multiple prefixes, THE Sorta SHALL use the longest matching prefix

### Requirement 4: ISO Date Extraction

**User Story:** As a user, I want Sorta to extract dates from filenames, so that files can be organized by year.

#### Acceptance Criteria

1. WHEN a file matches a prefix rule, THE Sorta SHALL extract the ISO_Date from the filename
2. THE Sorta SHALL expect the ISO_Date in YYYY-MM-DD format immediately after the prefix and delimiter
3. WHEN the ISO_Date is valid, THE Sorta SHALL extract the year component for folder organization
4. WHEN the ISO_Date is missing or invalid, THEN THE Sorta SHALL classify the file as unclassified

### Requirement 5: File Movement to Target Directory

**User Story:** As a user, I want matched files moved to organized subdirectories, so that my files are structured by year and prefix.

#### Acceptance Criteria

1. WHEN a file is successfully classified, THE Sorta SHALL move it to the mapped Target_Directory
2. THE Sorta SHALL create a Year_Prefix_Subfolder using the format `<year> <normalised prefix>`
3. THE Sorta SHALL normalise the filename prefix to the canonical casing from configuration
4. THE Sorta SHALL preserve the remainder of the filename exactly after the prefix
5. WHEN the Target_Directory or Year_Prefix_Subfolder does not exist, THE Sorta SHALL create them

### Requirement 6: Unclassified File Handling

**User Story:** As a user, I want files that cannot be classified moved to a review directory, so that I can manually handle edge cases.

#### Acceptance Criteria

1. WHEN a file does not match any configured Prefix_Rule, THE Sorta SHALL move it to the For_Review_Directory
2. WHEN a file matches a prefix but lacks the required space delimiter, THE Sorta SHALL move it to the For_Review_Directory
3. WHEN a file matches a prefix but lacks a valid ISO_Date, THE Sorta SHALL move it to the For_Review_Directory
4. THE Sorta SHALL not modify the filename when moving to For_Review_Directory
5. WHEN the For_Review_Directory does not exist, THE Sorta SHALL create it

### Requirement 7: File Integrity Constraints

**User Story:** As a user, I want Sorta to preserve my file contents and avoid reprocessing, so that my data remains safe and operations are predictable.

#### Acceptance Criteria

1. THE Sorta SHALL not modify file contents during any operation
2. THE Sorta SHALL not infer dates from filesystem metadata
3. THE Sorta SHALL not recursively reprocess files already moved to Target_Directories
4. THE Sorta SHALL not recursively reprocess files already moved to the For_Review_Directory
5. THE Sorta SHALL produce deterministic results when run multiple times with the same configuration

### Requirement 8: Configuration Serialization

**User Story:** As a developer, I want configuration to be parsed from and serialized to JSON, so that configuration can be reliably stored and loaded.

#### Acceptance Criteria

1. WHEN loading configuration, THE Sorta SHALL parse the JSON into a Configuration object
2. WHEN saving configuration, THE Sorta SHALL serialize the Configuration object to valid JSON
3. FOR ALL valid Configuration objects, parsing then serializing then parsing SHALL produce an equivalent Configuration object (round-trip property)
