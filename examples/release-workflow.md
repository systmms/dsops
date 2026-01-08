# Release Workflow Guide

This guide explains how to create releases for dsops using the automated release infrastructure.

## Overview

dsops uses a two-step release process:

1. **Release Please** creates a Release PR automatically when commits are merged to `main`
2. **GoReleaser** builds and publishes artifacts when a version tag is pushed

## Creating a Release

### Automatic (Recommended)

Simply merge commits to `main` following [Conventional Commits](https://www.conventionalcommits.org/):

```bash
# Features trigger minor version bump (0.1.0 -> 0.2.0)
git commit -m "feat: add new provider support"

# Fixes trigger patch version bump (0.1.0 -> 0.1.1)
git commit -m "fix: resolve timeout issue"

# Breaking changes trigger major version bump (0.1.0 -> 1.0.0)
git commit -m "feat!: change configuration format"
# or
git commit -m "feat: change API" \
  -m "BREAKING CHANGE: configuration schema has changed"
```

Release Please will:
1. Create/update a Release PR with changelog
2. When merged, create a version tag
3. GoReleaser then builds all artifacts

### Manual (For Hotfixes)

If you need to create a release manually:

```bash
# Ensure you're on main with latest changes
git checkout main
git pull origin main

# Create and push a version tag
git tag -a v0.2.5 -m "Release v0.2.5"
git push origin v0.2.5
```

## Version Tag Format

- Production: `v0.2.3`, `v1.0.0`
- Pre-release: `v0.2.3-rc.1`, `v0.2.3-beta.1`
- Test: `v0.2.3-test.1` (for testing release workflow)

## What Gets Published

When a tag is pushed, the release workflow:

1. **Runs tests** - Ensures code quality
2. **Builds binaries** for:
   - macOS (arm64, amd64)
   - Linux (amd64, arm64)
   - Windows (amd64)
3. **Signs macOS binaries** with Apple Developer ID
4. **Notarizes macOS binaries** for Gatekeeper
5. **Publishes to**:
   - GitHub Releases (binaries + checksums)
   - GitHub Container Registry (Docker image)
   - Homebrew tap (formula update)

## Monitoring Releases

### Check Workflow Status

```bash
# List recent workflow runs
gh run list --repo systmms/dsops --limit 5

# Watch a specific run
gh run watch <run-id> --repo systmms/dsops
```

### Verify Release Artifacts

```bash
# List release assets
gh release view v0.2.3 --repo systmms/dsops

# Download and verify checksum
curl -LO https://github.com/systmms/dsops/releases/download/v0.2.3/dsops_0.2.3_darwin_arm64.tar.gz
curl -LO https://github.com/systmms/dsops/releases/download/v0.2.3/dsops_0.2.3_checksums.txt
sha256sum -c dsops_0.2.3_checksums.txt --ignore-missing
```

### Verify Docker Image

```bash
docker pull ghcr.io/systmms/dsops:0.2.3
docker run --rm ghcr.io/systmms/dsops:0.2.3 --version
```

### Verify Homebrew

```bash
brew update
brew info systmms/tap/dsops  # Check version
brew upgrade dsops           # Upgrade if installed
```

## Troubleshooting

### Release PR Not Created

1. Check Release Please workflow ran: `.github/workflows/release-please.yml`
2. Verify commits follow conventional format
3. Check for workflow errors in Actions tab

### GoReleaser Fails

1. Check for test failures (tests run first)
2. Verify GITHUB_TOKEN permissions
3. Check GoReleaser logs for specific errors

### Homebrew Formula Not Updated

1. Verify HOMEBREW_TAP_GITHUB_TOKEN secret is set
2. Check `systmms/homebrew-tap` repository for push
3. GoReleaser skips formula for pre-release tags

### macOS Signing Fails

1. Verify Apple secrets are configured:
   - MACOS_SIGN_P12
   - MACOS_SIGN_PASSWORD
   - MACOS_NOTARY_ISSUER_ID
   - MACOS_NOTARY_KEY_ID
   - MACOS_NOTARY_KEY
2. Check certificate hasn't expired
3. Signing is optional - release continues without it

## Release Checklist

Before merging a Release PR:

- [ ] All CI checks pass
- [ ] Changelog looks correct
- [ ] Version bump is appropriate (major/minor/patch)
- [ ] No breaking changes without `!` or `BREAKING CHANGE`

After release completes:

- [ ] GitHub Release page shows all assets
- [ ] Docker image pulls successfully
- [ ] Homebrew formula updated (non-prerelease only)
- [ ] Binary runs without Gatekeeper warnings on macOS
