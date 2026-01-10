// Package organizer handles file movement and organization for Sorta.
package organizer

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// duplicatePattern matches filenames with _duplicate or _duplicate_N suffix before extension
var duplicatePattern = regexp.MustCompile(`^(.+)_duplicate(?:_(\d+))?(\.[^.]+)?$`)

// FileExists checks if a file exists at the given path.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GenerateDuplicateName creates a unique filename for duplicates.
// If the destination file exists, it appends "_duplicate" before the extension.
// If "_duplicate" already exists, it appends "_duplicate_2", "_duplicate_3", etc.
//
// Examples:
//   - "file.pdf" -> "file_duplicate.pdf" (if file.pdf exists)
//   - "file_duplicate.pdf" -> "file_duplicate_2.pdf" (if file_duplicate.pdf exists)
//   - "file_duplicate_2.pdf" -> "file_duplicate_3.pdf" (if file_duplicate_2.pdf exists)
func GenerateDuplicateName(destDir, filename string) string {
	destPath := filepath.Join(destDir, filename)

	// If file doesn't exist, return original filename
	if !FileExists(destPath) {
		return filename
	}

	// Extract base name and extension
	ext := filepath.Ext(filename)
	baseName := strings.TrimSuffix(filename, ext)

	// Check if filename already has a duplicate suffix
	if matches := duplicatePattern.FindStringSubmatch(filename); matches != nil {
		// matches[1] = base name without _duplicate suffix
		// matches[2] = number (if present, empty string otherwise)
		// matches[3] = extension (if present)
		originalBase := matches[1]
		numStr := matches[2]
		originalExt := matches[3]

		var nextNum int
		if numStr == "" {
			// Has _duplicate but no number, next is _duplicate_2
			nextNum = 2
		} else {
			// Has _duplicate_N, increment N
			num, _ := strconv.Atoi(numStr)
			nextNum = num + 1
		}

		// Try incrementing numbers until we find a unique name
		for {
			newFilename := originalBase + "_duplicate_" + strconv.Itoa(nextNum) + originalExt
			newPath := filepath.Join(destDir, newFilename)
			if !FileExists(newPath) {
				return newFilename
			}
			nextNum++
		}
	}

	// No duplicate suffix yet, try adding _duplicate
	duplicateFilename := baseName + "_duplicate" + ext
	duplicatePath := filepath.Join(destDir, duplicateFilename)
	if !FileExists(duplicatePath) {
		return duplicateFilename
	}

	// _duplicate exists, try _duplicate_2, _duplicate_3, etc.
	for n := 2; ; n++ {
		numberedFilename := baseName + "_duplicate_" + strconv.Itoa(n) + ext
		numberedPath := filepath.Join(destDir, numberedFilename)
		if !FileExists(numberedPath) {
			return numberedFilename
		}
	}
}
