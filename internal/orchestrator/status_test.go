// Package orchestrator coordinates the file organization workflow for Sorta.
package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sorta/internal/config"
)

// TestStatusGroupsFilesByMatchingPrefix verifies that files are correctly grouped by their matching prefix destination.
// Requirements: 2.2 - Display pending files grouped by their destination (matched prefix)
func TestStatusGroupsFilesByMatchingPrefix(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "status-prefix-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	invoiceDir := filepath.Join(tempDir, "invoices")
	receiptDir := filepath.Join(tempDir, "receipts")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(invoiceDir, 0755)
	os.MkdirAll(receiptDir, 0755)

	// Create test files with different prefixes
	invoiceFile1 := filepath.Join(sourceDir, "Invoice 2024-03-15 Doc1.pdf")
	invoiceFile2 := filepath.Join(sourceDir, "Invoice 2024-04-20 Doc2.pdf")
	receiptFile := filepath.Join(sourceDir, "Receipt 2024-05-10 Doc3.pdf")

	os.WriteFile(invoiceFile1, []byte("invoice 1"), 0644)
	os.WriteFile(invoiceFile2, []byte("invoice 2"), 0644)
	os.WriteFile(receiptFile, []byte("receipt"), 0644)

	// Create config with multiple prefix rules
	cfg := &config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: invoiceDir},
			{Prefix: "Receipt", OutboundDirectory: receiptDir},
		},
	}

	// Create orchestrator and run status
	o := NewOrchestrator(cfg)
	result, err := o.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Verify the inbound directory is in results
	inboundStatus, ok := result.ByInbound[sourceDir]
	if !ok {
		t.Fatalf("Expected inbound directory %s in results", sourceDir)
	}

	// Verify files are grouped by destination
	// Invoice files should be grouped under "invoices/2024 Invoice"
	invoiceDest := filepath.Join(invoiceDir, "2024 Invoice")
	invoiceFiles, ok := inboundStatus.ByDestination[invoiceDest]
	if !ok {
		t.Errorf("Expected destination %s in results", invoiceDest)
	} else if len(invoiceFiles) != 2 {
		t.Errorf("Expected 2 invoice files, got %d", len(invoiceFiles))
	}

	// Receipt file should be grouped under "receipts/2024 Receipt"
	receiptDest := filepath.Join(receiptDir, "2024 Receipt")
	receiptFiles, ok := inboundStatus.ByDestination[receiptDest]
	if !ok {
		t.Errorf("Expected destination %s in results", receiptDest)
	} else if len(receiptFiles) != 1 {
		t.Errorf("Expected 1 receipt file, got %d", len(receiptFiles))
	}
}

// TestStatusGroupsUnmatchedFilesUnderForReview verifies that unmatched files are grouped under for-review directories.
// Requirements: 2.2 - Display pending files grouped by their destination (for-review)
func TestStatusGroupsUnmatchedFilesUnderForReview(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "status-review-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create files that won't match any prefix (will go to for-review)
	unmatchedFile1 := filepath.Join(sourceDir, "RandomFile1.pdf")
	unmatchedFile2 := filepath.Join(sourceDir, "RandomFile2.pdf")
	os.WriteFile(unmatchedFile1, []byte("random 1"), 0644)
	os.WriteFile(unmatchedFile2, []byte("random 2"), 0644)

	// Create config with a prefix that won't match
	cfg := &config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
		},
	}

	// Create orchestrator and run status
	o := NewOrchestrator(cfg)
	result, err := o.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Verify the inbound directory is in results
	inboundStatus, ok := result.ByInbound[sourceDir]
	if !ok {
		t.Fatalf("Expected inbound directory %s in results", sourceDir)
	}

	// Verify unmatched files are grouped under for-review
	forReviewDest := filepath.Join(sourceDir, "for-review")
	forReviewFiles, ok := inboundStatus.ByDestination[forReviewDest]
	if !ok {
		t.Errorf("Expected for-review destination %s in results", forReviewDest)
	} else if len(forReviewFiles) != 2 {
		t.Errorf("Expected 2 for-review files, got %d", len(forReviewFiles))
	}

	// Verify the for-review directory was NOT created (status is read-only)
	if _, err := os.Stat(forReviewDest); !os.IsNotExist(err) {
		t.Error("For-review directory should NOT have been created by status command")
	}
}

// TestStatusPerDirectoryCountsAreAccurate verifies that per-directory counts are accurate.
// Requirements: 2.3 - Display the total count of pending files per inbound directory
func TestStatusPerDirectoryCountsAreAccurate(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "status-counts-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir1 := filepath.Join(tempDir, "source1")
	sourceDir2 := filepath.Join(tempDir, "source2")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir1, 0755)
	os.MkdirAll(sourceDir2, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create 3 files in source1
	os.WriteFile(filepath.Join(sourceDir1, "Invoice 2024-01-01 A.pdf"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(sourceDir1, "Invoice 2024-01-02 B.pdf"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(sourceDir1, "RandomFile.pdf"), []byte("c"), 0644)

	// Create 2 files in source2
	os.WriteFile(filepath.Join(sourceDir2, "Invoice 2024-02-01 D.pdf"), []byte("d"), 0644)
	os.WriteFile(filepath.Join(sourceDir2, "Receipt 2024-02-02 E.pdf"), []byte("e"), 0644)

	// Create config
	cfg := &config.Configuration{
		InboundDirectories: []string{sourceDir1, sourceDir2},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
			{Prefix: "Receipt", OutboundDirectory: targetDir},
		},
	}

	// Create orchestrator and run status
	o := NewOrchestrator(cfg)
	result, err := o.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Verify source1 has 3 files
	inbound1, ok := result.ByInbound[sourceDir1]
	if !ok {
		t.Fatalf("Expected inbound directory %s in results", sourceDir1)
	}
	if inbound1.Total != 3 {
		t.Errorf("Expected source1 total of 3, got %d", inbound1.Total)
	}

	// Verify source2 has 2 files
	inbound2, ok := result.ByInbound[sourceDir2]
	if !ok {
		t.Fatalf("Expected inbound directory %s in results", sourceDir2)
	}
	if inbound2.Total != 2 {
		t.Errorf("Expected source2 total of 2, got %d", inbound2.Total)
	}

	// Verify per-directory count equals sum of files in ByDestination
	var countFromDestinations int
	for _, files := range inbound1.ByDestination {
		countFromDestinations += len(files)
	}
	if countFromDestinations != inbound1.Total {
		t.Errorf("source1: ByDestination sum (%d) != Total (%d)", countFromDestinations, inbound1.Total)
	}

	countFromDestinations = 0
	for _, files := range inbound2.ByDestination {
		countFromDestinations += len(files)
	}
	if countFromDestinations != inbound2.Total {
		t.Errorf("source2: ByDestination sum (%d) != Total (%d)", countFromDestinations, inbound2.Total)
	}
}

// TestStatusGrandTotalEqualsSumOfAllGroups verifies that grand total equals the sum of all per-directory counts.
// Requirements: 2.4 - Display a grand total of all pending files
func TestStatusGrandTotalEqualsSumOfAllGroups(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "status-grandtotal-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir1 := filepath.Join(tempDir, "source1")
	sourceDir2 := filepath.Join(tempDir, "source2")
	sourceDir3 := filepath.Join(tempDir, "source3")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir1, 0755)
	os.MkdirAll(sourceDir2, 0755)
	os.MkdirAll(sourceDir3, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create files in each source directory
	// source1: 2 files
	os.WriteFile(filepath.Join(sourceDir1, "Invoice 2024-01-01 A.pdf"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(sourceDir1, "Invoice 2024-01-02 B.pdf"), []byte("b"), 0644)

	// source2: 3 files
	os.WriteFile(filepath.Join(sourceDir2, "Receipt 2024-02-01 C.pdf"), []byte("c"), 0644)
	os.WriteFile(filepath.Join(sourceDir2, "Receipt 2024-02-02 D.pdf"), []byte("d"), 0644)
	os.WriteFile(filepath.Join(sourceDir2, "RandomFile.pdf"), []byte("e"), 0644)

	// source3: 1 file
	os.WriteFile(filepath.Join(sourceDir3, "Invoice 2024-03-01 F.pdf"), []byte("f"), 0644)

	// Create config
	cfg := &config.Configuration{
		InboundDirectories: []string{sourceDir1, sourceDir2, sourceDir3},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
			{Prefix: "Receipt", OutboundDirectory: targetDir},
		},
	}

	// Create orchestrator and run status
	o := NewOrchestrator(cfg)
	result, err := o.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Calculate expected grand total (2 + 3 + 1 = 6)
	expectedGrandTotal := 6

	// Verify grand total
	if result.GrandTotal != expectedGrandTotal {
		t.Errorf("Expected grand total of %d, got %d", expectedGrandTotal, result.GrandTotal)
	}

	// Verify grand total equals sum of all per-directory totals
	var sumOfTotals int
	for _, inboundStatus := range result.ByInbound {
		sumOfTotals += inboundStatus.Total
	}
	if sumOfTotals != result.GrandTotal {
		t.Errorf("Sum of per-directory totals (%d) != GrandTotal (%d)", sumOfTotals, result.GrandTotal)
	}
}

// TestStatusWithEmptyDirectories verifies status handles empty directories correctly.
// Requirements: 2.5 - When no pending files exist, display appropriate message
func TestStatusWithEmptyDirectories(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "status-empty-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Don't create any files in sourceDir

	// Create config
	cfg := &config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
		},
	}

	// Create orchestrator and run status
	o := NewOrchestrator(cfg)
	result, err := o.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Verify grand total is 0
	if result.GrandTotal != 0 {
		t.Errorf("Expected grand total of 0 for empty directory, got %d", result.GrandTotal)
	}

	// Verify inbound directory is in results with 0 total
	inboundStatus, ok := result.ByInbound[sourceDir]
	if !ok {
		t.Fatalf("Expected inbound directory %s in results", sourceDir)
	}
	if inboundStatus.Total != 0 {
		t.Errorf("Expected total of 0 for empty directory, got %d", inboundStatus.Total)
	}
	if len(inboundStatus.ByDestination) != 0 {
		t.Errorf("Expected empty ByDestination for empty directory, got %d entries", len(inboundStatus.ByDestination))
	}
}

// TestStatusWithMixedMatchedAndUnmatchedFiles verifies status correctly handles mixed files.
// Requirements: 2.2, 3.2 - Group files by destination directory
func TestStatusWithMixedMatchedAndUnmatchedFiles(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "status-mixed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create matched files
	os.WriteFile(filepath.Join(sourceDir, "Invoice 2024-01-01 A.pdf"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "Receipt 2024-02-01 B.pdf"), []byte("b"), 0644)

	// Create unmatched files
	os.WriteFile(filepath.Join(sourceDir, "RandomFile1.pdf"), []byte("c"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "RandomFile2.pdf"), []byte("d"), 0644)

	// Create config
	cfg := &config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
			{Prefix: "Receipt", OutboundDirectory: targetDir},
		},
	}

	// Create orchestrator and run status
	o := NewOrchestrator(cfg)
	result, err := o.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Verify total count
	inboundStatus := result.ByInbound[sourceDir]
	if inboundStatus.Total != 4 {
		t.Errorf("Expected total of 4, got %d", inboundStatus.Total)
	}

	// Verify we have 3 destination groups (Invoice, Receipt, for-review)
	if len(inboundStatus.ByDestination) != 3 {
		t.Errorf("Expected 3 destination groups, got %d", len(inboundStatus.ByDestination))
	}

	// Verify for-review has 2 files
	forReviewDest := filepath.Join(sourceDir, "for-review")
	forReviewFiles := inboundStatus.ByDestination[forReviewDest]
	if len(forReviewFiles) != 2 {
		t.Errorf("Expected 2 for-review files, got %d", len(forReviewFiles))
	}
}

// TestStatusDoesNotModifyFilesystem verifies that status command is read-only.
// Requirements: 2.6 - Status shall not modify any files or directories
func TestStatusDoesNotModifyFilesystem(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "status-readonly-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create test files
	testFile := filepath.Join(sourceDir, "Invoice 2024-03-15 TestDoc.pdf")
	os.WriteFile(testFile, []byte("test content"), 0644)

	// Record initial state
	initialSourceEntries, _ := os.ReadDir(sourceDir)
	initialTargetEntries, _ := os.ReadDir(targetDir)

	// Create config
	cfg := &config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
		},
	}

	// Create orchestrator and run status
	o := NewOrchestrator(cfg)
	_, err = o.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Verify source directory unchanged
	finalSourceEntries, _ := os.ReadDir(sourceDir)
	if len(finalSourceEntries) != len(initialSourceEntries) {
		t.Errorf("Source directory was modified: had %d entries, now has %d",
			len(initialSourceEntries), len(finalSourceEntries))
	}

	// Verify target directory unchanged
	finalTargetEntries, _ := os.ReadDir(targetDir)
	if len(finalTargetEntries) != len(initialTargetEntries) {
		t.Errorf("Target directory was modified: had %d entries, now has %d",
			len(initialTargetEntries), len(finalTargetEntries))
	}

	// Verify source file still exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Source file should NOT have been moved by status command")
	}
}

// TestStatusHandlesMissingInboundDirectory verifies status handles missing directories gracefully.
// Error handling - Report error, continue with other directories
func TestStatusHandlesMissingInboundDirectory(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "status-missing-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	existingDir := filepath.Join(tempDir, "existing")
	missingDir := filepath.Join(tempDir, "missing")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(existingDir, 0755)
	os.MkdirAll(targetDir, 0755)
	// Don't create missingDir

	// Create test file in existing directory
	os.WriteFile(filepath.Join(existingDir, "Invoice 2024-03-15 TestDoc.pdf"), []byte("test"), 0644)

	// Create config with both existing and missing directories
	cfg := &config.Configuration{
		InboundDirectories: []string{existingDir, missingDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
		},
	}

	// Create orchestrator and run status
	o := NewOrchestrator(cfg)
	result, err := o.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Verify existing directory was processed
	existingStatus, ok := result.ByInbound[existingDir]
	if !ok {
		t.Fatalf("Expected existing directory %s in results", existingDir)
	}
	if existingStatus.Total != 1 {
		t.Errorf("Expected 1 file in existing directory, got %d", existingStatus.Total)
	}

	// Verify missing directory is in results with 0 total
	missingStatus, ok := result.ByInbound[missingDir]
	if !ok {
		t.Fatalf("Expected missing directory %s in results", missingDir)
	}
	if missingStatus.Total != 0 {
		t.Errorf("Expected 0 files for missing directory, got %d", missingStatus.Total)
	}

	// Verify grand total only counts existing directory
	if result.GrandTotal != 1 {
		t.Errorf("Expected grand total of 1, got %d", result.GrandTotal)
	}
}

// TestStatusFromPathConvenienceFunction verifies the StatusFromPath convenience function works.
func TestStatusFromPathConvenienceFunction(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "status-frompath-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create test file
	os.WriteFile(filepath.Join(sourceDir, "Invoice 2024-03-15 TestDoc.pdf"), []byte("test"), 0644)

	// Create config file
	cfg := config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
		},
	}
	configPath := filepath.Join(tempDir, "config.json")
	configData, _ := json.Marshal(cfg)
	os.WriteFile(configPath, configData, 0644)

	// Use convenience function
	result, err := StatusFromPath(configPath)
	if err != nil {
		t.Fatalf("StatusFromPath failed: %v", err)
	}

	// Verify result
	if result.GrandTotal != 1 {
		t.Errorf("Expected grand total of 1, got %d", result.GrandTotal)
	}
}

// TestStatusWithMultipleYears verifies files from different years are grouped correctly.
// Requirements: 2.2 - Group files by destination (year subfolder)
func TestStatusWithMultipleYears(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "status-years-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create files from different years
	os.WriteFile(filepath.Join(sourceDir, "Invoice 2023-06-15 OldDoc.pdf"), []byte("old"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "Invoice 2024-03-15 NewDoc.pdf"), []byte("new"), 0644)

	// Create config
	cfg := &config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
		},
	}

	// Create orchestrator and run status
	o := NewOrchestrator(cfg)
	result, err := o.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	inboundStatus := result.ByInbound[sourceDir]

	// Verify files are grouped by year
	dest2023 := filepath.Join(targetDir, "2023 Invoice")
	dest2024 := filepath.Join(targetDir, "2024 Invoice")

	files2023 := inboundStatus.ByDestination[dest2023]
	files2024 := inboundStatus.ByDestination[dest2024]

	if len(files2023) != 1 {
		t.Errorf("Expected 1 file for 2023, got %d", len(files2023))
	}
	if len(files2024) != 1 {
		t.Errorf("Expected 1 file for 2024, got %d", len(files2024))
	}

	// Verify total is correct
	if inboundStatus.Total != 2 {
		t.Errorf("Expected total of 2, got %d", inboundStatus.Total)
	}
}
