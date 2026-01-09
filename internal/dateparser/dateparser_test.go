package dateparser

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: sorta-file-organizer, Property 4: Valid ISO Date Extraction
// Validates: Requirements 4.1, 4.3

// genValidYear generates years in a reasonable range (1000-9999).
func genValidYear() gopter.Gen {
	return gen.IntRange(1000, 9999)
}

// genValidMonth generates months (1-12).
func genValidMonth() gopter.Gen {
	return gen.IntRange(1, 12)
}

// genValidIsoDate generates a valid ISO date string and its expected components.
type validDateInput struct {
	DateString string
	Year       int
	Month      int
	Day        int
}

func genValidIsoDate() gopter.Gen {
	return gopter.CombineGens(
		genValidYear(),
		genValidMonth(),
	).FlatMap(func(v interface{}) gopter.Gen {
		vals := v.([]interface{})
		year := vals[0].(int)
		month := vals[1].(int)
		maxDay := daysInMonth(year, month)
		return gen.IntRange(1, maxDay).Map(func(day int) validDateInput {
			dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
			return validDateInput{
				DateString: dateStr,
				Year:       year,
				Month:      month,
				Day:        day,
			}
		})
	}, reflect.TypeOf(validDateInput{}))
}

func TestValidIsoDateExtraction(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Valid ISO dates are correctly parsed and extracted", prop.ForAll(
		func(input validDateInput) bool {
			result, err := ParseIsoDate(input.DateString)
			if err != nil {
				t.Logf("ParseIsoDate failed for valid date %s: %v", input.DateString, err)
				return false
			}

			// Verify year extraction
			if result.Year != input.Year {
				t.Logf("Year mismatch: expected %d, got %d", input.Year, result.Year)
				return false
			}

			// Verify month extraction
			if result.Month != input.Month {
				t.Logf("Month mismatch: expected %d, got %d", input.Month, result.Month)
				return false
			}

			// Verify day extraction
			if result.Day != input.Day {
				t.Logf("Day mismatch: expected %d, got %d", input.Day, result.Day)
				return false
			}

			return true
		},
		genValidIsoDate(),
	))

	properties.TestingRun(t)
}
