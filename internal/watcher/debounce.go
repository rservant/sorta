// Package watcher provides file system monitoring for automatic file organization.
package watcher

import (
	"sync"
	"time"
)

// Debouncer delays processing until file activity settles.
// It coalesces rapid events for the same file, ensuring that only one
// callback is triggered after the debounce delay expires.
type Debouncer struct {
	delay    time.Duration
	pending  map[string]*time.Timer
	callback func(path string)
	mu       sync.Mutex
}

// NewDebouncer creates a new Debouncer with the specified delay and callback.
// The callback is invoked for each file path after the debounce delay expires,
// provided no new events for that path have been received.
func NewDebouncer(delay time.Duration, callback func(path string)) *Debouncer {
	return &Debouncer{
		delay:    delay,
		pending:  make(map[string]*time.Timer),
		callback: callback,
	}
}

// Add schedules a file for processing after the debounce delay.
// If the file is already pending, the timer is reset, effectively
// coalescing rapid events for the same file.
func (d *Debouncer) Add(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// If there's already a pending timer for this path, stop it
	if timer, exists := d.pending[path]; exists {
		timer.Stop()
	}

	// Create a new timer that will fire after the debounce delay
	d.pending[path] = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		delete(d.pending, path)
		d.mu.Unlock()

		// Invoke the callback outside the lock to avoid potential deadlocks
		if d.callback != nil {
			d.callback(path)
		}
	})
}

// Cancel removes a pending file from processing.
// If the file is not pending, this is a no-op.
func (d *Debouncer) Cancel(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if timer, exists := d.pending[path]; exists {
		timer.Stop()
		delete(d.pending, path)
	}
}

// CancelAll cancels all pending file processing.
// This is useful during shutdown to prevent callbacks from firing.
func (d *Debouncer) CancelAll() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for path, timer := range d.pending {
		timer.Stop()
		delete(d.pending, path)
	}
}

// PendingCount returns the number of files currently pending processing.
// This is primarily useful for testing.
func (d *Debouncer) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}

// IsPending returns true if the specified file is currently pending processing.
// This is primarily useful for testing.
func (d *Debouncer) IsPending(path string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, exists := d.pending[path]
	return exists
}

// GetDelay returns the configured debounce delay.
func (d *Debouncer) GetDelay() time.Duration {
	return d.delay
}
