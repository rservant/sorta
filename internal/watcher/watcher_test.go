package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWatcher_NewFile_TriggersOrganization verifies that when a new file is created
// in a watched directory, the file handler is called to organize it.
// Validates: Requirements 1.1, 1.2
func TestWatcher_NewFile_TriggersOrganization(t *testing.T) {
	tmpDir := t.TempDir()

	var handlerCalled atomic.Int32
	var handledPath string
	var mu sync.Mutex

	handler := func(path string) (organized bool, reviewed bool, err error) {
		mu.Lock()
		handledPath = path
		mu.Unlock()
		handlerCalled.Add(1)
		return true, false, nil
	}

	config := &WatchConfig{
		DebounceSeconds:   0, // No debounce for this test
		StableThresholdMs: 0, // No stability check for this test
		IgnorePatterns:    []string{},
	}

	w := New(config, handler)
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Create a new file in the watched directory
	testFile := filepath.Join(tmpDir, "test-document.pdf")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for the event to be processed
	time.Sleep(200 * time.Millisecond)

	if handlerCalled.Load() != 1 {
		t.Errorf("Expected handler to be called once, got %d", handlerCalled.Load())
	}

	mu.Lock()
	if handledPath != testFile {
		t.Errorf("Expected handled path %s, got %s", testFile, handledPath)
	}
	mu.Unlock()
}

// TestWatcher_TmpFilesIgnored verifies that .tmp files are not processed.
// Validates: Requirement 1.8
func TestWatcher_TmpFilesIgnored(t *testing.T) {
	tmpDir := t.TempDir()

	var handlerCalled atomic.Int32

	handler := func(path string) (organized bool, reviewed bool, err error) {
		handlerCalled.Add(1)
		return true, false, nil
	}

	config := DefaultWatchConfig()
	config.DebounceSeconds = 0
	config.StableThresholdMs = 0

	w := New(config, handler)
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Create a .tmp file - should be ignored
	tmpFile := filepath.Join(tmpDir, "download.tmp")
	if err := os.WriteFile(tmpFile, []byte("temp content"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Wait for potential event processing
	time.Sleep(200 * time.Millisecond)

	if handlerCalled.Load() != 0 {
		t.Errorf("Expected handler NOT to be called for .tmp file, got %d calls", handlerCalled.Load())
	}

	// Verify the file was counted as skipped
	summary := w.Stop()
	if summary.FilesSkipped != 1 {
		t.Errorf("Expected 1 file skipped, got %d", summary.FilesSkipped)
	}
}

// TestWatcher_PartFilesIgnored verifies that .part files are not processed.
// Validates: Requirement 1.8
func TestWatcher_PartFilesIgnored(t *testing.T) {
	tmpDir := t.TempDir()

	var handlerCalled atomic.Int32

	handler := func(path string) (organized bool, reviewed bool, err error) {
		handlerCalled.Add(1)
		return true, false, nil
	}

	config := DefaultWatchConfig()
	config.DebounceSeconds = 0
	config.StableThresholdMs = 0

	w := New(config, handler)
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Create a .part file - should be ignored
	partFile := filepath.Join(tmpDir, "video.part")
	if err := os.WriteFile(partFile, []byte("partial content"), 0644); err != nil {
		t.Fatalf("Failed to create part file: %v", err)
	}

	// Wait for potential event processing
	time.Sleep(200 * time.Millisecond)

	if handlerCalled.Load() != 0 {
		t.Errorf("Expected handler NOT to be called for .part file, got %d calls", handlerCalled.Load())
	}
}

// TestWatcher_DownloadFilesIgnored verifies that .download files are not processed.
// Validates: Requirement 1.8
func TestWatcher_DownloadFilesIgnored(t *testing.T) {
	tmpDir := t.TempDir()

	var handlerCalled atomic.Int32

	handler := func(path string) (organized bool, reviewed bool, err error) {
		handlerCalled.Add(1)
		return true, false, nil
	}

	config := DefaultWatchConfig()
	config.DebounceSeconds = 0
	config.StableThresholdMs = 0

	w := New(config, handler)
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Create a .download file - should be ignored
	downloadFile := filepath.Join(tmpDir, "archive.download")
	if err := os.WriteFile(downloadFile, []byte("downloading content"), 0644); err != nil {
		t.Fatalf("Failed to create download file: %v", err)
	}

	// Wait for potential event processing
	time.Sleep(200 * time.Millisecond)

	if handlerCalled.Load() != 0 {
		t.Errorf("Expected handler NOT to be called for .download file, got %d calls", handlerCalled.Load())
	}
}

// TestWatcher_Summary_TracksOrganizedFiles verifies that the summary correctly
// counts files that were organized.
// Validates: Requirements 1.1, 1.2
func TestWatcher_Summary_TracksOrganizedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	handler := func(path string) (organized bool, reviewed bool, err error) {
		return true, false, nil // File was organized
	}

	config := &WatchConfig{
		DebounceSeconds:   0,
		StableThresholdMs: 0,
		IgnorePatterns:    []string{},
	}

	w := New(config, handler)
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Create multiple files
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(tmpDir, filepath.Base(t.Name())+string(rune('a'+i))+".pdf")
		if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Wait for events to be processed
	time.Sleep(300 * time.Millisecond)

	summary := w.Stop()
	if summary.FilesOrganized != 3 {
		t.Errorf("Expected 3 files organized, got %d", summary.FilesOrganized)
	}
}

// TestWatcher_Summary_TracksReviewedFiles verifies that the summary correctly
// counts files that were sent to for-review.
// Validates: Requirements 1.1, 1.2
func TestWatcher_Summary_TracksReviewedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	handler := func(path string) (organized bool, reviewed bool, err error) {
		return false, true, nil // File was sent to review
	}

	config := &WatchConfig{
		DebounceSeconds:   0,
		StableThresholdMs: 0,
		IgnorePatterns:    []string{},
	}

	w := New(config, handler)
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Create a file
	testFile := filepath.Join(tmpDir, "unknown-file.xyz")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for event to be processed
	time.Sleep(200 * time.Millisecond)

	summary := w.Stop()
	if summary.FilesReviewed != 1 {
		t.Errorf("Expected 1 file reviewed, got %d", summary.FilesReviewed)
	}
}

// TestWatcher_Summary_TracksSkippedFiles verifies that the summary correctly
// counts files that were skipped (including temp files and errors).
// Validates: Requirements 1.1, 1.8
func TestWatcher_Summary_TracksSkippedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	handler := func(path string) (organized bool, reviewed bool, err error) {
		return false, false, nil // File was skipped
	}

	config := DefaultWatchConfig()
	config.DebounceSeconds = 0
	config.StableThresholdMs = 0

	w := New(config, handler)
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Create a temp file (should be skipped due to pattern)
	tmpFile := filepath.Join(tmpDir, "file.tmp")
	if err := os.WriteFile(tmpFile, []byte("temp"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Create a regular file that handler skips
	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("regular"), 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	// Wait for events to be processed
	time.Sleep(300 * time.Millisecond)

	summary := w.Stop()
	if summary.FilesSkipped != 2 {
		t.Errorf("Expected 2 files skipped, got %d", summary.FilesSkipped)
	}
}

// TestWatcher_StartStop verifies that the watcher can be started and stopped cleanly.
// Validates: Requirements 1.1, 1.6, 1.7
func TestWatcher_StartStop(t *testing.T) {
	tmpDir := t.TempDir()

	w := New(nil, nil)

	// Start the watcher
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	if !w.IsRunning() {
		t.Error("Watcher should be running after Start")
	}

	// Stop the watcher
	summary := w.Stop()

	if w.IsRunning() {
		t.Error("Watcher should not be running after Stop")
	}

	if summary == nil {
		t.Error("Stop should return a summary")
	}
}

// TestWatcher_StartWithInvalidDirectory verifies that starting with an invalid
// directory returns an error.
func TestWatcher_StartWithInvalidDirectory(t *testing.T) {
	w := New(nil, nil)

	err := w.Start([]string{"/nonexistent/directory/path"})
	if err == nil {
		t.Error("Expected error when starting with invalid directory")
		w.Stop()
	}
}

// TestWatcher_MultipleDirectories verifies that the watcher can monitor
// multiple directories simultaneously.
// Validates: Requirement 1.1
func TestWatcher_MultipleDirectories(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	var handlerCalled atomic.Int32
	var mu sync.Mutex
	handledPaths := make(map[string]bool)

	handler := func(path string) (organized bool, reviewed bool, err error) {
		mu.Lock()
		handledPaths[path] = true
		mu.Unlock()
		handlerCalled.Add(1)
		return true, false, nil
	}

	config := &WatchConfig{
		DebounceSeconds:   0,
		StableThresholdMs: 0,
		IgnorePatterns:    []string{},
	}

	w := New(config, handler)
	if err := w.Start([]string{tmpDir1, tmpDir2}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Create files in both directories
	file1 := filepath.Join(tmpDir1, "file1.pdf")
	file2 := filepath.Join(tmpDir2, "file2.pdf")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Wait for events to be processed
	time.Sleep(300 * time.Millisecond)

	if handlerCalled.Load() != 2 {
		t.Errorf("Expected handler to be called twice, got %d", handlerCalled.Load())
	}

	mu.Lock()
	if !handledPaths[file1] {
		t.Errorf("Expected file1 to be handled")
	}
	if !handledPaths[file2] {
		t.Errorf("Expected file2 to be handled")
	}
	mu.Unlock()
}

// TestWatcher_DefaultConfig verifies that default configuration values are applied.
// Validates: Requirements 2.1, 2.2, 2.3, 2.4
func TestWatcher_DefaultConfig(t *testing.T) {
	config := DefaultWatchConfig()

	if config.DebounceSeconds != 2 {
		t.Errorf("Expected default debounce 2s, got %d", config.DebounceSeconds)
	}

	if config.StableThresholdMs != 1000 {
		t.Errorf("Expected default stability threshold 1000ms, got %d", config.StableThresholdMs)
	}

	if len(config.IgnorePatterns) == 0 {
		t.Error("Expected default ignore patterns to be set")
	}

	// Verify default patterns include required ones
	patterns := config.IgnorePatterns
	hasRequired := map[string]bool{"*.tmp": false, "*.part": false, "*.download": false}
	for _, p := range patterns {
		if _, ok := hasRequired[p]; ok {
			hasRequired[p] = true
		}
	}
	for pattern, found := range hasRequired {
		if !found {
			t.Errorf("Expected default patterns to include %s", pattern)
		}
	}
}

// TestWatcher_NilConfig verifies that nil config uses defaults.
func TestWatcher_NilConfig(t *testing.T) {
	w := New(nil, nil)

	config := w.GetConfig()
	if config == nil {
		t.Fatal("GetConfig should not return nil")
	}

	if config.DebounceSeconds != 2 {
		t.Errorf("Expected default debounce 2s, got %d", config.DebounceSeconds)
	}
}

// TestWatcher_HandlerError_CountsAsSkipped verifies that when the handler
// returns an error, the file is counted as skipped.
func TestWatcher_HandlerError_CountsAsSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	handler := func(path string) (organized bool, reviewed bool, err error) {
		return false, false, os.ErrPermission // Simulate an error
	}

	config := &WatchConfig{
		DebounceSeconds:   0,
		StableThresholdMs: 0,
		IgnorePatterns:    []string{},
	}

	w := New(config, handler)
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Create a file
	testFile := filepath.Join(tmpDir, "error-file.pdf")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for event to be processed
	time.Sleep(200 * time.Millisecond)

	summary := w.Stop()
	if summary.FilesSkipped != 1 {
		t.Errorf("Expected 1 file skipped due to error, got %d", summary.FilesSkipped)
	}
	if summary.FilesOrganized != 0 {
		t.Errorf("Expected 0 files organized, got %d", summary.FilesOrganized)
	}
}

// TestWatcher_NoHandler_CountsAsOrganized verifies that when no handler is
// provided, files are counted as organized (for testing purposes).
func TestWatcher_NoHandler_CountsAsOrganized(t *testing.T) {
	tmpDir := t.TempDir()

	config := &WatchConfig{
		DebounceSeconds:   0,
		StableThresholdMs: 0,
		IgnorePatterns:    []string{},
	}

	w := New(config, nil) // No handler
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Create a file
	testFile := filepath.Join(tmpDir, "test-file.pdf")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for event to be processed
	time.Sleep(200 * time.Millisecond)

	summary := w.Stop()
	if summary.FilesOrganized != 1 {
		t.Errorf("Expected 1 file organized (no handler), got %d", summary.FilesOrganized)
	}
}

// TestWatcher_SummaryDuration verifies that the summary includes the watch duration.
// Validates: Requirement 1.7
func TestWatcher_SummaryDuration(t *testing.T) {
	tmpDir := t.TempDir()

	w := New(nil, nil)
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	summary := w.Stop()
	if summary.Duration < 100*time.Millisecond {
		t.Errorf("Expected duration >= 100ms, got %v", summary.Duration)
	}
}

// TestWatcher_CustomIgnorePatterns verifies that custom ignore patterns work.
// Validates: Requirement 2.3
func TestWatcher_CustomIgnorePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	var handlerCalled atomic.Int32

	handler := func(path string) (organized bool, reviewed bool, err error) {
		handlerCalled.Add(1)
		return true, false, nil
	}

	config := &WatchConfig{
		DebounceSeconds:   0,
		StableThresholdMs: 0,
		IgnorePatterns:    []string{"*.bak", "*.swp"}, // Custom patterns
	}

	w := New(config, handler)
	if err := w.Start([]string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Create a .bak file - should be ignored
	bakFile := filepath.Join(tmpDir, "backup.bak")
	if err := os.WriteFile(bakFile, []byte("backup"), 0644); err != nil {
		t.Fatalf("Failed to create bak file: %v", err)
	}

	// Create a .swp file - should be ignored
	swpFile := filepath.Join(tmpDir, "editor.swp")
	if err := os.WriteFile(swpFile, []byte("swap"), 0644); err != nil {
		t.Fatalf("Failed to create swp file: %v", err)
	}

	// Create a regular file - should be processed
	regularFile := filepath.Join(tmpDir, "document.pdf")
	if err := os.WriteFile(regularFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	// Wait for events to be processed
	time.Sleep(300 * time.Millisecond)

	if handlerCalled.Load() != 1 {
		t.Errorf("Expected handler to be called once (for regular file), got %d", handlerCalled.Load())
	}
}

// TestDebouncer_Integration_RapidEventsCoalesced verifies that rapid file events
// for the same file are coalesced into a single processing call.
// Validates: Requirement 1.3
func TestDebouncer_Integration_RapidEventsCoalesced(t *testing.T) {
	var callCount atomic.Int32
	var mu sync.Mutex
	calledPaths := make(map[string]int)

	delay := 100 * time.Millisecond
	d := NewDebouncer(delay, func(path string) {
		mu.Lock()
		calledPaths[path]++
		mu.Unlock()
		callCount.Add(1)
	})

	// Simulate rapid events for the same file (like a file being written in chunks)
	for i := 0; i < 10; i++ {
		d.Add("/test/file.txt")
		time.Sleep(20 * time.Millisecond) // Less than debounce delay
	}

	// Wait for debounce to complete
	time.Sleep(delay + 50*time.Millisecond)

	if callCount.Load() != 1 {
		t.Errorf("Expected 1 call (coalesced), got %d", callCount.Load())
	}

	mu.Lock()
	if calledPaths["/test/file.txt"] != 1 {
		t.Errorf("Expected file to be processed once, got %d", calledPaths["/test/file.txt"])
	}
	mu.Unlock()
}

// TestStabilityChecker_Integration_WaitsForStableSize verifies that the stability
// checker waits for file size to stabilize before returning.
// Validates: Requirement 1.4
func TestStabilityChecker_Integration_WaitsForStableSize(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "growing.txt")

	// Create initial file
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Use short threshold for testing
	s := NewStabilityCheckerWithOptions(150*time.Millisecond, 2*time.Second, 50*time.Millisecond)

	// Start writing to file in background
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 3; i++ {
			time.Sleep(50 * time.Millisecond)
			f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return
			}
			f.WriteString("more")
			f.Close()
		}
	}()

	// Wait for stability
	start := time.Now()
	err := s.WaitForStable(testFile)
	elapsed := time.Since(start)
	<-done

	if err != nil {
		t.Errorf("Expected file to stabilize, got error: %v", err)
	}

	// Should have waited at least until writes stopped + threshold
	// Writes take ~150ms (3 * 50ms), then threshold is 150ms
	if elapsed < 200*time.Millisecond {
		t.Errorf("Expected to wait at least 200ms, waited %v", elapsed)
	}
}

// TestFileFilter_Integration_AllTempPatternsIgnored verifies that all default
// temporary file patterns are correctly ignored.
// Validates: Requirement 1.8
func TestFileFilter_Integration_AllTempPatternsIgnored(t *testing.T) {
	filter := NewFileFilter(nil) // Use defaults

	tempFiles := []string{
		"file.tmp",
		"download.part",
		"archive.download",
		"chrome.crdownload",
		"data.partial",
		".~lock.file",
	}

	for _, file := range tempFiles {
		if !filter.ShouldIgnore(file) {
			t.Errorf("Expected %s to be ignored", file)
		}
	}

	regularFiles := []string{
		"document.pdf",
		"image.jpg",
		"video.mp4",
		"archive.zip",
		"readme.txt",
	}

	for _, file := range regularFiles {
		if filter.ShouldIgnore(file) {
			t.Errorf("Expected %s NOT to be ignored", file)
		}
	}
}
