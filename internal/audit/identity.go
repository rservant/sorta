package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// IdentityMatch represents the result of identity verification.
type IdentityMatch int

const (
	// IdentityMatches indicates the file matches the expected identity.
	IdentityMatches IdentityMatch = iota
	// IdentityHashMismatch indicates the content hash does not match.
	IdentityHashMismatch
	// IdentitySizeMismatch indicates the file size does not match.
	IdentitySizeMismatch
	// IdentityNotFound indicates the file was not found.
	IdentityNotFound
)

// IdentityResolver provides methods for capturing and verifying file identity.
type IdentityResolver struct{}

// NewIdentityResolver creates a new IdentityResolver instance.
func NewIdentityResolver() *IdentityResolver {
	return &IdentityResolver{}
}

// CaptureIdentity captures the identity of a file at the given path.
// It computes the SHA-256 hash, file size, and modification time.
// Requirements: 4.1, 4.2, 4.3
func (r *IdentityResolver) CaptureIdentity(path string) (*FileIdentity, error) {
	// Get file info for size and mod time
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file")
	}

	// Compute SHA-256 hash
	hash, err := computeSHA256(path)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash: %w", err)
	}

	return &FileIdentity{
		ContentHash: hash,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
	}, nil
}

// VerifyIdentity compares a file at the given path against an expected identity.
// It returns the match result indicating whether the file matches or why it doesn't.
// Requirements: 4.6
func (r *IdentityResolver) VerifyIdentity(path string, expected FileIdentity) (IdentityMatch, error) {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return IdentityNotFound, nil
		}
		return IdentityNotFound, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check size first (fast check)
	if info.Size() != expected.Size {
		return IdentitySizeMismatch, nil
	}

	// Compute and compare hash
	hash, err := computeSHA256(path)
	if err != nil {
		return IdentityNotFound, fmt.Errorf("failed to compute hash: %w", err)
	}

	if hash != expected.ContentHash {
		return IdentityHashMismatch, nil
	}

	return IdentityMatches, nil
}

// FindByHash searches the given directories for files matching the specified content hash.
// It returns a list of paths to files with matching hashes.
// Requirements: 4.6
func (r *IdentityResolver) FindByHash(hash string, searchDirs []string) ([]string, error) {
	var matches []string

	for _, dir := range searchDirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// Skip files/directories we can't access
				return nil
			}

			if info.IsDir() {
				return nil
			}

			fileHash, err := computeSHA256(path)
			if err != nil {
				// Skip files we can't read
				return nil
			}

			if fileHash == hash {
				matches = append(matches, path)
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory %s: %w", dir, err)
		}
	}

	return matches, nil
}

// computeSHA256 computes the SHA-256 hash of a file and returns it as a hex string.
func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
