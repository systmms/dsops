# Implementation Plan: Security Trust Infrastructure

**Branch**: `023-security-trust` | **Date**: 2026-01-09 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/023-security-trust/spec.md`

## Summary

Implement trust-building infrastructure for dsops including vulnerability disclosure policy (SECURITY.md), release artifact signing (cosign), SBOM generation, security documentation (threat model), and memory protection for secrets.

## Technical Context

**Language/Version**: Go 1.25
**Primary Dependencies**: GoReleaser v2, cosign (Sigstore), syft (SBOM), memguard
**Storage**: N/A (documentation + CI/CD changes + runtime memory protection)
**Testing**: go test, manual verification of signatures
**Target Platform**: Linux, macOS, Windows (cross-platform)
**Project Type**: Single CLI application
**Performance Goals**: Memory protection should add <5% overhead to secret operations
**Constraints**: Keyless signing via Sigstore OIDC (no key management), mlock limits on some systems
**Scale/Scope**: 10 functional requirements, 5 user stories, affects release workflow

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| **I. Ephemeral-First** | ✅ Pass | Memory protection enhances ephemeral design |
| **II. Security by Default** | ✅ Pass | **Direct alignment** - panic handler scrubbing, memory protection explicitly mentioned |
| **III. Provider-Agnostic** | ✅ Pass | No changes to provider interface |
| **IV. Data-Driven** | ✅ Pass | No changes to service architecture |
| **V. Developer Experience** | ✅ Pass | Clear verification docs, helpful error messages |
| **VI. Cross-Platform** | ⚠️ Attention | mlock behavior varies by platform; must document |
| **VII. Test-Driven** | ✅ Pass | Tests required for memory protection |
| **VIII. Explicit Over Implicit** | ✅ Pass | Memory protection is opt-in via build flags if needed |
| **IX. Deterministic** | ✅ Pass | No changes to resolution pipeline |

**Gate Result**: PASS - All principles satisfied or explicitly aligned.

## Project Structure

### Documentation (this feature)

```text
specs/023-security-trust/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # N/A (no data models)
├── quickstart.md        # Phase 1 output
├── contracts/           # N/A (no APIs)
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
# Root level
SECURITY.md                                    # NEW: Vulnerability disclosure policy

# Documentation
docs/content/security/
├── _index.md                                  # NEW: Security overview
├── threat-model.md                            # NEW: Threat model
├── architecture.md                            # NEW: Security architecture
└── verify-releases.md                         # NEW: Release verification guide

# Release infrastructure
.goreleaser.yml                                # MODIFY: Add SBOM, cosign signing
.github/workflows/release.yml                  # MODIFY: Add cosign steps

# Memory protection
internal/secure/
├── enclave.go                                 # NEW: Secure memory wrapper
├── enclave_test.go                            # NEW: Tests
└── doc.go                                     # NEW: Package documentation

# Integration points
pkg/provider/provider.go                       # No changes (SecretValue remains string)
internal/resolve/resolver.go                   # MODIFY: Use secure enclave for values
```

**Structure Decision**: Extends existing structure with new `internal/secure/` package for memory protection and `docs/content/security/` for security documentation.

## Complexity Tracking

> No violations requiring justification.

## Implementation Phases

### Phase 1: Documentation (FR-001, FR-002, FR-007, FR-008)

**Priority**: P1 - Immediate trust building, no code changes

1. **Create SECURITY.md** (root level)
   - Supported versions (current major + previous)
   - Reporting methods (GitHub Security Advisories, security@systmms.com)
   - Response timeline (48h acknowledgment, 90-day disclosure)
   - Out-of-scope issues

2. **Create security documentation**
   - `docs/content/security/_index.md` - Overview
   - `docs/content/security/threat-model.md` - What dsops protects against
   - `docs/content/security/architecture.md` - Ephemeral design, redaction, isolation
   - `docs/content/security/verify-releases.md` - How to verify signatures

### Phase 2: Release Signing & SBOM (FR-003, FR-004, FR-005, FR-006)

**Priority**: P1 - Supply chain security

1. **Add SBOM generation to GoReleaser**
   ```yaml
   sboms:
     - artifacts: archive
       documents:
         - "{{ .ProjectName }}_{{ .Version }}_sbom.spdx.json"
   ```

2. **Add cosign signing to release workflow**
   - Sign checksums file with keyless signing
   - Sign Docker images with cosign
   - Use OIDC identity from GitHub Actions

3. **Update verification documentation**
   - Document `cosign verify-blob` commands
   - Document `cosign verify` for Docker images
   - Include fallback SHA256 verification

### Phase 3: Memory Protection (FR-009, FR-010)

**Priority**: P2 - Runtime security enhancement

1. **Create `internal/secure` package**
   - `Enclave` type wrapping sensitive byte slices
   - mlock to prevent swapping
   - Secure zeroing on destruction
   - Guard pages (if supported)

2. **Integration approach** (minimal changes)
   - Wrap secret values during resolution
   - Zero values after injection into child process
   - Log warning if mlock unavailable
   - No changes to Provider interface (keep simple)

3. **Platform considerations**
   - Linux: mlock with RLIMIT_MEMLOCK
   - macOS: mlock available
   - Windows: VirtualLock equivalent
   - Graceful degradation with logging

## Dependencies

- **Existing**: GoReleaser v2, GitHub Actions, Docker buildx
- **New**: cosign (Sigstore), syft (SBOM generation), memguard (memory protection)
- **Related specs**: SPEC-020 (release-distribution) provides foundation

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Sigstore outage during release | Release delayed | Checksums still generated; can sign later |
| mlock limits exceeded | Secrets not protected | Log warning, document ulimit configuration |
| memguard compatibility issues | Build failures | Wrap in build tags, provide fallback |

## Verification Plan

1. **SECURITY.md**: Manual review for completeness
2. **Documentation**: Hugo build + review
3. **Signing**: Create test release, verify with `cosign verify-blob`
4. **SBOM**: Validate SPDX format with `syft`
5. **Memory protection**: Unit tests + manual verification with core dump test
