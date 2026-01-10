package discovery

import (
	"fmt"
	"os"
	"path/filepath"
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
					{Prefix: toggleCase(prefix), TargetDirectory: "/some/other/dir"},
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
