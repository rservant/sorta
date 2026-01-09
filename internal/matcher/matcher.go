// Package matcher handles filename prefix matching for Sorta.
package matcher

import (
	"sort"
	"strings"

	"sorta/internal/config"
)

// MatchResult represents the result of matching a filename against prefix rules.
type MatchResult struct {
	Matched   bool
	Rule      *config.PrefixRule
	Remainder string
}

// Match evaluates a filename against prefix rules using case-insensitive matching.
// It returns the longest matching prefix rule, or a non-matched result if no rule matches.
// A match requires the prefix to be followed by a single space delimiter.
func Match(filename string, rules []config.PrefixRule) *MatchResult {
	if len(rules) == 0 {
		return &MatchResult{Matched: false}
	}

	// Sort rules by prefix length descending for longest-match-first
	sortedRules := make([]config.PrefixRule, len(rules))
	copy(sortedRules, rules)
	sort.Slice(sortedRules, func(i, j int) bool {
		return len(sortedRules[i].Prefix) > len(sortedRules[j].Prefix)
	})

	filenameLower := strings.ToLower(filename)

	for i := range sortedRules {
		rule := &sortedRules[i]
		prefixLower := strings.ToLower(rule.Prefix)
		prefixLen := len(rule.Prefix)

		// Check if filename starts with prefix (case-insensitive)
		if !strings.HasPrefix(filenameLower, prefixLower) {
			continue
		}

		// Verify single space delimiter after prefix
		if len(filename) <= prefixLen || filename[prefixLen] != ' ' {
			continue
		}

		// Return match with remainder (everything after prefix and space)
		remainder := filename[prefixLen+1:]
		return &MatchResult{
			Matched:   true,
			Rule:      rule,
			Remainder: remainder,
		}
	}

	return &MatchResult{Matched: false}
}
