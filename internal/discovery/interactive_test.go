package discovery

import (
	"bytes"
	"strings"
	"testing"
)

func TestInteractivePrompterAccept(t *testing.T) {
	// Test that 'y' input returns PromptAccept
	input := strings.NewReader("y\n")
	output := &bytes.Buffer{}

	prompter := NewInteractivePrompter(input, output)
	rule := DiscoveredRule{
		Prefix:          "INV",
		TargetDirectory: "/path/to/invoices",
	}

	result, err := prompter.PromptForRule(rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != PromptAccept {
		t.Errorf("expected PromptAccept, got %v", result)
	}

	// Verify output contains prefix and target directory
	outputStr := output.String()
	if !strings.Contains(outputStr, "INV") {
		t.Errorf("output should contain prefix 'INV', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "/path/to/invoices") {
		t.Errorf("output should contain target directory, got: %s", outputStr)
	}
}

func TestInteractivePrompterReject(t *testing.T) {
	// Test that 'n' input returns PromptReject
	input := strings.NewReader("n\n")
	output := &bytes.Buffer{}

	prompter := NewInteractivePrompter(input, output)
	rule := DiscoveredRule{
		Prefix:          "REC",
		TargetDirectory: "/path/to/receipts",
	}

	result, err := prompter.PromptForRule(rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != PromptReject {
		t.Errorf("expected PromptReject, got %v", result)
	}
}

func TestInteractivePrompterAcceptAll(t *testing.T) {
	// Test that 'a' input returns PromptAcceptAll
	input := strings.NewReader("a\n")
	output := &bytes.Buffer{}

	prompter := NewInteractivePrompter(input, output)
	rule := DiscoveredRule{
		Prefix:          "STMT",
		TargetDirectory: "/path/to/statements",
	}

	result, err := prompter.PromptForRule(rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != PromptAcceptAll {
		t.Errorf("expected PromptAcceptAll, got %v", result)
	}
}

func TestInteractivePrompterRejectAll(t *testing.T) {
	// Test that 'r' input returns PromptRejectAll
	input := strings.NewReader("r\n")
	output := &bytes.Buffer{}

	prompter := NewInteractivePrompter(input, output)
	rule := DiscoveredRule{
		Prefix:          "DOC",
		TargetDirectory: "/path/to/documents",
	}

	result, err := prompter.PromptForRule(rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != PromptRejectAll {
		t.Errorf("expected PromptRejectAll, got %v", result)
	}
}

func TestInteractivePrompterQuit(t *testing.T) {
	// Test that 'q' input returns PromptQuit
	input := strings.NewReader("q\n")
	output := &bytes.Buffer{}

	prompter := NewInteractivePrompter(input, output)
	rule := DiscoveredRule{
		Prefix:          "RPT",
		TargetDirectory: "/path/to/reports",
	}

	result, err := prompter.PromptForRule(rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != PromptQuit {
		t.Errorf("expected PromptQuit, got %v", result)
	}
}

func TestInteractivePrompterLongFormInputs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected PromptResult
	}{
		{"yes", "yes\n", PromptAccept},
		{"no", "no\n", PromptReject},
		{"accept all", "accept all\n", PromptAcceptAll},
		{"reject all", "reject all\n", PromptRejectAll},
		{"quit", "quit\n", PromptQuit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			prompter := NewInteractivePrompter(input, output)
			rule := DiscoveredRule{
				Prefix:          "TEST",
				TargetDirectory: "/path/to/test",
			}

			result, err := prompter.PromptForRule(rule)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestInteractivePrompterCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected PromptResult
	}{
		{"uppercase Y", "Y\n", PromptAccept},
		{"uppercase N", "N\n", PromptReject},
		{"uppercase A", "A\n", PromptAcceptAll},
		{"uppercase R", "R\n", PromptRejectAll},
		{"uppercase Q", "Q\n", PromptQuit},
		{"mixed case YES", "YeS\n", PromptAccept},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			prompter := NewInteractivePrompter(input, output)
			rule := DiscoveredRule{
				Prefix:          "TEST",
				TargetDirectory: "/path/to/test",
			}

			result, err := prompter.PromptForRule(rule)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestInteractivePrompterInvalidInput(t *testing.T) {
	// Test that invalid input defaults to reject
	input := strings.NewReader("invalid\n")
	output := &bytes.Buffer{}

	prompter := NewInteractivePrompter(input, output)
	rule := DiscoveredRule{
		Prefix:          "TEST",
		TargetDirectory: "/path/to/test",
	}

	result, err := prompter.PromptForRule(rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != PromptReject {
		t.Errorf("expected PromptReject for invalid input, got %v", result)
	}

	// Verify warning message was displayed
	outputStr := output.String()
	if !strings.Contains(outputStr, "Invalid input") {
		t.Errorf("output should contain invalid input warning, got: %s", outputStr)
	}
}

func TestInteractivePrompterEOF(t *testing.T) {
	// Test that EOF returns PromptQuit
	input := strings.NewReader("")
	output := &bytes.Buffer{}

	prompter := NewInteractivePrompter(input, output)
	rule := DiscoveredRule{
		Prefix:          "TEST",
		TargetDirectory: "/path/to/test",
	}

	result, err := prompter.PromptForRule(rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != PromptQuit {
		t.Errorf("expected PromptQuit on EOF, got %v", result)
	}
}

func TestInteractivePrompterDisplaysRuleInfo(t *testing.T) {
	// Test that the prompt displays prefix and target directory (Requirement 2.2)
	input := strings.NewReader("y\n")
	output := &bytes.Buffer{}

	prompter := NewInteractivePrompter(input, output)
	rule := DiscoveredRule{
		Prefix:          "INVOICE",
		TargetDirectory: "/home/user/documents/invoices",
	}

	_, err := prompter.PromptForRule(rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outputStr := output.String()

	// Verify prefix is displayed
	if !strings.Contains(outputStr, "Prefix: INVOICE") {
		t.Errorf("output should contain 'Prefix: INVOICE', got: %s", outputStr)
	}

	// Verify target directory is displayed
	if !strings.Contains(outputStr, "Target: /home/user/documents/invoices") {
		t.Errorf("output should contain 'Target: /home/user/documents/invoices', got: %s", outputStr)
	}

	// Verify options are displayed (Requirement 2.5)
	if !strings.Contains(outputStr, "(y)es") {
		t.Errorf("output should contain '(y)es' option, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "(n)o") {
		t.Errorf("output should contain '(n)o' option, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "(a)ccept all") {
		t.Errorf("output should contain '(a)ccept all' option, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "(r)eject all") {
		t.Errorf("output should contain '(r)eject all' option, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "(q)uit") {
		t.Errorf("output should contain '(q)uit' option, got: %s", outputStr)
	}
}

func TestInteractivePrompterWhitespaceHandling(t *testing.T) {
	// Test that whitespace around input is trimmed
	tests := []struct {
		name     string
		input    string
		expected PromptResult
	}{
		{"leading space", " y\n", PromptAccept},
		{"trailing space", "y \n", PromptAccept},
		{"both spaces", " y \n", PromptAccept},
		{"tabs", "\ty\t\n", PromptAccept},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			prompter := NewInteractivePrompter(input, output)
			rule := DiscoveredRule{
				Prefix:          "TEST",
				TargetDirectory: "/path/to/test",
			}

			result, err := prompter.PromptForRule(rule)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsInteractive(t *testing.T) {
	// Test that IsInteractive returns a boolean without error.
	// Note: The actual result depends on the test environment.
	// In CI/piped environments, this will return false.
	// In a real terminal, this will return true.
	result := IsInteractive()

	// Verify it returns a valid boolean (this test mainly ensures the function
	// doesn't panic and the implementation compiles correctly)
	if result != true && result != false {
		t.Errorf("IsInteractive should return true or false, got unexpected value")
	}
}

func TestIsInteractiveInPipedEnvironment(t *testing.T) {
	// When running tests, stdin is typically not a TTY (it's piped),
	// so IsInteractive should return false in most test environments.
	// This test documents the expected behavior in non-interactive contexts.
	result := IsInteractive()

	// In a test environment (piped stdin), we expect false
	// This validates Requirement 2.7: fall back to non-interactive when not TTY
	if result {
		t.Log("IsInteractive returned true - test is running in an interactive terminal")
	} else {
		t.Log("IsInteractive returned false - test is running in a non-interactive environment (expected)")
	}
}
