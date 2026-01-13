// Package orchestrator coordinates the file organization workflow for Sorta.
package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"

	"sorta/internal/audit"
	"sorta/internal/classifier"
	"sorta/internal/config"
	"sorta/internal/organizer"
	"sorta/internal/scanner"
)

// Result represents the outcome of organizing a single file.
type Result struct {
	SourcePath      string
	DestinationPath string
	Success         bool
	Error           error
	IsDuplicate     bool   // True if the file was renamed due to a duplicate
	OriginalName    string // Original filename before duplicate renaming (empty if not a duplicate)
	EventType       string // Type of event: MOVE, ROUTE_TO_REVIEW, SKIP, ERROR
	ReasonCode      string // Reason code for skip/review routing
}

// Summary represents the overall results of a Sorta run.
type Summary struct {
	TotalFiles     int
	SuccessCount   int
	ErrorCount     int
	DuplicateCount int // Number of files moved as duplicates
	SkippedCount   int // Number of files skipped
	ReviewCount    int // Number of files routed to review
	Results        []Result
	ScanErrors     []error
}

// ProgressCallback is called during file processing to report progress.
// Parameters: current file index (1-based), total files, file path, result of processing
type ProgressCallback func(current, total int, file string, result *Result)

// Options contains optional configuration for a Sorta run.
type Options struct {
	AuditConfig      *audit.AuditConfig // Audit configuration (nil to disable auditing)
	AppVersion       string             // Application version for audit records
	MachineID        string             // Machine identifier for audit records
	ProgressCallback ProgressCallback   // Progress reporting callback (optional)
	ScanDepth        *int               // Override scan depth (nil = use config default)
	SymlinkPolicy    string             // Override symlink policy (empty = use config default)
}

// Run executes the Sorta file organization workflow.
// It loads configuration, scans source directories, classifies files,
// and organizes them according to the rules.
func Run(configPath string) (*Summary, error) {
	return RunWithOptions(configPath, nil)
}

// RunWithOptions executes the Sorta file organization workflow with optional audit support.
// If options.AuditConfig is provided, all file operations are logged to the audit trail.
// Requirements: 11.1, 11.4 - Fail-fast on audit write failure, audit before move
func RunWithOptions(configPath string, options *Options) (*Summary, error) {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	summary := &Summary{
		Results:    make([]Result, 0),
		ScanErrors: make([]error, 0),
	}

	// Initialize audit writer if audit config is provided
	var auditWriter *audit.AuditWriter
	var runID audit.RunID
	var identityResolver *audit.IdentityResolver

	if options != nil && options.AuditConfig != nil {
		auditWriter, err = audit.NewAuditWriter(*options.AuditConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize audit writer: %w", err)
		}
		defer auditWriter.Close()

		// Start the audit run before processing any files
		// Requirements: 11.4 - audit record must be written before file operations
		appVersion := options.AppVersion
		if appVersion == "" {
			appVersion = "unknown"
		}
		machineID := options.MachineID
		if machineID == "" {
			machineID = getMachineID()
		}

		runID, err = auditWriter.StartRun(appVersion, machineID)
		if err != nil {
			return nil, fmt.Errorf("failed to start audit run: %w", err)
		}

		identityResolver = audit.NewIdentityResolver()
	}

	// Scan all inbound directories and collect files
	// Determine scan options
	scanOpts := scanner.DefaultScanOptions()

	// Use config values as defaults
	scanOpts.MaxDepth = cfg.GetScanDepth()
	scanOpts.SymlinkPolicy = cfg.GetSymlinkPolicy()

	// Apply overrides from options
	if options != nil {
		if options.ScanDepth != nil {
			scanOpts.MaxDepth = *options.ScanDepth
		}
		if options.SymlinkPolicy != "" {
			scanOpts.SymlinkPolicy = options.SymlinkPolicy
		}
	}

	var allFiles []scanner.FileEntry
	for _, sourceDir := range cfg.InboundDirectories {
		// Runtime path validation: check if directory exists before scanning
		// Requirements: 4.1, 4.2 - validate inbound directories exist before processing
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			summary.ScanErrors = append(summary.ScanErrors, fmt.Errorf("inbound directory does not exist: %s", sourceDir))
			continue
		}

		files, err := scanner.ScanWithOptions(sourceDir, scanOpts)
		if err != nil {
			// Log error and continue with remaining directories (Requirement 2.2)
			summary.ScanErrors = append(summary.ScanErrors, fmt.Errorf("failed to scan %s: %w", sourceDir, err))
			continue
		}
		allFiles = append(allFiles, files...)
	}

	summary.TotalFiles = len(allFiles)

	// Track if we need to fail-fast due to audit write failure
	var auditError error

	// Process each file
	for i, file := range allFiles {
		result := processFileWithAudit(file, cfg, auditWriter, identityResolver)
		summary.Results = append(summary.Results, result)

		if result.Success {
			summary.SuccessCount++
			if result.IsDuplicate {
				summary.DuplicateCount++
			}
			if result.EventType == "ROUTE_TO_REVIEW" {
				summary.ReviewCount++
			}
		} else {
			if result.EventType == "SKIP" {
				summary.SkippedCount++
			} else {
				summary.ErrorCount++
			}
		}

		// Call progress callback after each file is processed
		// Requirements: 5.1 - progress indicator for run command
		if options != nil && options.ProgressCallback != nil {
			options.ProgressCallback(i+1, summary.TotalFiles, file.FullPath, &result)
		}

		// Check for audit write failure - fail-fast
		// Requirements: 11.1 - halt all file operations if audit write fails
		if result.Error != nil && isAuditError(result.Error) {
			auditError = result.Error
			break
		}
	}

	// End the audit run with summary
	if auditWriter != nil {
		runStatus := audit.RunStatusCompleted
		if auditError != nil {
			runStatus = audit.RunStatusFailed
		} else if len(summary.ScanErrors) > 0 || summary.ErrorCount > 0 {
			runStatus = audit.RunStatusCompleted // Still completed, just with errors
		}

		auditSummary := audit.RunSummary{
			TotalFiles:   summary.TotalFiles,
			Moved:        summary.SuccessCount - summary.ReviewCount,
			Skipped:      summary.SkippedCount,
			RoutedReview: summary.ReviewCount,
			Duplicates:   summary.DuplicateCount,
			Errors:       summary.ErrorCount,
		}

		if err := auditWriter.EndRun(runID, runStatus, auditSummary); err != nil {
			// If we can't write the end run event, return the error
			if auditError == nil {
				auditError = fmt.Errorf("failed to end audit run: %w", err)
			}
		}
	}

	// If there was an audit error, return it
	if auditError != nil {
		return summary, auditError
	}

	return summary, nil
}

// processFile classifies and organizes a single file.
func processFile(file scanner.FileEntry, cfg *config.Configuration) Result {
	return processFileWithAudit(file, cfg, nil, nil)
}

// processFileWithAudit classifies and organizes a single file with optional audit support.
// If auditWriter is provided, it records audit events for each operation.
// Requirements: 11.4 - audit record must be durably written before file move
func processFileWithAudit(file scanner.FileEntry, cfg *config.Configuration, auditWriter *audit.AuditWriter, identityResolver *audit.IdentityResolver) Result {
	// Classify the file
	classification := classifier.Classify(file.Name, cfg.PrefixRules)

	// Capture file identity before any operation (if auditing is enabled)
	var fileIdentity *audit.FileIdentity
	if auditWriter != nil && identityResolver != nil {
		var err error
		fileIdentity, err = identityResolver.CaptureIdentity(file.FullPath)
		if err != nil {
			// Record error event and return
			auditErr := auditWriter.RecordError(file.FullPath, "IDENTITY_CAPTURE_FAILED", err.Error(), "capture_identity")
			if auditErr != nil {
				return Result{
					SourcePath: file.FullPath,
					Success:    false,
					Error:      &AuditWriteError{Err: auditErr},
					EventType:  "ERROR",
				}
			}
			return Result{
				SourcePath: file.FullPath,
				Success:    false,
				Error:      err,
				EventType:  "ERROR",
			}
		}
	}

	// Handle unclassified files - route to review
	if classification.IsUnclassified() {
		// Determine reason code based on classification reason
		reasonCode := mapClassificationReasonToAuditReason(classification.Reason)

		// For unclassified files, we route to review directory
		destDir := organizer.GetForReviewPath(filepath.Dir(file.FullPath))
		destPath := filepath.Join(destDir, file.Name)

		// Record audit event BEFORE the move (Requirements: 11.4)
		if auditWriter != nil {
			if err := auditWriter.RecordRouteToReview(file.FullPath, destPath, reasonCode); err != nil {
				return Result{
					SourcePath: file.FullPath,
					Success:    false,
					Error:      &AuditWriteError{Err: err},
					EventType:  "ERROR",
				}
			}
		}

		// Now perform the actual move
		moveResult, err := organizer.Organize(file, classification, cfg)
		if err != nil {
			// Record error event
			if auditWriter != nil {
				auditWriter.RecordError(file.FullPath, "MOVE_FAILED", err.Error(), "organize")
			}
			return Result{
				SourcePath: file.FullPath,
				Success:    false,
				Error:      err,
				EventType:  "ERROR",
			}
		}

		return Result{
			SourcePath:      moveResult.SourcePath,
			DestinationPath: moveResult.DestinationPath,
			Success:         true,
			EventType:       "ROUTE_TO_REVIEW",
			ReasonCode:      string(reasonCode),
		}
	}

	// Handle classified files - move to destination
	// Record audit event BEFORE the move (Requirements: 11.4)
	if auditWriter != nil {
		// We need to predict the destination path before the move
		// This is calculated the same way as in organizer.Organize
		prefix := extractPrefixFromNormalisedFilename(classification.NormalisedFilename)
		subfolder := fmt.Sprintf("%d %s", classification.Year, prefix)
		destDir := filepath.Join(classification.OutboundDirectory, subfolder)
		destFilename := classification.NormalisedFilename

		// Check if this will be a duplicate
		destPath := filepath.Join(destDir, destFilename)
		isDuplicate := organizer.FileExists(destPath)

		if isDuplicate {
			// Generate the duplicate name to predict actual destination
			actualFilename := organizer.GenerateDuplicateName(destDir, destFilename)
			actualDestPath := filepath.Join(destDir, actualFilename)

			// Record duplicate event
			if err := auditWriter.RecordDuplicate(file.FullPath, destPath, actualDestPath, audit.ReasonDuplicateRenamed); err != nil {
				return Result{
					SourcePath: file.FullPath,
					Success:    false,
					Error:      &AuditWriteError{Err: err},
					EventType:  "ERROR",
				}
			}
		} else {
			// Record move event
			if err := auditWriter.RecordMove(file.FullPath, destPath, fileIdentity); err != nil {
				return Result{
					SourcePath: file.FullPath,
					Success:    false,
					Error:      &AuditWriteError{Err: err},
					EventType:  "ERROR",
				}
			}
		}
	}

	// Organize (move) the file
	moveResult, err := organizer.Organize(file, classification, cfg)
	if err != nil {
		// Record error event
		if auditWriter != nil {
			auditWriter.RecordError(file.FullPath, "MOVE_FAILED", err.Error(), "organize")
		}
		return Result{
			SourcePath: file.FullPath,
			Success:    false,
			Error:      err,
			EventType:  "ERROR",
		}
	}

	eventType := "MOVE"
	if moveResult.IsDuplicate {
		eventType = "DUPLICATE_DETECTED"
	}

	return Result{
		SourcePath:      moveResult.SourcePath,
		DestinationPath: moveResult.DestinationPath,
		Success:         true,
		IsDuplicate:     moveResult.IsDuplicate,
		OriginalName:    moveResult.OriginalName,
		EventType:       eventType,
	}
}

// extractPrefixFromNormalisedFilename extracts the prefix portion from a normalised filename.
// The prefix is everything before the first space.
func extractPrefixFromNormalisedFilename(filename string) string {
	for i, c := range filename {
		if c == ' ' {
			return filename[:i]
		}
	}
	return filename
}

// mapClassificationReasonToAuditReason maps classifier reason to audit reason code.
func mapClassificationReasonToAuditReason(reason classifier.UnclassifiedReason) audit.ReasonCode {
	switch reason {
	case classifier.NoPrefixMatch:
		return audit.ReasonUnclassified
	case classifier.MissingDelimiter:
		return audit.ReasonParseError
	case classifier.InvalidDate:
		return audit.ReasonInvalidDate
	default:
		return audit.ReasonUnclassified
	}
}

// AuditWriteError wraps an audit write error to distinguish it from other errors.
type AuditWriteError struct {
	Err error
}

func (e *AuditWriteError) Error() string {
	return fmt.Sprintf("audit write failed: %v", e.Err)
}

func (e *AuditWriteError) Unwrap() error {
	return e.Err
}

// isAuditError checks if an error is an audit write error.
func isAuditError(err error) bool {
	_, ok := err.(*AuditWriteError)
	return ok
}

// getMachineID returns a stable machine identifier.
func getMachineID() string {
	// Try to get hostname as a simple machine identifier
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// HasErrors returns true if there were any errors during the run.
func (s *Summary) HasErrors() bool {
	return s.ErrorCount > 0 || len(s.ScanErrors) > 0
}

// PrintSummary returns a formatted summary string.
func (s *Summary) PrintSummary() string {
	if s.DuplicateCount > 0 {
		return fmt.Sprintf("Processed %d files: %d successful (%d duplicates), %d errors",
			s.TotalFiles, s.SuccessCount, s.DuplicateCount, s.ErrorCount)
	}
	return fmt.Sprintf("Processed %d files: %d successful, %d errors",
		s.TotalFiles, s.SuccessCount, s.ErrorCount)
}
