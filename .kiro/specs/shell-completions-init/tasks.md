# Implementation Plan: Shell Completions and Init Wizard

## Overview

This implementation adds shell completion scripts and an interactive init wizard. New components are created for completion generation and the wizard.

## Tasks

- [ ] 1. Implement Shell Completion Generator
  - [ ] 1.1 Create completion generator component
    - Create `internal/completion/completion.go`
    - Add `Generator`, `CommandInfo`, `FlagInfo` types
    - Define all commands and flags as data
    - _Requirements: 1.4, 1.5_

  - [ ] 1.2 Implement bash completion generation
    - Implement `GenerateBash(w io.Writer) error`
    - Include all subcommands and flags
    - Add file path completion for path arguments
    - _Requirements: 1.1, 1.6_

  - [ ] 1.3 Implement zsh completion generation
    - Implement `GenerateZsh(w io.Writer) error`
    - Use zsh completion format
    - Include descriptions for commands and flags
    - _Requirements: 1.2, 1.6_

  - [ ] 1.4 Implement fish completion generation
    - Implement `GenerateFish(w io.Writer) error`
    - Use fish completion format
    - Include descriptions for commands and flags
    - _Requirements: 1.3, 1.6_

  - [ ]* 1.5 Write property test for completion validity
    - **Property 1: Completion Script Validity**
    - **Validates: Requirements 1.1, 1.2, 1.3**

  - [ ]* 1.6 Write property test for completion coverage
    - **Property 2: Completion Coverage**
    - **Validates: Requirements 1.4, 1.5, 1.6**

- [ ] 2. Implement Init Wizard
  - [ ] 2.1 Create wizard component
    - Create `internal/init/wizard.go`
    - Add `Wizard` and `WizardResult` types
    - Implement `Run() (*WizardResult, error)`
    - _Requirements: 3.1_

  - [ ] 2.2 Implement wizard prompts
    - Implement `PromptInboundDirs() ([]string, error)`
    - Implement `PromptPrefixRules() ([]config.PrefixRule, error)`
    - Implement `PromptConfigPath() (string, error)`
    - _Requirements: 3.2, 3.3, 3.4_

  - [ ] 2.3 Implement config file handling
    - Check if config file exists
    - Prompt for overwrite or merge if exists
    - Write configuration to file
    - Display next steps on completion
    - _Requirements: 3.5, 3.6, 3.7_

  - [ ]* 2.4 Write property test for wizard prompts
    - **Property 3: Init Wizard Prompts**
    - **Validates: Requirements 3.2, 3.3, 3.4**

  - [ ]* 2.5 Write property test for config creation
    - **Property 4: Init Config Creation**
    - **Validates: Requirements 3.5, 3.6**

- [ ] 3. Implement Discovery Integration in Wizard
  - [ ] 3.1 Add discovery option to wizard
    - Offer to scan for existing rules
    - Prompt for directory to scan
    - Display discovered rules
    - Allow selection of rules to include
    - _Requirements: 4.1, 4.2, 4.3, 4.4_

  - [ ]* 3.2 Write property test for discovery integration
    - **Property 5: Init Discovery Integration**
    - **Validates: Requirements 4.1, 4.2, 4.3, 4.4**

- [ ] 4. Update CLI
  - [ ] 4.1 Add completion command
    - Implement `completion` subcommand
    - Accept shell type argument (bash, zsh, fish)
    - Output completion script to stdout
    - Display error for invalid shell
    - _Requirements: 1.1, 1.2, 1.3, 1.7_

  - [ ] 4.2 Add completion help with installation instructions
    - Implement `completion --help`
    - Include installation commands for each shell
    - Explain persistence across sessions
    - _Requirements: 2.1, 2.2, 2.3_

  - [ ] 4.3 Add init command
    - Implement `init` subcommand
    - Check for interactive terminal
    - Display error if not interactive
    - Run wizard and handle result
    - _Requirements: 3.1, 3.8_

- [ ] 5. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are property-based tests
- Completion scripts should be tested by parsing, not by running in actual shells
- Wizard tests use simulated stdin/stdout
