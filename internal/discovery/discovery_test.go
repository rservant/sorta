package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"sorta/internal/config"
)

// Feature: config-auto-discover, Property 4: Candidate Directory Detection
// Validates: Requirements 5.1, 5.2, 5.3

// genDirName generates a valid directory name.
func genDirName() gopter.Gen {
	return gen.SliceOfN(8, gen.AlphaLowerChar()).Map(func(chars []rune) string {
		return string(chars)
	})
}

// genValidISODateForTest generates a valid ISO date string (YYYY-MM-DD).
func genValidISODateForTest() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(2000, 2030),
		gen.IntRange(1, 12),
		gen.IntRange(1, 28),
	).Map(func(vals []interface{}) string {
		year := vals[0].(int)
		month := vals[1].(int)
		day := vals[2].(int)
		return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	})
}

// genValidPrefixForTest generates a valid prefix (starts with letter, followed by alphanumeric).
func genValidPrefixForTest() gopter.Gen {
	return gen.SliceOfN(6, gen.AlphaChar()).Map(func(chars []rune) string {
		return string(chars)
	})
}

func TestCandidateDirectoryDetection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: scanTargetCandidates returns only immediate child directories
	properties.Property("scanTargetCandidates returns only immediate subdirectories", prop.ForAll(
		func(dirNames []string) bool {
			if len(dirNames) == 0 {
				return true
			}

			// Create temp directory structure
			scanDir, err := os.MkdirTemp("", "sorta-scan-*")
			if err != nil {
				t.Logf("Failed to create scan dir: %v", err)
				return false
			}
			defer os.RemoveAll(scanDir)

			// Create immediate subdirectories
			expectedDirs := make(map[string]bool)
			for _, name := range dirNames {
				dirPath := filepath.Join(scanDir, name)
				if err := os.MkdirAll(dirPath, 0755); err != nil {
					continue
				}
				expectedDirs[dirPath] = true

				// Create nested subdirectory (should NOT be returned)
				nestedPath := filepath.Join(dirPath, "nested")
				os.MkdirAll(nestedPath, 0755)
			}

			// Scan for candidates
			candidates, err := scanTargetCandidates(scanDir)
			if err != nil {
				t.Logf("scanTargetCandidates failed: %v", err)
				return false
			}

			// Verify only immediate subdirectories are returned
			for _, candidate := range candidates {
				// Check it's an immediate child (parent is scanDir)
				if filepath.Dir(candidate) != scanDir {
					t.Logf("Non-immediate directory returned: %s", candidate)
					return false
				}
			}

			return true
		},
		gen.SliceOfN(5, genDirName()),
	))

	// Property: Discover includes directory only if it contains files matching prefix pattern
	properties.Property("Discover includes directory if and only if it contains matching files", prop.ForAll(
		func(prefix string, date string, hasMatchingFile bool) bool {
			// Create temp directory structure
			scanDir, err := os.MkdirTemp("", "sorta-scan-*")
			if err != nil {
				t.Logf("Failed to create scan dir: %v", err)
				return false
			}
			defer os.RemoveAll(scanDir)

			// Create a candidate directory
			candidateDir := filepath.Join(scanDir, "candidate")
			if err := os.MkdirAll(candidateDir, 0755); err != nil {
				t.Logf("Failed to create candidate dir: %v", err)
				return false
			}

			if hasMatchingFile {
				// Create a file matching the prefix pattern
				filename := prefix + " " + date + " document.pdf"
				filePath := filepath.Join(candidateDir, filename)
				if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
					t.Logf("Failed to create matching file: %v", err)
					return false
				}
			} else {
				// Create a file that doesn't match the pattern
				filePath := filepath.Join(candidateDir, "random-file.txt")
				if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
					t.Logf("Failed to create non-matching file: %v", err)
					return false
				}
			}

			// Run discovery
			result, err := Discover(scanDir, nil)
			if err != nil {
				t.Logf("Discover failed: %v", err)
				return false
			}

			// Check if rules were discovered
			hasRules := len(result.NewRules) > 0

			// Should have rules if and only if there was a matching file
			return hasRules == hasMatchingFile
		},
		genValidPrefixForTest(),
		genValidISODateForTest(),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// Feature: config-auto-discover, Property 5: Recursive File Analysis
// Validates: Requirements 6.1

func TestRecursiveFileAnalysis(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: analyzeDirectory examines files at all depths
	properties.Property("analyzeDirectory finds prefixes at all directory depths", prop.ForAll(
		func(prefix string, date string, depth int) bool {
			// Create temp directory structure
			baseDir, err := os.MkdirTemp("", "sorta-analyze-*")
			if err != nil {
				t.Logf("Failed to create base dir: %v", err)
				return false
			}
			defer os.RemoveAll(baseDir)

			// Create nested directory structure
			currentDir := baseDir
			for i := 0; i < depth; i++ {
				currentDir = filepath.Join(currentDir, fmt.Sprintf("level%d", i))
				if err := os.MkdirAll(currentDir, 0755); err != nil {
					t.Logf("Failed to create nested dir: %v", err)
					return false
				}
			}

			// Create a file with matching pattern at the deepest level
			filename := prefix + " " + date + " document.pdf"
			filePath := filepath.Join(currentDir, filename)
			if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
				t.Logf("Failed to create file: %v", err)
				return false
			}

			// Analyze the base directory
			prefixes, err := analyzeDirectory(baseDir)
			if err != nil {
				t.Logf("analyzeDirectory failed: %v", err)
				return false
			}

			// The prefix should be found regardless of depth
			found := false
			for _, p := range prefixes {
				if p == prefix {
					found = true
					break
				}
			}

			if !found {
				t.Logf("Prefix %q not found at depth %d", prefix, depth)
				return false
			}

			return true
		},
		genValidPrefixForTest(),
		genValidISODateForTest(),
		gen.IntRange(0, 5), // depth from 0 to 5 levels
	))

	// Property: analyzeDirectory returns unique prefixes
	properties.Property("analyzeDirectory returns unique prefixes only", prop.ForAll(
		func(prefix string, date string, fileCount int) bool {
			// Create temp directory
			baseDir, err := os.MkdirTemp("", "sorta-analyze-*")
			if err != nil {
				t.Logf("Failed to create base dir: %v", err)
				return false
			}
			defer os.RemoveAll(baseDir)

			// Create multiple files with the same prefix
			for i := 0; i < fileCount; i++ {
				filename := prefix + " " + date + fmt.Sprintf(" document%d.pdf", i)
				filePath := filepath.Join(baseDir, filename)
				if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
					t.Logf("Failed to create file: %v", err)
					return false
				}
			}

			// Analyze the directory
			prefixes, err := analyzeDirectory(baseDir)
			if err != nil {
				t.Logf("analyzeDirectory failed: %v", err)
				return false
			}

			// Count occurrences of the prefix
			count := 0
			for _, p := range prefixes {
				if p == prefix {
					count++
				}
			}

			// Should appear exactly once (unique)
			if count != 1 {
				t.Logf("Prefix %q appeared %d times, expected 1", prefix, count)
				return false
			}

			return true
		},
		genValidPrefixForTest(),
		genValidISODateForTest(),
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// Test for duplicate prefix filtering in Discover
func TestDiscoverDuplicateFiltering(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Discover skips prefixes that already exist in config (case-insensitive)
	properties.Property("Discover skips existing prefixes case-insensitively", prop.ForAll(
		func(prefix string, date string) bool {
			// Create temp directory structure
			scanDir, err := os.MkdirTemp("", "sorta-scan-*")
			if err != nil {
				t.Logf("Failed to create scan dir: %v", err)
				return false
			}
			defer os.RemoveAll(scanDir)

			// Create a candidate directory with a matching file
			candidateDir := filepath.Join(scanDir, "candidate")
			if err := os.MkdirAll(candidateDir, 0755); err != nil {
				t.Logf("Failed to create candidate dir: %v", err)
				return false
			}

			filename := prefix + " " + date + " document.pdf"
			filePath := filepath.Join(candidateDir, filename)
			if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
				t.Logf("Failed to create file: %v", err)
				return false
			}

			// Create config with the prefix already present (different case)
			existingConfig := &config.Configuration{
				PrefixRules: []config.PrefixRule{
					{Prefix: toggleCase(prefix), OutboundDirectory: "/some/other/dir"},
				},
			}

			// Run discovery
			result, err := Discover(scanDir, existingConfig)
			if err != nil {
				t.Logf("Discover failed: %v", err)
				return false
			}

			// The prefix should be in skipped rules, not new rules
			if len(result.NewRules) > 0 {
				t.Logf("Expected no new rules, got %d", len(result.NewRules))
				return false
			}

			if len(result.SkippedRules) != 1 {
				t.Logf("Expected 1 skipped rule, got %d", len(result.SkippedRules))
				return false
			}

			return true
		},
		genValidPrefixForTest(),
		genValidISODateForTest(),
	))

	properties.TestingRun(t)
}

// toggleCase toggles the case of the first character of a string.
func toggleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	first := rune(s[0])
	if first >= 'a' && first <= 'z' {
		first = first - 'a' + 'A'
	} else if first >= 'A' && first <= 'Z' {
		first = first - 'A' + 'a'
	}
	return string(first) + s[1:]
}

// Feature: discovery-directory-filtering, Property 2: ISO-Date Directory Scanning
// ISO-date directories are SKIPPED during scanning (Requirement 1.5 from discovery-enhancements).

func TestISODateDirectoryScanning(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Files in ISO-date subdirectories are NOT analyzed (directories are skipped)
	// Validates: Requirement 1.5 - ISO-date directories are skipped regardless of depth setting
	properties.Property("Files in ISO-date subdirectories are skipped", prop.ForAll(
		func(prefix string, date string, isoDateDir string) bool {
			// Create temp directory structure
			baseDir, err := os.MkdirTemp("", "sorta-isofilter-*")
			if err != nil {
				t.Logf("Failed to create base dir: %v", err)
				return false
			}
			defer os.RemoveAll(baseDir)

			// Create an ISO-date subdirectory
			isoDir := filepath.Join(baseDir, isoDateDir)
			if err := os.MkdirAll(isoDir, 0755); err != nil {
				t.Logf("Failed to create ISO date dir: %v", err)
				return false
			}

			// Create a file with matching pattern inside the ISO-date directory
			filename := prefix + " " + date + " document.pdf"
			filePath := filepath.Join(isoDir, filename)
			if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
				t.Logf("Failed to create file: %v", err)
				return false
			}

			// Analyze the base directory
			prefixes, err := analyzeDirectory(baseDir)
			if err != nil {
				t.Logf("analyzeDirectory failed: %v", err)
				return false
			}

			// The prefix should NOT be found because ISO-date directories are skipped
			found := false
			for _, p := range prefixes {
				if p == prefix {
					found = true
					break
				}
			}

			if found {
				t.Logf("Prefix %q was found but should have been skipped (in ISO-date dir %q)", prefix, isoDateDir)
				return false
			}

			return true
		},
		genValidPrefixForTest(),
		genValidISODateForTest(),
		genValidISODateDirNameForTest(),
	))

	// Property: Files in non-ISO-date subdirectories ARE analyzed
	properties.Property("Files in non-ISO-date subdirectories are analyzed", prop.ForAll(
		func(prefix string, date string, nonIsoDir string) bool {
			// Create temp directory structure
			baseDir, err := os.MkdirTemp("", "sorta-nonisofilter-*")
			if err != nil {
				t.Logf("Failed to create base dir: %v", err)
				return false
			}
			defer os.RemoveAll(baseDir)

			// Create a non-ISO-date subdirectory
			subDir := filepath.Join(baseDir, nonIsoDir)
			if err := os.MkdirAll(subDir, 0755); err != nil {
				t.Logf("Failed to create non-ISO date dir: %v", err)
				return false
			}

			// Create a file with matching pattern inside the non-ISO-date directory
			filename := prefix + " " + date + " document.pdf"
			filePath := filepath.Join(subDir, filename)
			if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
				t.Logf("Failed to create file: %v", err)
				return false
			}

			// Analyze the base directory
			prefixes, err := analyzeDirectory(baseDir)
			if err != nil {
				t.Logf("analyzeDirectory failed: %v", err)
				return false
			}

			// The prefix SHOULD be found because the file is in a non-ISO-date directory
			found := false
			for _, p := range prefixes {
				if p == prefix {
					found = true
					break
				}
			}

			if !found {
				t.Logf("Prefix %q was not found but should have been (in non-ISO-date dir %q)", prefix, nonIsoDir)
				return false
			}

			return true
		},
		genValidPrefixForTest(),
		genValidISODateForTest(),
		genNonISODateDirNameForTest(),
	))

	// Property: Files in root of target directory are always analyzed
	properties.Property("Files in root directory are always analyzed", prop.ForAll(
		func(prefix string, date string) bool {
			// Create temp directory structure
			baseDir, err := os.MkdirTemp("", "sorta-rootfile-*")
			if err != nil {
				t.Logf("Failed to create base dir: %v", err)
				return false
			}
			defer os.RemoveAll(baseDir)

			// Create a file with matching pattern in the root directory
			filename := prefix + " " + date + " document.pdf"
			filePath := filepath.Join(baseDir, filename)
			if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
				t.Logf("Failed to create file: %v", err)
				return false
			}

			// Analyze the base directory
			prefixes, err := analyzeDirectory(baseDir)
			if err != nil {
				t.Logf("analyzeDirectory failed: %v", err)
				return false
			}

			// The prefix SHOULD be found because the file is in the root
			found := false
			for _, p := range prefixes {
				if p == prefix {
					found = true
					break
				}
			}

			if !found {
				t.Logf("Prefix %q was not found in root directory", prefix)
				return false
			}

			return true
		},
		genValidPrefixForTest(),
		genValidISODateForTest(),
	))

	properties.TestingRun(t)
}

// genValidISODateDirNameForTest generates directory names that start with a valid ISO date.
func genValidISODateDirNameForTest() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(2000, 2030),
		gen.IntRange(1, 12),
		gen.IntRange(1, 28),
		gen.OneConstOf("", " Documents", " Archive"),
	).Map(func(vals []interface{}) string {
		year := vals[0].(int)
		month := vals[1].(int)
		day := vals[2].(int)
		suffix := vals[3].(string)
		return fmt.Sprintf("%04d-%02d-%02d%s", year, month, day, suffix)
	})
}

// genNonISODateDirNameForTest generates directory names that do NOT start with a valid ISO date.
func genNonISODateDirNameForTest() gopter.Gen {
	return gen.SliceOfN(8, gen.AlphaLowerChar()).Map(func(chars []rune) string {
		return string(chars)
	}).SuchThat(func(s string) bool {
		// Ensure it doesn't accidentally match an ISO date pattern
		return len(s) > 0 && !IsISODateDirectory(s)
	})
}

// TestDirectoryFilteringBehavior tests specific examples of directory scanning behavior.
// Updated to reflect Requirement 1.5: ISO-date directories are SKIPPED.
func TestDirectoryFilteringBehavior(t *testing.T) {
	t.Run("files in ISO-date subdirectories are skipped", func(t *testing.T) {
		// Create temp directory structure
		baseDir, err := os.MkdirTemp("", "sorta-filter-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create an ISO-date subdirectory
		isoDir := filepath.Join(baseDir, "2024-01-15 Documents")
		if err := os.MkdirAll(isoDir, 0755); err != nil {
			t.Fatalf("Failed to create ISO date dir: %v", err)
		}

		// Create a file with matching pattern inside the ISO-date directory
		filePath := filepath.Join(isoDir, "Invoice 2024-01-15 Acme Corp.pdf")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Analyze the base directory
		prefixes, err := analyzeDirectory(baseDir)
		if err != nil {
			t.Fatalf("analyzeDirectory failed: %v", err)
		}

		// The prefix should NOT be found (ISO-date directories are skipped per Requirement 1.5)
		found := false
		for _, p := range prefixes {
			if p == "Invoice" {
				found = true
				break
			}
		}
		if found {
			t.Errorf("Prefix 'Invoice' was found but should have been skipped (ISO-date dir)")
		}
	})

	t.Run("files in non-ISO-date subdirectories are analyzed", func(t *testing.T) {
		// Create temp directory structure
		baseDir, err := os.MkdirTemp("", "sorta-filter-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create a non-ISO-date subdirectory
		subDir := filepath.Join(baseDir, "invoices")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("Failed to create subdir: %v", err)
		}

		// Create a file with matching pattern inside the subdirectory
		filePath := filepath.Join(subDir, "Invoice 2024-01-15 Acme Corp.pdf")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Analyze the base directory
		prefixes, err := analyzeDirectory(baseDir)
		if err != nil {
			t.Fatalf("analyzeDirectory failed: %v", err)
		}

		// The prefix SHOULD be found
		found := false
		for _, p := range prefixes {
			if p == "Invoice" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Prefix 'Invoice' was not found but should have been")
		}
	})

	t.Run("nested directory structures with mixed ISO and non-ISO dirs", func(t *testing.T) {
		// Create temp directory structure
		baseDir, err := os.MkdirTemp("", "sorta-filter-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create structure:
		// baseDir/
		//   invoices/                    <- non-ISO, should be scanned
		//     Invoice 2024-01-15 A.pdf   <- should be found
		//     2024-02-01 Archive/        <- ISO, should be SKIPPED
		//       Receipt 2024-02-01 B.pdf <- should NOT be found (ISO dirs are skipped)

		invoicesDir := filepath.Join(baseDir, "invoices")
		if err := os.MkdirAll(invoicesDir, 0755); err != nil {
			t.Fatalf("Failed to create invoices dir: %v", err)
		}

		// File in non-ISO directory (should be found)
		filePath1 := filepath.Join(invoicesDir, "Invoice 2024-01-15 A.pdf")
		if err := os.WriteFile(filePath1, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// ISO-date subdirectory inside invoices (should be skipped)
		archiveDir := filepath.Join(invoicesDir, "2024-02-01 Archive")
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			t.Fatalf("Failed to create archive dir: %v", err)
		}

		// File in ISO-date directory (should NOT be found - ISO dirs are skipped)
		filePath2 := filepath.Join(archiveDir, "Receipt 2024-02-01 B.pdf")
		if err := os.WriteFile(filePath2, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Analyze the base directory
		prefixes, err := analyzeDirectory(baseDir)
		if err != nil {
			t.Fatalf("analyzeDirectory failed: %v", err)
		}

		// Invoice should be found, Receipt should NOT be found
		foundInvoice := false
		foundReceipt := false
		for _, p := range prefixes {
			if p == "Invoice" {
				foundInvoice = true
			}
			if p == "Receipt" {
				foundReceipt = true
			}
		}

		if !foundInvoice {
			t.Errorf("Prefix 'Invoice' was not found but should have been")
		}
		if foundReceipt {
			t.Errorf("Prefix 'Receipt' was found but should have been skipped (ISO-date dir)")
		}
	})

	t.Run("deeply nested ISO-date directories are skipped", func(t *testing.T) {
		// Create temp directory structure
		baseDir, err := os.MkdirTemp("", "sorta-filter-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create structure:
		// baseDir/
		//   level1/
		//     level2/
		//       2024-03-15 Deep Archive/  <- ISO, should be SKIPPED
		//         Statement 2024-03-15 X.pdf <- should NOT be found

		deepDir := filepath.Join(baseDir, "level1", "level2", "2024-03-15 Deep Archive")
		if err := os.MkdirAll(deepDir, 0755); err != nil {
			t.Fatalf("Failed to create deep dir: %v", err)
		}

		filePath := filepath.Join(deepDir, "Statement 2024-03-15 X.pdf")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Analyze the base directory
		prefixes, err := analyzeDirectory(baseDir)
		if err != nil {
			t.Fatalf("analyzeDirectory failed: %v", err)
		}

		// Statement should NOT be found (ISO-date directories are skipped)
		found := false
		for _, p := range prefixes {
			if p == "Statement" {
				found = true
				break
			}
		}
		if found {
			t.Errorf("Prefix 'Statement' was found but should have been skipped (ISO-date dir)")
		}
	})

	t.Run("files in root of target candidate are analyzed", func(t *testing.T) {
		// Create temp directory structure
		baseDir, err := os.MkdirTemp("", "sorta-filter-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create a file directly in the root
		filePath := filepath.Join(baseDir, "Invoice 2024-01-15 Root File.pdf")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Analyze the base directory
		prefixes, err := analyzeDirectory(baseDir)
		if err != nil {
			t.Fatalf("analyzeDirectory failed: %v", err)
		}

		// Invoice should be found
		found := false
		for _, p := range prefixes {
			if p == "Invoice" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Prefix 'Invoice' was not found in root directory")
		}
	})
}

// Feature: discovery-directory-filtering, Property 1: File-Only Prefix Extraction
// Validates: Requirements 1.1, 1.2, 1.3

// TestFileOnlyPrefixExtraction tests that prefixes are extracted only from files, not directories.
func TestFileOnlyPrefixExtraction(t *testing.T) {
	t.Run("directory named with prefix pattern does NOT produce prefix", func(t *testing.T) {
		// Create temp directory structure
		baseDir, err := os.MkdirTemp("", "sorta-fileonly-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create a directory named with the prefix pattern (should NOT produce prefix)
		dirPath := filepath.Join(baseDir, "Invoice 2024-01-15 Vendor")
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Analyze the base directory
		prefixes, err := analyzeDirectory(baseDir)
		if err != nil {
			t.Fatalf("analyzeDirectory failed: %v", err)
		}

		// "Invoice" prefix should NOT be found (directory names are ignored)
		for _, p := range prefixes {
			if p == "Invoice" {
				t.Errorf("Prefix 'Invoice' was extracted from directory name but should have been ignored")
			}
		}
	})

	t.Run("file named with prefix pattern DOES produce prefix", func(t *testing.T) {
		// Create temp directory structure
		baseDir, err := os.MkdirTemp("", "sorta-fileonly-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create a file named with the prefix pattern (should produce prefix)
		filePath := filepath.Join(baseDir, "Invoice 2024-01-15 Vendor.pdf")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Analyze the base directory
		prefixes, err := analyzeDirectory(baseDir)
		if err != nil {
			t.Fatalf("analyzeDirectory failed: %v", err)
		}

		// "Invoice" prefix SHOULD be found (from file)
		found := false
		for _, p := range prefixes {
			if p == "Invoice" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Prefix 'Invoice' was not extracted from file but should have been")
		}
	})

	t.Run("same pattern in directory and file - only file produces prefix", func(t *testing.T) {
		// Create temp directory structure
		baseDir, err := os.MkdirTemp("", "sorta-fileonly-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create a directory named with the prefix pattern
		dirPath := filepath.Join(baseDir, "Receipt 2024-03-20 Store")
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Create a file with a DIFFERENT prefix pattern
		filePath := filepath.Join(baseDir, "Statement 2024-02-15 Bank.pdf")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Analyze the base directory
		prefixes, err := analyzeDirectory(baseDir)
		if err != nil {
			t.Fatalf("analyzeDirectory failed: %v", err)
		}

		// "Receipt" prefix should NOT be found (from directory)
		// "Statement" prefix SHOULD be found (from file)
		foundReceipt := false
		foundStatement := false
		for _, p := range prefixes {
			if p == "Receipt" {
				foundReceipt = true
			}
			if p == "Statement" {
				foundStatement = true
			}
		}

		if foundReceipt {
			t.Errorf("Prefix 'Receipt' was extracted from directory name but should have been ignored")
		}
		if !foundStatement {
			t.Errorf("Prefix 'Statement' was not extracted from file but should have been")
		}
	})

	t.Run("nested directory with prefix pattern does NOT produce prefix", func(t *testing.T) {
		// Create temp directory structure
		baseDir, err := os.MkdirTemp("", "sorta-fileonly-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create nested directories with prefix patterns
		nestedDir := filepath.Join(baseDir, "documents", "Invoice 2024-01-15 Vendor")
		if err := os.MkdirAll(nestedDir, 0755); err != nil {
			t.Fatalf("Failed to create nested directory: %v", err)
		}

		// Create a file with a different prefix in the nested directory
		filePath := filepath.Join(nestedDir, "Receipt 2024-01-20 Shop.pdf")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Analyze the base directory
		prefixes, err := analyzeDirectory(baseDir)
		if err != nil {
			t.Fatalf("analyzeDirectory failed: %v", err)
		}

		// "Invoice" prefix should NOT be found (from directory name)
		// "Receipt" prefix SHOULD be found (from file)
		foundInvoice := false
		foundReceipt := false
		for _, p := range prefixes {
			if p == "Invoice" {
				foundInvoice = true
			}
			if p == "Receipt" {
				foundReceipt = true
			}
		}

		if foundInvoice {
			t.Errorf("Prefix 'Invoice' was extracted from nested directory name but should have been ignored")
		}
		if !foundReceipt {
			t.Errorf("Prefix 'Receipt' was not extracted from file but should have been")
		}
	})

	t.Run("multiple directories with prefix patterns produce no prefixes", func(t *testing.T) {
		// Create temp directory structure
		baseDir, err := os.MkdirTemp("", "sorta-fileonly-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create multiple directories with prefix patterns
		dirs := []string{
			"Invoice 2024-01-15 Vendor A",
			"Receipt 2024-02-20 Store B",
			"Statement 2024-03-25 Bank C",
		}
		for _, dir := range dirs {
			dirPath := filepath.Join(baseDir, dir)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}

		// Analyze the base directory
		prefixes, err := analyzeDirectory(baseDir)
		if err != nil {
			t.Fatalf("analyzeDirectory failed: %v", err)
		}

		// No prefixes should be found (all are from directories)
		if len(prefixes) > 0 {
			t.Errorf("Expected no prefixes, but found: %v", prefixes)
		}
	})
}

// Feature: discovery-directory-filtering, Property 1: File-Only Prefix Extraction
// Validates: Requirements 1.1, 1.2, 1.3
// Property: For any directory structure containing both files and directories that match the prefix pattern,
// the discovery engine SHALL return prefixes extracted only from files.
// Directory names matching the prefix pattern SHALL NOT appear in the discovered prefixes.

func TestFileOnlyPrefixExtractionProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property 1: File-Only Prefix Extraction
	// Directories with prefix patterns do NOT produce prefixes
	properties.Property("Directories with prefix patterns do not produce prefixes", prop.ForAll(
		func(dirPrefix string, date string) bool {
			// Create temp directory structure
			baseDir, err := os.MkdirTemp("", "sorta-prop-fileonly-*")
			if err != nil {
				t.Logf("Failed to create base dir: %v", err)
				return false
			}
			defer os.RemoveAll(baseDir)

			// Create a directory named with the prefix pattern
			dirName := dirPrefix + " " + date + " Vendor"
			dirPath := filepath.Join(baseDir, dirName)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				t.Logf("Failed to create directory: %v", err)
				return false
			}

			// Analyze the base directory
			prefixes, err := analyzeDirectory(baseDir)
			if err != nil {
				t.Logf("analyzeDirectory failed: %v", err)
				return false
			}

			// The prefix should NOT be found (directory names are ignored)
			for _, p := range prefixes {
				if strings.EqualFold(p, dirPrefix) {
					t.Logf("Prefix %q was extracted from directory name but should have been ignored", dirPrefix)
					return false
				}
			}

			return true
		},
		genValidPrefixForTest(),
		genValidISODateForTest(),
	))

	// Property 1: File-Only Prefix Extraction
	// Files with prefix patterns DO produce prefixes
	properties.Property("Files with prefix patterns produce prefixes", prop.ForAll(
		func(filePrefix string, date string) bool {
			// Create temp directory structure
			baseDir, err := os.MkdirTemp("", "sorta-prop-fileonly-*")
			if err != nil {
				t.Logf("Failed to create base dir: %v", err)
				return false
			}
			defer os.RemoveAll(baseDir)

			// Create a file named with the prefix pattern
			fileName := filePrefix + " " + date + " Document.pdf"
			filePath := filepath.Join(baseDir, fileName)
			if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
				t.Logf("Failed to create file: %v", err)
				return false
			}

			// Analyze the base directory
			prefixes, err := analyzeDirectory(baseDir)
			if err != nil {
				t.Logf("analyzeDirectory failed: %v", err)
				return false
			}

			// The prefix SHOULD be found (from file)
			found := false
			for _, p := range prefixes {
				if strings.EqualFold(p, filePrefix) {
					found = true
					break
				}
			}

			if !found {
				t.Logf("Prefix %q was not extracted from file but should have been", filePrefix)
				return false
			}

			return true
		},
		genValidPrefixForTest(),
		genValidISODateForTest(),
	))

	// Property 1: File-Only Prefix Extraction
	// When both directory and file have prefix patterns, only file prefix is extracted
	properties.Property("Only file prefixes are extracted when both directory and file have patterns", prop.ForAll(
		func(dirPrefix string, filePrefix string, date string) bool {
			// Skip if prefixes are the same (case-insensitive) - we want to test distinct prefixes
			if strings.EqualFold(dirPrefix, filePrefix) {
				return true
			}

			// Create temp directory structure
			baseDir, err := os.MkdirTemp("", "sorta-prop-fileonly-*")
			if err != nil {
				t.Logf("Failed to create base dir: %v", err)
				return false
			}
			defer os.RemoveAll(baseDir)

			// Create a directory named with one prefix pattern
			dirName := dirPrefix + " " + date + " Vendor"
			dirPath := filepath.Join(baseDir, dirName)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				t.Logf("Failed to create directory: %v", err)
				return false
			}

			// Create a file named with a different prefix pattern
			fileName := filePrefix + " " + date + " Document.pdf"
			filePath := filepath.Join(baseDir, fileName)
			if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
				t.Logf("Failed to create file: %v", err)
				return false
			}

			// Analyze the base directory
			prefixes, err := analyzeDirectory(baseDir)
			if err != nil {
				t.Logf("analyzeDirectory failed: %v", err)
				return false
			}

			// Directory prefix should NOT be found
			// File prefix SHOULD be found
			foundDirPrefix := false
			foundFilePrefix := false
			for _, p := range prefixes {
				if strings.EqualFold(p, dirPrefix) {
					foundDirPrefix = true
				}
				if strings.EqualFold(p, filePrefix) {
					foundFilePrefix = true
				}
			}

			if foundDirPrefix {
				t.Logf("Directory prefix %q was extracted but should have been ignored", dirPrefix)
				return false
			}
			if !foundFilePrefix {
				t.Logf("File prefix %q was not extracted but should have been", filePrefix)
				return false
			}

			return true
		},
		genValidPrefixForTest(),
		genValidPrefixForTest(),
		genValidISODateForTest(),
	))

	properties.TestingRun(t)
}

// =============================================================================
// Feature: discovery-enhancements, Depth Limiting Unit Tests
// Validates: Requirements 1.1, 1.2, 1.3, 1.4, 1.5, 1.6
// =============================================================================

// TestDepthLimitingZero tests that depth=0 returns only files in the immediate
// target candidate directory (no subdirectories are scanned).
func TestDepthLimitingZero(t *testing.T) {
	// Create temp directory structure:
	// baseDir/
	//   RootFile 2024-01-15 Doc.pdf      <- should be found (depth 0)
	//   subdir/
	//     SubFile 2024-01-16 Doc.pdf     <- should NOT be found (depth 1)
	//     nested/
	//       DeepFile 2024-01-17 Doc.pdf  <- should NOT be found (depth 2)

	baseDir, err := os.MkdirTemp("", "sorta-depth0-*")
	if err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	// Create file at root (depth 0)
	rootFile := filepath.Join(baseDir, "RootFile 2024-01-15 Doc.pdf")
	if err := os.WriteFile(rootFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create root file: %v", err)
	}

	// Create subdirectory with file (depth 1)
	subDir := filepath.Join(baseDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	subFile := filepath.Join(subDir, "SubFile 2024-01-16 Doc.pdf")
	if err := os.WriteFile(subFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	// Create nested subdirectory with file (depth 2)
	nestedDir := filepath.Join(subDir, "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}
	deepFile := filepath.Join(nestedDir, "DeepFile 2024-01-17 Doc.pdf")
	if err := os.WriteFile(deepFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create deep file: %v", err)
	}

	// Analyze with depth=0
	prefixes, err := analyzeDirectoryWithDepth(baseDir, 0, nil, nil)
	if err != nil {
		t.Fatalf("analyzeDirectoryWithDepth failed: %v", err)
	}

	// Should only find RootFile prefix
	foundRoot := false
	foundSub := false
	foundDeep := false
	for _, p := range prefixes {
		if p == "RootFile" {
			foundRoot = true
		}
		if p == "SubFile" {
			foundSub = true
		}
		if p == "DeepFile" {
			foundDeep = true
		}
	}

	if !foundRoot {
		t.Errorf("Expected to find 'RootFile' prefix at depth 0, but didn't")
	}
	if foundSub {
		t.Errorf("Found 'SubFile' prefix at depth 0, but should not have (it's at depth 1)")
	}
	if foundDeep {
		t.Errorf("Found 'DeepFile' prefix at depth 0, but should not have (it's at depth 2)")
	}
}

// TestDepthLimitingOne tests that depth=1 includes files in the target candidate
// and its immediate subdirectories, but not deeper.
func TestDepthLimitingOne(t *testing.T) {
	// Create temp directory structure:
	// baseDir/
	//   RootFile 2024-01-15 Doc.pdf      <- should be found (depth 0)
	//   subdir/
	//     SubFile 2024-01-16 Doc.pdf     <- should be found (depth 1)
	//     nested/
	//       DeepFile 2024-01-17 Doc.pdf  <- should NOT be found (depth 2)

	baseDir, err := os.MkdirTemp("", "sorta-depth1-*")
	if err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	// Create file at root (depth 0)
	rootFile := filepath.Join(baseDir, "RootFile 2024-01-15 Doc.pdf")
	if err := os.WriteFile(rootFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create root file: %v", err)
	}

	// Create subdirectory with file (depth 1)
	subDir := filepath.Join(baseDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	subFile := filepath.Join(subDir, "SubFile 2024-01-16 Doc.pdf")
	if err := os.WriteFile(subFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	// Create nested subdirectory with file (depth 2)
	nestedDir := filepath.Join(subDir, "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}
	deepFile := filepath.Join(nestedDir, "DeepFile 2024-01-17 Doc.pdf")
	if err := os.WriteFile(deepFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create deep file: %v", err)
	}

	// Analyze with depth=1
	prefixes, err := analyzeDirectoryWithDepth(baseDir, 1, nil, nil)
	if err != nil {
		t.Fatalf("analyzeDirectoryWithDepth failed: %v", err)
	}

	// Should find RootFile and SubFile, but not DeepFile
	foundRoot := false
	foundSub := false
	foundDeep := false
	for _, p := range prefixes {
		if p == "RootFile" {
			foundRoot = true
		}
		if p == "SubFile" {
			foundSub = true
		}
		if p == "DeepFile" {
			foundDeep = true
		}
	}

	if !foundRoot {
		t.Errorf("Expected to find 'RootFile' prefix at depth 1, but didn't")
	}
	if !foundSub {
		t.Errorf("Expected to find 'SubFile' prefix at depth 1, but didn't")
	}
	if foundDeep {
		t.Errorf("Found 'DeepFile' prefix at depth 1, but should not have (it's at depth 2)")
	}
}

// TestDepthLimitingUnlimited tests that depth=-1 (unlimited) traverses all levels.
func TestDepthLimitingUnlimited(t *testing.T) {
	// Create temp directory structure:
	// baseDir/
	//   RootFile 2024-01-15 Doc.pdf      <- should be found
	//   level1/
	//     Level1File 2024-01-16 Doc.pdf  <- should be found
	//     level2/
	//       Level2File 2024-01-17 Doc.pdf <- should be found
	//       level3/
	//         Level3File 2024-01-18 Doc.pdf <- should be found

	baseDir, err := os.MkdirTemp("", "sorta-depthunlimited-*")
	if err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	// Create file at root
	rootFile := filepath.Join(baseDir, "RootFile 2024-01-15 Doc.pdf")
	if err := os.WriteFile(rootFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create root file: %v", err)
	}

	// Create level1 directory with file
	level1Dir := filepath.Join(baseDir, "level1")
	if err := os.MkdirAll(level1Dir, 0755); err != nil {
		t.Fatalf("Failed to create level1 dir: %v", err)
	}
	level1File := filepath.Join(level1Dir, "Level1File 2024-01-16 Doc.pdf")
	if err := os.WriteFile(level1File, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create level1 file: %v", err)
	}

	// Create level2 directory with file
	level2Dir := filepath.Join(level1Dir, "level2")
	if err := os.MkdirAll(level2Dir, 0755); err != nil {
		t.Fatalf("Failed to create level2 dir: %v", err)
	}
	level2File := filepath.Join(level2Dir, "Level2File 2024-01-17 Doc.pdf")
	if err := os.WriteFile(level2File, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create level2 file: %v", err)
	}

	// Create level3 directory with file
	level3Dir := filepath.Join(level2Dir, "level3")
	if err := os.MkdirAll(level3Dir, 0755); err != nil {
		t.Fatalf("Failed to create level3 dir: %v", err)
	}
	level3File := filepath.Join(level3Dir, "Level3File 2024-01-18 Doc.pdf")
	if err := os.WriteFile(level3File, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create level3 file: %v", err)
	}

	// Analyze with depth=-1 (unlimited)
	prefixes, err := analyzeDirectoryWithDepth(baseDir, -1, nil, nil)
	if err != nil {
		t.Fatalf("analyzeDirectoryWithDepth failed: %v", err)
	}

	// Should find all prefixes
	foundRoot := false
	foundLevel1 := false
	foundLevel2 := false
	foundLevel3 := false
	for _, p := range prefixes {
		if p == "RootFile" {
			foundRoot = true
		}
		if p == "Level1File" {
			foundLevel1 = true
		}
		if p == "Level2File" {
			foundLevel2 = true
		}
		if p == "Level3File" {
			foundLevel3 = true
		}
	}

	if !foundRoot {
		t.Errorf("Expected to find 'RootFile' prefix with unlimited depth, but didn't")
	}
	if !foundLevel1 {
		t.Errorf("Expected to find 'Level1File' prefix with unlimited depth, but didn't")
	}
	if !foundLevel2 {
		t.Errorf("Expected to find 'Level2File' prefix with unlimited depth, but didn't")
	}
	if !foundLevel3 {
		t.Errorf("Expected to find 'Level3File' prefix with unlimited depth, but didn't")
	}
}

// TestDepthLimitingWithDiscoverOptions tests depth limiting through DiscoverWithOptions.
func TestDepthLimitingWithDiscoverOptions(t *testing.T) {
	// Create temp directory structure:
	// scanDir/
	//   candidate/                        <- target candidate (immediate subdir of scanDir)
	//     CandidateFile 2024-01-15 Doc.pdf <- should be found at depth 0 (root of candidate)
	//     subdir/
	//       SubFile 2024-01-16 Doc.pdf    <- should NOT be found at depth 0

	scanDir, err := os.MkdirTemp("", "sorta-discover-depth-*")
	if err != nil {
		t.Fatalf("Failed to create scan dir: %v", err)
	}
	defer os.RemoveAll(scanDir)

	// Create candidate directory (this is the target candidate)
	candidateDir := filepath.Join(scanDir, "candidate")
	if err := os.MkdirAll(candidateDir, 0755); err != nil {
		t.Fatalf("Failed to create candidate dir: %v", err)
	}

	// Create file at root of candidate (depth 0 within candidate)
	candidateFile := filepath.Join(candidateDir, "CandidateFile 2024-01-15 Doc.pdf")
	if err := os.WriteFile(candidateFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create candidate file: %v", err)
	}

	// Create subdirectory with file (depth 1 within candidate)
	subDir := filepath.Join(candidateDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	subFile := filepath.Join(subDir, "SubFile 2024-01-16 Doc.pdf")
	if err := os.WriteFile(subFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	// Test with depth=0 - should only find files at root of each candidate
	opts := DiscoverOptions{MaxDepth: 0}
	result, err := DiscoverWithOptions(scanDir, nil, opts, nil)
	if err != nil {
		t.Fatalf("DiscoverWithOptions failed: %v", err)
	}

	// Should only find CandidateFile (at depth 0 within candidate)
	// Note: prefixes may be stored in lowercase, so use case-insensitive comparison
	foundCandidate := false
	foundSub := false
	for _, rule := range result.NewRules {
		if strings.EqualFold(rule.Prefix, "CandidateFile") {
			foundCandidate = true
		}
		if strings.EqualFold(rule.Prefix, "SubFile") {
			foundSub = true
		}
	}

	if !foundCandidate {
		t.Errorf("Expected to find 'CandidateFile' rule with depth=0, but didn't. Rules found: %v", result.NewRules)
	}
	if foundSub {
		t.Errorf("Found 'SubFile' rule with depth=0, but should not have")
	}

	// Test with depth=1 - should find both files
	opts = DiscoverOptions{MaxDepth: 1}
	result, err = DiscoverWithOptions(scanDir, nil, opts, nil)
	if err != nil {
		t.Fatalf("DiscoverWithOptions failed: %v", err)
	}

	foundCandidate = false
	foundSub = false
	for _, rule := range result.NewRules {
		if strings.EqualFold(rule.Prefix, "CandidateFile") {
			foundCandidate = true
		}
		if strings.EqualFold(rule.Prefix, "SubFile") {
			foundSub = true
		}
	}

	if !foundCandidate {
		t.Errorf("Expected to find 'CandidateFile' rule with depth=1, but didn't")
	}
	if !foundSub {
		t.Errorf("Expected to find 'SubFile' rule with depth=1, but didn't")
	}
}

// TestISODateDirectoriesSkippedAtAllDepths tests that ISO-date directories are
// skipped regardless of the depth setting.
// Validates: Requirement 1.5
func TestISODateDirectoriesSkippedAtAllDepths(t *testing.T) {
	tests := []struct {
		name     string
		maxDepth int
	}{
		{"depth=0", 0},
		{"depth=1", 1},
		{"depth=2", 2},
		{"depth=-1 (unlimited)", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory structure:
			// baseDir/
			//   RegularFile 2024-01-15 Doc.pdf     <- should be found
			//   2024-01-20 Archive/                <- ISO-date dir, should be SKIPPED
			//     ISOFile 2024-01-20 Doc.pdf       <- should NOT be found (in skipped dir)
			//   regular/
			//     RegularSubFile 2024-01-16 Doc.pdf <- should be found (if depth allows)
			//     2024-02-15 Backup/               <- nested ISO-date dir, should be SKIPPED
			//       NestedISOFile 2024-02-15 Doc.pdf <- should NOT be found

			baseDir, err := os.MkdirTemp("", "sorta-iso-skip-*")
			if err != nil {
				t.Fatalf("Failed to create base dir: %v", err)
			}
			defer os.RemoveAll(baseDir)

			// Create regular file at root
			regularFile := filepath.Join(baseDir, "RegularFile 2024-01-15 Doc.pdf")
			if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create regular file: %v", err)
			}

			// Create ISO-date directory at root level
			isoDir := filepath.Join(baseDir, "2024-01-20 Archive")
			if err := os.MkdirAll(isoDir, 0755); err != nil {
				t.Fatalf("Failed to create ISO date dir: %v", err)
			}
			isoFile := filepath.Join(isoDir, "ISOFile 2024-01-20 Doc.pdf")
			if err := os.WriteFile(isoFile, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create ISO file: %v", err)
			}

			// Create regular subdirectory
			regularDir := filepath.Join(baseDir, "regular")
			if err := os.MkdirAll(regularDir, 0755); err != nil {
				t.Fatalf("Failed to create regular dir: %v", err)
			}
			regularSubFile := filepath.Join(regularDir, "RegularSubFile 2024-01-16 Doc.pdf")
			if err := os.WriteFile(regularSubFile, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create regular sub file: %v", err)
			}

			// Create nested ISO-date directory
			nestedISODir := filepath.Join(regularDir, "2024-02-15 Backup")
			if err := os.MkdirAll(nestedISODir, 0755); err != nil {
				t.Fatalf("Failed to create nested ISO dir: %v", err)
			}
			nestedISOFile := filepath.Join(nestedISODir, "NestedISOFile 2024-02-15 Doc.pdf")
			if err := os.WriteFile(nestedISOFile, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create nested ISO file: %v", err)
			}

			// Analyze with specified depth
			prefixes, err := analyzeDirectoryWithDepth(baseDir, tt.maxDepth, nil, nil)
			if err != nil {
				t.Fatalf("analyzeDirectoryWithDepth failed: %v", err)
			}

			// Check results
			foundRegular := false
			foundISO := false
			foundRegularSub := false
			foundNestedISO := false
			for _, p := range prefixes {
				if p == "RegularFile" {
					foundRegular = true
				}
				if p == "ISOFile" {
					foundISO = true
				}
				if p == "RegularSubFile" {
					foundRegularSub = true
				}
				if p == "NestedISOFile" {
					foundNestedISO = true
				}
			}

			// RegularFile should always be found (it's at depth 0)
			if !foundRegular {
				t.Errorf("Expected to find 'RegularFile' prefix, but didn't")
			}

			// ISOFile should NEVER be found (ISO-date directories are skipped)
			if foundISO {
				t.Errorf("Found 'ISOFile' prefix, but ISO-date directories should be skipped")
			}

			// RegularSubFile should be found if depth allows (depth >= 1 or unlimited)
			if tt.maxDepth >= 1 || tt.maxDepth == -1 {
				if !foundRegularSub {
					t.Errorf("Expected to find 'RegularSubFile' prefix at depth %d, but didn't", tt.maxDepth)
				}
			}

			// NestedISOFile should NEVER be found (nested ISO-date directories are skipped)
			if foundNestedISO {
				t.Errorf("Found 'NestedISOFile' prefix, but nested ISO-date directories should be skipped")
			}
		})
	}
}

// TestDepthLimitingEdgeCases tests edge cases for depth limiting.
func TestDepthLimitingEdgeCases(t *testing.T) {
	t.Run("empty directory returns no prefixes", func(t *testing.T) {
		baseDir, err := os.MkdirTemp("", "sorta-empty-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		prefixes, err := analyzeDirectoryWithDepth(baseDir, 0, nil, nil)
		if err != nil {
			t.Fatalf("analyzeDirectoryWithDepth failed: %v", err)
		}

		if len(prefixes) != 0 {
			t.Errorf("Expected no prefixes for empty directory, got %v", prefixes)
		}
	})

	t.Run("files without prefix pattern are ignored", func(t *testing.T) {
		baseDir, err := os.MkdirTemp("", "sorta-noprefix-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create files without prefix pattern
		noPatternFile := filepath.Join(baseDir, "random-file.txt")
		if err := os.WriteFile(noPatternFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		prefixes, err := analyzeDirectoryWithDepth(baseDir, 0, nil, nil)
		if err != nil {
			t.Fatalf("analyzeDirectoryWithDepth failed: %v", err)
		}

		if len(prefixes) != 0 {
			t.Errorf("Expected no prefixes for files without pattern, got %v", prefixes)
		}
	})

	t.Run("multiple files with same prefix at different depths", func(t *testing.T) {
		baseDir, err := os.MkdirTemp("", "sorta-sameprefix-*")
		if err != nil {
			t.Fatalf("Failed to create base dir: %v", err)
		}
		defer os.RemoveAll(baseDir)

		// Create files with same prefix at different depths
		rootFile := filepath.Join(baseDir, "Invoice 2024-01-15 A.pdf")
		if err := os.WriteFile(rootFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create root file: %v", err)
		}

		subDir := filepath.Join(baseDir, "subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("Failed to create subdir: %v", err)
		}
		subFile := filepath.Join(subDir, "Invoice 2024-01-16 B.pdf")
		if err := os.WriteFile(subFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create sub file: %v", err)
		}

		// With depth=0, should find Invoice once (from root)
		prefixes, err := analyzeDirectoryWithDepth(baseDir, 0, nil, nil)
		if err != nil {
			t.Fatalf("analyzeDirectoryWithDepth failed: %v", err)
		}

		invoiceCount := 0
		for _, p := range prefixes {
			if p == "Invoice" {
				invoiceCount++
			}
		}

		if invoiceCount != 1 {
			t.Errorf("Expected exactly 1 'Invoice' prefix at depth 0, got %d", invoiceCount)
		}

		// With depth=1, should still find Invoice once (unique prefixes)
		prefixes, err = analyzeDirectoryWithDepth(baseDir, 1, nil, nil)
		if err != nil {
			t.Fatalf("analyzeDirectoryWithDepth failed: %v", err)
		}

		invoiceCount = 0
		for _, p := range prefixes {
			if p == "Invoice" {
				invoiceCount++
			}
		}

		if invoiceCount != 1 {
			t.Errorf("Expected exactly 1 'Invoice' prefix at depth 1 (unique), got %d", invoiceCount)
		}
	})
}
