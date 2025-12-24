# Research: Release & Distribution Infrastructure

**Date**: 2025-12-24
**Feature**: SPEC-020 Release & Distribution

## Research Questions

1. What is the best tool for Go binary releases?
2. What is the optimal Docker base image for a Go CLI?
3. How should Homebrew tap be configured for automatic updates?
4. What are the best practices for GitHub Actions release workflows?

---

## Decision 1: Release Automation Tool

**Decision**: Use GoReleaser v2.x

**Rationale**:
- Industry standard for Go project releases
- Native support for cross-platform builds (GOOS/GOARCH)
- Built-in changelog generation from conventional commits
- Direct integration with Homebrew taps
- Docker image building and pushing
- Checksum generation
- GitHub Release creation

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| Manual Makefile + scripts | More maintenance, no changelog automation |
| goreleaser-action only | Less flexible than full GoReleaser config |
| ko (Google) | Designed for K8s images, not general CLI distribution |
| Custom GitHub Actions | Reinvents wheel, more error-prone |

**Key Configuration Choices**:
- Archive format: `.tar.gz` for Unix, `.zip` for Windows
- Changelog: Use conventional commits with `use: github`
- Version: Embed via ldflags (existing Makefile pattern)
- Completions: Generate bash, zsh, fish during build

---

## Decision 2: Docker Base Image

**Decision**: Use `gcr.io/distroless/static-debian12:nonroot`

**Rationale**:
- Minimal attack surface (no shell, no package manager)
- Very small size (~2MB base)
- Go static binaries don't need glibc
- `nonroot` variant for security best practices
- Well-maintained by Google

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| `alpine:latest` | Larger (~7MB), includes shell which is attack surface |
| `scratch` | No CA certificates, harder to debug |
| `debian:slim` | Much larger (~80MB), unnecessary for static Go binary |
| `ubuntu:latest` | Very large (~78MB), unnecessary |

**Key Configuration Choices**:
- Multi-stage build: First stage builds, second stage copies binary
- Copy CA certificates from builder for HTTPS
- Set non-root user (UID 65532 for distroless)
- Working directory: `/work` for volume mounts

**Image Size Target**: Under 20MB (distroless base + ~15MB Go binary)

---

## Decision 3: Homebrew Tap Structure

**Decision**: Create `systmms/homebrew-dsops` as a separate repository

**Rationale**:
- Homebrew convention: taps are named `homebrew-<name>`
- Enables `brew tap systmms/dsops` then `brew install dsops`
- Or direct: `brew install systmms/tap/dsops`
- GoReleaser can auto-update formula via GitHub token

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| Formula in main repo | Homebrew doesn't support this pattern well |
| Submit to homebrew-core | Requires significant adoption first |
| GitHub Packages Registry | Less familiar to users than taps |

**Key Configuration Choices**:
- Formula template: GoReleaser auto-generates from release artifacts
- Token: `HOMEBREW_TAP_GITHUB_TOKEN` secret with repo scope
- Dependencies: None (static binary)
- Test: Simple `system "#{bin}/dsops", "--version"` check

---

## Decision 4: GitHub Actions Workflow Pattern

**Decision**: Single release workflow triggered by version tags

**Rationale**:
- Simple trigger: `push: tags: ['v*']`
- Tests run before release (fail-fast)
- GoReleaser handles all artifact creation
- Single workflow = single source of truth

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| Manual workflow_dispatch | More friction, easy to forget steps |
| Release on push to main | Too frequent, no version control |
| Separate workflows per artifact | Harder to coordinate, race conditions |

**Key Configuration Choices**:
- Trigger: `on: push: tags: ['v*']`
- Permissions: `contents: write`, `packages: write`
- Concurrency: Cancel in-progress for same tag
- Fail-fast: Entire workflow fails if any step fails

---

## Decision 5: Version Embedding

**Decision**: Use existing Makefile LDFLAGS pattern via GoReleaser

**Rationale**:
- Makefile already defines: `-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)`
- GoReleaser supports same pattern with `ldflags` config
- Consistent between local builds and releases

**Key Configuration Choices**:
```yaml
# .goreleaser.yml
ldflags:
  - -s -w
  - -X main.version={{.Version}}
  - -X main.commit={{.ShortCommit}}
  - -X main.date={{.Date}}
```

---

## Decision 6: Shell Completion Bundling

**Decision**: Generate completions during GoReleaser build, include in archives

**Rationale**:
- dsops already supports `completion` subcommand
- Pre-generated completions save users a step
- Standard practice for CLI tools

**Implementation**:
- GoReleaser `hooks.post` runs `dsops completion bash/zsh/fish`
- Completions added to archive contents
- Homebrew formula installs to correct locations

---

## References

- [GoReleaser Documentation](https://goreleaser.com/intro/)
- [GoReleaser Homebrew Integration](https://goreleaser.com/customization/homebrew/)
- [Distroless Images](https://github.com/GoogleContainerTools/distroless)
- [GitHub Actions Permissions](https://docs.github.com/en/actions/security-guides/automatic-token-authentication)
- [Homebrew Tap Creation](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap)
