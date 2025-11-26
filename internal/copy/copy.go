package copy

import (
	"fmt"
	"io"
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
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	if info.IsDir() {
		return copyDir(src, dest)
	}
	return copyFile(src, dest, info.Mode())
}

func copyDir(src, dest string) error {
	// Use copy-on-write on macOS (APFS)
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("cp", "-cRp", src, dest)
		if err := cmd.Run(); err == nil {
			return nil
		}
		// Fall back to regular copy if -c fails
	}

	// Regular copy for Linux and fallback
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dest, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}
		return copyFile(path, destPath, info.Mode())
	})
}

func copyFile(src, dest string, mode fs.FileMode) error {
	// Try copy-on-write on macOS first
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("cp", "-cp", src, dest)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// Regular copy
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}
