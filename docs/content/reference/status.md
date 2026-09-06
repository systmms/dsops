---
title: "Project Status & Roadmap"
description: "Current implementation status and future roadmap"
lead: "High-level project status with links to detailed feature specifications and implementation plans."
date: 2025-11-13T12:00:00-07:00
lastmod: 2025-11-15T10:00:00-07:00
draft: false
weight: 30
---

## Current Version: v0.2.x

**Status**: Core features 100% complete, release infrastructure complete, ready for production use

- ✅ **CLI & Architecture**: Cobra-based framework with 9 commands
- ✅ **Provider System**: 17 providers (1Password, Bitwarden, pass, AWS, GCP, Azure, Vault, Doppler, Keychain, Infisical, Akeyless)
- ✅ **Security Features**: Ephemeral execution, automatic redaction, process isolation, memory protection (memguard)
- ✅ **Transform Pipeline**: 8 transform functions (JSON, YAML, base64, etc.)
- ✅ **Output Formats**: dotenv, JSON, YAML, Go templates
- ✅ **Documentation**: 100% complete for all features
- ✅ **Release Infrastructure**: Automated releases with GoReleaser, Homebrew, Docker, macOS code signing, cosign signatures, SBOM

See [retrospective specs](https://github.com/systmms/dsops/tree/main/specs) (SPEC-001 through SPEC-004, SPEC-080 through SPEC-089) for detailed feature documentation.

### Release & Distribution (SPEC-020)

**Status**: ✅ **COMPLETE** - All distribution channels operational

- ✅ **GitHub Releases**: Automated binary builds for macOS, Linux, Windows (5 arch combinations)
- ✅ **Homebrew**: `brew install systmms/tap/dsops`
- ✅ **Docker**: `docker pull ghcr.io/systmms/dsops`
- ✅ **go install**: `go install github.com/systmms/dsops/cmd/dsops@latest`
- ✅ **macOS Signing**: Binaries signed and notarized with Apple Developer ID
- ✅ **Release Please**: Automated versioning from Conventional Commits
- ✅ **Checksums**: SHA256 verification for all release artifacts

**See [SPEC-020: Release & Distribution](https://github.com/systmms/dsops/blob/main/specs/020-release-distribution/spec.md) for details.**

### Machine-level Secret Stores (SPEC-027)

**Status**: 🟢 **IMPLEMENTED** - Projects reference secret stores by name; each machine declares them in a user-level config

- ✅ User config discovered via `--user-config`, `DSOPS_USER_CONFIG`, `$XDG_CONFIG_HOME/dsops/config.yaml`, `~/.config/dsops/config.yaml` (`%APPDATA%` on Windows); `none` disables
- ✅ Only `secretStores:` / legacy `providers:` allowed in the user file; any other section is rejected
- ✅ Project always wins on name collisions; shadowed user entries reported by `dsops doctor`
- ✅ `dsops doctor` and `dsops providers` print a "Configuration sources" block and a `SOURCE` column
- ✅ Missing-store errors name both the project file and the user file to create
- ✅ Warns when the user file (symlink target) or its directory is writable by others; nix-store files pass
- ✅ `DSOPS_CONFIG` now honoured as the default for `--config`
- ✅ Backwards compatible: no user file → byte-identical behaviour

**See [SPEC-027: Machine-level Secret Stores](https://github.com/systmms/dsops/blob/main/specs/027-machine-level-secret-stores/spec.md) for details.**

### Bitwarden Multi-Account (SPEC-026)

**Status**: 🟢 **IMPLEMENTED** - Bitwarden provider supports multiple accounts in one dsops.yaml

- ✅ Per-instance `appDataDir` field injects `BITWARDENCLI_APPDATA_DIR` on every `bw` subprocess
- ✅ Optional `server` URL reconciled via `bw config server` once per process
- ✅ Optional `email` field verified case-insensitively against `bw status`'s userEmail
- ✅ `dsops doctor` renders a per-Bitwarden-instance block with collision warnings
- ✅ Two headless providers sharing an `appDataDir` fail fast with a configuration error
- ✅ Backwards compatible: omitting all three new fields is byte-identical to pre-SPEC-026 behavior

**See [SPEC-026: Bitwarden Multi-Account](https://github.com/systmms/dsops/blob/main/specs/026-bitwarden-multi-account/spec.md) for details.**

### Security Trust Infrastructure (SPEC-023)

**Status**: ✅ **COMPLETE** - Security trust infrastructure operational

- ✅ **SECURITY.md**: Vulnerability disclosure policy with response timelines
- ✅ **Cosign Signatures**: Keyless signing for binaries and Docker images
- ✅ **SBOM Generation**: SPDX format Software Bill of Materials
- ✅ **Memory Protection**: memguard-based secure memory handling
- ✅ **Security Docs**: Threat model, architecture docs, verification guide

**See [SPEC-023: Security Trust](https://github.com/systmms/dsops/blob/main/specs/023-security-trust/spec.md) for details.**

---

## Upcoming: v0.2 - Testing & Rotation (Q1 2025)

### Testing Infrastructure

**Status**: ✅ **SPEC-005 COMPLETE** - All 169 implementation tasks finished (100%)
**Current Coverage**: 54.4% (was 22.1% → improved +32.3%)
**Target**: 80% test coverage
**Gap**: 25.6% remaining
**Overall Progress**: 70% complete (Infrastructure ✅, Core business logic ✅, Integration tests ✅)

**Package Coverage Highlights** (as of 2025-11-18):
- ✅ **Excellent** (≥80%): 17 packages including resolve (94%), secretstores (94%), template (91%), rotation (80%), config (83%), policy (100%), exec (100%)
- ⚠️ **Good** (50-79%): providers (35%, +24% from CLI abstraction), rotation pkg (52%), execenv (68%)
- ❌ **Remaining Gaps**: commands (26%, target 70%), 3 SDK providers need mock client refactoring

**Achievements (169/169 tasks complete)**:
- ✅ Test infrastructure production-ready (testutil/, fakes/, Docker Compose)
- ✅ Integration tests operational (Vault, AWS/LocalStack, PostgreSQL, error edge cases)
- ✅ Provider contract test framework with 4 CLI providers refactored
- ✅ TDD documentation published (workflow guide, test patterns, best practices)
- ✅ Critical business logic exceeds targets (resolve, rotation, services all ≥80%)
- ✅ CI/CD workflows with coverage gates enforced
- ✅ Security tests complete (redaction validation, race detection passing)
- ✅ 750+ new test cases added across 7 implementation phases
- ✅ Protocol adapters tested (SQL with sqlmock, NoSQL, certificates)

**Implementation Complete, Coverage Gap Analysis**:

The testing infrastructure is **production-ready** and all planned tasks are finished. The current 54.4% coverage represents a **+146% improvement** from the starting point (22.1%). Critical business logic packages exceed targets, but 3 areas need focused work to reach 80%:

1. **SDK Provider Refactoring** (internal/providers: 35% → 85%, gap -50%)
   - CLI-based providers successfully refactored with mock executor (Phase 7)
   - AWS, Azure, GCP providers need similar mock client interface pattern
   - **Estimated effort**: 3-5 days

2. **Command Test Expansion** (cmd/dsops/commands: 26% → 70%, gap -44%)
   - Infrastructure exists, needs more edge case coverage
   - **Estimated effort**: 2-3 days

3. **Batch Rotation** (pkg/rotation: 52% → 70%, gap -18%)
   - Feature not yet implemented (deferred to v0.3)
   - **Estimated effort**: 2-3 days if prioritized

**See [SPEC-005: Testing Strategy & Plan](https://github.com/systmms/dsops/blob/main/specs/features/005-testing-strategy.md) for:**
- Complete implementation summary (all 169 tasks)
- [TESTING-STATUS.md](https://github.com/systmms/dsops/blob/main/specs/005-testing-strategy/TESTING-STATUS.md) - Detailed coverage analysis
- Package-by-package coverage achievements vs targets
- TDD workflow guide and testing best practices (docs/developer/)
- Path to 80% coverage (well-scoped, 7-10 days estimated)

**Next Steps for 80% Coverage** (Follow-up Work):
1. **SDK Provider Mocking** (P0): Create mock clients for AWS, Azure, GCP SDKs - ~3-5 days
2. **Command Tests** (P0): Expand command handler edge case tests - ~2-3 days
3. **Batch Rotation** (P1): Implement and test (or defer to v0.3) - ~2-3 days
4. **Final Validation** (P1): Run coverage suite, update docs - ~1 day

**Estimated Time to 80%**: 7-10 business days of focused work

### Rotation Phase 5: Advanced Features

**Current**: 100% complete ✅ (16/16 features)
**Target**: 100% complete ✅

**See [SPEC-009: Rotation Phase 5 Completion](../../specs/009-phase-5-completion/spec.md) for:**
- 13 detailed user stories with acceptance criteria
- 11-week implementation plan
- Architecture diagrams and interfaces
- Configuration examples and security considerations

**Four major categories**:

1. **Notifications** (0% → 100%)
   - Slack integration for rotation events
   - Email notifications (SMTP with batching)
   - PagerDuty integration for incidents
   - Generic webhook notifications

2. **Rollback & Recovery** (25% → 100%)
   - Automatic rollback on verification failure
   - Manual rollback command (`dsops rotation rollback`)
   - Rollback notifications across all channels

3. **Health Monitoring** (25% → 100%)
   - Protocol-specific service health checks
   - Custom health scripts for validation
   - Prometheus metrics (success rate, duration, health status)

4. **Gradual Rollout** (25% → 100%)
   - Canary rotation strategy (single instance first)
   - Percentage rollout strategy (progressive waves)
   - Service group rotation (coordinate related services)

---

## Future: v0.3+ - Enterprise Features

**Phase 6 features** (see [SPEC-050](https://github.com/systmms/dsops/blob/main/specs/rotation/050-phase-5-completion.md) Future Enhancements):
- Rotation policies and compliance reporting (PCI-DSS, SOC2)
- Approval workflows and break-glass procedures
- Multi-environment coordination
- Scheduled maintenance windows
- Plugin system for custom strategies

**Additional planned work**:
- Terraform provider
- Kubernetes operator
- Additional password managers (LastPass, KeePassXC)
- Chaos testing integration

### New Secret Store Providers

**Status**: 📋 **SPEC-021 PLANNING** - Specification and contracts complete
**Spec**: [SPEC-021: New Providers](https://github.com/systmms/dsops/blob/main/specs/021-new-providers/spec.md)

Adding 3 new secret store providers to expand coverage:

| Provider | Type | Score | Use Case | Status |
|----------|------|-------|----------|--------|
| OS Keychain | `keychain` | 8.25 | Local dev, offline workflows | 📋 Spec complete |
| Infisical | `infisical` | 8.5 | Open-source SaaS alternative | 📋 Spec complete |
| Akeyless | `akeyless` | 8.2 | Enterprise zero-knowledge | 📋 Spec complete |

**Deliverables**:
- ✅ Unified specification with 5 user stories, 17 functional requirements
- ✅ Research document (library selection, API patterns, testing strategy)
- ✅ Data model with configuration schemas and error types
- ✅ Go contracts/interfaces for all 3 providers with mocks
- ⏳ Implementation tasks (next: `/speckit.tasks`)

---

## How to Contribute

Interested in contributing? Here's how to get started:

1. **Review specifications**: Read [SPEC-005](https://github.com/systmms/dsops/blob/main/specs/features/005-testing-strategy.md) or [SPEC-050](https://github.com/systmms/dsops/blob/main/specs/rotation/050-phase-5-completion.md) for detailed implementation plans
2. **Check open issues**: Visit [GitHub Issues](https://github.com/systmms/dsops/issues) for specific tasks
3. **Read contribution guide**: See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development setup
4. **Join discussions**: Ask questions in [GitHub Discussions](https://github.com/systmms/dsops/discussions)

---

## Implementation Tracking

For maintainers and contributors tracking detailed progress:

- **Project Principles**: [constitution.md](https://github.com/systmms/dsops/blob/main/.specify/memory/constitution.md)
- **All Specifications**: [specs/](https://github.com/systmms/dsops/tree/main/specs)
- **Testing Roadmap**: [SPEC-005](https://github.com/systmms/dsops/blob/main/specs/features/005-testing-strategy.md)
- **Rotation Roadmap**: [SPEC-050](https://github.com/systmms/dsops/blob/main/specs/rotation/050-phase-5-completion.md)

---

## Quick Stats

| Metric | Current | Target (v0.2) |
|--------|---------|---------------|
| Core Features | 100% ✅ | 100% |
| Provider Support | 17 providers ✅ | 17 (3 new in SPEC-021) |
| Test Coverage | 54.4% (Infrastructure ✅) | 80% ⚠️ |
| Test Infrastructure | 100% ✅ | 100% ✅ |
| Rotation Features | 56/61 (91%) | 61/61 (100%) ✅ |
| Release Infrastructure | 100% ✅ | 100% ✅ |
| Documentation | 100% ✅ | 100% |

Last updated: January 9, 2026
