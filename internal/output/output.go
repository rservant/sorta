// Package output handles CLI output formatting including verbose mode and progress indicators.
package output

import (
	"fmt"
	"io"
	"os"
	"sort"
	"sorta/internal/orchestrator"
	"strings"
	"sync"

	"golang.org/x/term"
)

// Config holds output configuration.
type Config struct {
	Verbose   bool      // Enable verbose output
	Writer    io.Writer // Output destination (default: os.Stdout)
	ErrWriter io.Writer // Error output destination (default: os.Stderr)
	IsTTY     bool      // Whether output is a terminal
}

// Output handles formatted output with verbose and progress support.
type Output struct {
	config          Config
	progressActive  bool
	progressTotal   int
	progressCurrent int
	progressMu      sync.Mutex
}

// New creates a new Output instance with the given configuration.
func New(config Config) *Output {
	if config.Writer == nil {
		config.Writer = os.Stdout
	}
	if config.ErrWriter == nil {
		config.ErrWriter = os.Stderr
	}
	return &Output{
		config: config,
	}
}

// DefaultConfig returns a Config with sensible defaults and TTY detection.
func DefaultConfig() Config {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	return Config{
		Verbose:   false,
		Writer:    os.Stdout,
		ErrWriter: os.Stderr,
		IsTTY:     isTTY,
	}
}

// Verbose prints a message only when verbose mode is enabled.
func (o *Output) Verbose(format string, args ...interface{}) {
	if !o.config.Verbose {
		return
	}
	o.clearProgressLine()
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprint(o.config.Writer, msg)
}

// Info prints an informational message (always shown).
func (o *Output) Info(format string, args ...interface{}) {
	o.clearProgressLine()
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprint(o.config.Writer, msg)
}

// Error prints an error message to stderr.
func (o *Output) Error(format string, args ...interface{}) {
	o.clearProgressLine()
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprint(o.config.ErrWriter, msg)
}

// clearProgressLine clears the current progress line if active.
func (o *Output) clearProgressLine() {
	o.progressMu.Lock()
	defer o.progressMu.Unlock()
	if o.progressActive && o.config.IsTTY {
		// Clear the line with spaces and return to start
		fmt.Fprint(o.config.Writer, "\r"+strings.Repeat(" ", 60)+"\r")
	}
}

// StartProgress begins a progress indicator session.
func (o *Output) StartProgress(total int) {
	// Suppress progress when not TTY or when verbose mode is enabled
	if !o.config.IsTTY || o.config.Verbose {
		return
	}
	o.progressMu.Lock()
	defer o.progressMu.Unlock()
	o.progressActive = true
	o.progressTotal = total
	o.progressCurrent = 0
}

// UpdateProgress updates the progress indicator.
func (o *Output) UpdateProgress(current int, message string) {
	// Suppress progress when not TTY or when verbose mode is enabled
	if !o.config.IsTTY || o.config.Verbose {
		return
	}
	o.progressMu.Lock()
	defer o.progressMu.Unlock()
	if !o.progressActive {
		return
	}
	o.progressCurrent = current
	// Use carriage return for in-place updates
	progressMsg := fmt.Sprintf("\rProcessing file %d/%d...", current, o.progressTotal)
	if message != "" {
		progressMsg = fmt.Sprintf("\r%s %d/%d...", message, current, o.progressTotal)
	}
	fmt.Fprint(o.config.Writer, progressMsg)
}

// EndProgress clears the progress indicator.
func (o *Output) EndProgress() {
	// Suppress progress when not TTY or when verbose mode is enabled
	if !o.config.IsTTY || o.config.Verbose {
		return
	}
	o.progressMu.Lock()
	defer o.progressMu.Unlock()
	if !o.progressActive {
		return
	}
	o.progressActive = false
	// Clear the line with spaces and return to start
	fmt.Fprint(o.config.Writer, "\r"+strings.Repeat(" ", 60)+"\r")
}

// IsVerbose returns whether verbose mode is enabled.
func (o *Output) IsVerbose() bool {
	return o.config.Verbose
}

// IsTTY returns whether the output is a terminal.
func (o *Output) IsTTY() bool {
	return o.config.IsTTY
}

// PrintDryRunResult formats and prints dry-run results.
// It shows each planned operation with source → destination format.
// Requirements: 1.2, 1.3, 3.1 - Display dry-run results with source and destination paths
func (o *Output) PrintDryRunResult(result *orchestrator.RunResult) {
	if result == nil {
		return
	}

	// Print files that would be moved
	if len(result.Moved) > 0 {
		o.Info("Files to be moved:")
		for _, op := range result.Moved {
			o.Info("  %s → %s", op.Source, op.Destination)
			if o.config.Verbose && op.Prefix != "" {
				o.Verbose("    Matched prefix: %s", op.Prefix)
			}
		}
		o.Info("")
	}

	// Print files that would go to for-review
	// Requirements: 1.3 - Display files that would go to for-review directories
	if len(result.ForReview) > 0 {
		o.Info("Files for review:")
		for _, op := range result.ForReview {
			o.Info("  %s → %s", op.Source, op.Destination)
			if o.config.Verbose && op.Reason != "" {
				o.Verbose("    Reason: %s", op.Reason)
			}
		}
		o.Info("")
	}

	// Print skipped files
	if len(result.Skipped) > 0 {
		o.Info("Files to be skipped:")
		for _, op := range result.Skipped {
			o.Info("  %s", op.Source)
			if op.Reason != "" {
				o.Info("    Reason: %s", op.Reason)
			}
		}
		o.Info("")
	}

	// Print errors
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			o.Error("Error: %v", err)
		}
		o.Info("")
	}
}

// PrintStatusResult formats and prints status results.
// It groups files by destination directory.
// Requirements: 2.2, 2.3, 3.2 - Display status results grouped by destination
func (o *Output) PrintStatusResult(result *orchestrator.StatusResult) {
	if result == nil {
		return
	}

	// Handle empty results
	// Requirements: 2.5 - Display message when no pending files exist
	if result.GrandTotal == 0 {
		o.Info("All directories are organized. No pending files found.")
		return
	}

	// Sort inbound directories for consistent output
	inboundDirs := make([]string, 0, len(result.ByInbound))
	for dir := range result.ByInbound {
		inboundDirs = append(inboundDirs, dir)
	}
	sort.Strings(inboundDirs)

	// Print status for each inbound directory
	for _, inboundDir := range inboundDirs {
		status := result.ByInbound[inboundDir]
		if status.Total == 0 {
			continue
		}

		o.Info("Inbound: %s (%d files)", status.Directory, status.Total)

		// Sort destinations for consistent output
		destinations := make([]string, 0, len(status.ByDestination))
		for dest := range status.ByDestination {
			destinations = append(destinations, dest)
		}
		sort.Strings(destinations)

		// Group files by destination
		// Requirements: 2.2, 3.2 - Group files by destination directory
		for _, dest := range destinations {
			files := status.ByDestination[dest]
			o.Info("  → %s (%d files)", dest, len(files))
			if o.config.Verbose {
				for _, file := range files {
					o.Verbose("      %s", file)
				}
			}
		}
		o.Info("")
	}

	// Print grand total
	// Requirements: 2.4 - Display grand total of all pending files
	o.Info("Total pending files: %d", result.GrandTotal)
}

// PrintSummary prints operation summary counts.
// Requirements: 1.6 - Display summary count of files that would be moved, reviewed, and skipped
func (o *Output) PrintSummary(moved, forReview, skipped int) {
	total := moved + forReview + skipped

	var parts []string
	if moved > 0 {
		parts = append(parts, fmt.Sprintf("%d moved", moved))
	}
	if forReview > 0 {
		parts = append(parts, fmt.Sprintf("%d for review", forReview))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}

	if len(parts) == 0 {
		o.Info("Summary: No files to process")
		return
	}

	o.Info("Summary: %d files (%s)", total, strings.Join(parts, ", "))
}
