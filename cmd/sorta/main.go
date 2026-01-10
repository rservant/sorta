// Package main provides the CLI entry point for Sorta.
package main

import (
	"errors"
	"fmt"
	"os"
	"sorta/internal/config"
	"sorta/internal/discovery"
	"sorta/internal/orchestrator"
	"strings"
)

const defaultConfigPath = "sorta-config.json"

// ParseResult holds the result of parsing command line arguments.
type ParseResult struct {
	Command    string
	CmdArgs    []string
	ConfigPath string
}

// parseArgs parses command line arguments and extracts the command, command arguments, and config path.
// It handles -c/--config flag for specifying a custom config file path.
func parseArgs(args []string) (command string, cmdArgs []string, configPath string, err error) {
	configPath = defaultConfigPath
	cmdArgs = []string{}

	if len(args) == 0 {
		return "", nil, "", errors.New("no command specified")
	}

	i := 0
	// Parse global flags first
	for i < len(args) {
		arg := args[i]
		if arg == "-c" || arg == "--config" {
			if i+1 >= len(args) {
				return "", nil, "", errors.New("missing value for config flag")
			}
			configPath = args[i+1]
			i += 2
			continue
		}
		// Check for -c=value or --config=value format
		if strings.HasPrefix(arg, "-c=") {
			configPath = strings.TrimPrefix(arg, "-c=")
			i++
			continue
		}
		if strings.HasPrefix(arg, "--config=") {
			configPath = strings.TrimPrefix(arg, "--config=")
			i++
			continue
		}
		// Not a flag, must be the command
		break
	}

	if i >= len(args) {
		return "", nil, "", errors.New("no command specified")
	}

	command = args[i]
	i++

	// Remaining args are command arguments
	if i < len(args) {
		cmdArgs = args[i:]
	}

	return command, cmdArgs, configPath, nil
}

func main() {
	// Handle help flag early
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "-h" || arg == "--help" || arg == "-help" || arg == "help" {
			printUsage()
			os.Exit(0)
		}
	}

	// Parse command-line arguments (skip program name)
	command, cmdArgs, configPath, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		printUsage()
		os.Exit(1)
	}

	// Execute the appropriate command
	var exitCode int
	switch command {
	case "config":
		exitCode = runConfigCommand(configPath)
	case "add-source":
		exitCode = runAddSourceCommand(configPath, cmdArgs)
	case "discover":
		exitCode = runDiscoverCommand(configPath, cmdArgs)
	case "run":
		exitCode = runRunCommand(configPath)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n", command)
		printUsage()
		exitCode = 1
	}

	os.Exit(exitCode)
}

// runConfigCommand displays the current configuration.
func runConfigCommand(configPath string) int {
	cfg, err := config.Load(configPath)
	if err != nil {
		var configErr *config.ConfigError
		if errors.As(err, &configErr) {
			switch configErr.Type {
			case config.FileNotFound:
				fmt.Fprintf(os.Stderr, "Error: Configuration file not found: %s\n", configPath)
			case config.InvalidJSON:
				fmt.Fprintf(os.Stderr, "Error: Invalid JSON in configuration: %s\n", configErr.Message)
			default:
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return 1
	}

	displayConfig(cfg)
	return 0
}

// displayConfig formats and prints the configuration to stdout.
func displayConfig(cfg *config.Configuration) string {
	var sb strings.Builder

	sb.WriteString("Configuration:\n")
	sb.WriteString("\nSource Directories:\n")
	if len(cfg.SourceDirectories) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, dir := range cfg.SourceDirectories {
			sb.WriteString(fmt.Sprintf("  - %s\n", dir))
		}
	}

	sb.WriteString("\nPrefix Rules:\n")
	if len(cfg.PrefixRules) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, rule := range cfg.PrefixRules {
			sb.WriteString(fmt.Sprintf("  - %s -> %s\n", rule.Prefix, rule.TargetDirectory))
		}
	}

	output := sb.String()
	fmt.Print(output)
	return output
}

// runAddSourceCommand adds a source directory to the configuration.
func runAddSourceCommand(configPath string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: missing directory argument\n")
		fmt.Fprintf(os.Stderr, "Usage: sorta add-source <directory>\n")
		return 1
	}

	directory := args[0]

	// Load or create configuration
	cfg, err := config.LoadOrCreate(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Try to add the directory
	if !cfg.AddSourceDirectory(directory) {
		fmt.Printf("Directory already exists in configuration: %s\n", directory)
		return 0
	}

	// Save the updated configuration
	if err := config.Save(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving configuration: %v\n", err)
		return 1
	}

	fmt.Printf("Added source directory: %s\n", directory)
	return 0
}

// runDiscoverCommand scans a directory for prefix patterns and updates the configuration.
func runDiscoverCommand(configPath string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: missing scan-directory argument\n")
		fmt.Fprintf(os.Stderr, "Usage: sorta discover <scan-directory>\n")
		return 1
	}

	scanDir := args[0]

	// Load or create configuration
	cfg, err := config.LoadOrCreate(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Run discovery
	result, err := discovery.Discover(scanDir, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during discovery: %v\n", err)
		return 1
	}

	// Display results
	displayDiscoveryResult(result)

	// Add new rules to configuration
	for _, rule := range result.NewRules {
		cfg.AddPrefixRule(config.PrefixRule{
			Prefix:          rule.Prefix,
			TargetDirectory: rule.TargetDirectory,
		})
	}

	// Save the updated configuration if there are new rules
	if len(result.NewRules) > 0 {
		if err := config.Save(cfg, configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving configuration: %v\n", err)
			return 1
		}
		fmt.Printf("\nConfiguration saved to: %s\n", configPath)
	}

	return 0
}

// displayDiscoveryResult formats and prints the discovery results to stdout.
func displayDiscoveryResult(result *discovery.DiscoveryResult) string {
	var sb strings.Builder

	sb.WriteString("Discovery Results:\n")
	sb.WriteString(fmt.Sprintf("  Directories scanned: %d\n", result.ScannedDirs))
	sb.WriteString(fmt.Sprintf("  Files analyzed: %d\n", result.FilesAnalyzed))

	if len(result.NewRules) == 0 && len(result.SkippedRules) == 0 {
		sb.WriteString("\nNo prefix rules discovered.\n")
	} else {
		if len(result.NewRules) > 0 {
			sb.WriteString(fmt.Sprintf("\nNew prefix rules found: %d\n", len(result.NewRules)))
			for _, rule := range result.NewRules {
				sb.WriteString(fmt.Sprintf("  - %s -> %s\n", rule.Prefix, rule.TargetDirectory))
			}
		}

		if len(result.SkippedRules) > 0 {
			sb.WriteString(fmt.Sprintf("\nSkipped (already configured): %d\n", len(result.SkippedRules)))
			for _, rule := range result.SkippedRules {
				sb.WriteString(fmt.Sprintf("  - %s (already configured)\n", rule.Prefix))
			}
		}
	}

	output := sb.String()
	fmt.Print(output)
	return output
}

// runRunCommand executes the file organization workflow.
func runRunCommand(configPath string) int {
	// Run the orchestrator
	summary, err := orchestrator.Run(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
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
		return 1
	}
	return 0
}

func printUsage() {
	fmt.Println(`Sorta - File organization utility

Usage: sorta [flags] <command> [arguments]

Commands:
  config                Display current configuration
  add-source <dir>      Add a source directory to configuration
  discover <dir>        Auto-discover prefix rules from existing directories
  run                   Execute file organization

Flags:
  -c, --config <path>   Config file path (default: sorta-config.json)
  -h, --help            Show this help message

Examples:
  sorta config                          Show current configuration
  sorta add-source /path/to/source      Add a source directory
  sorta discover /path/to/organized     Discover prefix rules from existing files
  sorta run                             Organize files according to configuration
  sorta -c custom.json run              Use custom config file

Config file format (JSON):
  {
    "sourceDirectories": ["/path/to/source"],
    "prefixRules": [
      { "prefix": "Invoice", "targetDirectory": "/path/to/invoices" }
    ]
  }

Files matching "<prefix> <YYYY-MM-DD> <description>" are moved to:
  <targetDirectory>/<year> <prefix>/<normalized filename>

Files not matching any rule go to a "for-review" subdirectory within their source directory.`)
}
