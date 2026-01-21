package watcher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewDebouncer(t *testing.T) {
	delay := 100 * time.Millisecond
	callback := func(path string) {}

	d := NewDebouncer(delay, callback)

	if d == nil {
		t.Fatal("NewDebouncer returned nil")
	}
	if d.delay != delay {
		t.Errorf("expected delay %v, got %v", delay, d.delay)
	}
	if d.pending == nil {
		t.Error("pending map should be initialized")
	}
	if d.PendingCount() != 0 {
		t.Errorf("expected 0 pending, got %d", d.PendingCount())
	}
}

func TestDebouncer_Add_SingleFile(t *testing.T) {
	var called atomic.Int32
	var calledPath string
	var mu sync.Mutex

	delay := 50 * time.Millisecond
	d := NewDebouncer(delay, func(path string) {
		mu.Lock()
		calledPath = path
		mu.Unlock()
		called.Add(1)
	})

	d.Add("/test/file.txt")

	// Should be pending immediately
	if !d.IsPending("/test/file.txt") {
		t.Error("file should be pending after Add")
	}

	// Wait for debounce delay plus some buffer
	time.Sleep(delay + 30*time.Millisecond)

	// Callback should have been called exactly once
	if called.Load() != 1 {
		t.Errorf("expected callback to be called once, got %d", called.Load())
	}

	mu.Lock()
	if calledPath != "/test/file.txt" {
		t.Errorf("expected path /test/file.txt, got %s", calledPath)
	}
	mu.Unlock()

	// Should no longer be pending
	if d.IsPending("/test/file.txt") {
		t.Error("file should not be pending after callback")
	}
}

func TestDebouncer_Add_CoalescesRapidEvents(t *testing.T) {
	var callCount atomic.Int32

	delay := 100 * time.Millisecond
	d := NewDebouncer(delay, func(path string) {
		callCount.Add(1)
	})

	// Add the same file multiple times rapidly
	for i := 0; i < 5; i++ {
		d.Add("/test/file.txt")
		time.Sleep(20 * time.Millisecond) // Less than debounce delay
	}

	// Should still be pending (timer keeps getting reset)
	if !d.IsPending("/test/file.txt") {
		t.Error("file should still be pending")
	}

	// Wait for debounce delay after last Add
	time.Sleep(delay + 30*time.Millisecond)

	// Callback should have been called exactly once (events coalesced)
	if callCount.Load() != 1 {
		t.Errorf("expected callback to be called once (coalesced), got %d", callCount.Load())
	}
}

func TestDebouncer_Add_MultipleFiles(t *testing.T) {
	var mu sync.Mutex
	calledPaths := make(map[string]int)

	delay := 50 * time.Millisecond
	d := NewDebouncer(delay, func(path string) {
		mu.Lock()
		calledPaths[path]++
		mu.Unlock()
	})

	// Add multiple different files
	d.Add("/test/file1.txt")
	d.Add("/test/file2.txt")
	d.Add("/test/file3.txt")

	// All should be pending
	if d.PendingCount() != 3 {
		t.Errorf("expected 3 pending, got %d", d.PendingCount())
	}

	// Wait for debounce delay
	time.Sleep(delay + 30*time.Millisecond)

	// All callbacks should have been called
	mu.Lock()
	defer mu.Unlock()

	if len(calledPaths) != 3 {
		t.Errorf("expected 3 paths called, got %d", len(calledPaths))
	}
	for _, count := range calledPaths {
		if count != 1 {
			t.Errorf("expected each path to be called once, got %d", count)
		}
	}
}

func TestDebouncer_Cancel(t *testing.T) {
	var called atomic.Int32

	delay := 100 * time.Millisecond
	d := NewDebouncer(delay, func(path string) {
		called.Add(1)
	})

	d.Add("/test/file.txt")

	// Should be pending
	if !d.IsPending("/test/file.txt") {
		t.Error("file should be pending after Add")
	}

	// Cancel before debounce delay expires
	d.Cancel("/test/file.txt")

	// Should no longer be pending
	if d.IsPending("/test/file.txt") {
		t.Error("file should not be pending after Cancel")
	}

	// Wait for what would have been the debounce delay
	time.Sleep(delay + 30*time.Millisecond)

	// Callback should not have been called
	if called.Load() != 0 {
		t.Errorf("expected callback not to be called after Cancel, got %d", called.Load())
	}
}

func TestDebouncer_Cancel_NonExistent(t *testing.T) {
	d := NewDebouncer(100*time.Millisecond, func(path string) {})

	// Should not panic when canceling non-existent file
	d.Cancel("/test/nonexistent.txt")

	if d.PendingCount() != 0 {
		t.Errorf("expected 0 pending, got %d", d.PendingCount())
	}
}

func TestDebouncer_CancelAll(t *testing.T) {
	var called atomic.Int32

	delay := 100 * time.Millisecond
	d := NewDebouncer(delay, func(path string) {
		called.Add(1)
	})

	// Add multiple files
	d.Add("/test/file1.txt")
	d.Add("/test/file2.txt")
	d.Add("/test/file3.txt")

	if d.PendingCount() != 3 {
		t.Errorf("expected 3 pending, got %d", d.PendingCount())
	}

	// Cancel all
	d.CancelAll()

	if d.PendingCount() != 0 {
		t.Errorf("expected 0 pending after CancelAll, got %d", d.PendingCount())
	}

	// Wait for what would have been the debounce delay
	time.Sleep(delay + 30*time.Millisecond)

	// No callbacks should have been called
	if called.Load() != 0 {
		t.Errorf("expected no callbacks after CancelAll, got %d", called.Load())
	}
}

func TestDebouncer_GetDelay(t *testing.T) {
	delay := 2 * time.Second
	d := NewDebouncer(delay, func(path string) {})

	if d.GetDelay() != delay {
		t.Errorf("expected delay %v, got %v", delay, d.GetDelay())
	}
}

func TestDebouncer_NilCallback(t *testing.T) {
	delay := 50 * time.Millisecond
	d := NewDebouncer(delay, nil)

	// Should not panic with nil callback
	d.Add("/test/file.txt")

	// Wait for debounce delay
	time.Sleep(delay + 30*time.Millisecond)

	// Should complete without panic
	if d.IsPending("/test/file.txt") {
		t.Error("file should not be pending after delay")
	}
}

func TestDebouncer_ConcurrentAccess(t *testing.T) {
	var callCount atomic.Int32

	delay := 50 * time.Millisecond
	d := NewDebouncer(delay, func(path string) {
		callCount.Add(1)
	})

	// Simulate concurrent access from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Each goroutine adds the same file multiple times
			for j := 0; j < 5; j++ {
				d.Add("/test/concurrent.txt")
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Wait for debounce delay after all adds complete
	time.Sleep(delay + 50*time.Millisecond)

	// Should have been called exactly once (all events coalesced)
	if callCount.Load() != 1 {
		t.Errorf("expected callback to be called once (coalesced), got %d", callCount.Load())
	}
}
