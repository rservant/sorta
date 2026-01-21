package organizer

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"sorta/internal/classifier"
	"sorta/internal/config"
	"sorta/internal/scanner"
)

// Feature: sorta-file-organizer, Property 7: Unclassified Filename Preservation
// Validates: Requirements 6.4

// genNonMatchingFilename generates filenames that won't match any prefix rules.
func genNonMatchingFilename() gopter.Gen {
	return gen.SliceOfN(10, gen.AlphaChar()).Map(func(chars []rune) string {
		// Prefix with NOMATCH_ to ensure it doesn't match any rules
		return "NOMATCH_" + string(chars) + ".txt"
	})
}

func TestUnclassifiedFilenamePreservation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)

	properties.Property("Unclassified files preserve their original filename exactly", prop.ForAll(
		func(filename string) bool {
			// Create temp directories
			sourceDir, err := os.MkdirTemp("", "sorta-source-*")
			if err != nil {
				t.Logf("Failed to create source dir: %v", err)
				return false
			}
			defer os.RemoveAll(sourceDir)

			forReviewDir, err := os.MkdirTemp("", "sorta-review-*")
			if err != nil {
				t.Logf("Failed to create review dir: %v", err)
				return false
			}
			defer os.RemoveAll(forReviewDir)

			// Create source file
			sourcePath := filepath.Join(sourceDir, filename)
			if err := os.WriteFile(sourcePath, []byte("test content"), 0644); err != nil {
				t.Logf("Failed to create source file: %v", err)
				return false
			}

			// Create file entry
			absPath, _ := filepath.Abs(sourcePath)
			fileEntry := scanner.FileEntry{
				Name:     filename,
				FullPath: absPath,
			}

			// Create unclassified classification
			classification := &classifier.Classification{
				Type:   "UNCLASSIFIED",
				Reason: classifier.NoPrefixMatch,
			}

			// Create config
			cfg := &config.Configuration{
				InboundDirectories: []string{sourceDir},
				PrefixRules:        []config.PrefixRule{{Prefix: "Invoice", OutboundDirectory: "/tmp/invoices"}},
			}

			// Organize the file
			result, err := Organize(fileEntry, classification, cfg)
			if err != nil {
				t.Logf("Organize failed: %v", err)
				return false
			}

			// Verify destination filename equals source filename exactly
			destFilename := filepath.Base(result.DestinationPath)
			if destFilename != filename {
				t.Logf("Filename not preserved: expected %q, got %q", filename, destFilename)
				return false
			}

			return true
		},
		genNonMatchingFilename(),
	))

	properties.TestingRun(t)
}

// Feature: sorta-file-organizer, Property 8: File Content Integrity
// Validates: Requirements 7.1

// genRandomContent generates random byte content for files.
func genRandomContent() gopter.Gen {
	return gen.SliceOfN(100, gen.UInt8()).Map(func(bytes []uint8) []byte {
		result := make([]byte, len(bytes))
		for i, b := range bytes {
			result[i] = byte(b)
		}
		return result
	})
}

// genValidFilename generates a valid filename with prefix and date.
func genValidFilename() gopter.Gen {
	return gopter.CombineGens(
		gen.SliceOfN(5, gen.AlphaChar()).Map(func(chars []rune) string { return string(chars) }),
		gen.IntRange(2000, 2030),
		gen.IntRange(1, 12),
		gen.IntRange(1, 28),
	).Map(func(vals []interface{}) string {
		prefix := vals[0].(string)
		year := vals[1].(int)
		month := vals[2].(int)
		day := vals[3].(int)
		return prefix + " " + formatDate(year, month, day) + " document.txt"
	})
}

func formatDate(year, month, day int) string {
	return string([]byte{
		byte('0' + year/1000),
		byte('0' + (year/100)%10),
		byte('0' + (year/10)%10),
		byte('0' + year%10),
		'-',
		byte('0' + month/10),
		byte('0' + month%10),
		'-',
		byte('0' + day/10),
		byte('0' + day%10),
	})
}

func TestFileContentIntegrity(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)

	properties.Property("File contents are byte-for-byte identical after processing", prop.ForAll(
		func(content []byte, filename string) bool {
			// Create temp directories
			sourceDir, err := os.MkdirTemp("", "sorta-source-*")
			if err != nil {
				t.Logf("Failed to create source dir: %v", err)
				return false
			}
			defer os.RemoveAll(sourceDir)

			targetDir, err := os.MkdirTemp("", "sorta-target-*")
			if err != nil {
				t.Logf("Failed to create target dir: %v", err)
				return false
			}
			defer os.RemoveAll(targetDir)

			forReviewDir, err := os.MkdirTemp("", "sorta-review-*")
			if err != nil {
				t.Logf("Failed to create review dir: %v", err)
				return false
			}
			defer os.RemoveAll(forReviewDir)

			// Create source file with random content
			sourcePath := filepath.Join(sourceDir, filename)
			if err := os.WriteFile(sourcePath, content, 0644); err != nil {
				t.Logf("Failed to create source file: %v", err)
				return false
			}

			// Calculate original hash
			originalHash := sha256.Sum256(content)

			// Create file entry
			absPath, _ := filepath.Abs(sourcePath)
			fileEntry := scanner.FileEntry{
				Name:     filename,
				FullPath: absPath,
			}

			// Extract prefix from filename (everything before first space)
			prefix := extractPrefixFromNormalisedFilename(filename)

			// Create config with matching prefix rule
			cfg := &config.Configuration{
				InboundDirectories: []string{sourceDir},
				PrefixRules: []config.PrefixRule{
					{Prefix: prefix, OutboundDirectory: targetDir},
				},
			}

			// Classify the file
			classification := classifier.Classify(filename, cfg.PrefixRules)

			// Organize the file
			result, err := Organize(fileEntry, classification, cfg)
			if err != nil {
				t.Logf("Organize failed: %v", err)
				return false
			}

			// Read destination file and calculate hash
			destContent, err := os.ReadFile(result.DestinationPath)
			if err != nil {
				t.Logf("Failed to read destination file: %v", err)
				return false
			}

			destHash := sha256.Sum256(destContent)

			// Verify hashes match
			if originalHash != destHash {
				t.Logf("Content hash mismatch: original %x, destination %x", originalHash, destHash)
				return false
			}

			return true
		},
		genRandomContent(),
		genValidFilename(),
	))

	properties.TestingRun(t)
}

// Feature: config-auto-discover, Property 9: For-Review Path Generation
// Validates: Requirements 9.1, 9.3

// genSourceDirectoryPath generates valid source directory paths.
func genSourceDirectoryPath() gopter.Gen {
	return gen.SliceOfN(5, gen.AlphaChar()).Map(func(chars []rune) string {
		return "/path/to/" + string(chars)
	})
}

func TestForReviewPathGeneration_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)

	// Property 9a: For-review path is always a subdirectory named "for-review" within the source directory
	properties.Property("For-review path is the for-review subdirectory within source directory", prop.ForAll(
		func(sourceDir string) bool {
			forReviewPath := GetForReviewPath(sourceDir)

			// The for-review path should be sourceDir + "/for-review"
			expectedPath := filepath.Join(sourceDir, "for-review")
			if forReviewPath != expectedPath {
				t.Logf("Expected %q, got %q", expectedPath, forReviewPath)
				return false
			}

			// The parent of for-review path should be the source directory
			parentDir := filepath.Dir(forReviewPath)
			if parentDir != sourceDir {
				t.Logf("Parent directory mismatch: expected %q, got %q", sourceDir, parentDir)
				return false
			}

			// The base name should be "for-review"
			baseName := filepath.Base(forReviewPath)
			if baseName != "for-review" {
				t.Logf("Base name mismatch: expected 'for-review', got %q", baseName)
				return false
			}

			return true
		},
		genSourceDirectoryPath(),
	))

	// Property 9b: Unclassified files from a source are moved to that source's for-review subdirectory
	properties.Property("Unclassified files are moved to their source's for-review subdirectory", prop.ForAll(
		func(filename string) bool {
			// Create temp source directory
			sourceDir, err := os.MkdirTemp("", "sorta-source-*")
			if err != nil {
				t.Logf("Failed to create source dir: %v", err)
				return false
			}
			defer os.RemoveAll(sourceDir)

			// Create source file
			sourcePath := filepath.Join(sourceDir, filename)
			if err := os.WriteFile(sourcePath, []byte("test content"), 0644); err != nil {
				t.Logf("Failed to create source file: %v", err)
				return false
			}

			// Create file entry
			absPath, _ := filepath.Abs(sourcePath)
			fileEntry := scanner.FileEntry{
				Name:     filename,
				FullPath: absPath,
			}

			// Create unclassified classification
			classification := &classifier.Classification{
				Type:   "UNCLASSIFIED",
				Reason: classifier.NoPrefixMatch,
			}

			// Create config
			cfg := &config.Configuration{
				InboundDirectories: []string{sourceDir},
				PrefixRules:        []config.PrefixRule{{Prefix: "Invoice", OutboundDirectory: "/tmp/invoices"}},
			}

			// Organize the file
			result, err := Organize(fileEntry, classification, cfg)
			if err != nil {
				t.Logf("Organize failed: %v", err)
				return false
			}

			// Verify the file was moved to the for-review subdirectory of its source
			expectedForReviewDir := GetForReviewPath(sourceDir)
			actualDir := filepath.Dir(result.DestinationPath)

			// Normalize paths for comparison
			expectedForReviewDir, _ = filepath.Abs(expectedForReviewDir)
			actualDir, _ = filepath.Abs(actualDir)

			if actualDir != expectedForReviewDir {
				t.Logf("File not moved to correct for-review dir: expected %q, got %q", expectedForReviewDir, actualDir)
				return false
			}

			// Verify the for-review directory is within the source directory
			absSourceDir, _ := filepath.Abs(sourceDir)
			if filepath.Dir(actualDir) != absSourceDir {
				t.Logf("For-review dir not within source: source=%q, for-review parent=%q", absSourceDir, filepath.Dir(actualDir))
				return false
			}

			return true
		},
		genNonMatchingFilename(),
	))

	properties.TestingRun(t)
}
