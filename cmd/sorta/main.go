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
		fmt.Fprintln(os.Stderr, "Usage: sorta <config-file>")
		os.Exit(1)
	}

	configPath := os.Args[1]

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
