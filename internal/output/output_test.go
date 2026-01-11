package output

import (
	"bytes"
	"regexp"
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
