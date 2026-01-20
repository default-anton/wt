## Repo map

- Go CLI entrypoint: `cmd/wt/main.go` (Cobra)
  - commands: `add`, `cd`, `rm`, `ls`, `init`, `shell-init`
- Git plumbing (shell-out): `internal/git/worktree.go`
  - `GetRepoRoot`, `ListWorktrees`, `CreateWorktree`, `RemoveWorktree`, `BranchExists`
- Integration tests: `integration/` (testscript)
- Config: `internal/config/config.go`
  - config file: `.wt.toml`
  - note: `DefaultConfig().WorktreeDir` = `./worktrees`; sample/docs mention `.worktrees`
- Branch preprocessing: `internal/preprocess/preprocess.go`
  - runs `preprocess_script` (path resolved vs repo root)
  - expects branch name on stdout; trims; empty = error
- Copy step: `internal/copy/*`
  - gitignore-like patterns (supports `**`, negation)
- Post hooks: `internal/hooks/hooks.go`
  - `sh -c <hook.run>` in worktree dir
  - optional guard: `if_exists`
- TUI: `internal/tui/*` (Bubble Tea)
  - opens `/dev/tty` directly; interactive commands not CI-friendly unless PTY emulation

## Dev loop

- run: `go run ./cmd/wt --help`
- after changes: run `make fmt` (goimports -w), then `make check` (runs: tidy, goimports, vet, unit tests, integration tests, vulncheck, build)

## CI/releasing

- release: tag-driven; `.github/workflows/release.yaml` → GoReleaser (`.goreleaser.yaml`)
- docs: `docs/releasing.md`
- no push/PR CI workflow today (only tag release)

