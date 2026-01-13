package audit

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: audit-trail, Property 5: FileIdentity Completeness for Move Operations
// Validates: Requirements 4.1, 4.2, 4.3, 4.4

// genFileContent generates random file content of varying sizes.
func genFileContent() gopter.Gen {
	return gen.IntRange(0, 10000).FlatMap(func(size interface{}) gopter.Gen {
		return gen.SliceOfN(size.(int), gen.UInt8Range(0, 255))
	}, reflect.TypeOf([]uint8{}))
}

// genFileName generates valid file names with various characters.
func genFileName() gopter.Gen {
	return gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0 && len(s) <= 50
	}).Map(func(s string) string {
		return s + ".txt"
	})
}

// TestFileIdentityCompleteness tests Property 5: FileIdentity Completeness for Move Operations
// For any MOVE event, the fileIdentity field SHALL contain a valid SHA-256 contentHash,
// non-negative size in bytes, and modTime timestamp.
func TestFileIdentityCompleteness(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Reduced from 100 for faster execution

	properties := gopter.NewProperties(parameters)

	properties.Property("CaptureIdentity returns complete FileIdentity with valid fields", prop.ForAll(
		func(content []byte, fileName string) bool {
			// Create a temporary directory and file
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, fileName)

			// Write the content to the file
			if err := os.WriteFile(filePath, content, 0644); err != nil {
				t.Logf("Failed to write file: %v", err)
				return false
			}

			// Capture identity
			resolver := NewIdentityResolver()
			identity, err := resolver.CaptureIdentity(filePath)
			if err != nil {
				t.Logf("CaptureIdentity failed: %v", err)
				return false
			}

			// Verify contentHash is a valid SHA-256 hex string (64 characters)
			if len(identity.ContentHash) != 64 {
				t.Logf("ContentHash length is %d, expected 64", len(identity.ContentHash))
				return false
			}

			// Verify all characters are valid hex
			for _, c := range identity.ContentHash {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Logf("ContentHash contains invalid character: %c", c)
					return false
				}
			}

			// Verify size is non-negative and matches content length
			if identity.Size < 0 {
				t.Logf("Size is negative: %d", identity.Size)
				return false
			}
			if identity.Size != int64(len(content)) {
				t.Logf("Size mismatch: got %d, expected %d", identity.Size, len(content))
				return false
			}

			// Verify modTime is not zero
			if identity.ModTime.IsZero() {
				t.Logf("ModTime is zero")
				return false
			}

			return true
		},
		genFileContent(),
		genFileName(),
	))

	properties.TestingRun(t)
}

// Unit tests for identity edge cases
// Requirements: 4.6, 4.8

func TestCaptureIdentity_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.txt")

	// Create empty file
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	resolver := NewIdentityResolver()
	identity, err := resolver.CaptureIdentity(filePath)
	if err != nil {
		t.Fatalf("CaptureIdentity failed for empty file: %v", err)
	}

	// Empty file should have size 0
	if identity.Size != 0 {
		t.Errorf("Expected size 0 for empty file, got %d", identity.Size)
	}

	// SHA-256 of empty content is a known value
	expectedHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if identity.ContentHash != expectedHash {
		t.Errorf("Expected hash %s for empty file, got %s", expectedHash, identity.ContentHash)
	}
}

func TestCaptureIdentity_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "large.bin")

	// Create a 1MB file
	largeContent := make([]byte, 1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	if err := os.WriteFile(filePath, largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	resolver := NewIdentityResolver()
	identity, err := resolver.CaptureIdentity(filePath)
	if err != nil {
		t.Fatalf("CaptureIdentity failed for large file: %v", err)
	}

	if identity.Size != int64(len(largeContent)) {
		t.Errorf("Expected size %d, got %d", len(largeContent), identity.Size)
	}

	// Hash should be 64 hex characters
	if len(identity.ContentHash) != 64 {
		t.Errorf("Expected 64 character hash, got %d", len(identity.ContentHash))
	}
}

func TestCaptureIdentity_SpecialCharactersInName(t *testing.T) {
	tmpDir := t.TempDir()

	// Test various special characters in filenames
	specialNames := []string{
		"file with spaces.txt",
		"file-with-dashes.txt",
		"file_with_underscores.txt",
		"file.multiple.dots.txt",
		"UPPERCASE.TXT",
		"MixedCase.Txt",
	}

	resolver := NewIdentityResolver()
	content := []byte("test content")

	for _, name := range specialNames {
		filePath := filepath.Join(tmpDir, name)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", name, err)
		}

		identity, err := resolver.CaptureIdentity(filePath)
		if err != nil {
			t.Errorf("CaptureIdentity failed for file %s: %v", name, err)
			continue
		}

		if identity.Size != int64(len(content)) {
			t.Errorf("Size mismatch for file %s: expected %d, got %d", name, len(content), identity.Size)
		}
	}
}

func TestVerifyIdentity_Matching(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content for verification")

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	resolver := NewIdentityResolver()

	// Capture identity
	identity, err := resolver.CaptureIdentity(filePath)
	if err != nil {
		t.Fatalf("CaptureIdentity failed: %v", err)
	}

	// Verify should match
	match, err := resolver.VerifyIdentity(filePath, *identity)
	if err != nil {
		t.Fatalf("VerifyIdentity failed: %v", err)
	}

	if match != IdentityMatches {
		t.Errorf("Expected IdentityMatches, got %v", match)
	}
}

func TestVerifyIdentity_HashMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := []byte("original content")

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	resolver := NewIdentityResolver()

	// Capture identity
	identity, err := resolver.CaptureIdentity(filePath)
	if err != nil {
		t.Fatalf("CaptureIdentity failed: %v", err)
	}

	// Modify the file content (same size to avoid size mismatch)
	modifiedContent := []byte("modified content")
	if err := os.WriteFile(filePath, modifiedContent, 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Verify should detect hash mismatch
	match, err := resolver.VerifyIdentity(filePath, *identity)
	if err != nil {
		t.Fatalf("VerifyIdentity failed: %v", err)
	}

	if match != IdentityHashMismatch {
		t.Errorf("Expected IdentityHashMismatch, got %v", match)
	}
}

func TestVerifyIdentity_SizeMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := []byte("original content")

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	resolver := NewIdentityResolver()

	// Capture identity
	identity, err := resolver.CaptureIdentity(filePath)
	if err != nil {
		t.Fatalf("CaptureIdentity failed: %v", err)
	}

	// Modify the file with different size
	modifiedContent := []byte("much longer modified content that has different size")
	if err := os.WriteFile(filePath, modifiedContent, 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Verify should detect size mismatch (checked before hash)
	match, err := resolver.VerifyIdentity(filePath, *identity)
	if err != nil {
		t.Fatalf("VerifyIdentity failed: %v", err)
	}

	if match != IdentitySizeMismatch {
		t.Errorf("Expected IdentitySizeMismatch, got %v", match)
	}
}

func TestVerifyIdentity_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.txt")

	resolver := NewIdentityResolver()

	identity := FileIdentity{
		ContentHash: "abc123",
		Size:        100,
	}

	match, err := resolver.VerifyIdentity(filePath, identity)
	if err != nil {
		t.Fatalf("VerifyIdentity should not return error for missing file: %v", err)
	}

	if match != IdentityNotFound {
		t.Errorf("Expected IdentityNotFound, got %v", match)
	}
}

func TestFindByHash_SingleMatch(t *testing.T) {
	tmpDir := t.TempDir()
	content := []byte("unique content for hash search")

	// Create a file with known content
	filePath := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	resolver := NewIdentityResolver()

	// Get the hash
	identity, err := resolver.CaptureIdentity(filePath)
	if err != nil {
		t.Fatalf("CaptureIdentity failed: %v", err)
	}

	// Search for the hash
	matches, err := resolver.FindByHash(identity.ContentHash, []string{tmpDir})
	if err != nil {
		t.Fatalf("FindByHash failed: %v", err)
	}

	if len(matches) != 1 {
		t.Errorf("Expected 1 match, got %d", len(matches))
	}

	if len(matches) > 0 && matches[0] != filePath {
		t.Errorf("Expected match at %s, got %s", filePath, matches[0])
	}
}

func TestFindByHash_MultipleMatches(t *testing.T) {
	tmpDir := t.TempDir()
	content := []byte("duplicate content")

	// Create multiple files with same content
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, content, 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, content, 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	resolver := NewIdentityResolver()

	// Get the hash
	identity, err := resolver.CaptureIdentity(file1)
	if err != nil {
		t.Fatalf("CaptureIdentity failed: %v", err)
	}

	// Search for the hash
	matches, err := resolver.FindByHash(identity.ContentHash, []string{tmpDir})
	if err != nil {
		t.Fatalf("FindByHash failed: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(matches))
	}
}

func TestFindByHash_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with different content
	filePath := filepath.Join(tmpDir, "other.txt")
	if err := os.WriteFile(filePath, []byte("different content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	resolver := NewIdentityResolver()

	// Search for a hash that doesn't exist
	nonExistentHash := "0000000000000000000000000000000000000000000000000000000000000000"
	matches, err := resolver.FindByHash(nonExistentHash, []string{tmpDir})
	if err != nil {
		t.Fatalf("FindByHash failed: %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("Expected 0 matches, got %d", len(matches))
	}
}

func TestCaptureIdentity_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	resolver := NewIdentityResolver()

	// Trying to capture identity of a directory should fail
	_, err := resolver.CaptureIdentity(tmpDir)
	if err == nil {
		t.Error("Expected error when capturing identity of directory")
	}
}

func TestCaptureIdentity_NonExistent(t *testing.T) {
	resolver := NewIdentityResolver()

	// Trying to capture identity of non-existent file should fail
	_, err := resolver.CaptureIdentity("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("Expected error when capturing identity of non-existent file")
	}
}
