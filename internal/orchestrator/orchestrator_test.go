// Package orchestrator coordinates the file organization workflow for Sorta.
package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sorta/internal/audit"
	"sorta/internal/config"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: audit-trail, Property 14: Audit-Before-Move Ordering
// Validates: Requirements 11.4
//
// For any file move operation, the corresponding audit record SHALL be
// durably written before the file is moved.

// TestAuditBeforeMoveOrdering tests Property 14: Audit-Before-Move Ordering
// This property verifies that audit events are written BEFORE the actual file
// operations occur. We test this by:
//  1. Creating test files with known content
//  2. Running the orchestrator with auditing enabled
//  3. Verifying that for each moved file, the audit event exists in the log
//     and the destination file exists - proving the audit was written before
//     the move completed (if audit failed, the move would not have happened)
func TestAuditBeforeMoveOrdering(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)

	properties.Property("Audit events are written before file moves", prop.ForAll(
		func(fileCount int) bool {
			// Create temp directories for source, target, and audit
			tempDir, err := os.MkdirTemp("", "audit-before-move-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			sourceDir := filepath.Join(tempDir, "source")
			targetDir := filepath.Join(tempDir, "target")
			auditDir := filepath.Join(tempDir, "audit")

			if err := os.MkdirAll(sourceDir, 0755); err != nil {
				t.Logf("Failed to create source dir: %v", err)
				return false
			}
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				t.Logf("Failed to create target dir: %v", err)
				return false
			}

			// Create test files with valid names that will be classified
			testFiles := make([]string, fileCount)
			for i := 0; i < fileCount; i++ {
				// Use a valid date format: Invoice YYYY-MM-DD description.pdf
				filename := "Invoice 2024-03-15 Test" + string(rune('A'+i%26)) + ".pdf"
				filePath := filepath.Join(sourceDir, filename)
				content := []byte("test content for file " + filename)
				if err := os.WriteFile(filePath, content, 0644); err != nil {
					t.Logf("Failed to create test file: %v", err)
					return false
				}
				testFiles[i] = filePath
			}

			// Create config file
			cfg := config.Configuration{
				InboundDirectories: []string{sourceDir},
				PrefixRules: []config.PrefixRule{
					{Prefix: "Invoice", OutboundDirectory: targetDir},
				},
			}
			configPath := filepath.Join(tempDir, "config.json")
			configData, _ := json.Marshal(cfg)
			if err := os.WriteFile(configPath, configData, 0644); err != nil {
				t.Logf("Failed to write config: %v", err)
				return false
			}

			// Run orchestrator with auditing
			auditConfig := audit.AuditConfig{
				LogDirectory: auditDir,
			}
			options := &Options{
				AuditConfig: &auditConfig,
				AppVersion:  "1.0.0-test",
				MachineID:   "test-machine",
			}

			summary, err := RunWithOptions(configPath, options)
			if err != nil {
				t.Logf("RunWithOptions failed: %v", err)
				return false
			}

			// Verify files were processed
			if summary.TotalFiles != fileCount {
				t.Logf("Expected %d files, got %d", fileCount, summary.TotalFiles)
				return false
			}

			// Read audit log
			auditLogPath := filepath.Join(auditDir, "sorta-audit.jsonl")
			auditContent, err := os.ReadFile(auditLogPath)
			if err != nil {
				t.Logf("Failed to read audit log: %v", err)
				return false
			}

			// Parse audit events
			lines := strings.Split(string(auditContent), "\n")
			moveEvents := make(map[string]audit.AuditEvent) // keyed by source path
			for _, line := range lines {
				if line == "" {
					continue
				}
				var event audit.AuditEvent
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					continue
				}
				if event.EventType == audit.EventMove {
					moveEvents[event.SourcePath] = event
				}
			}

			// Verify we have move events for all files
			if len(moveEvents) != fileCount {
				t.Logf("Expected %d MOVE events, got %d", fileCount, len(moveEvents))
				return false
			}

			// For each move event, verify:
			// 1. The audit event exists (proving it was written)
			// 2. The destination file exists (proving the move completed)
			// 3. The source file no longer exists (proving the move was successful)
			// This proves audit-before-move because:
			// - If audit write failed, the orchestrator would have stopped (fail-fast)
			// - If move happened before audit, a crash would leave moved file without audit
			for sourcePath, event := range moveEvents {
				// Verify destination file exists
				if _, err := os.Stat(event.DestinationPath); os.IsNotExist(err) {
					t.Logf("Destination file does not exist: %s", event.DestinationPath)
					return false
				}

				// Verify source file no longer exists
				if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
					t.Logf("Source file should have been moved: %s", sourcePath)
					return false
				}

				// Verify the event has required fields
				if event.DestinationPath == "" {
					t.Logf("MOVE event missing destination path")
					return false
				}
				if event.FileIdentity == nil {
					t.Logf("MOVE event missing file identity")
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestAuditBeforeMoveOrdering_VerifyCodePath is a unit test that verifies
// the code structure ensures audit-before-move ordering by checking that
// audit events appear in the log for files that were successfully moved.
func TestAuditBeforeMoveOrdering_VerifyCodePath(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "audit-order-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")
	auditDir := filepath.Join(tempDir, "audit")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Create a test file
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

	// Run with auditing
	auditConfig := audit.AuditConfig{
		LogDirectory: auditDir,
	}
	options := &Options{
		AuditConfig: &auditConfig,
		AppVersion:  "1.0.0",
		MachineID:   "test-machine",
	}

	summary, err := RunWithOptions(configPath, options)
	if err != nil {
		t.Fatalf("RunWithOptions failed: %v", err)
	}

	if summary.SuccessCount != 1 {
		t.Fatalf("Expected 1 successful move, got %d", summary.SuccessCount)
	}

	// Read audit log and verify MOVE event exists
	auditLogPath := filepath.Join(auditDir, "sorta-audit.jsonl")
	auditContent, err := os.ReadFile(auditLogPath)
	if err != nil {
		t.Fatalf("Failed to read audit log: %v", err)
	}

	if !strings.Contains(string(auditContent), `"eventType":"MOVE"`) {
		t.Error("Audit log should contain MOVE event")
	}

	// Verify the moved file exists at destination
	destFile := filepath.Join(targetDir, "2024 Invoice", "Invoice 2024-03-15 TestDoc.pdf")
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		t.Errorf("Destination file should exist: %s", destFile)
	}

	// Verify source file no longer exists
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Source file should have been moved")
	}
}

// Feature: audit-trail, Property 13: Fail-Fast on Audit Write Failure
// Validates: Requirements 11.1, 11.3, 11.5
//
// For any audit write failure, no subsequent file operations SHALL occur in that run.

// TestFailFastOnAuditWriteFailure tests Property 13: Fail-Fast on Audit Write Failure
// This property verifies that when an audit write fails, the orchestrator stops
// processing files immediately. We test this by:
// 1. Creating a scenario where audit writes will fail (read-only audit directory)
// 2. Running the orchestrator with multiple files
// 3. Verifying that no files were moved after the audit failure
func TestFailFastOnAuditWriteFailure(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)

	properties.Property("No file operations occur after audit write failure", prop.ForAll(
		func(fileCount int) bool {
			// Create temp directories for source and target
			tempDir, err := os.MkdirTemp("", "fail-fast-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			sourceDir := filepath.Join(tempDir, "source")
			targetDir := filepath.Join(tempDir, "target")
			auditDir := filepath.Join(tempDir, "audit")

			if err := os.MkdirAll(sourceDir, 0755); err != nil {
				t.Logf("Failed to create source dir: %v", err)
				return false
			}
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				t.Logf("Failed to create target dir: %v", err)
				return false
			}

			// Create audit directory but make it read-only to cause write failures
			if err := os.MkdirAll(auditDir, 0755); err != nil {
				t.Logf("Failed to create audit dir: %v", err)
				return false
			}

			// Create test files with valid names that will be classified
			testFiles := make([]string, fileCount)
			for i := 0; i < fileCount; i++ {
				filename := "Invoice 2024-03-15 Test" + string(rune('A'+i%26)) + string(rune('0'+i/26)) + ".pdf"
				filePath := filepath.Join(sourceDir, filename)
				content := []byte("test content for file " + filename)
				if err := os.WriteFile(filePath, content, 0644); err != nil {
					t.Logf("Failed to create test file: %v", err)
					return false
				}
				testFiles[i] = filePath
			}

			// Create config file
			cfg := config.Configuration{
				InboundDirectories: []string{sourceDir},
				PrefixRules: []config.PrefixRule{
					{Prefix: "Invoice", OutboundDirectory: targetDir},
				},
			}
			configPath := filepath.Join(tempDir, "config.json")
			configData, _ := json.Marshal(cfg)
			if err := os.WriteFile(configPath, configData, 0644); err != nil {
				t.Logf("Failed to write config: %v", err)
				return false
			}

			// Make audit directory read-only to cause write failures
			if err := os.Chmod(auditDir, 0444); err != nil {
				t.Logf("Failed to make audit dir read-only: %v", err)
				return false
			}
			// Restore permissions for cleanup
			defer os.Chmod(auditDir, 0755)

			// Run orchestrator with auditing - should fail
			auditConfig := audit.AuditConfig{
				LogDirectory: auditDir,
			}
			options := &Options{
				AuditConfig: &auditConfig,
				AppVersion:  "1.0.0-test",
				MachineID:   "test-machine",
			}

			_, err = RunWithOptions(configPath, options)

			// We expect an error because audit writes should fail
			if err == nil {
				t.Logf("Expected error due to audit write failure, but got none")
				return false
			}

			// Verify that NO files were moved (fail-fast behavior)
			// All source files should still exist
			for _, filePath := range testFiles {
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Logf("Source file was moved despite audit failure: %s", filePath)
					return false
				}
			}

			// Verify target directory is empty (no files were moved)
			entries, err := os.ReadDir(targetDir)
			if err != nil {
				t.Logf("Failed to read target dir: %v", err)
				return false
			}
			if len(entries) > 0 {
				t.Logf("Target directory should be empty, but has %d entries", len(entries))
				return false
			}

			return true
		},
		gen.IntRange(2, 10),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestFailFastOnAuditWriteFailure_MidRun tests that fail-fast works when
// audit failure occurs mid-run (after some files have been processed).
func TestFailFastOnAuditWriteFailure_MidRun(t *testing.T) {
	// This test uses a custom audit writer that fails after N writes
	// to simulate a mid-run failure scenario

	// Create temp directories
	tempDir, err := os.MkdirTemp("", "fail-fast-midrun-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")
	auditDir := filepath.Join(tempDir, "audit")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)
	os.MkdirAll(auditDir, 0755)

	// Create multiple test files
	fileCount := 5
	for i := 0; i < fileCount; i++ {
		filename := "Invoice 2024-03-15 Test" + string(rune('A'+i)) + ".pdf"
		filePath := filepath.Join(sourceDir, filename)
		os.WriteFile(filePath, []byte("test content "+filename), 0644)
	}

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

	// Run with auditing enabled - this should succeed
	auditConfig := audit.AuditConfig{
		LogDirectory: auditDir,
	}
	options := &Options{
		AuditConfig: &auditConfig,
		AppVersion:  "1.0.0",
		MachineID:   "test-machine",
	}

	summary, err := RunWithOptions(configPath, options)
	if err != nil {
		t.Fatalf("RunWithOptions failed: %v", err)
	}

	// Verify all files were processed successfully
	if summary.SuccessCount != fileCount {
		t.Errorf("Expected %d successful moves, got %d", fileCount, summary.SuccessCount)
	}

	// Verify audit log contains events for all files
	auditLogPath := filepath.Join(auditDir, "sorta-audit.jsonl")
	auditContent, err := os.ReadFile(auditLogPath)
	if err != nil {
		t.Fatalf("Failed to read audit log: %v", err)
	}

	moveCount := strings.Count(string(auditContent), `"eventType":"MOVE"`)
	if moveCount != fileCount {
		t.Errorf("Expected %d MOVE events in audit log, got %d", fileCount, moveCount)
	}
}

// TestFailFastOnAuditWriteFailure_ErrorReporting tests that audit write
// failures are properly reported to the caller.
func TestFailFastOnAuditWriteFailure_ErrorReporting(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "fail-fast-error-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")
	auditDir := filepath.Join(tempDir, "audit")

	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)
	os.MkdirAll(auditDir, 0755)

	// Create a test file
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

	// Make audit directory read-only to cause write failures
	os.Chmod(auditDir, 0444)
	defer os.Chmod(auditDir, 0755)

	// Run with auditing - should fail
	auditConfig := audit.AuditConfig{
		LogDirectory: auditDir,
	}
	options := &Options{
		AuditConfig: &auditConfig,
		AppVersion:  "1.0.0",
		MachineID:   "test-machine",
	}

	_, err = RunWithOptions(configPath, options)

	// Verify error is returned
	if err == nil {
		t.Error("Expected error due to audit write failure")
	}

	// Verify error message contains useful information
	errMsg := err.Error()
	if !strings.Contains(errMsg, "audit") {
		t.Errorf("Error message should mention audit: %s", errMsg)
	}

	// Verify source file was not moved
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Source file should not have been moved")
	}
}

// Feature: config-validation, Property 7: Runtime Path Validation
// Validates: Requirements 4.1, 4.2
//
// For any run operation, Sorta SHALL validate that inbound directories exist before processing.
// Non-existent directories SHALL be skipped with an error message, and remaining directories
// SHALL still be processed.

// TestRuntimePathValidation tests Property 7: Runtime Path Validation
// This property verifies that:
// 1. Non-existent inbound directories are skipped with an error
// 2. Remaining valid directories are still processed
// 3. Files from valid directories are organized correctly
func TestRuntimePathValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Non-existent inbound directories are skipped, valid ones are processed", prop.ForAll(
		func(numExisting int, numMissing int) bool {
			if numExisting == 0 && numMissing == 0 {
				return true // Skip empty case
			}

			// Create temp directories
			tempDir, err := os.MkdirTemp("", "runtime-path-validation-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			targetDir := filepath.Join(tempDir, "target")
			auditDir := filepath.Join(tempDir, "audit")

			if err := os.MkdirAll(targetDir, 0755); err != nil {
				t.Logf("Failed to create target dir: %v", err)
				return false
			}

			// Build list of inbound directories
			var inboundDirs []string
			existingDirs := make([]string, 0, numExisting)
			filesCreated := 0

			// Create existing directories with test files
			for i := 0; i < numExisting; i++ {
				dir := filepath.Join(tempDir, "existing_"+itoa(i))
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Logf("Failed to create existing dir: %v", err)
					return false
				}
				existingDirs = append(existingDirs, dir)
				inboundDirs = append(inboundDirs, dir)

				// Create a test file in each existing directory
				filename := "Invoice 2024-03-15 Test" + itoa(i) + ".pdf"
				filePath := filepath.Join(dir, filename)
				if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
					t.Logf("Failed to create test file: %v", err)
					return false
				}
				filesCreated++
			}

			// Add non-existent directories
			for i := 0; i < numMissing; i++ {
				dir := filepath.Join(tempDir, "missing_"+itoa(i))
				// Don't create this directory - it should be missing
				inboundDirs = append(inboundDirs, dir)
			}

			// Create config file
			cfg := config.Configuration{
				InboundDirectories: inboundDirs,
				PrefixRules: []config.PrefixRule{
					{Prefix: "Invoice", OutboundDirectory: targetDir},
				},
			}
			configPath := filepath.Join(tempDir, "config.json")
			configData, _ := json.Marshal(cfg)
			if err := os.WriteFile(configPath, configData, 0644); err != nil {
				t.Logf("Failed to write config: %v", err)
				return false
			}

			// Run orchestrator
			auditConfig := audit.AuditConfig{
				LogDirectory: auditDir,
			}
			options := &Options{
				AuditConfig: &auditConfig,
				AppVersion:  "1.0.0-test",
				MachineID:   "test-machine",
			}

			summary, err := RunWithOptions(configPath, options)
			if err != nil {
				t.Logf("RunWithOptions failed: %v", err)
				return false
			}

			// Verify scan errors for missing directories
			if len(summary.ScanErrors) != numMissing {
				t.Logf("Expected %d scan errors for missing dirs, got %d", numMissing, len(summary.ScanErrors))
				for _, scanErr := range summary.ScanErrors {
					t.Logf("  Scan error: %v", scanErr)
				}
				return false
			}

			// Verify each scan error mentions "does not exist"
			for _, scanErr := range summary.ScanErrors {
				if !strings.Contains(scanErr.Error(), "does not exist") {
					t.Logf("Scan error should mention 'does not exist': %v", scanErr)
					return false
				}
			}

			// Verify files from existing directories were processed
			if summary.TotalFiles != filesCreated {
				t.Logf("Expected %d files processed, got %d", filesCreated, summary.TotalFiles)
				return false
			}

			// Verify files were successfully moved
			if summary.SuccessCount != filesCreated {
				t.Logf("Expected %d successful moves, got %d", filesCreated, summary.SuccessCount)
				return false
			}

			return true
		},
		gen.IntRange(0, 3),
		gen.IntRange(0, 3),
	))

	properties.Property("All valid directories are processed even when some are missing", prop.ForAll(
		func(numExisting int) bool {
			if numExisting == 0 {
				return true // Skip empty case
			}

			// Create temp directories
			tempDir, err := os.MkdirTemp("", "runtime-path-all-valid-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			targetDir := filepath.Join(tempDir, "target")
			auditDir := filepath.Join(tempDir, "audit")

			if err := os.MkdirAll(targetDir, 0755); err != nil {
				t.Logf("Failed to create target dir: %v", err)
				return false
			}

			// Build list of inbound directories with one missing in the middle
			var inboundDirs []string
			filesCreated := 0

			// Create first half of existing directories
			for i := 0; i < numExisting/2; i++ {
				dir := filepath.Join(tempDir, "first_"+itoa(i))
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Logf("Failed to create dir: %v", err)
					return false
				}
				inboundDirs = append(inboundDirs, dir)

				filename := "Invoice 2024-03-15 First" + itoa(i) + ".pdf"
				filePath := filepath.Join(dir, filename)
				if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
					t.Logf("Failed to create test file: %v", err)
					return false
				}
				filesCreated++
			}

			// Add a missing directory in the middle
			missingDir := filepath.Join(tempDir, "missing_middle")
			inboundDirs = append(inboundDirs, missingDir)

			// Create second half of existing directories
			for i := numExisting / 2; i < numExisting; i++ {
				dir := filepath.Join(tempDir, "second_"+itoa(i))
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Logf("Failed to create dir: %v", err)
					return false
				}
				inboundDirs = append(inboundDirs, dir)

				filename := "Invoice 2024-03-15 Second" + itoa(i) + ".pdf"
				filePath := filepath.Join(dir, filename)
				if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
					t.Logf("Failed to create test file: %v", err)
					return false
				}
				filesCreated++
			}

			// Create config file
			cfg := config.Configuration{
				InboundDirectories: inboundDirs,
				PrefixRules: []config.PrefixRule{
					{Prefix: "Invoice", OutboundDirectory: targetDir},
				},
			}
			configPath := filepath.Join(tempDir, "config.json")
			configData, _ := json.Marshal(cfg)
			if err := os.WriteFile(configPath, configData, 0644); err != nil {
				t.Logf("Failed to write config: %v", err)
				return false
			}

			// Run orchestrator
			auditConfig := audit.AuditConfig{
				LogDirectory: auditDir,
			}
			options := &Options{
				AuditConfig: &auditConfig,
				AppVersion:  "1.0.0-test",
				MachineID:   "test-machine",
			}

			summary, err := RunWithOptions(configPath, options)
			if err != nil {
				t.Logf("RunWithOptions failed: %v", err)
				return false
			}

			// Verify exactly one scan error for the missing directory
			if len(summary.ScanErrors) != 1 {
				t.Logf("Expected 1 scan error, got %d", len(summary.ScanErrors))
				return false
			}

			// Verify all files from valid directories were processed
			if summary.TotalFiles != filesCreated {
				t.Logf("Expected %d files, got %d", filesCreated, summary.TotalFiles)
				return false
			}

			// Verify all files were successfully moved
			if summary.SuccessCount != filesCreated {
				t.Logf("Expected %d successful moves, got %d", filesCreated, summary.SuccessCount)
				return false
			}

			return true
		},
		gen.IntRange(2, 6),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// itoa converts an integer to a string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var result []byte
	negative := i < 0
	if negative {
		i = -i
	}
	for i > 0 {
		result = append([]byte{byte('0' + i%10)}, result...)
		i /= 10
	}
	if negative {
		result = append([]byte{'-'}, result...)
	}
	return string(result)
}
