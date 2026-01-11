package matcher

import (
	"testing"
	"unicode"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"sorta/internal/config"
)

// Feature: sorta-file-organizer, Property 2: Case-Insensitive Prefix Matching
// Validates: Requirements 3.1, 3.2, 3.3

// randomizeCase applies random casing to a string
func randomizeCase(s string, seed int64) string {
	runes := []rune(s)
	for i := range runes {
		if (seed>>uint(i%64))&1 == 1 {
			runes[i] = unicode.ToUpper(runes[i])
		} else {
			runes[i] = unicode.ToLower(runes[i])
		}
	}
	return string(runes)
}

// genNonEmptyAlphaString generates non-empty alphabetic strings
func genNonEmptyAlphaString() gopter.Gen {
	return gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0
	})
}

// genPrefixRule generates a valid PrefixRule
func genPrefixRule() gopter.Gen {
	return gopter.CombineGens(
		genNonEmptyAlphaString(),
		genNonEmptyAlphaString(),
	).Map(func(vals []interface{}) config.PrefixRule {
		return config.PrefixRule{
			Prefix:            vals[0].(string),
			OutboundDirectory: vals[1].(string),
		}
	})
}

// genCasingVariation generates a random casing variation seed
func genCasingVariation() gopter.Gen {
	return gen.Int64()
}

// genRemainder generates a valid remainder (date + optional suffix)
func genRemainder() gopter.Gen {
	return genNonEmptyAlphaString().Map(func(s string) string {
		return "2024-01-15 " + s
	})
}

func TestCaseInsensitivePrefixMatching(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Match succeeds regardless of prefix casing", prop.ForAll(
		func(rule config.PrefixRule, casingSeed int64, remainder string) bool {
			// Create filename with random casing of the prefix
			randomCasedPrefix := randomizeCase(rule.Prefix, casingSeed)
			filename := randomCasedPrefix + " " + remainder

			rules := []config.PrefixRule{rule}
			result := Match(filename, rules)

			// Should match regardless of casing
			if !result.Matched {
				t.Logf("Expected match for filename %q with prefix %q", filename, rule.Prefix)
				return false
			}

			// Should return the correct rule
			if result.Rule.Prefix != rule.Prefix {
				t.Logf("Expected rule prefix %q, got %q", rule.Prefix, result.Rule.Prefix)
				return false
			}

			// Remainder should be everything after prefix and space
			if result.Remainder != remainder {
				t.Logf("Expected remainder %q, got %q", remainder, result.Remainder)
				return false
			}

			return true
		},
		genPrefixRule(),
		genCasingVariation(),
		genRemainder(),
	))

	properties.TestingRun(t)
}

// Feature: sorta-file-organizer, Property 3: Longest Prefix Wins
// Validates: Requirements 3.4

func TestLongestPrefixWins(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Longest matching prefix is selected", prop.ForAll(
		func(basePrefix string, extension string, remainder string) bool {
			// Create two overlapping prefixes: short and long
			shortPrefix := basePrefix
			longPrefix := basePrefix + extension

			shortRule := config.PrefixRule{
				Prefix:            shortPrefix,
				OutboundDirectory: "/short",
			}
			longRule := config.PrefixRule{
				Prefix:            longPrefix,
				OutboundDirectory: "/long",
			}

			// Create filename that matches the longer prefix
			filename := longPrefix + " " + remainder

			// Test with rules in both orders to ensure sorting works
			rulesShortFirst := []config.PrefixRule{shortRule, longRule}
			rulesLongFirst := []config.PrefixRule{longRule, shortRule}

			resultShortFirst := Match(filename, rulesShortFirst)
			resultLongFirst := Match(filename, rulesLongFirst)

			// Both should match the longer prefix
			if !resultShortFirst.Matched || !resultLongFirst.Matched {
				t.Logf("Expected match for filename %q", filename)
				return false
			}

			if resultShortFirst.Rule.Prefix != longPrefix {
				t.Logf("Short-first: Expected prefix %q, got %q", longPrefix, resultShortFirst.Rule.Prefix)
				return false
			}

			if resultLongFirst.Rule.Prefix != longPrefix {
				t.Logf("Long-first: Expected prefix %q, got %q", longPrefix, resultLongFirst.Rule.Prefix)
				return false
			}

			// Remainder should be the same
			if resultShortFirst.Remainder != remainder || resultLongFirst.Remainder != remainder {
				t.Logf("Expected remainder %q", remainder)
				return false
			}

			return true
		},
		genNonEmptyAlphaString(),
		genNonEmptyAlphaString(),
		genRemainder(),
	))

	properties.TestingRun(t)
}
