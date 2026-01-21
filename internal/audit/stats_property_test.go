// Package audit provides audit trail functionality for Sorta file operations.
package audit

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: watch-mode, Property: Stats Totals Equal Sum of Parts
// **Validates: Requirements 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7**
//
// For any set of audit logs with random events:
// - Total organized files equals the sum of all per-prefix counts
// - Total for-review equals the sum of for-review counts across all runs
// - When --since filter is applied, only runs after that time are included
//
// This is a mathematical invariant that must hold for ANY audit log configuration.

// TestRunData represents generated data for a single run.
type TestRunData struct {
	IsUndo       bool
	MoveEvents   []TestMoveEvent
	ReviewEvents int
}

// TestMoveEvent represents a generated move event with prefix.
type TestMoveEvent struct {
	Prefix string
}

// genPrefix generates a valid prefix string.
func genPrefix() gopter.Gen {
	return gen.OneConstOf("invoices", "receipts", "statements", "taxes", "contracts", "reports")
}

// genMoveEvent generates a test move event.
func genMoveEvent() gopter.Gen {
	return genPrefix().Map(func(prefix string) TestMoveEvent {
		return TestMoveEvent{Prefix: prefix}
	})
}

// genTestRunData generates data for a single run.
func genTestRunData() gopter.Gen {
	return gopter.CombineGens(
		gen.Bool(),                       // IsUndo
		gen.SliceOfN(10, genMoveEvent()), // MoveEvents (0-10 moves per run)
		gen.IntRange(0, 5),               // ReviewEvents (0-5 reviews per run)
	).Map(func(vals []interface{}) TestRunData {
		return TestRunData{
			IsUndo:       vals[0].(bool),
			MoveEvents:   vals[1].([]TestMoveEvent),
			ReviewEvents: vals[2].(int),
		}
	})
}

// genTestRuns generates a slice of test runs.
func genTestRuns() gopter.Gen {
	return gen.SliceOfN(5, genTestRunData()) // 1-5 runs
}

// createTestAuditLogs creates audit logs from generated test data and returns expected stats.
// Note: The implementation counts ALL runs (including undo runs) for TotalOrganized and ByPrefix.
// TotalOrganized comes from run.Summary.Moved, and ByPrefix comes from MOVE events.
func createTestAuditLogs(tmpDir string, runs []TestRunData) (expectedOrganized int, expectedForReview int, expectedRuns int, expectedUndos int, expectedByPrefix map[string]int, err error) {
	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		return 0, 0, 0, 0, nil, err
	}
	defer writer.Close()

	expectedByPrefix = make(map[string]int)
	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}

	for i, run := range runs {
		var runID RunID
		if run.IsUndo {
			// For undo runs, we need a target run ID (use a fake one)
			runID, err = writer.StartUndoRun("1.0.0", "test-machine", RunID("fake-target-run"))
			if err != nil {
				return 0, 0, 0, 0, nil, err
			}
			expectedUndos++
		} else {
			runID, err = writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				return 0, 0, 0, 0, nil, err
			}
			expectedRuns++
		}

		// Record move events - ALL runs (including undo) contribute to ByPrefix
		for j, move := range run.MoveEvents {
			sourcePath := "/source/file" + itoa(i) + "_" + itoa(j) + ".pdf"
			destPath := "/organized/" + move.Prefix + "/file" + itoa(i) + "_" + itoa(j) + ".pdf"
			if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
				return 0, 0, 0, 0, nil, err
			}
			// ByPrefix counts ALL MOVE events regardless of run type
			expectedByPrefix[move.Prefix]++
		}

		// Record review events
		for j := 0; j < run.ReviewEvents; j++ {
			sourcePath := "/source/unknown" + itoa(i) + "_" + itoa(j) + ".pdf"
			destPath := "/for-review/unknown" + itoa(i) + "_" + itoa(j) + ".pdf"
			if err := writer.RecordRouteToReview(sourcePath, destPath, ReasonUnclassified); err != nil {
				return 0, 0, 0, 0, nil, err
			}
		}

		// TotalOrganized and TotalForReview come from run.Summary, which includes ALL runs
		expectedOrganized += len(run.MoveEvents)
		expectedForReview += run.ReviewEvents

		// End the run
		summary := RunSummary{
			TotalFiles:   len(run.MoveEvents) + run.ReviewEvents,
			Moved:        len(run.MoveEvents),
			RoutedReview: run.ReviewEvents,
		}
		if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
			return 0, 0, 0, 0, nil, err
		}
	}

	return expectedOrganized, expectedForReview, expectedRuns, expectedUndos, expectedByPrefix, nil
}

// itoa converts an integer to a string (simple helper).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if negative {
		result = "-" + result
	}
	return result
}

// TestStatsTotalsEqualSumOfParts tests the mathematical invariant that
// aggregated totals must equal the sum of their components.
// **Validates: Requirements 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7**
func TestStatsTotalsEqualSumOfParts(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)

	properties.Property("Total organized equals sum of per-prefix counts", prop.ForAll(
		func(runs []TestRunData) bool {
			// Skip empty runs
			if len(runs) == 0 {
				return true
			}

			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "stats-property-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tmpDir)

			// Create audit logs and get expected values
			expectedOrganized, expectedForReview, expectedRuns, expectedUndos, expectedByPrefix, err := createTestAuditLogs(tmpDir, runs)
			if err != nil {
				t.Logf("Failed to create audit logs: %v", err)
				return false
			}

			// Aggregate stats
			stats, err := AggregateStats(tmpDir, StatsOptions{})
			if err != nil {
				t.Logf("AggregateStats failed: %v", err)
				return false
			}

			// PROPERTY 1: TotalOrganized equals sum of per-prefix counts
			// Requirements 4.2, 4.3
			sumOfPrefixes := 0
			for _, count := range stats.ByPrefix {
				sumOfPrefixes += count
			}
			if stats.TotalOrganized != sumOfPrefixes {
				t.Logf("TotalOrganized (%d) != sum of per-prefix counts (%d)", stats.TotalOrganized, sumOfPrefixes)
				return false
			}

			// PROPERTY 2: TotalOrganized matches expected from generated data
			// Requirements 4.2
			if stats.TotalOrganized != expectedOrganized {
				t.Logf("TotalOrganized (%d) != expected (%d)", stats.TotalOrganized, expectedOrganized)
				return false
			}

			// PROPERTY 3: TotalForReview matches expected from generated data
			// Requirements 4.4
			if stats.TotalForReview != expectedForReview {
				t.Logf("TotalForReview (%d) != expected (%d)", stats.TotalForReview, expectedForReview)
				return false
			}

			// PROPERTY 4: TotalRuns matches expected (non-undo runs only)
			// Requirements 4.5
			if stats.TotalRuns != expectedRuns {
				t.Logf("TotalRuns (%d) != expected (%d)", stats.TotalRuns, expectedRuns)
				return false
			}

			// PROPERTY 5: TotalUndos matches expected
			// Requirements 4.6
			if stats.TotalUndos != expectedUndos {
				t.Logf("TotalUndos (%d) != expected (%d)", stats.TotalUndos, expectedUndos)
				return false
			}

			// PROPERTY 6: Per-prefix counts match expected
			// Requirements 4.3
			for prefix, expectedCount := range expectedByPrefix {
				if stats.ByPrefix[prefix] != expectedCount {
					t.Logf("ByPrefix[%s] (%d) != expected (%d)", prefix, stats.ByPrefix[prefix], expectedCount)
					return false
				}
			}

			return true
		},
		genTestRuns(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestStatsSinceFilterExcludesOlderRuns tests that --since filtering correctly
// excludes runs that occurred before the specified time.
// **Validates: Requirements 4.7**
//
// Note: This test is converted to a unit test with specific examples rather than
// a property test because the --since filter requires real time delays to create
// distinct timestamps (RFC3339 has second precision). Running 100 iterations with
// 2+ seconds of sleep each would take over 3 minutes.
func TestStatsSinceFilterExcludesOlderRuns(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "stats-since-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := AuditConfig{LogDirectory: tmpDir}
	writer, err := NewAuditWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	identity := &FileIdentity{ContentHash: "abc123", Size: 1000}

	// Create 2 "old" runs with invoices prefix
	oldOrganized := 0
	for i := 0; i < 2; i++ {
		runID, err := writer.StartRun("1.0.0", "test-machine")
		if err != nil {
			writer.Close()
			t.Fatalf("Failed to start old run: %v", err)
		}

		for j := 0; j < 3; j++ {
			sourcePath := "/source/old" + itoa(i) + "_" + itoa(j) + ".pdf"
			destPath := "/organized/invoices/old" + itoa(i) + "_" + itoa(j) + ".pdf"
			if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
				writer.Close()
				t.Fatalf("Failed to record move: %v", err)
			}
			oldOrganized++
		}

		summary := RunSummary{TotalFiles: 3, Moved: 3}
		if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
			writer.Close()
			t.Fatalf("Failed to end old run: %v", err)
		}
	}

	// Wait to ensure timestamp difference (RFC3339 has second precision)
	time.Sleep(1200 * time.Millisecond)

	// Record the filter time
	filterTime := time.Now().UTC().Truncate(time.Second)

	// Wait to ensure new runs are after filter time
	time.Sleep(1100 * time.Millisecond)

	// Create 2 "new" runs with receipts prefix
	newOrganized := 0
	for i := 0; i < 2; i++ {
		runID, err := writer.StartRun("1.0.0", "test-machine")
		if err != nil {
			writer.Close()
			t.Fatalf("Failed to start new run: %v", err)
		}

		for j := 0; j < 3; j++ {
			sourcePath := "/source/new" + itoa(i) + "_" + itoa(j) + ".pdf"
			destPath := "/organized/receipts/new" + itoa(i) + "_" + itoa(j) + ".pdf"
			if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
				writer.Close()
				t.Fatalf("Failed to record move: %v", err)
			}
			newOrganized++
		}

		summary := RunSummary{TotalFiles: 3, Moved: 3}
		if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
			writer.Close()
			t.Fatalf("Failed to end new run: %v", err)
		}
	}

	writer.Close()

	// Test without filter - should include all runs
	statsAll, err := AggregateStats(tmpDir, StatsOptions{})
	if err != nil {
		t.Fatalf("AggregateStats (no filter) failed: %v", err)
	}

	expectedTotal := oldOrganized + newOrganized
	if statsAll.TotalOrganized != expectedTotal {
		t.Errorf("Without filter: TotalOrganized (%d) != expected (%d)", statsAll.TotalOrganized, expectedTotal)
	}
	if statsAll.TotalRuns != 4 {
		t.Errorf("Without filter: TotalRuns (%d) != expected (4)", statsAll.TotalRuns)
	}

	// Test with --since filter
	statsFiltered, err := AggregateStats(tmpDir, StatsOptions{Since: &filterTime})
	if err != nil {
		t.Fatalf("AggregateStats (with filter) failed: %v", err)
	}

	// With filter, should only include new runs
	if statsFiltered.TotalOrganized != newOrganized {
		t.Errorf("With filter: TotalOrganized (%d) != expected new (%d)",
			statsFiltered.TotalOrganized, newOrganized)
	}

	if statsFiltered.TotalRuns != 2 {
		t.Errorf("With filter: TotalRuns (%d) != expected (2)", statsFiltered.TotalRuns)
	}

	// Old prefix (invoices) should not appear in filtered results
	if statsFiltered.ByPrefix["invoices"] != 0 {
		t.Errorf("With filter: invoices prefix should be 0, got %d", statsFiltered.ByPrefix["invoices"])
	}

	// New prefix (receipts) should appear in filtered results
	if statsFiltered.ByPrefix["receipts"] != newOrganized {
		t.Errorf("With filter: receipts prefix (%d) != expected (%d)", statsFiltered.ByPrefix["receipts"], newOrganized)
	}
}

// TestStatsTopNFiltering tests that TopN filtering correctly limits prefix counts
// while preserving the mathematical invariant.
// **Validates: Requirements 4.3**
func TestStatsTopNFiltering(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)

	properties.Property("TopN filtering preserves top prefixes by count", prop.ForAll(
		func(prefixCounts []int, topN int) bool {
			// Need at least one prefix
			if len(prefixCounts) == 0 {
				return true
			}

			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "stats-topn-property-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tmpDir)

			config := AuditConfig{LogDirectory: tmpDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				t.Logf("Failed to create writer: %v", err)
				return false
			}

			prefixes := []string{"invoices", "receipts", "statements", "taxes", "contracts"}
			identity := &FileIdentity{ContentHash: "abc123", Size: 1000}

			runID, err := writer.StartRun("1.0.0", "test-machine")
			if err != nil {
				writer.Close()
				t.Logf("Failed to start run: %v", err)
				return false
			}

			// Create files for each prefix based on generated counts
			expectedByPrefix := make(map[string]int)
			totalMoved := 0
			for i, count := range prefixCounts {
				if i >= len(prefixes) {
					break
				}
				prefix := prefixes[i]
				for j := 0; j < count; j++ {
					sourcePath := "/source/" + prefix + itoa(j) + ".pdf"
					destPath := "/organized/" + prefix + "/" + prefix + itoa(j) + ".pdf"
					if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
						writer.Close()
						t.Logf("Failed to record move: %v", err)
						return false
					}
					expectedByPrefix[prefix]++
					totalMoved++
				}
			}

			summary := RunSummary{TotalFiles: totalMoved, Moved: totalMoved}
			if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
				writer.Close()
				t.Logf("Failed to end run: %v", err)
				return false
			}
			writer.Close()

			// Aggregate with TopN filter
			stats, err := AggregateStats(tmpDir, StatsOptions{TopN: topN})
			if err != nil {
				t.Logf("AggregateStats failed: %v", err)
				return false
			}

			// PROPERTY: Number of prefixes should be min(topN, actual prefixes) when topN > 0
			numActualPrefixes := 0
			for _, count := range expectedByPrefix {
				if count > 0 {
					numActualPrefixes++
				}
			}

			if topN > 0 {
				expectedPrefixCount := topN
				if numActualPrefixes < topN {
					expectedPrefixCount = numActualPrefixes
				}
				if len(stats.ByPrefix) != expectedPrefixCount {
					t.Logf("TopN=%d: got %d prefixes, expected %d", topN, len(stats.ByPrefix), expectedPrefixCount)
					return false
				}
			}

			// PROPERTY: TotalOrganized should still equal total moved (TopN doesn't affect totals)
			if stats.TotalOrganized != totalMoved {
				t.Logf("TotalOrganized (%d) != totalMoved (%d)", stats.TotalOrganized, totalMoved)
				return false
			}

			// PROPERTY: All returned prefixes should be from the top N by count
			if topN > 0 && len(stats.ByPrefix) > 0 {
				// Find the minimum count in the returned prefixes
				minReturnedCount := -1
				for _, count := range stats.ByPrefix {
					if minReturnedCount == -1 || count < minReturnedCount {
						minReturnedCount = count
					}
				}

				// All excluded prefixes should have counts <= minReturnedCount
				for prefix, count := range expectedByPrefix {
					if _, inResult := stats.ByPrefix[prefix]; !inResult && count > 0 {
						if count > minReturnedCount {
							t.Logf("Excluded prefix %s has count %d > min returned count %d", prefix, count, minReturnedCount)
							return false
						}
					}
				}
			}

			return true
		},
		gen.SliceOfN(5, gen.IntRange(0, 10)), // prefixCounts for up to 5 prefixes
		gen.IntRange(0, 6),                   // topN (0 means all)
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestStatsDateRangeCorrectness tests that FirstRun and LastRun are correctly calculated.
// **Validates: Requirements 4.5**
//
// Note: Converted to a unit test because the property test with time.Sleep(10ms)
// between runs × 100 iterations adds unnecessary test time. The date range logic
// is straightforward and well-covered by specific examples.
func TestStatsDateRangeCorrectness(t *testing.T) {
	testCases := []struct {
		name    string
		numRuns int
	}{
		{"single run", 1},
		{"two runs", 2},
		{"five runs", 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "stats-daterange-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			config := AuditConfig{LogDirectory: tmpDir}
			writer, err := NewAuditWriter(config)
			if err != nil {
				t.Fatalf("Failed to create writer: %v", err)
			}

			identity := &FileIdentity{ContentHash: "abc123", Size: 1000}
			firstRunTime := time.Now()

			for i := 0; i < tc.numRuns; i++ {
				runID, err := writer.StartRun("1.0.0", "test-machine")
				if err != nil {
					writer.Close()
					t.Fatalf("Failed to start run: %v", err)
				}

				sourcePath := "/source/file" + itoa(i) + ".pdf"
				destPath := "/organized/invoices/file" + itoa(i) + ".pdf"
				if err := writer.RecordMove(sourcePath, destPath, identity); err != nil {
					writer.Close()
					t.Fatalf("Failed to record move: %v", err)
				}

				summary := RunSummary{TotalFiles: 1, Moved: 1}
				if err := writer.EndRun(runID, RunStatusCompleted, summary); err != nil {
					writer.Close()
					t.Fatalf("Failed to end run: %v", err)
				}

				// Small delay between runs to ensure different timestamps
				if i < tc.numRuns-1 {
					time.Sleep(10 * time.Millisecond)
				}
			}

			writer.Close()

			stats, err := AggregateStats(tmpDir, StatsOptions{})
			if err != nil {
				t.Fatalf("AggregateStats failed: %v", err)
			}

			// FirstRun should not be zero
			if stats.FirstRun.IsZero() {
				t.Error("FirstRun is zero")
			}

			// LastRun should not be zero
			if stats.LastRun.IsZero() {
				t.Error("LastRun is zero")
			}

			// FirstRun <= LastRun
			if stats.FirstRun.After(stats.LastRun) {
				t.Errorf("FirstRun (%v) is after LastRun (%v)", stats.FirstRun, stats.LastRun)
			}

			// For single run, FirstRun should equal LastRun
			if tc.numRuns == 1 && !stats.FirstRun.Equal(stats.LastRun) {
				t.Errorf("Single run: FirstRun (%v) != LastRun (%v)", stats.FirstRun, stats.LastRun)
			}

			// FirstRun should be within reasonable range of our recorded time
			timeDiff := stats.FirstRun.Sub(firstRunTime)
			if timeDiff < -time.Second || timeDiff > time.Second {
				t.Errorf("FirstRun time difference too large: %v", timeDiff)
			}
		})
	}
}

// genTestRunDataNonUndo generates data for a non-undo run (for simpler tests).
func genTestRunDataNonUndo() gopter.Gen {
	return gopter.CombineGens(
		gen.SliceOfN(10, genMoveEvent()), // MoveEvents (0-10 moves per run)
		gen.IntRange(0, 5),               // ReviewEvents (0-5 reviews per run)
	).Map(func(vals []interface{}) TestRunData {
		return TestRunData{
			IsUndo:       false,
			MoveEvents:   vals[0].([]TestMoveEvent),
			ReviewEvents: vals[1].(int),
		}
	})
}

// genTestRunsNonUndo generates a slice of non-undo test runs.
func genTestRunsNonUndo() gopter.Gen {
	return gen.SliceOfN(5, genTestRunDataNonUndo())
}

// TestStatsSumInvariant tests the core mathematical invariant:
// sum(ByPrefix values) == TotalOrganized (when TopN=0)
// **Validates: Requirements 4.2, 4.3**
func TestStatsSumInvariant(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)

	properties.Property("Sum of ByPrefix values equals TotalOrganized", prop.ForAll(
		func(runs []TestRunData) bool {
			if len(runs) == 0 {
				return true
			}

			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "stats-sum-invariant-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tmpDir)

			// Create audit logs
			_, _, _, _, _, err = createTestAuditLogs(tmpDir, runs)
			if err != nil {
				t.Logf("Failed to create audit logs: %v", err)
				return false
			}

			// Aggregate stats with TopN=0 (all prefixes)
			stats, err := AggregateStats(tmpDir, StatsOptions{TopN: 0})
			if err != nil {
				t.Logf("AggregateStats failed: %v", err)
				return false
			}

			// CORE INVARIANT: sum(ByPrefix) == TotalOrganized
			sumOfPrefixes := 0
			for _, count := range stats.ByPrefix {
				sumOfPrefixes += count
			}

			if sumOfPrefixes != stats.TotalOrganized {
				t.Logf("INVARIANT VIOLATED: sum(ByPrefix)=%d != TotalOrganized=%d", sumOfPrefixes, stats.TotalOrganized)
				t.Logf("ByPrefix: %v", stats.ByPrefix)
				return false
			}

			return true
		},
		genTestRunsNonUndo(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Ensure reflect is used (for gopter generators)
var _ = reflect.TypeOf(TestRunData{})
