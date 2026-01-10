// Package main provides the CLI entry point for Sorta.
package main

import (
	"fmt"
	"os"

	"sorta/internal/orchestrator"
)

func main() {
	// Parse command-line arguments
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	configPath := os.Args[1]

	// Handle help flag
	if configPath == "-h" || configPath == "--help" || configPath == "-help" {
		printUsage()
		os.Exit(0)
	}

	// Run the orchestrator
	summary, err := orchestrator.Run(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print scan errors if any
	for _, scanErr := range summary.ScanErrors {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", scanErr)
	}

	// Print individual file errors
	for _, result := range summary.Results {
		if !result.Success {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", result.SourcePath, result.Error)
		}
	}

	// Print summary
	fmt.Println(summary.PrintSummary())

	// Exit with error code if there were any errors
	if summary.HasErrors() {
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Sorta - File organization utility

Usage: sorta <config-file>

Sorta organizes files from source directories into structured folders
based on filename prefixes and embedded ISO dates (YYYY-MM-DD).

Example:
  sorta config.json

Config file format (JSON):
  {
    "sourceDirectories": ["/path/to/source"],
    "prefixRules": [
      { "prefix": "Invoice", "targetDirectory": "/path/to/invoices" }
    ],
    "forReviewDirectory": "/path/to/review"
  }

Files matching "<prefix> <YYYY-MM-DD> <description>" are moved to:
  <targetDirectory>/<year> <prefix>/<normalized filename>

Files not matching any rule go to the forReviewDirectory.`)
}
