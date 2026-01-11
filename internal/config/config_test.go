package config

import (
	"os"
	"path/filepath"
	"reflect"
	"sorta/internal/audit"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: rename-source-target, Property 1: Configuration JSON Round-Trip
// Validates: Requirements 1.1, 1.2, 1.3, 1.4

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
			Prefix:            vals[0].(string),
			OutboundDirectory: vals[1].(string),
		}
	})
}

// genAuditConfig generates a valid AuditConfig.
func genAuditConfig() gopter.Gen {
	return gopter.CombineGens(
		genNonEmptyString(),                   // LogDirectory
		gen.Int64Range(1024, 100*1024*1024),   // RotationSize (1KB to 100MB)
		gen.OneConstOf("", "daily", "weekly"), // RotationPeriod
		gen.IntRange(0, 365),                  // RetentionDays
		gen.IntRange(0, 1000),                 // RetentionRuns
		gen.IntRange(1, 30),                   // MinRetentionDays
	).Map(func(vals []interface{}) *audit.AuditConfig {
		return &audit.AuditConfig{
			LogDirectory:     vals[0].(string),
			RotationSize:     vals[1].(int64),
			RotationPeriod:   vals[2].(string),
			RetentionDays:    vals[3].(int),
			RetentionRuns:    vals[4].(int),
			MinRetentionDays: vals[5].(int),
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
		genAuditConfig(),
	).Map(func(vals []interface{}) *Configuration {
		return &Configuration{
			InboundDirectories: vals[0].([]string),
			PrefixRules:        vals[1].([]PrefixRule),
			Audit:              vals[2].(*audit.AuditConfig),
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

// Feature: rename-source-target, Property 2: Inbound Directory Duplicate Prevention
// Validates: Requirements 2.3, 3.2
func TestInboundDirectoryDuplicatePrevention(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("AddInboundDirectory does not add duplicate directories", prop.ForAll(
		func(existingDirs []string, newDir string) bool {
			// Create a configuration with existing directories
			config := &Configuration{
				InboundDirectories: existingDirs,
				PrefixRules:        []PrefixRule{},
			}

			// Check if the directory already exists
			alreadyExists := config.HasInboundDirectory(newDir)
			originalLen := len(config.InboundDirectories)

			// Try to add the directory
			added := config.AddInboundDirectory(newDir)

			if alreadyExists {
				// If it already existed, it should not be added
				return !added && len(config.InboundDirectories) == originalLen
			}
			// If it didn't exist, it should be added
			return added && len(config.InboundDirectories) == originalLen+1 && config.HasInboundDirectory(newDir)
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
				InboundDirectories: []string{},
				PrefixRules:        existingRules,
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

// Feature: audit-trail, Task 22.2: Unit tests for audit config parsing
// Validates: Requirements 9.1, 10.1

func TestAuditConfigDefaultsAppliedWhenMissing(t *testing.T) {
	// Create a config file without audit section
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"inboundDirectories": ["source1"],
		"prefixRules": [{"prefix": "Invoice", "outboundDirectory": "invoices"}]
	}`

	if err := os.WriteFile(tmpFile, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the config
	config, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify audit defaults are applied
	if config.Audit == nil {
		t.Fatal("Expected Audit config to be set with defaults, got nil")
	}

	defaults := audit.DefaultAuditConfig()

	if config.Audit.LogDirectory != defaults.LogDirectory {
		t.Errorf("LogDirectory: expected %q, got %q", defaults.LogDirectory, config.Audit.LogDirectory)
	}
	if config.Audit.RotationSize != defaults.RotationSize {
		t.Errorf("RotationSize: expected %d, got %d", defaults.RotationSize, config.Audit.RotationSize)
	}
	if config.Audit.RetentionDays != defaults.RetentionDays {
		t.Errorf("RetentionDays: expected %d, got %d", defaults.RetentionDays, config.Audit.RetentionDays)
	}
	if config.Audit.MinRetentionDays != defaults.MinRetentionDays {
		t.Errorf("MinRetentionDays: expected %d, got %d", defaults.MinRetentionDays, config.Audit.MinRetentionDays)
	}
}

func TestAuditConfigCustomValuesOverrideDefaults(t *testing.T) {
	// Create a config file with custom audit section
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"inboundDirectories": ["source1"],
		"prefixRules": [{"prefix": "Invoice", "outboundDirectory": "invoices"}],
		"audit": {
			"logDirectory": "/custom/audit/logs",
			"rotationSizeBytes": 52428800,
			"rotationPeriod": "daily",
			"retentionDays": 90,
			"retentionRuns": 100,
			"minRetentionDays": 14
		}
	}`

	if err := os.WriteFile(tmpFile, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the config
	config, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify custom values are preserved
	if config.Audit == nil {
		t.Fatal("Expected Audit config to be set, got nil")
	}

	if config.Audit.LogDirectory != "/custom/audit/logs" {
		t.Errorf("LogDirectory: expected %q, got %q", "/custom/audit/logs", config.Audit.LogDirectory)
	}
	if config.Audit.RotationSize != 52428800 {
		t.Errorf("RotationSize: expected %d, got %d", 52428800, config.Audit.RotationSize)
	}
	if config.Audit.RotationPeriod != "daily" {
		t.Errorf("RotationPeriod: expected %q, got %q", "daily", config.Audit.RotationPeriod)
	}
	if config.Audit.RetentionDays != 90 {
		t.Errorf("RetentionDays: expected %d, got %d", 90, config.Audit.RetentionDays)
	}
	if config.Audit.RetentionRuns != 100 {
		t.Errorf("RetentionRuns: expected %d, got %d", 100, config.Audit.RetentionRuns)
	}
	if config.Audit.MinRetentionDays != 14 {
		t.Errorf("MinRetentionDays: expected %d, got %d", 14, config.Audit.MinRetentionDays)
	}
}

func TestAuditConfigPartialOverrideAppliesDefaults(t *testing.T) {
	// Create a config file with partial audit section
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	// Only specify some audit fields, others should get defaults
	configJSON := `{
		"inboundDirectories": ["source1"],
		"prefixRules": [{"prefix": "Invoice", "outboundDirectory": "invoices"}],
		"audit": {
			"logDirectory": "/custom/logs",
			"retentionDays": 60
		}
	}`

	if err := os.WriteFile(tmpFile, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the config
	config, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	defaults := audit.DefaultAuditConfig()

	// Verify custom values are preserved
	if config.Audit.LogDirectory != "/custom/logs" {
		t.Errorf("LogDirectory: expected %q, got %q", "/custom/logs", config.Audit.LogDirectory)
	}
	if config.Audit.RetentionDays != 60 {
		t.Errorf("RetentionDays: expected %d, got %d", 60, config.Audit.RetentionDays)
	}

	// Verify defaults are applied for unspecified fields
	if config.Audit.RotationSize != defaults.RotationSize {
		t.Errorf("RotationSize: expected default %d, got %d", defaults.RotationSize, config.Audit.RotationSize)
	}
	if config.Audit.MinRetentionDays != defaults.MinRetentionDays {
		t.Errorf("MinRetentionDays: expected default %d, got %d", defaults.MinRetentionDays, config.Audit.MinRetentionDays)
	}
}

func TestLoadOrCreateAppliesAuditDefaults(t *testing.T) {
	// Test that LoadOrCreate also applies audit defaults
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"inboundDirectories": ["source1"],
		"prefixRules": [{"prefix": "Invoice", "outboundDirectory": "invoices"}]
	}`

	if err := os.WriteFile(tmpFile, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load using LoadOrCreate
	config, err := LoadOrCreate(tmpFile)
	if err != nil {
		t.Fatalf("LoadOrCreate failed: %v", err)
	}

	// Verify audit defaults are applied
	if config.Audit == nil {
		t.Fatal("Expected Audit config to be set with defaults, got nil")
	}

	defaults := audit.DefaultAuditConfig()
	if config.Audit.RotationSize != defaults.RotationSize {
		t.Errorf("RotationSize: expected %d, got %d", defaults.RotationSize, config.Audit.RotationSize)
	}
}

func TestLoadOrCreateNewFileHasAuditDefaults(t *testing.T) {
	// Test that LoadOrCreate returns audit defaults for non-existent file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "nonexistent.json")

	// Load non-existent file
	config, err := LoadOrCreate(tmpFile)
	if err != nil {
		t.Fatalf("LoadOrCreate failed: %v", err)
	}

	// Verify audit defaults are applied
	if config.Audit == nil {
		t.Fatal("Expected Audit config to be set with defaults, got nil")
	}

	defaults := audit.DefaultAuditConfig()
	if config.Audit.LogDirectory != defaults.LogDirectory {
		t.Errorf("LogDirectory: expected %q, got %q", defaults.LogDirectory, config.Audit.LogDirectory)
	}
	if config.Audit.RotationSize != defaults.RotationSize {
		t.Errorf("RotationSize: expected %d, got %d", defaults.RotationSize, config.Audit.RotationSize)
	}
	if config.Audit.MinRetentionDays != defaults.MinRetentionDays {
		t.Errorf("MinRetentionDays: expected %d, got %d", defaults.MinRetentionDays, config.Audit.MinRetentionDays)
	}
}
