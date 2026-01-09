package scanner

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: sorta-file-organizer, Property 9: Scanner Returns Only Files
// Validates: Requirements 2.3

// DirectoryStructure represents a generated directory structure for testing.
type DirectoryStructure struct {
	Files       []string // List of file names to create
	Directories []string // List of subdirectory names to create
}

// genFileName generates valid file names.
func genFileName() gopter.Gen {
	return gen.IntRange(1, 20).FlatMap(func(length interface{}) gopter.Gen {
		return gen.SliceOfN(length.(int), gen.AlphaLowerChar())
	}, reflect.TypeOf([]rune{})).Map(func(chars []rune) string {
		return string(chars) + ".txt"
	})
}

// genDirName generates valid directory names.
func genDirName() gopter.Gen {
	return gen.IntRange(1, 20).FlatMap(func(length interface{}) gopter.Gen {
		return gen.SliceOfN(length.(int), gen.AlphaLowerChar())
	}, reflect.TypeOf([]rune{})).Map(func(chars []rune) string {
		return "dir_" + string(chars)
	})
}

// genDirectoryStructure generates a directory structure with files and subdirectories.
func genDirectoryStructure() gopter.Gen {
	return gopter.CombineGens(
		gen.SliceOfN(5, genFileName()),
		gen.SliceOfN(3, genDirName()),
	).Map(func(vals []interface{}) DirectoryStructure {
		files := vals[0].([]string)
		dirs := vals[1].([]string)

		// Ensure uniqueness
		fileSet := make(map[string]bool)
		uniqueFiles := []string{}
		for _, f := range files {
			if !fileSet[f] {
				fileSet[f] = true
				uniqueFiles = append(uniqueFiles, f)
			}
		}

		dirSet := make(map[string]bool)
		uniqueDirs := []string{}
		for _, d := range dirs {
			if !dirSet[d] && !fileSet[d] {
				dirSet[d] = true
				uniqueDirs = append(uniqueDirs, d)
			}
		}

		return DirectoryStructure{
			Files:       uniqueFiles,
			Directories: uniqueDirs,
		}
	})
}

// setupTestDirectory creates a temporary directory with the given structure.
func setupTestDirectory(t *testing.T, structure DirectoryStructure) string {
	tmpDir := t.TempDir()

	// Create files
	for _, fileName := range structure.Files {
		filePath := filepath.Join(tmpDir, fileName)
		if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fileName, err)
		}
	}

	// Create subdirectories
	for _, dirName := range structure.Directories {
		dirPath := filepath.Join(tmpDir, dirName)
		if err := os.Mkdir(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dirName, err)
		}
	}

	return tmpDir
}

func TestScannerReturnsOnlyFiles(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Scanner returns only files, excluding subdirectories", prop.ForAll(
		func(structure DirectoryStructure) bool {
			// Setup the test directory
			tmpDir := setupTestDirectory(t, structure)

			// Scan the directory
			entries, err := Scan(tmpDir)
			if err != nil {
				t.Logf("Scan failed: %v", err)
				return false
			}

			// Verify the count matches expected files
			if len(entries) != len(structure.Files) {
				t.Logf("Expected %d files, got %d", len(structure.Files), len(entries))
				return false
			}

			// Verify all returned entries are files (not directories)
			for _, entry := range entries {
				info, err := os.Stat(entry.FullPath)
				if err != nil {
					t.Logf("Failed to stat %s: %v", entry.FullPath, err)
					return false
				}
				if info.IsDir() {
					t.Logf("Entry %s is a directory, expected file", entry.Name)
					return false
				}
			}

			// Verify all expected files are present
			fileSet := make(map[string]bool)
			for _, entry := range entries {
				fileSet[entry.Name] = true
			}
			for _, expectedFile := range structure.Files {
				if !fileSet[expectedFile] {
					t.Logf("Expected file %s not found in results", expectedFile)
					return false
				}
			}

			// Verify no directories are in the results
			for _, dirName := range structure.Directories {
				if fileSet[dirName] {
					t.Logf("Directory %s should not be in results", dirName)
					return false
				}
			}

			return true
		},
		genDirectoryStructure(),
	))

	properties.TestingRun(t)
}
