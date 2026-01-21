# Implementation Plan: Discovery Enhancements

## Overview

This implementation adds depth limiting and interactive mode to the discovery command. Changes are in the discovery module and CLI.

## Tasks

- [x] 1. Implement Depth-Limited Discovery
  - [x] 1.1 Add DiscoverOptions type and depth tracking
    - Add `DiscoverOptions` struct with `MaxDepth` and `Interactive` fields
    - Add `DiscoverWithOptions` function signature
    - _Requirements: 1.1, 1.2_

  - [x] 1.2 Implement depth-limited directory walking
    - Modify `analyzeDirectoryWithCallback` to accept `maxDepth` parameter
    - Track current depth during filepath.Walk
    - Skip directories beyond maxDepth
    - Default to -1 (unlimited) when not specified
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.6_

  - [x] 1.3 Write unit tests for depth limiting
    - Test depth=0 returns only root files
    - Test depth=1 includes immediate subdirectories
    - Test depth=-1 (unlimited) traverses all levels
    - Test ISO-date directories skipped at all depths
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_

- [x] 2. Implement Interactive Discovery Mode
  - [x] 2.1 Create interactive prompter component
    - Create `internal/discovery/interactive.go`
    - Implement `InteractivePrompter` struct
    - Implement `PromptForRule(rule DiscoveredRule) (PromptResult, error)`
    - Support accept, reject, accept-all, reject-all, quit options
    - _Requirements: 2.1, 2.2, 2.5_

  - [x] 2.2 Add IsInteractive terminal detection
    - Implement `IsInteractive() bool` to check if terminal supports input
    - Fall back to non-interactive with warning when not TTY
    - _Requirements: 2.7_

  - [x] 2.3 Write unit tests for interactive prompter
    - Test prompt displays prefix and target directory
    - Test accept adds rule to config
    - Test reject skips rule
    - Test accept-all adds remaining rules
    - Test reject-all skips remaining rules
    - Test non-interactive mode auto-adds all rules
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.6_

- [x] 3. Update CLI for Discovery Options
  - [x] 3.1 Add --depth flag to discover command
    - Parse `--depth N` flag
    - Validate depth is non-negative
    - Pass to `DiscoverWithOptions`
    - _Requirements: 1.1_

  - [x] 3.2 Add --interactive flag to discover command
    - Parse `--interactive` flag
    - Check terminal interactivity
    - Invoke interactive prompter for each rule
    - _Requirements: 2.1, 2.7_

  - [x] 3.3 Write unit tests for CLI flag parsing
    - Test --depth with valid values
    - Test --depth with invalid values returns error
    - Test --interactive enables interactive mode
    - Test combined --depth and --interactive
    - _Requirements: 3.1, 3.2_

- [x] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Depth limiting is applied during the walk, not post-filtering
- Interactive mode uses stdin/stdout for prompts
- Unit tests with specific examples provide sufficient coverage for this feature
