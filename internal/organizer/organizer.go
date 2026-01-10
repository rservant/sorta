// Package organizer handles file movement and organization for Sorta.
package organizer

import (
	"fmt"
	"os"
	"path/filepath"

	"sorta/internal/classifier"
	"sorta/internal/config"
	"sorta/internal/scanner"
)

// MoveErrorType represents the type of move error.
type MoveErrorType string

const (
	// SourceNotFound indicates the source file does not exist.
	SourceNotFound MoveErrorType = "SOURCE_NOT_FOUND"
	// DestinationExists indicates a file already exists at the destination.
	DestinationExists MoveErrorType = "DESTINATION_EXISTS"
	// PermissionDenied indicates insufficient permissions for the operation.
	PermissionDenied MoveErrorType = "PERMISSION_DENIED"
)

// MoveError represents an error that occurred during file movement.
type MoveError struct {
	Type MoveErrorType
	Path string
	Err  error
}

func (e *MoveError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Type, e.Path, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Path)
}

func (e *MoveError) Unwrap() error {
	return e.Err
}

// MoveResult represents the result of a successful file move operation.
type MoveResult struct {
	SourcePath      string
	DestinationPath string
	IsDuplicate     bool   // True if the file was renamed due to a duplicate
	OriginalName    string // Original filename before duplicate renaming (empty if not a duplicate)
}

// Organize moves a file to its appropriate destination based on classification.
// For CLASSIFIED files: moves to <targetDir>/<year> <prefix>/<normalisedFilename>
// For UNCLASSIFIED files: moves to for-review subdirectory within the source directory
// If a file with the same name exists at the destination, it will be renamed with a duplicate suffix.
func Organize(file scanner.FileEntry, classification *classifier.Classification, cfg *config.Configuration) (*MoveResult, error) {
	var destDir string
	var destFilename string

	if classification.IsClassified() {
		// Build destination path: <targetDir>/<year> <prefix>/
		// Extract the canonical prefix from the normalised filename
		// The normalised filename starts with the canonical prefix
		prefix := extractPrefixFromNormalisedFilename(classification.NormalisedFilename)
		subfolder := fmt.Sprintf("%d %s", classification.Year, prefix)
		destDir = filepath.Join(classification.TargetDirectory, subfolder)
		destFilename = classification.NormalisedFilename
	} else {
		// Move to for-review subdirectory within the source directory
		destDir = GetForReviewPath(filepath.Dir(file.FullPath))
		destFilename = file.Name
	}

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destDir, 0755); err != nil {
		if os.IsPermission(err) {
			return nil, &MoveError{
				Type: PermissionDenied,
				Path: destDir,
				Err:  err,
			}
		}
		return nil, err
	}

	// Check if source exists
	if _, err := os.Stat(file.FullPath); os.IsNotExist(err) {
		return nil, &MoveError{
			Type: SourceNotFound,
			Path: file.FullPath,
			Err:  err,
		}
	}

	// Handle duplicate files - generate unique name if destination exists
	originalFilename := destFilename
	isDuplicate := false
	if FileExists(filepath.Join(destDir, destFilename)) {
		destFilename = GenerateDuplicateName(destDir, destFilename)
		isDuplicate = true
	}

	destPath := filepath.Join(destDir, destFilename)

	// Move the file (rename)
	if err := os.Rename(file.FullPath, destPath); err != nil {
		if os.IsPermission(err) {
			return nil, &MoveError{
				Type: PermissionDenied,
				Path: file.FullPath,
				Err:  err,
			}
		}
		// If rename fails (e.g., cross-device), fall back to copy+delete
		if err := copyAndDelete(file.FullPath, destPath); err != nil {
			return nil, err
		}
	}

	result := &MoveResult{
		SourcePath:      file.FullPath,
		DestinationPath: destPath,
		IsDuplicate:     isDuplicate,
	}
	if isDuplicate {
		result.OriginalName = originalFilename
	}

	return result, nil
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

// copyAndDelete copies a file to a new location and deletes the original.
// Used as a fallback when os.Rename fails (e.g., cross-device moves).
func copyAndDelete(src, dst string) error {
	// Read source file
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return &MoveError{
				Type: SourceNotFound,
				Path: src,
				Err:  err,
			}
		}
		if os.IsPermission(err) {
			return &MoveError{
				Type: PermissionDenied,
				Path: src,
				Err:  err,
			}
		}
		return err
	}

	// Get source file permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Write to destination
	if err := os.WriteFile(dst, data, srcInfo.Mode()); err != nil {
		if os.IsPermission(err) {
			return &MoveError{
				Type: PermissionDenied,
				Path: dst,
				Err:  err,
			}
		}
		return err
	}

	// Delete source
	if err := os.Remove(src); err != nil {
		// If we can't delete source, try to clean up destination
		os.Remove(dst)
		if os.IsPermission(err) {
			return &MoveError{
				Type: PermissionDenied,
				Path: src,
				Err:  err,
			}
		}
		return err
	}

	return nil
}

// GetForReviewPath returns the for-review subdirectory for a source directory.
// The for-review directory is created within each source directory to hold
// unclassified files, keeping them close to their source location.
func GetForReviewPath(sourceDir string) string {
	return filepath.Join(sourceDir, "for-review")
}
