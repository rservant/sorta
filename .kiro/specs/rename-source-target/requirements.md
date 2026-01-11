# Requirements Document

## Introduction

This feature renames the terminology used throughout the Sorta codebase from "source" to "inbound" and "target" to "outbound". This provides clearer semantics: files come from "inbound" directories and are organized into "outbound" directories.

## Glossary

- **Inbound_Directory**: A directory that Sorta scans for files to organize (previously called "source directory")
- **Outbound_Directory**: A directory where Sorta moves organized files (previously called "target directory")
- **Configuration**: The JSON configuration file that defines inbound directories and prefix rules
- **Prefix_Rule**: A mapping from a filename prefix to an outbound directory

## Requirements

### Requirement 1: Configuration Field Renaming

**User Story:** As a user, I want the configuration to use "inbound" and "outbound" terminology, so that the purpose of each directory type is clearer.

#### Acceptance Criteria

1. THE Configuration SHALL use `inboundDirectories` as the JSON key for directories to scan (replacing `sourceDirectories`).
2. THE Configuration SHALL use `outboundDirectory` as the JSON key in prefix rules (replacing `targetDirectory`).
3. WHEN loading a configuration file, THE Config_Loader SHALL parse the new field names correctly.
4. WHEN saving a configuration file, THE Config_Saver SHALL write the new field names.

### Requirement 2: Go Struct Field Renaming

**User Story:** As a developer, I want the Go structs to use consistent "inbound/outbound" naming, so that the code is self-documenting.

#### Acceptance Criteria

1. THE Configuration struct SHALL have a field named `InboundDirectories` (replacing `SourceDirectories`).
2. THE PrefixRule struct SHALL have a field named `OutboundDirectory` (replacing `TargetDirectory`).
3. THE Configuration struct methods SHALL be renamed: `HasInboundDirectory` (from `HasSourceDirectory`), `AddInboundDirectory` (from `AddSourceDirectory`).

### Requirement 3: CLI Command Renaming

**User Story:** As a user, I want the CLI commands to use "inbound" terminology, so that the interface is consistent with the configuration.

#### Acceptance Criteria

1. THE CLI SHALL provide an `add-inbound` subcommand (replacing `add-source`).
2. WHEN the user runs `add-inbound <directory>`, THE CLI SHALL add the directory to `inboundDirectories`.
3. THE CLI help text SHALL reference "inbound" and "outbound" directories.

### Requirement 4: Documentation Updates

**User Story:** As a user, I want the documentation to use consistent terminology, so that I can understand how to use Sorta.

#### Acceptance Criteria

1. THE README SHALL use "inbound" when referring to directories that are scanned.
2. THE README SHALL use "outbound" when referring to directories where files are moved.
3. THE spec documents SHALL be updated to use the new terminology where applicable.

### Requirement 5: Test Data Updates

**User Story:** As a developer, I want the test data to use the new terminology, so that tests remain consistent with the implementation.

#### Acceptance Criteria

1. THE test configuration files SHALL use `inboundDirectories` and `outboundDirectory`.
2. THE test assertions SHALL reference the new field names.
