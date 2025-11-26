# wt

A fast CLI tool for managing git worktrees with fuzzy selection.

## Features

- **Quick worktree creation** with optional branch name preprocessing
- **Interactive fuzzy finder** for navigating between worktrees
- **Multi-select deletion** of worktrees
- **File copying** with gitignore-style patterns
- **Post-creation hooks** for automated setup
- **Shell integration** for seamless directory switching
- **Tmux support** for opening worktrees in new panes

## Installation

### Homebrew

```bash
brew install default-anton/tap/wt
```

### From releases

Download the latest release from [GitHub Releases](https://github.com/default-anton/wt/releases).

### From source

```bash
go install github.com/default-anton/wt/cmd/wt@latest
```

## Shell Setup

For `wt cd` and `wt add` to automatically change your directory, add shell integration.

> **Note:** If installed via Homebrew, shell completions are already set up. You only need the `shell-init` line below.

### Bash

```bash
# Add to ~/.bashrc
eval "$(wt shell-init bash)"
eval "$(wt completion bash)"  # Skip if installed via Homebrew
```

### Zsh

```bash
# Add to ~/.zshrc
eval "$(wt shell-init zsh)"
eval "$(wt completion zsh)"  # Skip if installed via Homebrew
```

### Fish

```fish
# Add to ~/.config/fish/config.fish
wt shell-init fish | source
wt completion fish | source  # Skip if installed via Homebrew
```

## Usage

### Create a worktree

```bash
# Create worktree with branch name
wt add my-feature

# With tmux (opens in new pane)
wt add my-feature -t  # or --tmux

# With custom base branch
wt add my-feature --base develop
```

### Go to a worktree

```bash
# Interactive fuzzy finder
wt cd

# With tmux
wt cd -t  # or --tmux
```

### Remove worktrees

```bash
# Interactive multi-select
wt rm

# Direct removal
wt rm .worktrees/my-feature

# Force removal
wt rm -f .worktrees/my-feature
```

### List worktrees

```bash
wt ls
```

### Initialize config

```bash
wt init
```

## Configuration

Run `wt init` to create a `.wt.toml` configuration file in your repository root. This command also adds the worktree directory to `.gitignore`.

Example configuration:

```toml
# Base branch for new worktrees (default: main)
base_branch = "main"

# Directory for worktrees (default: .worktrees)
worktree_dir = ".worktrees"

# Preprocessing script (receives input, outputs branch name)
preprocess_script = ".wt/preprocess.sh"

# Files/directories to copy (gitignore-like patterns)
# Supports ** for recursive matching (e.g., **/node_modules for monorepos)
copy_patterns = [
  "**/node_modules",
  ".env*",
  "vendor",
  "!.env.example",
]

# Post-creation hooks
[[post_hooks]]
name = "Install dependencies"
run = "npm install"

[[post_hooks]]
name = "Setup database"
run = "bin/rails db:prepare"
if_exists = "bin/rails"
```

### Preprocessing Script

You can define a script that transforms the input into a branch name. This is useful for extracting branch names from issue tracker URLs:

```bash
#!/bin/bash
# .wt/preprocess.sh

# Example: Extract JIRA ticket from URL
# Input: https://jira.example.com/browse/PROJ-123
# Output: PROJ-123-feature-name

INPUT="$1"

# Extract ticket ID from URL or use as-is
if [[ "$INPUT" =~ ([A-Z]+-[0-9]+) ]]; then
  TICKET="${BASH_REMATCH[1]}"
  echo "$TICKET"
else
  echo "$INPUT"
fi
```

Make sure the script is executable: `chmod +x .wt/preprocess.sh`

## License

MIT License - see [LICENSE](LICENSE) for details.
