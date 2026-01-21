// Package config handles configuration loading and validation for Sorta.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sorta/internal/audit"
	"strings"
)

// ConfigErrorType represents the type of configuration error.
type ConfigErrorType string

const (
	FileNotFound    ConfigErrorType = "FILE_NOT_FOUND"
	InvalidJSON     ConfigErrorType = "INVALID_JSON"
	ValidationError ConfigErrorType = "VALIDATION_ERROR"
)

// ConfigError represents an error that occurred during configuration loading.
type ConfigError struct {
	Type    ConfigErrorType
	Path    string
	Message string
}

func (e *ConfigError) Error() string {
	switch e.Type {
	case FileNotFound:
		return fmt.Sprintf("configuration file not found: %s", e.Path)
	case InvalidJSON:
		return fmt.Sprintf("invalid JSON in configuration file: %s", e.Message)
	case ValidationError:
		return fmt.Sprintf("configuration validation error: %s", e.Message)
	default:
		return fmt.Sprintf("configuration error: %s", e.Message)
	}
}

// PrefixRule maps a filename prefix to an outbound directory.
type PrefixRule struct {
	Prefix            string `json:"prefix"`
	OutboundDirectory string `json:"outboundDirectory"`
}

// Symlink policy constants
const (
	SymlinkPolicyFollow = "follow"
	SymlinkPolicySkip   = "skip"
	SymlinkPolicyError  = "error"
)

// Watch configuration defaults
const (
	DefaultDebounceSeconds   = 2
	DefaultStableThresholdMs = 1000
)

// DefaultIgnorePatterns returns the default patterns for files to ignore during watch.
func DefaultIgnorePatterns() []string {
	return []string{".tmp", ".part", ".download"}
}

// WatchConfig contains settings for watch mode.
type WatchConfig struct {
	DebounceSeconds   int      `json:"debounceSeconds,omitempty"`   // default: 2
	StableThresholdMs int      `json:"stableThresholdMs,omitempty"` // default: 1000
	IgnorePatterns    []string `json:"ignorePatterns,omitempty"`    // default: [".tmp", ".part", ".download"]
}

// DefaultWatchConfig returns a WatchConfig with sensible defaults.
func DefaultWatchConfig() *WatchConfig {
	return &WatchConfig{
		DebounceSeconds:   DefaultDebounceSeconds,
		StableThresholdMs: DefaultStableThresholdMs,
		IgnorePatterns:    DefaultIgnorePatterns(),
	}
}

// Configuration holds all settings for Sorta.
type Configuration struct {
	InboundDirectories []string           `json:"inboundDirectories"`
	PrefixRules        []PrefixRule       `json:"prefixRules"`
	Audit              *audit.AuditConfig `json:"audit,omitempty"`
	SymlinkPolicy      string             `json:"symlinkPolicy,omitempty"`
	ScanDepth          *int               `json:"scanDepth,omitempty"` // nil = default (0)
	Watch              *WatchConfig       `json:"watch,omitempty"`
}

// GetSymlinkPolicy returns the configured symlink policy or default "skip".
func (c *Configuration) GetSymlinkPolicy() string {
	if c.SymlinkPolicy == "" {
		return SymlinkPolicySkip
	}
	return c.SymlinkPolicy
}

// GetScanDepth returns the configured scan depth or default 0.
func (c *Configuration) GetScanDepth() int {
	if c.ScanDepth == nil {
		return 0
	}
	return *c.ScanDepth
}

// Validate checks that the configuration has all required fields.
func (c *Configuration) Validate() error {
	if len(c.InboundDirectories) == 0 {
		return &ConfigError{
			Type:    ValidationError,
			Message: "inboundDirectories must contain at least one directory",
		}
	}

	if len(c.PrefixRules) == 0 {
		return &ConfigError{
			Type:    ValidationError,
			Message: "prefixRules must contain at least one rule",
		}
	}

	for i, rule := range c.PrefixRules {
		if rule.Prefix == "" {
			return &ConfigError{
				Type:    ValidationError,
				Message: fmt.Sprintf("prefixRules[%d].prefix cannot be empty", i),
			}
		}
		if rule.OutboundDirectory == "" {
			return &ConfigError{
				Type:    ValidationError,
				Message: fmt.Sprintf("prefixRules[%d].outboundDirectory cannot be empty", i),
			}
		}
	}

	return nil
}

// ApplyAuditDefaults ensures the Audit configuration has sensible defaults.
// If Audit is nil, it creates a new AuditConfig with defaults.
// If Audit exists but has zero values, it applies defaults for those fields.
func (c *Configuration) ApplyAuditDefaults() {
	defaults := audit.DefaultAuditConfig()

	if c.Audit == nil {
		c.Audit = &defaults
		return
	}

	// Apply defaults for zero values
	if c.Audit.LogDirectory == "" {
		c.Audit.LogDirectory = defaults.LogDirectory
	}
	if c.Audit.RotationSize == 0 {
		c.Audit.RotationSize = defaults.RotationSize
	}
	// RotationPeriod can be empty (no time-based rotation)
	// RetentionDays 0 means unlimited, so we don't override
	// RetentionRuns 0 means unlimited, so we don't override
	if c.Audit.MinRetentionDays == 0 {
		c.Audit.MinRetentionDays = defaults.MinRetentionDays
	}
}

// ApplyWatchDefaults ensures the Watch configuration has sensible defaults.
// If Watch is nil, it creates a new WatchConfig with defaults.
// If Watch exists but has zero values, it applies defaults for those fields.
func (c *Configuration) ApplyWatchDefaults() {
	defaults := DefaultWatchConfig()

	if c.Watch == nil {
		c.Watch = defaults
		return
	}

	// Apply defaults for zero values
	if c.Watch.DebounceSeconds == 0 {
		c.Watch.DebounceSeconds = defaults.DebounceSeconds
	}
	if c.Watch.StableThresholdMs == 0 {
		c.Watch.StableThresholdMs = defaults.StableThresholdMs
	}
	if c.Watch.IgnorePatterns == nil {
		c.Watch.IgnorePatterns = defaults.IgnorePatterns
	}
}

// GetWatchConfig returns the watch configuration with defaults applied.
// This is useful when you need the watch config but don't want to modify the Configuration.
func (c *Configuration) GetWatchConfig() *WatchConfig {
	if c.Watch == nil {
		return DefaultWatchConfig()
	}

	// Return a copy with defaults applied for zero values
	result := &WatchConfig{
		DebounceSeconds:   c.Watch.DebounceSeconds,
		StableThresholdMs: c.Watch.StableThresholdMs,
		IgnorePatterns:    c.Watch.IgnorePatterns,
	}

	if result.DebounceSeconds == 0 {
		result.DebounceSeconds = DefaultDebounceSeconds
	}
	if result.StableThresholdMs == 0 {
		result.StableThresholdMs = DefaultStableThresholdMs
	}
	if result.IgnorePatterns == nil {
		result.IgnorePatterns = DefaultIgnorePatterns()
	}

	return result
}

// HasPrefix checks if a prefix already exists in the configuration (case-insensitive).
func (c *Configuration) HasPrefix(prefix string) bool {
	lowerPrefix := strings.ToLower(prefix)
	for _, rule := range c.PrefixRules {
		if strings.ToLower(rule.Prefix) == lowerPrefix {
			return true
		}
	}
	return false
}

// AddPrefixRule adds a rule if the prefix doesn't already exist (case-insensitive).
// Returns true if the rule was added, false if it was a duplicate.
func (c *Configuration) AddPrefixRule(rule PrefixRule) bool {
	if c.HasPrefix(rule.Prefix) {
		return false
	}
	c.PrefixRules = append(c.PrefixRules, rule)
	return true
}

// HasInboundDirectory checks if a directory already exists in inboundDirectories.
func (c *Configuration) HasInboundDirectory(dir string) bool {
	for _, d := range c.InboundDirectories {
		if d == dir {
			return true
		}
	}
	return false
}

// AddInboundDirectory adds a directory if it doesn't already exist.
// Returns true if the directory was added, false if it was a duplicate.
func (c *Configuration) AddInboundDirectory(dir string) bool {
	if c.HasInboundDirectory(dir) {
		return false
	}
	c.InboundDirectories = append(c.InboundDirectories, dir)
	return true
}

// Load reads and parses a configuration file from the given path.
func Load(filePath string) (*Configuration, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &ConfigError{
				Type: FileNotFound,
				Path: filePath,
			}
		}
		return nil, &ConfigError{
			Type:    FileNotFound,
			Path:    filePath,
			Message: err.Error(),
		}
	}

	var config Configuration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, &ConfigError{
			Type:    InvalidJSON,
			Message: err.Error(),
		}
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Apply audit defaults for missing or partial audit configuration
	config.ApplyAuditDefaults()

	return &config, nil
}

// LoadOrCreate loads config if it exists, or returns an empty config if the file doesn't exist.
func LoadOrCreate(filePath string) (*Configuration, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Return empty configuration with audit defaults if file doesn't exist
			defaults := audit.DefaultAuditConfig()
			return &Configuration{
				InboundDirectories: []string{},
				PrefixRules:        []PrefixRule{},
				Audit:              &defaults,
			}, nil
		}
		return nil, &ConfigError{
			Type:    FileNotFound,
			Path:    filePath,
			Message: err.Error(),
		}
	}

	var config Configuration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, &ConfigError{
			Type:    InvalidJSON,
			Message: err.Error(),
		}
	}

	// Apply audit defaults for missing or partial audit configuration
	config.ApplyAuditDefaults()

	return &config, nil
}

// Save serializes and writes a configuration to the given path.
func Save(config *Configuration, filePath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return &ConfigError{
			Type:    InvalidJSON,
			Message: err.Error(),
		}
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return &ConfigError{
			Type:    ValidationError,
			Message: fmt.Sprintf("failed to write configuration file: %s", err.Error()),
		}
	}

	return nil
}
