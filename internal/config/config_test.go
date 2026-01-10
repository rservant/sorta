package config

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: config-auto-discover, Property 11: Configuration Round-Trip
// Validates: Requirements 11.2

// genNonEmptyString generates non-empty strings for configuration fields.
func genNonEmptyString() gopter.Gen {
	return gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0
	})
}

// genPrefixRule generates a valid PrefixRule.
func genPrefixRule() gopter.Gen {
	return gopter.CombineGens(
		genNonEmptyString(),
		genNonEmptyString(),
	).Map(func(vals []interface{}) PrefixRule {
		return PrefixRule{
			Prefix:          vals[0].(string),
			TargetDirectory: vals[1].(string),
		}
	})
}

// genConfiguration generates a valid Configuration object.
func genConfiguration() gopter.Gen {
	return gopter.CombineGens(
		gen.SliceOfN(3, genNonEmptyString()).SuchThat(func(s []string) bool {
			return len(s) > 0
		}),
		gen.SliceOfN(3, genPrefixRule()).SuchThat(func(rules []PrefixRule) bool {
			return len(rules) > 0
		}),
	).Map(func(vals []interface{}) *Configuration {
		return &Configuration{
			SourceDirectories: vals[0].([]string),
			PrefixRules:       vals[1].([]PrefixRule),
		}
	})
}

func TestConfigurationRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Configuration round-trip preserves data", prop.ForAll(
		func(config *Configuration) bool {
			// Create a temporary file for the test
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "config.json")

			// Save the configuration
			if err := Save(config, tmpFile); err != nil {
				t.Logf("Save failed: %v", err)
				return false
			}

			// Load it back using LoadOrCreate (doesn't validate)
			loaded, err := LoadOrCreate(tmpFile)
			if err != nil {
				t.Logf("LoadOrCreate failed: %v", err)
				return false
			}

			// Compare the configurations
			return reflect.DeepEqual(config, loaded)
		},
		genConfiguration(),
	))

	properties.TestingRun(t)
}

// Feature: config-auto-discover, Property 3: Source Directory Duplicate Prevention
// Validates: Requirements 3.4
func TestSourceDirectoryDuplicatePrevention(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("AddSourceDirectory does not add duplicate directories", prop.ForAll(
		func(existingDirs []string, newDir string) bool {
			// Create a configuration with existing directories
			config := &Configuration{
				SourceDirectories: existingDirs,
				PrefixRules:       []PrefixRule{},
			}

			// Check if the directory already exists
			alreadyExists := config.HasSourceDirectory(newDir)
			originalLen := len(config.SourceDirectories)

			// Try to add the directory
			added := config.AddSourceDirectory(newDir)

			if alreadyExists {
				// If it already existed, it should not be added
				return !added && len(config.SourceDirectories) == originalLen
			}
			// If it didn't exist, it should be added
			return added && len(config.SourceDirectories) == originalLen+1 && config.HasSourceDirectory(newDir)
		},
		gen.SliceOf(genNonEmptyString()),
		genNonEmptyString(),
	))

	properties.TestingRun(t)
}

// Feature: config-auto-discover, Property 7: Prefix Rule Duplicate Prevention
// Validates: Requirements 7.3, 7.4, 7.5
func TestPrefixRuleDuplicatePrevention(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("AddPrefixRule does not add duplicate prefixes (case-insensitive)", prop.ForAll(
		func(existingRules []PrefixRule, newRule PrefixRule) bool {
			// Create a configuration with existing rules
			config := &Configuration{
				SourceDirectories: []string{},
				PrefixRules:       existingRules,
			}

			// Check if the prefix already exists (case-insensitive)
			alreadyExists := config.HasPrefix(newRule.Prefix)
			originalLen := len(config.PrefixRules)

			// Try to add the rule
			added := config.AddPrefixRule(newRule)

			if alreadyExists {
				// If it already existed, it should not be added and existing rules unchanged
				return !added && len(config.PrefixRules) == originalLen
			}
			// If it didn't exist, it should be added
			return added && len(config.PrefixRules) == originalLen+1 && config.HasPrefix(newRule.Prefix)
		},
		gen.SliceOf(genPrefixRule()),
		genPrefixRule(),
	))

	properties.TestingRun(t)
}
