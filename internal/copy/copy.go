package copy

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// CopyFiles copies files matching the given patterns from srcDir to destDir.
func CopyFiles(patterns []string, srcDir, destDir string) error {
	if len(patterns) == 0 {
		return nil
	}

	// Separate include and exclude patterns
	var includePatterns, excludePatterns []string
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			excludePatterns = append(excludePatterns, strings.TrimPrefix(p, "!"))
		} else {
			includePatterns = append(includePatterns, p)
		}
	}

	// Find all files/dirs matching include patterns
	matches := make(map[string]bool)
	for _, pattern := range includePatterns {
		found, err := findMatches(srcDir, pattern)
		if err != nil {
			return fmt.Errorf("error matching pattern %q: %w", pattern, err)
		}
		for _, f := range found {
			matches[f] = true
		}
	}

	// Remove excluded files
	for _, pattern := range excludePatterns {
		excluded, err := findMatches(srcDir, pattern)
		if err != nil {
			return fmt.Errorf("error matching exclude pattern %q: %w", pattern, err)
		}
		for _, f := range excluded {
			delete(matches, f)
		}
	}

	// Copy matched files
	for relPath := range matches {
		srcPath := filepath.Join(srcDir, relPath)
		destPath := filepath.Join(destDir, relPath)

		if err := copyPath(srcPath, destPath); err != nil {
			return fmt.Errorf("failed to copy %q: %w", relPath, err)
		}
		fmt.Printf("Copied: %s\n", relPath)
	}

	return nil
}

func findMatches(baseDir, pattern string) ([]string, error) {
	var matches []string

	// Check if pattern is a literal path (no glob chars)
	if !strings.ContainsAny(pattern, "*?[]{}") {
		path := filepath.Join(baseDir, pattern)
		if _, err := os.Stat(path); err == nil {
			matches = append(matches, pattern)
		}
		return matches, nil
	}

	// Use doublestar for glob matching
	err := doublestar.GlobWalk(os.DirFS(baseDir), pattern, func(path string, d fs.DirEntry) error {
		matches = append(matches, path)
		return nil
	})

	return matches, err
}

func copyPath(src, dest string) error {
	// Skip if destination already exists (may have been copied as part of a parent directory)
	if _, err := os.Lstat(dest); err == nil {
		return nil
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(dest)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		// MkdirAll can fail if a path component is a symlink (common in node_modules).
		// Check if the parent is still accessible as a directory (symlink to valid dir).
		if parentInfo, statErr := os.Stat(parentDir); statErr == nil && parentInfo.IsDir() {
			// Parent is accessible as a directory via symlink, proceed
		} else {
			// Parent is inaccessible (broken symlink or other issue), skip this copy.
			// This happens when a symlink points to a not-yet-copied or external location.
			return nil
		}
	}

	if info.IsDir() {
		return copyDir(src, dest)
	}
	return copyFile(src, dest, info.Mode())
}

func copyDir(src, dest string) error {
	switch runtime.GOOS {
	case "darwin":
		// Try copy-on-write on macOS (APFS)
		if err := exec.Command("cp", "-cRp", src, dest).Run(); err == nil {
			return nil
		}
		// Fall back to regular copy if -c fails
		return exec.Command("cp", "-Rp", src, dest).Run()
	case "linux":
		// Try copy-on-write on Btrfs/XFS
		if err := exec.Command("cp", "-Rp", "--reflink=auto", src, dest).Run(); err == nil {
			return nil
		}
		// Fall back to regular copy if --reflink fails
		return exec.Command("cp", "-Rp", src, dest).Run()
	default:
		// Other OSes: just use cp
		return exec.Command("cp", "-Rp", src, dest).Run()
	}
}

func copyFile(src, dest string, mode fs.FileMode) error {
	switch runtime.GOOS {
	case "darwin":
		// Try copy-on-write on macOS (APFS)
		if err := exec.Command("cp", "-cp", src, dest).Run(); err == nil {
			return nil
		}
		// Fall back to regular copy if -c fails
		return exec.Command("cp", "-p", src, dest).Run()
	case "linux":
		// Try copy-on-write on Btrfs/XFS
		if err := exec.Command("cp", "-p", "--reflink=auto", src, dest).Run(); err == nil {
			return nil
		}
		// Fall back to regular copy if --reflink fails
		return exec.Command("cp", "-p", src, dest).Run()
	default:
		// Other OSes: just use cp
		return exec.Command("cp", "-p", src, dest).Run()
	}
}
