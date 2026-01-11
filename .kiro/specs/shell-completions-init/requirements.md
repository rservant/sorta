# Requirements Document

## Introduction

This feature improves the onboarding experience for Sorta by adding shell completion scripts for bash, zsh, and fish, plus an interactive `init` command that guides new users through initial setup.

## Glossary

- **Sorta**: The file organization program that processes and moves files based on configuration rules
- **Shell_Completion**: Tab-completion functionality for command-line arguments
- **Init_Wizard**: An interactive setup process that guides users through initial configuration
- **Configuration**: The JSON file containing inbound directories, prefix rules, and settings

## Requirements

### Requirement 1: Shell Completion Generation

**User Story:** As a user, I want shell completions for Sorta commands, so that I can use tab completion for faster command entry.

#### Acceptance Criteria

1. WHEN the user runs `sorta completion bash`, THE Sorta SHALL output a bash completion script
2. WHEN the user runs `sorta completion zsh`, THE Sorta SHALL output a zsh completion script
3. WHEN the user runs `sorta completion fish`, THE Sorta SHALL output a fish completion script
4. THE completion scripts SHALL complete all subcommands (run, config, discover, audit, undo, etc.)
5. THE completion scripts SHALL complete all flags (--config, --verbose, --dry-run, etc.)
6. THE completion scripts SHALL complete file paths for arguments that expect paths
7. WHEN an invalid shell is specified, THE Sorta SHALL display an error with supported shells

### Requirement 2: Completion Installation Instructions

**User Story:** As a user, I want clear instructions for installing completions, so that I can easily enable them in my shell.

#### Acceptance Criteria

1. WHEN the user runs `sorta completion --help`, THE Sorta SHALL display installation instructions for each shell
2. THE instructions SHALL include the command to generate and install completions
3. THE instructions SHALL explain how to persist completions across shell sessions

### Requirement 3: Interactive Init Wizard

**User Story:** As a new user, I want a guided setup process, so that I can quickly configure Sorta without reading documentation.

#### Acceptance Criteria

1. WHEN the user runs `sorta init`, THE Sorta SHALL start an interactive setup wizard
2. THE wizard SHALL prompt for inbound directories to monitor
3. THE wizard SHALL prompt for common prefix rules or offer to run discovery
4. THE wizard SHALL prompt for the configuration file location
5. THE wizard SHALL create the configuration file with the provided settings
6. IF a configuration file already exists, THEN THE Sorta SHALL ask whether to overwrite or merge
7. WHEN the wizard completes, THE Sorta SHALL display next steps for using Sorta
8. IF the terminal is not interactive, THEN THE Sorta SHALL display an error message

### Requirement 4: Init with Discovery Integration

**User Story:** As a user, I want init to optionally discover existing rules, so that I can bootstrap configuration from my current file organization.

#### Acceptance Criteria

1. THE wizard SHALL offer to scan existing organized directories for prefix rules
2. WHEN the user chooses to discover, THE wizard SHALL prompt for the directory to scan
3. THE wizard SHALL display discovered rules and allow the user to select which to include
4. THE wizard SHALL add selected rules to the new configuration
