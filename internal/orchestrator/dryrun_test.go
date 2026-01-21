// Package orchestrator coordinates the file organization workflow for Sorta.
package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sorta/internal/config"
)

// TestDryRunDoesNotMoveFiles verifies that dry-run mode does not move files.
// Requirements: 1.1, 1.4 - Dry run simulates without modifying filesystem
func TestDryRunDoesNotMoveFiles(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "dryrun-test-*")
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

	// Create config
	cfg := config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
		},
	}
	configPath := filepath.Join(tempDir, "config.json")
	configData, _ := json.Marshal(cfg)
	os.WriteFile(configPath, configData, 0644)

	// Run in dry-run mode
	opts := RunOptions{DryRun: true}
	result, err := RunDryRun(configPath, opts)
	if err != nil {
		t.Fatalf("RunDryRun failed: %v", err)
	}

	// Verify source file still exists (not moved)
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Source file should NOT have been moved in dry-run mode")
	}

	// Verify target directory is empty (no files moved)
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		t.Fatalf("Failed to read target dir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("Target directory should be empty in dry-run mode, but has %d entries", len(entries))
	}

	// Verify result contains the planned operation
	if len(result.Moved) != 1 {
		t.Errorf("Expected 1 planned move operation, got %d", len(result.Moved))
	}
}

// TestDryRunDoesNotCreateDirectories verifies that dry-run mode does not create directories.
// Requirements: 1.4 - Dry run shall not create any directories
func TestDryRunDoesNotCreateDirectories(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "dryrun-nodir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create test file
	testFile := filepath.Join(sourceDir, "Invoice 2024-03-15 TestDoc.pdf")
	os.WriteFile(testFile, []byte("test content"), 0644)

	// Create config
	cfg := config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
		},
	}
	configPath := filepath.Join(tempDir, "config.json")
	configData, _ := json.Marshal(cfg)
	os.WriteFile(configPath, configData, 0644)

	// Run in dry-run mode
	opts := RunOptions{DryRun: true}
	_, err = RunDryRun(configPath, opts)
	if err != nil {
		t.Fatalf("RunDryRun failed: %v", err)
	}

	// Verify the year subfolder was NOT created
	expectedSubfolder := filepath.Join(targetDir, "2024 Invoice")
	if _, err := os.Stat(expectedSubfolder); !os.IsNotExist(err) {
		t.Error("Year subfolder should NOT have been created in dry-run mode")
	}
}

// TestDryRunCollectsForReviewFiles verifies that dry-run mode correctly identifies for-review files.
// Requirements: 1.3 - Dry run shall display files that would go to for-review directories
func TestDryRunCollectsForReviewFiles(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "dryrun-review-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create a file that won't match any prefix (will go to for-review)
	unmatchedFile := filepath.Join(sourceDir, "RandomFile.pdf")
	os.WriteFile(unmatchedFile, []byte("test content"), 0644)

	// Create config with a prefix that won't match
	cfg := config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
		},
	}
	configPath := filepath.Join(tempDir, "config.json")
	configData, _ := json.Marshal(cfg)
	os.WriteFile(configPath, configData, 0644)

	// Run in dry-run mode
	opts := RunOptions{DryRun: true}
	result, err := RunDryRun(configPath, opts)
	if err != nil {
		t.Fatalf("RunDryRun failed: %v", err)
	}

	// Verify file is in ForReview list
	if len(result.ForReview) != 1 {
		t.Errorf("Expected 1 for-review file, got %d", len(result.ForReview))
	}

	// Verify the for-review directory was NOT created
	forReviewDir := filepath.Join(sourceDir, "for-review")
	if _, err := os.Stat(forReviewDir); !os.IsNotExist(err) {
		t.Error("For-review directory should NOT have been created in dry-run mode")
	}

	// Verify source file still exists
	if _, err := os.Stat(unmatchedFile); os.IsNotExist(err) {
		t.Error("Source file should NOT have been moved in dry-run mode")
	}
}

// TestDryRunReturnsCorrectDestinations verifies that dry-run mode returns correct destination paths.
// Requirements: 1.2, 3.1 - Dry run shall display each file with its destination path
func TestDryRunReturnsCorrectDestinations(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "dryrun-dest-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create test file
	testFile := filepath.Join(sourceDir, "Invoice 2024-03-15 TestDoc.pdf")
	os.WriteFile(testFile, []byte("test content"), 0644)

	// Create config
	cfg := config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
		},
	}
	configPath := filepath.Join(tempDir, "config.json")
	configData, _ := json.Marshal(cfg)
	os.WriteFile(configPath, configData, 0644)

	// Run in dry-run mode
	opts := RunOptions{DryRun: true}
	result, err := RunDryRun(configPath, opts)
	if err != nil {
		t.Fatalf("RunDryRun failed: %v", err)
	}

	// Verify result contains correct source and destination
	if len(result.Moved) != 1 {
		t.Fatalf("Expected 1 planned move operation, got %d", len(result.Moved))
	}

	op := result.Moved[0]
	if op.Source != testFile {
		t.Errorf("Expected source %s, got %s", testFile, op.Source)
	}

	expectedDest := filepath.Join(targetDir, "2024 Invoice", "Invoice 2024-03-15 TestDoc.pdf")
	if op.Destination != expectedDest {
		t.Errorf("Expected destination %s, got %s", expectedDest, op.Destination)
	}

	if op.Prefix != "Invoice" {
		t.Errorf("Expected prefix 'Invoice', got '%s'", op.Prefix)
	}
}

// TestDryRunWithMixedFiles verifies dry-run mode correctly categorizes mixed files.
// Requirements: 1.2, 1.3 - Dry run shall display files that would be moved and for-review
func TestDryRunWithMixedFiles(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "dryrun-mixed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create files that will be classified
	classifiedFile1 := filepath.Join(sourceDir, "Invoice 2024-03-15 Doc1.pdf")
	classifiedFile2 := filepath.Join(sourceDir, "Receipt 2024-04-20 Doc2.pdf")
	os.WriteFile(classifiedFile1, []byte("test content 1"), 0644)
	os.WriteFile(classifiedFile2, []byte("test content 2"), 0644)

	// Create files that won't match (for-review)
	unmatchedFile := filepath.Join(sourceDir, "RandomFile.pdf")
	os.WriteFile(unmatchedFile, []byte("test content 3"), 0644)

	// Create config
	cfg := config.Configuration{
		InboundDirectories: []string{sourceDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
			{Prefix: "Receipt", OutboundDirectory: targetDir},
		},
	}
	configPath := filepath.Join(tempDir, "config.json")
	configData, _ := json.Marshal(cfg)
	os.WriteFile(configPath, configData, 0644)

	// Run in dry-run mode
	opts := RunOptions{DryRun: true}
	result, err := RunDryRun(configPath, opts)
	if err != nil {
		t.Fatalf("RunDryRun failed: %v", err)
	}

	// Verify correct categorization
	if len(result.Moved) != 2 {
		t.Errorf("Expected 2 planned move operations, got %d", len(result.Moved))
	}

	if len(result.ForReview) != 1 {
		t.Errorf("Expected 1 for-review file, got %d", len(result.ForReview))
	}

	// Verify no files were actually moved
	if _, err := os.Stat(classifiedFile1); os.IsNotExist(err) {
		t.Error("Classified file 1 should NOT have been moved")
	}
	if _, err := os.Stat(classifiedFile2); os.IsNotExist(err) {
		t.Error("Classified file 2 should NOT have been moved")
	}
	if _, err := os.Stat(unmatchedFile); os.IsNotExist(err) {
		t.Error("Unmatched file should NOT have been moved")
	}
}

// TestDryRunHandlesMissingInboundDirectory verifies dry-run mode handles missing directories.
// Requirements: Error handling - Report error, continue with other directories
func TestDryRunHandlesMissingInboundDirectory(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "dryrun-missing-test-*")
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
	testFile := filepath.Join(existingDir, "Invoice 2024-03-15 TestDoc.pdf")
	os.WriteFile(testFile, []byte("test content"), 0644)

	// Create config with both existing and missing directories
	cfg := config.Configuration{
		InboundDirectories: []string{existingDir, missingDir},
		PrefixRules: []config.PrefixRule{
			{Prefix: "Invoice", OutboundDirectory: targetDir},
		},
	}
	configPath := filepath.Join(tempDir, "config.json")
	configData, _ := json.Marshal(cfg)
	os.WriteFile(configPath, configData, 0644)

	// Run in dry-run mode
	opts := RunOptions{DryRun: true}
	result, err := RunDryRun(configPath, opts)
	if err != nil {
		t.Fatalf("RunDryRun failed: %v", err)
	}

	// Verify error was recorded for missing directory
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error for missing directory, got %d", len(result.Errors))
	}

	// Verify file from existing directory was still processed
	if len(result.Moved) != 1 {
		t.Errorf("Expected 1 planned move operation from existing directory, got %d", len(result.Moved))
	}
}
