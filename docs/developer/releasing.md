# Release Process

This document describes how to create releases for dsops.

## Prerequisites

- Push access to `systmms/dsops` repository
- Push access to `systmms/homebrew-tap` repository (for Homebrew updates)
- `HOMEBREW_TAP_GITHUB_TOKEN` secret configured in repository settings

## Creating a Release

### Step 1: Prepare the Release

1. Ensure all changes are merged to `main`
2. Verify CI passes on `main` branch
3. Review the changes since the last release

### Step 2: Create and Push Version Tag

```bash
# Determine the next version (follow semver)
# - MAJOR: Breaking changes
# - MINOR: New features, backward compatible
# - PATCH: Bug fixes, backward compatible

# Create annotated tag
git tag -a v1.0.0 -m "Release v1.0.0: Description of release"

# Push tag to trigger release workflow
git push origin v1.0.0
```

### Step 3: Monitor Release Workflow

1. Go to **Actions** tab in GitHub
2. Find the running "Release" workflow
3. Wait for completion (~5-10 minutes)

The workflow will:
- Run tests to verify build
- Build binaries for all platforms
- Generate checksums
- Create GitHub Release with changelog
- Build and push Docker image to ghcr.io
- Update Homebrew formula (for non-pre-releases)

### Step 4: Verify Release Artifacts

After workflow completes, verify:

- **GitHub Releases**: New release with:
  - 5 platform archives (darwin-arm64, darwin-amd64, linux-amd64, linux-arm64, windows-amd64)
  - Checksums file
  - Auto-generated changelog

- **Docker**: `docker pull ghcr.io/systmms/dsops:v1.0.0`

- **Homebrew**: Formula updated in `systmms/homebrew-tap`

## Pre-releases (Beta/RC)

For pre-release versions, use semver pre-release syntax:

```bash
# Beta release
git tag -a v1.0.0-beta.1 -m "Beta release v1.0.0-beta.1"
git push origin v1.0.0-beta.1

# Release candidate
git tag -a v1.0.0-rc.1 -m "Release candidate v1.0.0-rc.1"
git push origin v1.0.0-rc.1
```

Pre-releases:
- Are published to GitHub Releases (marked as pre-release)
- Are published to Docker with version tag (no `:latest`)
- Do NOT update Homebrew tap (stable releases only)

## Local Testing

Test GoReleaser locally without publishing:

```bash
# Install GoReleaser
brew install goreleaser

# Dry run (no publishing)
goreleaser release --snapshot --clean

# Check generated artifacts
ls dist/
```

## Troubleshooting

### Release workflow failed

1. Check workflow logs in GitHub Actions
2. Fix the issue
3. Delete the failed release (if created): `gh release delete v1.0.0`
4. Delete the tag: `git push --delete origin v1.0.0`
5. Re-create and push the tag

### Homebrew formula not updated

1. Check if `HOMEBREW_TAP_GITHUB_TOKEN` secret is configured
2. Manually update formula in `systmms/homebrew-tap`:
   ```bash
   cd ../homebrew-tap
   # Edit Formula/dsops.rb with new version and sha256
   git add . && git commit -m "dsops v1.0.0"
   git push
   ```

### Docker image not published

1. Check GitHub Container Registry permissions
2. Verify `packages: write` permission in workflow
3. Manually push if needed:
   ```bash
   docker build -t ghcr.io/systmms/dsops:v1.0.0 .
   docker push ghcr.io/systmms/dsops:v1.0.0
   ```

## Release Infrastructure

The release process uses:

- **GoReleaser**: Cross-platform builds, changelog, checksum generation
- **GitHub Actions**: Workflow automation
- **GitHub Container Registry**: Docker image hosting
- **Homebrew Tap** (`systmms/homebrew-tap`): macOS/Linux package distribution

Configuration files:
- `.goreleaser.yml` - GoReleaser configuration
- `.github/workflows/release.yml` - Release workflow
- `Dockerfile` - Container image definition
