// Package audit provides audit trail functionality for Sorta file operations.
package audit

import (
	"os"
	"testing"
	"time"
)

// TestAggregateStats_EmptyDirectory tests aggregation with no audit logs.
func TestAggregateStats_EmptyDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stats, err := AggregateStats(tmpDir, StatsOptions{})
	if err != nil {
		t.Fatalf("AggregateStats failed: %v", err)
	}

	if stats.TotalOrganized != 0 {
		t.Errorf("Expected TotalOrganized=0, got %d", stats.TotalOrganized)
	}
	if stats.TotalForReview != 0 {
		t.Errorf("Expected TotalForReview=0, got %d", stats.TotalForReview)
	}
	if stats.TotalRuns != 0 {
		t.Errorf("Expected TotalRuns=0, got %d", stats.TotalRuns)
	}
	if stats.TotalUndos != 0 {
		t.Errorf("Expected TotalUndos=0, got %d", stats.TotalUndos)
	}
}

// TestAggregateStats_TotalOrganized tests that total organized files are counted correctly.
// Validates: Requirement 4.2
func TestAggregateStats_TotalOrganized(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create a run with 3 organized files
	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}
	writer.RecordMove("/source/file1.pdf", "/organized/invoices/file1.pdf", identity)
	writer.RecordMove("/source/file2.pdf", "/organized/receipts/file2.pdf", identity)
	writer.RecordMove("/source/file3.pdf", "/organized/invoices/file3.pdf", identity)

	summary := RunSummary{TotalFiles: 3, Moved: 3}
	if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}

	stats, err := AggregateStats(tmpDir, StatsOptions{})
	if err != nil {
		t.Fatalf("AggregateStats failed: %v", err)
	}

	if stats.TotalOrganized != 3 {
		t.Errorf("Expected TotalOrganized=3, got %d", stats.TotalOrganized)
	}
}

// TestAggregateStats_TotalForReview tests that for-review files are counted correctly.
// Validates: Requirement 4.4
func TestAggregateStats_TotalForReview(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create a run with 2 for-review files
	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}
	writer.RecordMove("/source/file1.pdf", "/organized/invoices/file1.pdf", identity)
	writer.RecordRouteToReview("/source/unknown1.pdf", "/for-review/unknown1.pdf", ReasonUnclassified)
	writer.RecordRouteToReview("/source/unknown2.pdf", "/for-review/unknown2.pdf", ReasonParseError)

	summary := RunSummary{TotalFiles: 3, Moved: 1, RoutedReview: 2}
	if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}

	stats, err := AggregateStats(tmpDir, StatsOptions{})
	if err != nil {
		t.Fatalf("AggregateStats failed: %v", err)
	}

	if stats.TotalForReview != 2 {
		t.Errorf("Expected TotalForReview=2, got %d", stats.TotalForReview)
	}
}

// TestAggregateStats_TotalRunsAndDateRange tests run count and date range calculation.
// Validates: Requirement 4.5
func TestAggregateStats_TotalRunsAndDateRange(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create first run
	runID1, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 1: %v", err)
	}
	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}
	writer.RecordMove("/source/file1.pdf", "/organized/invoices/file1.pdf", identity)
	summary1 := RunSummary{TotalFiles: 1, Moved: 1}
	if err := writer.EndRun(runID1, RunStatusCompleted, summary1); err != nil {
		t.Fatalf("Failed to end run 1: %v", err)
	}

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Create second run
	runID2, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 2: %v", err)
	}
	writer.RecordMove("/source/file2.pdf", "/organized/receipts/file2.pdf", identity)
	summary2 := RunSummary{TotalFiles: 1, Moved: 1}
	if err := writer.EndRun(runID2, RunStatusCompleted, summary2); err != nil {
		t.Fatalf("Failed to end run 2: %v", err)
	}

	stats, err := AggregateStats(tmpDir, StatsOptions{})
	if err != nil {
		t.Fatalf("AggregateStats failed: %v", err)
	}

	if stats.TotalRuns != 2 {
		t.Errorf("Expected TotalRuns=2, got %d", stats.TotalRuns)
	}

	if stats.FirstRun.IsZero() {
		t.Error("Expected FirstRun to be set")
	}
	if stats.LastRun.IsZero() {
		t.Error("Expected LastRun to be set")
	}
	if !stats.FirstRun.Before(stats.LastRun) && !stats.FirstRun.Equal(stats.LastRun) {
		t.Errorf("Expected FirstRun <= LastRun, got FirstRun=%v, LastRun=%v", stats.FirstRun, stats.LastRun)
	}
}

// TestAggregateStats_TotalUndos tests that undo operations are counted correctly.
// Validates: Requirement 4.6
func TestAggregateStats_TotalUndos(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create an organize run first
	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start organize run: %v", err)
	}
	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}
	writer.RecordMove("/source/file1.pdf", "/organized/invoices/file1.pdf", identity)
	summary := RunSummary{TotalFiles: 1, Moved: 1}
	if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
		t.Fatalf("Failed to end organize run: %v", err)
	}

	// Create an undo run
	_, err = writer.StartUndoRun("1.0.0", "test-machine", runID)
	if err != nil {
		t.Fatalf("Failed to start undo run: %v", err)
	}
	// End the undo run - need to get the current run ID
	reader := NewAuditReader(tmpDir)
	runs, _ := reader.ListRuns()
	var undoRunID RunID
	for _, r := range runs {
		if r.RunType == RunTypeUndo {
			undoRunID = r.RunID
			break
		}
	}
	undoSummary := RunSummary{}
	if err := writer.EndRun(undoRunID, RunStatusCompleted, undoSummary); err != nil {
		t.Fatalf("Failed to end undo run: %v", err)
	}

	stats, err := AggregateStats(tmpDir, StatsOptions{})
	if err != nil {
		t.Fatalf("AggregateStats failed: %v", err)
	}

	if stats.TotalRuns != 1 {
		t.Errorf("Expected TotalRuns=1 (organize runs only), got %d", stats.TotalRuns)
	}
	if stats.TotalUndos != 1 {
		t.Errorf("Expected TotalUndos=1, got %d", stats.TotalUndos)
	}
}

// TestAggregateStats_ByPrefix tests per-prefix file counts.
// Validates: Requirement 4.3
func TestAggregateStats_ByPrefix(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}
	// 3 invoices, 2 receipts, 1 statement
	writer.RecordMove("/source/file1.pdf", "/organized/invoices/2024/file1.pdf", identity)
	writer.RecordMove("/source/file2.pdf", "/organized/invoices/2024/file2.pdf", identity)
	writer.RecordMove("/source/file3.pdf", "/organized/invoices/2023/file3.pdf", identity)
	writer.RecordMove("/source/file4.pdf", "/organized/receipts/file4.pdf", identity)
	writer.RecordMove("/source/file5.pdf", "/organized/receipts/file5.pdf", identity)
	writer.RecordMove("/source/file6.pdf", "/organized/statements/file6.pdf", identity)

	summary := RunSummary{TotalFiles: 6, Moved: 6}
	if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}

	stats, err := AggregateStats(tmpDir, StatsOptions{})
	if err != nil {
		t.Fatalf("AggregateStats failed: %v", err)
	}

	if stats.ByPrefix["invoices"] != 3 {
		t.Errorf("Expected invoices=3, got %d", stats.ByPrefix["invoices"])
	}
	if stats.ByPrefix["receipts"] != 2 {
		t.Errorf("Expected receipts=2, got %d", stats.ByPrefix["receipts"])
	}
	if stats.ByPrefix["statements"] != 1 {
		t.Errorf("Expected statements=1, got %d", stats.ByPrefix["statements"])
	}
}

// TestAggregateStats_TopN tests that top N prefix filtering works.
// Validates: Requirement 4.3
func TestAggregateStats_TopN(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}

	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}
	// Create files with different prefix counts: invoices(5), receipts(3), statements(2), misc(1)
	for i := 0; i < 5; i++ {
		writer.RecordMove("/source/inv.pdf", "/organized/invoices/inv.pdf", identity)
	}
	for i := 0; i < 3; i++ {
		writer.RecordMove("/source/rec.pdf", "/organized/receipts/rec.pdf", identity)
	}
	for i := 0; i < 2; i++ {
		writer.RecordMove("/source/stmt.pdf", "/organized/statements/stmt.pdf", identity)
	}
	writer.RecordMove("/source/misc.pdf", "/organized/misc/misc.pdf", identity)

	summary := RunSummary{TotalFiles: 11, Moved: 11}
	if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}

	// Request top 2 prefixes
	stats, err := AggregateStats(tmpDir, StatsOptions{TopN: 2})
	if err != nil {
		t.Fatalf("AggregateStats failed: %v", err)
	}

	if len(stats.ByPrefix) != 2 {
		t.Errorf("Expected 2 prefixes, got %d", len(stats.ByPrefix))
	}

	// Should have invoices (5) and receipts (3)
	if _, ok := stats.ByPrefix["invoices"]; !ok {
		t.Error("Expected invoices in top 2")
	}
	if _, ok := stats.ByPrefix["receipts"]; !ok {
		t.Error("Expected receipts in top 2")
	}
}

// TestAggregateStats_MultipleRuns tests aggregation across multiple runs.
func TestAggregateStats_MultipleRuns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}

	// Run 1: 2 organized, 1 for-review
	runID1, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 1: %v", err)
	}
	writer.RecordMove("/source/file1.pdf", "/organized/invoices/file1.pdf", identity)
	writer.RecordMove("/source/file2.pdf", "/organized/invoices/file2.pdf", identity)
	writer.RecordRouteToReview("/source/unknown1.pdf", "/for-review/unknown1.pdf", ReasonUnclassified)
	summary1 := RunSummary{TotalFiles: 3, Moved: 2, RoutedReview: 1}
	if err := writer.EndRun(runID1, RunStatusCompleted, summary1); err != nil {
		t.Fatalf("Failed to end run 1: %v", err)
	}

	// Run 2: 3 organized, 2 for-review
	runID2, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 2: %v", err)
	}
	writer.RecordMove("/source/file3.pdf", "/organized/receipts/file3.pdf", identity)
	writer.RecordMove("/source/file4.pdf", "/organized/receipts/file4.pdf", identity)
	writer.RecordMove("/source/file5.pdf", "/organized/receipts/file5.pdf", identity)
	writer.RecordRouteToReview("/source/unknown2.pdf", "/for-review/unknown2.pdf", ReasonParseError)
	writer.RecordRouteToReview("/source/unknown3.pdf", "/for-review/unknown3.pdf", ReasonUnclassified)
	summary2 := RunSummary{TotalFiles: 5, Moved: 3, RoutedReview: 2}
	if err := writer.EndRun(runID2, RunStatusCompleted, summary2); err != nil {
		t.Fatalf("Failed to end run 2: %v", err)
	}

	stats, err := AggregateStats(tmpDir, StatsOptions{})
	if err != nil {
		t.Fatalf("AggregateStats failed: %v", err)
	}

	// Total: 5 organized, 3 for-review, 2 runs
	if stats.TotalOrganized != 5 {
		t.Errorf("Expected TotalOrganized=5, got %d", stats.TotalOrganized)
	}
	if stats.TotalForReview != 3 {
		t.Errorf("Expected TotalForReview=3, got %d", stats.TotalForReview)
	}
	if stats.TotalRuns != 2 {
		t.Errorf("Expected TotalRuns=2, got %d", stats.TotalRuns)
	}

	// Prefix counts: invoices=2, receipts=3
	if stats.ByPrefix["invoices"] != 2 {
		t.Errorf("Expected invoices=2, got %d", stats.ByPrefix["invoices"])
	}
	if stats.ByPrefix["receipts"] != 3 {
		t.Errorf("Expected receipts=3, got %d", stats.ByPrefix["receipts"])
	}
}

// TestExtractPrefix tests the prefix extraction helper function.
func TestExtractPrefix(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/organized/invoices/2024/file.pdf", "invoices"},
		{"/organized/receipts/file.pdf", "receipts"},
		{"organized/statements/file.pdf", "statements"},
		{"/output/taxes/2024/file.pdf", "taxes"},
		{"invoices/file.pdf", "invoices"},
		{"/file.pdf", "file.pdf"},
		{"", ""},
		{"./invoices/file.pdf", "invoices"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractPrefix(tt.path)
			if result != tt.expected {
				t.Errorf("extractPrefix(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

// TestFilterTopN tests the top N filtering helper function.
func TestFilterTopN(t *testing.T) {
	counts := map[string]int{
		"a": 10,
		"b": 5,
		"c": 8,
		"d": 3,
		"e": 8, // Same as c, should be sorted alphabetically
	}

	// Test top 3
	result := filterTopN(counts, 3)
	if len(result) != 3 {
		t.Errorf("Expected 3 results, got %d", len(result))
	}
	if result["a"] != 10 {
		t.Errorf("Expected a=10, got %d", result["a"])
	}
	// c and e both have 8, c should come first alphabetically
	if _, ok := result["c"]; !ok {
		t.Error("Expected c in top 3")
	}

	// Test n=0 returns all
	result = filterTopN(counts, 0)
	if len(result) != 5 {
		t.Errorf("Expected 5 results for n=0, got %d", len(result))
	}

	// Test n > len returns all
	result = filterTopN(counts, 10)
	if len(result) != 5 {
		t.Errorf("Expected 5 results for n=10, got %d", len(result))
	}
}

// TestAggregateStats_SinceFilter tests that --since flag filters runs correctly.
// Validates: Requirement 4.7
func TestAggregateStats_SinceFilter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}

	// Create first run (older)
	runID1, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 1: %v", err)
	}
	writer.RecordMove("/source/file1.pdf", "/organized/invoices/file1.pdf", identity)
	writer.RecordMove("/source/file2.pdf", "/organized/invoices/file2.pdf", identity)
	summary1 := RunSummary{TotalFiles: 2, Moved: 2}
	if err := writer.EndRun(runID1, RunStatusCompleted, summary1); err != nil {
		t.Fatalf("Failed to end run 1: %v", err)
	}

	// Get the first run's timestamp to use as filter
	reader := NewAuditReader(tmpDir)
	runs, err := reader.ListRuns()
	if err != nil {
		t.Fatalf("Failed to list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("Expected 1 run after first run, got %d", len(runs))
	}

	// Wait for at least 1 second to ensure different timestamps
	// (RFC3339 format only has second precision)
	time.Sleep(1100 * time.Millisecond)

	// Use a filter time just after the first run (add 500ms to be safe)
	filterTime := runs[0].StartTime.Add(500 * time.Millisecond)

	// Create second run (newer - after filter time)
	runID2, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run 2: %v", err)
	}
	writer.RecordMove("/source/file3.pdf", "/organized/receipts/file3.pdf", identity)
	writer.RecordMove("/source/file4.pdf", "/organized/receipts/file4.pdf", identity)
	writer.RecordMove("/source/file5.pdf", "/organized/receipts/file5.pdf", identity)
	summary2 := RunSummary{TotalFiles: 3, Moved: 3}
	if err := writer.EndRun(runID2, RunStatusCompleted, summary2); err != nil {
		t.Fatalf("Failed to end run 2: %v", err)
	}

	// Test without filter - should include all runs
	statsAll, err := AggregateStats(tmpDir, StatsOptions{})
	if err != nil {
		t.Fatalf("AggregateStats (no filter) failed: %v", err)
	}

	if statsAll.TotalRuns != 2 {
		t.Errorf("Without filter: Expected TotalRuns=2, got %d", statsAll.TotalRuns)
	}
	if statsAll.TotalOrganized != 5 {
		t.Errorf("Without filter: Expected TotalOrganized=5, got %d", statsAll.TotalOrganized)
	}

	// Test with --since filter - should only include the second run
	statsFiltered, err := AggregateStats(tmpDir, StatsOptions{Since: &filterTime})
	if err != nil {
		t.Fatalf("AggregateStats (with filter) failed: %v", err)
	}

	if statsFiltered.TotalRuns != 1 {
		t.Errorf("With filter: Expected TotalRuns=1, got %d", statsFiltered.TotalRuns)
	}
	if statsFiltered.TotalOrganized != 3 {
		t.Errorf("With filter: Expected TotalOrganized=3, got %d", statsFiltered.TotalOrganized)
	}

	// Verify prefix counts are filtered too
	if statsFiltered.ByPrefix["invoices"] != 0 {
		t.Errorf("With filter: Expected invoices=0 (filtered out), got %d", statsFiltered.ByPrefix["invoices"])
	}
	if statsFiltered.ByPrefix["receipts"] != 3 {
		t.Errorf("With filter: Expected receipts=3, got %d", statsFiltered.ByPrefix["receipts"])
	}

	// Verify date range is correct for filtered results
	if statsFiltered.FirstRun.Before(filterTime) {
		t.Errorf("With filter: FirstRun should be after filter time, got %v (filter: %v)", statsFiltered.FirstRun, filterTime)
	}
}

// TestAggregateStats_SinceFilterExcludesAllRuns tests that --since filter can exclude all runs.
// Validates: Requirement 4.7
func TestAggregateStats_SinceFilterExcludesAllRuns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}

	// Create a run
	runID, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start run: %v", err)
	}
	writer.RecordMove("/source/file1.pdf", "/organized/invoices/file1.pdf", identity)
	summary := RunSummary{TotalFiles: 1, Moved: 1}
	if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
		t.Fatalf("Failed to end run: %v", err)
	}

	// Filter with a future time - should exclude all runs
	futureTime := time.Now().Add(1 * time.Hour)
	stats, err := AggregateStats(tmpDir, StatsOptions{Since: &futureTime})
	if err != nil {
		t.Fatalf("AggregateStats failed: %v", err)
	}

	if stats.TotalRuns != 0 {
		t.Errorf("Expected TotalRuns=0 (all filtered), got %d", stats.TotalRuns)
	}
	if stats.TotalOrganized != 0 {
		t.Errorf("Expected TotalOrganized=0 (all filtered), got %d", stats.TotalOrganized)
	}
	if stats.TotalForReview != 0 {
		t.Errorf("Expected TotalForReview=0 (all filtered), got %d", stats.TotalForReview)
	}
	if len(stats.ByPrefix) != 0 {
		t.Errorf("Expected empty ByPrefix (all filtered), got %v", stats.ByPrefix)
	}
}

// TestAggregateStats_SinceFilterWithUndos tests that --since filter works with undo operations.
// Validates: Requirement 4.7
func TestAggregateStats_SinceFilterWithUndos(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}

	// Create an organize run
	runID1, err := writer.StartRun("1.0.0", "test-machine")
	if err != nil {
		t.Fatalf("Failed to start organize run: %v", err)
	}
	writer.RecordMove("/source/file1.pdf", "/organized/invoices/file1.pdf", identity)
	summary1 := RunSummary{TotalFiles: 1, Moved: 1}
	if err := writer.EndRun(runID1, RunStatusCompleted, summary1); err != nil {
		t.Fatalf("Failed to end organize run: %v", err)
	}

	// Get the organize run's timestamp to use as filter
	reader := NewAuditReader(tmpDir)
	runs, err := reader.ListRuns()
	if err != nil {
		t.Fatalf("Failed to list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("Expected 1 run after organize run, got %d", len(runs))
	}

	// Wait for at least 1 second to ensure different timestamps
	// (RFC3339 format only has second precision)
	time.Sleep(1100 * time.Millisecond)

	// Use a filter time just after the organize run
	filterTime := runs[0].StartTime.Add(500 * time.Millisecond)

	// Create an undo run (after filter time)
	undoRunID, err := writer.StartUndoRun("1.0.0", "test-machine", runID1)
	if err != nil {
		t.Fatalf("Failed to start undo run: %v", err)
	}
	undoSummary := RunSummary{}
	if err := writer.EndRun(undoRunID, RunStatusCompleted, undoSummary); err != nil {
		t.Fatalf("Failed to end undo run: %v", err)
	}

	// Test with --since filter - should only include the undo run
	stats, err := AggregateStats(tmpDir, StatsOptions{Since: &filterTime})
	if err != nil {
		t.Fatalf("AggregateStats failed: %v", err)
	}

	// The organize run should be filtered out, only undo remains
	if stats.TotalRuns != 0 {
		t.Errorf("Expected TotalRuns=0 (organize run filtered), got %d", stats.TotalRuns)
	}
	if stats.TotalUndos != 1 {
		t.Errorf("Expected TotalUndos=1, got %d", stats.TotalUndos)
	}
	if stats.TotalOrganized != 0 {
		t.Errorf("Expected TotalOrganized=0 (organize run filtered), got %d", stats.TotalOrganized)
	}
}
