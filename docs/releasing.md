# Releasing

Tag-driven; GitHub Actions + GoReleaser.

## Trigger
- Push git tag matching `v*`.
- Workflow: `.github/workflows/release.yaml` → `goreleaser release --clean`.

## What ships
- Config: `.goreleaser.yaml`
- Builds: `./cmd/wt` (linux/darwin; amd64/arm64)
- Publishes: GitHub Release (`default-anton/wt`)
- Homebrew: updates `default-anton/homebrew-tap`
- Version injected: `-X main.version={{.Version}}`

## Cut release
```bash
git checkout main
git pull --ff-only
go test ./...

git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

## Preflight (local)
```bash
goreleaser release --snapshot --clean
```

## Gotchas
- Changelog excludes commits starting `docs:`, `test:`, `chore:`.
