// Package audit provides audit trail functionality for Sorta file operations.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RotationIndex tracks all log segments for discovery.
// Requirements: 9.4
type RotationIndex struct {
	Segments    []SegmentInfo `json:"segments"`
	ActiveLog   string        `json:"activeLog"`
	LastUpdated time.Time     `json:"lastUpdated"`
}

// SegmentInfo contains metadata about a rotated log segment.
type SegmentInfo struct {
	Filename  string    `json:"filename"`
	CreatedAt time.Time `json:"createdAt"`
	Size      int64     `json:"size"`
}

// RotationManager handles log rotation logic.
// Requirements: 9.1, 9.2, 9.3, 9.4, 9.6
type RotationManager struct {
	config       AuditConfig
	lastRotation time.Time
}

// NewRotationManager creates a new RotationManager with the given configuration.
func NewRotationManager(config AuditConfig) *RotationManager {
	return &RotationManager{
		config:       config,
		lastRotation: time.Now(),
	}
}

// NeedsRotation checks if the current log file needs rotation based on size or time.
// Requirements: 9.1, 9.2
func (rm *RotationManager) NeedsRotation(logPath string) (bool, error) {
	// Check if file exists
	info, err := os.Stat(logPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to stat log file: %w", err)
	}

	// Check size-based rotation
	if rm.config.RotationSize > 0 && info.Size() >= rm.config.RotationSize {
		return true, nil
	}

	// Check time-based rotation
	if rm.config.RotationPeriod != "" {
		needsTimeRotation, err := rm.needsTimeBasedRotation(info.ModTime())
		if err != nil {
			return false, err
		}
		if needsTimeRotation {
			return true, nil
		}
	}

	return false, nil
}

// needsTimeBasedRotation checks if rotation is needed based on time period.
func (rm *RotationManager) needsTimeBasedRotation(lastModTime time.Time) (bool, error) {
	now := time.Now()

	switch rm.config.RotationPeriod {
	case "daily":
		// Rotate if the file was last modified on a different day
		lastDay := lastModTime.Truncate(24 * time.Hour)
		today := now.Truncate(24 * time.Hour)
		return lastDay.Before(today), nil

	case "weekly":
		// Rotate if the file was last modified in a different week
		_, lastWeek := lastModTime.ISOWeek()
		_, currentWeek := now.ISOWeek()
		lastYear := lastModTime.Year()
		currentYear := now.Year()
		return lastYear != currentYear || lastWeek != currentWeek, nil

	case "":
		return false, nil

	default:
		return false, fmt.Errorf("unknown rotation period: %s", rm.config.RotationPeriod)
	}
}

// GenerateRotatedFilename creates a filename for a rotated log segment.
// Format: sorta-audit-YYYYMMDD-HHMMSS-NNN.jsonl (with milliseconds for uniqueness)
// Requirements: 9.3
func (rm *RotationManager) GenerateRotatedFilename() string {
	now := time.Now()
	return fmt.Sprintf("sorta-audit-%s-%03d.jsonl", now.Format("20060102-150405"), now.Nanosecond()/1000000)
}

// Rotate performs the log rotation.
// It renames the current log file and updates the index.
// Requirements: 9.3, 9.4, 9.6
func (rm *RotationManager) Rotate(logPath string) (string, error) {
	rotatedFilename := rm.GenerateRotatedFilename()
	return rm.RotateWithFilename(logPath, rotatedFilename)
}

// RotateWithFilename performs the log rotation with a specific filename.
// This is used when the filename needs to be consistent with a previously written ROTATION event.
// Requirements: 9.3, 9.4, 9.6
func (rm *RotationManager) RotateWithFilename(logPath, rotatedFilename string) (string, error) {
	// Generate new filename for the rotated segment
	dir := filepath.Dir(logPath)
	rotatedPath := filepath.Join(dir, rotatedFilename)

	// Get current file info for the index
	info, err := os.Stat(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat log file for rotation: %w", err)
	}

	// Rename current log to rotated filename
	if err := os.Rename(logPath, rotatedPath); err != nil {
		return "", fmt.Errorf("failed to rename log file during rotation: %w", err)
	}

	// Update the rotation index
	if err := rm.updateIndex(dir, rotatedFilename, info.Size()); err != nil {
		// Log the error but don't fail - the rotation itself succeeded
		// The index can be rebuilt from the filesystem if needed
		fmt.Fprintf(os.Stderr, "warning: failed to update rotation index: %v\n", err)
	}

	rm.lastRotation = time.Now()
	return rotatedPath, nil
}

// updateIndex updates or creates the rotation index file.
// Requirements: 9.4
func (rm *RotationManager) updateIndex(logDir, rotatedFilename string, size int64) error {
	indexPath := filepath.Join(logDir, "sorta-audit-index.json")

	// Load existing index or create new one
	index, err := rm.loadIndex(indexPath)
	if err != nil {
		// If index doesn't exist or is corrupt, create a new one
		index = &RotationIndex{
			Segments:  []SegmentInfo{},
			ActiveLog: "sorta-audit.jsonl",
		}
	}

	// Add the new segment
	segment := SegmentInfo{
		Filename:  rotatedFilename,
		CreatedAt: time.Now(),
		Size:      size,
	}
	index.Segments = append(index.Segments, segment)
	index.LastUpdated = time.Now()

	// Write the updated index
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	return nil
}

// loadIndex loads the rotation index from disk.
func (rm *RotationManager) loadIndex(indexPath string) (*RotationIndex, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	var index RotationIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	return &index, nil
}

// LoadIndex loads the rotation index from the log directory.
// This is a public method for external use.
func LoadIndex(logDir string) (*RotationIndex, error) {
	indexPath := filepath.Join(logDir, "sorta-audit-index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	var index RotationIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	return &index, nil
}

// DiscoverSegments finds all log segments in the directory.
// This can be used to rebuild the index or when the index is missing.
// Requirements: 9.4
func DiscoverSegments(logDir string) ([]string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read log directory: %w", err)
	}

	var segments []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Match rotated segments: sorta-audit-YYYYMMDD-HHMMSS.jsonl
		if strings.HasPrefix(name, "sorta-audit-") && strings.HasSuffix(name, ".jsonl") && name != "sorta-audit.jsonl" {
			segments = append(segments, name)
		}
	}

	// Sort segments chronologically (oldest first)
	sort.Strings(segments)

	return segments, nil
}

// GetAllLogFiles returns all log files in chronological order (oldest first).
// This includes both rotated segments and the active log.
// Requirements: 9.4, 9.5
func GetAllLogFiles(logDir string) ([]string, error) {
	segments, err := DiscoverSegments(logDir)
	if err != nil {
		return nil, err
	}

	// Convert to full paths
	var files []string
	for _, seg := range segments {
		files = append(files, filepath.Join(logDir, seg))
	}

	// Add active log if it exists
	activeLog := filepath.Join(logDir, "sorta-audit.jsonl")
	if _, err := os.Stat(activeLog); err == nil {
		files = append(files, activeLog)
	}

	return files, nil
}

// CreateRotationEvent creates a ROTATION event to be written before switching files.
// Requirements: 9.6
func CreateRotationEvent(runID RunID, oldFile, newFile string) AuditEvent {
	return AuditEvent{
		Timestamp: time.Now().UTC(),
		RunID:     runID,
		EventType: EventRotation,
		Status:    StatusSuccess,
		Metadata: map[string]string{
			"previousFile": oldFile,
			"newFile":      newFile,
			"reason":       "rotation",
		},
	}
}
