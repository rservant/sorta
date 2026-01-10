// Package main provides the CLI entry point for Sorta.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sorta/internal/audit"
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
	case "audit":
		exitCode = runAuditCommand(cmdArgs)
	case "undo":
		exitCode = runUndoCommand(cmdArgs)
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
	// Load configuration to get audit settings
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return 1
	}

	// Set up audit configuration (cfg.Audit is already populated with defaults)
	auditConfig := *cfg.Audit
	if auditConfig.LogDirectory == "" {
		auditConfig.LogDirectory = getAuditLogDir()
	}

	// Create the audit log directory if it doesn't exist
	if err := os.MkdirAll(auditConfig.LogDirectory, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating audit directory: %v\n", err)
		return 1
	}

	options := &orchestrator.Options{
		AuditConfig: &auditConfig,
		AppVersion:  "1.0.0",
		MachineID:   getMachineID(),
	}

	// Run the orchestrator with auditing enabled
	summary, err := orchestrator.RunWithOptions(configPath, options)
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

// getAuditLogDir returns the audit log directory path.
// It uses the default .sorta/audit directory relative to the current working directory.
func getAuditLogDir() string {
	return filepath.Join(".sorta", "audit")
}

// runAuditCommand handles the audit subcommands.
// Requirements: 15.1, 15.2, 15.3, 15.4, 15.5, 15.6
func runAuditCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: missing audit subcommand\n")
		printAuditUsage()
		return 1
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "list":
		return runAuditListCommand()
	case "show":
		return runAuditShowCommand(subArgs)
	case "export":
		return runAuditExportCommand(subArgs)
	case "help", "-h", "--help":
		printAuditUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown audit subcommand '%s'\n", subcommand)
		printAuditUsage()
		return 1
	}
}

// runAuditListCommand lists all runs with summary statistics.
// Requirements: 15.1, 15.3
func runAuditListCommand() int {
	logDir := getAuditLogDir()
	reader := audit.NewAuditReader(logDir)

	runs, err := reader.ListRuns()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading audit log: %v\n", err)
		return 1
	}

	if len(runs) == 0 {
		fmt.Println("No runs found in audit log.")
		return 0
	}

	fmt.Println("Audit Trail - Run History")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("%-36s  %-20s  %6s  %6s  %6s  %6s  %-10s\n",
		"Run ID", "Timestamp", "Moved", "Skip", "Review", "Errors", "Status")
	fmt.Println(strings.Repeat("-", 80))

	for _, run := range runs {
		timestamp := run.StartTime.Format("2006-01-02 15:04:05")
		status := string(run.Status)
		if run.RunType == audit.RunTypeUndo {
			status = "UNDO"
		}

		fmt.Printf("%-36s  %-20s  %6d  %6d  %6d  %6d  %-10s\n",
			run.RunID,
			timestamp,
			run.Summary.Moved,
			run.Summary.Skipped,
			run.Summary.RoutedReview,
			run.Summary.Errors,
			status,
		)
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Total runs: %d\n", len(runs))

	return 0
}

// runAuditShowCommand shows detailed events for a specific run.
// Requirements: 15.2, 15.4, 15.5
func runAuditShowCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: missing run-id argument\n")
		fmt.Fprintf(os.Stderr, "Usage: sorta audit show <run-id> [--type <event-type>]\n")
		return 1
	}

	runID := audit.RunID(args[0])
	var filterType string

	// Parse optional --type flag
	for i := 1; i < len(args); i++ {
		if args[i] == "--type" && i+1 < len(args) {
			filterType = strings.ToUpper(args[i+1])
			i++
		}
	}

	logDir := getAuditLogDir()
	reader := audit.NewAuditReader(logDir)

	// Get run info first
	runInfo, err := reader.GetRunByID(runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Get events with optional filtering
	var events []audit.AuditEvent
	if filterType != "" {
		filter := audit.EventFilter{
			EventTypes: []audit.EventType{audit.EventType(filterType)},
		}
		events, err = reader.FilterEvents(runID, filter)
	} else {
		events, err = reader.GetRun(runID)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading events: %v\n", err)
		return 1
	}

	// Display run header
	fmt.Println("Audit Trail - Run Details")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Run ID:     %s\n", runInfo.RunID)
	fmt.Printf("Type:       %s\n", runInfo.RunType)
	fmt.Printf("Status:     %s\n", runInfo.Status)
	fmt.Printf("Started:    %s\n", runInfo.StartTime.Format("2006-01-02 15:04:05"))
	if runInfo.EndTime != nil {
		fmt.Printf("Ended:      %s\n", runInfo.EndTime.Format("2006-01-02 15:04:05"))
	}
	if runInfo.UndoTargetID != nil {
		fmt.Printf("Undo of:    %s\n", *runInfo.UndoTargetID)
	}
	fmt.Printf("App Ver:    %s\n", runInfo.AppVersion)
	fmt.Printf("Machine:    %s\n", runInfo.MachineID)
	fmt.Println()

	// Display summary
	fmt.Println("Summary:")
	fmt.Printf("  Total Files: %d\n", runInfo.Summary.TotalFiles)
	fmt.Printf("  Moved:       %d\n", runInfo.Summary.Moved)
	fmt.Printf("  Skipped:     %d\n", runInfo.Summary.Skipped)
	fmt.Printf("  Review:      %d\n", runInfo.Summary.RoutedReview)
	fmt.Printf("  Duplicates:  %d\n", runInfo.Summary.Duplicates)
	fmt.Printf("  Errors:      %d\n", runInfo.Summary.Errors)
	fmt.Println()

	// Display events
	if filterType != "" {
		fmt.Printf("Events (filtered by type: %s):\n", filterType)
	} else {
		fmt.Println("Events:")
	}
	fmt.Println(strings.Repeat("-", 80))

	for _, event := range events {
		displayEvent(event)
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Total events shown: %d\n", len(events))

	return 0
}

// displayEvent formats and prints a single audit event.
func displayEvent(event audit.AuditEvent) {
	timestamp := event.Timestamp.Format("15:04:05")
	fmt.Printf("[%s] %-20s %s\n", timestamp, event.EventType, event.Status)

	if event.SourcePath != "" {
		fmt.Printf("         Source: %s\n", event.SourcePath)
	}
	if event.DestinationPath != "" {
		fmt.Printf("         Dest:   %s\n", event.DestinationPath)
	}
	if event.ReasonCode != "" {
		fmt.Printf("         Reason: %s\n", event.ReasonCode)
	}
	if event.ErrorDetails != nil {
		fmt.Printf("         Error:  [%s] %s\n", event.ErrorDetails.ErrorType, event.ErrorDetails.ErrorMessage)
	}
	if event.FileIdentity != nil {
		fmt.Printf("         Hash:   %s (size: %d)\n", event.FileIdentity.ContentHash[:16]+"...", event.FileIdentity.Size)
	}
	fmt.Println()
}

// runAuditExportCommand exports run audit data to a file.
// Requirements: 15.6
func runAuditExportCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: missing run-id argument\n")
		fmt.Fprintf(os.Stderr, "Usage: sorta audit export <run-id> [output-file]\n")
		return 1
	}

	runID := audit.RunID(args[0])
	outputFile := ""
	if len(args) > 1 {
		outputFile = args[1]
	} else {
		// Default output filename
		outputFile = fmt.Sprintf("audit-export-%s.json", runID)
	}

	logDir := getAuditLogDir()
	reader := audit.NewAuditReader(logDir)

	// Get run info
	runInfo, err := reader.GetRunByID(runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Get all events for the run
	events, err := reader.GetRun(runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading events: %v\n", err)
		return 1
	}

	// Create export structure
	export := struct {
		RunInfo audit.RunInfo      `json:"runInfo"`
		Events  []audit.AuditEvent `json:"events"`
	}{
		RunInfo: *runInfo,
		Events:  events,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling export data: %v\n", err)
		return 1
	}

	// Write to file
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing export file: %v\n", err)
		return 1
	}

	fmt.Printf("Exported run %s to %s\n", runID, outputFile)
	fmt.Printf("  Events: %d\n", len(events))

	return 0
}

// runUndoCommand handles the undo command.
// Requirements: 5.1, 6.1, 7.2
func runUndoCommand(args []string) int {
	var runID string
	var preview bool
	var pathMappings []audit.PathMapping

	// Parse arguments
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--preview":
			preview = true
		case arg == "--path-mapping" && i+1 < len(args):
			i++
			mapping, err := parsePathMapping(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing path mapping: %v\n", err)
				return 1
			}
			pathMappings = append(pathMappings, mapping)
		case !strings.HasPrefix(arg, "-"):
			runID = arg
		default:
			fmt.Fprintf(os.Stderr, "Error: unknown flag '%s'\n", arg)
			printUndoUsage()
			return 1
		}
	}

	logDir := getAuditLogDir()
	reader := audit.NewAuditReader(logDir)

	// If preview mode, show what would be undone
	if preview {
		return runUndoPreview(reader, runID, pathMappings)
	}

	// Create writer for recording undo operations
	auditConfig := audit.DefaultAuditConfig()
	auditConfig.LogDirectory = logDir
	writer, err := audit.NewAuditWriter(auditConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing audit writer: %v\n", err)
		return 1
	}
	defer writer.Close()

	// Create undo engine
	engine := audit.NewUndoEngine(reader, writer, "1.0.0", getMachineID())

	var result *audit.UndoResult
	if runID == "" {
		// Undo most recent run
		result, err = engine.UndoLatest(pathMappings)
	} else {
		// Undo specific run
		result, err = engine.UndoRun(audit.RunID(runID), pathMappings)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during undo: %v\n", err)
		return 1
	}

	// Display results
	fmt.Println("Undo Operation Complete")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Undo Run ID:    %s\n", result.UndoRunID)
	fmt.Printf("Target Run ID:  %s\n", result.TargetRunID)
	fmt.Printf("Total Events:   %d\n", result.TotalEvents)
	fmt.Printf("Restored:       %d\n", result.Restored)
	fmt.Printf("Skipped:        %d\n", result.Skipped)
	fmt.Printf("Failed:         %d\n", result.Failed)

	if len(result.FailureDetails) > 0 {
		fmt.Println("\nFailure Details:")
		for _, failure := range result.FailureDetails {
			fmt.Printf("  - %s: %s (%s)\n", failure.SourcePath, failure.Message, failure.Reason)
		}
	}

	if result.Failed > 0 {
		return 1
	}
	return 0
}

// runUndoPreview shows what would be undone without executing.
func runUndoPreview(reader *audit.AuditReader, runID string, pathMappings []audit.PathMapping) int {
	// Create a temporary writer (won't actually write)
	auditConfig := audit.DefaultAuditConfig()
	auditConfig.LogDirectory = getAuditLogDir()
	writer, err := audit.NewAuditWriter(auditConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing audit writer: %v\n", err)
		return 1
	}
	defer writer.Close()

	engine := audit.NewUndoEngine(reader, writer, "1.0.0", getMachineID())

	var targetRunID audit.RunID
	if runID == "" {
		// Get most recent run
		latestRun, err := reader.GetLatestRun()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		targetRunID = latestRun.RunID
	} else {
		targetRunID = audit.RunID(runID)
	}

	preview, err := engine.PreviewUndo(targetRunID, pathMappings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating preview: %v\n", err)
		return 1
	}

	fmt.Println("Undo Preview (no changes will be made)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Target Run ID: %s\n", preview.TargetRunID)
	fmt.Printf("Total Moves:   %d\n", preview.TotalMoves)
	fmt.Printf("Total Reviews: %d\n", preview.TotalReviews)
	fmt.Printf("No-Op Events:  %d\n", preview.TotalNoOps)
	fmt.Println()

	if len(preview.EventsToUndo) > 0 {
		fmt.Println("Events to process:")
		fmt.Println(strings.Repeat("-", 60))
		for _, event := range preview.EventsToUndo {
			action := "SKIP"
			if event.WillRestore {
				action = "RESTORE"
			}
			fmt.Printf("[%s] %s\n", action, event.EventType)
			if event.DestPath != "" {
				fmt.Printf("         From: %s\n", event.DestPath)
			}
			if event.SourcePath != "" {
				fmt.Printf("         To:   %s\n", event.SourcePath)
			}
			fmt.Println()
		}
	}

	return 0
}

// parsePathMapping parses a path mapping string in the format "original:mapped".
func parsePathMapping(s string) (audit.PathMapping, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return audit.PathMapping{}, fmt.Errorf("invalid path mapping format, expected 'original:mapped'")
	}
	return audit.PathMapping{
		OriginalPrefix: parts[0],
		MappedPrefix:   parts[1],
	}, nil
}

// getMachineID returns a stable machine identifier.
func getMachineID() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// printAuditUsage prints usage information for the audit command.
func printAuditUsage() {
	fmt.Println(`Usage: sorta audit <subcommand> [arguments]

Subcommands:
  list                  List all runs with summary statistics
  show <run-id>         Show detailed events for a specific run
  export <run-id>       Export run audit data to a file

Options for 'show':
  --type <event-type>   Filter events by type (e.g., MOVE, SKIP, ERROR)

Examples:
  sorta audit list
  sorta audit show abc123-def456-...
  sorta audit show abc123-def456-... --type MOVE
  sorta audit export abc123-def456-... output.json`)
}

// printUndoUsage prints usage information for the undo command.
func printUndoUsage() {
	fmt.Println(`Usage: sorta undo [run-id] [options]

Arguments:
  run-id                Specific run ID to undo (optional, defaults to most recent)

Options:
  --preview             Show what would be undone without making changes
  --path-mapping <map>  Path mapping for cross-machine undo (format: original:mapped)

Examples:
  sorta undo                                    Undo most recent run
  sorta undo abc123-def456-...                  Undo specific run
  sorta undo --preview                          Preview undo of most recent run
  sorta undo --path-mapping /old/path:/new/path Cross-machine undo with path mapping`)
}

func printUsage() {
	fmt.Println(`Sorta - File organization utility

Usage: sorta [flags] <command> [arguments]

Commands:
  config                Display current configuration
  add-source <dir>      Add a source directory to configuration
  discover <dir>        Auto-discover prefix rules from existing directories
  run                   Execute file organization
  audit <subcommand>    View audit trail history
  undo [run-id]         Undo file operations from a run

Flags:
  -c, --config <path>   Config file path (default: sorta-config.json)
  -h, --help            Show this help message

Audit Subcommands:
  audit list            List all runs with summary statistics
  audit show <run-id>   Show detailed events for a specific run
  audit export <run-id> Export run audit data to a file

Undo Options:
  --preview             Show what would be undone without making changes
  --path-mapping <map>  Path mapping for cross-machine undo (format: original:mapped)

Examples:
  sorta config                          Show current configuration
  sorta add-source /path/to/source      Add a source directory
  sorta discover /path/to/organized     Discover prefix rules from existing files
  sorta run                             Organize files according to configuration
  sorta audit list                      List all audit runs
  sorta audit show <run-id>             Show details for a specific run
  sorta undo                            Undo most recent run
  sorta undo --preview                  Preview what would be undone
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
