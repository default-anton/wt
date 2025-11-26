package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/default-anton/wt/internal/config"
)

// Run executes the post-creation hooks in the given working directory.
func Run(hooks []config.Hook, workDir string) error {
	for _, hook := range hooks {
		// Check if_exists condition
		if hook.IfExists != "" {
			checkPath := hook.IfExists
			if !filepath.IsAbs(checkPath) {
				checkPath = filepath.Join(workDir, checkPath)
			}
			if _, err := os.Stat(checkPath); os.IsNotExist(err) {
				fmt.Printf("Skipping hook %q: %s not found\n", hook.Name, hook.IfExists)
				continue
			}
		}

		fmt.Printf("Running hook: %s\n", hook.Name)

		cmd := exec.Command("sh", "-c", hook.Run)
		cmd.Dir = workDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("hook %q failed: %w", hook.Name, err)
		}
	}
	return nil
}
