package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStabilityChecker(t *testing.T) {
	threshold := 100 * time.Millisecond
	s := NewStabilityChecker(threshold)

	if s == nil {
		t.Fatal("NewStabilityChecker returned nil")
	}
	if s.threshold != threshold {
		t.Errorf("expected threshold %v, got %v", threshold, s.threshold)
	}
	if s.timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", s.timeout)
	}
	// Interval should be threshold/4 but at least 50ms
	expectedInterval := threshold / 4
	if expectedInterval < 50*time.Millisecond {
		expectedInterval = 50 * time.Millisecond
	}
	if s.interval != expectedInterval {
		t.Errorf("expected interval %v, got %v", expectedInterval, s.interval)
	}
}

func TestNewStabilityChecker_SmallThreshold(t *testing.T) {
	// With a very small threshold, interval should be at least 50ms
	threshold := 100 * time.Millisecond
	s := NewStabilityChecker(threshold)

	if s.interval < 50*time.Millisecond {
		t.Errorf("interval should be at least 50ms, got %v", s.interval)
	}
}

func TestNewStabilityCheckerWithOptions(t *testing.T) {
	threshold := 200 * time.Millisecond
	timeout := 5 * time.Second
	interval := 100 * time.Millisecond

	s := NewStabilityCheckerWithOptions(threshold, timeout, interval)

	if s.threshold != threshold {
		t.Errorf("expected threshold %v, got %v", threshold, s.threshold)
	}
	if s.timeout != timeout {
		t.Errorf("expected timeout %v, got %v", timeout, s.timeout)
	}
	if s.interval != interval {
		t.Errorf("expected interval %v, got %v", interval, s.interval)
	}
}

func TestStabilityChecker_WaitForStable_StableFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "stable.txt")

	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Use short threshold for testing
	s := NewStabilityCheckerWithOptions(100*time.Millisecond, 2*time.Second, 50*time.Millisecond)

	// File should stabilize quickly since it's not being written to
	err := s.WaitForStable(tmpFile)
	if err != nil {
		t.Errorf("expected no error for stable file, got %v", err)
	}
}

func TestStabilityChecker_WaitForStable_NonExistentFile(t *testing.T) {
	s := NewStabilityChecker(100 * time.Millisecond)

	err := s.WaitForStable("/nonexistent/path/file.txt")
	if err != ErrFileNotFound {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestStabilityChecker_WaitForStable_FileBeingWritten(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "growing.txt")

	// Create initial file
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Use short threshold for testing
	s := NewStabilityCheckerWithOptions(150*time.Millisecond, 2*time.Second, 50*time.Millisecond)

	// Start a goroutine that writes to the file periodically
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 3; i++ {
			time.Sleep(50 * time.Millisecond)
			f, err := os.OpenFile(tmpFile, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return
			}
			f.WriteString("more data")
			f.Close()
		}
	}()

	// Wait for the file to stabilize
	err := s.WaitForStable(tmpFile)
	<-done // Ensure writer goroutine completes

	if err != nil {
		t.Errorf("expected file to eventually stabilize, got %v", err)
	}
}

func TestStabilityChecker_WaitForStableWithContext_Cancelled(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	s := NewStabilityCheckerWithOptions(5*time.Second, 30*time.Second, 100*time.Millisecond)

	// Create a context that we'll cancel immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := s.WaitForStableWithContext(ctx, tmpFile)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestStabilityChecker_WaitForStable_FileDeletedDuringWait(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "todelete.txt")

	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Use longer threshold so we have time to delete
	s := NewStabilityCheckerWithOptions(500*time.Millisecond, 2*time.Second, 50*time.Millisecond)

	// Start waiting in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.WaitForStable(tmpFile)
	}()

	// Delete the file after a short delay
	time.Sleep(100 * time.Millisecond)
	os.Remove(tmpFile)

	// Should get ErrFileNotFound
	err := <-errCh
	if err != ErrFileNotFound {
		t.Errorf("expected ErrFileNotFound after file deletion, got %v", err)
	}
}

func TestStabilityChecker_IsStable_StableFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "stable.txt")

	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Use very short threshold for testing
	s := NewStabilityChecker(50 * time.Millisecond)

	if !s.IsStable(tmpFile) {
		t.Error("expected stable file to return true")
	}
}

func TestStabilityChecker_IsStable_NonExistentFile(t *testing.T) {
	s := NewStabilityChecker(50 * time.Millisecond)

	if s.IsStable("/nonexistent/path/file.txt") {
		t.Error("expected non-existent file to return false")
	}
}

func TestStabilityChecker_IsStable_GrowingFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "growing.txt")

	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Use threshold that's long enough for us to write during
	s := NewStabilityChecker(200 * time.Millisecond)

	// Start checking stability in a goroutine
	resultCh := make(chan bool, 1)
	go func() {
		resultCh <- s.IsStable(tmpFile)
	}()

	// Write to the file during the stability check
	time.Sleep(100 * time.Millisecond)
	f, err := os.OpenFile(tmpFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open file for writing: %v", err)
	}
	f.WriteString("more data")
	f.Close()

	// Should return false because file changed
	if <-resultCh {
		t.Error("expected growing file to return false")
	}
}

func TestStabilityChecker_IsStableQuick(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "stable.txt")

	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	s := NewStabilityChecker(1 * time.Second) // Long threshold

	// Quick check with short interval should still work for stable file
	if !s.IsStableQuick(tmpFile, 50*time.Millisecond) {
		t.Error("expected stable file to return true with quick check")
	}
}

func TestStabilityChecker_IsStableQuick_NonExistentFile(t *testing.T) {
	s := NewStabilityChecker(100 * time.Millisecond)

	if s.IsStableQuick("/nonexistent/path/file.txt", 50*time.Millisecond) {
		t.Error("expected non-existent file to return false")
	}
}

func TestStabilityChecker_GetThreshold(t *testing.T) {
	threshold := 500 * time.Millisecond
	s := NewStabilityChecker(threshold)

	if s.GetThreshold() != threshold {
		t.Errorf("expected threshold %v, got %v", threshold, s.GetThreshold())
	}
}

func TestStabilityChecker_GetTimeout(t *testing.T) {
	s := NewStabilityChecker(100 * time.Millisecond)

	if s.GetTimeout() != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", s.GetTimeout())
	}

	s2 := NewStabilityCheckerWithOptions(100*time.Millisecond, 5*time.Second, 50*time.Millisecond)
	if s2.GetTimeout() != 5*time.Second {
		t.Errorf("expected custom timeout 5s, got %v", s2.GetTimeout())
	}
}

func TestStabilityChecker_GetInterval(t *testing.T) {
	s := NewStabilityCheckerWithOptions(200*time.Millisecond, 30*time.Second, 75*time.Millisecond)

	if s.GetInterval() != 75*time.Millisecond {
		t.Errorf("expected interval 75ms, got %v", s.GetInterval())
	}
}
