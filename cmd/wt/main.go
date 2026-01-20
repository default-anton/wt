package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/default-anton/wt/internal/config"
	"github.com/default-anton/wt/internal/copy"
	"github.com/default-anton/wt/internal/git"
	"github.com/default-anton/wt/internal/hooks"
	"github.com/default-anton/wt/internal/preprocess"
	"github.com/default-anton/wt/internal/styles"
	"github.com/default-anton/wt/internal/tui"
)

var (
	version = "dev"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "wt",
	Short:   "Git worktree manager",
	Long:    `A fast CLI tool for managing git worktrees with fuzzy selection.`,
	Version: version,
}

var addCmd = &cobra.Command{
	Use:   "add <input>",
	Short: "Create a new worktree",
	Long: `Create a new git worktree.

If a preprocessing script is configured, the input is passed to it
to generate the branch name. Otherwise, input is used as the branch name.`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

var (
	addBase      string
	addTmux      bool
	addPrintPath bool
)

func init() {
	addCmd.Flags().StringVar(&addBase, "base", "", "Base branch for new branches (overrides config)")
	addCmd.Flags().BoolVarP(&addTmux, "tmux", "t", false, "Open in new tmux pane")
	addCmd.Flags().BoolVar(&addPrintPath, "print-path", false, "Print worktree path (for shell integration)")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(cdCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(shellInitCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	input := args[0]

	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return err
	}

	cfg, err := config.LoadFromDir(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	branch, err := preprocess.Run(cfg.PreprocessScript, input, repoRoot)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Branch name: %s\n", branch)

	baseBranch := cfg.BaseBranch
	if addBase != "" {
		baseBranch = addBase
	}

	worktreeDir, err := git.GetWorktreeDir(cfg.WorktreeDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	dirName := git.SanitizeBranchName(branch)
	worktreePath := filepath.Join(worktreeDir, dirName)

	local, remote := git.BranchExists(branch)
	if local || remote {
		fmt.Fprintf(os.Stderr, "Using existing branch: %s\n", branch)
	} else {
		fmt.Fprintf(os.Stderr, "Creating new branch from %s: %s\n", baseBranch, branch)
	}

	if err := git.CreateWorktree(branch, worktreePath, baseBranch); err != nil {
		return err
	}

	if len(cfg.CopyPatterns) > 0 {
		fmt.Fprintln(os.Stderr, "Copying files...")
		if err := copy.CopyFiles(cfg.CopyPatterns, repoRoot, worktreePath); err != nil {
			return fmt.Errorf("failed to copy files: %w", err)
		}
	}

	if len(cfg.PostHooks) > 0 {
		fmt.Fprintln(os.Stderr, "Running post-creation hooks...")
		if err := hooks.Run(cfg.PostHooks, worktreePath); err != nil {
			return err
		}
	}

	if addTmux {
		return openTmuxPane(worktreePath)
	}

	fmt.Fprintf(os.Stderr, "Worktree created at: %s\n", worktreePath)
	if addPrintPath {
		fmt.Println(worktreePath)
	} else {
		fmt.Printf("cd %s\n", worktreePath)
	}

	return nil
}

var cdCmd = &cobra.Command{
	Use:   "cd",
	Short: "Go to a worktree",
	Long:  `Interactive fuzzy finder to go to a worktree.`,
	RunE:  runCd,
}

var (
	cdTmux      bool
	cdPrintPath bool
)

func init() {
	cdCmd.Flags().BoolVarP(&cdTmux, "tmux", "t", false, "Open in new tmux pane")
	cdCmd.Flags().BoolVar(&cdPrintPath, "print-path", false, "Print worktree path (for shell integration)")
}

func runCd(cmd *cobra.Command, args []string) error {
	worktrees, err := git.ListWorktrees()
	if err != nil {
		return err
	}

	// Filter out main worktree
	var items []tui.Item
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		label := wt.Branch
		if label == "" {
			label = filepath.Base(wt.Path)
		}
		items = append(items, tui.Item{
			Label: label,
			Value: wt.Path,
		})
	}

	if len(items) == 0 {
		fmt.Println("No worktrees to switch to.")
		return nil
	}

	selected, err := tui.Select(items)
	if err != nil {
		return err
	}

	if selected == "" {
		return nil // User cancelled
	}

	if cdTmux {
		return openTmuxPane(selected)
	}

	if cdPrintPath {
		fmt.Println(selected)
	} else {
		fmt.Printf("cd %s\n", selected)
	}

	return nil
}

var removeCmd = &cobra.Command{
	Use:     "rm [path]",
	Aliases: []string{"remove"},
	Short:   "Remove worktree(s)",
	Long:    `Remove one or more worktrees. If no path is given, shows interactive selection.`,
	RunE:    runRemove,
}

var removeForce bool

func init() {
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Force removal even if worktree is dirty")
}

func runRemove(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return removeWorktreeWithConfirm(args[0], removeForce)
	}

	// Interactive selection
	worktrees, err := git.ListWorktrees()
	if err != nil {
		return err
	}

	var items []tui.Item
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		label := fmt.Sprintf("%s (%s)", wt.Branch, wt.Path)
		if wt.Branch == "" {
			label = wt.Path
		}
		items = append(items, tui.Item{
			Label: label,
			Value: wt.Path,
		})
	}

	if len(items) == 0 {
		fmt.Println("No worktrees to remove.")
		return nil
	}

	selected, err := tui.MultiSelect(items)
	if err != nil {
		return err
	}

	if len(selected) == 0 {
		fmt.Println("No worktrees selected.")
		return nil
	}

	for _, path := range selected {
		fmt.Printf("Removing worktree: %s\n", path)
		if err := removeWorktreeWithConfirm(path, removeForce); err != nil {
			return err
		}
	}

	return nil
}

// removeWorktreeWithConfirm attempts to remove a worktree and prompts for
// confirmation if it contains modified or untracked files.
func removeWorktreeWithConfirm(path string, force bool) error {
	err := git.RemoveWorktree(path, force)
	if err == nil {
		return nil
	}

	if !errors.Is(err, git.ErrDirtyWorktree) {
		return err
	}

	fmt.Printf("Worktree '%s' contains modified or untracked files.\n", path)
	confirmed, confirmErr := tui.Confirm("Force remove anyway?")
	if confirmErr != nil {
		return confirmErr
	}

	if !confirmed {
		fmt.Println("Skipped.")
		return nil
	}

	return git.RemoveWorktree(path, true)
}

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all worktrees",
	RunE:  runLs,
}

func runLs(cmd *cobra.Command, args []string) error {
	worktrees, err := git.ListWorktrees()
	if err != nil {
		return err
	}

	homeDir, _ := os.UserHomeDir()

	// Group worktrees by parent directory
	groups := make(map[string][]git.Worktree)
	var mainWorktree *git.Worktree

	for i := range worktrees {
		wt := &worktrees[i]
		if wt.IsMain {
			mainWorktree = wt
		} else {
			parentDir := filepath.Dir(wt.Path)
			groups[parentDir] = append(groups[parentDir], *wt)
		}
	}

	// Print main worktree first
	if mainWorktree != nil {
		path := shortenHome(mainWorktree.Path, homeDir)
		branch := styles.BranchStyle.Render(mainWorktree.Branch)
		badge := styles.CursorStyle.Render("(main)")
		fmt.Printf("%s %s %s\n", path, branch, badge)
	}

	// Print grouped worktrees
	for parentDir, wts := range groups {
		fmt.Println()
		fmt.Println(styles.DimStyle.Render(shortenHome(parentDir, homeDir) + "/"))
		for _, wt := range wts {
			dirName := filepath.Base(wt.Path)
			if dirName == wt.Branch {
				fmt.Printf("  %s\n", styles.BranchStyle.Render(dirName))
			} else {
				branch := styles.BranchStyle.Render(wt.Branch)
				fmt.Printf("  %s %s\n", dirName, branch)
			}
		}
	}

	return nil
}

func shortenHome(path, homeDir string) string {
	if homeDir != "" && strings.HasPrefix(path, homeDir) {
		return "~" + path[len(homeDir):]
	}
	return path
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a sample .wt.toml config file",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	configPath := config.ConfigFileName

	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("%s already exists", configPath)
	}

	if err := os.WriteFile(configPath, []byte(config.SampleConfig()), 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	cfg := config.DefaultConfig()
	if err := ensureGitignoreHasWorktreeDir(cfg.WorktreeDir); err != nil {
		return err
	}

	fmt.Printf("Created %s\n", configPath)
	return nil
}

func ensureGitignoreHasWorktreeDir(worktreeDir string) error {
	entry := strings.TrimSpace(worktreeDir)
	entry = strings.TrimPrefix(entry, "./")
	if entry == "" {
		return nil
	}
	if !strings.HasSuffix(entry, "/") {
		entry += "/"
	}

	const gitignorePath = ".gitignore"

	existing, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", gitignorePath, err)
	}
	if err == nil {
		for _, line := range strings.Split(string(existing), "\n") {
			line = strings.TrimSpace(line)
			if line == entry || line == strings.TrimSuffix(entry, "/") {
				return nil
			}
		}
	}

	f, err := os.OpenFile(gitignorePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", gitignorePath, err)
	}
	defer f.Close()

	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write %s: %w", gitignorePath, err)
		}
	}
	if _, err := f.WriteString(entry + "\n"); err != nil {
		return fmt.Errorf("failed to write %s: %w", gitignorePath, err)
	}
	return nil
}

var shellInitCmd = &cobra.Command{
	Use:   "shell-init <shell>",
	Short: "Print shell integration code",
	Long:  `Print shell integration code for the specified shell (bash, zsh, fish).`,
	Args:  cobra.ExactArgs(1),
	RunE:  runShellInit,
}

func runShellInit(cmd *cobra.Command, args []string) error {
	shell := args[0]

	switch shell {
	case "bash", "zsh":
		fmt.Print(bashZshIntegration)
	case "fish":
		fmt.Print(fishIntegration)
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", shell)
	}

	return nil
}

func openTmuxPane(path string) error {
	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("not inside a tmux session")
	}

	cmd := exec.Command("tmux", "new-window", "-c", path)
	return cmd.Run()
}

const bashZshIntegration = `# wt shell integration
# Add this to your .bashrc or .zshrc:
#   eval "$(wt shell-init bash)"  # for bash
#   eval "$(wt shell-init zsh)"   # for zsh

wt() {
  if [[ "$1" == "cd" ]] && [[ ! " $* " =~ " --tmux " ]] && [[ ! " $* " =~ " -t " ]]; then
    local result
    result=$(command wt cd --print-path "${@:2}")
    if [[ -n "$result" && -d "$result" ]]; then
      cd "$result"
    fi
  elif [[ "$1" == "add" ]] && [[ ! " $* " =~ " --tmux " ]] && [[ ! " $* " =~ " -t " ]]; then
    local result
    result=$(command wt add "${@:2}" --print-path)
    if [[ -n "$result" && -d "$result" ]]; then
      cd "$result"
    fi
  else
    command wt "$@"
  fi
}
`

const fishIntegration = `# wt shell integration
# Add this to your config.fish:
#   wt shell-init fish | source

function wt
  if test "$argv[1]" = "cd"; and not contains -- --tmux $argv; and not contains -- -t $argv
    set -l result (command wt cd --print-path $argv[2..])
    if test -n "$result"; and test -d "$result"
      cd $result
    end
  else if test "$argv[1]" = "add"; and not contains -- --tmux $argv; and not contains -- -t $argv
    set -l result (command wt add $argv[2..] --print-path)
    if test -n "$result"; and test -d "$result"
      cd $result
    end
  else
    command wt $argv
  end
end
`
