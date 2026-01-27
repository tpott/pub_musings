package pathvalidator

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	// Test creating with valid directories
	v, err := New("uploads", "data")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if len(v.baseDirs) != 2 {
		t.Errorf("Expected 2 base dirs, got %d", len(v.baseDirs))
	}

	// Verify paths are absolute
	for _, dir := range v.baseDirs {
		if !filepath.IsAbs(dir) {
			t.Errorf("Base dir should be absolute: %s", dir)
		}
	}
}

func TestValidatePath(t *testing.T) {
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "uploads")
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(uploadsDir, 0755)
	os.MkdirAll(dataDir, 0755)

	v, err := New(uploadsDir, dataDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{
			name:    "empty path",
			path:    "",
			wantErr: ErrEmptyPath,
		},
		{
			name:    "simple relative path",
			path:    "video.mp4",
			wantErr: nil,
		},
		{
			name:    "nested relative path",
			path:    "chunks/abc123/chunk_0.part",
			wantErr: nil,
		},
		{
			name:    "path traversal - double dot",
			path:    "../etc/passwd",
			wantErr: ErrPathTraversal,
		},
		{
			name:    "path traversal - middle of path",
			path:    "uploads/../etc/passwd",
			wantErr: ErrPathTraversal,
		},
		{
			name:    "path traversal - looks encoded but starts with dots",
			path:    "..%2F..%2Fetc/passwd",
			wantErr: ErrPathTraversal, // Caught because it starts with ".."
		},
		{
			name:    "path with null byte",
			path:    "video\x00.mp4",
			wantErr: ErrPathTraversal,
		},
		{
			name:    "absolute path within allowed",
			path:    filepath.Join(uploadsDir, "video.mp4"),
			wantErr: nil,
		},
		{
			name:    "absolute path outside allowed",
			path:    "/etc/passwd",
			wantErr: ErrOutsideBaseDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidatePath(tt.path)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSafeJoin(t *testing.T) {
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "uploads")
	os.MkdirAll(uploadsDir, 0755)

	v, err := New(uploadsDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name       string
		baseDir    string
		components []string
		wantSuffix string
		wantErr    error
	}{
		{
			name:       "simple join",
			baseDir:    uploadsDir,
			components: []string{"video.mp4"},
			wantSuffix: "video.mp4",
			wantErr:    nil,
		},
		{
			name:       "nested join",
			baseDir:    uploadsDir,
			components: []string{"chunks", "session123", "chunk_0.part"},
			wantSuffix: "chunks/session123/chunk_0.part",
			wantErr:    nil,
		},
		{
			name:       "traversal attempt via component",
			baseDir:    uploadsDir,
			components: []string{"..", "etc", "passwd"},
			wantErr:    ErrPathTraversal,
		},
		{
			name:       "traversal in nested component",
			baseDir:    uploadsDir,
			components: []string{"chunks", "../../../etc", "passwd"},
			wantErr:    ErrPathTraversal,
		},
		{
			name:       "base dir outside allowed",
			baseDir:    "/etc",
			components: []string{"passwd"},
			wantErr:    ErrOutsideBaseDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.SafeJoin(tt.baseDir, tt.components...)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("SafeJoin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && !filepath.IsAbs(result) {
				t.Errorf("SafeJoin() returned non-absolute path: %s", result)
			}
			if err == nil && tt.wantSuffix != "" {
				if !containsSuffix(result, tt.wantSuffix) {
					t.Errorf("SafeJoin() = %s, want suffix %s", result, tt.wantSuffix)
				}
			}
		})
	}
}

func TestValidateAbsolutePath(t *testing.T) {
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "uploads")
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(uploadsDir, 0755)
	os.MkdirAll(dataDir, 0755)

	v, err := New(uploadsDir, dataDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{
			name:    "path in uploads",
			path:    filepath.Join(uploadsDir, "video.mp4"),
			wantErr: nil,
		},
		{
			name:    "path in data",
			path:    filepath.Join(dataDir, "subtitler.db"),
			wantErr: nil,
		},
		{
			name:    "nested path in uploads",
			path:    filepath.Join(uploadsDir, "chunks", "session123", "chunk.part"),
			wantErr: nil,
		},
		{
			name:    "path outside allowed dirs",
			path:    "/etc/passwd",
			wantErr: ErrOutsideBaseDir,
		},
		{
			name:    "path in parent of uploads",
			path:    tempDir,
			wantErr: ErrOutsideBaseDir,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: ErrEmptyPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateAbsolutePath(tt.path)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateAbsolutePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAndResolve(t *testing.T) {
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "uploads")
	os.MkdirAll(uploadsDir, 0755)

	// Create a test file
	testFile := filepath.Join(uploadsDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	v, err := New(uploadsDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{
			name:    "existing file in allowed dir",
			path:    testFile,
			wantErr: nil,
		},
		{
			name:    "non-existing file in allowed dir",
			path:    filepath.Join(uploadsDir, "nonexistent.txt"),
			wantErr: nil, // Non-existence is OK, we just validate the path
		},
		{
			name:    "path outside allowed via traversal",
			path:    filepath.Join(uploadsDir, "..", "outside.txt"),
			wantErr: ErrOutsideBaseDir, // filepath.Join cleans the path, so it's outside, not traversal
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := v.ValidateAndResolve(tt.path)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateAndResolve(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestIsWithin(t *testing.T) {
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "uploads")
	os.MkdirAll(uploadsDir, 0755)

	tests := []struct {
		name    string
		path    string
		baseDir string
		want    bool
	}{
		{
			name:    "file within directory",
			path:    filepath.Join(uploadsDir, "video.mp4"),
			baseDir: uploadsDir,
			want:    true,
		},
		{
			name:    "nested file within directory",
			path:    filepath.Join(uploadsDir, "a", "b", "c.txt"),
			baseDir: uploadsDir,
			want:    true,
		},
		{
			name:    "file outside directory",
			path:    "/etc/passwd",
			baseDir: uploadsDir,
			want:    false,
		},
		{
			name:    "parent directory",
			path:    tempDir,
			baseDir: uploadsDir,
			want:    false,
		},
		{
			name:    "sibling directory",
			path:    filepath.Join(tempDir, "other", "file.txt"),
			baseDir: uploadsDir,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsWithin(tt.path, tt.baseDir)
			if err != nil {
				t.Errorf("IsWithin() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("IsWithin(%q, %q) = %v, want %v", tt.path, tt.baseDir, got, tt.want)
			}
		})
	}
}

func TestPathTraversalAttacks(t *testing.T) {
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "uploads")
	os.MkdirAll(uploadsDir, 0755)

	v, err := New(uploadsDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Various path traversal attack patterns
	attacks := []string{
		"../etc/passwd",
		"..\\etc\\passwd",
		"....//....//etc/passwd",
		"%2e%2e%2f",        // URL encoded ../
		"..%252f",          // Double URL encoded
		"..;/etc/passwd",   // Null-byte-like
		"..%00/etc/passwd", // Null byte
		"uploads/../../../etc/passwd",
		".\\..\\..\\etc\\passwd",
		"chunks/../../etc/passwd",
	}

	for _, attack := range attacks {
		t.Run(attack, func(t *testing.T) {
			// ValidatePath should catch these
			err := v.ValidatePath(attack)
			if err == nil {
				// If ValidatePath passes, SafeJoin should still catch it
				_, err = v.SafeJoin(uploadsDir, attack)
				if err == nil {
					// Check if the result is actually outside the base directory
					result := filepath.Clean(filepath.Join(uploadsDir, attack))
					absResult, _ := filepath.Abs(result)
					if !isWithinDir(absResult, uploadsDir) {
						t.Errorf("Attack %q escaped base directory: %s", attack, absResult)
					}
				}
			}
		})
	}
}

func TestContainsTraversalPatterns(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"video.mp4", false},
		{"chunks/session/chunk.part", false},
		{"../evil", true},
		{"a/../b", true},
		{"a/..b", false}, // ..b is a valid name, not traversal
		{"a\x00b", true}, // Null byte
		{"..", true},
		{"...", false}, // Three dots is a valid name
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := containsTraversalPatterns(tt.path)
			if got != tt.want {
				t.Errorf("containsTraversalPatterns(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsWithinDir(t *testing.T) {
	tests := []struct {
		path string
		dir  string
		want bool
	}{
		{"/foo/bar", "/foo", true},
		{"/foo/bar/baz", "/foo", true},
		{"/foo", "/foo", true},
		{"/foobar", "/foo", false}, // foobar is not within foo
		{"/foo/bar", "/foo/bar", true},
		{"/bar", "/foo", false},
		{"/foo/bar", "/foo/baz", false},
	}

	for _, tt := range tests {
		t.Run(tt.path+"_"+tt.dir, func(t *testing.T) {
			got := isWithinDir(tt.path, tt.dir)
			if got != tt.want {
				t.Errorf("isWithinDir(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
			}
		})
	}
}

func TestMustNew(t *testing.T) {
	// Should not panic with valid directories
	v := MustNew("uploads", "data")
	if v == nil {
		t.Error("MustNew returned nil")
	}
}

// Helper function to check if path ends with suffix
func containsSuffix(path, suffix string) bool {
	// Normalize separators
	path = filepath.ToSlash(path)
	suffix = filepath.ToSlash(suffix)
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}
