package discovery

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestExtractPrefixFromFilename(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		wantPrefix  string
		wantMatched bool
	}{
		{
			name:        "valid invoice filename",
			filename:    "Invoice 2024-01-15 Acme Corp.pdf",
			wantPrefix:  "Invoice",
			wantMatched: true,
		},
		{
			name:        "valid receipt filename",
			filename:    "Receipt 2023-06-15 Local Store.pdf",
			wantPrefix:  "Receipt",
			wantMatched: true,
		},
		{
			name:        "valid statement filename",
			filename:    "Statement 2024-03-31 Q1 Summary.pdf",
			wantPrefix:  "Statement",
			wantMatched: true,
		},
		{
			name:        "prefix with numbers",
			filename:    "Doc123 2024-01-01 Test.pdf",
			wantPrefix:  "Doc123",
			wantMatched: true,
		},
		{
			name:        "filename without extension",
			filename:    "Invoice 2024-01-15 Acme Corp",
			wantPrefix:  "Invoice",
			wantMatched: true,
		},
		{
			name:        "no prefix match - starts with number",
			filename:    "123Invoice 2024-01-15 Acme Corp.pdf",
			wantPrefix:  "",
			wantMatched: false,
		},
		{
			name:        "no prefix match - no date",
			filename:    "Invoice no-date document.pdf",
			wantPrefix:  "",
			wantMatched: false,
		},
		{
			name:        "no prefix match - invalid date format",
			filename:    "Receipt 2024-13-45 invalid date.pdf",
			wantPrefix:  "",
			wantMatched: false,
		},
		{
			name:        "no prefix match - missing space before date",
			filename:    "Invoice2024-01-15 Acme Corp.pdf",
			wantPrefix:  "",
			wantMatched: false,
		},
		{
			name:        "no prefix match - missing space after date",
			filename:    "Invoice 2024-01-15Acme Corp.pdf",
			wantPrefix:  "",
			wantMatched: false,
		},
		{
			name:        "random file without pattern",
			filename:    "random-file.txt",
			wantPrefix:  "",
			wantMatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPrefix, gotMatched := ExtractPrefixFromFilename(tt.filename)
			if gotPrefix != tt.wantPrefix {
				t.Errorf("ExtractPrefixFromFilename() prefix = %v, want %v", gotPrefix, tt.wantPrefix)
			}
			if gotMatched != tt.wantMatched {
				t.Errorf("ExtractPrefixFromFilename() matched = %v, want %v", gotMatched, tt.wantMatched)
			}
		})
	}
}

// Feature: config-auto-discover, Property 6: Prefix Extraction
// Validates: Requirements 6.2, 6.3, 6.4

// genValidPrefix generates a valid prefix (starts with letter, followed by alphanumeric).
func genValidPrefix() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaChar(),
		gen.AlphaString(),
	).Map(func(vals []interface{}) string {
		firstChar := vals[0].(rune)
		rest := vals[1].(string)
		return string(firstChar) + rest
	}).SuchThat(func(s string) bool {
		return len(s) > 0
	})
}

// genValidISODate generates a valid ISO date string (YYYY-MM-DD).
func genValidISODate() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1900, 2100), // year
		gen.IntRange(1, 12),      // month
		gen.IntRange(1, 28),      // day (use 28 to avoid month-specific issues)
	).Map(func(vals []interface{}) string {
		year := vals[0].(int)
		month := vals[1].(int)
		day := vals[2].(int)
		return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	})
}

// genOtherInfo generates the "other info" part of the filename (non-empty).
func genOtherInfo() gopter.Gen {
	return gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0
	})
}

// genFileExtension generates common file extensions.
func genFileExtension() gopter.Gen {
	return gen.OneConstOf(".pdf", ".txt", ".doc", ".xlsx", "")
}

// genValidFilename generates a filename that matches the pattern.
func genValidFilename() gopter.Gen {
	return gopter.CombineGens(
		genValidPrefix(),
		genValidISODate(),
		genOtherInfo(),
		genFileExtension(),
	).Map(func(vals []interface{}) string {
		prefix := vals[0].(string)
		date := vals[1].(string)
		otherInfo := vals[2].(string)
		ext := vals[3].(string)
		return prefix + " " + date + " " + otherInfo + ext
	})
}

// genInvalidFilename generates filenames that should NOT match the pattern.
func genInvalidFilename() gopter.Gen {
	return gen.OneGenOf(
		// Starts with number
		gen.AlphaString().Map(func(s string) string {
			return "123" + s + " 2024-01-15 test.pdf"
		}),
		// No date
		genValidPrefix().Map(func(prefix string) string {
			return prefix + " no-date document.pdf"
		}),
		// Missing space before date
		gopter.CombineGens(genValidPrefix(), genValidISODate()).Map(func(vals []interface{}) string {
			return vals[0].(string) + vals[1].(string) + " test.pdf"
		}),
		// Missing space after date
		gopter.CombineGens(genValidPrefix(), genValidISODate()).Map(func(vals []interface{}) string {
			return vals[0].(string) + " " + vals[1].(string) + "test.pdf"
		}),
		// Invalid month (13)
		genValidPrefix().Map(func(prefix string) string {
			return prefix + " 2024-13-15 test.pdf"
		}),
		// Invalid day (32)
		genValidPrefix().Map(func(prefix string) string {
			return prefix + " 2024-01-32 test.pdf"
		}),
		// Random string without pattern
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0
		}).Map(func(s string) string {
			return s + ".txt"
		}),
	)
}

func TestPrefixExtractionProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)

	// Property: Valid filenames should always extract the correct prefix
	properties.Property("Valid filenames extract correct prefix", prop.ForAll(
		func(prefix string, date string, otherInfo string, ext string) bool {
			filename := prefix + " " + date + " " + otherInfo + ext
			extractedPrefix, matched := ExtractPrefixFromFilename(filename)

			// Should match and extract the exact prefix
			return matched && extractedPrefix == prefix
		},
		genValidPrefix(),
		genValidISODate(),
		genOtherInfo(),
		genFileExtension(),
	))

	// Property: Invalid filenames should not match
	properties.Property("Invalid filenames do not match", prop.ForAll(
		func(filename string) bool {
			_, matched := ExtractPrefixFromFilename(filename)
			return !matched
		},
		genInvalidFilename(),
	))

	// Property: Multiple distinct prefixes can be detected from different filenames
	properties.Property("Multiple distinct prefixes are detected", prop.ForAll(
		func(prefixes []string, date string, otherInfo string) bool {
			if len(prefixes) == 0 {
				return true
			}

			detectedPrefixes := make(map[string]bool)
			for _, prefix := range prefixes {
				filename := prefix + " " + date + " " + otherInfo + ".pdf"
				extractedPrefix, matched := ExtractPrefixFromFilename(filename)
				if matched {
					detectedPrefixes[extractedPrefix] = true
				}
			}

			// All valid prefixes should be detected
			for _, prefix := range prefixes {
				if !detectedPrefixes[prefix] {
					return false
				}
			}
			return true
		},
		gen.SliceOfN(5, genValidPrefix()),
		genValidISODate(),
		genOtherInfo(),
	))

	properties.TestingRun(t)
}

// Feature: discovery-directory-filtering, Property 3: ISO Date Pattern Detection
// Validates: Requirements 2.3

// genValidISODateDirName generates directory names that start with a valid ISO date.
func genValidISODateDirName() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1900, 2100), // year
		gen.IntRange(1, 12),      // month
		gen.IntRange(1, 28),      // day (use 28 to avoid month-specific issues)
		gen.OneConstOf("", " Some Folder", " Documents", " Archive"),
	).Map(func(vals []interface{}) string {
		year := vals[0].(int)
		month := vals[1].(int)
		day := vals[2].(int)
		suffix := vals[3].(string)
		return fmt.Sprintf("%04d-%02d-%02d%s", year, month, day, suffix)
	})
}

// genInvalidISODateDirName generates directory names that do NOT start with a valid ISO date.
func genInvalidISODateDirName() gopter.Gen {
	return gen.OneGenOf(
		// Prefix before date
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0
		}).Map(func(prefix string) string {
			return prefix + " 2024-01-15"
		}),
		// Invalid month (13)
		gen.Const("2024-13-01"),
		// Invalid month (00)
		gen.Const("2024-00-15"),
		// Invalid day (32)
		gen.Const("2024-01-32"),
		// Invalid day (00)
		gen.Const("2024-01-00"),
		// No dashes
		gen.Const("20240115"),
		// Wrong separator
		gen.Const("2024/01/15"),
		// Plain text
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0
		}),
	)
}

func TestIsISODateDirectoryProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)

	// Property 3: ISO Date Pattern Detection
	// For any directory name, IsISODateDirectory returns true if and only if
	// the name starts with a valid ISO date pattern (YYYY-MM-DD where MM is 01-12 and DD is 01-31).

	// Property: Valid ISO date directory names should return true
	properties.Property("Valid ISO date directories are detected", prop.ForAll(
		func(dirName string) bool {
			return IsISODateDirectory(dirName)
		},
		genValidISODateDirName(),
	))

	// Property: Invalid ISO date directory names should return false
	properties.Property("Invalid ISO date directories are not detected", prop.ForAll(
		func(dirName string) bool {
			return !IsISODateDirectory(dirName)
		},
		genInvalidISODateDirName(),
	))

	properties.TestingRun(t)
}

// TestIsISODateDirectory tests the IsISODateDirectory function with specific examples.
// Validates: Requirements 2.3
func TestIsISODateDirectory(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		expected bool
	}{
		// Valid ISO dates - should return true
		{
			name:     "valid ISO date only",
			dirName:  "2024-01-15",
			expected: true,
		},
		{
			name:     "valid ISO date with suffix",
			dirName:  "2024-01-15 Some Folder",
			expected: true,
		},
		{
			name:     "valid ISO date with text suffix",
			dirName:  "2024-12-31 Year End Documents",
			expected: true,
		},
		{
			name:     "valid ISO date first day of month",
			dirName:  "2024-01-01",
			expected: true,
		},
		{
			name:     "valid ISO date last valid day",
			dirName:  "2024-01-31",
			expected: true,
		},
		// Invalid patterns - should return false
		{
			name:     "prefix before date",
			dirName:  "Invoice 2024-01-15",
			expected: false,
		},
		{
			name:     "invalid month 13",
			dirName:  "2024-13-01",
			expected: false,
		},
		{
			name:     "invalid month 00",
			dirName:  "2024-00-15",
			expected: false,
		},
		{
			name:     "invalid day 32",
			dirName:  "2024-01-32",
			expected: false,
		},
		{
			name:     "invalid day 00",
			dirName:  "2024-01-00",
			expected: false,
		},
		{
			name:     "no dashes - compact format",
			dirName:  "20240115",
			expected: false,
		},
		{
			name:     "wrong separator - slashes",
			dirName:  "2024/01/15",
			expected: false,
		},
		{
			name:     "plain text directory",
			dirName:  "Documents",
			expected: false,
		},
		{
			name:     "empty string",
			dirName:  "",
			expected: false,
		},
		{
			name:     "partial date - year only",
			dirName:  "2024",
			expected: false,
		},
		{
			name:     "partial date - year and month",
			dirName:  "2024-01",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsISODateDirectory(tt.dirName)
			if result != tt.expected {
				t.Errorf("IsISODateDirectory(%q) = %v, want %v", tt.dirName, result, tt.expected)
			}
		})
	}
}
