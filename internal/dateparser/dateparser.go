// Package dateparser handles ISO date parsing and validation for Sorta.
package dateparser

import (
	"fmt"
	"regexp"
	"strconv"
)

// DateParseErrorType represents the type of date parsing error.
type DateParseErrorType string

const (
	InvalidFormat DateParseErrorType = "INVALID_FORMAT"
	InvalidDate   DateParseErrorType = "INVALID_DATE"
)

// DateParseError represents an error that occurred during date parsing.
type DateParseError struct {
	Type   DateParseErrorType
	Reason string
}

func (e *DateParseError) Error() string {
	switch e.Type {
	case InvalidFormat:
		return "invalid date format: expected YYYY-MM-DD"
	case InvalidDate:
		return fmt.Sprintf("invalid date: %s", e.Reason)
	default:
		return fmt.Sprintf("date parse error: %s", e.Reason)
	}
}

// IsoDate represents a parsed ISO date with year, month, and day components.
type IsoDate struct {
	Year  int
	Month int
	Day   int
}

// isoDatePattern matches the YYYY-MM-DD format strictly.
var isoDatePattern = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)

// ParseIsoDate parses a string in YYYY-MM-DD format and returns an IsoDate.
// It validates the format strictly and checks that the date is valid
// (correct month range, day range for the month, and leap year handling).
func ParseIsoDate(segment string) (*IsoDate, error) {
	matches := isoDatePattern.FindStringSubmatch(segment)
	if matches == nil {
		return nil, &DateParseError{Type: InvalidFormat}
	}

	// Parse year, month, day from regex groups
	year, _ := strconv.Atoi(matches[1])
	month, _ := strconv.Atoi(matches[2])
	day, _ := strconv.Atoi(matches[3])

	// Validate month range (01-12)
	if month < 1 || month > 12 {
		return nil, &DateParseError{
			Type:   InvalidDate,
			Reason: fmt.Sprintf("month %02d is out of range (01-12)", month),
		}
	}

	// Validate day range for the given month
	maxDay := daysInMonth(year, month)
	if day < 1 || day > maxDay {
		return nil, &DateParseError{
			Type:   InvalidDate,
			Reason: fmt.Sprintf("day %02d is out of range for month %02d (01-%02d)", day, month, maxDay),
		}
	}

	return &IsoDate{
		Year:  year,
		Month: month,
		Day:   day,
	}, nil
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
