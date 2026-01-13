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
// ISO-date directories ARE now scanned for prefix extraction.

func TestISODateDirectoryScanning(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Files in ISO-date subdirectories ARE analyzed (directories are scanned)
	properties.Property("Files in ISO-date subdirectories are analyzed", prop.ForAll(
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

			// The prefix SHOULD be found because ISO-date directories are now scanned
			found := false
			for _, p := range prefixes {
				if p == prefix {
					found = true
					break
				}
			}

			if !found {
				t.Logf("Prefix %q was not found but should have been (in ISO-date dir %q)", prefix, isoDateDir)
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
func TestDirectoryFilteringBehavior(t *testing.T) {
	t.Run("files in ISO-date subdirectories are analyzed", func(t *testing.T) {
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

		// The prefix SHOULD be found (ISO-date directories are now scanned)
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
		//     2024-02-01 Archive/        <- ISO, now also scanned
		//       Receipt 2024-02-01 B.pdf <- should be found (ISO dirs are now scanned)

		invoicesDir := filepath.Join(baseDir, "invoices")
		if err := os.MkdirAll(invoicesDir, 0755); err != nil {
			t.Fatalf("Failed to create invoices dir: %v", err)
		}

		// File in non-ISO directory (should be found)
		filePath1 := filepath.Join(invoicesDir, "Invoice 2024-01-15 A.pdf")
		if err := os.WriteFile(filePath1, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// ISO-date subdirectory inside invoices (now scanned)
		archiveDir := filepath.Join(invoicesDir, "2024-02-01 Archive")
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			t.Fatalf("Failed to create archive dir: %v", err)
		}

		// File in ISO-date directory (should now be found)
		filePath2 := filepath.Join(archiveDir, "Receipt 2024-02-01 B.pdf")
		if err := os.WriteFile(filePath2, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Analyze the base directory
		prefixes, err := analyzeDirectory(baseDir)
		if err != nil {
			t.Fatalf("analyzeDirectory failed: %v", err)
		}

		// Both Invoice and Receipt should be found
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
		if !foundReceipt {
			t.Errorf("Prefix 'Receipt' was not found but should have been (ISO-date dirs are now scanned)")
		}
	})

	t.Run("deeply nested ISO-date directories are scanned", func(t *testing.T) {
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
		//       2024-03-15 Deep Archive/  <- ISO, now scanned
		//         Statement 2024-03-15 X.pdf <- should be found

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

		// Statement SHOULD be found (ISO-date directories are now scanned)
		found := false
		for _, p := range prefixes {
			if p == "Statement" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Prefix 'Statement' was not found but should have been (ISO-date dirs are now scanned)")
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
