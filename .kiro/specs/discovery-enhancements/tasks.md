# Implementation Plan: Discovery Enhancements

## Overview

This implementation adds depth limiting and interactive mode to the discovery command. Changes are in the discovery module and CLI.

## Tasks

- [ ] 1. Implement Depth-Limited Discovery
  - [ ] 1.1 Add DiscoverOptions type and depth tracking
    - Add `DiscoverOptions` struct with `MaxDepth` and `Interactive` fields
    - Add `DiscoverWithOptions` function signature
    - _Requirements: 1.1, 1.2_

  - [ ] 1.2 Implement depth-limited directory walking
    - Modify `analyzeDirectoryWithCallback` to accept `maxDepth` parameter
    - Track current depth during filepath.Walk
    - Skip directories beyond maxDepth
    - Default to -1 (unlimited) when not specified
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.6_

  - [ ]* 1.3 Write property test for depth limiting
    - **Property 1: Depth Limiting Correctness**
    - **Validates: Requirements 1.1, 1.3, 1.4, 1.6**

  - [ ]* 1.4 Write property test for unlimited depth default
    - **Property 2: Unlimited Depth Default**
    - **Validates: Requirements 1.2**

  - [ ]* 1.5 Write property test for ISO-date skip at all depths
    - **Property 3: ISO-Date Skip at All Depths**
    - **Validates: Requirements 1.5**

- [ ] 2. Implement Interactive Discovery Mode
  - [ ] 2.1 Create interactive prompter component
    - Create `internal/discovery/interactive.go`
    - Implement `InteractivePrompter` struct
    - Implement `PromptForRule(rule DiscoveredRule) (PromptResult, error)`
    - Support accept, reject, accept-all, reject-all, quit options
    - _Requirements: 2.1, 2.2, 2.5_

  - [ ] 2.2 Add IsInteractive terminal detection
    - Implement `IsInteractive() bool` to check if terminal supports input
    - Fall back to non-interactive with warning when not TTY
    - _Requirements: 2.7_

  - [ ]* 2.3 Write property test for interactive prompt content
    - **Property 4: Interactive Prompt Content**
    - **Validates: Requirements 2.1, 2.2**

  - [ ]* 2.4 Write property test for accept/reject behavior
    - **Property 5: Interactive Accept/Reject Behavior**
    - **Validates: Requirements 2.3, 2.4**

  - [ ]* 2.5 Write property test for non-interactive auto-add
    - **Property 6: Non-Interactive Auto-Add**
    - **Validates: Requirements 2.6**

- [ ] 3. Update CLI for Discovery Options
  - [ ] 3.1 Add --depth flag to discover command
    - Parse `--depth N` flag
    - Validate depth is non-negative
    - Pass to `DiscoverWithOptions`
    - _Requirements: 1.1_

  - [ ] 3.2 Add --interactive flag to discover command
    - Parse `--interactive` flag
    - Check terminal interactivity
    - Invoke interactive prompter for each rule
    - _Requirements: 2.1, 2.7_

  - [ ]* 3.3 Write property test for combined flags
    - **Property 7: Combined Flags Behavior**
    - **Validates: Requirements 3.1, 3.2**

- [ ] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are property-based tests
- Depth limiting is applied during the walk, not post-filtering
- Interactive mode uses stdin/stdout for prompts
