package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const ConfigFileName = ".wt.toml"

type Hook struct {
	Name     string `toml:"name"`
	Run      string `toml:"run"`
	IfExists string `toml:"if_exists,omitempty"`
}

type Config struct {
	BaseBranch        string   `toml:"base_branch"`
	WorktreeDir       string   `toml:"worktree_dir"`
	PreprocessScript  string   `toml:"preprocess_script"`
	CopyPatterns      []string `toml:"copy_patterns"`
	PostHooks         []Hook   `toml:"post_hooks"`
}

func DefaultConfig() *Config {
	return &Config{
		BaseBranch:   "main",
		WorktreeDir:  "./worktrees",
		CopyPatterns: []string{},
		PostHooks:    []Hook{},
	}
}

// Load finds and parses .wt.toml from the current directory or parent directories.
// Returns default config if no config file is found.
func Load() (*Config, error) {
	configPath, err := findConfig()
	if err != nil {
		return DefaultConfig(), nil
	}
	return loadFromPath(configPath)
}

// LoadFromDir loads config from a specific directory.
func LoadFromDir(dir string) (*Config, error) {
	configPath := filepath.Join(dir, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	return loadFromPath(configPath)
}

func loadFromPath(path string) (*Config, error) {
	cfg := DefaultConfig()
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func findConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(dir, ConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// SampleConfig returns a sample configuration file content.
func SampleConfig() string {
	return `# wt configuration file

# Base branch for new worktrees (default: main)
base_branch = "main"

# Directory for worktrees (default: .worktrees)
worktree_dir = ".worktrees"

# Preprocessing script (receives input, outputs branch name)
# Script can be any executable - bash, python, etc.
# preprocess_script = ".wt/preprocess.sh"

# Files/directories to copy (gitignore-like patterns)
# Supports ** for recursive matching (e.g., **/node_modules for monorepos)
# copy_patterns = [
#   "**/node_modules",
#   ".env*",
#   "vendor",
#   "!.env.example",
# ]

# Post-creation hooks (run in order after worktree is created)
# [[post_hooks]]
# name = "Install dependencies"
# run = "npm install"
#
# [[post_hooks]]
# name = "Setup database"
# run = "bin/rails db:prepare"
# if_exists = "bin/rails"
`
}
