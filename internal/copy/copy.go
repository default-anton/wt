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

	var includePatterns, excludePatterns []string
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			excludePatterns = append(excludePatterns, strings.TrimPrefix(p, "!"))
			continue
		}
		includePatterns = append(includePatterns, p)
	}

	matches := make(map[string]bool)
	for _, pattern := range includePatterns {
		found, err := findMatches(srcDir, pattern)
		if err != nil {
			return fmt.Errorf("error matching pattern %q: %w", pattern, err)
		}
		for _, f := range found {
			if f == "" {
				continue
			}
			matches[f] = true
		}
	}

	for _, pattern := range excludePatterns {
		excluded, err := findMatches(srcDir, pattern)
		if err != nil {
			return fmt.Errorf("error matching exclude pattern %q: %w", pattern, err)
		}
		for _, f := range excluded {
			delete(matches, f)
		}
	}

	paths := filterDescendants(matches, srcDir)
	sort.Strings(paths)

	for _, relPath := range paths {
		srcPath := filepath.Join(srcDir, relPath)
		destPath := filepath.Join(destDir, relPath)

		copied, err := copyPath(srcPath, destPath)
		if err != nil {
			return fmt.Errorf("failed to copy %q: %w", relPath, err)
		}
		if copied {
			fmt.Fprintf(os.Stderr, "Copied: %s\n", relPath)
		}
	}

	return nil
}

func normalizeRelPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimSuffix(p, "/")
	p = strings.TrimSuffix(p, string(filepath.Separator))
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}

// filterDescendants removes paths that are descendants of other paths in the set.
// This prevents redundant copying when a parent directory is already being copied.
// Only filters directory descendants; files and symlinks are always kept.
func filterDescendants(matches map[string]bool, baseDir string) []string {
	paths := make([]string, 0, len(matches))
	for p := range matches {
		paths = append(paths, normalizeRelPath(p))
	}

	sort.Slice(paths, func(i, j int) bool {
		if len(paths[i]) != len(paths[j]) {
			return len(paths[i]) < len(paths[j])
		}
		return paths[i] < paths[j]
	})

	var kept []string
	keptDirs := make(map[string]bool)

	for _, p := range paths {
		if p == "" {
			continue
		}

		isDescendant := false
		for dir := range keptDirs {
			if strings.HasPrefix(p, dir+string(filepath.Separator)) {
				isDescendant = true
				break
			}
		}

		if isDescendant {
			continue
		}

		kept = append(kept, p)

		fullPath := filepath.Join(baseDir, p)
		info, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if info.IsDir() {
			keptDirs[p] = true
		}
	}

	return kept
}

func findMatches(baseDir, pattern string) ([]string, error) {
	var matches []string

	// Literal path (no glob chars)
	if !strings.ContainsAny(pattern, "*?[]{}") {
		rel := normalizeRelPath(pattern)
		if rel == "" {
			return nil, nil
		}
		path := filepath.Join(baseDir, rel)
		if _, err := os.Lstat(path); err == nil {
			matches = append(matches, rel)
		}
		return matches, nil
	}

	err := doublestar.GlobWalk(os.DirFS(baseDir), pattern, func(path string, d fs.DirEntry) error {
		rel := normalizeRelPath(path)
		if rel != "" {
			matches = append(matches, rel)
		}
		return nil
	})

	return matches, err
}

// copyPath copies src to dest. Returns true if a copy was performed, false if skipped.
func copyPath(src, dest string) (bool, error) {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return false, err
	}

	destInfo, destErr := os.Lstat(dest)
	destExists := destErr == nil

	srcIsSymlink := srcInfo.Mode()&os.ModeSymlink != 0
	srcIsDir := srcInfo.IsDir() && !srcIsSymlink
	destIsSymlink := destExists && (destInfo.Mode()&os.ModeSymlink != 0)
	destIsDir := destExists && destInfo.IsDir() && !destIsSymlink

	if srcIsDir {
		if destExists && !destIsDir {
			return false, fmt.Errorf("destination exists and is not a directory")
		}
	} else {
		if destExists && destIsDir {
			return false, fmt.Errorf("destination exists and is a directory")
		}
	}

	// For files/symlinks: skip if destination already exists (may have been copied as part of a parent directory)
	if destExists && !srcIsDir {
		return false, nil
	}

	parentDir := filepath.Dir(dest)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		// MkdirAll can fail if a path component is a symlink (common in node_modules).
		// Check if the parent is still accessible as a directory (symlink to valid dir).
		if parentInfo, statErr := os.Stat(parentDir); statErr == nil && parentInfo.IsDir() {
			// proceed
		} else {
			return false, nil
		}
	}

	if srcIsDir {
		// If destination directory already exists (e.g., from git checkout with tracked files),
		// merge contents instead of skipping.
		if destExists && destIsDir {
			return true, mergeDirContents(src, dest)
		}
		return true, copyDir(src, dest)
	}

	return true, copyFile(src, dest, srcInfo.Mode())
}

func copyDir(src, dest string) error {
	switch runtime.GOOS {
	case "darwin":
		// Try copy-on-write on macOS (APFS)
		if err := exec.Command("cp", "-c", "-R", "-P", "-p", src, dest).Run(); err == nil {
			return nil
		}
		return runWithOutput("cp", "-R", "-P", "-p", src, dest)
	case "linux":
		// Try copy-on-write on Btrfs/XFS
		if err := exec.Command("cp", "-R", "-P", "-p", "--reflink=auto", src, dest).Run(); err == nil {
			return nil
		}
		return runWithOutput("cp", "-R", "-P", "-p", src, dest)
	default:
		return runWithOutput("cp", "-R", "-P", "-p", src, dest)
	}
}

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
	srcContents := src + string(filepath.Separator) + "."

	cmd := exec.Command("cp", "-R", "-P", "-p", "-n", srcContents, dest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(output)
		// macOS: cp -n returns exit code 1 when it skips files.
		if len(outStr) == 0 {
			return nil
		}
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
		if err := exec.Command("cp", "-c", "-P", "-p", src, dest).Run(); err == nil {
			return nil
		}
		return runWithOutput("cp", "-P", "-p", src, dest)
	case "linux":
		// Try copy-on-write on Btrfs/XFS
		if err := exec.Command("cp", "-P", "-p", "--reflink=auto", src, dest).Run(); err == nil {
			return nil
		}
		return runWithOutput("cp", "-P", "-p", src, dest)
	default:
		return runWithOutput("cp", "-P", "-p", src, dest)
	}
}
