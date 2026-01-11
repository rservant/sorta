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
	"sorta/internal/output"
	"strings"
)

const defaultConfigPath = "sorta-config.json"

// ParseResult holds the result of parsing command line arguments.
type ParseResult struct {
	Command    string
	CmdArgs    []string
	ConfigPath string
	Verbose    bool
}

// parseArgs parses command line arguments and extracts the command, command arguments, config path, and verbose flag.
// It handles -c/--config flag for specifying a custom config file path and -v/--verbose for verbose mode.
func parseArgs(args []string) (ParseResult, error) {
	result := ParseResult{
		ConfigPath: defaultConfigPath,
		CmdArgs:    []string{},
	}

	if len(args) == 0 {
		return ParseResult{}, errors.New("no command specified")
	}

	i := 0
	// Parse global flags first
	for i < len(args) {
		arg := args[i]
		if arg == "-c" || arg == "--config" {
			if i+1 >= len(args) {
				return ParseResult{}, errors.New("missing value for config flag")
			}
			result.ConfigPath = args[i+1]
			i += 2
			continue
		}
		// Check for -c=value or --config=value format
		if strings.HasPrefix(arg, "-c=") {
			result.ConfigPath = strings.TrimPrefix(arg, "-c=")
			i++
			continue
		}
		if strings.HasPrefix(arg, "--config=") {
			result.ConfigPath = strings.TrimPrefix(arg, "--config=")
			i++
			continue
		}
		// Check for verbose flags
		if arg == "-v" || arg == "--verbose" {
			result.Verbose = true
			i++
			continue
		}
		// Check for -v=true/--verbose=true format
		if strings.HasPrefix(arg, "-v=") {
			result.Verbose = strings.TrimPrefix(arg, "-v=") == "true"
			i++
			continue
		}
		if strings.HasPrefix(arg, "--verbose=") {
			result.Verbose = strings.TrimPrefix(arg, "--verbose=") == "true"
			i++
			continue
		}
		// Not a flag, must be the command
		break
	}

	if i >= len(args) {
		return ParseResult{}, errors.New("no command specified")
	}

	result.Command = args[i]
	i++

	// Remaining args are command arguments
	if i < len(args) {
		result.CmdArgs = args[i:]
	}

	return result, nil
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
	parsed, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		printUsage()
		os.Exit(1)
	}

	// Execute the appropriate command
	var exitCode int
	switch parsed.Command {
	case "config":
		exitCode = runConfigCommand(parsed.ConfigPath, parsed.Verbose)
	case "add-inbound":
		exitCode = runAddInboundCommand(parsed.ConfigPath, parsed.CmdArgs, parsed.Verbose)
	case "discover":
		exitCode = runDiscoverCommand(parsed.ConfigPath, parsed.CmdArgs, parsed.Verbose)
	case "run":
		exitCode = runRunCommand(parsed.ConfigPath, parsed.Verbose)
	case "audit":
		exitCode = runAuditCommand(parsed.CmdArgs, parsed.Verbose)
	case "undo":
		exitCode = runUndoCommand(parsed.CmdArgs, parsed.Verbose)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n", parsed.Command)
		printUsage()
		exitCode = 1
	}

	os.Exit(exitCode)
}

// runConfigCommand displays the current configuration.
// Requirements: 1.2 - verbose flag passed to command
func runConfigCommand(configPath string, verbose bool) int {
	// Create output instance with verbose config
	outConfig := output.DefaultConfig()
	outConfig.Verbose = verbose
	out := output.New(outConfig)

	cfg, err := config.Load(configPath)
	if err != nil {
		var configErr *config.ConfigError
		if errors.As(err, &configErr) {
			switch configErr.Type {
			case config.FileNotFound:
				out.Error("Error: Configuration file not found: %s", configPath)
			case config.InvalidJSON:
				out.Error("Error: Invalid JSON in configuration: %s", configErr.Message)
			default:
				out.Error("Error: %v", err)
			}
		} else {
			out.Error("Error: %v", err)
		}
		return 1
	}

	displayConfigWithOutput(cfg, out)
	return 0
}

// displayConfig formats and prints the configuration to stdout.
func displayConfig(cfg *config.Configuration) string {
	var sb strings.Builder

	sb.WriteString("Configuration:\n")
	sb.WriteString("\nInbound Directories:\n")
	if len(cfg.InboundDirectories) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, dir := range cfg.InboundDirectories {
			sb.WriteString(fmt.Sprintf("  - %s\n", dir))
		}
	}

	sb.WriteString("\nPrefix Rules:\n")
	if len(cfg.PrefixRules) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, rule := range cfg.PrefixRules {
			sb.WriteString(fmt.Sprintf("  - %s -> %s\n", rule.Prefix, rule.OutboundDirectory))
		}
	}

	output := sb.String()
	fmt.Print(output)
	return output
}

// displayConfigWithOutput formats and prints the configuration using the output package.
func displayConfigWithOutput(cfg *config.Configuration, out *output.Output) {
	out.Info("Configuration:")
	out.Info("")
	out.Info("Inbound Directories:")
	if len(cfg.InboundDirectories) == 0 {
		out.Info("  (none)")
	} else {
		for _, dir := range cfg.InboundDirectories {
			out.Info("  - %s", dir)
		}
	}

	out.Info("")
	out.Info("Prefix Rules:")
	if len(cfg.PrefixRules) == 0 {
		out.Info("  (none)")
	} else {
		for _, rule := range cfg.PrefixRules {
			out.Info("  - %s -> %s", rule.Prefix, rule.OutboundDirectory)
		}
	}
}

// runAddInboundCommand adds an inbound directory to the configuration.
// Requirements: 1.2 - verbose flag passed to command
func runAddInboundCommand(configPath string, args []string, verbose bool) int {
	// Create output instance with verbose config
	outConfig := output.DefaultConfig()
	outConfig.Verbose = verbose
	out := output.New(outConfig)

	if len(args) == 0 {
		out.Error("Error: missing directory argument")
		out.Error("Usage: sorta add-inbound <directory>")
		return 1
	}

	directory := args[0]

	// Load or create configuration
	cfg, err := config.LoadOrCreate(configPath)
	if err != nil {
		out.Error("Error: %v", err)
		return 1
	}

	// Try to add the directory
	if !cfg.AddInboundDirectory(directory) {
		out.Info("Directory already exists in configuration: %s", directory)
		return 0
	}

	// Save the updated configuration
	if err := config.Save(cfg, configPath); err != nil {
		out.Error("Error saving configuration: %v", err)
		return 1
	}

	out.Info("Added inbound directory: %s", directory)
	return 0
}

// runDiscoverCommand scans a directory for prefix patterns and updates the configuration.
// Requirements: 3.1, 3.2, 3.3, 5.2 - verbose output and progress indicators
func runDiscoverCommand(configPath string, args []string, verbose bool) int {
	// Create output instance with verbose config
	outConfig := output.DefaultConfig()
	outConfig.Verbose = verbose
	out := output.New(outConfig)

	if len(args) == 0 {
		out.Error("Error: missing scan-directory argument")
		out.Error("Usage: sorta discover <scan-directory>")
		return 1
	}

	scanDir := args[0]

	// Load or create configuration
	cfg, err := config.LoadOrCreate(configPath)
	if err != nil {
		out.Error("Error: %v", err)
		return 1
	}

	// Track progress for non-verbose mode
	progressStarted := false
	fileCount := 0

	// Create discovery callback for verbose output and progress indicator
	// Requirements: 3.1, 3.2, 3.3, 5.2
	discoveryCallback := func(event discovery.DiscoveryEvent) {
		switch event.Type {
		case discovery.EventTypeDir:
			// Requirement 3.1: Display each directory being scanned
			out.Verbose("Scanning directory: %s", event.Path)

			// Start progress on first directory (now we know the total)
			if !progressStarted && event.Total > 0 {
				out.StartProgress(event.Total)
				progressStarted = true
			}

			// Update progress indicator (only shown in non-verbose TTY mode)
			out.UpdateProgress(event.Current, "Scanning directory")

		case discovery.EventTypeFile:
			// Requirement 3.2: Display each file being analyzed
			out.Verbose("  Analyzing file: %s", event.Path)
			fileCount++

		case discovery.EventTypePattern:
			// Requirement 3.3: Display detected patterns as they are found
			out.Verbose("  Found pattern: %s (in %s)", event.Pattern, event.Path)
		}
	}

	// Run discovery with callback
	result, err := discovery.DiscoverWithCallback(scanDir, cfg, discoveryCallback)

	// End progress indicator before showing results
	out.EndProgress()

	if err != nil {
		out.Error("Error during discovery: %v", err)
		return 1
	}

	// Display results
	displayDiscoveryResult(result)

	// Add new rules to configuration
	for _, rule := range result.NewRules {
		cfg.AddPrefixRule(config.PrefixRule{
			Prefix:            rule.Prefix,
			OutboundDirectory: rule.TargetDirectory,
		})
	}

	// Save the updated configuration if there are new rules
	if len(result.NewRules) > 0 {
		if err := config.Save(cfg, configPath); err != nil {
			out.Error("Error saving configuration: %v", err)
			return 1
		}
		out.Info("Configuration saved to: %s", configPath)
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
// Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 5.1 - verbose output and progress indicators
func runRunCommand(configPath string, verbose bool) int {
	// Create output instance with verbose config
	outConfig := output.DefaultConfig()
	outConfig.Verbose = verbose
	out := output.New(outConfig)

	// Load configuration to get audit settings
	cfg, err := config.Load(configPath)
	if err != nil {
		out.Error("Error loading config: %v", err)
		return 1
	}

	// Set up audit configuration (cfg.Audit is already populated with defaults)
	auditConfig := *cfg.Audit
	if auditConfig.LogDirectory == "" {
		auditConfig.LogDirectory = getAuditLogDir()
	}

	// Create the audit log directory if it doesn't exist
	if err := os.MkdirAll(auditConfig.LogDirectory, 0755); err != nil {
		out.Error("Error creating audit directory: %v", err)
		return 1
	}

	// Track if progress has been started
	progressStarted := false

	// Create progress callback for verbose output and progress indicator
	// Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 5.1
	progressCallback := func(current, total int, file string, result *orchestrator.Result) {
		// Start progress on first file (now we know the total)
		if !progressStarted {
			out.StartProgress(total)
			progressStarted = true
		}

		// Update progress indicator (only shown in non-verbose TTY mode)
		out.UpdateProgress(current, "Processing file")

		// Verbose output for each file operation
		if verbose {
			// Requirement 2.1: Display each file being processed with its source path
			out.Verbose("Processing: %s", result.SourcePath)

			switch result.EventType {
			case "MOVE", "DUPLICATE_DETECTED":
				// Requirement 2.2: Display source and destination paths for moves
				out.Verbose("  Moved to: %s", result.DestinationPath)
				if result.IsDuplicate {
					out.Verbose("  (duplicate renamed from: %s)", result.OriginalName)
				}
			case "ROUTE_TO_REVIEW":
				// Requirement 2.4: Display review routing reason
				out.Verbose("  Routed to review: %s", result.DestinationPath)
				if result.ReasonCode != "" {
					out.Verbose("  Reason: %s", result.ReasonCode)
				}
			case "SKIP":
				// Requirement 2.3: Display skip reason
				out.Verbose("  Skipped")
				if result.ReasonCode != "" {
					out.Verbose("  Reason: %s", result.ReasonCode)
				}
			case "ERROR":
				// Requirement 2.5: Display detailed error information
				if result.Error != nil {
					out.Verbose("  Error: %v", result.Error)
				}
			}
		}
	}

	options := &orchestrator.Options{
		AuditConfig:      &auditConfig,
		AppVersion:       "1.0.0",
		MachineID:        getMachineID(),
		ProgressCallback: progressCallback,
	}

	// Run the orchestrator with auditing enabled
	summary, err := orchestrator.RunWithOptions(configPath, options)

	// End progress indicator before showing results
	out.EndProgress()

	if err != nil {
		out.Error("Error: %v", err)
		return 1
	}

	// Print scan errors if any
	for _, scanErr := range summary.ScanErrors {
		out.Error("Warning: %v", scanErr)
	}

	// Print individual file errors (only in non-verbose mode, verbose already showed them)
	if !verbose {
		for _, result := range summary.Results {
			if !result.Success && result.EventType == "ERROR" {
				out.Error("Error processing %s: %v", result.SourcePath, result.Error)
			}
		}
	}

	// Print summary
	out.Info("%s", summary.PrintSummary())

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
// Requirements: 15.1, 15.2, 15.3, 15.4, 15.5, 15.6, 1.2 - verbose flag passed to command
func runAuditCommand(args []string, verbose bool) int {
	// Create output instance with verbose config
	outConfig := output.DefaultConfig()
	outConfig.Verbose = verbose
	out := output.New(outConfig)

	if len(args) == 0 {
		out.Error("Error: missing audit subcommand")
		printAuditUsage()
		return 1
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "list":
		return runAuditListCommand(out)
	case "show":
		return runAuditShowCommand(subArgs, out)
	case "export":
		return runAuditExportCommand(subArgs, out)
	case "help", "-h", "--help":
		printAuditUsage()
		return 0
	default:
		out.Error("Error: unknown audit subcommand '%s'", subcommand)
		printAuditUsage()
		return 1
	}
}

// runAuditListCommand lists all runs with summary statistics.
// Requirements: 15.1, 15.3
func runAuditListCommand(out *output.Output) int {
	logDir := getAuditLogDir()
	reader := audit.NewAuditReader(logDir)

	runs, err := reader.ListRuns()
	if err != nil {
		out.Error("Error reading audit log: %v", err)
		return 1
	}

	if len(runs) == 0 {
		out.Info("No runs found in audit log.")
		return 0
	}

	out.Info("Audit Trail - Run History")
	out.Info("%s", strings.Repeat("=", 80))
	out.Info("%-36s  %-20s  %6s  %6s  %6s  %6s  %-10s",
		"Run ID", "Timestamp", "Moved", "Skip", "Review", "Errors", "Status")
	out.Info("%s", strings.Repeat("-", 80))

	for _, run := range runs {
		timestamp := run.StartTime.Format("2006-01-02 15:04:05")
		status := string(run.Status)
		if run.RunType == audit.RunTypeUndo {
			status = "UNDO"
		}

		out.Info("%-36s  %-20s  %6d  %6d  %6d  %6d  %-10s",
			run.RunID,
			timestamp,
			run.Summary.Moved,
			run.Summary.Skipped,
			run.Summary.RoutedReview,
			run.Summary.Errors,
			status,
		)
	}

	out.Info("%s", strings.Repeat("-", 80))
	out.Info("Total runs: %d", len(runs))

	return 0
}

// runAuditShowCommand shows detailed events for a specific run.
// Requirements: 15.2, 15.4, 15.5
func runAuditShowCommand(args []string, out *output.Output) int {
	if len(args) == 0 {
		out.Error("Error: missing run-id argument")
		out.Error("Usage: sorta audit show <run-id> [--type <event-type>]")
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
		out.Error("Error: %v", err)
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
		out.Error("Error reading events: %v", err)
		return 1
	}

	// Display run header
	out.Info("Audit Trail - Run Details")
	out.Info("%s", strings.Repeat("=", 80))
	out.Info("Run ID:     %s", runInfo.RunID)
	out.Info("Type:       %s", runInfo.RunType)
	out.Info("Status:     %s", runInfo.Status)
	out.Info("Started:    %s", runInfo.StartTime.Format("2006-01-02 15:04:05"))
	if runInfo.EndTime != nil {
		out.Info("Ended:      %s", runInfo.EndTime.Format("2006-01-02 15:04:05"))
	}
	if runInfo.UndoTargetID != nil {
		out.Info("Undo of:    %s", *runInfo.UndoTargetID)
	}
	out.Info("App Ver:    %s", runInfo.AppVersion)
	out.Info("Machine:    %s", runInfo.MachineID)
	out.Info("")

	// Display summary
	out.Info("Summary:")
	out.Info("  Total Files: %d", runInfo.Summary.TotalFiles)
	out.Info("  Moved:       %d", runInfo.Summary.Moved)
	out.Info("  Skipped:     %d", runInfo.Summary.Skipped)
	out.Info("  Review:      %d", runInfo.Summary.RoutedReview)
	out.Info("  Duplicates:  %d", runInfo.Summary.Duplicates)
	out.Info("  Errors:      %d", runInfo.Summary.Errors)
	out.Info("")

	// Display events
	if filterType != "" {
		out.Info("Events (filtered by type: %s):", filterType)
	} else {
		out.Info("Events:")
	}
	out.Info("%s", strings.Repeat("-", 80))

	for _, event := range events {
		displayEventWithOutput(event, out)
	}

	out.Info("%s", strings.Repeat("-", 80))
	out.Info("Total events shown: %d", len(events))

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

// displayEventWithOutput formats and prints a single audit event using the output package.
func displayEventWithOutput(event audit.AuditEvent, out *output.Output) {
	timestamp := event.Timestamp.Format("15:04:05")
	out.Info("[%s] %-20s %s", timestamp, event.EventType, event.Status)

	if event.SourcePath != "" {
		out.Info("         Source: %s", event.SourcePath)
	}
	if event.DestinationPath != "" {
		out.Info("         Dest:   %s", event.DestinationPath)
	}
	if event.ReasonCode != "" {
		out.Info("         Reason: %s", event.ReasonCode)
	}
	if event.ErrorDetails != nil {
		out.Info("         Error:  [%s] %s", event.ErrorDetails.ErrorType, event.ErrorDetails.ErrorMessage)
	}
	if event.FileIdentity != nil {
		out.Info("         Hash:   %s (size: %d)", event.FileIdentity.ContentHash[:16]+"...", event.FileIdentity.Size)
	}
	out.Info("")
}

// runAuditExportCommand exports run audit data to a file.
// Requirements: 15.6
func runAuditExportCommand(args []string, out *output.Output) int {
	if len(args) == 0 {
		out.Error("Error: missing run-id argument")
		out.Error("Usage: sorta audit export <run-id> [output-file]")
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
		out.Error("Error: %v", err)
		return 1
	}

	// Get all events for the run
	events, err := reader.GetRun(runID)
	if err != nil {
		out.Error("Error reading events: %v", err)
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
		out.Error("Error marshaling export data: %v", err)
		return 1
	}

	// Write to file
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		out.Error("Error writing export file: %v", err)
		return 1
	}

	out.Info("Exported run %s to %s", runID, outputFile)
	out.Info("  Events: %d", len(events))

	return 0
}

// runUndoCommand handles the undo command.
// Requirements: 4.1, 4.2, 4.3, 5.1, 5.3, 6.1, 7.2
func runUndoCommand(args []string, verbose bool) int {
	// Create output instance with verbose config
	outConfig := output.DefaultConfig()
	outConfig.Verbose = verbose
	out := output.New(outConfig)

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
				out.Error("Error parsing path mapping: %v", err)
				return 1
			}
			pathMappings = append(pathMappings, mapping)
		case !strings.HasPrefix(arg, "-"):
			runID = arg
		default:
			out.Error("Error: unknown flag '%s'", arg)
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
		out.Error("Error initializing audit writer: %v", err)
		return 1
	}
	defer writer.Close()

	// Create undo engine
	engine := audit.NewUndoEngine(reader, writer, "1.0.0", getMachineID())

	// Track if progress has been started
	progressStarted := false

	// Set up callback for verbose output and progress indicator
	// Requirements: 4.1, 4.2, 4.3, 5.3
	undoCallback := func(event audit.UndoProgressEvent) {
		// Start progress on first event (now we know the total)
		if !progressStarted && event.Total > 0 {
			out.StartProgress(event.Total)
			progressStarted = true
		}

		// Update progress indicator (only shown in non-verbose TTY mode)
		out.UpdateProgress(event.Current, "Restoring file")

		// Verbose output for each undo operation
		switch event.Type {
		case "restore":
			// Requirement 4.1: Display each file being restored with source and destination
			out.Verbose("Restoring: %s", event.SourcePath)
			out.Verbose("  From: %s", event.DestPath)
			out.Verbose("  To: %s", event.SourcePath)
		case "skip":
			// Requirement 4.2: Display skip reasons for files that cannot be restored
			out.Verbose("Skipping: %s", event.SourcePath)
			if event.Reason != "" {
				out.Verbose("  Reason: %s", event.Reason)
			}
		case "verify":
			// Requirement 4.3: Display verification status for file identity checks
			if event.VerifyStatus == "match" {
				out.Verbose("Verified: %s (identity match)", event.DestPath)
			} else {
				out.Verbose("Verification failed: %s", event.DestPath)
				out.Verbose("  Status: %s", event.VerifyStatus)
				if event.Reason != "" {
					out.Verbose("  Reason: %s", event.Reason)
				}
			}
		case "error":
			// Display error information
			out.Verbose("Error: %s", event.SourcePath)
			if event.Reason != "" {
				out.Verbose("  Reason: %s", event.Reason)
			}
		}
	}

	// Set the callback on the engine
	engine.SetCallback(undoCallback)

	var result *audit.UndoResult
	if runID == "" {
		// Undo most recent run
		result, err = engine.UndoLatest(pathMappings)
	} else {
		// Undo specific run
		result, err = engine.UndoRun(audit.RunID(runID), pathMappings)
	}

	// End progress indicator before showing results
	out.EndProgress()

	if err != nil {
		out.Error("Error during undo: %v", err)
		return 1
	}

	// Display results
	out.Info("Undo Operation Complete")
	out.Info("%s", strings.Repeat("=", 50))
	out.Info("Undo Run ID:    %s", result.UndoRunID)
	out.Info("Target Run ID:  %s", result.TargetRunID)
	out.Info("Total Events:   %d", result.TotalEvents)
	out.Info("Restored:       %d", result.Restored)
	out.Info("Skipped:        %d", result.Skipped)
	out.Info("Failed:         %d", result.Failed)

	if len(result.FailureDetails) > 0 {
		out.Info("\nFailure Details:")
		for _, failure := range result.FailureDetails {
			out.Info("  - %s: %s (%s)", failure.SourcePath, failure.Message, failure.Reason)
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
  add-inbound <dir>     Add an inbound directory to configuration
  discover <dir>        Auto-discover prefix rules from existing directories
  run                   Execute file organization
  audit <subcommand>    View audit trail history
  undo [run-id]         Undo file operations from a run

Flags:
  -c, --config <path>   Config file path (default: sorta-config.json)
  -v, --verbose         Enable verbose output for detailed operation information
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
  sorta add-inbound /path/to/inbound    Add an inbound directory
  sorta discover /path/to/organized     Discover prefix rules from existing files
  sorta run                             Organize files according to configuration
  sorta -v run                          Run with verbose output
  sorta audit list                      List all audit runs
  sorta audit show <run-id>             Show details for a specific run
  sorta undo                            Undo most recent run
  sorta undo --preview                  Preview what would be undone
  sorta -c custom.json run              Use custom config file

Config file format (JSON):
  {
    "inboundDirectories": ["/path/to/inbound"],
    "prefixRules": [
      { "prefix": "Invoice", "outboundDirectory": "/path/to/invoices" }
    ]
  }

Files matching "<prefix> <YYYY-MM-DD> <description>" are moved to:
  <outboundDirectory>/<year> <prefix>/<normalized filename>

Files not matching any rule go to a "for-review" subdirectory within their inbound directory.`)
}
