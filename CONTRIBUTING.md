# Contributing to dsops

Thank you for your interest in contributing to dsops!

## Development Setup

```bash
# Enter development shell (has Go, tools, provider CLIs)
nix develop --impure

# Or use direnv for automatic activation
echo "use flake --impure" > .envrc && direnv allow

# Build and test
make build
make test
```

## Making Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run tests: `make test`
5. Run linter: `make lint`
6. Commit with conventional commit format (see below)
7. Push and open a Pull Request

## Conventional Commits

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for automated versioning and changelog generation.

### Commit Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

| Type | Description | Version Bump |
|------|-------------|--------------|
| `feat` | New feature | Minor (0.x.0) |
| `fix` | Bug fix | Patch (0.0.x) |
| `perf` | Performance improvement | Patch |
| `refactor` | Code refactoring | None |
| `docs` | Documentation only | None |
| `test` | Adding/updating tests | None |
| `ci` | CI/CD changes | None |
| `chore` | Maintenance tasks | None |

### Examples

```bash
# Feature (triggers minor version bump)
git commit -m "feat(providers): add Azure Key Vault support"

# Bug fix (triggers patch version bump)
git commit -m "fix(resolve): handle empty secret values correctly"

# Breaking change (triggers major version bump when v1.0+)
git commit -m "feat(config)!: rename 'providers' to 'secretStores'

BREAKING CHANGE: The 'providers' key in dsops.yaml is now 'secretStores'."
```

## Release Process

Releases are automated via [release-please](https://github.com/googleapis/release-please).

### How It Works

1. **Merge PRs to main** with conventional commit messages
2. **release-please creates a Release PR** that:
   - Bumps version based on commits
   - Updates CHANGELOG.md
   - Updates version in code
3. **Merge the Release PR** to:
   - Create a git tag (e.g., `v0.2.4`)
   - Trigger GoReleaser workflow
   - Publish binaries to GitHub Releases
   - Push Docker image to ghcr.io
   - Update Homebrew formula

### What Gets Released

- **GitHub Releases**: Pre-built binaries for macOS (arm64, amd64), Linux (arm64, amd64), Windows (amd64)
- **Docker**: `ghcr.io/systmms/dsops:<version>` and `ghcr.io/systmms/dsops:latest`
- **Homebrew**: `brew install systmms/tap/dsops`
- **Go**: `go install github.com/systmms/dsops/cmd/dsops@latest`

### Manual Release (Maintainers Only)

If you need to trigger a release manually:

```bash
# Create and push a version tag
git tag v0.3.0
git push origin v0.3.0
```

The release workflow will automatically:
- Run tests
- Build binaries for all platforms
- Create GitHub Release with changelog
- Push Docker images
- Update Homebrew tap

## Code Style

- Follow Go best practices
- Run `make lint` before committing
- Add tests for new functionality
- Keep commits focused and atomic

## Questions?

Open an issue or discussion on GitHub.
