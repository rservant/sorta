// Package watcher provides file system monitoring for automatic file organization.
package watcher

import (
	"context"
	"errors"
	"os"
	"time"
)

// ErrFileNotFound is returned when the file does not exist.
var ErrFileNotFound = errors.New("file not found")

// ErrFileUnstable is returned when the file does not stabilize within the timeout.
var ErrFileUnstable = errors.New("file did not stabilize within timeout")

// StabilityChecker waits for file size to stabilize before processing.
// This is useful for detecting when a file is still being written.
type StabilityChecker struct {
	threshold time.Duration // Time the file size must remain unchanged
	timeout   time.Duration // Maximum time to wait for stability
	interval  time.Duration // How often to check file size
}

// NewStabilityChecker creates a new StabilityChecker with the specified threshold.
// The threshold is the duration the file size must remain unchanged to be considered stable.
// Default timeout is 30 seconds, default check interval is threshold/4.
func NewStabilityChecker(threshold time.Duration) *StabilityChecker {
	interval := threshold / 4
	if interval < 50*time.Millisecond {
		interval = 50 * time.Millisecond
	}
	return &StabilityChecker{
		threshold: threshold,
		timeout:   30 * time.Second,
		interval:  interval,
	}
}

// NewStabilityCheckerWithOptions creates a StabilityChecker with custom timeout and interval.
func NewStabilityCheckerWithOptions(threshold, timeout, interval time.Duration) *StabilityChecker {
	return &StabilityChecker{
		threshold: threshold,
		timeout:   timeout,
		interval:  interval,
	}
}

// WaitForStable blocks until the file size is stable for the threshold duration.
// It returns an error if the file doesn't exist, cannot be accessed, or doesn't
// stabilize within the timeout period.
func (s *StabilityChecker) WaitForStable(path string) error {
	return s.WaitForStableWithContext(context.Background(), path)
}

// WaitForStableWithContext blocks until the file size is stable, with context support.
// This allows for cancellation of the wait operation.
func (s *StabilityChecker) WaitForStableWithContext(ctx context.Context, path string) error {
	// Create a timeout context if not already set
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Get initial file size
	lastSize, err := s.getFileSize(path)
	if err != nil {
		return err
	}
	lastChangeTime := time.Now()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return ErrFileUnstable
			}
			return ctx.Err()
		case <-ticker.C:
			currentSize, err := s.getFileSize(path)
			if err != nil {
				// File might have been deleted or moved
				if os.IsNotExist(err) {
					return ErrFileNotFound
				}
				return err
			}

			if currentSize != lastSize {
				// File size changed, reset the stability timer
				lastSize = currentSize
				lastChangeTime = time.Now()
			} else if time.Since(lastChangeTime) >= s.threshold {
				// File has been stable for the threshold duration
				return nil
			}
		}
	}
}

// IsStable returns true if the file size hasn't changed over the threshold duration.
// This is a non-blocking check that samples the file size twice with the threshold
// duration between samples.
func (s *StabilityChecker) IsStable(path string) bool {
	// Get initial file size
	initialSize, err := s.getFileSize(path)
	if err != nil {
		return false
	}

	// Wait for the threshold duration
	time.Sleep(s.threshold)

	// Get final file size
	finalSize, err := s.getFileSize(path)
	if err != nil {
		return false
	}

	// File is stable if size hasn't changed
	return initialSize == finalSize
}

// IsStableQuick performs a quick stability check by comparing two samples
// taken at the specified interval. This is faster than IsStable but less accurate.
func (s *StabilityChecker) IsStableQuick(path string, sampleInterval time.Duration) bool {
	// Get initial file size
	initialSize, err := s.getFileSize(path)
	if err != nil {
		return false
	}

	// Wait for the sample interval
	time.Sleep(sampleInterval)

	// Get final file size
	finalSize, err := s.getFileSize(path)
	if err != nil {
		return false
	}

	// File is stable if size hasn't changed
	return initialSize == finalSize
}

// getFileSize returns the size of the file at the given path.
func (s *StabilityChecker) getFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrFileNotFound
		}
		return 0, err
	}
	return info.Size(), nil
}

// GetThreshold returns the configured stability threshold.
func (s *StabilityChecker) GetThreshold() time.Duration {
	return s.threshold
}

// GetTimeout returns the configured timeout duration.
func (s *StabilityChecker) GetTimeout() time.Duration {
	return s.timeout
}

// GetInterval returns the configured check interval.
func (s *StabilityChecker) GetInterval() time.Duration {
	return s.interval
}
