package preprocess

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Run executes the preprocessing script with the given input and returns the branch name.
// The script receives the input as the first argument and should output the branch name to stdout.
func Run(scriptPath, input, repoRoot string) (string, error) {
	if scriptPath == "" {
		return input, nil
	}

	// Resolve script path relative to repo root
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(repoRoot, scriptPath)
	}

	// Check if script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return "", fmt.Errorf("preprocessing script not found: %s", scriptPath)
	}

	// Execute the script
	cmd := exec.Command(scriptPath, input)
	cmd.Dir = repoRoot
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("preprocessing script failed: %w", err)
	}

	branch := strings.TrimSpace(stdout.String())
	if branch == "" {
		return "", fmt.Errorf("preprocessing script returned empty branch name")
	}

	return branch, nil
}
