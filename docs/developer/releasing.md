# Release Process

This document describes how to create releases for dsops.

## Overview

Releases are automated via **release-please**:

1. Push commits with conventional messages (`feat:`, `fix:`, etc.) to `main`
2. Release-please automatically creates/updates a **Release PR**
3. Merge the Release PR to trigger a release
4. GoReleaser builds and publishes all artifacts

## Prerequisites

- Push access to `systmms/dsops` repository
- `RELEASE_PLEASE_TOKEN` secret configured (for automated versioning)
- `HOMEBREW_TAP_GITHUB_TOKEN` secret configured (for Homebrew updates)

## Conventional Commits

Use these prefixes for your commit messages:

| Prefix | Version Bump | Example |
|--------|--------------|---------|
| `fix:` | Patch (0.0.X) | `fix: resolve config parsing error` |
| `feat:` | Minor (0.X.0) | `feat: add new provider support` |
| `feat!:` | Minor* (0.X.0) | `feat!: change config format` |
| `BREAKING CHANGE:` | Minor* (0.X.0) | Footer in commit body |

*While version < 1.0.0, breaking changes bump minor, not major.

## Creating a Release (Automated)

### Step 1: Write Code with Conventional Commits

```bash
git commit -m "feat: add Azure Key Vault provider"
git commit -m "fix: handle empty config files gracefully"
git push origin main
```

### Step 2: Review the Release PR

After pushing to `main`, release-please will:
- Create or update a PR titled "chore(main): release X.Y.Z"
- Update `CHANGELOG.md` with commit summaries
- Bump version based on commit types

Review the PR to verify:
- Version bump is correct
- Changelog entries are accurate

### Step 3: Merge the Release PR

When ready to release:
1. Approve and merge the Release PR
2. Release-please creates a version tag (e.g., `v0.2.0`)
3. The tag triggers the GoReleaser workflow

### Step 4: Verify Release Artifacts

After ~5-10 minutes, verify:

- **GitHub Releases**: New release with binaries and checksums
- **Docker**: `docker pull ghcr.io/systmms/dsops:v0.2.0`
- **Homebrew**: Formula updated in `systmms/homebrew-tap`

## Manual Release (Alternative)

If you need to create a release without release-please:

```bash
# Create annotated tag
git tag -a v1.0.0 -m "Release v1.0.0: Description"

# Push tag to trigger release workflow
git push origin v1.0.0
```

## Pre-releases (Beta/RC)

For pre-release versions:

```bash
git tag -a v1.0.0-beta.1 -m "Beta release v1.0.0-beta.1"
git push origin v1.0.0-beta.1
```

Pre-releases:
- Are published to GitHub Releases (marked as pre-release)
- Are published to Docker with version tag (no `:latest`)
- Do NOT update Homebrew tap (stable releases only)

## Local Testing

Test GoReleaser locally without publishing:

```bash
# Dry run (no publishing)
goreleaser release --snapshot --clean

# Check generated artifacts
ls dist/
```

## Troubleshooting

### Release PR not created

1. Verify commits use conventional format (`feat:`, `fix:`, etc.)
2. Check `RELEASE_PLEASE_TOKEN` secret is configured
3. View release-please workflow logs in Actions tab

### GoReleaser not triggered after merge

1. Verify `RELEASE_PLEASE_TOKEN` has repo scope (not just `contents: write`)
2. Check if tag was created: `git ls-remote --tags origin`
3. Manually trigger if needed: push the tag again

### Homebrew formula not updated

1. Check `HOMEBREW_TAP_GITHUB_TOKEN` secret is configured
2. Verify it's not a pre-release (Homebrew skips pre-releases)
3. Manually update formula in `systmms/homebrew-tap`

### Docker image not published

1. Check GitHub Container Registry permissions
2. Verify `packages: write` permission in workflow
3. Check workflow logs for Docker login errors

## Release Infrastructure

| Component | Purpose |
|-----------|---------|
| **release-please** | Automated versioning, changelog, tags |
| **GoReleaser** | Cross-platform builds, Docker, Homebrew |
| **GitHub Actions** | Workflow automation |
| **GitHub Container Registry** | Docker image hosting |
| **Homebrew Tap** | macOS/Linux package distribution |

Configuration files:
- `.github/workflows/release-please.yml` - Version management
- `.github/workflows/release.yml` - Build and publish
- `release-please-config.json` - Release-please settings
- `.release-please-manifest.json` - Current version
- `.goreleaser.yml` - GoReleaser configuration
- `Dockerfile` - Container image definition
