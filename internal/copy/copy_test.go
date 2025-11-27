package copy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindMatches_GlobPatternWithTrailingSlash(t *testing.T) {
	// Create a temp directory structure
	tmpDir := t.TempDir()

	// Create .turbo directories at different levels
	dirs := []string{
		".turbo",
		"packages/app/.turbo",
		"packages/lib/.turbo",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
		// Create a file inside to make it non-empty
		if err := os.WriteFile(filepath.Join(fullPath, "cache.json"), []byte("{}"), 0644); err != nil {
			t.Fatalf("failed to create file in %s: %v", dir, err)
		}
	}

	tests := []struct {
		name     string
		pattern  string
		wantLen  int
		wantDirs []string
	}{
		{
			name:     "glob pattern with trailing slash",
			pattern:  "**/.turbo/",
			wantLen:  3,
			wantDirs: []string{".turbo", "packages/app/.turbo", "packages/lib/.turbo"},
		},
		{
			name:     "glob pattern without trailing slash",
			pattern:  "**/.turbo",
			wantLen:  3,
			wantDirs: []string{".turbo", "packages/app/.turbo", "packages/lib/.turbo"},
		},
		{
			name:     "literal pattern with trailing slash",
			pattern:  ".turbo/",
			wantLen:  1,
			wantDirs: []string{".turbo/"},
		},
		{
			name:     "literal pattern without trailing slash",
			pattern:  ".turbo",
			wantLen:  1,
			wantDirs: []string{".turbo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := findMatches(tmpDir, tt.pattern)
			if err != nil {
				t.Fatalf("findMatches failed: %v", err)
			}
			if len(matches) != tt.wantLen {
				t.Errorf("got %d matches, want %d. Matches: %v", len(matches), tt.wantLen, matches)
			}
		})
	}
}

func TestFindMatches_LiteralHiddenDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create hidden directories
	dirs := []string{".certs", ".claude", ".vscode", "attachments"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	tests := []struct {
		pattern string
		exists  bool
	}{
		{".certs", true},
		{".certs/", true},
		{".claude", true},
		{"attachments", true},
		{"attachments/", true},
		{".nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			matches, err := findMatches(tmpDir, tt.pattern)
			if err != nil {
				t.Fatalf("findMatches failed: %v", err)
			}
			if tt.exists && len(matches) == 0 {
				t.Errorf("expected match for %q, got none", tt.pattern)
			}
			if !tt.exists && len(matches) > 0 {
				t.Errorf("expected no match for %q, got %v", tt.pattern, matches)
			}
		})
	}
}

func TestCopyFiles_MergesIntoExistingDir(t *testing.T) {
	// Create source directory with files
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Source has: .certs/untracked.pem
	srcCerts := filepath.Join(srcDir, ".certs")
	if err := os.MkdirAll(srcCerts, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcCerts, "untracked.pem"), []byte("untracked"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcCerts, "tracked.pem"), []byte("src-tracked"), 0644); err != nil {
		t.Fatal(err)
	}

	// Dest already has: .certs/tracked.pem (simulating git checkout)
	destCerts := filepath.Join(destDir, ".certs")
	if err := os.MkdirAll(destCerts, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destCerts, "tracked.pem"), []byte("dest-tracked"), 0644); err != nil {
		t.Fatal(err)
	}

	// Copy should merge, adding untracked.pem
	if err := CopyFiles([]string{".certs"}, srcDir, destDir); err != nil {
		t.Fatalf("CopyFiles failed: %v", err)
	}

	// Verify untracked.pem was copied
	untrackedPath := filepath.Join(destCerts, "untracked.pem")
	if _, err := os.Stat(untrackedPath); os.IsNotExist(err) {
		t.Error("untracked.pem was not copied to existing directory")
	} else {
		content, _ := os.ReadFile(untrackedPath)
		if string(content) != "untracked" {
			t.Errorf("untracked.pem has wrong content: got %q, want %q", string(content), "untracked")
		}
	}

	// Verify tracked.pem was overwritten with src content
	trackedPath := filepath.Join(destCerts, "tracked.pem")
	content, err := os.ReadFile(trackedPath)
	if err != nil {
		t.Fatalf("failed to read tracked.pem: %v", err)
	}
	if string(content) != "src-tracked" {
		t.Errorf("tracked.pem should be overwritten: got %q, want %q", string(content), "src-tracked")
	}
}
