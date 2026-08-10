# Releasing

Releases are automated with [GoReleaser](https://goreleaser.com) via
[`.github/workflows/release.yml`](.github/workflows/release.yml), triggered by pushing a `v*` tag.

## What a release produces

- Cross-compiled binaries — `darwin`/`linux`/`windows` × `amd64`/`arm64` — plus `checksums.txt`, attached to a GitHub Release.
- A Homebrew **cask** published to [`srikalachallagundla32-cloud/homebrew-tap`](https://github.com/srikalachallagundla32-cloud/homebrew-tap), enabling:

  ```bash
  brew install srikalachallagundla32-cloud/tap/finops-guard
  ```

## One-time prerequisites

1. **Tap repo** — create a public repo `srikalachallagundla32-cloud/homebrew-tap` and initialize it with a README (so `main` exists).
2. **Secret** — add `HOMEBREW_TAP_GITHUB_TOKEN` (a Personal Access Token with `repo` scope) under
   **Settings → Secrets and variables → Actions**. The default `GITHUB_TOKEN` cannot push to a *different* repository, which is why a PAT is required.

## Cutting a release

```bash
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
```

The workflow then builds, publishes the GitHub Release, and updates the tap.

## Local dry run

```bash
goreleaser check                                                   # validate .goreleaser.yaml
HOMEBREW_TAP_GITHUB_TOKEN=dummy goreleaser release --snapshot --clean   # build everything, publish nothing
```

Artifacts land in `dist/` (git-ignored), including the generated cask at `dist/homebrew/Casks/finops-guard.rb`.

## Notes

- **Every version needs a new tag.** Re-running an existing tag fails with *"release already exists"* — bump the patch (e.g. `v1.0.1`) instead.
- `finops-guard --version` reports the tag version (injected via `-ldflags -X main.version=…`).
- If a release publishes binaries but the Homebrew step fails, it means the two prerequisites above aren't in place yet — the GitHub Release is still valid; only the cask is missing.
