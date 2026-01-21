# Requirements Document

## Introduction

This feature adds configuration management commands to Sorta including: listing current configuration, adding source directories, and auto-discovering prefix rules from existing file structures. The for-review directory is automatically created as a subdirectory within each source directory rather than being globally configured. The application uses a default config file location with optional override.

## Glossary

- **Sorta**: The file organization program that processes and moves files based on configuration rules
- **Default_Config_Path**: The default configuration file path `sorta-config.json` in the current working directory
- **Config_Flag**: The `-c` or `--config` command line flag to override the default config file path
- **Config_Command**: A CLI command for viewing the current configuration
- **Add_Source_Command**: A CLI command for adding a source directory to the configuration
- **Auto_Discover_Command**: A CLI command that scans directories to automatically detect and add prefix rules
- **Source_Directory**: A directory configured to be scanned for files to organize
- **Scan_Directory**: The root directory to scan for potential target directories during auto-discovery
- **Target_Directory_Candidate**: A subdirectory within the Scan_Directory that may contain organized files
- **Prefix_Pattern**: The filename pattern `<prefix> <ISO_Date> <other_info>` used to identify file organization
- **ISO_Date**: A date in YYYY-MM-DD format embedded in filenames
- **Prefix_Rule**: A configuration mapping that associates a filename prefix with a target directory
- **Configuration**: The JSON configuration object containing source directories and prefix rules
- **For_Review_Subdirectory**: A subdirectory named `for-review` created within each source directory for unclassified files
- **Duplicate_File**: A file that would overwrite an existing file at the destination path

## Requirements

### Requirement 1: Default Configuration File Location

**User Story:** As a user, I want Sorta to use a default config file location, so that I don't have to specify it every time.

#### Acceptance Criteria

1. WHEN no config file is specified, THE Sorta SHALL look for `sorta-config.json` in the current working directory
2. WHEN the `-c` or `--config` flag is provided, THE Sorta SHALL use the specified config file path instead
3. WHEN the default config file does not exist and no override is provided, THEN THE Sorta SHALL report that no config file was found
4. THE Sorta SHALL apply the config flag to all commands that require configuration

### Requirement 2: Config List Command

**User Story:** As a user, I want to view my current configuration, so that I can understand how Sorta is configured.

#### Acceptance Criteria

1. WHEN the user invokes `sorta config`, THE Sorta SHALL display the current configuration details
2. WHEN displaying configuration, THE Sorta SHALL list all configured source directories
3. WHEN displaying configuration, THE Sorta SHALL list all prefix rules with their prefixes and target directories
4. WHEN the config file does not exist, THEN THE Sorta SHALL report an error
5. WHEN the config file contains invalid JSON, THEN THE Sorta SHALL report a parsing error

### Requirement 3: Add Source Directory Command

**User Story:** As a user, I want to add source directories to my configuration via command line, so that I can configure scan locations without editing JSON.

#### Acceptance Criteria

1. WHEN the user invokes `sorta add-source <directory>`, THE Sorta SHALL add the directory to sourceDirectories
2. WHEN the directory argument is missing, THEN THE Sorta SHALL report an error and display usage instructions
3. WHEN the config file does not exist, THEN THE Sorta SHALL create a new configuration with the source directory
4. WHEN the source directory already exists in configuration, THE Sorta SHALL not add a duplicate entry
5. THE Sorta SHALL save the updated configuration back to the config file

### Requirement 4: Auto-Discover Command Interface

**User Story:** As a user, I want a command to auto-discover prefix rules from existing directories, so that I can quickly configure Sorta based on my existing file organization.

#### Acceptance Criteria

1. WHEN the user invokes `sorta discover <scan-directory>`, THE Sorta SHALL scan the directory for prefix patterns
2. WHEN the scan-directory argument is missing, THEN THE Sorta SHALL report an error and display usage instructions
3. THE Sorta SHALL display discovered prefix rules to the user before adding them

### Requirement 5: Target Directory Detection

**User Story:** As a user, I want Sorta to identify directories that likely contain organized files, so that only relevant directories are analyzed.

#### Acceptance Criteria

1. WHEN scanning, THE Sorta SHALL examine immediate subdirectories of the Scan_Directory as Target_Directory_Candidates
2. THE Sorta SHALL not recursively scan beyond immediate subdirectories for candidates
3. WHEN a Target_Directory_Candidate contains files matching the Prefix_Pattern, THE Sorta SHALL consider it a valid target directory

### Requirement 6: Prefix Pattern Detection

**User Story:** As a user, I want Sorta to detect file prefixes from existing files, so that prefix rules can be automatically generated.

#### Acceptance Criteria

1. WHEN analyzing files in a Target_Directory_Candidate, THE Sorta SHALL scan files recursively within that directory
2. WHEN a filename matches the pattern `<prefix> <YYYY-MM-DD> <other_info>`, THE Sorta SHALL extract the prefix
3. THE Sorta SHALL detect multiple distinct prefixes within the same Target_Directory_Candidate
4. THE Sorta SHALL require a space delimiter between the prefix and the ISO_Date
5. WHEN no files match the Prefix_Pattern in a directory, THE Sorta SHALL skip that directory

### Requirement 7: Configuration Update from Discovery

**User Story:** As a user, I want discovered prefix rules added to my configuration, so that I don't have to manually edit the config file.

#### Acceptance Criteria

1. WHEN the config file exists, THE Sorta SHALL load and update the existing configuration
2. WHEN the config file does not exist, THEN THE Sorta SHALL create a new configuration with discovered rules
3. WHEN adding a prefix rule, THE Sorta SHALL not add duplicate prefixes that already exist in the configuration
4. THE Sorta SHALL perform duplicate detection case-insensitively
5. WHEN new prefix rules are discovered, THE Sorta SHALL append them to the existing prefixRules array
6. THE Sorta SHALL save the updated configuration back to the config file

### Requirement 8: Discovery Output

**User Story:** As a user, I want to see what prefix rules were discovered, so that I can verify the auto-discovery results.

#### Acceptance Criteria

1. WHEN discovery completes, THE Sorta SHALL display the count of new prefix rules added
2. WHEN discovery completes, THE Sorta SHALL display each new prefix and its target directory
3. WHEN no new prefix rules are discovered, THE Sorta SHALL inform the user that no new rules were found
4. WHEN duplicate prefixes are skipped, THE Sorta SHALL inform the user which prefixes were already configured

### Requirement 9: For-Review Directory as Subdirectory

**User Story:** As a user, I want unclassified files placed in a for-review subdirectory within each source directory, so that review files stay close to their source.

#### Acceptance Criteria

1. THE Sorta SHALL create a `for-review` subdirectory within each Source_Directory for unclassified files
2. THE Sorta SHALL not require a global forReviewDirectory in the configuration
3. WHEN a file cannot be classified, THE Sorta SHALL move it to the `for-review` subdirectory of its Source_Directory
4. WHEN the `for-review` subdirectory does not exist, THE Sorta SHALL create it

### Requirement 10: Duplicate File Handling

**User Story:** As a user, I want duplicate files handled gracefully, so that no files are lost when a file with the same name already exists at the destination.

#### Acceptance Criteria

1. WHEN a file would overwrite an existing file at the destination, THE Sorta SHALL rename the file to include a duplicate marker
2. THE Sorta SHALL append `_duplicate` before the file extension for duplicate files
3. WHEN multiple duplicates exist, THE Sorta SHALL append `_duplicate_N` where N is an incrementing number
4. THE Sorta SHALL move the renamed duplicate file to the target directory
5. THE Sorta SHALL report when a file is moved as a duplicate

### Requirement 11: Configuration Serialization

**User Story:** As a developer, I want configuration changes to be reliably persisted, so that discovered rules are saved correctly.

#### Acceptance Criteria

1. WHEN saving configuration, THE Sorta SHALL serialize the Configuration object to valid JSON with indentation
2. FOR ALL valid Configuration objects, serializing then parsing SHALL produce an equivalent Configuration object (round-trip property)
