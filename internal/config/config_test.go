package config

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: sorta-file-organizer, Property 1: Configuration Round-Trip
// Validates: Requirements 8.3

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
		genNonEmptyString(),
	).Map(func(vals []interface{}) *Configuration {
		return &Configuration{
			SourceDirectories:  vals[0].([]string),
			PrefixRules:        vals[1].([]PrefixRule),
			ForReviewDirectory: vals[2].(string),
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

			// Load it back
			loaded, err := Load(tmpFile)
			if err != nil {
				t.Logf("Load failed: %v", err)
				return false
			}

			// Compare the configurations
			return reflect.DeepEqual(config, loaded)
		},
		genConfiguration(),
	))

	properties.TestingRun(t)
}
