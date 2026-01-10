// Package discovery handles auto-discovery of prefix rules from existing file structures.
package discovery

import (
	"regexp"
	"strings"
)

// PrefixPattern represents the regex for detecting file patterns.
// Pattern: ^([A-Za-z][A-Za-z0-9]*)\s+(\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01]))\s+(.+)$
// This matches filenames like "Invoice 2024-01-15 Acme Corp.pdf"
// The date portion validates:
// - Month: 01-12
// - Day: 01-31
var PrefixPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9]*)\s+(\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01]))\s+(.+)$`)

// ExtractPrefixFromFilename returns the prefix if the filename matches the pattern.
// The pattern is: <prefix> <YYYY-MM-DD> <other_info>
// where prefix starts with a letter, is followed by alphanumeric characters,
// and is separated from the date by exactly one space.
//
// Returns the extracted prefix and true if matched, or empty string and false if not matched.
func ExtractPrefixFromFilename(filename string) (prefix string, matched bool) {
	// Remove file extension for matching
	nameWithoutExt := removeExtension(filename)

	matches := PrefixPattern.FindStringSubmatch(nameWithoutExt)
	if matches == nil {
		return "", false
	}

	// matches[0] is the full match
	// matches[1] is the prefix
	// matches[2] is the date
	// matches[3] is the other info
	return matches[1], true
}

// removeExtension removes the file extension from a filename.
func removeExtension(filename string) string {
	// Find the last dot in the filename
	lastDot := strings.LastIndex(filename, ".")
	if lastDot == -1 {
		return filename
	}
	return filename[:lastDot]
}
