package copy

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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

	// Filter out paths that are descendants of other matched paths.
	// For example, if both "node_modules" and "node_modules/foo/node_modules" match,
	// we only need to copy "node_modules" since it includes all nested directories.
	paths := filterDescendants(matches, srcDir)

	// Copy matched files
	for _, relPath := range paths {
		srcPath := filepath.Join(srcDir, relPath)
		destPath := filepath.Join(destDir, relPath)

		copied, err := copyPath(srcPath, destPath)
		if err != nil {
			return fmt.Errorf("failed to copy %q: %w", relPath, err)
		}
		if copied {
			fmt.Printf("Copied: %s\n", relPath)
		}
	}

	return nil
}

// filterDescendants removes paths that are descendants of other paths in the set.
// This prevents redundant copying when a parent directory is already being copied.
// Only filters directory descendants; files are always kept.
func filterDescendants(matches map[string]bool, baseDir string) []string {
	paths := make([]string, 0, len(matches))
	for p := range matches {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i]) < len(paths[j])
	})

	var kept []string
	keptDirs := make(map[string]bool)

	for _, p := range paths {
		isDescendant := false
		for dir := range keptDirs {
			if strings.HasPrefix(p, dir+string(filepath.Separator)) {
				isDescendant = true
				break
			}
		}

		if !isDescendant {
			kept = append(kept, p)
			fullPath := filepath.Join(baseDir, p)
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
				keptDirs[p] = true
			}
		}
	}

	return kept
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

// copyPath copies src to dest. Returns true if a copy was performed, false if skipped.
func copyPath(src, dest string) (bool, error) {
	info, err := os.Stat(src)
	if err != nil {
		return false, err
	}

	destInfo, destErr := os.Lstat(dest)
	destExists := destErr == nil

	// For files: skip if destination already exists (may have been copied as part of a parent directory)
	if destExists && !info.IsDir() {
		return false, nil
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
			return false, nil
		}
	}

	if info.IsDir() {
		// If destination directory already exists (e.g., from git checkout with tracked files),
		// merge contents instead of skipping. This ensures untracked files get copied.
		if destExists && destInfo.IsDir() {
			return true, mergeDirContents(src, dest)
		}
		return true, copyDir(src, dest)
	}
	return true, copyFile(src, dest, info.Mode())
}

func copyDir(src, dest string) error {
	switch runtime.GOOS {
	case "darwin":
		// Try copy-on-write on macOS (APFS)
		if err := exec.Command("cp", "-cRp", src, dest).Run(); err == nil {
			return nil
		}
		// Fall back to regular copy if -c fails
		return runWithOutput("cp", "-Rp", src, dest)
	case "linux":
		// Try copy-on-write on Btrfs/XFS
		if err := exec.Command("cp", "-Rp", "--reflink=auto", src, dest).Run(); err == nil {
			return nil
		}
		// Fall back to regular copy if --reflink fails
		return runWithOutput("cp", "-Rp", src, dest)
	default:
		// Other OSes: just use cp
		return runWithOutput("cp", "-Rp", src, dest)
	}
}

// runWithOutput runs a command and returns an error that includes stderr output
func runWithOutput(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// mergeDirContents copies contents of src directory into existing dest directory,
// skipping files that already exist in dest.
func mergeDirContents(src, dest string) error {
	// Use "src/." to copy contents of src into dest (POSIX standard)
	srcContents := src + string(filepath.Separator) + "."

	// Use cp -n (no-clobber) to skip existing files.
	// On macOS, cp -n returns exit code 1 when it skips files, even though
	// the operation succeeded. We treat exit code 1 with empty stderr as success.
	cmd := exec.Command("cp", "-Rpn", srcContents, dest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(output)
		// Exit code 1 with empty output means files were skipped (expected on macOS)
		if len(outStr) == 0 {
			return nil
		}
		// Check for actual error messages vs benign "not overwritten" messages
		if strings.Contains(outStr, "Permission denied") ||
			strings.Contains(outStr, "No such file") ||
			strings.Contains(outStr, "No space") ||
			strings.Contains(outStr, "Read-only") {
			return fmt.Errorf("%w: %s", err, outStr)
		}
		return nil
	}
	return nil
}

func copyFile(src, dest string, mode fs.FileMode) error {
	switch runtime.GOOS {
	case "darwin":
		// Try copy-on-write on macOS (APFS)
		if err := exec.Command("cp", "-cp", src, dest).Run(); err == nil {
			return nil
		}
		// Fall back to regular copy if -c fails
		return runWithOutput("cp", "-p", src, dest)
	case "linux":
		// Try copy-on-write on Btrfs/XFS
		if err := exec.Command("cp", "-p", "--reflink=auto", src, dest).Run(); err == nil {
			return nil
		}
		// Fall back to regular copy if --reflink fails
		return runWithOutput("cp", "-p", src, dest)
	default:
		// Other OSes: just use cp
		return runWithOutput("cp", "-p", src, dest)
	}
}
