package normalizer

import (
	"strings"
	"testing"
	"unicode"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: sorta-file-organizer, Property 6: Filename Normalization Preserves Structure
// Validates: Requirements 5.3, 5.4

// randomizeCase applies random casing to a string based on a seed
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

// genCasingVariation generates a random casing variation seed
func genCasingVariation() gopter.Gen {
	return gen.Int64()
}

// genRemainder generates a valid remainder (space + date + optional suffix)
func genRemainder() gopter.Gen {
	return genNonEmptyAlphaString().Map(func(s string) string {
		return " 2024-01-15 " + s
	})
}

func TestFilenameNormalizationPreservesStructure(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Prefix is replaced with canonical casing while remainder is preserved exactly", prop.ForAll(
		func(canonicalPrefix string, casingSeed int64, remainder string) bool {
			// Create a non-canonical casing of the prefix
			matchedPrefix := randomizeCase(canonicalPrefix, casingSeed)

			// Create the original filename with non-canonical prefix
			filename := matchedPrefix + remainder

			// Normalize the filename
			normalized := Normalize(filename, matchedPrefix, canonicalPrefix)

			// Property 1: The normalized filename should start with the canonical prefix
			if !strings.HasPrefix(normalized, canonicalPrefix) {
				t.Logf("Expected normalized filename to start with canonical prefix %q, got %q", canonicalPrefix, normalized)
				return false
			}

			// Property 2: The remainder (everything after the prefix) should be preserved exactly
			expectedRemainder := remainder
			actualRemainder := normalized[len(canonicalPrefix):]
			if actualRemainder != expectedRemainder {
				t.Logf("Expected remainder %q, got %q", expectedRemainder, actualRemainder)
				return false
			}

			// Property 3: The total length should be canonical prefix length + remainder length
			expectedLen := len(canonicalPrefix) + len(remainder)
			if len(normalized) != expectedLen {
				t.Logf("Expected length %d, got %d", expectedLen, len(normalized))
				return false
			}

			return true
		},
		genNonEmptyAlphaString(),
		genCasingVariation(),
		genRemainder(),
	))

	properties.TestingRun(t)
}
