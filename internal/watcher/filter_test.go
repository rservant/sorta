package watcher

import (
	"testing"
)

func TestDefaultIgnorePatterns(t *testing.T) {
	patterns := DefaultIgnorePatterns()

	// Verify default patterns include the required ones
	required := []string{"*.tmp", "*.part", "*.download"}
	for _, req := range required {
		found := false
		for _, p := range patterns {
			if p == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultIgnorePatterns() missing required pattern %q", req)
		}
	}
}

func TestNewFileFilter_WithNilPatterns(t *testing.T) {
	filter := NewFileFilter(nil)
	patterns := filter.GetPatterns()

	if len(patterns) == 0 {
		t.Error("NewFileFilter(nil) should use default patterns")
	}
}

func TestNewFileFilter_WithEmptyPatterns(t *testing.T) {
	filter := NewFileFilter([]string{})
	patterns := filter.GetPatterns()

	if len(patterns) == 0 {
		t.Error("NewFileFilter([]) should use default patterns")
	}
}

func TestNewFileFilter_WithCustomPatterns(t *testing.T) {
	custom := []string{"*.bak", "*.swp"}
	filter := NewFileFilter(custom)
	patterns := filter.GetPatterns()

	if len(patterns) != 2 {
		t.Errorf("NewFileFilter(custom) got %d patterns, want 2", len(patterns))
	}
}

func TestFileFilter_ShouldIgnore_TmpFiles(t *testing.T) {
	filter := NewFileFilter(nil) // Use defaults

	tests := []struct {
		path     string
		expected bool
	}{
		// .tmp files should be ignored
		{"/path/to/file.tmp", true},
		{"file.tmp", true},
		{"document.tmp", true},

		// .part files should be ignored
		{"/downloads/video.part", true},
		{"archive.part", true},

		// .download files should be ignored
		{"/home/user/file.download", true},
		{"image.download", true},

		// .crdownload (Chrome) should be ignored
		{"file.crdownload", true},

		// .partial files should be ignored
		{"data.partial", true},

		// Regular files should NOT be ignored
		{"/path/to/document.pdf", false},
		{"image.jpg", false},
		{"video.mp4", false},
		{"archive.zip", false},
		{"readme.txt", false},

		// Files with similar but different extensions should NOT be ignored
		{"file.template", false},
		{"file.party", false},
		{"file.downloader", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := filter.ShouldIgnore(tt.path)
			if got != tt.expected {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestFileFilter_ShouldIgnore_CustomPatterns(t *testing.T) {
	filter := NewFileFilter([]string{"*.bak", "*.swp", "~*"})

	tests := []struct {
		path     string
		expected bool
	}{
		// Custom patterns should be matched
		{"file.bak", true},
		{"/path/to/document.bak", true},
		{"editor.swp", true},
		{"~tempfile", true},

		// Default patterns should NOT be matched (custom patterns replace defaults)
		{"file.tmp", false},
		{"file.part", false},
		{"file.download", false},

		// Regular files should NOT be ignored
		{"document.pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := filter.ShouldIgnore(tt.path)
			if got != tt.expected {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestFileFilter_ShouldIgnore_GlobPatterns(t *testing.T) {
	filter := NewFileFilter([]string{"*.tmp", "temp_*", "??.bak"})

	tests := []struct {
		path     string
		expected bool
	}{
		// Suffix glob pattern
		{"file.tmp", true},
		{"document.tmp", true},

		// Prefix glob pattern
		{"temp_file.txt", true},
		{"temp_data.csv", true},

		// Single character wildcard
		{"ab.bak", true},
		{"xy.bak", true},
		{"abc.bak", false}, // 3 chars, doesn't match ??

		// Non-matching
		{"regular.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := filter.ShouldIgnore(tt.path)
			if got != tt.expected {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestFileFilter_AddPattern(t *testing.T) {
	filter := NewFileFilter([]string{"*.tmp"})

	// Initially should not ignore .bak
	if filter.ShouldIgnore("file.bak") {
		t.Error("Should not ignore .bak before adding pattern")
	}

	// Add pattern
	filter.AddPattern("*.bak")

	// Now should ignore .bak
	if !filter.ShouldIgnore("file.bak") {
		t.Error("Should ignore .bak after adding pattern")
	}
}

func TestFileFilter_GetPatterns_ReturnsCopy(t *testing.T) {
	original := []string{"*.tmp", "*.part"}
	filter := NewFileFilter(original)

	patterns := filter.GetPatterns()
	patterns[0] = "modified"

	// Original should not be affected
	got := filter.GetPatterns()
	if got[0] == "modified" {
		t.Error("GetPatterns() should return a copy, not the original slice")
	}
}

func TestIsTemporaryFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"file.tmp", true},
		{"file.part", true},
		{"file.download", true},
		{"file.pdf", false},
		{"document.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsTemporaryFile(tt.path)
			if got != tt.expected {
				t.Errorf("IsTemporaryFile(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestFileFilter_ShouldIgnore_HiddenTempFiles(t *testing.T) {
	filter := NewFileFilter(nil) // Use defaults which include ".~*"

	tests := []struct {
		path     string
		expected bool
	}{
		// Hidden temp files starting with .~
		{".~lock.file", true},
		{".~temp", true},

		// Regular hidden files should NOT be ignored
		{".gitignore", false},
		{".bashrc", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := filter.ShouldIgnore(tt.path)
			if got != tt.expected {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}
