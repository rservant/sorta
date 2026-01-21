package output

import (
	"bytes"
	"fmt"
	"regexp"
	"sorta/internal/orchestrator"
	"strconv"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Test TTY detection behavior with mocked writers
// Requirements: 6.1, 6.3

func TestVerboseOutputOnlyAppearsWhenEnabled(t *testing.T) {
	tests := []struct {
		name        string
		verbose     bool
		expectEmpty bool
	}{
		{"verbose disabled - no output", false, true},
		{"verbose enabled - has output", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			out := New(Config{
				Verbose:   tt.verbose,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     false,
			})

			out.Verbose("test message")

			if tt.expectEmpty && buf.Len() > 0 {
				t.Errorf("expected no output when verbose disabled, got: %q", buf.String())
			}
			if !tt.expectEmpty && buf.Len() == 0 {
				t.Error("expected output when verbose enabled, got nothing")
			}
			if !tt.expectEmpty && !strings.Contains(buf.String(), "test message") {
				t.Errorf("expected output to contain 'test message', got: %q", buf.String())
			}
		})
	}
}

func TestInfoOutputAlwaysShown(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
	}{
		{"verbose disabled", false},
		{"verbose enabled", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			out := New(Config{
				Verbose:   tt.verbose,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     false,
			})

			out.Info("info message")

			if !strings.Contains(buf.String(), "info message") {
				t.Errorf("expected Info output regardless of verbose mode, got: %q", buf.String())
			}
		})
	}
}

func TestErrorOutputGoesToErrWriter(t *testing.T) {
	var stdoutBuf, stderrBuf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &stdoutBuf,
		ErrWriter: &stderrBuf,
		IsTTY:     false,
	})

	out.Error("error message")

	if stdoutBuf.Len() > 0 {
		t.Errorf("expected no stdout output for Error, got: %q", stdoutBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "error message") {
		t.Errorf("expected stderr to contain 'error message', got: %q", stderrBuf.String())
	}
}

// Test progress format matches "Processing file N/M..." pattern
// Requirements: 5.4

func TestProgressFormatMatchesPattern(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     true, // Progress only works with TTY
	})

	out.StartProgress(10)
	out.UpdateProgress(5, "")

	output := buf.String()
	if !strings.Contains(output, "Processing file 5/10...") {
		t.Errorf("expected progress format 'Processing file 5/10...', got: %q", output)
	}
}

func TestProgressWithCustomMessage(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     true,
	})

	out.StartProgress(20)
	out.UpdateProgress(7, "Scanning")

	output := buf.String()
	if !strings.Contains(output, "Scanning 7/20...") {
		t.Errorf("expected progress format 'Scanning 7/20...', got: %q", output)
	}
}

// Test progress suppression when not TTY
// Requirements: 6.1

func TestProgressSuppressedWhenNotTTY(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false, // Not a TTY
	})

	out.StartProgress(10)
	out.UpdateProgress(5, "")
	out.EndProgress()

	if buf.Len() > 0 {
		t.Errorf("expected no progress output when not TTY, got: %q", buf.String())
	}
}

// Test progress suppression when verbose mode is enabled
// Requirements: 5.5, 5.6

func TestProgressSuppressedWhenVerbose(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   true, // Verbose enabled
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     true,
	})

	out.StartProgress(10)
	out.UpdateProgress(5, "")
	out.EndProgress()

	// Should have no progress output (verbose mode suppresses progress)
	if strings.Contains(buf.String(), "Processing file") {
		t.Errorf("expected no progress output when verbose enabled, got: %q", buf.String())
	}
}

func TestEndProgressClearsLine(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     true,
	})

	out.StartProgress(10)
	out.UpdateProgress(5, "")
	out.EndProgress()

	// After EndProgress, the line should be cleared (ends with \r and spaces)
	output := buf.String()
	if !strings.HasSuffix(output, "\r") {
		t.Errorf("expected output to end with carriage return after EndProgress, got: %q", output)
	}
}

func TestProgressUsesCarriageReturn(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     true,
	})

	out.StartProgress(10)
	out.UpdateProgress(1, "")
	out.UpdateProgress(2, "")

	output := buf.String()
	// Should contain carriage returns for in-place updates
	if !strings.Contains(output, "\r") {
		t.Errorf("expected progress to use carriage return, got: %q", output)
	}
}

func TestIsVerbose(t *testing.T) {
	tests := []struct {
		name     string
		verbose  bool
		expected bool
	}{
		{"verbose enabled", true, true},
		{"verbose disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := New(Config{Verbose: tt.verbose})
			if out.IsVerbose() != tt.expected {
				t.Errorf("IsVerbose() = %v, want %v", out.IsVerbose(), tt.expected)
			}
		})
	}
}

func TestIsTTY(t *testing.T) {
	tests := []struct {
		name     string
		isTTY    bool
		expected bool
	}{
		{"is TTY", true, true},
		{"not TTY", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := New(Config{IsTTY: tt.isTTY})
			if out.IsTTY() != tt.expected {
				t.Errorf("IsTTY() = %v, want %v", out.IsTTY(), tt.expected)
			}
		})
	}
}

func TestNewWithNilWriters(t *testing.T) {
	// Test that New() handles nil writers by defaulting to os.Stdout/os.Stderr
	out := New(Config{})

	// Should not panic when using the output
	// We can't easily test the actual writers, but we can verify it doesn't crash
	if out == nil {
		t.Error("expected non-nil Output")
	}
}

func TestVerboseAddsNewline(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose: true,
		Writer:  &buf,
	})

	out.Verbose("message without newline")

	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("expected Verbose to add newline, got: %q", buf.String())
	}
}

func TestInfoAddsNewline(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Writer: &buf,
	})

	out.Info("message without newline")

	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("expected Info to add newline, got: %q", buf.String())
	}
}

func TestErrorAddsNewline(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		ErrWriter: &buf,
	})

	out.Error("message without newline")

	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("expected Error to add newline, got: %q", buf.String())
	}
}

// Feature: verbose-progress, Property 5: Progress Indicator Format and Lifecycle
// Validates: Requirements 5.1, 5.2, 5.3, 5.4, 5.5, 5.6
//
// For any long-running command (run, discover, undo) executed in non-verbose mode on a TTY,
// the progress indicator SHALL display in the format "Processing file N/M..." (or similar),
// use carriage return for in-place updates, and be cleared before the final summary is displayed.

// TestProgressIndicatorFormatAndLifecycle tests Property 5: Progress Indicator Format and Lifecycle
func TestProgressIndicatorFormatAndLifecycle(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Progress format matches "Processing file N/M..." pattern for default message
	// Validates: Requirement 5.4
	properties.Property("progress format matches 'Processing file N/M...' pattern", prop.ForAll(
		func(current, total int) bool {
			// Ensure current <= total for valid progress
			if current > total {
				current, total = total, current
			}
			if current < 1 {
				current = 1
			}
			if total < 1 {
				total = 1
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   false,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     true,
			})

			out.StartProgress(total)
			out.UpdateProgress(current, "")

			output := buf.String()

			// Check format matches "Processing file N/M..."
			pattern := regexp.MustCompile(`Processing file \d+/\d+\.\.\.`)
			if !pattern.MatchString(output) {
				return false
			}

			// Verify the exact numbers are present
			expectedPattern := regexp.MustCompile(`Processing file ` + regexp.QuoteMeta(strconv.Itoa(current)) + `/` + regexp.QuoteMeta(strconv.Itoa(total)) + `\.\.\.`)
			return expectedPattern.MatchString(output)
		},
		gen.IntRange(1, 1000),
		gen.IntRange(1, 1000),
	))

	// Property: Progress format matches "Message N/M..." pattern for custom message
	// Validates: Requirement 5.4
	properties.Property("progress format matches 'Message N/M...' pattern for custom message", prop.ForAll(
		func(current, total int, message string) bool {
			// Ensure current <= total for valid progress
			if current > total {
				current, total = total, current
			}
			if current < 1 {
				current = 1
			}
			if total < 1 {
				total = 1
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   false,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     true,
			})

			out.StartProgress(total)
			out.UpdateProgress(current, message)

			output := buf.String()

			// Check format matches "Message N/M..."
			expectedPattern := regexp.MustCompile(regexp.QuoteMeta(message) + ` \d+/\d+\.\.\.`)
			if !expectedPattern.MatchString(output) {
				return false
			}

			// Verify the exact numbers are present
			exactPattern := regexp.MustCompile(regexp.QuoteMeta(message) + ` ` + regexp.QuoteMeta(strconv.Itoa(current)) + `/` + regexp.QuoteMeta(strconv.Itoa(total)) + `\.\.\.`)
			return exactPattern.MatchString(output)
		},
		gen.IntRange(1, 1000),
		gen.IntRange(1, 1000),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property: Progress uses carriage return for in-place updates
	// Validates: Requirement 5.5
	properties.Property("progress uses carriage return for in-place updates", prop.ForAll(
		func(total int) bool {
			if total < 2 {
				total = 2
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   false,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     true,
			})

			out.StartProgress(total)
			out.UpdateProgress(1, "")
			out.UpdateProgress(2, "")

			output := buf.String()

			// Each update should start with carriage return
			return strings.Contains(output, "\r")
		},
		gen.IntRange(2, 100),
	))

	// Property: EndProgress clears the progress line
	// Validates: Requirement 5.6
	properties.Property("EndProgress clears the progress line", prop.ForAll(
		func(current, total int) bool {
			if current > total {
				current, total = total, current
			}
			if current < 1 {
				current = 1
			}
			if total < 1 {
				total = 1
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   false,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     true,
			})

			out.StartProgress(total)
			out.UpdateProgress(current, "")
			out.EndProgress()

			output := buf.String()

			// After EndProgress, the line should end with carriage return (cleared)
			return strings.HasSuffix(output, "\r")
		},
		gen.IntRange(1, 1000),
		gen.IntRange(1, 1000),
	))

	// Property: Progress is suppressed when verbose mode is enabled
	// Validates: Requirement 5.5 (verbose suppresses progress)
	properties.Property("progress is suppressed when verbose mode is enabled", prop.ForAll(
		func(current, total int) bool {
			if current > total {
				current, total = total, current
			}
			if current < 1 {
				current = 1
			}
			if total < 1 {
				total = 1
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   true, // Verbose enabled
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     true,
			})

			out.StartProgress(total)
			out.UpdateProgress(current, "")
			out.EndProgress()

			// No progress output should be produced when verbose is enabled
			return buf.Len() == 0
		},
		gen.IntRange(1, 1000),
		gen.IntRange(1, 1000),
	))

	// Property: Progress is suppressed when not TTY
	// Validates: Requirement 5.5 (non-TTY suppresses progress)
	properties.Property("progress is suppressed when not TTY", prop.ForAll(
		func(current, total int) bool {
			if current > total {
				current, total = total, current
			}
			if current < 1 {
				current = 1
			}
			if total < 1 {
				total = 1
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   false,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     false, // Not a TTY
			})

			out.StartProgress(total)
			out.UpdateProgress(current, "")
			out.EndProgress()

			// No progress output should be produced when not TTY
			return buf.Len() == 0
		},
		gen.IntRange(1, 1000),
		gen.IntRange(1, 1000),
	))

	// Property: Progress lifecycle is complete (start, update, end)
	// Validates: Requirements 5.1, 5.2, 5.3, 5.4, 5.5, 5.6
	properties.Property("progress lifecycle is complete", prop.ForAll(
		func(total int, updates []int) bool {
			if total < 1 {
				total = 1
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   false,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     true,
			})

			// Start progress
			out.StartProgress(total)

			// Apply updates
			for _, current := range updates {
				if current < 1 {
					current = 1
				}
				if current > total {
					current = total
				}
				out.UpdateProgress(current, "")
			}

			// End progress
			out.EndProgress()

			output := buf.String()

			// If there were updates, output should contain progress format
			if len(updates) > 0 {
				pattern := regexp.MustCompile(`Processing file \d+/\d+\.\.\.`)
				if !pattern.MatchString(output) {
					return false
				}
			}

			// Output should end with carriage return (cleared)
			if len(updates) > 0 && !strings.HasSuffix(output, "\r") {
				return false
			}

			return true
		},
		gen.IntRange(1, 100),
		gen.SliceOfN(5, gen.IntRange(1, 100)),
	))

	// Property: Multiple progress updates use carriage return for each
	// Validates: Requirement 5.5
	properties.Property("multiple progress updates each start with carriage return", prop.ForAll(
		func(total int) bool {
			if total < 3 {
				total = 3
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   false,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     true,
			})

			out.StartProgress(total)
			out.UpdateProgress(1, "")
			out.UpdateProgress(2, "")
			out.UpdateProgress(3, "")

			output := buf.String()

			// Count carriage returns - should have at least 3 (one per update)
			crCount := strings.Count(output, "\r")
			return crCount >= 3
		},
		gen.IntRange(3, 100),
	))

	properties.TestingRun(t)
}

// Feature: verbose-progress, Property 6: TTY Detection Behavior
// Validates: Requirements 6.1, 6.2, 6.3
//
// For any command execution, when stdout is not a TTY, progress indicators SHALL be suppressed;
// when verbose mode is enabled, verbose output SHALL appear regardless of TTY status;
// and final summaries SHALL always appear regardless of TTY status.

// TestTTYDetectionBehavior tests Property 6: TTY Detection Behavior
func TestTTYDetectionBehavior(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: When stdout is not a TTY, progress indicators are suppressed
	// Validates: Requirement 6.1
	properties.Property("progress indicators suppressed when not TTY", prop.ForAll(
		func(current, total int, verbose bool) bool {
			// Ensure valid progress values
			if current > total {
				current, total = total, current
			}
			if current < 1 {
				current = 1
			}
			if total < 1 {
				total = 1
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   verbose,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     false, // Not a TTY
			})

			out.StartProgress(total)
			out.UpdateProgress(current, "")
			out.UpdateProgress(current+1, "Processing")
			out.EndProgress()

			output := buf.String()

			// Progress indicators should be suppressed when not TTY
			// Should not contain progress format patterns
			hasProgressFormat := strings.Contains(output, "Processing file") ||
				strings.Contains(output, "/") && strings.Contains(output, "...")

			return !hasProgressFormat
		},
		gen.IntRange(1, 1000),
		gen.IntRange(1, 1000),
		gen.Bool(),
	))

	// Property: Final summaries (Info) always appear regardless of TTY status
	// Validates: Requirement 6.2
	properties.Property("final summaries always appear regardless of TTY status", prop.ForAll(
		func(isTTY, verbose bool, message string) bool {
			if len(message) == 0 {
				message = "summary"
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   verbose,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     isTTY,
			})

			// Info represents final summaries and results
			out.Info("%s", message)

			output := buf.String()

			// Info output should always appear regardless of TTY status
			return strings.Contains(output, message)
		},
		gen.Bool(),
		gen.Bool(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property: Verbose output appears regardless of TTY status when verbose mode is enabled
	// Validates: Requirement 6.3
	properties.Property("verbose output appears regardless of TTY status when enabled", prop.ForAll(
		func(isTTY bool, message string) bool {
			if len(message) == 0 {
				message = "verbose message"
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   true, // Verbose enabled
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     isTTY,
			})

			out.Verbose("%s", message)

			output := buf.String()

			// Verbose output should appear regardless of TTY status
			return strings.Contains(output, message)
		},
		gen.Bool(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property: Verbose output is suppressed when verbose mode is disabled (regardless of TTY)
	// Validates: Requirement 6.3 (inverse case)
	properties.Property("verbose output suppressed when verbose mode disabled", prop.ForAll(
		func(isTTY bool, message string) bool {
			if len(message) == 0 {
				message = "verbose message"
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   false, // Verbose disabled
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     isTTY,
			})

			out.Verbose("%s", message)

			output := buf.String()

			// Verbose output should NOT appear when verbose mode is disabled
			return !strings.Contains(output, message)
		},
		gen.Bool(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property: Error output always appears regardless of TTY status
	// Validates: Requirement 6.2 (errors are part of results)
	properties.Property("error output always appears regardless of TTY status", prop.ForAll(
		func(isTTY, verbose bool, message string) bool {
			if len(message) == 0 {
				message = "error"
			}

			var stdoutBuf, stderrBuf bytes.Buffer
			out := New(Config{
				Verbose:   verbose,
				Writer:    &stdoutBuf,
				ErrWriter: &stderrBuf,
				IsTTY:     isTTY,
			})

			out.Error("%s", message)

			// Error output should always appear in stderr regardless of TTY status
			return strings.Contains(stderrBuf.String(), message)
		},
		gen.Bool(),
		gen.Bool(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property: Progress only appears when TTY is true AND verbose is false
	// Validates: Requirements 6.1, 6.3 (combined behavior)
	properties.Property("progress only appears when TTY and not verbose", prop.ForAll(
		func(isTTY, verbose bool, current, total int) bool {
			// Ensure valid progress values
			if current > total {
				current, total = total, current
			}
			if current < 1 {
				current = 1
			}
			if total < 1 {
				total = 1
			}

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   verbose,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     isTTY,
			})

			out.StartProgress(total)
			out.UpdateProgress(current, "")
			out.EndProgress()

			output := buf.String()
			hasProgress := strings.Contains(output, "Processing file")

			// Progress should only appear when TTY is true AND verbose is false
			expectedProgress := isTTY && !verbose

			return hasProgress == expectedProgress
		},
		gen.Bool(),
		gen.Bool(),
		gen.IntRange(1, 1000),
		gen.IntRange(1, 1000),
	))

	// Property: TTY detection state is correctly reported by IsTTY()
	// Validates: Requirement 6.1 (TTY detection)
	properties.Property("IsTTY correctly reports TTY state", prop.ForAll(
		func(isTTY bool) bool {
			out := New(Config{
				IsTTY: isTTY,
			})

			return out.IsTTY() == isTTY
		},
		gen.Bool(),
	))

	// Property: Verbose mode state is correctly reported by IsVerbose()
	// Validates: Requirement 6.3 (verbose mode detection)
	properties.Property("IsVerbose correctly reports verbose state", prop.ForAll(
		func(verbose bool) bool {
			out := New(Config{
				Verbose: verbose,
			})

			return out.IsVerbose() == verbose
		},
		gen.Bool(),
	))

	// Property: Combined TTY and verbose states work correctly together
	// Validates: Requirements 6.1, 6.2, 6.3 (all combined)
	properties.Property("combined TTY and verbose states work correctly", prop.ForAll(
		func(isTTY, verbose bool, current, total int) bool {
			if current > total {
				current, total = total, current
			}
			if current < 1 {
				current = 1
			}
			if total < 1 {
				total = 1
			}

			// Use unique, distinguishable messages
			infoMsg := "INFO_UNIQUE_MARKER_12345"
			verboseMsg := "VERBOSE_UNIQUE_MARKER_67890"

			var buf bytes.Buffer
			out := New(Config{
				Verbose:   verbose,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     isTTY,
			})

			// Perform all operations
			out.StartProgress(total)
			out.UpdateProgress(current, "")
			out.Verbose("%s", verboseMsg)
			out.Info("%s", infoMsg)
			out.EndProgress()

			output := buf.String()

			// Check all conditions:
			// 1. Info always appears (Requirement 6.2)
			infoAppears := strings.Contains(output, infoMsg)

			// 2. Verbose appears only when verbose mode enabled (Requirement 6.3)
			verboseAppears := strings.Contains(output, verboseMsg)
			verboseCorrect := verboseAppears == verbose

			// 3. Progress appears only when TTY and not verbose (Requirement 6.1)
			progressAppears := strings.Contains(output, "Processing file")
			progressCorrect := progressAppears == (isTTY && !verbose)

			return infoAppears && verboseCorrect && progressCorrect
		},
		gen.Bool(),
		gen.Bool(),
		gen.IntRange(1, 100),
		gen.IntRange(1, 100),
	))

	properties.TestingRun(t)
}

// =============================================================================
// Unit Tests for Output Formatting (Dry Run Preview)
// Requirements: 1.2, 1.3, 1.6, 3.1
// =============================================================================

// TestPrintDryRunResult_MovedFilesAppearInOutput tests that each planned move operation appears in output
// Requirements: 1.2, 3.1 - Display each file that would be moved along with its destination path
func TestPrintDryRunResult_MovedFilesAppearInOutput(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved: []orchestrator.FileOperation{
			{Source: "/inbound/ABC 2024-01-15 Invoice.pdf", Destination: "/organized/2024 ABC/ABC 2024-01-15 Invoice.pdf", Prefix: "ABC"},
			{Source: "/inbound/XYZ 2024-02-20 Report.docx", Destination: "/organized/2024 XYZ/XYZ 2024-02-20 Report.docx", Prefix: "XYZ"},
		},
		ForReview: []orchestrator.FileOperation{},
		Skipped:   []orchestrator.FileOperation{},
		Errors:    []error{},
	}

	out.PrintDryRunResult(result)
	output := buf.String()

	// Verify each moved file appears in output
	if !strings.Contains(output, "/inbound/ABC 2024-01-15 Invoice.pdf") {
		t.Errorf("expected output to contain first source file, got: %q", output)
	}
	if !strings.Contains(output, "/organized/2024 ABC/ABC 2024-01-15 Invoice.pdf") {
		t.Errorf("expected output to contain first destination, got: %q", output)
	}
	if !strings.Contains(output, "/inbound/XYZ 2024-02-20 Report.docx") {
		t.Errorf("expected output to contain second source file, got: %q", output)
	}
	if !strings.Contains(output, "/organized/2024 XYZ/XYZ 2024-02-20 Report.docx") {
		t.Errorf("expected output to contain second destination, got: %q", output)
	}
}

// TestPrintDryRunResult_ForReviewFilesAppearInOutput tests that files going to for-review appear in output
// Requirements: 1.3 - Display files that would go to for-review directories
func TestPrintDryRunResult_ForReviewFilesAppearInOutput(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved: []orchestrator.FileOperation{},
		ForReview: []orchestrator.FileOperation{
			{Source: "/inbound/unknown-file.pdf", Destination: "/inbound/for-review/unknown-file.pdf", Reason: "no_prefix_match"},
			{Source: "/inbound/another-unmatched.docx", Destination: "/inbound/for-review/another-unmatched.docx", Reason: "no_prefix_match"},
		},
		Skipped: []orchestrator.FileOperation{},
		Errors:  []error{},
	}

	out.PrintDryRunResult(result)
	output := buf.String()

	// Verify each for-review file appears in output
	if !strings.Contains(output, "/inbound/unknown-file.pdf") {
		t.Errorf("expected output to contain first for-review source, got: %q", output)
	}
	if !strings.Contains(output, "/inbound/for-review/unknown-file.pdf") {
		t.Errorf("expected output to contain first for-review destination, got: %q", output)
	}
	if !strings.Contains(output, "/inbound/another-unmatched.docx") {
		t.Errorf("expected output to contain second for-review source, got: %q", output)
	}
	if !strings.Contains(output, "Files for review") {
		t.Errorf("expected output to contain 'Files for review' header, got: %q", output)
	}
}

// TestPrintDryRunResult_SourceToDestinationFormat tests the source → destination format
// Requirements: 3.1 - Show source path and destination path for each file
func TestPrintDryRunResult_SourceToDestinationFormat(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved: []orchestrator.FileOperation{
			{Source: "/source/file.pdf", Destination: "/dest/file.pdf", Prefix: "ABC"},
		},
		ForReview: []orchestrator.FileOperation{
			{Source: "/source/unknown.pdf", Destination: "/source/for-review/unknown.pdf"},
		},
		Skipped: []orchestrator.FileOperation{},
		Errors:  []error{},
	}

	out.PrintDryRunResult(result)
	output := buf.String()

	// Verify the arrow format is used
	if !strings.Contains(output, "→") {
		t.Errorf("expected output to contain '→' arrow format, got: %q", output)
	}

	// Verify source → destination pattern for moved files
	if !strings.Contains(output, "/source/file.pdf → /dest/file.pdf") {
		t.Errorf("expected output to contain 'source → destination' format for moved files, got: %q", output)
	}

	// Verify source → destination pattern for for-review files
	if !strings.Contains(output, "/source/unknown.pdf → /source/for-review/unknown.pdf") {
		t.Errorf("expected output to contain 'source → destination' format for for-review files, got: %q", output)
	}
}

// TestPrintDryRunResult_SkippedFilesAppearInOutput tests that skipped files appear in output
// Requirements: 1.2 - Display each file operation
func TestPrintDryRunResult_SkippedFilesAppearInOutput(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved:     []orchestrator.FileOperation{},
		ForReview: []orchestrator.FileOperation{},
		Skipped: []orchestrator.FileOperation{
			{Source: "/inbound/skipped-file.pdf", Reason: "permission denied"},
		},
		Errors: []error{},
	}

	out.PrintDryRunResult(result)
	output := buf.String()

	// Verify skipped file appears in output
	if !strings.Contains(output, "/inbound/skipped-file.pdf") {
		t.Errorf("expected output to contain skipped file, got: %q", output)
	}
	if !strings.Contains(output, "permission denied") {
		t.Errorf("expected output to contain skip reason, got: %q", output)
	}
	if !strings.Contains(output, "Files to be skipped") {
		t.Errorf("expected output to contain 'Files to be skipped' header, got: %q", output)
	}
}

// TestPrintDryRunResult_NilResult tests handling of nil result
func TestPrintDryRunResult_NilResult(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	// Should not panic with nil result
	out.PrintDryRunResult(nil)

	if buf.Len() > 0 {
		t.Errorf("expected no output for nil result, got: %q", buf.String())
	}
}

// TestPrintDryRunResult_EmptyResult tests handling of empty result
func TestPrintDryRunResult_EmptyResult(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved:     []orchestrator.FileOperation{},
		ForReview: []orchestrator.FileOperation{},
		Skipped:   []orchestrator.FileOperation{},
		Errors:    []error{},
	}

	out.PrintDryRunResult(result)

	// Empty result should produce no output (no headers for empty sections)
	if strings.Contains(buf.String(), "Files to be moved") {
		t.Errorf("expected no 'Files to be moved' header for empty result, got: %q", buf.String())
	}
}

// TestPrintDryRunResult_VerboseShowsPrefix tests that verbose mode shows additional details
// Requirements: 3.4 - Verbose mode shows additional details about rule matching
func TestPrintDryRunResult_VerboseShowsPrefix(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   true,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved: []orchestrator.FileOperation{
			{Source: "/inbound/ABC 2024-01-15 Invoice.pdf", Destination: "/organized/2024 ABC/ABC 2024-01-15 Invoice.pdf", Prefix: "ABC"},
		},
		ForReview: []orchestrator.FileOperation{},
		Skipped:   []orchestrator.FileOperation{},
		Errors:    []error{},
	}

	out.PrintDryRunResult(result)
	output := buf.String()

	// Verbose mode should show matched prefix
	if !strings.Contains(output, "Matched prefix: ABC") {
		t.Errorf("expected verbose output to contain 'Matched prefix: ABC', got: %q", output)
	}
}

// TestPrintDryRunResult_VerboseShowsReason tests that verbose mode shows reason for for-review files
func TestPrintDryRunResult_VerboseShowsReason(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   true,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved: []orchestrator.FileOperation{},
		ForReview: []orchestrator.FileOperation{
			{Source: "/inbound/unknown.pdf", Destination: "/inbound/for-review/unknown.pdf", Reason: "no_prefix_match"},
		},
		Skipped: []orchestrator.FileOperation{},
		Errors:  []error{},
	}

	out.PrintDryRunResult(result)
	output := buf.String()

	// Verbose mode should show reason
	if !strings.Contains(output, "Reason: no_prefix_match") {
		t.Errorf("expected verbose output to contain 'Reason: no_prefix_match', got: %q", output)
	}
}

// TestPrintSummary_CountsMatchOperations tests that summary counts are accurate
// Requirements: 1.6 - Display summary count of files that would be moved, reviewed, and skipped
func TestPrintSummary_CountsMatchOperations(t *testing.T) {
	tests := []struct {
		name      string
		moved     int
		forReview int
		skipped   int
		expected  []string
	}{
		{
			name:      "all categories have files",
			moved:     5,
			forReview: 3,
			skipped:   2,
			expected:  []string{"10 files", "5 moved", "3 for review", "2 skipped"},
		},
		{
			name:      "only moved files",
			moved:     7,
			forReview: 0,
			skipped:   0,
			expected:  []string{"7 files", "7 moved"},
		},
		{
			name:      "only for-review files",
			moved:     0,
			forReview: 4,
			skipped:   0,
			expected:  []string{"4 files", "4 for review"},
		},
		{
			name:      "only skipped files",
			moved:     0,
			forReview: 0,
			skipped:   6,
			expected:  []string{"6 files", "6 skipped"},
		},
		{
			name:      "no files",
			moved:     0,
			forReview: 0,
			skipped:   0,
			expected:  []string{"No files to process"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			out := New(Config{
				Verbose:   false,
				Writer:    &buf,
				ErrWriter: &buf,
				IsTTY:     false,
			})

			out.PrintSummary(tt.moved, tt.forReview, tt.skipped)
			output := buf.String()

			for _, exp := range tt.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("expected output to contain %q, got: %q", exp, output)
				}
			}
		})
	}
}

// TestPrintSummary_TotalEqualsSum tests that total equals sum of all categories
// Requirements: 1.6 - Summary counts are accurate
func TestPrintSummary_TotalEqualsSum(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	moved := 10
	forReview := 5
	skipped := 3
	expectedTotal := moved + forReview + skipped

	out.PrintSummary(moved, forReview, skipped)
	output := buf.String()

	// Verify total is correct
	expectedTotalStr := strconv.Itoa(expectedTotal) + " files"
	if !strings.Contains(output, expectedTotalStr) {
		t.Errorf("expected output to contain '%s', got: %q", expectedTotalStr, output)
	}
}

// TestPrintStatusResult_GroupedByDestination tests that status results are grouped by destination
// Requirements: 2.2, 3.2 - Display pending files grouped by their destination
func TestPrintStatusResult_GroupedByDestination(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.StatusResult{
		ByInbound: map[string]*orchestrator.InboundStatus{
			"/inbound1": {
				Directory: "/inbound1",
				ByDestination: map[string][]string{
					"/organized/2024 ABC": {"/inbound1/ABC 2024-01-15 Invoice.pdf", "/inbound1/ABC 2024-02-20 Report.pdf"},
					"/organized/2024 XYZ": {"/inbound1/XYZ 2024-03-10 Contract.pdf"},
				},
				Total: 3,
			},
		},
		GrandTotal: 3,
	}

	out.PrintStatusResult(result)
	output := buf.String()

	// Verify files are grouped by destination
	if !strings.Contains(output, "/organized/2024 ABC") {
		t.Errorf("expected output to contain destination '/organized/2024 ABC', got: %q", output)
	}
	if !strings.Contains(output, "/organized/2024 XYZ") {
		t.Errorf("expected output to contain destination '/organized/2024 XYZ', got: %q", output)
	}
	// Verify file counts per destination
	if !strings.Contains(output, "2 files") {
		t.Errorf("expected output to contain '2 files' for ABC destination, got: %q", output)
	}
	if !strings.Contains(output, "1 files") {
		t.Errorf("expected output to contain '1 files' for XYZ destination, got: %q", output)
	}
}

// TestPrintStatusResult_PerDirectoryCounts tests that per-directory counts are accurate
// Requirements: 2.3 - Display total count of pending files per inbound directory
func TestPrintStatusResult_PerDirectoryCounts(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.StatusResult{
		ByInbound: map[string]*orchestrator.InboundStatus{
			"/inbound1": {
				Directory: "/inbound1",
				ByDestination: map[string][]string{
					"/organized/2024 ABC": {"/inbound1/file1.pdf", "/inbound1/file2.pdf"},
				},
				Total: 2,
			},
			"/inbound2": {
				Directory: "/inbound2",
				ByDestination: map[string][]string{
					"/organized/2024 XYZ": {"/inbound2/file3.pdf", "/inbound2/file4.pdf", "/inbound2/file5.pdf"},
				},
				Total: 3,
			},
		},
		GrandTotal: 5,
	}

	out.PrintStatusResult(result)
	output := buf.String()

	// Verify per-directory counts
	if !strings.Contains(output, "/inbound1 (2 files)") {
		t.Errorf("expected output to contain '/inbound1 (2 files)', got: %q", output)
	}
	if !strings.Contains(output, "/inbound2 (3 files)") {
		t.Errorf("expected output to contain '/inbound2 (3 files)', got: %q", output)
	}
}

// TestPrintStatusResult_GrandTotal tests that grand total is displayed
// Requirements: 2.4 - Display grand total of all pending files
func TestPrintStatusResult_GrandTotal(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.StatusResult{
		ByInbound: map[string]*orchestrator.InboundStatus{
			"/inbound1": {
				Directory: "/inbound1",
				ByDestination: map[string][]string{
					"/organized/2024 ABC": {"/inbound1/file1.pdf", "/inbound1/file2.pdf"},
				},
				Total: 2,
			},
			"/inbound2": {
				Directory: "/inbound2",
				ByDestination: map[string][]string{
					"/organized/2024 XYZ": {"/inbound2/file3.pdf"},
				},
				Total: 1,
			},
		},
		GrandTotal: 3,
	}

	out.PrintStatusResult(result)
	output := buf.String()

	// Verify grand total is displayed
	if !strings.Contains(output, "Total pending files: 3") {
		t.Errorf("expected output to contain 'Total pending files: 3', got: %q", output)
	}
}

// TestPrintStatusResult_EmptyDirectoriesMessage tests the message when no pending files exist
// Requirements: 2.5 - Display message indicating all directories are organized
func TestPrintStatusResult_EmptyDirectoriesMessage(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.StatusResult{
		ByInbound:  map[string]*orchestrator.InboundStatus{},
		GrandTotal: 0,
	}

	out.PrintStatusResult(result)
	output := buf.String()

	// Verify empty message is displayed
	if !strings.Contains(output, "All directories are organized") {
		t.Errorf("expected output to contain 'All directories are organized', got: %q", output)
	}
}

// TestPrintStatusResult_NilResult tests handling of nil result
func TestPrintStatusResult_NilResult(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	// Should not panic with nil result
	out.PrintStatusResult(nil)

	if buf.Len() > 0 {
		t.Errorf("expected no output for nil result, got: %q", buf.String())
	}
}

// TestPrintStatusResult_VerboseShowsFileList tests that verbose mode shows individual files
func TestPrintStatusResult_VerboseShowsFileList(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   true,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.StatusResult{
		ByInbound: map[string]*orchestrator.InboundStatus{
			"/inbound1": {
				Directory: "/inbound1",
				ByDestination: map[string][]string{
					"/organized/2024 ABC": {"/inbound1/ABC 2024-01-15 Invoice.pdf"},
				},
				Total: 1,
			},
		},
		GrandTotal: 1,
	}

	out.PrintStatusResult(result)
	output := buf.String()

	// Verbose mode should show individual file paths
	if !strings.Contains(output, "/inbound1/ABC 2024-01-15 Invoice.pdf") {
		t.Errorf("expected verbose output to contain individual file path, got: %q", output)
	}
}

// TestPrintStatusResult_NonVerboseHidesFileList tests that non-verbose mode hides individual files
func TestPrintStatusResult_NonVerboseHidesFileList(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.StatusResult{
		ByInbound: map[string]*orchestrator.InboundStatus{
			"/inbound1": {
				Directory: "/inbound1",
				ByDestination: map[string][]string{
					"/organized/2024 ABC": {"/inbound1/ABC 2024-01-15 Invoice.pdf"},
				},
				Total: 1,
			},
		},
		GrandTotal: 1,
	}

	out.PrintStatusResult(result)
	output := buf.String()

	// Non-verbose mode should NOT show individual file paths (only counts)
	// The file path should not appear in non-verbose output
	// Note: The destination path will appear, but not the individual source file paths
	if strings.Contains(output, "/inbound1/ABC 2024-01-15 Invoice.pdf") {
		t.Errorf("expected non-verbose output to NOT contain individual file path, got: %q", output)
	}
}

// TestPrintStatusResult_MultipleInboundDirectories tests handling of multiple inbound directories
// Requirements: 2.1 - Scan all configured inbound directories
func TestPrintStatusResult_MultipleInboundDirectories(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.StatusResult{
		ByInbound: map[string]*orchestrator.InboundStatus{
			"/downloads": {
				Directory: "/downloads",
				ByDestination: map[string][]string{
					"/organized/2024 ABC": {"/downloads/file1.pdf"},
				},
				Total: 1,
			},
			"/documents/incoming": {
				Directory: "/documents/incoming",
				ByDestination: map[string][]string{
					"/organized/2024 XYZ": {"/documents/incoming/file2.pdf", "/documents/incoming/file3.pdf"},
				},
				Total: 2,
			},
		},
		GrandTotal: 3,
	}

	out.PrintStatusResult(result)
	output := buf.String()

	// Verify both inbound directories appear
	if !strings.Contains(output, "/downloads") {
		t.Errorf("expected output to contain '/downloads', got: %q", output)
	}
	if !strings.Contains(output, "/documents/incoming") {
		t.Errorf("expected output to contain '/documents/incoming', got: %q", output)
	}
}

// TestPrintStatusResult_ForReviewDestination tests that for-review destinations are shown
func TestPrintStatusResult_ForReviewDestination(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.StatusResult{
		ByInbound: map[string]*orchestrator.InboundStatus{
			"/inbound": {
				Directory: "/inbound",
				ByDestination: map[string][]string{
					"/inbound/for-review": {"/inbound/unknown-file.pdf"},
				},
				Total: 1,
			},
		},
		GrandTotal: 1,
	}

	out.PrintStatusResult(result)
	output := buf.String()

	// Verify for-review destination appears
	if !strings.Contains(output, "/inbound/for-review") {
		t.Errorf("expected output to contain '/inbound/for-review', got: %q", output)
	}
}

// TestPrintDryRunResult_ErrorsAppearInOutput tests that errors are displayed
func TestPrintDryRunResult_ErrorsAppearInOutput(t *testing.T) {
	var stdoutBuf, stderrBuf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &stdoutBuf,
		ErrWriter: &stderrBuf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved:     []orchestrator.FileOperation{},
		ForReview: []orchestrator.FileOperation{},
		Skipped:   []orchestrator.FileOperation{},
		Errors:    []error{fmt.Errorf("failed to read directory: permission denied")},
	}

	out.PrintDryRunResult(result)

	// Errors should appear in stderr
	if !strings.Contains(stderrBuf.String(), "failed to read directory: permission denied") {
		t.Errorf("expected stderr to contain error message, got: %q", stderrBuf.String())
	}
}

// TestPrintDryRunResult_MixedOperations tests output with all operation types
func TestPrintDryRunResult_MixedOperations(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved: []orchestrator.FileOperation{
			{Source: "/inbound/ABC 2024-01-15 Invoice.pdf", Destination: "/organized/2024 ABC/ABC 2024-01-15 Invoice.pdf", Prefix: "ABC"},
		},
		ForReview: []orchestrator.FileOperation{
			{Source: "/inbound/unknown.pdf", Destination: "/inbound/for-review/unknown.pdf", Reason: "no_prefix_match"},
		},
		Skipped: []orchestrator.FileOperation{
			{Source: "/inbound/locked.pdf", Reason: "file locked"},
		},
		Errors: []error{},
	}

	out.PrintDryRunResult(result)
	output := buf.String()

	// Verify all sections appear
	if !strings.Contains(output, "Files to be moved") {
		t.Errorf("expected output to contain 'Files to be moved' header, got: %q", output)
	}
	if !strings.Contains(output, "Files for review") {
		t.Errorf("expected output to contain 'Files for review' header, got: %q", output)
	}
	if !strings.Contains(output, "Files to be skipped") {
		t.Errorf("expected output to contain 'Files to be skipped' header, got: %q", output)
	}
}

// =============================================================================
// Unit Tests for Verbose Mode (Task 4.3)
// Requirements: 3.4 - Verbose mode shows additional details about rule matching
// =============================================================================

// TestDryRunVerboseMode_ShowsMatchedPrefix tests that verbose mode shows matched prefix details
// Requirements: 3.4 - When verbose mode is enabled with dry-run, show additional details about rule matching
func TestDryRunVerboseMode_ShowsMatchedPrefix(t *testing.T) {
	var verboseBuf, nonVerboseBuf bytes.Buffer

	// Create verbose output
	verboseOut := New(Config{
		Verbose:   true,
		Writer:    &verboseBuf,
		ErrWriter: &verboseBuf,
		IsTTY:     false,
	})

	// Create non-verbose output
	nonVerboseOut := New(Config{
		Verbose:   false,
		Writer:    &nonVerboseBuf,
		ErrWriter: &nonVerboseBuf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved: []orchestrator.FileOperation{
			{Source: "/inbound/ABC 2024-01-15 Invoice.pdf", Destination: "/organized/2024 ABC/ABC 2024-01-15 Invoice.pdf", Prefix: "ABC"},
			{Source: "/inbound/XYZ 2024-02-20 Report.docx", Destination: "/organized/2024 XYZ/XYZ 2024-02-20 Report.docx", Prefix: "XYZ"},
		},
		ForReview: []orchestrator.FileOperation{},
		Skipped:   []orchestrator.FileOperation{},
		Errors:    []error{},
	}

	verboseOut.PrintDryRunResult(result)
	nonVerboseOut.PrintDryRunResult(result)

	verboseOutput := verboseBuf.String()
	nonVerboseOutput := nonVerboseBuf.String()

	// Verbose output should contain matched prefix details
	if !strings.Contains(verboseOutput, "Matched prefix: ABC") {
		t.Errorf("expected verbose output to contain 'Matched prefix: ABC', got: %q", verboseOutput)
	}
	if !strings.Contains(verboseOutput, "Matched prefix: XYZ") {
		t.Errorf("expected verbose output to contain 'Matched prefix: XYZ', got: %q", verboseOutput)
	}

	// Non-verbose output should NOT contain matched prefix details
	if strings.Contains(nonVerboseOutput, "Matched prefix:") {
		t.Errorf("expected non-verbose output to NOT contain 'Matched prefix:', got: %q", nonVerboseOutput)
	}
}

// TestDryRunVerboseMode_ShowsForReviewReason tests that verbose mode shows reason for for-review files
// Requirements: 3.4 - When verbose mode is enabled with dry-run, show additional details
func TestDryRunVerboseMode_ShowsForReviewReason(t *testing.T) {
	var verboseBuf, nonVerboseBuf bytes.Buffer

	// Create verbose output
	verboseOut := New(Config{
		Verbose:   true,
		Writer:    &verboseBuf,
		ErrWriter: &verboseBuf,
		IsTTY:     false,
	})

	// Create non-verbose output
	nonVerboseOut := New(Config{
		Verbose:   false,
		Writer:    &nonVerboseBuf,
		ErrWriter: &nonVerboseBuf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved: []orchestrator.FileOperation{},
		ForReview: []orchestrator.FileOperation{
			{Source: "/inbound/unknown-file.pdf", Destination: "/inbound/for-review/unknown-file.pdf", Reason: "no_prefix_match"},
			{Source: "/inbound/another-unmatched.docx", Destination: "/inbound/for-review/another-unmatched.docx", Reason: "invalid_format"},
		},
		Skipped: []orchestrator.FileOperation{},
		Errors:  []error{},
	}

	verboseOut.PrintDryRunResult(result)
	nonVerboseOut.PrintDryRunResult(result)

	verboseOutput := verboseBuf.String()
	nonVerboseOutput := nonVerboseBuf.String()

	// Verbose output should contain reason details
	if !strings.Contains(verboseOutput, "Reason: no_prefix_match") {
		t.Errorf("expected verbose output to contain 'Reason: no_prefix_match', got: %q", verboseOutput)
	}
	if !strings.Contains(verboseOutput, "Reason: invalid_format") {
		t.Errorf("expected verbose output to contain 'Reason: invalid_format', got: %q", verboseOutput)
	}

	// Non-verbose output should NOT contain reason details (for for-review files)
	// Note: Skipped files always show reason, but for-review files only show reason in verbose mode
	if strings.Contains(nonVerboseOutput, "Reason: no_prefix_match") {
		t.Errorf("expected non-verbose output to NOT contain 'Reason: no_prefix_match', got: %q", nonVerboseOutput)
	}
}

// TestDryRunNonVerboseMode_IsConcise tests that non-verbose output is concise
// Requirements: 3.4 - Non-verbose output should be concise without additional details
func TestDryRunNonVerboseMode_IsConcise(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved: []orchestrator.FileOperation{
			{Source: "/inbound/ABC 2024-01-15 Invoice.pdf", Destination: "/organized/2024 ABC/ABC 2024-01-15 Invoice.pdf", Prefix: "ABC"},
		},
		ForReview: []orchestrator.FileOperation{
			{Source: "/inbound/unknown.pdf", Destination: "/inbound/for-review/unknown.pdf", Reason: "no_prefix_match"},
		},
		Skipped: []orchestrator.FileOperation{},
		Errors:  []error{},
	}

	out.PrintDryRunResult(result)
	output := buf.String()

	// Non-verbose output should contain basic info (source → destination)
	if !strings.Contains(output, "/inbound/ABC 2024-01-15 Invoice.pdf → /organized/2024 ABC/ABC 2024-01-15 Invoice.pdf") {
		t.Errorf("expected non-verbose output to contain source → destination, got: %q", output)
	}

	// Non-verbose output should NOT contain verbose-only details
	if strings.Contains(output, "Matched prefix:") {
		t.Errorf("expected non-verbose output to NOT contain 'Matched prefix:', got: %q", output)
	}
	if strings.Contains(output, "Reason: no_prefix_match") {
		t.Errorf("expected non-verbose output to NOT contain 'Reason:' for for-review files, got: %q", output)
	}
}

// TestStatusVerboseMode_ShowsIndividualFilePaths tests that verbose mode shows individual file paths
// Requirements: 3.4 - Verbose mode shows additional details
func TestStatusVerboseMode_ShowsIndividualFilePaths(t *testing.T) {
	var verboseBuf, nonVerboseBuf bytes.Buffer

	// Create verbose output
	verboseOut := New(Config{
		Verbose:   true,
		Writer:    &verboseBuf,
		ErrWriter: &verboseBuf,
		IsTTY:     false,
	})

	// Create non-verbose output
	nonVerboseOut := New(Config{
		Verbose:   false,
		Writer:    &nonVerboseBuf,
		ErrWriter: &nonVerboseBuf,
		IsTTY:     false,
	})

	result := &orchestrator.StatusResult{
		ByInbound: map[string]*orchestrator.InboundStatus{
			"/inbound": {
				Directory: "/inbound",
				ByDestination: map[string][]string{
					"/organized/2024 ABC": {
						"/inbound/ABC 2024-01-15 Invoice.pdf",
						"/inbound/ABC 2024-02-20 Report.docx",
						"/inbound/ABC 2024-03-10 Contract.pdf",
					},
				},
				Total: 3,
			},
		},
		GrandTotal: 3,
	}

	verboseOut.PrintStatusResult(result)
	nonVerboseOut.PrintStatusResult(result)

	verboseOutput := verboseBuf.String()
	nonVerboseOutput := nonVerboseBuf.String()

	// Verbose output should contain individual file paths
	if !strings.Contains(verboseOutput, "/inbound/ABC 2024-01-15 Invoice.pdf") {
		t.Errorf("expected verbose output to contain individual file path, got: %q", verboseOutput)
	}
	if !strings.Contains(verboseOutput, "/inbound/ABC 2024-02-20 Report.docx") {
		t.Errorf("expected verbose output to contain individual file path, got: %q", verboseOutput)
	}
	if !strings.Contains(verboseOutput, "/inbound/ABC 2024-03-10 Contract.pdf") {
		t.Errorf("expected verbose output to contain individual file path, got: %q", verboseOutput)
	}

	// Non-verbose output should NOT contain individual file paths
	if strings.Contains(nonVerboseOutput, "/inbound/ABC 2024-01-15 Invoice.pdf") {
		t.Errorf("expected non-verbose output to NOT contain individual file path, got: %q", nonVerboseOutput)
	}
}

// TestStatusNonVerboseMode_ShowsOnlyCounts tests that non-verbose mode shows only counts
// Requirements: 3.4 - Non-verbose output is concise
func TestStatusNonVerboseMode_ShowsOnlyCounts(t *testing.T) {
	var buf bytes.Buffer
	out := New(Config{
		Verbose:   false,
		Writer:    &buf,
		ErrWriter: &buf,
		IsTTY:     false,
	})

	result := &orchestrator.StatusResult{
		ByInbound: map[string]*orchestrator.InboundStatus{
			"/inbound": {
				Directory: "/inbound",
				ByDestination: map[string][]string{
					"/organized/2024 ABC": {
						"/inbound/ABC 2024-01-15 Invoice.pdf",
						"/inbound/ABC 2024-02-20 Report.docx",
					},
					"/organized/2024 XYZ": {
						"/inbound/XYZ 2024-03-10 Contract.pdf",
					},
				},
				Total: 3,
			},
		},
		GrandTotal: 3,
	}

	out.PrintStatusResult(result)
	output := buf.String()

	// Non-verbose output should contain counts
	if !strings.Contains(output, "3 files") {
		t.Errorf("expected non-verbose output to contain total count '3 files', got: %q", output)
	}
	if !strings.Contains(output, "2 files") {
		t.Errorf("expected non-verbose output to contain destination count '2 files', got: %q", output)
	}
	if !strings.Contains(output, "1 files") {
		t.Errorf("expected non-verbose output to contain destination count '1 files', got: %q", output)
	}

	// Non-verbose output should contain destination directories
	if !strings.Contains(output, "/organized/2024 ABC") {
		t.Errorf("expected non-verbose output to contain destination directory, got: %q", output)
	}
	if !strings.Contains(output, "/organized/2024 XYZ") {
		t.Errorf("expected non-verbose output to contain destination directory, got: %q", output)
	}

	// Non-verbose output should NOT contain individual file paths
	if strings.Contains(output, "/inbound/ABC 2024-01-15 Invoice.pdf") {
		t.Errorf("expected non-verbose output to NOT contain individual file paths, got: %q", output)
	}
}

// TestStatusVerboseMode_MultipleInboundDirectories tests verbose mode with multiple inbound directories
// Requirements: 3.4 - Verbose mode shows additional details
func TestStatusVerboseMode_MultipleInboundDirectories(t *testing.T) {
	var verboseBuf, nonVerboseBuf bytes.Buffer

	// Create verbose output
	verboseOut := New(Config{
		Verbose:   true,
		Writer:    &verboseBuf,
		ErrWriter: &verboseBuf,
		IsTTY:     false,
	})

	// Create non-verbose output
	nonVerboseOut := New(Config{
		Verbose:   false,
		Writer:    &nonVerboseBuf,
		ErrWriter: &nonVerboseBuf,
		IsTTY:     false,
	})

	result := &orchestrator.StatusResult{
		ByInbound: map[string]*orchestrator.InboundStatus{
			"/downloads": {
				Directory: "/downloads",
				ByDestination: map[string][]string{
					"/organized/2024 ABC": {"/downloads/ABC file1.pdf"},
				},
				Total: 1,
			},
			"/documents/incoming": {
				Directory: "/documents/incoming",
				ByDestination: map[string][]string{
					"/organized/2024 XYZ": {"/documents/incoming/XYZ file2.pdf", "/documents/incoming/XYZ file3.pdf"},
				},
				Total: 2,
			},
		},
		GrandTotal: 3,
	}

	verboseOut.PrintStatusResult(result)
	nonVerboseOut.PrintStatusResult(result)

	verboseOutput := verboseBuf.String()
	nonVerboseOutput := nonVerboseBuf.String()

	// Both outputs should contain inbound directory headers with counts
	if !strings.Contains(verboseOutput, "/downloads (1 files)") {
		t.Errorf("expected verbose output to contain '/downloads (1 files)', got: %q", verboseOutput)
	}
	if !strings.Contains(nonVerboseOutput, "/downloads (1 files)") {
		t.Errorf("expected non-verbose output to contain '/downloads (1 files)', got: %q", nonVerboseOutput)
	}

	// Verbose output should contain individual file paths
	if !strings.Contains(verboseOutput, "/downloads/ABC file1.pdf") {
		t.Errorf("expected verbose output to contain individual file path, got: %q", verboseOutput)
	}
	if !strings.Contains(verboseOutput, "/documents/incoming/XYZ file2.pdf") {
		t.Errorf("expected verbose output to contain individual file path, got: %q", verboseOutput)
	}

	// Non-verbose output should NOT contain individual file paths
	if strings.Contains(nonVerboseOutput, "/downloads/ABC file1.pdf") {
		t.Errorf("expected non-verbose output to NOT contain individual file path, got: %q", nonVerboseOutput)
	}
	if strings.Contains(nonVerboseOutput, "/documents/incoming/XYZ file2.pdf") {
		t.Errorf("expected non-verbose output to NOT contain individual file path, got: %q", nonVerboseOutput)
	}
}

// TestDryRunVerboseMode_MixedOperations tests verbose mode with all operation types
// Requirements: 3.4 - Verbose mode shows additional details about rule matching
func TestDryRunVerboseMode_MixedOperations(t *testing.T) {
	var verboseBuf, nonVerboseBuf bytes.Buffer

	// Create verbose output
	verboseOut := New(Config{
		Verbose:   true,
		Writer:    &verboseBuf,
		ErrWriter: &verboseBuf,
		IsTTY:     false,
	})

	// Create non-verbose output
	nonVerboseOut := New(Config{
		Verbose:   false,
		Writer:    &nonVerboseBuf,
		ErrWriter: &nonVerboseBuf,
		IsTTY:     false,
	})

	result := &orchestrator.RunResult{
		Moved: []orchestrator.FileOperation{
			{Source: "/inbound/ABC 2024-01-15 Invoice.pdf", Destination: "/organized/2024 ABC/ABC 2024-01-15 Invoice.pdf", Prefix: "ABC"},
			{Source: "/inbound/XYZ 2024-02-20 Report.docx", Destination: "/organized/2024 XYZ/XYZ 2024-02-20 Report.docx", Prefix: "XYZ"},
		},
		ForReview: []orchestrator.FileOperation{
			{Source: "/inbound/unknown.pdf", Destination: "/inbound/for-review/unknown.pdf", Reason: "no_prefix_match"},
		},
		Skipped: []orchestrator.FileOperation{
			{Source: "/inbound/locked.pdf", Reason: "file locked"},
		},
		Errors: []error{},
	}

	verboseOut.PrintDryRunResult(result)
	nonVerboseOut.PrintDryRunResult(result)

	verboseOutput := verboseBuf.String()
	nonVerboseOutput := nonVerboseBuf.String()

	// Both outputs should contain basic file info
	if !strings.Contains(verboseOutput, "/inbound/ABC 2024-01-15 Invoice.pdf") {
		t.Errorf("expected verbose output to contain source file, got: %q", verboseOutput)
	}
	if !strings.Contains(nonVerboseOutput, "/inbound/ABC 2024-01-15 Invoice.pdf") {
		t.Errorf("expected non-verbose output to contain source file, got: %q", nonVerboseOutput)
	}

	// Verbose output should contain additional details
	if !strings.Contains(verboseOutput, "Matched prefix: ABC") {
		t.Errorf("expected verbose output to contain 'Matched prefix: ABC', got: %q", verboseOutput)
	}
	if !strings.Contains(verboseOutput, "Matched prefix: XYZ") {
		t.Errorf("expected verbose output to contain 'Matched prefix: XYZ', got: %q", verboseOutput)
	}
	if !strings.Contains(verboseOutput, "Reason: no_prefix_match") {
		t.Errorf("expected verbose output to contain 'Reason: no_prefix_match', got: %q", verboseOutput)
	}

	// Non-verbose output should NOT contain verbose-only details
	if strings.Contains(nonVerboseOutput, "Matched prefix:") {
		t.Errorf("expected non-verbose output to NOT contain 'Matched prefix:', got: %q", nonVerboseOutput)
	}
	// Note: Skipped files always show reason in both modes
	if strings.Contains(nonVerboseOutput, "Reason: no_prefix_match") {
		t.Errorf("expected non-verbose output to NOT contain 'Reason: no_prefix_match' for for-review files, got: %q", nonVerboseOutput)
	}
}

// TestVerboseMode_OutputLengthComparison tests that verbose output is longer than non-verbose
// Requirements: 3.4 - Verbose mode shows additional details
func TestVerboseMode_OutputLengthComparison(t *testing.T) {
	var verboseBuf, nonVerboseBuf bytes.Buffer

	// Create verbose output
	verboseOut := New(Config{
		Verbose:   true,
		Writer:    &verboseBuf,
		ErrWriter: &verboseBuf,
		IsTTY:     false,
	})

	// Create non-verbose output
	nonVerboseOut := New(Config{
		Verbose:   false,
		Writer:    &nonVerboseBuf,
		ErrWriter: &nonVerboseBuf,
		IsTTY:     false,
	})

	// Test with dry-run result
	dryRunResult := &orchestrator.RunResult{
		Moved: []orchestrator.FileOperation{
			{Source: "/inbound/ABC 2024-01-15 Invoice.pdf", Destination: "/organized/2024 ABC/ABC 2024-01-15 Invoice.pdf", Prefix: "ABC"},
		},
		ForReview: []orchestrator.FileOperation{
			{Source: "/inbound/unknown.pdf", Destination: "/inbound/for-review/unknown.pdf", Reason: "no_prefix_match"},
		},
		Skipped: []orchestrator.FileOperation{},
		Errors:  []error{},
	}

	verboseOut.PrintDryRunResult(dryRunResult)
	nonVerboseOut.PrintDryRunResult(dryRunResult)

	// Verbose output should be longer due to additional details
	if verboseBuf.Len() <= nonVerboseBuf.Len() {
		t.Errorf("expected verbose output (%d bytes) to be longer than non-verbose output (%d bytes)",
			verboseBuf.Len(), nonVerboseBuf.Len())
	}

	// Reset buffers for status result test
	verboseBuf.Reset()
	nonVerboseBuf.Reset()

	// Test with status result
	statusResult := &orchestrator.StatusResult{
		ByInbound: map[string]*orchestrator.InboundStatus{
			"/inbound": {
				Directory: "/inbound",
				ByDestination: map[string][]string{
					"/organized/2024 ABC": {
						"/inbound/ABC 2024-01-15 Invoice.pdf",
						"/inbound/ABC 2024-02-20 Report.docx",
					},
				},
				Total: 2,
			},
		},
		GrandTotal: 2,
	}

	verboseOut.PrintStatusResult(statusResult)
	nonVerboseOut.PrintStatusResult(statusResult)

	// Verbose output should be longer due to individual file paths
	if verboseBuf.Len() <= nonVerboseBuf.Len() {
		t.Errorf("expected verbose status output (%d bytes) to be longer than non-verbose output (%d bytes)",
			verboseBuf.Len(), nonVerboseBuf.Len())
	}
}
