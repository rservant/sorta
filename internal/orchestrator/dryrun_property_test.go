// Package orchestrator coordinates the file organization workflow for Sorta.
package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sorta/internal/config"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: dry-run-preview, Property 1: Dry-Run Filesystem Immutability
// **Validates: Requirements 1.1, 1.4, 1.5, 2.6**
//
// For any configuration and set of files in inbound directories, running
// `sorta run --dry-run` SHALL NOT modify any files, create any directories,
// or write to the audit log. The filesystem state after dry-run SHALL be
// identical to the state before.

// FileSnapshot represents the state of a file for comparison.
type FileSnapshot struct {
	Path    string
	Size    int64
	Content []byte
}

// DirectorySnapshot represents the state of a directory tree for comparison.
type DirectorySnapshot struct {
	Files       []FileSnapshot
	Directories []string
}

// captureDirectorySnapshot captures the current state of a directory tree.
func captureDirectorySnapshot(rootDir string) (*DirectorySnapshot, error) {
	snapshot := &DirectorySnapshot{
		Files:       make([]FileSnapshot, 0),
		Directories: make([]string, 0),
	}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(rootDir, path)

		if info.IsDir() {
			if relPath != "." {
				snapshot.Directories = append(snapshot.Directories, relPath)
			}
		} else {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			snapshot.Files = append(snapshot.Files, FileSnapshot{
				Path:    relPath,
				Size:    info.Size(),
				Content: content,
			})
		}
		return nil
	})

	// Sort for consistent comparison
	sort.Strings(snapshot.Directories)
	sort.Slice(snapshot.Files, func(i, j int) bool {
		return snapshot.Files[i].Path < snapshot.Files[j].Path
	})

	return snapshot, err
}

// snapshotsEqual compares two directory snapshots for equality.
func snapshotsEqual(before, after *DirectorySnapshot) bool {
	// Compare directories
	if !reflect.DeepEqual(before.Directories, after.Directories) {
		return false
	}

	// Compare files
	if len(before.Files) != len(after.Files) {
		return false
	}

	for i := range before.Files {
		if before.Files[i].Path != after.Files[i].Path {
			return false
		}
		if before.Files[i].Size != after.Files[i].Size {
			return false
		}
		if !reflect.DeepEqual(before.Files[i].Content, after.Files[i].Content) {
			return false
		}
	}

	return true
}

// genValidPrefix generates a valid prefix string (alphabetic, 3-10 chars).
func genValidPrefix() gopter.Gen {
	return gen.IntRange(3, 10).FlatMap(func(length interface{}) gopter.Gen {
		return gen.SliceOfN(length.(int), gen.AlphaUpperChar())
	}, reflect.TypeOf([]rune{})).Map(func(chars []rune) string {
		return string(chars)
	})
}

// genValidDate generates a valid date string in YYYY-MM-DD format.
func genValidDate() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(2020, 2025), // year
		gen.IntRange(1, 12),      // month
		gen.IntRange(1, 28),      // day (use 28 to avoid invalid dates)
	).Map(func(vals []interface{}) string {
		year := vals[0].(int)
		month := vals[1].(int)
		day := vals[2].(int)
		return itoa(year) + "-" + padZero(month) + "-" + padZero(day)
	})
}

// padZero pads a number with a leading zero if needed.
func padZero(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

// genFileSuffix generates a valid file suffix (alphabetic, 3-15 chars).
func genFileSuffix() gopter.Gen {
	return gen.IntRange(3, 15).FlatMap(func(length interface{}) gopter.Gen {
		return gen.SliceOfN(length.(int), gen.AlphaChar())
	}, reflect.TypeOf([]rune{})).Map(func(chars []rune) string {
		return string(chars)
	})
}

// genFileExtension generates a common file extension.
func genFileExtension() gopter.Gen {
	return gen.OneConstOf(".pdf", ".txt", ".doc", ".xlsx", ".png")
}

// TestFileInfo holds generated test file information.
type TestFileInfo struct {
	Prefix    string
	Date      string
	Suffix    string
	Extension string
	Content   string
}

// Filename returns the full filename for this test file.
func (t TestFileInfo) Filename() string {
	return t.Prefix + " " + t.Date + " " + t.Suffix + t.Extension
}

// genTestFileInfo generates a valid test file info.
func genTestFileInfo() gopter.Gen {
	return gopter.CombineGens(
		genValidPrefix(),
		genValidDate(),
		genFileSuffix(),
		genFileExtension(),
		gen.AlphaString().Map(func(s string) string {
			if len(s) == 0 {
				return "content"
			}
			return s
		}),
	).Map(func(vals []interface{}) TestFileInfo {
		return TestFileInfo{
			Prefix:    vals[0].(string),
			Date:      vals[1].(string),
			Suffix:    vals[2].(string),
			Extension: vals[3].(string),
			Content:   vals[4].(string),
		}
	})
}

// genUnmatchedFileInfo generates a file that won't match any prefix rules.
func genUnmatchedFileInfo() gopter.Gen {
	return gopter.CombineGens(
		genFileSuffix(),
		genFileExtension(),
		gen.AlphaString().Map(func(s string) string {
			if len(s) == 0 {
				return "content"
			}
			return s
		}),
	).Map(func(vals []interface{}) TestFileInfo {
		return TestFileInfo{
			Prefix:    "", // No prefix - will be unmatched
			Date:      "",
			Suffix:    vals[0].(string),
			Extension: vals[1].(string),
			Content:   vals[2].(string),
		}
	})
}

// TestDryRunFilesystemImmutability tests Property 1: Dry-Run Filesystem Immutability
// This property verifies that dry-run mode NEVER modifies the filesystem:
// - No files are created, moved, or deleted
// - No directories are created
// - No audit log entries are written
//
// This is a universal invariant that must hold for ANY configuration and file set.
// **Validates: Requirements 1.1, 1.4, 1.5, 2.6**
func TestDryRunFilesystemImmutability(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Dry-run mode never modifies filesystem state", prop.ForAll(
		func(numMatchingFiles int, numUnmatchedFiles int, numPrefixRules int) bool {
			// Ensure we have at least some files and rules
			if numMatchingFiles == 0 && numUnmatchedFiles == 0 {
				return true // Skip empty case
			}
			if numPrefixRules == 0 {
				numPrefixRules = 1 // Need at least one rule for valid config
			}

			// Create temp directories
			tempDir, err := os.MkdirTemp("", "dryrun-immutability-*")
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
			if err := os.MkdirAll(auditDir, 0755); err != nil {
				t.Logf("Failed to create audit dir: %v", err)
				return false
			}

			// Generate prefix rules
			prefixes := make([]string, numPrefixRules)
			prefixRules := make([]config.PrefixRule, numPrefixRules)
			for i := 0; i < numPrefixRules; i++ {
				// Use deterministic prefixes to ensure matching files can be created
				prefixes[i] = "PREFIX" + itoa(i)
				prefixRules[i] = config.PrefixRule{
					Prefix:            prefixes[i],
					OutboundDirectory: targetDir,
				}
			}

			// Create matching files (files that will match prefix rules)
			for i := 0; i < numMatchingFiles; i++ {
				prefix := prefixes[i%numPrefixRules]
				filename := prefix + " 2024-03-15 TestFile" + itoa(i) + ".pdf"
				filePath := filepath.Join(sourceDir, filename)
				content := []byte("matching file content " + itoa(i))
				if err := os.WriteFile(filePath, content, 0644); err != nil {
					t.Logf("Failed to create matching file: %v", err)
					return false
				}
			}

			// Create unmatched files (files that won't match any prefix)
			for i := 0; i < numUnmatchedFiles; i++ {
				filename := "UnmatchedFile" + itoa(i) + ".pdf"
				filePath := filepath.Join(sourceDir, filename)
				content := []byte("unmatched file content " + itoa(i))
				if err := os.WriteFile(filePath, content, 0644); err != nil {
					t.Logf("Failed to create unmatched file: %v", err)
					return false
				}
			}

			// Create config file
			cfg := config.Configuration{
				InboundDirectories: []string{sourceDir},
				PrefixRules:        prefixRules,
			}
			configPath := filepath.Join(tempDir, "config.json")
			configData, _ := json.Marshal(cfg)
			if err := os.WriteFile(configPath, configData, 0644); err != nil {
				t.Logf("Failed to write config: %v", err)
				return false
			}

			// Capture filesystem state BEFORE dry-run
			sourceBefore, err := captureDirectorySnapshot(sourceDir)
			if err != nil {
				t.Logf("Failed to capture source snapshot before: %v", err)
				return false
			}
			targetBefore, err := captureDirectorySnapshot(targetDir)
			if err != nil {
				t.Logf("Failed to capture target snapshot before: %v", err)
				return false
			}
			auditBefore, err := captureDirectorySnapshot(auditDir)
			if err != nil {
				t.Logf("Failed to capture audit snapshot before: %v", err)
				return false
			}

			// Run in dry-run mode
			opts := RunOptions{DryRun: true}
			_, err = RunDryRun(configPath, opts)
			if err != nil {
				t.Logf("RunDryRun failed: %v", err)
				return false
			}

			// Capture filesystem state AFTER dry-run
			sourceAfter, err := captureDirectorySnapshot(sourceDir)
			if err != nil {
				t.Logf("Failed to capture source snapshot after: %v", err)
				return false
			}
			targetAfter, err := captureDirectorySnapshot(targetDir)
			if err != nil {
				t.Logf("Failed to capture target snapshot after: %v", err)
				return false
			}
			auditAfter, err := captureDirectorySnapshot(auditDir)
			if err != nil {
				t.Logf("Failed to capture audit snapshot after: %v", err)
				return false
			}

			// PROPERTY: Source directory must be unchanged
			// Requirements 1.1, 2.6 - dry-run shall not modify filesystem
			if !snapshotsEqual(sourceBefore, sourceAfter) {
				t.Logf("Source directory was modified during dry-run!")
				t.Logf("Before: %d files, %d dirs", len(sourceBefore.Files), len(sourceBefore.Directories))
				t.Logf("After: %d files, %d dirs", len(sourceAfter.Files), len(sourceAfter.Directories))
				return false
			}

			// PROPERTY: Target directory must be unchanged (no files moved, no dirs created)
			// Requirements 1.4 - dry-run shall not create any directories
			if !snapshotsEqual(targetBefore, targetAfter) {
				t.Logf("Target directory was modified during dry-run!")
				t.Logf("Before: %d files, %d dirs", len(targetBefore.Files), len(targetBefore.Directories))
				t.Logf("After: %d files, %d dirs", len(targetAfter.Files), len(targetAfter.Directories))
				return false
			}

			// PROPERTY: Audit directory must be unchanged (no audit log entries)
			// Requirements 1.5 - dry-run shall not write to the audit log
			if !snapshotsEqual(auditBefore, auditAfter) {
				t.Logf("Audit directory was modified during dry-run!")
				t.Logf("Before: %d files, %d dirs", len(auditBefore.Files), len(auditBefore.Directories))
				t.Logf("After: %d files, %d dirs", len(auditAfter.Files), len(auditAfter.Directories))
				return false
			}

			return true
		},
		gen.IntRange(0, 10), // numMatchingFiles
		gen.IntRange(0, 5),  // numUnmatchedFiles
		gen.IntRange(1, 5),  // numPrefixRules
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestDryRunFilesystemImmutabilityWithMultipleInboundDirs tests the immutability
// property with multiple inbound directories.
// **Validates: Requirements 1.1, 1.4, 1.5, 2.6**
func TestDryRunFilesystemImmutabilityWithMultipleInboundDirs(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Dry-run with multiple inbound dirs never modifies filesystem", prop.ForAll(
		func(numInboundDirs int, filesPerDir int) bool {
			if numInboundDirs == 0 || filesPerDir == 0 {
				return true // Skip empty case
			}

			// Create temp directories
			tempDir, err := os.MkdirTemp("", "dryrun-multi-inbound-*")
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
			if err := os.MkdirAll(auditDir, 0755); err != nil {
				t.Logf("Failed to create audit dir: %v", err)
				return false
			}

			// Create multiple inbound directories with files
			inboundDirs := make([]string, numInboundDirs)
			for i := 0; i < numInboundDirs; i++ {
				inboundDir := filepath.Join(tempDir, "inbound"+itoa(i))
				if err := os.MkdirAll(inboundDir, 0755); err != nil {
					t.Logf("Failed to create inbound dir: %v", err)
					return false
				}
				inboundDirs[i] = inboundDir

				// Create files in this inbound directory
				for j := 0; j < filesPerDir; j++ {
					filename := "Invoice 2024-03-15 Doc" + itoa(i) + "_" + itoa(j) + ".pdf"
					filePath := filepath.Join(inboundDir, filename)
					content := []byte("content for dir " + itoa(i) + " file " + itoa(j))
					if err := os.WriteFile(filePath, content, 0644); err != nil {
						t.Logf("Failed to create file: %v", err)
						return false
					}
				}
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

			// Capture snapshots of all directories BEFORE dry-run
			inboundSnapshots := make([]*DirectorySnapshot, numInboundDirs)
			for i, dir := range inboundDirs {
				snapshot, err := captureDirectorySnapshot(dir)
				if err != nil {
					t.Logf("Failed to capture inbound snapshot: %v", err)
					return false
				}
				inboundSnapshots[i] = snapshot
			}
			targetBefore, err := captureDirectorySnapshot(targetDir)
			if err != nil {
				t.Logf("Failed to capture target snapshot: %v", err)
				return false
			}
			auditBefore, err := captureDirectorySnapshot(auditDir)
			if err != nil {
				t.Logf("Failed to capture audit snapshot: %v", err)
				return false
			}

			// Run in dry-run mode
			opts := RunOptions{DryRun: true}
			_, err = RunDryRun(configPath, opts)
			if err != nil {
				t.Logf("RunDryRun failed: %v", err)
				return false
			}

			// Verify all inbound directories are unchanged
			for i, dir := range inboundDirs {
				snapshotAfter, err := captureDirectorySnapshot(dir)
				if err != nil {
					t.Logf("Failed to capture inbound snapshot after: %v", err)
					return false
				}
				if !snapshotsEqual(inboundSnapshots[i], snapshotAfter) {
					t.Logf("Inbound directory %d was modified during dry-run!", i)
					return false
				}
			}

			// Verify target directory is unchanged
			targetAfter, err := captureDirectorySnapshot(targetDir)
			if err != nil {
				t.Logf("Failed to capture target snapshot after: %v", err)
				return false
			}
			if !snapshotsEqual(targetBefore, targetAfter) {
				t.Logf("Target directory was modified during dry-run!")
				return false
			}

			// Verify audit directory is unchanged
			auditAfter, err := captureDirectorySnapshot(auditDir)
			if err != nil {
				t.Logf("Failed to capture audit snapshot after: %v", err)
				return false
			}
			if !snapshotsEqual(auditBefore, auditAfter) {
				t.Logf("Audit directory was modified during dry-run!")
				return false
			}

			return true
		},
		gen.IntRange(1, 4), // numInboundDirs
		gen.IntRange(1, 5), // filesPerDir
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
