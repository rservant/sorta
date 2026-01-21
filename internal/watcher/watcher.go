// Package watcher provides file system monitoring for automatic file organization.
package watcher

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchConfig contains watcher settings.
type WatchConfig struct {
	DebounceSeconds   int      // Delay before processing (default: 2)
	StableThresholdMs int      // File size stability threshold in milliseconds (default: 1000)
	IgnorePatterns    []string // Glob patterns to ignore (e.g., "*.tmp", "*.part", "*.download")
}

// DefaultWatchConfig returns a WatchConfig with sensible defaults.
func DefaultWatchConfig() *WatchConfig {
	return &WatchConfig{
		DebounceSeconds:   2,
		StableThresholdMs: 1000,
		IgnorePatterns:    DefaultIgnorePatterns(),
	}
}

// WatchSummary contains stats from the watch session.
type WatchSummary struct {
	FilesOrganized int
	FilesReviewed  int
	FilesSkipped   int
	Duration       time.Duration
}

// FileHandler is a callback function that processes a file.
// It returns:
// - organized: true if file was moved to an organized destination
// - reviewed: true if file was moved to for-review
// - err: any error that occurred during processing
type FileHandler func(path string) (organized bool, reviewed bool, err error)

// Watcher monitors directories for file changes.
type Watcher struct {
	config      *WatchConfig
	fileHandler FileHandler
	fsWatcher   *fsnotify.Watcher
	fileFilter  *FileFilter
	done        chan struct{}
	wg          sync.WaitGroup
	startTime   time.Time

	// Statistics tracking
	mu             sync.Mutex
	filesOrganized int
	filesReviewed  int
	filesSkipped   int
}

// New creates a new Watcher with the given configuration.
// If config is nil, default configuration is used.
// The fileHandler is called for each file that needs to be organized.
func New(config *WatchConfig, fileHandler FileHandler) *Watcher {
	if config == nil {
		config = DefaultWatchConfig()
	}
	return &Watcher{
		config:      config,
		fileHandler: fileHandler,
		fileFilter:  NewFileFilter(config.IgnorePatterns),
		done:        make(chan struct{}),
	}
}

// Start begins watching the specified directories for file changes.
// It returns an error if the watcher cannot be initialized.
// The watcher runs until Stop() is called.
func (w *Watcher) Start(dirs []string) error {
	var err error
	w.fsWatcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Add directories to watch
	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			w.fsWatcher.Close()
			return err
		}
		if err := w.fsWatcher.Add(absDir); err != nil {
			w.fsWatcher.Close()
			return err
		}
	}

	w.startTime = time.Now()
	w.done = make(chan struct{})

	// Start the event processing goroutine
	w.wg.Add(1)
	go w.processEvents()

	return nil
}

// Stop gracefully shuts down the watcher and returns a summary of the session.
func (w *Watcher) Stop() *WatchSummary {
	// Signal the event processing goroutine to stop
	close(w.done)

	// Wait for the goroutine to finish
	w.wg.Wait()

	// Close the fsnotify watcher
	if w.fsWatcher != nil {
		w.fsWatcher.Close()
	}

	// Build and return the summary
	w.mu.Lock()
	defer w.mu.Unlock()

	return &WatchSummary{
		FilesOrganized: w.filesOrganized,
		FilesReviewed:  w.filesReviewed,
		FilesSkipped:   w.filesSkipped,
		Duration:       time.Since(w.startTime),
	}
}

// processEvents handles file system events from fsnotify.
func (w *Watcher) processEvents() {
	defer w.wg.Done()

	for {
		select {
		case <-w.done:
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			// Only process Create events (new files)
			if event.Op&fsnotify.Create == fsnotify.Create {
				w.handleFileEvent(event.Name)
			}
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			// Log error but continue watching
			_ = err // TODO: Add proper logging
		}
	}
}

// handleFileEvent processes a single file event.
// This is a placeholder that will be enhanced with debouncing and stability checking
// in subsequent tasks.
func (w *Watcher) handleFileEvent(path string) {
	// Check if file should be ignored based on patterns
	if w.shouldIgnore(path) {
		w.mu.Lock()
		w.filesSkipped++
		w.mu.Unlock()
		return
	}

	// Call the file handler if provided
	if w.fileHandler != nil {
		organized, reviewed, err := w.fileHandler(path)
		w.mu.Lock()
		if err != nil {
			w.filesSkipped++
		} else if organized {
			w.filesOrganized++
		} else if reviewed {
			w.filesReviewed++
		} else {
			w.filesSkipped++
		}
		w.mu.Unlock()
		return
	}

	// No handler provided, just count as organized for testing purposes
	w.mu.Lock()
	w.filesOrganized++
	w.mu.Unlock()
}

// shouldIgnore checks if a file path matches any of the ignore patterns.
func (w *Watcher) shouldIgnore(path string) bool {
	return w.fileFilter.ShouldIgnore(path)
}

// GetConfig returns the current watcher configuration.
func (w *Watcher) GetConfig() *WatchConfig {
	return w.config
}

// IsRunning returns true if the watcher is currently running.
func (w *Watcher) IsRunning() bool {
	select {
	case <-w.done:
		return false
	default:
		return w.fsWatcher != nil
	}
}
