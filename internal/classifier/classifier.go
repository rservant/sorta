// Package classifier handles file classification based on prefix and date for Sorta.
package classifier

import (
	"sorta/internal/config"
	"sorta/internal/dateparser"
	"sorta/internal/matcher"
	"sorta/internal/normalizer"
)

// UnclassifiedReason represents why a file could not be classified.
type UnclassifiedReason string

const (
	NoPrefixMatch    UnclassifiedReason = "NO_PREFIX_MATCH"
	MissingDelimiter UnclassifiedReason = "MISSING_DELIMITER"
	InvalidDate      UnclassifiedReason = "INVALID_DATE"
)

// Classification represents the result of classifying a file.
// It is either CLASSIFIED (with destination info) or UNCLASSIFIED (with reason).
type Classification struct {
	Type               string // "CLASSIFIED" or "UNCLASSIFIED"
	Year               int
	NormalisedFilename string
	TargetDirectory    string
	Reason             UnclassifiedReason
}

// Classify determines the classification of a file based on its filename and prefix rules.
// For valid files, it returns CLASSIFIED with year, normalised filename, and target directory.
// For invalid files, it returns UNCLASSIFIED with the reason.
func Classify(filename string, rules []config.PrefixRule) *Classification {
	// Step 1: Match filename against prefix rules
	matchResult := matcher.Match(filename, rules)

	if !matchResult.Matched {
		return &Classification{
			Type:   "UNCLASSIFIED",
			Reason: NoPrefixMatch,
		}
	}

	// Step 2: Extract ISO date from remainder
	// The remainder should start with the date (YYYY-MM-DD)
	remainder := matchResult.Remainder

	// Check if remainder is long enough to contain a date
	if len(remainder) < 10 {
		return &Classification{
			Type:   "UNCLASSIFIED",
			Reason: InvalidDate,
		}
	}

	// Extract the date portion (first 10 characters)
	datePortion := remainder[:10]

	// Parse the ISO date
	isoDate, err := dateparser.ParseIsoDate(datePortion)
	if err != nil {
		return &Classification{
			Type:   "UNCLASSIFIED",
			Reason: InvalidDate,
		}
	}

	// Step 3: Normalize the filename
	// The matched prefix in the filename is the original casing
	// We need to extract it from the original filename
	matchedPrefix := filename[:len(matchResult.Rule.Prefix)]
	canonicalPrefix := matchResult.Rule.Prefix

	normalisedFilename := normalizer.Normalize(filename, matchedPrefix, canonicalPrefix)

	return &Classification{
		Type:               "CLASSIFIED",
		Year:               isoDate.Year,
		NormalisedFilename: normalisedFilename,
		TargetDirectory:    matchResult.Rule.TargetDirectory,
	}
}

// extractDateFromRemainder extracts the date portion from the remainder string.
// The date should be at the beginning of the remainder in YYYY-MM-DD format.
func extractDateFromRemainder(remainder string) string {
	// Date format is exactly 10 characters: YYYY-MM-DD
	if len(remainder) < 10 {
		return ""
	}

	// Check if the first 10 characters look like a date pattern
	datePortion := remainder[:10]

	// Basic format check: should have dashes at positions 4 and 7
	if len(datePortion) == 10 && datePortion[4] == '-' && datePortion[7] == '-' {
		return datePortion
	}

	return ""
}

// IsClassified returns true if the classification is CLASSIFIED.
func (c *Classification) IsClassified() bool {
	return c.Type == "CLASSIFIED"
}

// IsUnclassified returns true if the classification is UNCLASSIFIED.
func (c *Classification) IsUnclassified() bool {
	return c.Type == "UNCLASSIFIED"
}

// ClassifyWithMatchResult classifies a file given an existing match result.
// This is useful when you've already performed the prefix matching.
func ClassifyWithMatchResult(filename string, matchResult *matcher.MatchResult) *Classification {
	if !matchResult.Matched {
		return &Classification{
			Type:   "UNCLASSIFIED",
			Reason: NoPrefixMatch,
		}
	}

	remainder := matchResult.Remainder

	// Check if remainder is long enough to contain a date
	if len(remainder) < 10 {
		return &Classification{
			Type:   "UNCLASSIFIED",
			Reason: InvalidDate,
		}
	}

	// Extract and parse the date portion
	datePortion := remainder[:10]
	isoDate, err := dateparser.ParseIsoDate(datePortion)
	if err != nil {
		return &Classification{
			Type:   "UNCLASSIFIED",
			Reason: InvalidDate,
		}
	}

	// Normalize the filename
	matchedPrefix := filename[:len(matchResult.Rule.Prefix)]
	canonicalPrefix := matchResult.Rule.Prefix
	normalisedFilename := normalizer.Normalize(filename, matchedPrefix, canonicalPrefix)

	return &Classification{
		Type:               "CLASSIFIED",
		Year:               isoDate.Year,
		NormalisedFilename: normalisedFilename,
		TargetDirectory:    matchResult.Rule.TargetDirectory,
	}
}

// Helper function to check if a string starts with a valid date pattern
func hasValidDatePrefix(s string) bool {
	if len(s) < 10 {
		return false
	}
	// Check for YYYY-MM-DD pattern structure
	for i := 0; i < 4; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	if s[4] != '-' {
		return false
	}
	for i := 5; i < 7; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	if s[7] != '-' {
		return false
	}
	for i := 8; i < 10; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
