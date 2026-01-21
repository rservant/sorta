package organizer

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestFileExists(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "sorta-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test non-existent file
	nonExistent := filepath.Join(tempDir, "nonexistent.txt")
	if FileExists(nonExistent) {
		t.Error("FileExists returned true for non-existent file")
	}

	// Create a file and test it exists
	existingFile := filepath.Join(tempDir, "existing.txt")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if !FileExists(existingFile) {
		t.Error("FileExists returned false for existing file")
	}
}

func TestGenerateDuplicateName_NoConflict(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sorta-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// When no file exists, should return original filename
	result := GenerateDuplicateName(tempDir, "Invoice 2024-01-15 Acme Corp.pdf")
	if result != "Invoice 2024-01-15 Acme Corp.pdf" {
		t.Errorf("Expected original filename, got %q", result)
	}
}

func TestGenerateDuplicateName_FirstDuplicate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sorta-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create existing file
	existingFile := filepath.Join(tempDir, "Invoice 2024-01-15 Acme Corp.pdf")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result := GenerateDuplicateName(tempDir, "Invoice 2024-01-15 Acme Corp.pdf")
	expected := "Invoice 2024-01-15 Acme Corp_duplicate.pdf"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGenerateDuplicateName_SecondDuplicate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sorta-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create existing files
	files := []string{
		"Invoice 2024-01-15 Acme Corp.pdf",
		"Invoice 2024-01-15 Acme Corp_duplicate.pdf",
	}
	for _, f := range files {
		path := filepath.Join(tempDir, f)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	result := GenerateDuplicateName(tempDir, "Invoice 2024-01-15 Acme Corp.pdf")
	expected := "Invoice 2024-01-15 Acme Corp_duplicate_2.pdf"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGenerateDuplicateName_ThirdDuplicate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sorta-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create existing files
	files := []string{
		"Invoice 2024-01-15 Acme Corp.pdf",
		"Invoice 2024-01-15 Acme Corp_duplicate.pdf",
		"Invoice 2024-01-15 Acme Corp_duplicate_2.pdf",
	}
	for _, f := range files {
		path := filepath.Join(tempDir, f)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	result := GenerateDuplicateName(tempDir, "Invoice 2024-01-15 Acme Corp.pdf")
	expected := "Invoice 2024-01-15 Acme Corp_duplicate_3.pdf"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGenerateDuplicateName_NoExtension(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sorta-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create existing file without extension
	existingFile := filepath.Join(tempDir, "README")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result := GenerateDuplicateName(tempDir, "README")
	expected := "README_duplicate"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGenerateDuplicateName_MultipleDots(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sorta-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create existing file with multiple dots
	existingFile := filepath.Join(tempDir, "file.backup.tar.gz")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result := GenerateDuplicateName(tempDir, "file.backup.tar.gz")
	expected := "file.backup.tar_duplicate.gz"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// Feature: config-auto-discover, Property 10: Duplicate File Naming
// Validates: Requirements 10.1, 10.2, 10.3, 10.4, 10.5

// genFilenameWithExtension generates a filename with a valid extension
func genFilenameWithExtension() gopter.Gen {
	return gopter.CombineGens(
		gen.SliceOfN(8, gen.AlphaNumChar()).Map(func(chars []rune) string { return string(chars) }),
		gen.OneConstOf(".pdf", ".txt", ".doc", ".jpg", ".png"),
	).Map(func(vals []interface{}) string {
		base := vals[0].(string)
		ext := vals[1].(string)
		return base + ext
	})
}

// genNumExistingDuplicates generates a number of existing duplicates (0-5)
func genNumExistingDuplicates() gopter.Gen {
	return gen.IntRange(0, 5)
}

func TestDuplicateFileNaming_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)

	// Property 10: Duplicate File Naming
	// For any file being moved to a destination where a file with the same name exists,
	// the system SHALL generate a unique filename by appending _duplicate (or _duplicate_N
	// for subsequent duplicates) before the file extension.
	properties.Property("Generated duplicate names are unique and follow naming convention", prop.ForAll(
		func(filename string, numExisting int) bool {
			// Create temp directory
			tempDir, err := os.MkdirTemp("", "sorta-prop-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			// Create the original file and existing duplicates
			existingFiles := make([]string, 0, numExisting+1)
			existingFiles = append(existingFiles, filename)

			ext := filepath.Ext(filename)
			baseName := strings.TrimSuffix(filename, ext)

			// Create duplicate files based on numExisting
			for i := 0; i < numExisting; i++ {
				var dupName string
				if i == 0 {
					dupName = baseName + "_duplicate" + ext
				} else {
					dupName = baseName + "_duplicate_" + strconv.Itoa(i+1) + ext
				}
				existingFiles = append(existingFiles, dupName)
			}

			// Create all existing files
			for _, f := range existingFiles {
				path := filepath.Join(tempDir, f)
				if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
					t.Logf("Failed to create test file: %v", err)
					return false
				}
			}

			// Generate duplicate name
			result := GenerateDuplicateName(tempDir, filename)

			// Verify: result should be unique (not in existingFiles)
			for _, existing := range existingFiles {
				if result == existing {
					t.Logf("Generated name %q conflicts with existing file", result)
					return false
				}
			}

			// Verify: result should follow the naming convention
			// Either _duplicate or _duplicate_N before extension
			if !strings.Contains(result, "_duplicate") {
				t.Logf("Generated name %q does not contain _duplicate", result)
				return false
			}

			// Verify: the _duplicate suffix should be before the extension
			resultExt := filepath.Ext(result)
			resultBase := strings.TrimSuffix(result, resultExt)
			if !strings.HasSuffix(resultBase, "_duplicate") && !strings.Contains(resultBase, "_duplicate_") {
				t.Logf("Generated name %q has _duplicate in wrong position", result)
				return false
			}

			// Verify: extension should be preserved
			if resultExt != ext {
				t.Logf("Extension not preserved: expected %q, got %q", ext, resultExt)
				return false
			}

			// Verify: file with generated name doesn't exist
			resultPath := filepath.Join(tempDir, result)
			if FileExists(resultPath) {
				t.Logf("Generated name %q already exists", result)
				return false
			}

			return true
		},
		genFilenameWithExtension(),
		genNumExistingDuplicates(),
	))

	properties.TestingRun(t)
}
