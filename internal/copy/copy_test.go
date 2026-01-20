package copy

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestFindMatches_GlobPatternWithTrailingSlash(t *testing.T) {
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
		if err := os.WriteFile(filepath.Join(fullPath, "cache.json"), []byte("{}"), 0644); err != nil {
			t.Fatalf("failed to create file in %s: %v", dir, err)
		}
	}

	tests := []struct {
		name     string
		pattern  string
		wantDirs []string
	}{
		{
			name:     "glob pattern with trailing slash",
			pattern:  "**/.turbo/",
			wantDirs: []string{".turbo", "packages/app/.turbo", "packages/lib/.turbo"},
		},
		{
			name:     "glob pattern without trailing slash",
			pattern:  "**/.turbo",
			wantDirs: []string{".turbo", "packages/app/.turbo", "packages/lib/.turbo"},
		},
		{
			name:     "literal pattern with trailing slash (normalized)",
			pattern:  ".turbo/",
			wantDirs: []string{".turbo"},
		},
		{
			name:     "literal pattern without trailing slash",
			pattern:  ".turbo",
			wantDirs: []string{".turbo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := findMatches(tmpDir, tt.pattern)
			if err != nil {
				t.Fatalf("findMatches failed: %v", err)
			}
			sort.Strings(matches)
			sort.Strings(tt.wantDirs)
			if len(matches) != len(tt.wantDirs) {
				t.Fatalf("got %d matches, want %d. Matches: %v", len(matches), len(tt.wantDirs), matches)
			}
			for i := range matches {
				if matches[i] != tt.wantDirs[i] {
					t.Fatalf("mismatch at %d: got %q, want %q (matches=%v)", i, matches[i], tt.wantDirs[i], matches)
				}
			}
		})
	}
}

func TestFindMatches_LiteralHiddenDirs(t *testing.T) {
	tmpDir := t.TempDir()

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

func TestFilterDescendants(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure simulating node_modules
	dirs := []string{
		"node_modules",
		"node_modules/foo/node_modules",
		"node_modules/bar/node_modules",
		"packages/app/node_modules",
		"packages/lib/node_modules",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Also create a file to test that files are not filtered
	if err := os.WriteFile(filepath.Join(tmpDir, "node_modules", "file.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		matches map[string]bool
		want    []string
	}{
		{
			name: "filters nested node_modules under root",
			matches: map[string]bool{
				"node_modules":                  true,
				"node_modules/foo/node_modules": true,
				"node_modules/bar/node_modules": true,
				"packages/app/node_modules":     true,
				"packages/lib/node_modules":     true,
			},
			want: []string{
				"node_modules",
				"packages/app/node_modules",
				"packages/lib/node_modules",
			},
		},
		{
			name: "keeps all when no nesting",
			matches: map[string]bool{
				"packages/app/node_modules": true,
				"packages/lib/node_modules": true,
			},
			want: []string{
				"packages/app/node_modules",
				"packages/lib/node_modules",
			},
		},
		{
			name: "single path unchanged",
			matches: map[string]bool{
				"node_modules": true,
			},
			want: []string{"node_modules"},
		},
		{
			name: "trailing slash normalized",
			matches: map[string]bool{
				"node_modules/":                 true,
				"node_modules/foo/node_modules": true,
			},
			want: []string{"node_modules"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterDescendants(tt.matches, tmpDir)

			if len(got) != len(tt.want) {
				t.Errorf("got %d paths, want %d. Got: %v, Want: %v", len(got), len(tt.want), got, tt.want)
				return
			}

			gotMap := make(map[string]bool)
			for _, p := range got {
				gotMap[p] = true
			}
			for _, w := range tt.want {
				if !gotMap[w] {
					t.Errorf("missing expected path %q in result %v", w, got)
				}
			}
		})
	}
}

func TestCopyFiles_MergesIntoExistingDir(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

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

	destCerts := filepath.Join(destDir, ".certs")
	if err := os.MkdirAll(destCerts, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destCerts, "tracked.pem"), []byte("dest-tracked"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFiles([]string{".certs"}, srcDir, destDir); err != nil {
		t.Fatalf("CopyFiles failed: %v", err)
	}

	untrackedPath := filepath.Join(destCerts, "untracked.pem")
	if _, err := os.Stat(untrackedPath); os.IsNotExist(err) {
		t.Error("untracked.pem was not copied to existing directory")
	} else {
		content, _ := os.ReadFile(untrackedPath)
		if string(content) != "untracked" {
			t.Errorf("untracked.pem has wrong content: got %q, want %q", string(content), "untracked")
		}
	}

	trackedPath := filepath.Join(destCerts, "tracked.pem")
	content, err := os.ReadFile(trackedPath)
	if err != nil {
		t.Fatalf("failed to read tracked.pem: %v", err)
	}
	if string(content) != "dest-tracked" {
		t.Errorf("tracked.pem should NOT be overwritten: got %q, want %q", string(content), "dest-tracked")
	}
}

func TestCopyFiles_DestinationConflict_FileOverDir(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "conflict"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(destDir, "conflict"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := CopyFiles([]string{"conflict"}, srcDir, destDir); err == nil {
		t.Fatal("expected error due to destination conflict, got nil")
	}
}

func TestCopyFiles_DoesNotFollowSymlink(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create real dir and symlink to it.
	if err := os.MkdirAll(filepath.Join(srcDir, "real"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "real", "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(srcDir, "link")); err != nil {
		t.Fatal(err)
	}

	if err := CopyFiles([]string{"link"}, srcDir, destDir); err != nil {
		t.Fatalf("CopyFiles failed: %v", err)
	}

	info, err := os.Lstat(filepath.Join(destDir, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected dest/link to be a symlink, got mode=%v", info.Mode())
	}

	// Ensure we did not copy the symlink target as a directory/file.
	if _, err := os.Stat(filepath.Join(destDir, "real")); !os.IsNotExist(err) {
		t.Fatalf("expected dest/real to not exist, err=%v", err)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()

	readDone := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		readDone <- b
	}()

	fn()
	_ = w.Close()
	b := <-readDone
	_ = r.Close()
	return string(b)
}

func TestCopyFiles_ProgressToStderr_DeterministicOrder(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}

	out := captureStderr(t, func() {
		if err := CopyFiles([]string{"b.txt", "a.txt"}, srcDir, destDir); err != nil {
			t.Fatalf("CopyFiles failed: %v", err)
		}
	})

	want := "Copied: a.txt\nCopied: b.txt\n"
	if out != want {
		t.Fatalf("unexpected stderr.\nGot:\n%s\nWant:\n%s", out, want)
	}
}

func TestCopyFiles_DestinationConflict_DirOverFile(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(srcDir, "conflict"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "conflict", "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "conflict"), []byte("dest"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFiles([]string{"conflict"}, srcDir, destDir); err == nil {
		t.Fatal("expected error due to destination conflict, got nil")
	}
}

func TestCopyFiles_DirCopy_DoesNotFollowSymlink(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(srcDir, "real"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "real", "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "d"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../real", filepath.Join(srcDir, "d", "link")); err != nil {
		t.Fatal(err)
	}

	if err := CopyFiles([]string{"d"}, srcDir, destDir); err != nil {
		t.Fatalf("CopyFiles failed: %v", err)
	}

	info, err := os.Lstat(filepath.Join(destDir, "d", "link"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected dest/d/link to be a symlink, got mode=%v", info.Mode())
	}

	if _, err := os.Stat(filepath.Join(destDir, "d", "link", "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected dest/d/link/file.txt to not exist (symlink not followed), err=%v", err)
	}
}

func TestCopyFiles_MergeDir_DoesNotFollowSymlink(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(srcDir, "real"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "real", "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "d"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../real", filepath.Join(srcDir, "d", "link")); err != nil {
		t.Fatal(err)
	}

	// force mergeDirContents
	if err := os.MkdirAll(filepath.Join(destDir, "d"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := CopyFiles([]string{"d"}, srcDir, destDir); err != nil {
		t.Fatalf("CopyFiles failed: %v", err)
	}

	info, err := os.Lstat(filepath.Join(destDir, "d", "link"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected dest/d/link to be a symlink, got mode=%v", info.Mode())
	}

	if _, err := os.Stat(filepath.Join(destDir, "d", "link", "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected dest/d/link/file.txt to not exist (symlink not followed), err=%v", err)
	}
}
