## Testing gotchas (integration)

- Require `git` binary; run in temp repo
- Avoid interactive paths in headless CI:
  - skip `wt cd` and interactive `wt rm` (needs `/dev/tty`)
  - prefer non-interactive surfaces: `wt add --print-path`, `wt rm <path> -f`, `wt ls`, `wt init`
- Hermetic integration tests: create temp git repo + optional local bare `origin` remote; no network required
