# wt

A fast CLI tool for managing git worktrees with fuzzy selection.

## Features

- **Quick worktree creation** with optional branch name preprocessing
- **Interactive fuzzy finder** for switching between worktrees
- **Multi-select deletion** of worktrees
- **File copying** with gitignore-style patterns
- **Post-creation hooks** for automated setup
- **Shell integration** for seamless directory switching
- **Tmux support** for opening worktrees in new panes

## Installation

### From releases

Download the latest release from [GitHub Releases](https://github.com/default-anton/wt/releases).

### From source

```bash
go install github.com/default-anton/wt/cmd/wt@latest
```

## Shell Integration

For `wt switch` and `wt add` to automatically change your directory, add shell integration:

### Bash

```bash
# Add to ~/.bashrc
eval "$(wt shell-init bash)"
```

### Zsh

```bash
# Add to ~/.zshrc
eval "$(wt shell-init zsh)"
```

### Fish

```fish
# Add to ~/.config/fish/config.fish
wt shell-init fish | source
```

## Usage

### Create a worktree

```bash
# Create worktree with branch name
wt add my-feature

# With tmux (opens in new pane)
wt add my-feature --tmux

# With custom base branch
wt add my-feature --base develop
```

### Switch between worktrees

```bash
# Interactive fuzzy finder
wt switch

# With tmux
wt switch --tmux
```

### Remove worktrees

```bash
# Interactive multi-select
wt remove

# Direct removal
wt remove ./worktrees/my-feature

# Force removal
wt remove -f ./worktrees/my-feature
```

### List worktrees

```bash
wt list
```

### Initialize config

```bash
wt init
```

## Configuration

Create a `.wt.toml` file in your repository root:

```toml
# Base branch for new worktrees (default: main)
base_branch = "main"

# Directory for worktrees (default: ./worktrees)
worktree_dir = "./worktrees"

# Preprocessing script (receives input, outputs branch name)
preprocess_script = ".wt/preprocess.sh"

# Files/directories to copy (gitignore-like patterns)
copy_patterns = [
  "node_modules",
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
