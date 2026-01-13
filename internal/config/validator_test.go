package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: config-validation, Property 1: Validation Reports All Errors
// Validates: Requirements 1.1, 1.7, 1.8
func TestValidationReportsAllErrors(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Validation reports all errors, not just the first", prop.ForAll(
		func(numMissingInbound int, numDuplicatePrefixes int) bool {
			tmpDir := t.TempDir()

			// Create a config with multiple intentional errors
			cfg := &Configuration{
				InboundDirectories: []string{},
				PrefixRules:        []PrefixRule{},
			}

			// Add non-existent inbound directories
			expectedInboundErrors := 0
			for i := 0; i < numMissingInbound; i++ {
				cfg.InboundDirectories = append(cfg.InboundDirectories,
					filepath.Join(tmpDir, "nonexistent_"+itoa(i)))
				expectedInboundErrors++
			}

			// Add duplicate prefixes (case-insensitive)
			expectedDuplicateErrors := 0
			for i := 0; i < numDuplicatePrefixes; i++ {
				prefix := "Prefix" + itoa(i)
				// Add original
				cfg.PrefixRules = append(cfg.PrefixRules, PrefixRule{
					Prefix:            prefix,
					OutboundDirectory: tmpDir,
				})
				// Add duplicate (different case)
				cfg.PrefixRules = append(cfg.PrefixRules, PrefixRule{
					Prefix:            strings.ToLower(prefix),
					OutboundDirectory: tmpDir,
				})
				expectedDuplicateErrors++
			}

			// Run validation
			result := ValidateConfig(cfg)

			// Count actual errors by type
			inboundErrors := 0
			duplicateErrors := 0
			for _, err := range result.Errors {
				if strings.Contains(err.Field, "inboundDirectories") {
					inboundErrors++
				}
				if strings.Contains(err.Message, "duplicate prefix") {
					duplicateErrors++
				}
			}

			// Verify all errors are reported
			if inboundErrors != expectedInboundErrors {
				t.Logf("Expected %d inbound errors, got %d", expectedInboundErrors, inboundErrors)
				return false
			}
			if duplicateErrors != expectedDuplicateErrors {
				t.Logf("Expected %d duplicate errors, got %d", expectedDuplicateErrors, duplicateErrors)
				return false
			}

			// Verify Valid flag is false when there are errors
			totalExpectedErrors := expectedInboundErrors + expectedDuplicateErrors
			if totalExpectedErrors > 0 && result.Valid {
				t.Log("Expected Valid=false when there are errors")
				return false
			}
			if totalExpectedErrors == 0 && !result.Valid {
				t.Log("Expected Valid=true when there are no errors")
				return false
			}

			return true
		},
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t)
}

// Feature: config-validation, Property 2: Path Existence Validation
// Validates: Requirements 1.2, 1.3
func TestPathExistenceValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Inbound directories must exist and be accessible", prop.ForAll(
		func(numExisting int, numMissing int) bool {
			tmpDir := t.TempDir()

			cfg := &Configuration{
				InboundDirectories: []string{},
				PrefixRules: []PrefixRule{
					{Prefix: "Test", OutboundDirectory: tmpDir},
				},
			}

			// Add existing directories
			for i := 0; i < numExisting; i++ {
				dir := filepath.Join(tmpDir, "existing_"+itoa(i))
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Logf("Failed to create test dir: %v", err)
					return false
				}
				cfg.InboundDirectories = append(cfg.InboundDirectories, dir)
			}

			// Add non-existent directories
			for i := 0; i < numMissing; i++ {
				cfg.InboundDirectories = append(cfg.InboundDirectories,
					filepath.Join(tmpDir, "missing_"+itoa(i)))
			}

			result := ValidateConfig(cfg)

			// Count inbound directory errors
			inboundErrors := 0
			for _, err := range result.Errors {
				if strings.Contains(err.Field, "inboundDirectories") &&
					strings.Contains(err.Message, "does not exist") {
					inboundErrors++
				}
			}

			// Should have exactly numMissing errors for missing directories
			if inboundErrors != numMissing {
				t.Logf("Expected %d missing dir errors, got %d", numMissing, inboundErrors)
				return false
			}

			return true
		},
		gen.IntRange(0, 3),
		gen.IntRange(0, 3),
	))

	properties.Property("Outbound directories must exist or have writable parent", prop.ForAll(
		func(numExisting int, numCreatable int, numUnwritable int) bool {
			tmpDir := t.TempDir()

			cfg := &Configuration{
				InboundDirectories: []string{tmpDir},
				PrefixRules:        []PrefixRule{},
			}

			// Add rules with existing outbound directories
			for i := 0; i < numExisting; i++ {
				dir := filepath.Join(tmpDir, "outbound_existing_"+itoa(i))
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Logf("Failed to create test dir: %v", err)
					return false
				}
				cfg.PrefixRules = append(cfg.PrefixRules, PrefixRule{
					Prefix:            "Existing" + itoa(i),
					OutboundDirectory: dir,
				})
			}

			// Add rules with non-existent but creatable outbound directories
			for i := 0; i < numCreatable; i++ {
				// Parent exists (tmpDir), so this should be creatable
				dir := filepath.Join(tmpDir, "outbound_creatable_"+itoa(i))
				cfg.PrefixRules = append(cfg.PrefixRules, PrefixRule{
					Prefix:            "Creatable" + itoa(i),
					OutboundDirectory: dir,
				})
			}

			// Add rules with non-existent and non-creatable outbound directories
			for i := 0; i < numUnwritable; i++ {
				// Parent doesn't exist
				dir := filepath.Join(tmpDir, "nonexistent_parent_"+itoa(i), "outbound")
				cfg.PrefixRules = append(cfg.PrefixRules, PrefixRule{
					Prefix:            "Unwritable" + itoa(i),
					OutboundDirectory: dir,
				})
			}

			result := ValidateConfig(cfg)

			// Count outbound directory errors
			outboundErrors := 0
			for _, err := range result.Errors {
				if strings.Contains(err.Field, "outboundDirectory") {
					outboundErrors++
				}
			}

			// Should have exactly numUnwritable errors (parent doesn't exist)
			if outboundErrors != numUnwritable {
				t.Logf("Expected %d outbound errors, got %d", numUnwritable, outboundErrors)
				for _, err := range result.Errors {
					t.Logf("  Error: %s - %s", err.Field, err.Message)
				}
				return false
			}

			return true
		},
		gen.IntRange(0, 2),
		gen.IntRange(0, 2),
		gen.IntRange(0, 2),
	))

	properties.TestingRun(t)
}

// Feature: config-validation, Property 3: Duplicate and Overlap Detection
// Validates: Requirements 1.4, 1.5
func TestDuplicateAndOverlapDetection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Duplicate prefixes (case-insensitive) are detected", prop.ForAll(
		func(prefix string, numDuplicates int) bool {
			if len(prefix) == 0 {
				return true // Skip empty prefixes
			}

			tmpDir := t.TempDir()

			cfg := &Configuration{
				InboundDirectories: []string{tmpDir},
				PrefixRules:        []PrefixRule{},
			}

			// Add original prefix
			cfg.PrefixRules = append(cfg.PrefixRules, PrefixRule{
				Prefix:            prefix,
				OutboundDirectory: tmpDir,
			})

			// Add duplicates with different cases
			for i := 0; i < numDuplicates; i++ {
				var dupPrefix string
				if i%2 == 0 {
					dupPrefix = strings.ToUpper(prefix)
				} else {
					dupPrefix = strings.ToLower(prefix)
				}
				cfg.PrefixRules = append(cfg.PrefixRules, PrefixRule{
					Prefix:            dupPrefix,
					OutboundDirectory: filepath.Join(tmpDir, "out_"+itoa(i)),
				})
			}

			result := ValidateConfig(cfg)

			// Count duplicate prefix errors
			duplicateErrors := 0
			for _, err := range result.Errors {
				if strings.Contains(err.Message, "duplicate prefix") {
					duplicateErrors++
				}
			}

			// Should detect all duplicates
			if duplicateErrors != numDuplicates {
				t.Logf("Expected %d duplicate errors, got %d", numDuplicates, duplicateErrors)
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.IntRange(0, 3),
	))

	properties.Property("Overlapping outbound directories are detected", prop.ForAll(
		func(numOverlaps int) bool {
			tmpDir := t.TempDir()

			cfg := &Configuration{
				InboundDirectories: []string{tmpDir},
				PrefixRules:        []PrefixRule{},
			}

			// Add base directory rule
			baseDir := filepath.Join(tmpDir, "base")
			if err := os.MkdirAll(baseDir, 0755); err != nil {
				t.Logf("Failed to create base dir: %v", err)
				return false
			}
			cfg.PrefixRules = append(cfg.PrefixRules, PrefixRule{
				Prefix:            "Base",
				OutboundDirectory: baseDir,
			})

			// Add overlapping subdirectory rules
			for i := 0; i < numOverlaps; i++ {
				subDir := filepath.Join(baseDir, "sub_"+itoa(i))
				if err := os.MkdirAll(subDir, 0755); err != nil {
					t.Logf("Failed to create sub dir: %v", err)
					return false
				}
				cfg.PrefixRules = append(cfg.PrefixRules, PrefixRule{
					Prefix:            "Sub" + itoa(i),
					OutboundDirectory: subDir,
				})
			}

			result := ValidateConfig(cfg)

			// Count overlap errors
			overlapErrors := 0
			for _, err := range result.Errors {
				if strings.Contains(err.Message, "overlapping") {
					overlapErrors++
				}
			}

			// Should detect all overlaps
			if overlapErrors != numOverlaps {
				t.Logf("Expected %d overlap errors, got %d", numOverlaps, overlapErrors)
				return false
			}

			return true
		},
		gen.IntRange(0, 3),
	))

	properties.Property("Non-overlapping directories pass validation", prop.ForAll(
		func(numDirs int) bool {
			tmpDir := t.TempDir()

			cfg := &Configuration{
				InboundDirectories: []string{tmpDir},
				PrefixRules:        []PrefixRule{},
			}

			// Add non-overlapping directories (siblings)
			for i := 0; i < numDirs; i++ {
				dir := filepath.Join(tmpDir, "sibling_"+itoa(i))
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Logf("Failed to create dir: %v", err)
					return false
				}
				cfg.PrefixRules = append(cfg.PrefixRules, PrefixRule{
					Prefix:            "Prefix" + itoa(i),
					OutboundDirectory: dir,
				})
			}

			result := ValidateConfig(cfg)

			// Should have no overlap errors
			overlapErrors := 0
			for _, err := range result.Errors {
				if strings.Contains(err.Message, "overlapping") {
					overlapErrors++
				}
			}

			if overlapErrors != 0 {
				t.Logf("Expected 0 overlap errors for siblings, got %d", overlapErrors)
				return false
			}

			return true
		},
		gen.IntRange(1, 5),
	))

	properties.TestingRun(t)
}

// Feature: config-validation, Property 4: Symlink Policy Validation
// Validates: Requirements 2.1, 2.6
func TestSymlinkPolicyValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Valid symlink policies pass validation", prop.ForAll(
		func(policyIndex int) bool {
			tmpDir := t.TempDir()

			validPolicies := []string{"follow", "skip", "error"}
			policy := validPolicies[policyIndex%len(validPolicies)]

			cfg := &Configuration{
				InboundDirectories: []string{tmpDir},
				PrefixRules: []PrefixRule{
					{Prefix: "Test", OutboundDirectory: tmpDir},
				},
				SymlinkPolicy: policy,
			}

			result := ValidateConfig(cfg)

			// Should have no symlink policy errors
			for _, err := range result.Errors {
				if strings.Contains(err.Field, "symlinkPolicy") {
					t.Logf("Unexpected error for valid policy %q: %s", policy, err.Message)
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 2),
	))

	properties.Property("Invalid symlink policies fail validation", prop.ForAll(
		func(invalidPolicy string) bool {
			// Skip if the generated string happens to be a valid policy
			validPolicies := map[string]bool{"follow": true, "skip": true, "error": true, "": true}
			if validPolicies[invalidPolicy] {
				return true // Skip this case
			}

			tmpDir := t.TempDir()

			cfg := &Configuration{
				InboundDirectories: []string{tmpDir},
				PrefixRules: []PrefixRule{
					{Prefix: "Test", OutboundDirectory: tmpDir},
				},
				SymlinkPolicy: invalidPolicy,
			}

			result := ValidateConfig(cfg)

			// Should have a symlink policy error
			foundError := false
			for _, err := range result.Errors {
				if strings.Contains(err.Field, "symlinkPolicy") {
					foundError = true
					break
				}
			}

			if !foundError {
				t.Logf("Expected error for invalid policy %q, but none found", invalidPolicy)
				return false
			}

			// Result should be invalid
			if result.Valid {
				t.Logf("Expected Valid=false for invalid policy %q", invalidPolicy)
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool {
			return s != "follow" && s != "skip" && s != "error" && s != ""
		}),
	))

	properties.Property("Empty symlink policy passes validation (uses default)", prop.ForAll(
		func(_ int) bool {
			tmpDir := t.TempDir()

			cfg := &Configuration{
				InboundDirectories: []string{tmpDir},
				PrefixRules: []PrefixRule{
					{Prefix: "Test", OutboundDirectory: tmpDir},
				},
				SymlinkPolicy: "", // Empty means use default
			}

			result := ValidateConfig(cfg)

			// Should have no symlink policy errors
			for _, err := range result.Errors {
				if strings.Contains(err.Field, "symlinkPolicy") {
					t.Logf("Unexpected error for empty policy: %s", err.Message)
					return false
				}
			}

			// GetSymlinkPolicy should return default "skip"
			if cfg.GetSymlinkPolicy() != "skip" {
				t.Logf("Expected default policy 'skip', got %q", cfg.GetSymlinkPolicy())
				return false
			}

			return true
		},
		gen.Const(0),
	))

	properties.TestingRun(t)
}

// Feature: config-validation, Property 6: Scan Depth Configuration Validation
// Validates: Requirements 3.1, 3.4, 3.7
func TestScanDepthValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)

	properties.Property("Non-negative scanDepth passes validation", prop.ForAll(
		func(depth int) bool {
			tmpDir := t.TempDir()

			cfg := &Configuration{
				InboundDirectories: []string{tmpDir},
				PrefixRules: []PrefixRule{
					{Prefix: "Test", OutboundDirectory: tmpDir},
				},
				ScanDepth: &depth,
			}

			result := ValidateConfig(cfg)

			// Should have no scanDepth errors for non-negative values
			for _, err := range result.Errors {
				if strings.Contains(err.Field, "scanDepth") {
					t.Logf("Unexpected error for non-negative scanDepth %d: %s", depth, err.Message)
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 100),
	))

	properties.Property("Negative scanDepth fails validation", prop.ForAll(
		func(depth int) bool {
			tmpDir := t.TempDir()

			negativeDepth := -depth - 1 // Ensure negative

			cfg := &Configuration{
				InboundDirectories: []string{tmpDir},
				PrefixRules: []PrefixRule{
					{Prefix: "Test", OutboundDirectory: tmpDir},
				},
				ScanDepth: &negativeDepth,
			}

			result := ValidateConfig(cfg)

			// Should have a scanDepth error
			foundError := false
			for _, err := range result.Errors {
				if strings.Contains(err.Field, "scanDepth") {
					foundError = true
					break
				}
			}

			if !foundError {
				t.Logf("Expected error for negative scanDepth %d, but none found", negativeDepth)
				return false
			}

			// Result should be invalid
			if result.Valid {
				t.Logf("Expected Valid=false for negative scanDepth %d", negativeDepth)
				return false
			}

			return true
		},
		gen.IntRange(0, 100),
	))

	properties.Property("Nil scanDepth passes validation and defaults to 0", prop.ForAll(
		func(_ int) bool {
			tmpDir := t.TempDir()

			cfg := &Configuration{
				InboundDirectories: []string{tmpDir},
				PrefixRules: []PrefixRule{
					{Prefix: "Test", OutboundDirectory: tmpDir},
				},
				ScanDepth: nil, // Not set
			}

			result := ValidateConfig(cfg)

			// Should have no scanDepth errors
			for _, err := range result.Errors {
				if strings.Contains(err.Field, "scanDepth") {
					t.Logf("Unexpected error for nil scanDepth: %s", err.Message)
					return false
				}
			}

			// GetScanDepth should return default 0
			if cfg.GetScanDepth() != 0 {
				t.Logf("Expected default scanDepth 0, got %d", cfg.GetScanDepth())
				return false
			}

			return true
		},
		gen.Const(0),
	))

	properties.TestingRun(t)
}
