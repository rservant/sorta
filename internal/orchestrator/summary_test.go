// Package orchestrator provides tests for the summary generator.
package orchestrator

import (
	"testing"
	"time"
)

// TestGenerateSummary_NilResult tests that GenerateSummary handles nil result gracefully.
func TestGenerateSummary_NilResult(t *testing.T) {
	duration := 5 * time.Second
	summary := GenerateSummary(nil, duration, false)

	if summary == nil {
		t.Fatal("Expected non-nil summary for nil result")
	}
	if summary.Moved != 0 {
		t.Errorf("Expected Moved=0, got %d", summary.Moved)
	}
	if summary.ForReview != 0 {
		t.Errorf("Expected ForReview=0, got %d", summary.ForReview)
	}
	if summary.Skipped != 0 {
		t.Errorf("Expected Skipped=0, got %d", summary.Skipped)
	}
	if summary.Errors != 0 {
		t.Errorf("Expected Errors=0, got %d", summary.Errors)
	}
	if summary.Duration != duration {
		t.Errorf("Expected Duration=%v, got %v", duration, summary.Duration)
	}
	if summary.ByPrefix != nil {
		t.Errorf("Expected ByPrefix=nil for non-verbose mode, got %v", summary.ByPrefix)
	}
}

// TestGenerateSummary_EmptyResult tests summary generation with an empty result.
func TestGenerateSummary_EmptyResult(t *testing.T) {
	result := &RunResult{
		Moved:     []FileOperation{},
		ForReview: []FileOperation{},
		Skipped:   []FileOperation{},
		Errors:    []error{},
	}
	duration := 100 * time.Millisecond

	summary := GenerateSummary(result, duration, false)

	if summary.Moved != 0 {
		t.Errorf("Expected Moved=0, got %d", summary.Moved)
	}
	if summary.ForReview != 0 {
		t.Errorf("Expected ForReview=0, got %d", summary.ForReview)
	}
	if summary.Skipped != 0 {
		t.Errorf("Expected Skipped=0, got %d", summary.Skipped)
	}
	if summary.Errors != 0 {
		t.Errorf("Expected Errors=0, got %d", summary.Errors)
	}
	if summary.Duration != duration {
		t.Errorf("Expected Duration=%v, got %v", duration, summary.Duration)
	}
}

// TestGenerateSummary_MovedCount tests that moved count matches actual moves.
// Requirements: 3.2 - Count of files moved to organized destinations
func TestGenerateSummary_MovedCount(t *testing.T) {
	result := &RunResult{
		Moved: []FileOperation{
			{Source: "/src/file1.pdf", Destination: "/dest/2024 Invoice/file1.pdf", Prefix: "Invoice"},
			{Source: "/src/file2.pdf", Destination: "/dest/2024 Invoice/file2.pdf", Prefix: "Invoice"},
			{Source: "/src/file3.pdf", Destination: "/dest/2024 Receipt/file3.pdf", Prefix: "Receipt"},
		},
		ForReview: []FileOperation{},
		Skipped:   []FileOperation{},
		Errors:    []error{},
	}
	duration := 2 * time.Second

	summary := GenerateSummary(result, duration, false)

	if summary.Moved != 3 {
		t.Errorf("Expected Moved=3, got %d", summary.Moved)
	}
}

// TestGenerateSummary_ForReviewCount tests that for-review count matches routed files.
// Requirements: 3.3 - Count of files moved to for-review
func TestGenerateSummary_ForReviewCount(t *testing.T) {
	result := &RunResult{
		Moved: []FileOperation{},
		ForReview: []FileOperation{
			{Source: "/src/unknown1.pdf", Destination: "/src/for-review/unknown1.pdf"},
			{Source: "/src/unknown2.pdf", Destination: "/src/for-review/unknown2.pdf"},
		},
		Skipped: []FileOperation{},
		Errors:  []error{},
	}
	duration := 1 * time.Second

	summary := GenerateSummary(result, duration, false)

	if summary.ForReview != 2 {
		t.Errorf("Expected ForReview=2, got %d", summary.ForReview)
	}
}

// TestGenerateSummary_SkippedCount tests that skipped count matches skipped files.
// Requirements: 3.4 - Count of files skipped
func TestGenerateSummary_SkippedCount(t *testing.T) {
	result := &RunResult{
		Moved:     []FileOperation{},
		ForReview: []FileOperation{},
		Skipped: []FileOperation{
			{Source: "/src/file1.pdf", Reason: "already organized"},
			{Source: "/src/file2.pdf", Reason: "permission denied"},
			{Source: "/src/file3.pdf", Reason: "symlink"},
		},
		Errors: []error{},
	}
	duration := 500 * time.Millisecond

	summary := GenerateSummary(result, duration, false)

	if summary.Skipped != 3 {
		t.Errorf("Expected Skipped=3, got %d", summary.Skipped)
	}
}

// TestGenerateSummary_ErrorCount tests that error count matches failures.
// Requirements: 3.4 - Count of errors
func TestGenerateSummary_ErrorCount(t *testing.T) {
	result := &RunResult{
		Moved:     []FileOperation{},
		ForReview: []FileOperation{},
		Skipped:   []FileOperation{},
		Errors: []error{
			&testError{msg: "error 1"},
			&testError{msg: "error 2"},
		},
	}
	duration := 300 * time.Millisecond

	summary := GenerateSummary(result, duration, false)

	if summary.Errors != 2 {
		t.Errorf("Expected Errors=2, got %d", summary.Errors)
	}
}

// TestGenerateSummary_Duration tests that duration is calculated correctly.
// Requirements: 3.5 - Total processing time
func TestGenerateSummary_Duration(t *testing.T) {
	result := &RunResult{
		Moved:     []FileOperation{},
		ForReview: []FileOperation{},
		Skipped:   []FileOperation{},
		Errors:    []error{},
	}
	expectedDuration := 1234 * time.Millisecond

	summary := GenerateSummary(result, expectedDuration, false)

	if summary.Duration != expectedDuration {
		t.Errorf("Expected Duration=%v, got %v", expectedDuration, summary.Duration)
	}
}

// TestGenerateSummary_VerboseByPrefix tests that verbose mode includes per-prefix breakdown.
// Requirements: 3.6 - Per-prefix breakdown in verbose mode
func TestGenerateSummary_VerboseByPrefix(t *testing.T) {
	result := &RunResult{
		Moved: []FileOperation{
			{Source: "/src/file1.pdf", Destination: "/dest/2024 Invoice/file1.pdf", Prefix: "Invoice"},
			{Source: "/src/file2.pdf", Destination: "/dest/2024 Invoice/file2.pdf", Prefix: "Invoice"},
			{Source: "/src/file3.pdf", Destination: "/dest/2024 Receipt/file3.pdf", Prefix: "Receipt"},
			{Source: "/src/file4.pdf", Destination: "/dest/2024 Statement/file4.pdf", Prefix: "Statement"},
			{Source: "/src/file5.pdf", Destination: "/dest/2024 Invoice/file5.pdf", Prefix: "Invoice"},
		},
		ForReview: []FileOperation{},
		Skipped:   []FileOperation{},
		Errors:    []error{},
	}
	duration := 2 * time.Second

	summary := GenerateSummary(result, duration, true)

	if summary.ByPrefix == nil {
		t.Fatal("Expected ByPrefix to be populated in verbose mode")
	}
	if len(summary.ByPrefix) != 3 {
		t.Errorf("Expected 3 prefixes, got %d", len(summary.ByPrefix))
	}
	if summary.ByPrefix["Invoice"] != 3 {
		t.Errorf("Expected Invoice=3, got %d", summary.ByPrefix["Invoice"])
	}
	if summary.ByPrefix["Receipt"] != 1 {
		t.Errorf("Expected Receipt=1, got %d", summary.ByPrefix["Receipt"])
	}
	if summary.ByPrefix["Statement"] != 1 {
		t.Errorf("Expected Statement=1, got %d", summary.ByPrefix["Statement"])
	}
}

// TestGenerateSummary_NonVerboseNoByPrefix tests that non-verbose mode does not include per-prefix breakdown.
func TestGenerateSummary_NonVerboseNoByPrefix(t *testing.T) {
	result := &RunResult{
		Moved: []FileOperation{
			{Source: "/src/file1.pdf", Destination: "/dest/2024 Invoice/file1.pdf", Prefix: "Invoice"},
			{Source: "/src/file2.pdf", Destination: "/dest/2024 Receipt/file2.pdf", Prefix: "Receipt"},
		},
		ForReview: []FileOperation{},
		Skipped:   []FileOperation{},
		Errors:    []error{},
	}
	duration := 1 * time.Second

	summary := GenerateSummary(result, duration, false)

	if summary.ByPrefix != nil {
		t.Errorf("Expected ByPrefix=nil in non-verbose mode, got %v", summary.ByPrefix)
	}
}

// TestGenerateSummary_MixedOperations tests summary with all operation types.
func TestGenerateSummary_MixedOperations(t *testing.T) {
	result := &RunResult{
		Moved: []FileOperation{
			{Source: "/src/file1.pdf", Destination: "/dest/2024 Invoice/file1.pdf", Prefix: "Invoice"},
			{Source: "/src/file2.pdf", Destination: "/dest/2024 Receipt/file2.pdf", Prefix: "Receipt"},
		},
		ForReview: []FileOperation{
			{Source: "/src/unknown.pdf", Destination: "/src/for-review/unknown.pdf"},
		},
		Skipped: []FileOperation{
			{Source: "/src/skip1.pdf", Reason: "already organized"},
			{Source: "/src/skip2.pdf", Reason: "symlink"},
		},
		Errors: []error{
			&testError{msg: "permission denied"},
		},
	}
	duration := 3 * time.Second

	summary := GenerateSummary(result, duration, false)

	if summary.Moved != 2 {
		t.Errorf("Expected Moved=2, got %d", summary.Moved)
	}
	if summary.ForReview != 1 {
		t.Errorf("Expected ForReview=1, got %d", summary.ForReview)
	}
	if summary.Skipped != 2 {
		t.Errorf("Expected Skipped=2, got %d", summary.Skipped)
	}
	if summary.Errors != 1 {
		t.Errorf("Expected Errors=1, got %d", summary.Errors)
	}
	if summary.Duration != duration {
		t.Errorf("Expected Duration=%v, got %v", duration, summary.Duration)
	}
}

// TestGenerateSummary_VerboseEmptyPrefix tests verbose mode with files that have empty prefix.
func TestGenerateSummary_VerboseEmptyPrefix(t *testing.T) {
	result := &RunResult{
		Moved: []FileOperation{
			{Source: "/src/file1.pdf", Destination: "/dest/2024 Invoice/file1.pdf", Prefix: "Invoice"},
			{Source: "/src/file2.pdf", Destination: "/dest/file2.pdf", Prefix: ""}, // Empty prefix
		},
		ForReview: []FileOperation{},
		Skipped:   []FileOperation{},
		Errors:    []error{},
	}
	duration := 1 * time.Second

	summary := GenerateSummary(result, duration, true)

	if summary.ByPrefix == nil {
		t.Fatal("Expected ByPrefix to be populated in verbose mode")
	}
	// Empty prefix should not be counted
	if len(summary.ByPrefix) != 1 {
		t.Errorf("Expected 1 prefix (empty prefix excluded), got %d", len(summary.ByPrefix))
	}
	if summary.ByPrefix["Invoice"] != 1 {
		t.Errorf("Expected Invoice=1, got %d", summary.ByPrefix["Invoice"])
	}
	if _, exists := summary.ByPrefix[""]; exists {
		t.Error("Empty prefix should not be included in ByPrefix")
	}
}

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
