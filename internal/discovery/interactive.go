// Package discovery handles auto-discovery of prefix rules from existing file structures.
package discovery

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// IsInteractive returns true if the terminal supports interactive input.
// It checks if stdin is a TTY (terminal) by examining the file mode.
// Returns false if stdin is not a terminal (e.g., piped input, redirected from file).
//
// Validates: Requirements 2.7
func IsInteractive() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// PromptResult represents the user's choice when prompted for a rule.
type PromptResult int

const (
	// PromptAccept indicates the user accepted this rule.
	PromptAccept PromptResult = iota
	// PromptReject indicates the user rejected this rule.
	PromptReject
	// PromptAcceptAll indicates the user wants to accept all remaining rules.
	PromptAcceptAll
	// PromptRejectAll indicates the user wants to reject all remaining rules.
	PromptRejectAll
	// PromptQuit indicates the user wants to quit without processing remaining rules.
	PromptQuit
)

// InteractivePrompter handles user prompts for rule selection.
type InteractivePrompter struct {
	reader io.Reader
	writer io.Writer
}

// NewInteractivePrompter creates a new InteractivePrompter with the given reader and writer.
// Use os.Stdin and os.Stdout for normal operation, or buffers for testing.
func NewInteractivePrompter(reader io.Reader, writer io.Writer) *InteractivePrompter {
	return &InteractivePrompter{
		reader: reader,
		writer: writer,
	}
}

// PromptForRule asks the user whether to accept a discovered rule.
// It displays the rule's prefix and target directory, then prompts for input.
// Returns the user's choice as a PromptResult.
//
// Validates: Requirements 2.1, 2.2, 2.5
func (p *InteractivePrompter) PromptForRule(rule DiscoveredRule) (PromptResult, error) {
	// Display the rule information (Requirement 2.2)
	fmt.Fprintf(p.writer, "\nDiscovered rule:\n")
	fmt.Fprintf(p.writer, "  Prefix: %s\n", rule.Prefix)
	fmt.Fprintf(p.writer, "  Target: %s\n", rule.TargetDirectory)

	// Show available options (Requirement 2.5)
	fmt.Fprintf(p.writer, "\nAccept this rule? (y)es, (n)o, (a)ccept all, (r)eject all, (q)uit: ")

	// Read user input
	scanner := bufio.NewScanner(p.reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return PromptQuit, fmt.Errorf("error reading input: %w", err)
		}
		// EOF reached, treat as quit
		return PromptQuit, nil
	}

	input := strings.TrimSpace(strings.ToLower(scanner.Text()))

	// Parse the input
	switch input {
	case "y", "yes":
		return PromptAccept, nil
	case "n", "no":
		return PromptReject, nil
	case "a", "accept all":
		return PromptAcceptAll, nil
	case "r", "reject all":
		return PromptRejectAll, nil
	case "q", "quit":
		return PromptQuit, nil
	default:
		// Invalid input, default to reject for safety
		fmt.Fprintf(p.writer, "Invalid input '%s', treating as reject.\n", input)
		return PromptReject, nil
	}
}
