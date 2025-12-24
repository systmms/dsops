# Quickstart: Creating a dsops Release

**Date**: 2025-12-24
**Feature**: SPEC-020 Release & Distribution

## Prerequisites

- Push access to `systmms/dsops` repository
- Push access to `systmms/homebrew-dsops` repository (for Homebrew updates)

## Creating a Release

### Step 1: Prepare the Release

1. Ensure all changes are merged to `main`
2. Verify CI passes on `main` branch
3. Update CHANGELOG.md if not auto-generated

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

### Step 4: Verify Release Artifacts

After workflow completes, verify:

- [ ] **GitHub Releases**: New release with:
  - 5 platform archives (darwin-arm64, darwin-amd64, linux-amd64, linux-arm64, windows-amd64)
  - Checksums file
  - Auto-generated changelog

- [ ] **Docker**: `docker pull ghcr.io/systmms/dsops:v1.0.0`

- [ ] **Homebrew**: Formula updated in `systmms/homebrew-dsops`

## Testing the Release

### Test Binary Download

```bash
# macOS (Apple Silicon)
curl -L https://github.com/systmms/dsops/releases/download/v1.0.0/dsops_1.0.0_darwin_arm64.tar.gz -o dsops.tar.gz
tar -xzf dsops.tar.gz
./dsops --version
```

### Test Homebrew

```bash
brew update
brew install systmms/tap/dsops
dsops --version
```

### Test Docker

```bash
docker run --rm ghcr.io/systmms/dsops:v1.0.0 --version
docker run --rm ghcr.io/systmms/dsops:latest --version
```

### Test go install

```bash
go install github.com/systmms/dsops/cmd/dsops@v1.0.0
dsops --version
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
2. Manually update formula in `systmms/homebrew-dsops`:
   ```bash
   cd ../homebrew-dsops
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

## Pre-release (Beta/RC) Versions

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
- ✅ Published to GitHub Releases (marked as pre-release)
- ✅ Published to Docker with version tag (no `:latest`)
- ❌ NOT published to Homebrew tap (stable only)

## Local Testing Before Release

Test GoReleaser locally without publishing:

```bash
# Install GoReleaser
brew install goreleaser

# Dry run (no publishing)
goreleaser release --snapshot --clean

# Check generated artifacts
ls dist/
```
