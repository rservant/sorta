// Package output handles CLI output formatting including verbose mode and progress indicators.
package output

import (
	"fmt"
	"io"
	"os"
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
