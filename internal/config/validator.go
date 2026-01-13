// Package config handles configuration loading and validation for Sorta.
package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ValidationSeverity represents the severity of a validation issue.
type ValidationSeverity string

const (
	SeverityError   ValidationSeverity = "error"
	SeverityWarning ValidationSeverity = "warning"
)

// ConfigValidationError represents a single validation issue.
type ConfigValidationError struct {
	Field    string             // Config field with issue (e.g., "inboundDirectories[0]")
	Message  string             // Human-readable description
	Severity ValidationSeverity // "error" or "warning"
}

// ValidationResult contains all validation findings.
type ValidationResult struct {
	Errors   []ConfigValidationError
	Warnings []ConfigValidationError
	Valid    bool // True if no errors (warnings OK)
}

// ValidateConfig checks the configuration for errors and returns all findings.
func ValidateConfig(cfg *Configuration) *ValidationResult {
	result := &ValidationResult{
		Errors:   []ConfigValidationError{},
		Warnings: []ConfigValidationError{},
		Valid:    true,
	}

	// Validate paths
	pathErrors := ValidatePaths(cfg)
	for _, err := range pathErrors {
		if err.Severity == SeverityError {
			result.Errors = append(result.Errors, err)
		} else {
			result.Warnings = append(result.Warnings, err)
		}
	}

	// Validate prefix rules
	ruleErrors := ValidatePrefixRules(cfg)
	for _, err := range ruleErrors {
		if err.Severity == SeverityError {
			result.Errors = append(result.Errors, err)
		} else {
			result.Warnings = append(result.Warnings, err)
		}
	}

	// Validate policies
	policyErrors := ValidatePolicies(cfg)
	for _, err := range policyErrors {
		if err.Severity == SeverityError {
			result.Errors = append(result.Errors, err)
		} else {
			result.Warnings = append(result.Warnings, err)
		}
	}

	// Set Valid based on whether there are any errors
	result.Valid = len(result.Errors) == 0

	return result
}

// ValidatePaths checks that all configured paths exist or are creatable.
func ValidatePaths(cfg *Configuration) []ConfigValidationError {
	var errors []ConfigValidationError

	// Check inbound directories exist and are accessible
	for i, dir := range cfg.InboundDirectories {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				errors = append(errors, ConfigValidationError{
					Field:    formatField("inboundDirectories", i),
					Message:  "directory does not exist: " + dir,
					Severity: SeverityError,
				})
			} else if os.IsPermission(err) {
				errors = append(errors, ConfigValidationError{
					Field:    formatField("inboundDirectories", i),
					Message:  "directory is not accessible: " + dir,
					Severity: SeverityError,
				})
			} else {
				errors = append(errors, ConfigValidationError{
					Field:    formatField("inboundDirectories", i),
					Message:  "error accessing directory: " + err.Error(),
					Severity: SeverityError,
				})
			}
			continue
		}

		if !info.IsDir() {
			errors = append(errors, ConfigValidationError{
				Field:    formatField("inboundDirectories", i),
				Message:  "path is not a directory: " + dir,
				Severity: SeverityError,
			})
		}
	}

	// Check outbound directories exist or parent is writable
	for i, rule := range cfg.PrefixRules {
		outDir := rule.OutboundDirectory
		info, err := os.Stat(outDir)

		if err == nil {
			// Directory exists, check if it's actually a directory
			if !info.IsDir() {
				errors = append(errors, ConfigValidationError{
					Field:    formatField("prefixRules", i) + ".outboundDirectory",
					Message:  "path exists but is not a directory: " + outDir,
					Severity: SeverityError,
				})
			}
			continue
		}

		if !os.IsNotExist(err) {
			// Some other error accessing the path
			errors = append(errors, ConfigValidationError{
				Field:    formatField("prefixRules", i) + ".outboundDirectory",
				Message:  "error accessing directory: " + err.Error(),
				Severity: SeverityError,
			})
			continue
		}

		// Directory doesn't exist, check if parent is writable
		parentDir := filepath.Dir(outDir)
		parentInfo, parentErr := os.Stat(parentDir)

		if parentErr != nil {
			if os.IsNotExist(parentErr) {
				errors = append(errors, ConfigValidationError{
					Field:    formatField("prefixRules", i) + ".outboundDirectory",
					Message:  "parent directory does not exist: " + parentDir,
					Severity: SeverityError,
				})
			} else {
				errors = append(errors, ConfigValidationError{
					Field:    formatField("prefixRules", i) + ".outboundDirectory",
					Message:  "error accessing parent directory: " + parentErr.Error(),
					Severity: SeverityError,
				})
			}
			continue
		}

		if !parentInfo.IsDir() {
			errors = append(errors, ConfigValidationError{
				Field:    formatField("prefixRules", i) + ".outboundDirectory",
				Message:  "parent path is not a directory: " + parentDir,
				Severity: SeverityError,
			})
			continue
		}

		// Check if parent directory is writable by attempting to create a temp file
		if !isDirectoryWritable(parentDir) {
			errors = append(errors, ConfigValidationError{
				Field:    formatField("prefixRules", i) + ".outboundDirectory",
				Message:  "parent directory is not writable: " + parentDir,
				Severity: SeverityError,
			})
		}
	}

	return errors
}

// formatField creates a field reference string for validation errors.
func formatField(name string, index int) string {
	return name + "[" + itoa(index) + "]"
}

// itoa converts an integer to a string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var result []byte
	negative := i < 0
	if negative {
		i = -i
	}
	for i > 0 {
		result = append([]byte{byte('0' + i%10)}, result...)
		i /= 10
	}
	if negative {
		result = append([]byte{'-'}, result...)
	}
	return string(result)
}

// isDirectoryWritable checks if a directory is writable by attempting to create a temp file.
func isDirectoryWritable(dir string) bool {
	testFile := filepath.Join(dir, ".sorta_write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}

// ValidatePrefixRules checks for duplicate prefixes and overlapping outbound directories.
func ValidatePrefixRules(cfg *Configuration) []ConfigValidationError {
	var errors []ConfigValidationError

	// Check for duplicate prefixes (case-insensitive)
	prefixMap := make(map[string]int) // lowercase prefix -> first index
	for i, rule := range cfg.PrefixRules {
		lowerPrefix := strings.ToLower(rule.Prefix)
		if firstIdx, exists := prefixMap[lowerPrefix]; exists {
			errors = append(errors, ConfigValidationError{
				Field:    formatField("prefixRules", i) + ".prefix",
				Message:  "duplicate prefix (case-insensitive): \"" + rule.Prefix + "\" conflicts with rule at index " + itoa(firstIdx),
				Severity: SeverityError,
			})
		} else {
			prefixMap[lowerPrefix] = i
		}
	}

	// Check for overlapping outbound directories
	// Two directories overlap if one is a parent/ancestor of the other
	for i := 0; i < len(cfg.PrefixRules); i++ {
		for j := i + 1; j < len(cfg.PrefixRules); j++ {
			dir1 := cfg.PrefixRules[i].OutboundDirectory
			dir2 := cfg.PrefixRules[j].OutboundDirectory

			if directoriesOverlap(dir1, dir2) {
				errors = append(errors, ConfigValidationError{
					Field:    formatField("prefixRules", j) + ".outboundDirectory",
					Message:  "overlapping outbound directory: \"" + dir2 + "\" overlaps with \"" + dir1 + "\" at index " + itoa(i),
					Severity: SeverityError,
				})
			}
		}
	}

	return errors
}

// directoriesOverlap checks if two directories overlap (one is parent/ancestor of the other).
func directoriesOverlap(dir1, dir2 string) bool {
	// Clean and normalize paths
	clean1 := filepath.Clean(dir1)
	clean2 := filepath.Clean(dir2)

	// Exact match
	if clean1 == clean2 {
		return true
	}

	// Check if one is a prefix of the other (with path separator)
	// dir1 is parent of dir2
	if strings.HasPrefix(clean2, clean1+string(filepath.Separator)) {
		return true
	}

	// dir2 is parent of dir1
	if strings.HasPrefix(clean1, clean2+string(filepath.Separator)) {
		return true
	}

	return false
}

// ValidatePolicies checks that policy values are valid.
func ValidatePolicies(cfg *Configuration) []ConfigValidationError {
	var errors []ConfigValidationError

	// Validate symlink policy if set
	if cfg.SymlinkPolicy != "" {
		validPolicies := map[string]bool{
			SymlinkPolicyFollow: true,
			SymlinkPolicySkip:   true,
			SymlinkPolicyError:  true,
		}
		if !validPolicies[cfg.SymlinkPolicy] {
			errors = append(errors, ConfigValidationError{
				Field:    "symlinkPolicy",
				Message:  "invalid symlink policy: \"" + cfg.SymlinkPolicy + "\". Must be \"follow\", \"skip\", or \"error\"",
				Severity: SeverityError,
			})
		}
	}

	// Validate scanDepth is non-negative if set
	if cfg.ScanDepth != nil && *cfg.ScanDepth < 0 {
		errors = append(errors, ConfigValidationError{
			Field:    "scanDepth",
			Message:  "scanDepth must be a non-negative integer",
			Severity: SeverityError,
		})
	}

	return errors
}
