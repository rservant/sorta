// Package config handles configuration loading and validation for Sorta.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

// PrefixRule maps a filename prefix to a target directory.
type PrefixRule struct {
	Prefix          string `json:"prefix"`
	TargetDirectory string `json:"targetDirectory"`
}

// Configuration holds all settings for Sorta.
type Configuration struct {
	SourceDirectories  []string     `json:"sourceDirectories"`
	PrefixRules        []PrefixRule `json:"prefixRules"`
	ForReviewDirectory string       `json:"forReviewDirectory"`
}


// Validate checks that the configuration has all required fields.
func (c *Configuration) Validate() error {
	if len(c.SourceDirectories) == 0 {
		return &ConfigError{
			Type:    ValidationError,
			Message: "sourceDirectories must contain at least one directory",
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
		if rule.TargetDirectory == "" {
			return &ConfigError{
				Type:    ValidationError,
				Message: fmt.Sprintf("prefixRules[%d].targetDirectory cannot be empty", i),
			}
		}
	}

	if c.ForReviewDirectory == "" {
		return &ConfigError{
			Type:    ValidationError,
			Message: "forReviewDirectory cannot be empty",
		}
	}

	return nil
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
