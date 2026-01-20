# Todo: hermetic integration tests (local git)

## Context
- Spec: no spec
- Task: "Implement Option A (Hermetic integration tests using local temp git repos; no network). Use iterative continuous mode. Fix bugs found along the way."
- Goal:
  - End-to-end CLI integration tests: temp local git repos; exec `wt`; assert filesystem + git effects.
  - Runnable: developer laptop + CI (GitHub Actions).
- Constraints/assumptions:
  - Avoid interactive TUI (`/dev/tty`) in integration suite.
  - No network / no GitHub API.

## Progress
- Added hermetic CLI integration suite using `github.com/rogpeppe/go-internal/testscript`.
- Covered: `wt init`, `wt add`, preprocess script, copy patterns, post hooks.
- Fixed behavior mismatches uncovered by integration coverage.
- Added CI workflow running `go test ./...`.

## Changes
- Added integration harness + scripts:
  - `integration/wt_test.go`
  - `integration/testdata/script/{init,add,preprocess,copy-hooks}.txtar`
- Bug fixes:
  - `internal/config/config.go`: default `WorktreeDir` now `.worktrees` (matches README + sample config).
  - `cmd/wt/main.go`:
    - `wt init` now appends worktree dir to `.gitignore`.
    - `wt add` now `MkdirAll` on worktree parent dir.
- CI:
  - `.github/workflows/ci.yaml`
- Deps:
  - `go.mod` / `go.sum`: add `github.com/rogpeppe/go-internal`.

## Open
- None.

## Next
1. (Optional) expand suite to cover `wt rm <path>` non-interactive cases.
2. (Optional) add `-short` gating if integration suite ever grows slow.
