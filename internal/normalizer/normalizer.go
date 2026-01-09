// Package normalizer handles filename normalization for Sorta.
package normalizer

// Normalize rewrites a filename with the canonical prefix casing.
// It replaces the matched prefix portion with the canonical casing
// while preserving the space delimiter and all characters following the prefix exactly.
//
// Parameters:
//   - filename: the original filename
//   - matchedPrefix: the prefix as it appears in the filename (may have different casing)
//   - canonicalPrefix: the canonical casing of the prefix from configuration
//
// Returns the normalized filename with canonical prefix casing.
func Normalize(filename string, matchedPrefix string, canonicalPrefix string) string {
	// The matched prefix length tells us how many characters to replace
	prefixLen := len(matchedPrefix)

	// Replace the prefix portion with canonical casing, preserve everything after
	remainder := filename[prefixLen:]

	return canonicalPrefix + remainder
}
