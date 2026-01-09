package classifier

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"sorta/internal/config"
)

// Feature: sorta-file-organizer, Property 5: Invalid Date Classification
// Validates: Requirements 4.4

// genNonEmptyAlphaString generates non-empty alphabetic strings (1+ chars).
func genNonEmptyAlphaString() gopter.Gen {
	return gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) >= 1
	})
}

// genPrefix generates a prefix string suitable for testing (3-10 chars).
func genPrefix() gopter.Gen {
	return gen.SliceOfN(5, gen.AlphaChar()).Map(func(chars []rune) string {
		return string(chars)
	})
}

// genInvalidDateString generates strings that look like dates but are invalid.
func genInvalidDateString() gopter.Gen {
	return gen.OneGenOf(
		// Wrong format (not YYYY-MM-DD)
		gen.Const("2024/01/15"),
		gen.Const("01-15-2024"),
		gen.Const("15-01-2024"),
		gen.Const("2024-1-15"),
		gen.Const("2024-01-5"),
		gen.Const("24-01-15"),
		// Invalid month (00, 13+)
		gen.IntRange(1000, 9999).Map(func(year int) string {
			return fmt.Sprintf("%04d-00-15", year)
		}),
		gen.IntRange(1000, 9999).Map(func(year int) string {
			return fmt.Sprintf("%04d-13-15", year)
		}),
		gen.IntRange(1000, 9999).Map(func(year int) string {
			return fmt.Sprintf("%04d-99-15", year)
		}),
		// Invalid day (00, 32+, or invalid for month)
		gen.IntRange(1000, 9999).Map(func(year int) string {
			return fmt.Sprintf("%04d-01-00", year)
		}),
		gen.IntRange(1000, 9999).Map(func(year int) string {
			return fmt.Sprintf("%04d-01-32", year)
		}),
		gen.IntRange(1000, 9999).Map(func(year int) string {
			return fmt.Sprintf("%04d-04-31", year)
		}),
		gen.IntRange(1000, 9999).Map(func(year int) string {
			return fmt.Sprintf("%04d-02-30", year)
		}),
		// Day 29 in February non-leap year
		gen.Const("2023-02-29"),
	)
}

// genSuffix generates an optional suffix for filenames.
func genSuffix() gopter.Gen {
	return gen.AlphaString()
}

func TestInvalidDateClassification(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Files with invalid dates are classified as UNCLASSIFIED with INVALID_DATE reason", prop.ForAll(
		func(prefix string, invalidDate string, suffix string) bool {
			// Build filename: prefix + space + invalidDate + space + suffix
			filename := prefix + " " + invalidDate + " " + suffix

			// Create a rule that matches the prefix
			rules := []config.PrefixRule{
				{
					Prefix:          prefix,
					TargetDirectory: "/target",
				},
			}

			result := Classify(filename, rules)

			// Should be UNCLASSIFIED
			if result.Type != "UNCLASSIFIED" {
				t.Logf("Expected UNCLASSIFIED for filename %q, got %s", filename, result.Type)
				return false
			}

			// Reason should be INVALID_DATE
			if result.Reason != InvalidDate {
				t.Logf("Expected reason INVALID_DATE for filename %q, got %s", filename, result.Reason)
				return false
			}

			return true
		},
		genPrefix(),
		genInvalidDateString(),
		genSuffix(),
	))

	properties.TestingRun(t)
}

// Feature: sorta-file-organizer, Property 10: Deterministic Classification
// Validates: Requirements 7.5

// genValidDateString generates a valid ISO date string.
func genValidDateString() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1000, 9999), // year
		gen.IntRange(1, 12),      // month
		gen.IntRange(1, 28),      // day (safe for all months)
	).Map(func(vals []interface{}) string {
		year := vals[0].(int)
		month := vals[1].(int)
		day := vals[2].(int)
		return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	})
}

// daysInMonth returns the number of days in the given month for the given year.
func daysInMonth(year, month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if isLeapYear(year) {
			return 29
		}
		return 28
	default:
		return 0
	}
}

// isLeapYear returns true if the given year is a leap year.
func isLeapYear(year int) bool {
	return (year%4 == 0 && year%100 != 0) || (year%400 == 0)
}

// genFilename generates either a valid or invalid filename for classification testing.
func genFilename() gopter.Gen {
	return gen.OneGenOf(
		// Valid filename with matching prefix and valid date
		gopter.CombineGens(
			genPrefix(),
			genValidDateString(),
			genSuffix(),
		).Map(func(vals []interface{}) string {
			prefix := vals[0].(string)
			date := vals[1].(string)
			suffix := vals[2].(string)
			return prefix + " " + date + " " + suffix
		}),
		// Filename with no matching prefix
		gen.AlphaString().Map(func(s string) string {
			return "NOMATCH_" + s
		}),
		// Filename with invalid date
		gopter.CombineGens(
			genPrefix(),
			genInvalidDateString(),
			genSuffix(),
		).Map(func(vals []interface{}) string {
			prefix := vals[0].(string)
			date := vals[1].(string)
			suffix := vals[2].(string)
			return prefix + " " + date + " " + suffix
		}),
	)
}

// genPrefixRules generates a list of prefix rules.
func genPrefixRules() gopter.Gen {
	return gen.SliceOfN(3, genPrefix()).Map(func(prefixes []string) []config.PrefixRule {
		rules := make([]config.PrefixRule, len(prefixes))
		for i, p := range prefixes {
			rules[i] = config.PrefixRule{
				Prefix:          p,
				TargetDirectory: fmt.Sprintf("/target/%d", i),
			}
		}
		return rules
	})
}

func TestDeterministicClassification(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Classification produces identical results on every invocation", prop.ForAll(
		func(filename string, rules []config.PrefixRule) bool {
			// Run classification multiple times
			const iterations = 5

			firstResult := Classify(filename, rules)

			for i := 1; i < iterations; i++ {
				result := Classify(filename, rules)

				// Check that all fields match
				if result.Type != firstResult.Type {
					t.Logf("Iteration %d: Type mismatch - expected %s, got %s", i, firstResult.Type, result.Type)
					return false
				}

				if result.Type == "CLASSIFIED" {
					if result.Year != firstResult.Year {
						t.Logf("Iteration %d: Year mismatch - expected %d, got %d", i, firstResult.Year, result.Year)
						return false
					}
					if result.NormalisedFilename != firstResult.NormalisedFilename {
						t.Logf("Iteration %d: NormalisedFilename mismatch - expected %q, got %q", i, firstResult.NormalisedFilename, result.NormalisedFilename)
						return false
					}
					if result.TargetDirectory != firstResult.TargetDirectory {
						t.Logf("Iteration %d: TargetDirectory mismatch - expected %q, got %q", i, firstResult.TargetDirectory, result.TargetDirectory)
						return false
					}
				} else {
					if result.Reason != firstResult.Reason {
						t.Logf("Iteration %d: Reason mismatch - expected %s, got %s", i, firstResult.Reason, result.Reason)
						return false
					}
				}
			}

			return true
		},
		genFilename(),
		genPrefixRules(),
	))

	properties.TestingRun(t)
}
