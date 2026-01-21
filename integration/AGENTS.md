## Testing gotchas (integration)

- Require `git` binary; run in temp repo
- Interactive coverage via pty (bash): `wt cd` + `wt cd --tmux`
  - uses fake `tmux` shim + `TMUX=1` to assert args
  - requires `/dev/tty` (pty-backed)
- Non-interactive coverage via testscript: `wt add --print-path`, `wt rm <path> -f`, `wt ls`, `wt init`
- Hermetic integration tests: temp git repo + optional local bare `origin` remote; no network required
