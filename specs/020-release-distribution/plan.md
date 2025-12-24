# Implementation Plan: Release & Distribution Infrastructure

**Branch**: `020-release-distribution` | **Date**: 2025-12-24 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/020-release-distribution/spec.md`

## Summary

Implement automated release infrastructure that enables users to install dsops via Homebrew, GitHub Releases, Docker, and `go install`. The solution uses GoReleaser for cross-platform binary builds with changelog generation, GitHub Actions for workflow automation, and a separate Homebrew tap repository for formula distribution.

## Technical Context

**Language/Version**: Go 1.21+ (matches existing project)
**Primary Dependencies**: GoReleaser (v2.x), GitHub Actions, Docker
**Storage**: N/A (stateless release infrastructure)
**Testing**: Manual release testing + workflow validation
**Target Platform**: macOS (arm64, amd64), Linux (amd64, arm64), Windows (amd64)
**Project Type**: DevOps/Infrastructure (CI/CD configuration)
**Performance Goals**: Release workflow completes in under 10 minutes; Docker image under 50MB
**Constraints**: No code signing required for initial release
**Scale/Scope**: 5 platform/arch combinations, 3 distribution channels (Releases, Homebrew, Docker)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Compliance | Notes |
|-----------|------------|-------|
| I. Ephemeral-First | N/A | Release infrastructure doesn't handle secrets at runtime |
| II. Security by Default | ✅ | Checksums provided; Docker uses minimal base image |
| III. Provider-Agnostic | N/A | Not a provider feature |
| IV. Data-Driven Services | N/A | Not a service feature |
| V. Developer Experience | ✅ | Single-command install via Homebrew; clear version info |
| VI. Cross-Platform | ✅ | Builds for macOS, Linux, Windows (5 arch combinations) |
| VII. Test-Driven | ⚠️ | Release workflows tested manually; no unit tests for configs |
| VIII. Explicit Over Implicit | ✅ | Releases triggered only by explicit version tags |
| IX. Deterministic | ✅ | Same tag always produces same artifacts; checksums verify |

**Gate Status**: ✅ PASS - All applicable principles satisfied. TDD waiver justified for declarative config files.

## Project Structure

### Documentation (this feature)

```text
specs/020-release-distribution/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0: GoReleaser, Docker, Homebrew research
├── data-model.md        # Phase 1: Release artifact definitions
├── quickstart.md        # Phase 1: How to create a release
├── contracts/           # Phase 1: Workflow structure
│   ├── goreleaser.yaml  # GoReleaser configuration schema
│   └── release.yaml     # GitHub Actions workflow
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
.github/
├── workflows/
│   ├── ci.yml           # Existing: unit + integration tests
│   └── release.yml      # NEW: Release automation workflow
│
.goreleaser.yml          # NEW: GoReleaser configuration
Dockerfile               # NEW: Production Docker image
```

### External Repository

```text
systmms/homebrew-tap/  # NEW: Separate GitHub repository
└── Formula/
    └── dsops.rb         # Homebrew formula (auto-updated by GoReleaser)
```

**Structure Decision**: Minimal additions to existing repository structure. Release configs live at repo root. Homebrew tap is a separate repository to follow Homebrew conventions.

## Complexity Tracking

No constitution violations require justification. The implementation uses standard Go release patterns without introducing unnecessary complexity.

## Implementation Phases

### Phase 1: Core Release Infrastructure

1. **Create `.goreleaser.yml`** at repository root
   - Configure builds for 5 platform/arch combinations
   - Enable changelog generation from conventional commits
   - Configure archive naming and contents (binary, completions, LICENSE)
   - Set up checksum generation

2. **Create `.github/workflows/release.yml`**
   - Trigger on `v*` tags
   - Run tests before build
   - Execute GoReleaser with GitHub token
   - Verify no partial releases on failure

### Phase 2: Docker Distribution

3. **Create `Dockerfile`** at repository root
   - Multi-stage build for minimal image size
   - Use distroless or alpine base
   - Copy binary and set entrypoint
   - Target: under 50MB final image

4. **Add Docker publishing to release workflow**
   - Build and push to ghcr.io
   - Tag with version and `:latest`
   - Configure GitHub Container Registry permissions

### Phase 3: Homebrew Tap

5. **Create `systmms/homebrew-tap` repository**
   - Initialize with README and Formula directory
   - Configure repository for GoReleaser access

6. **Configure GoReleaser Homebrew integration**
   - Add homebrew tap configuration to `.goreleaser.yml`
   - Set up GitHub App or token for cross-repo push
   - Test formula generation

### Phase 4: Validation & Documentation

7. **Test complete release workflow**
   - Push test tag (e.g., `v0.2.0-rc.1`)
   - Verify all artifacts created
   - Test installation from each channel

8. **Update documentation**
   - Verify installation.md matches reality
   - Update CONTRIBUTING.md with release process
   - Add release process to developer docs

## Dependencies

- **External**: GitHub Container Registry access, Homebrew tap repository
- **Internal**: Existing Makefile LDFLAGS for version embedding
- **Secrets**: `GITHUB_TOKEN` (automatic), possibly `HOMEBREW_TAP_GITHUB_TOKEN` for cross-repo

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| GoReleaser breaking changes | Pin GoReleaser version in workflow |
| Docker build failures | Multi-stage with explicit base versions |
| Homebrew tap push failures | Manual fallback: push formula directly |
| Large Docker image | Use distroless base; verify size in CI |

## Success Verification

- [ ] `brew install systmms/tap/dsops` works on macOS
- [ ] Binary downloads available on GitHub Releases
- [ ] `docker run ghcr.io/systmms/dsops:latest --version` works
- [ ] `go install github.com/systmms/dsops/cmd/dsops@latest` works
- [ ] All releases have SHA256 checksums
- [ ] Release workflow completes in under 10 minutes
