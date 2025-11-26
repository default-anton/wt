package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Worktree struct {
	Path   string
	Branch string
	Commit string
	IsMain bool
}

// GetRepoRoot returns the root directory of the git repository.
func GetRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository")
	}
	return strings.TrimSpace(string(output)), nil
}

// ListWorktrees returns all worktrees in the repository.
func ListWorktrees() ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []Worktree
	var current Worktree
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.Commit = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			branch := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
		case line == "bare":
			current.IsMain = true
		}
	}

	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	// Mark the first worktree as main if not bare
	if len(worktrees) > 0 && !worktrees[0].IsMain {
		worktrees[0].IsMain = true
	}

	return worktrees, nil
}

// BranchExists checks if a branch exists locally or remotely.
func BranchExists(branch string) (local bool, remote bool) {
	// Check local
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if cmd.Run() == nil {
		local = true
	}

	// Check remote
	cmd = exec.Command("git", "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	if cmd.Run() == nil {
		remote = true
	}

	return local, remote
}

// CreateWorktree creates a new worktree.
// If the branch exists, it uses it. Otherwise, it creates a new branch from baseBranch.
func CreateWorktree(branch, path, baseBranch string) error {
	local, remote := BranchExists(branch)

	var cmd *exec.Cmd
	if local || remote {
		// Use existing branch
		cmd = exec.Command("git", "worktree", "add", path, branch)
	} else {
		// Create new branch from base
		cmd = exec.Command("git", "worktree", "add", "-b", branch, path, baseBranch)
	}

	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RemoveWorktree removes a worktree.
func RemoveWorktree(path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetWorktreeDir returns the directory where worktrees should be created.
func GetWorktreeDir(configDir string) (string, error) {
	repoRoot, err := GetRepoRoot()
	if err != nil {
		return "", err
	}

	// If configDir is absolute, use it; otherwise, make it relative to repo root
	if filepath.IsAbs(configDir) {
		return configDir, nil
	}
	return filepath.Join(repoRoot, configDir), nil
}

// SanitizeBranchName sanitizes a branch name for use as a directory name.
func SanitizeBranchName(branch string) string {
	// Replace / with -
	return strings.ReplaceAll(branch, "/", "-")
}
